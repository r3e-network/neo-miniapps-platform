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
    /// ServiceLayerGateway - The main entry point for Service Layer on Neo N3.
    ///
    /// This contract is the GATE of all Service Layer services:
    /// - TEE master account registration and management
    /// - Service contract registration
    /// - Request routing from user contracts to service contracts
    /// - Callback routing from TEE to user contracts
    /// - Authenticity and authorization management
    ///
    /// Fee Management:
    /// - Fees are handled OFF-CHAIN via GasBank and Supabase
    /// - This contract does NOT manage user balances or collect fees
    /// - TEE verifies user balance off-chain before processing requests
    ///
    /// Flow:
    /// 1. User Contract → ServiceLayerGateway.requestService() → Service Contract (emit event)
    /// 2. TEE monitors events, verifies off-chain balance, processes
    /// 3. TEE → ServiceLayerGateway.fulfillRequest() → Service Contract → User Contract callback
    /// </summary>
    [DisplayName("ServiceLayerGateway")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Email", "dev@r3e.network")]
    [ManifestExtra("Description", "Service Layer Gateway - Main entry for all services")]
    [ManifestExtra("Version", "3.0.0")]
    [ContractPermission("*", "*")]
    public class ServiceLayerGateway : SmartContract
    {
        // ============================================================================
        // Storage Prefixes
        // ============================================================================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_PAUSED = 0x02;
        private const byte PREFIX_TEE_ACCOUNT = 0x10;          // TEE master accounts
        private const byte PREFIX_TEE_PUBKEY = 0x11;           // TEE public keys for verification
        private const byte PREFIX_SERVICE = 0x20;              // Registered service contracts
        private const byte PREFIX_REQUEST = 0x40;              // Service requests
        private const byte PREFIX_REQUEST_COUNT = 0x41;        // Request counter
        private const byte PREFIX_NONCE = 0x50;                // Used nonces (replay protection)

        // Request status
        public const byte STATUS_PENDING = 0;
        public const byte STATUS_PROCESSING = 1;
        public const byte STATUS_COMPLETED = 2;
        public const byte STATUS_FAILED = 3;

        // ============================================================================
        // Events
        // ============================================================================

        /// <summary>Emitted when a service request is created</summary>
        [DisplayName("ServiceRequest")]
        public static event Action<BigInteger, UInt160, UInt160, string, byte[]> OnServiceRequest;
        // requestId, userContract, caller, serviceType, payload

        /// <summary>Emitted when a request is fulfilled by TEE</summary>
        [DisplayName("RequestFulfilled")]
        public static event Action<BigInteger, byte[]> OnRequestFulfilled;
        // requestId, result

        /// <summary>Emitted when a request fails</summary>
        [DisplayName("RequestFailed")]
        public static event Action<BigInteger, string> OnRequestFailed;
        // requestId, reason

        /// <summary>Emitted when callback is executed to user contract</summary>
        [DisplayName("CallbackExecuted")]
        public static event Action<BigInteger, UInt160, string, bool> OnCallbackExecuted;
        // requestId, userContract, method, success

        /// <summary>Emitted when TEE account is registered</summary>
        [DisplayName("TEERegistered")]
        public static event Action<UInt160, ECPoint> OnTEERegistered;

        /// <summary>Emitted when service is registered</summary>
        [DisplayName("ServiceRegistered")]
        public static event Action<string, UInt160> OnServiceRegistered;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;

            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, tx.Sender);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest);
        }

        // ============================================================================
        // Admin Management
        // ============================================================================

        private static UInt160 GetAdmin() => (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });

        private static bool IsAdmin() => Runtime.CheckWitness(GetAdmin());

        private static void RequireAdmin()
        {
            if (!IsAdmin()) throw new Exception("Admin only");
        }

        public static UInt160 Admin() => GetAdmin();

        public static void TransferAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (newAdmin == null || !newAdmin.IsValid) throw new Exception("Invalid address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        // ============================================================================
        // Pause Control
        // ============================================================================

        private static bool IsPaused() => (BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }) == 1;

        private static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract paused");
        }

        public static bool Paused() => IsPaused();

        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }

        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ============================================================================
        // TEE Master Account Management
        // ============================================================================

        /// <summary>
        /// Register a TEE master account with its public key.
        /// Only TEE accounts can fulfill requests.
        /// </summary>
        public static void RegisterTEEAccount(UInt160 teeAccount, ECPoint teePubKey)
        {
            RequireAdmin();
            if (teeAccount == null || !teeAccount.IsValid) throw new Exception("Invalid TEE account");
            if (teePubKey == null) throw new Exception("Invalid public key");

            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Put(Storage.CurrentContext, accountKey, 1);
            Storage.Put(Storage.CurrentContext, pubKeyKey, teePubKey);

            OnTEERegistered(teeAccount, teePubKey);
        }

        /// <summary>Remove a TEE account</summary>
        public static void RemoveTEEAccount(UInt160 teeAccount)
        {
            RequireAdmin();
            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Delete(Storage.CurrentContext, accountKey);
            Storage.Delete(Storage.CurrentContext, pubKeyKey);
        }

        /// <summary>Check if an account is a registered TEE account</summary>
        public static bool IsTEEAccount(UInt160 account)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])account);
            return (BigInteger)Storage.Get(Storage.CurrentContext, key) == 1;
        }

        /// <summary>Get TEE public key for signature verification</summary>
        public static ECPoint GetTEEPublicKey(UInt160 teeAccount)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);
            return (ECPoint)Storage.Get(Storage.CurrentContext, key);
        }

        private static void RequireTEE()
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            if (!IsTEEAccount(tx.Sender)) throw new Exception("TEE account only");
        }

        // ============================================================================
        // Service Contract Registration
        // ============================================================================

        /// <summary>Register a service contract</summary>
        public static void RegisterService(string serviceType, UInt160 serviceContract)
        {
            RequireAdmin();
            if (string.IsNullOrEmpty(serviceType)) throw new Exception("Invalid service type");
            if (serviceContract == null || !serviceContract.IsValid) throw new Exception("Invalid contract");

            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceType.ToByteArray());
            Storage.Put(Storage.CurrentContext, key, serviceContract);

            OnServiceRegistered(serviceType, serviceContract);
        }

        /// <summary>Remove a service contract</summary>
        public static void RemoveService(string serviceType)
        {
            RequireAdmin();
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceType.ToByteArray());
            Storage.Delete(Storage.CurrentContext, key);
        }

        /// <summary>Get service contract address</summary>
        public static UInt160 GetServiceContract(string serviceType)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceType.ToByteArray());
            return (UInt160)Storage.Get(Storage.CurrentContext, key);
        }

        // ============================================================================
        // Service Request (Called by User Contracts)
        // ============================================================================

        /// <summary>
        /// Request a service. Called by user contracts.
        ///
        /// Fee verification is done OFF-CHAIN by TEE via GasBank/Supabase.
        /// This contract only routes requests and emits events.
        ///
        /// Flow: UserContract → ServiceLayerGateway.RequestService() → ServiceContract.OnRequest()
        ///
        /// The service contract will emit an event that TEE monitors.
        /// TEE verifies off-chain balance before processing.
        /// </summary>
        /// <param name="serviceType">Type of service (oracle, vrf, mixer, etc.)</param>
        /// <param name="payload">Service-specific request payload</param>
        /// <param name="callbackMethod">Method to call on user contract when fulfilled</param>
        /// <returns>Request ID</returns>
        public static BigInteger RequestService(string serviceType, byte[] payload, string callbackMethod)
        {
            RequireNotPaused();

            // Get caller (user contract)
            UInt160 userContract = Runtime.CallingScriptHash;

            // Get the actual user who initiated the transaction
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            UInt160 caller = tx.Sender;

            // Check service exists
            UInt160 serviceContract = GetServiceContract(serviceType);
            if (serviceContract == null) throw new Exception("Service not registered");

            // Create request (no fee charging - handled off-chain)
            BigInteger requestId = GetNextRequestId();

            RequestData request = new RequestData
            {
                Id = requestId,
                UserContract = userContract,
                Caller = caller,
                ServiceType = serviceType,
                ServiceContract = serviceContract,
                Payload = payload,
                CallbackMethod = callbackMethod ?? "",
                Status = STATUS_PENDING,
                CreatedAt = Runtime.Time
            };

            SaveRequest(requestId, request);

            // Call service contract to register the request
            // Service contract will emit specific event for TEE to monitor
            Contract.Call(serviceContract, "onRequest", CallFlags.All,
                new object[] { requestId, userContract, payload });

            OnServiceRequest(requestId, userContract, caller, serviceType, payload);

            return requestId;
        }

        // ============================================================================
        // Request Fulfillment (Called by TEE)
        // ============================================================================

        /// <summary>
        /// Fulfill a request. Called by TEE after processing.
        ///
        /// Fee deduction is done OFF-CHAIN before TEE calls this method.
        ///
        /// Flow: TEE → ServiceLayerGateway.FulfillRequest() → ServiceContract.OnFulfill() → UserContract.callback()
        /// </summary>
        /// <param name="requestId">Request ID to fulfill</param>
        /// <param name="result">Result data</param>
        /// <param name="nonce">Nonce for replay protection</param>
        /// <param name="signature">TEE signature over (requestId, result, nonce)</param>
        public static void FulfillRequest(BigInteger requestId, byte[] result, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            // Verify nonce not used
            VerifyAndMarkNonce(nonce);

            // Get request
            RequestData request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != STATUS_PENDING && request.Status != STATUS_PROCESSING)
                throw new Exception("Request already processed");

            // Verify TEE signature
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            ECPoint teePubKey = GetTEEPublicKey(tx.Sender);
            if (teePubKey == null) throw new Exception("TEE key not found");

            byte[] message = Helper.Concat(requestId.ToByteArray(), result);
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Update request status
            request.Status = STATUS_COMPLETED;
            request.Result = result;
            request.CompletedAt = Runtime.Time;
            SaveRequest(requestId, request);

            // Call service contract to finalize
            Contract.Call(request.ServiceContract, "onFulfill", CallFlags.All,
                new object[] { requestId, result });

            OnRequestFulfilled(requestId, result);

            // Execute callback to user contract
            if (!string.IsNullOrEmpty(request.CallbackMethod))
            {
                ExecuteCallback(requestId, request.UserContract, request.CallbackMethod, result, true, "");
            }
        }

        /// <summary>
        /// Mark a request as failed. Called by TEE.
        /// No refund needed - fees are managed off-chain.
        /// </summary>
        public static void FailRequest(BigInteger requestId, string reason, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            VerifyAndMarkNonce(nonce);

            RequestData request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != STATUS_PENDING && request.Status != STATUS_PROCESSING)
                throw new Exception("Request already processed");

            // Verify TEE signature
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            ECPoint teePubKey = GetTEEPublicKey(tx.Sender);

            byte[] message = Helper.Concat(requestId.ToByteArray(), reason.ToByteArray());
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Update request (no refund - handled off-chain)
            request.Status = STATUS_FAILED;
            request.Error = reason;
            request.CompletedAt = Runtime.Time;
            SaveRequest(requestId, request);

            OnRequestFailed(requestId, reason);

            // Execute failure callback
            if (!string.IsNullOrEmpty(request.CallbackMethod))
            {
                ExecuteCallback(requestId, request.UserContract, request.CallbackMethod, null, false, reason);
            }
        }

        /// <summary>Execute callback to user contract</summary>
        private static void ExecuteCallback(BigInteger requestId, UInt160 userContract, string method, byte[] result, bool success, string error)
        {
            bool callbackSuccess = false;
            try
            {
                Contract.Call(userContract, method, CallFlags.All,
                    new object[] { requestId, success, result, error });
                callbackSuccess = true;
            }
            catch
            {
                callbackSuccess = false;
            }

            OnCallbackExecuted(requestId, userContract, method, callbackSuccess);
        }

        // ============================================================================
        // Request Query
        // ============================================================================

        public static RequestData GetRequest(BigInteger requestId)
        {
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            ByteString data = requestMap.Get(requestId.ToByteArray());
            if (data == null) return null;
            return (RequestData)StdLib.Deserialize(data);
        }

        private static void SaveRequest(BigInteger requestId, RequestData request)
        {
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Put(requestId.ToByteArray(), StdLib.Serialize(request));
        }

        private static BigInteger GetNextRequestId()
        {
            byte[] key = new byte[] { PREFIX_REQUEST_COUNT };
            BigInteger id = (BigInteger)Storage.Get(Storage.CurrentContext, key);
            id += 1;
            Storage.Put(Storage.CurrentContext, key, id);
            return id;
        }

        /// <summary>Get total request count</summary>
        public static BigInteger GetRequestCount()
        {
            byte[] key = new byte[] { PREFIX_REQUEST_COUNT };
            return (BigInteger)Storage.Get(Storage.CurrentContext, key);
        }

        // ============================================================================
        // Nonce Management (Replay Protection)
        // ============================================================================

        private static void VerifyAndMarkNonce(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new Exception("Nonce already used");
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        /// <summary>Check if nonce was used</summary>
        public static bool IsNonceUsed(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            return Storage.Get(Storage.CurrentContext, key) != null;
        }
    }

    /// <summary>Service request data structure</summary>
    public class RequestData
    {
        public BigInteger Id;
        public UInt160 UserContract;      // The contract that made the request
        public UInt160 Caller;            // The account that initiated the transaction
        public string ServiceType;
        public UInt160 ServiceContract;
        public byte[] Payload;
        public string CallbackMethod;
        public byte Status;
        public ulong CreatedAt;
        public byte[] Result;
        public string Error;
        public ulong CompletedAt;
    }
}
