using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Oracle
{
    public partial class NeoOracleService
    {
        // ============================================================================
        // Request Handling (Called by Gateway)
        // ============================================================================

        /// <summary>
        /// Called by ServiceLayerGateway when a user contract requests Oracle service.
        /// </summary>
        public static void OnRequest(BigInteger requestId, UInt160 userContract, byte[] payload)
        {
            RequireGateway();

            // Parse payload
            OracleRequestPayload request = (OracleRequestPayload)StdLib.Deserialize((ByteString)payload);

            if (string.IsNullOrEmpty(request.Url))
                throw new Exception("URL required");

            // Store request
            OracleStoredRequest stored = new OracleStoredRequest
            {
                Url = request.Url,
                Method = request.Method ?? "GET",
                Headers = request.Headers ?? "",
                JsonPath = request.JsonPath ?? "",
                UserContract = userContract
            };

            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Put(requestId.ToByteArray(), StdLib.Serialize(stored));

            // Emit event for TEE
            OnOracleRequest(requestId, userContract, stored.Url, stored.Method, stored.Headers, stored.JsonPath);
        }

        /// <summary>
        /// Called by ServiceLayerGateway when TEE fulfills the Oracle request.
        /// </summary>
        public static void OnFulfill(BigInteger requestId, byte[] result)
        {
            RequireGateway();

            // Store result for future reference
            StorageMap resultMap = new StorageMap(Storage.CurrentContext, PREFIX_RESULT);
            resultMap.Put(requestId.ToByteArray(), result);

            // Clean up request
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Delete(requestId.ToByteArray());

            OnOracleFulfilled(requestId, result);
        }

        // ============================================================================
        // Query Functions
        // ============================================================================

        /// <summary>Get stored result for a request</summary>
        public static byte[] GetResult(BigInteger requestId)
        {
            StorageMap resultMap = new StorageMap(Storage.CurrentContext, PREFIX_RESULT);
            return (byte[])resultMap.Get(requestId.ToByteArray());
        }

        /// <summary>Get pending request details</summary>
        public static OracleStoredRequest GetRequest(BigInteger requestId)
        {
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            ByteString data = requestMap.Get(requestId.ToByteArray());
            if (data == null) return null;
            return (OracleStoredRequest)StdLib.Deserialize(data);
        }
    }
}
