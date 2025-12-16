using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Examples
{
    /// <summary>
    /// ExampleConsumer - Example user contract demonstrating Service Layer integration.
    ///
    /// This contract shows how to:
    /// 1. Request Oracle data
    /// 2. Handle callbacks from Service Layer
    ///
    /// Flow:
    /// 1. User calls ExampleConsumer.requestPrice()
    /// 2. ExampleConsumer calls Gateway.RequestService()
    /// 3. TEE processes request off-chain
    /// 4. Gateway calls ExampleConsumer.onServiceCallback()
    /// 5. ExampleConsumer stores/uses the result
    /// </summary>
    [DisplayName("ExampleConsumer")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Example Service Layer Consumer Contract")]
    [ManifestExtra("Version", "1.0.0")]
    [ContractPermission("*", "*")]
    public class ExampleConsumer : SmartContract
    {
        // Storage prefixes
        private const byte PREFIX_OWNER = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_PRICE = 0x10;
        private const byte PREFIX_PENDING = 0x30;

        // Events
        [DisplayName("PriceUpdated")]
        public static event Action<string, BigInteger, ulong> OnPriceUpdated;

        [DisplayName("RequestFailed")]
        public static event Action<BigInteger, string> OnRequestFailed;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_OWNER }, tx.Sender);
        }

        // ============================================================================
        // Configuration
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            RequireOwner();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static UInt160 GetGateway()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
        }

        private static void RequireOwner()
        {
            UInt160 owner = (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_OWNER });
            if (!Runtime.CheckWitness(owner)) throw new Exception("Owner only");
        }

        // ============================================================================
        // Oracle: Request Price Data
        // ============================================================================

        /// <summary>
        /// Request price data from an external API.
        /// Example: requestPrice("BTC/USD", "https://api.example.com/price", "data.price")
        /// </summary>
        public static BigInteger RequestPrice(string pair, string url, string jsonPath)
        {
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");

            // Build Oracle payload
            OraclePayload payload = new OraclePayload
            {
                Url = url,
                Method = "GET",
                JsonPath = jsonPath
            };

            byte[] payloadBytes = (byte[])StdLib.Serialize(payload);

            // Call Gateway to request Oracle service
            BigInteger requestId = (BigInteger)Contract.Call(gateway, "requestService", CallFlags.All,
                new object[] { "oracle", payloadBytes, "onServiceCallback" });

            // Store pending request info
            PendingRequest pending = new PendingRequest
            {
                RequestId = requestId,
                ServiceType = "oracle",
                Pair = pair
            };

            StorageMap pendingMap = new StorageMap(Storage.CurrentContext, PREFIX_PENDING);
            pendingMap.Put(requestId.ToByteArray(), StdLib.Serialize(pending));

            return requestId;
        }

        // ============================================================================
        // Callback Handler (Called by Gateway)
        // ============================================================================

        /// <summary>
        /// Callback from Service Layer Gateway.
        /// This method is called when the TEE fulfills or fails a request.
        /// </summary>
        /// <param name="requestId">The request ID</param>
        /// <param name="success">Whether the request succeeded</param>
        /// <param name="result">Result data (if success)</param>
        /// <param name="error">Error message (if failed)</param>
        public static void OnServiceCallback(BigInteger requestId, bool success, byte[] result, string error)
        {
            // Verify caller is the Gateway
            UInt160 gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can callback");

            // Get pending request
            StorageMap pendingMap = new StorageMap(Storage.CurrentContext, PREFIX_PENDING);
            ByteString pendingData = pendingMap.Get(requestId.ToByteArray());
            if (pendingData == null) throw new Exception("Unknown request");

            PendingRequest pending = (PendingRequest)StdLib.Deserialize((ByteString)pendingData);

            if (!success)
            {
                OnRequestFailed(requestId, error);
                pendingMap.Delete(requestId.ToByteArray());
                return;
            }

            // Handle based on service type
            if (pending.ServiceType == "oracle")
            {
                HandleOracleResult(pending, result);
            }

            // Clean up
            pendingMap.Delete(requestId.ToByteArray());
        }

        private static void HandleOracleResult(PendingRequest pending, byte[] result)
        {
            // Parse price from result (assuming it's a BigInteger)
            BigInteger price = new BigInteger(result);

            // Store price
            StorageMap priceMap = new StorageMap(Storage.CurrentContext, PREFIX_PRICE);
            PriceData priceData = new PriceData
            {
                Price = price,
                Timestamp = Runtime.Time
            };
            priceMap.Put(pending.Pair, StdLib.Serialize(priceData));

            OnPriceUpdated(pending.Pair, price, Runtime.Time);
        }

        // ============================================================================
        // Query Functions
        // ============================================================================

        /// <summary>Get stored price for a pair</summary>
        public static PriceData GetPrice(string pair)
        {
            StorageMap priceMap = new StorageMap(Storage.CurrentContext, PREFIX_PRICE);
            ByteString data = priceMap.Get(pair);
            if (data == null) return null;
            return (PriceData)StdLib.Deserialize((ByteString)data);
        }
    }

    // ============================================================================
    // Data Structures
    // ============================================================================

    public class OraclePayload
    {
        public string Url;
        public string Method;
        public string Headers;
        public string Body;
        public string JsonPath;
    }

    public class PendingRequest
    {
        public BigInteger RequestId;
        public string ServiceType;
        public string Pair;  // For oracle price requests
    }

    public class PriceData
    {
        public BigInteger Price;
        public ulong Timestamp;
    }
}
