using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    /// <summary>
    /// ServiceLayerGateway - The central entry point for all service requests.
    ///
    /// Architecture:
    /// - All user requests MUST go through this gateway
    /// - All service callbacks MUST go through this gateway
    /// - Gas fees are managed here
    /// - Request validation and routing happens here
    ///
    /// Flow:
    /// 1. User Contract -> Gateway.Request() -> Emits ServiceRequest event
    /// 2. Service Layer (TEE) monitors events, processes request
    /// 3. Service Layer -> Gateway.Callback() -> Calls user's callback method
    /// </summary>
    [DisplayName("ServiceLayerGateway")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Email", "dev@r3e.network")]
    [ManifestExtra("Description", "Service Layer Gateway Contract - Central entry point for all services")]
    [ContractPermission("*", "*")]
    public class ServiceLayerGateway : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_SERVICE_LAYER = 0x02;
        private const byte PREFIX_SERVICE = 0x10;
        private const byte PREFIX_REQUEST = 0x20;
        private const byte PREFIX_GAS_BALANCE = 0x30;
        private const byte PREFIX_REQUEST_COUNTER = 0x40;
        private const byte PREFIX_PAUSED = 0x50;
        private const byte PREFIX_MIN_GAS = 0x60;

        // ==================== Constants ====================
        private const long DEFAULT_MIN_GAS = 1_00000000; // 1 GAS minimum

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a new service request is created.
        /// Service Layer monitors this event to process requests.
        /// </summary>
        [DisplayName("ServiceRequest")]
        public static event Action<ByteString, string, UInt160, UInt160, string, ByteString, BigInteger> OnServiceRequest;
        // Parameters: requestId, serviceId, requester, callbackContract, callbackMethod, payload, gasDeposit

        /// <summary>
        /// Emitted when a service response is delivered.
        /// </summary>
        [DisplayName("ServiceResponse")]
        public static event Action<ByteString, string, bool, ByteString, BigInteger> OnServiceResponse;
        // Parameters: requestId, serviceId, success, result, gasUsed

        /// <summary>
        /// Emitted when GAS is deposited.
        /// </summary>
        [DisplayName("GasDeposited")]
        public static event Action<UInt160, BigInteger> OnGasDeposited;

        /// <summary>
        /// Emitted when GAS is withdrawn.
        /// </summary>
        [DisplayName("GasWithdrawn")]
        public static event Action<UInt160, BigInteger> OnGasWithdrawn;

        /// <summary>
        /// Emitted when a service is registered.
        /// </summary>
        [DisplayName("ServiceRegistered")]
        public static event Action<string, UInt160> OnServiceRegistered;

        // ==================== Admin Methods ====================

        public static UInt160 GetAdmin()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
            return stored != null ? (UInt160)stored : InitialAdmin;
        }

        public static void SetAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (!newAdmin.IsValid) throw new Exception("Invalid admin");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        public static UInt160 GetServiceLayer()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_SERVICE_LAYER });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        public static void SetServiceLayer(UInt160 serviceLayer)
        {
            RequireAdmin();
            if (!serviceLayer.IsValid) throw new Exception("Invalid service layer");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_SERVICE_LAYER }, serviceLayer);
        }

        public static BigInteger GetMinGas()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_MIN_GAS });
            return stored != null ? (BigInteger)stored : DEFAULT_MIN_GAS;
        }

        public static void SetMinGas(BigInteger minGas)
        {
            RequireAdmin();
            if (minGas < 0) throw new Exception("Invalid min gas");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_MIN_GAS }, minGas);
        }

        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        public static void Pause()
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1);
        }

        public static void Unpause()
        {
            RequireAdmin();
            Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
        }

        // ==================== Service Registration ====================

        /// <summary>
        /// Registers a service contract. Only admin can call.
        /// </summary>
        public static void RegisterService(string serviceId, UInt160 contractHash)
        {
            RequireAdmin();
            if (string.IsNullOrEmpty(serviceId)) throw new Exception("Invalid service ID");
            if (!contractHash.IsValid) throw new Exception("Invalid contract hash");

            var key = GetServiceKey(serviceId);
            Storage.Put(Storage.CurrentContext, key, contractHash);
            OnServiceRegistered(serviceId, contractHash);
        }

        /// <summary>
        /// Gets the contract hash for a service.
        /// </summary>
        public static UInt160 GetServiceContract(string serviceId)
        {
            var key = GetServiceKey(serviceId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        // ==================== Gas Management ====================

        /// <summary>
        /// Called when GAS is transferred to this contract.
        /// Automatically credits the sender's balance.
        /// </summary>
        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash)
                throw new Exception("Only GAS accepted");
            if (amount <= 0)
                throw new Exception("Invalid amount");

            // Credit the sender's gas balance
            var key = GetGasBalanceKey(from);
            var currentBalance = GetGasBalance(from);
            Storage.Put(Storage.CurrentContext, key, currentBalance + amount);

            OnGasDeposited(from, amount);
        }

        /// <summary>
        /// Withdraws GAS from the caller's balance.
        /// </summary>
        public static void WithdrawGas(BigInteger amount)
        {
            var caller = Runtime.CallingScriptHash;
            if (amount <= 0) throw new Exception("Invalid amount");

            var balance = GetGasBalance(caller);
            if (balance < amount) throw new Exception("Insufficient balance");

            var key = GetGasBalanceKey(caller);
            Storage.Put(Storage.CurrentContext, key, balance - amount);

            // Transfer GAS back to caller
            GAS.Transfer(Runtime.ExecutingScriptHash, caller, amount, null);

            OnGasWithdrawn(caller, amount);
        }

        /// <summary>
        /// Gets the GAS balance for a contract.
        /// </summary>
        public static BigInteger GetGasBalance(UInt160 account)
        {
            var key = GetGasBalanceKey(account);
            var stored = Storage.Get(Storage.CurrentContext, key);
            return stored != null ? (BigInteger)stored : 0;
        }

        // ==================== Service Request ====================

        /// <summary>
        /// Submits a service request. Called by user contracts.
        ///
        /// The request is validated, gas is reserved, and a ServiceRequest event is emitted.
        /// The Service Layer (TEE) monitors these events and processes requests.
        /// </summary>
        /// <param name="serviceId">The service to invoke (oracle, vrf, etc.)</param>
        /// <param name="callbackContract">Contract to receive the callback</param>
        /// <param name="callbackMethod">Method to call with the result</param>
        /// <param name="payload">Service-specific request data</param>
        /// <param name="gasLimit">Maximum GAS to use for this request</param>
        /// <returns>The request ID</returns>
        public static ByteString Request(
            string serviceId,
            UInt160 callbackContract,
            string callbackMethod,
            ByteString payload,
            BigInteger gasLimit)
        {
            RequireNotPaused();

            // Validate inputs
            if (string.IsNullOrEmpty(serviceId)) throw new Exception("Invalid service ID");
            if (!callbackContract.IsValid) throw new Exception("Invalid callback contract");
            if (string.IsNullOrEmpty(callbackMethod)) throw new Exception("Invalid callback method");
            if (gasLimit < GetMinGas()) throw new Exception("Gas limit too low");

            // Verify service is registered
            var serviceContract = GetServiceContract(serviceId);
            if (serviceContract == UInt160.Zero) throw new Exception("Service not registered");

            // Get requester (the calling contract)
            var requester = Runtime.CallingScriptHash;

            // Check and reserve gas
            var balance = GetGasBalance(requester);
            if (balance < gasLimit) throw new Exception("Insufficient gas balance");

            var balanceKey = GetGasBalanceKey(requester);
            Storage.Put(Storage.CurrentContext, balanceKey, balance - gasLimit);

            // Generate request ID
            var requestId = GenerateRequestId(requester);

            // Store request
            var request = new RequestData
            {
                RequestId = requestId,
                ServiceId = serviceId,
                Requester = requester,
                CallbackContract = callbackContract,
                CallbackMethod = callbackMethod,
                Payload = payload,
                GasDeposit = gasLimit,
                Status = 0, // Pending
                CreatedAt = Runtime.Time
            };
            StoreRequest(requestId, request);

            // Emit event for Service Layer to process
            OnServiceRequest(requestId, serviceId, requester, callbackContract, callbackMethod, payload, gasLimit);

            return requestId;
        }

        /// <summary>
        /// Delivers a service response. Only callable by Service Layer.
        ///
        /// This method:
        /// 1. Validates the response
        /// 2. Updates request status
        /// 3. Calls the user's callback method
        /// 4. Refunds unused gas
        /// </summary>
        public static void Callback(
            ByteString requestId,
            bool success,
            ByteString result,
            string error,
            BigInteger gasUsed,
            ByteString signature)
        {
            RequireServiceLayer();
            RequireNotPaused();

            // Get and validate request
            var request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != 0) throw new Exception("Request already processed");

            // Verify signature (TEE attestation)
            // TODO: Implement signature verification against TEE public key

            // Update request status
            request.Status = success ? (byte)1 : (byte)2; // Processed or Failed
            request.ProcessedAt = Runtime.Time;
            StoreRequest(requestId, request);

            // Calculate gas refund
            var gasRefund = request.GasDeposit - gasUsed;
            if (gasRefund > 0)
            {
                var balanceKey = GetGasBalanceKey(request.Requester);
                var currentBalance = GetGasBalance(request.Requester);
                Storage.Put(Storage.CurrentContext, balanceKey, currentBalance + gasRefund);
            }

            // Call user's callback
            if (success)
            {
                Contract.Call(request.CallbackContract, request.CallbackMethod, CallFlags.All, new object[] { requestId, result });
            }
            else
            {
                // For failures, pass the error message
                Contract.Call(request.CallbackContract, request.CallbackMethod, CallFlags.All, new object[] { requestId, (ByteString)error });
            }

            // Emit response event
            OnServiceResponse(requestId, request.ServiceId, success, result, gasUsed);
        }

        /// <summary>
        /// Cancels a pending request. Only callable by the requester.
        /// </summary>
        public static void CancelRequest(ByteString requestId)
        {
            var request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != 0) throw new Exception("Request not pending");

            // Only requester can cancel
            if (Runtime.CallingScriptHash != request.Requester)
                throw new Exception("Only requester can cancel");

            // Update status
            request.Status = 3; // Cancelled
            request.ProcessedAt = Runtime.Time;
            StoreRequest(requestId, request);

            // Refund gas
            var balanceKey = GetGasBalanceKey(request.Requester);
            var currentBalance = GetGasBalance(request.Requester);
            Storage.Put(Storage.CurrentContext, balanceKey, currentBalance + request.GasDeposit);
        }

        /// <summary>
        /// Gets a request by ID.
        /// </summary>
        public static RequestData GetRequest(ByteString requestId)
        {
            var key = GetRequestKey(requestId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (RequestData)StdLib.Deserialize(stored);
        }

        // ==================== Helper Methods ====================

        private static void RequireAdmin()
        {
            if (!Runtime.CheckWitness(GetAdmin()))
                throw new Exception("Only admin");
        }

        private static void RequireServiceLayer()
        {
            var serviceLayer = GetServiceLayer();
            if (serviceLayer == UInt160.Zero)
                throw new Exception("Service layer not configured");
            if (!Runtime.CheckWitness(serviceLayer))
                throw new Exception("Only service layer");
        }

        private static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract paused");
        }

        private static byte[] GetServiceKey(string serviceId)
        {
            return Helper.Concat(new byte[] { PREFIX_SERVICE }, (ByteString)serviceId);
        }

        private static byte[] GetRequestKey(ByteString requestId)
        {
            return Helper.Concat(new byte[] { PREFIX_REQUEST }, requestId);
        }

        private static byte[] GetGasBalanceKey(UInt160 account)
        {
            return Helper.Concat(new byte[] { PREFIX_GAS_BALANCE }, (ByteString)account);
        }

        private static ByteString GenerateRequestId(UInt160 requester)
        {
            // Get and increment counter
            var counterKey = new byte[] { PREFIX_REQUEST_COUNTER };
            var counter = Storage.Get(Storage.CurrentContext, counterKey);
            BigInteger counterValue = counter != null ? (BigInteger)counter : 0;
            counterValue++;
            Storage.Put(Storage.CurrentContext, counterKey, counterValue);

            // Generate ID: hash(requester + counter + time)
            var data = Helper.Concat((ByteString)requester, (ByteString)counterValue);
            BigInteger timeValue = Runtime.Time;
            data = Helper.Concat(data, (ByteString)timeValue);
            return CryptoLib.Sha256(data);
        }

        private static void StoreRequest(ByteString requestId, RequestData request)
        {
            var key = GetRequestKey(requestId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(request));
        }

        // ==================== Update ====================

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    /// <summary>
    /// Request data structure for storage.
    /// </summary>
    public class RequestData
    {
        public ByteString RequestId;
        public string ServiceId;
        public UInt160 Requester;
        public UInt160 CallbackContract;
        public string CallbackMethod;
        public ByteString Payload;
        public BigInteger GasDeposit;
        public byte Status; // 0=Pending, 1=Processed, 2=Failed, 3=Cancelled
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
    }
}
