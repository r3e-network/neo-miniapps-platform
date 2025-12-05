using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.CRE
{
    /// <summary>
    /// CREService - Chainlink Runtime Environment contract.
    ///
    /// Provides serverless function execution environment for Chainlink-compatible jobs.
    ///
    /// Flow:
    /// 1. User deploys function code via Gateway
    /// 2. Service Layer (TEE) executes function in isolated environment
    /// 3. Execution result delivered back to caller
    ///
    /// Supports JavaScript/TypeScript functions with secure execution guarantees.
    /// </summary>
    [DisplayName("CREService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "CRE Service - Chainlink Runtime Environment")]
    [ContractPermission("*", "*")]
    public class CREService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_FUNCTION = 0x10;
        private const byte PREFIX_EXECUTION = 0x11;
        private const byte PREFIX_FUNCTION_COUNT = 0x20;
        private const byte PREFIX_PAUSED = 0x30;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a function is deployed.
        /// </summary>
        [DisplayName("FunctionDeployed")]
        public static event Action<ByteString, UInt160, string, ByteString> OnFunctionDeployed;
        // Parameters: functionId, owner, runtime, codeHash

        /// <summary>
        /// Emitted when a function execution is requested.
        /// </summary>
        [DisplayName("FunctionExecutionRequest")]
        public static event Action<ByteString, ByteString, UInt160, ByteString> OnFunctionExecutionRequest;
        // Parameters: executionId, functionId, caller, input

        /// <summary>
        /// Emitted when function execution completes.
        /// </summary>
        [DisplayName("FunctionExecutionComplete")]
        public static event Action<ByteString, bool, ByteString, BigInteger> OnFunctionExecutionComplete;
        // Parameters: executionId, success, output, gasUsed

        // ==================== Admin Methods ====================

        public static UInt160 GetAdmin()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
            return stored != null ? (UInt160)stored : InitialAdmin;
        }

        public static void SetAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        public static UInt160 GetGateway()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ==================== Function Management ====================

        /// <summary>
        /// Deploys a new function. Called via Gateway.
        /// </summary>
        public static void DeployFunction(ByteString functionId, UInt160 owner, string runtime, ByteString code, ByteString metadata)
        {
            RequireGateway();
            RequireNotPaused();

            // Validate runtime
            if (runtime != "javascript" && runtime != "typescript" && runtime != "wasm")
                throw new Exception("Unsupported runtime");

            // Check if function already exists
            if (GetFunction(functionId) != null)
                throw new Exception("Function already exists");

            var codeHash = CryptoLib.Sha256(code);

            var function = new CREFunction
            {
                FunctionId = functionId,
                Owner = owner,
                Runtime = runtime,
                CodeHash = codeHash,
                Metadata = metadata,
                CreatedAt = Runtime.Time,
                IsActive = true
            };
            StoreFunction(functionId, function);

            IncrementFunctionCount();

            OnFunctionDeployed(functionId, owner, runtime, codeHash);
        }

        /// <summary>
        /// Gets a function by ID.
        /// </summary>
        public static CREFunction GetFunction(ByteString functionId)
        {
            var key = GetFunctionKey(functionId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (CREFunction)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Deactivates a function. Only owner can call.
        /// </summary>
        public static void DeactivateFunction(ByteString functionId)
        {
            var function = GetFunction(functionId);
            if (function == null) throw new Exception("Function not found");
            if (!Runtime.CheckWitness(function.Owner)) throw new Exception("Only owner");

            function.IsActive = false;
            StoreFunction(functionId, function);
        }

        /// <summary>
        /// Gets the total function count.
        /// </summary>
        public static BigInteger GetFunctionCount()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_FUNCTION_COUNT });
            return stored != null ? (BigInteger)stored : 0;
        }

        // ==================== Function Execution ====================

        /// <summary>
        /// Processes a function execution request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (CRERequestData)StdLib.Deserialize(payload);

            // Validate function exists and is active
            var function = GetFunction(requestData.FunctionId);
            if (function == null) throw new Exception("Function not found");
            if (!function.IsActive) throw new Exception("Function not active");

            // Store execution
            var execution = new CREExecution
            {
                ExecutionId = requestId,
                FunctionId = requestData.FunctionId,
                Caller = requester,
                Input = requestData.Input,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreExecution(requestId, execution);

            OnFunctionExecutionRequest(requestId, requestData.FunctionId, requester, requestData.Input);
        }

        /// <summary>
        /// Delivers function execution result. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString output, ByteString signature)
        {
            RequireGateway();

            var execution = GetExecution(requestId);
            if (execution == null) throw new Exception("Execution not found");
            if (execution.Status != 0) throw new Exception("Already processed");

            execution.Status = success ? (byte)1 : (byte)2;
            execution.ProcessedAt = Runtime.Time;
            execution.Output = output;
            StoreExecution(requestId, execution);

            // Calculate gas used (simplified)
            var gasUsed = Runtime.Time - execution.CreatedAt;

            OnFunctionExecutionComplete(requestId, success, output, gasUsed);
        }

        /// <summary>
        /// Gets an execution by ID.
        /// </summary>
        public static CREExecution GetExecution(ByteString executionId)
        {
            var key = GetExecutionKey(executionId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (CREExecution)StdLib.Deserialize(stored);
        }

        // ==================== Helper Methods ====================

        private static void RequireAdmin()
        {
            if (!Runtime.CheckWitness(GetAdmin()))
                throw new Exception("Only admin");
        }

        private static void RequireGateway()
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }

        private static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract paused");
        }

        private static byte[] GetFunctionKey(ByteString functionId)
        {
            return Helper.Concat(new byte[] { PREFIX_FUNCTION }, functionId);
        }

        private static byte[] GetExecutionKey(ByteString executionId)
        {
            return Helper.Concat(new byte[] { PREFIX_EXECUTION }, executionId);
        }

        private static void StoreFunction(ByteString functionId, CREFunction function)
        {
            var key = GetFunctionKey(functionId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(function));
        }

        private static void StoreExecution(ByteString executionId, CREExecution execution)
        {
            var key = GetExecutionKey(executionId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(execution));
        }

        private static void IncrementFunctionCount()
        {
            var count = GetFunctionCount();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_FUNCTION_COUNT }, count + 1);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class CRERequestData
    {
        public ByteString FunctionId;
        public ByteString Input;
    }

    public class CREFunction
    {
        public ByteString FunctionId;
        public UInt160 Owner;
        public string Runtime;
        public ByteString CodeHash;
        public ByteString Metadata;
        public BigInteger CreatedAt;
        public bool IsActive;
    }

    public class CREExecution
    {
        public ByteString ExecutionId;
        public ByteString FunctionId;
        public UInt160 Caller;
        public ByteString Input;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
        public ByteString Output;
    }
}
