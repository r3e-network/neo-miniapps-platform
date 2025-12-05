using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Confidential
{
    /// <summary>
    /// ConfidentialService - Confidential computing contract.
    ///
    /// Provides secure computation on encrypted data using TEE.
    ///
    /// Flow:
    /// 1. User submits encrypted computation request via Gateway
    /// 2. Service Layer (TEE) decrypts, computes, and re-encrypts result
    /// 3. Encrypted result delivered back to user
    ///
    /// Ensures data privacy throughout the computation lifecycle.
    /// </summary>
    [DisplayName("ConfidentialService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Confidential Service - Secure computation on encrypted data")]
    [ContractPermission("*", "*")]
    public class ConfidentialService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_COMPUTATION = 0x10;
        private const byte PREFIX_ENCLAVE_KEY = 0x20;
        private const byte PREFIX_PAUSED = 0x30;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a confidential computation request is created.
        /// </summary>
        [DisplayName("ConfidentialRequest")]
        public static event Action<ByteString, UInt160, string, ByteString> OnConfidentialRequest;
        // Parameters: requestId, requester, computationType, encryptedInput

        /// <summary>
        /// Emitted when computation result is delivered.
        /// </summary>
        [DisplayName("ConfidentialResponse")]
        public static event Action<ByteString, bool, ByteString> OnConfidentialResponse;
        // Parameters: requestId, success, encryptedOutput

        /// <summary>
        /// Emitted when enclave attestation is updated.
        /// </summary>
        [DisplayName("EnclaveAttested")]
        public static event Action<ByteString, ByteString> OnEnclaveAttested;
        // Parameters: enclaveId, attestationReport

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

        // ==================== Enclave Management ====================

        /// <summary>
        /// Registers an enclave public key. Only admin can call.
        /// </summary>
        public static void RegisterEnclaveKey(ByteString enclaveId, ByteString publicKey, ByteString attestation)
        {
            RequireAdmin();
            var key = GetEnclaveKeyKey(enclaveId);
            var enclaveKey = new EnclaveKey
            {
                EnclaveId = enclaveId,
                PublicKey = publicKey,
                Attestation = attestation,
                RegisteredAt = Runtime.Time,
                IsActive = true
            };
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(enclaveKey));
            OnEnclaveAttested(enclaveId, attestation);
        }

        /// <summary>
        /// Gets enclave key information.
        /// </summary>
        public static EnclaveKey GetEnclaveKey(ByteString enclaveId)
        {
            var key = GetEnclaveKeyKey(enclaveId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (EnclaveKey)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Revokes an enclave key. Only admin can call.
        /// </summary>
        public static void RevokeEnclaveKey(ByteString enclaveId)
        {
            RequireAdmin();
            var enclaveKey = GetEnclaveKey(enclaveId);
            if (enclaveKey == null) throw new Exception("Enclave not found");
            enclaveKey.IsActive = false;
            var key = GetEnclaveKeyKey(enclaveId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(enclaveKey));
        }

        // ==================== Confidential Computation ====================

        /// <summary>
        /// Processes a confidential computation request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (ConfidentialRequestData)StdLib.Deserialize(payload);

            // Validate computation type
            if (requestData.ComputationType != "aggregate" &&
                requestData.ComputationType != "transform" &&
                requestData.ComputationType != "analyze")
                throw new Exception("Invalid computation type");

            // Store computation request
            var computation = new ConfidentialComputation
            {
                RequestId = requestId,
                Requester = requester,
                ComputationType = requestData.ComputationType,
                EncryptedInput = requestData.EncryptedInput,
                EnclaveId = requestData.EnclaveId,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreComputation(requestId, computation);

            OnConfidentialRequest(requestId, requester, requestData.ComputationType, requestData.EncryptedInput);
        }

        /// <summary>
        /// Delivers confidential computation result. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString encryptedOutput, ByteString signature)
        {
            RequireGateway();

            var computation = GetComputation(requestId);
            if (computation == null) throw new Exception("Computation not found");
            if (computation.Status != 0) throw new Exception("Already processed");

            computation.Status = success ? (byte)1 : (byte)2;
            computation.ProcessedAt = Runtime.Time;
            computation.EncryptedOutput = encryptedOutput;
            StoreComputation(requestId, computation);

            OnConfidentialResponse(requestId, success, encryptedOutput);
        }

        /// <summary>
        /// Gets a computation by ID.
        /// </summary>
        public static ConfidentialComputation GetComputation(ByteString requestId)
        {
            var key = GetComputationKey(requestId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (ConfidentialComputation)StdLib.Deserialize(stored);
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

        private static byte[] GetComputationKey(ByteString requestId)
        {
            return Helper.Concat(new byte[] { PREFIX_COMPUTATION }, requestId);
        }

        private static byte[] GetEnclaveKeyKey(ByteString enclaveId)
        {
            return Helper.Concat(new byte[] { PREFIX_ENCLAVE_KEY }, enclaveId);
        }

        private static void StoreComputation(ByteString requestId, ConfidentialComputation computation)
        {
            var key = GetComputationKey(requestId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(computation));
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class ConfidentialRequestData
    {
        public string ComputationType;
        public ByteString EncryptedInput;
        public ByteString EnclaveId;
    }

    public class ConfidentialComputation
    {
        public ByteString RequestId;
        public UInt160 Requester;
        public string ComputationType;
        public ByteString EncryptedInput;
        public ByteString EnclaveId;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
        public ByteString EncryptedOutput;
    }

    public class EnclaveKey
    {
        public ByteString EnclaveId;
        public ByteString PublicKey;
        public ByteString Attestation;
        public BigInteger RegisteredAt;
        public bool IsActive;
    }
}
