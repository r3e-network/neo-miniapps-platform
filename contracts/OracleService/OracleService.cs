using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Oracle
{
    /// <summary>
    /// OracleService - HTTP data fetching oracle contract.
    ///
    /// Flow:
    /// 1. User Contract -> Gateway.Request("oracle", ...) with URL/method/headers
    /// 2. Service Layer (TEE) fetches data securely
    /// 3. Service Layer -> Gateway.Callback() -> User Contract callback
    ///
    /// This is a request-response pattern where each request gets a unique response.
    /// </summary>
    [DisplayName("OracleService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "HTTP Oracle Service - Secure external data fetching")]
    [ContractPermission("*", "*")]
    public class OracleService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_REQUEST = 0x10;
        private const byte PREFIX_ALLOWED_HOST = 0x20;
        private const byte PREFIX_PAUSED = 0x30;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when an oracle request is created.
        /// Service Layer monitors this to fetch the data.
        /// </summary>
        [DisplayName("OracleRequest")]
        public static event Action<ByteString, UInt160, string, string, ByteString, string> OnOracleRequest;
        // Parameters: requestId, requester, url, method, body, jsonPath

        /// <summary>
        /// Emitted when an oracle response is delivered.
        /// </summary>
        [DisplayName("OracleResponse")]
        public static event Action<ByteString, bool, ByteString, BigInteger> OnOracleResponse;
        // Parameters: requestId, success, data, statusCode

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

        // ==================== Host Allowlist ====================

        /// <summary>
        /// Adds a host to the allowlist. Only admin can call.
        /// </summary>
        public static void AddAllowedHost(string host)
        {
            RequireAdmin();
            var key = GetAllowedHostKey(host);
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        /// <summary>
        /// Removes a host from the allowlist.
        /// </summary>
        public static void RemoveAllowedHost(string host)
        {
            RequireAdmin();
            var key = GetAllowedHostKey(host);
            Storage.Delete(Storage.CurrentContext, key);
        }

        /// <summary>
        /// Checks if a host is allowed.
        /// </summary>
        public static bool IsHostAllowed(string host)
        {
            var key = GetAllowedHostKey(host);
            return Storage.Get(Storage.CurrentContext, key) != null;
        }

        // ==================== Oracle Request ====================

        /// <summary>
        /// Creates an oracle request. Called via Gateway.
        ///
        /// The payload should be serialized OracleRequestData.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            // Deserialize payload
            var requestData = (OracleRequestData)StdLib.Deserialize(payload);

            // Validate URL host is allowed (if allowlist is configured)
            // Note: Full URL validation happens in Service Layer

            // Store request
            var request = new OracleRequest
            {
                RequestId = requestId,
                Requester = requester,
                Url = requestData.Url,
                Method = requestData.Method,
                Headers = requestData.Headers,
                Body = requestData.Body,
                JsonPath = requestData.JsonPath,
                Status = 0, // Pending
                CreatedAt = Runtime.Time
            };
            StoreRequest(requestId, request);

            // Emit event for Service Layer
            OnOracleRequest(requestId, requester, requestData.Url, requestData.Method, requestData.Body, requestData.JsonPath);
        }

        /// <summary>
        /// Delivers oracle response. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString data, BigInteger statusCode, ByteString signature)
        {
            RequireGateway();

            var request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != 0) throw new Exception("Request already processed");

            // Update request
            request.Status = success ? (byte)1 : (byte)2;
            request.ProcessedAt = Runtime.Time;
            request.ResponseData = data;
            request.StatusCode = statusCode;
            StoreRequest(requestId, request);

            OnOracleResponse(requestId, success, data, statusCode);
        }

        /// <summary>
        /// Gets a request by ID.
        /// </summary>
        public static OracleRequest GetRequest(ByteString requestId)
        {
            var key = GetRequestKey(requestId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (OracleRequest)StdLib.Deserialize(stored);
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

        private static byte[] GetRequestKey(ByteString requestId)
        {
            return Helper.Concat(new byte[] { PREFIX_REQUEST }, requestId);
        }

        private static byte[] GetAllowedHostKey(string host)
        {
            return Helper.Concat(new byte[] { PREFIX_ALLOWED_HOST }, (ByteString)host);
        }

        private static void StoreRequest(ByteString requestId, OracleRequest request)
        {
            var key = GetRequestKey(requestId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(request));
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class OracleRequestData
    {
        public string Url;
        public string Method;
        public ByteString Headers;
        public ByteString Body;
        public string JsonPath;
    }

    public class OracleRequest
    {
        public ByteString RequestId;
        public UInt160 Requester;
        public string Url;
        public string Method;
        public ByteString Headers;
        public ByteString Body;
        public string JsonPath;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
        public ByteString ResponseData;
        public BigInteger StatusCode;
    }
}
