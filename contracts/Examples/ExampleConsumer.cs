using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Examples
{
    /// <summary>
    /// ExampleConsumer - Demonstrates how to use Service Layer services.
    ///
    /// This contract shows how user contracts interact with the Service Layer:
    /// 1. Deposit GAS to the Gateway for service fees
    /// 2. Request services (Oracle, VRF, etc.) through the Gateway
    /// 3. Receive callbacks with results
    ///
    /// All requests go through the Gateway, which:
    /// - Validates the request
    /// - Charges gas fees
    /// - Routes to the appropriate service
    /// - Delivers callbacks
    /// </summary>
    [DisplayName("ExampleConsumer")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Example contract demonstrating Service Layer usage")]
    [ContractPermission("*", "*")]
    public class ExampleConsumer : SmartContract
    {
        // Storage prefixes
        private const byte PREFIX_GATEWAY = 0x01;
        private const byte PREFIX_ORACLE_RESULT = 0x10;
        private const byte PREFIX_VRF_RESULT = 0x20;
        private const byte PREFIX_PRICE = 0x30;

        // ==================== Events ====================

        [DisplayName("OracleResultReceived")]
        public static event Action<ByteString, ByteString> OnOracleResultReceived;

        [DisplayName("VRFResultReceived")]
        public static event Action<ByteString, ByteString> OnVRFResultReceived;

        [DisplayName("PriceReceived")]
        public static event Action<string, BigInteger> OnPriceReceived;

        // ==================== Configuration ====================

        /// <summary>
        /// Sets the Gateway contract address. Must be called after deployment.
        /// </summary>
        public static void SetGateway(UInt160 gateway)
        {
            // In production, add access control
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static UInt160 GetGateway()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        // ==================== Oracle Example ====================

        /// <summary>
        /// Requests data from an external API via the Oracle service.
        ///
        /// Example: Get BTC price from CoinGecko
        /// RequestOracleData("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd", "GET", "$.bitcoin.usd")
        /// </summary>
        public static ByteString RequestOracleData(string url, string method, string jsonPath)
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");

            // Build the payload for Oracle service
            var payload = StdLib.Serialize(new OraclePayload
            {
                Url = url,
                Method = method,
                JsonPath = jsonPath
            });

            // Request through Gateway
            // Parameters: serviceId, callbackContract, callbackMethod, payload, gasLimit
            var requestId = (ByteString)Contract.Call(gateway, "request", CallFlags.All, new object[]
            {
                "oracle",                           // Service ID
                Runtime.ExecutingScriptHash,        // Callback to this contract
                "onOracleCallback",                 // Callback method
                payload,                            // Request payload
                5_00000000                          // Gas limit (5 GAS)
            });

            return requestId;
        }

        /// <summary>
        /// Callback method called by Gateway when Oracle result is ready.
        /// </summary>
        public static void OnOracleCallback(ByteString requestId, ByteString result)
        {
            // Verify caller is the Gateway
            var gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can call callback");

            // Store the result
            var key = Helper.Concat(new byte[] { PREFIX_ORACLE_RESULT }, requestId);
            Storage.Put(Storage.CurrentContext, key, result);

            OnOracleResultReceived(requestId, result);
        }

        /// <summary>
        /// Gets a stored Oracle result.
        /// </summary>
        public static ByteString GetOracleResult(ByteString requestId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_ORACLE_RESULT }, requestId);
            return Storage.Get(Storage.CurrentContext, key);
        }

        // ==================== VRF Example ====================

        /// <summary>
        /// Requests verifiable random numbers via the VRF service.
        ///
        /// Example: Get 3 random numbers for a lottery
        /// RequestRandomness(seed, 3)
        /// </summary>
        public static ByteString RequestRandomness(ByteString seed, BigInteger numWords)
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");

            // Build the payload for VRF service
            var payload = StdLib.Serialize(new VRFPayload
            {
                Seed = seed,
                NumWords = numWords
            });

            // Request through Gateway
            var requestId = (ByteString)Contract.Call(gateway, "request", CallFlags.All, new object[]
            {
                "vrf",
                Runtime.ExecutingScriptHash,
                "onVRFCallback",
                payload,
                3_00000000  // 3 GAS
            });

            return requestId;
        }

        /// <summary>
        /// Callback method called by Gateway when VRF result is ready.
        /// </summary>
        public static void OnVRFCallback(ByteString requestId, ByteString result)
        {
            var gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can call callback");

            // Store the result
            var key = Helper.Concat(new byte[] { PREFIX_VRF_RESULT }, requestId);
            Storage.Put(Storage.CurrentContext, key, result);

            OnVRFResultReceived(requestId, result);
        }

        /// <summary>
        /// Gets a stored VRF result.
        /// </summary>
        public static ByteString GetVRFResult(ByteString requestId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_VRF_RESULT }, requestId);
            return Storage.Get(Storage.CurrentContext, key);
        }

        // ==================== DataFeeds Example ====================

        /// <summary>
        /// Reads the latest price from DataFeeds contract.
        /// DataFeeds uses push model - prices are updated by Service Layer.
        /// User contracts just read the latest value.
        /// </summary>
        public static BigInteger GetLatestPrice(UInt160 dataFeedsContract, string feedId)
        {
            // Direct read from DataFeeds contract (no Gateway needed for reads)
            var price = (BigInteger)Contract.Call(dataFeedsContract, "getLatestPrice", CallFlags.ReadOnly, new object[]
            {
                feedId
            });

            // Store locally for reference
            var key = Helper.Concat(new byte[] { PREFIX_PRICE }, (ByteString)feedId);
            Storage.Put(Storage.CurrentContext, key, price);

            OnPriceReceived(feedId, price);
            return price;
        }

        /// <summary>
        /// Gets full price data including timestamp and round.
        /// </summary>
        public static PriceInfo GetPriceData(UInt160 dataFeedsContract, string feedId)
        {
            var result = Contract.Call(dataFeedsContract, "getLatestPriceData", CallFlags.ReadOnly, new object[]
            {
                feedId
            });

            // Parse the result (would need proper deserialization)
            return new PriceInfo
            {
                FeedId = feedId,
                Price = 0,
                Decimals = 8,
                Timestamp = Runtime.Time,
                RoundId = 0
            };
        }

        // ==================== Automation Example ====================

        /// <summary>
        /// Creates an automation trigger to call this contract periodically.
        ///
        /// Example: Update price cache every hour
        /// CreatePeriodicTrigger("updatePriceCache", 3600, 10_00000000)
        /// </summary>
        public static ByteString CreatePeriodicTrigger(string method, BigInteger intervalSeconds, BigInteger gasLimit)
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");

            // Build the payload for Automation service
            var payload = StdLib.Serialize(new AutomationPayload
            {
                TriggerType = 1,  // Interval
                Target = Runtime.ExecutingScriptHash,
                Method = method,
                Schedule = (ByteString)intervalSeconds,
                GasLimit = gasLimit,
                MaxExecutions = 0  // Unlimited
            });

            // Request through Gateway
            var requestId = (ByteString)Contract.Call(gateway, "request", CallFlags.All, new object[]
            {
                "automation",
                Runtime.ExecutingScriptHash,
                "onAutomationCallback",
                payload,
                1_00000000  // 1 GAS for registration
            });

            return requestId;
        }

        /// <summary>
        /// Method that can be called by automation triggers.
        /// </summary>
        public static void UpdatePriceCache(ByteString triggerId, ByteString args)
        {
            // This method is called by the Automation service via Gateway
            // Verify caller is Gateway
            var gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can call");

            // Perform the automated task
            // e.g., fetch latest prices, update state, etc.
        }

        /// <summary>
        /// Callback for automation registration.
        /// </summary>
        public static void OnAutomationCallback(ByteString requestId, ByteString result)
        {
            var gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can call callback");

            // Automation trigger registered successfully
            // Result contains the trigger ID
        }

        // ==================== Gas Management ====================

        /// <summary>
        /// Deposits GAS to the Gateway for service fees.
        /// Call this before making service requests.
        /// </summary>
        public static void DepositGas(BigInteger amount)
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");

            // Transfer GAS to Gateway
            // The Gateway's OnNEP17Payment will credit this contract's balance
            GAS.Transfer(Runtime.ExecutingScriptHash, gateway, amount, null);
        }

        /// <summary>
        /// Checks this contract's GAS balance in the Gateway.
        /// </summary>
        public static BigInteger GetGasBalance()
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) return 0;

            return (BigInteger)Contract.Call(gateway, "getGasBalance", CallFlags.ReadOnly, new object[]
            {
                Runtime.ExecutingScriptHash
            });
        }
    }

    // ==================== Payload Structures ====================

    public class OraclePayload
    {
        public string Url;
        public string Method;
        public string JsonPath;
    }

    public class VRFPayload
    {
        public ByteString Seed;
        public BigInteger NumWords;
    }

    public class AutomationPayload
    {
        public byte TriggerType;
        public UInt160 Target;
        public string Method;
        public ByteString Schedule;
        public BigInteger GasLimit;
        public BigInteger MaxExecutions;
    }

    public class PriceInfo
    {
        public string FeedId;
        public BigInteger Price;
        public byte Decimals;
        public BigInteger Timestamp;
        public BigInteger RoundId;
    }
}
