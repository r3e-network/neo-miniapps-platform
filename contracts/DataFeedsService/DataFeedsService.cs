using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.DataFeeds
{
    /// <summary>
    /// DataFeedsService - Price feed oracle contract.
    ///
    /// This contract operates in PUSH mode:
    /// 1. Admin configures price feeds (pairs, sources, heartbeat, deviation)
    /// 2. Service Layer (TEE) periodically fetches prices from configured sources
    /// 3. Service Layer pushes updates to this contract via Gateway
    /// 4. User contracts read latest prices directly from this contract
    ///
    /// Flow:
    /// - Service Layer -> Gateway -> DataFeedsService.UpdatePrice()
    /// - User Contract -> DataFeedsService.GetLatestPrice()
    /// </summary>
    [DisplayName("DataFeedsService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Price Feed Oracle Service - Push-based price updates")]
    [ContractPermission("*", "*")]
    public class DataFeedsService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_FEED_CONFIG = 0x10;
        private const byte PREFIX_FEED_DATA = 0x20;
        private const byte PREFIX_FEED_HISTORY = 0x30;
        private const byte PREFIX_SUBSCRIBER = 0x40;
        private const byte PREFIX_PAUSED = 0x50;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a price is updated.
        /// </summary>
        [DisplayName("PriceUpdated")]
        public static event Action<string, BigInteger, byte, BigInteger, BigInteger> OnPriceUpdated;
        // Parameters: feedId, price, decimals, roundId, timestamp

        /// <summary>
        /// Emitted when a new feed is created.
        /// </summary>
        [DisplayName("FeedCreated")]
        public static event Action<string, string, BigInteger, BigInteger> OnFeedCreated;
        // Parameters: feedId, pair, heartbeat, deviation

        /// <summary>
        /// Emitted when a feed is updated.
        /// </summary>
        [DisplayName("FeedConfigUpdated")]
        public static event Action<string> OnFeedConfigUpdated;

        /// <summary>
        /// Emitted when a contract subscribes to a feed.
        /// </summary>
        [DisplayName("Subscribed")]
        public static event Action<string, UInt160> OnSubscribed;

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

        public static UInt160 GetGateway()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            if (!gateway.IsValid) throw new Exception("Invalid gateway");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ==================== Feed Configuration ====================

        /// <summary>
        /// Creates a new price feed. Only admin can call.
        /// </summary>
        /// <param name="feedId">Unique feed identifier (e.g., "BTC/USD")</param>
        /// <param name="pair">Trading pair name</param>
        /// <param name="decimals">Price decimals (e.g., 8 for $12345.67890000)</param>
        /// <param name="heartbeat">Maximum seconds between updates</param>
        /// <param name="deviation">Minimum price change in basis points (100 = 1%)</param>
        public static void CreateFeed(string feedId, string pair, byte decimals, BigInteger heartbeat, BigInteger deviation)
        {
            RequireAdmin();
            if (string.IsNullOrEmpty(feedId)) throw new Exception("Invalid feed ID");
            if (string.IsNullOrEmpty(pair)) throw new Exception("Invalid pair");
            if (decimals > 18) throw new Exception("Invalid decimals");
            if (heartbeat <= 0) throw new Exception("Invalid heartbeat");
            if (deviation < 0 || deviation > 10000) throw new Exception("Invalid deviation");

            // Check feed doesn't exist
            var configKey = GetFeedConfigKey(feedId);
            if (Storage.Get(Storage.CurrentContext, configKey) != null)
                throw new Exception("Feed already exists");

            var config = new FeedConfig
            {
                FeedId = feedId,
                Pair = pair,
                Decimals = decimals,
                Heartbeat = heartbeat,
                Deviation = deviation,
                Active = true,
                CreatedAt = Runtime.Time
            };

            Storage.Put(Storage.CurrentContext, configKey, StdLib.Serialize(config));
            OnFeedCreated(feedId, pair, heartbeat, deviation);
        }

        /// <summary>
        /// Updates feed configuration. Only admin can call.
        /// </summary>
        public static void UpdateFeedConfig(string feedId, BigInteger heartbeat, BigInteger deviation, bool active)
        {
            RequireAdmin();
            var config = GetFeedConfig(feedId);
            if (config == null) throw new Exception("Feed not found");

            config.Heartbeat = heartbeat;
            config.Deviation = deviation;
            config.Active = active;

            var configKey = GetFeedConfigKey(feedId);
            Storage.Put(Storage.CurrentContext, configKey, StdLib.Serialize(config));
            OnFeedConfigUpdated(feedId);
        }

        /// <summary>
        /// Gets feed configuration.
        /// </summary>
        public static FeedConfig GetFeedConfig(string feedId)
        {
            var configKey = GetFeedConfigKey(feedId);
            var stored = Storage.Get(Storage.CurrentContext, configKey);
            if (stored == null) return null;
            return (FeedConfig)StdLib.Deserialize(stored);
        }

        // ==================== Price Updates (Push from Service Layer) ====================

        /// <summary>
        /// Updates the price for a feed. Only callable by Gateway (from Service Layer).
        ///
        /// The Service Layer (TEE) periodically:
        /// 1. Fetches prices from configured sources
        /// 2. Aggregates and validates the data
        /// 3. Signs the result
        /// 4. Calls Gateway.Callback() which routes here
        /// </summary>
        /// <param name="feedId">Feed identifier</param>
        /// <param name="price">New price (scaled by decimals)</param>
        /// <param name="timestamp">When the price was determined</param>
        /// <param name="signature">TEE signature over the data</param>
        public static void UpdatePrice(string feedId, BigInteger price, BigInteger timestamp, ByteString signature)
        {
            RequireGateway();
            RequireNotPaused();

            var config = GetFeedConfig(feedId);
            if (config == null) throw new Exception("Feed not found");
            if (!config.Active) throw new Exception("Feed not active");

            // Get current data
            var dataKey = GetFeedDataKey(feedId);
            var currentData = GetLatestPriceData(feedId);

            // Validate update
            if (currentData != null)
            {
                // Check timestamp is newer
                if (timestamp <= currentData.Timestamp)
                    throw new Exception("Stale price");

                // Check heartbeat or deviation threshold
                var timeDiff = timestamp - currentData.Timestamp;
                var priceDiff = price > currentData.Price
                    ? price - currentData.Price
                    : currentData.Price - price;
                var deviationBps = currentData.Price > 0
                    ? (priceDiff * 10000) / currentData.Price
                    : 10000;

                // Only update if heartbeat exceeded OR deviation threshold met
                if (timeDiff < config.Heartbeat * 1000 && deviationBps < config.Deviation)
                    throw new Exception("Update not needed");
            }

            // Calculate new round ID
            BigInteger roundId = currentData != null ? currentData.RoundId + 1 : 1;

            // Store new price data
            var newData = new PriceData
            {
                FeedId = feedId,
                Price = price,
                Decimals = config.Decimals,
                Timestamp = timestamp,
                RoundId = roundId,
                Signature = signature
            };

            Storage.Put(Storage.CurrentContext, dataKey, StdLib.Serialize(newData));

            // Store in history (optional, for historical queries)
            var historyKey = GetFeedHistoryKey(feedId, roundId);
            Storage.Put(Storage.CurrentContext, historyKey, StdLib.Serialize(newData));

            OnPriceUpdated(feedId, price, config.Decimals, roundId, timestamp);
        }

        // ==================== Price Reading (for User Contracts) ====================

        /// <summary>
        /// Gets the latest price for a feed.
        /// </summary>
        public static BigInteger GetLatestPrice(string feedId)
        {
            var data = GetLatestPriceData(feedId);
            if (data == null) throw new Exception("No price data");
            return data.Price;
        }

        /// <summary>
        /// Gets the latest price data including metadata.
        /// </summary>
        public static PriceData GetLatestPriceData(string feedId)
        {
            var dataKey = GetFeedDataKey(feedId);
            var stored = Storage.Get(Storage.CurrentContext, dataKey);
            if (stored == null) return null;
            return (PriceData)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets the latest round ID for a feed.
        /// </summary>
        public static BigInteger GetLatestRoundId(string feedId)
        {
            var data = GetLatestPriceData(feedId);
            return data != null ? data.RoundId : 0;
        }

        /// <summary>
        /// Gets price data for a specific round.
        /// </summary>
        public static PriceData GetRoundData(string feedId, BigInteger roundId)
        {
            var historyKey = GetFeedHistoryKey(feedId, roundId);
            var stored = Storage.Get(Storage.CurrentContext, historyKey);
            if (stored == null) return null;
            return (PriceData)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets the decimals for a feed.
        /// </summary>
        public static byte GetDecimals(string feedId)
        {
            var config = GetFeedConfig(feedId);
            if (config == null) throw new Exception("Feed not found");
            return config.Decimals;
        }

        // ==================== Subscription (Optional) ====================

        /// <summary>
        /// Subscribes a contract to receive price updates.
        /// The contract must implement OnPriceUpdate(feedId, price, timestamp).
        /// </summary>
        public static void Subscribe(string feedId, UInt160 subscriber)
        {
            var config = GetFeedConfig(feedId);
            if (config == null) throw new Exception("Feed not found");
            if (!subscriber.IsValid) throw new Exception("Invalid subscriber");

            var key = GetSubscriberKey(feedId, subscriber);
            Storage.Put(Storage.CurrentContext, key, 1);
            OnSubscribed(feedId, subscriber);
        }

        /// <summary>
        /// Unsubscribes a contract from price updates.
        /// </summary>
        public static void Unsubscribe(string feedId, UInt160 subscriber)
        {
            var key = GetSubscriberKey(feedId, subscriber);
            Storage.Delete(Storage.CurrentContext, key);
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

        private static byte[] GetFeedConfigKey(string feedId)
        {
            return Helper.Concat(new byte[] { PREFIX_FEED_CONFIG }, (ByteString)feedId);
        }

        private static byte[] GetFeedDataKey(string feedId)
        {
            return Helper.Concat(new byte[] { PREFIX_FEED_DATA }, (ByteString)feedId);
        }

        private static byte[] GetFeedHistoryKey(string feedId, BigInteger roundId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_FEED_HISTORY }, (ByteString)feedId);
            return Helper.Concat(key, (ByteString)roundId);
        }

        private static byte[] GetSubscriberKey(string feedId, UInt160 subscriber)
        {
            var key = Helper.Concat(new byte[] { PREFIX_SUBSCRIBER }, (ByteString)feedId);
            return Helper.Concat(key, (ByteString)subscriber);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class FeedConfig
    {
        public string FeedId;
        public string Pair;
        public byte Decimals;
        public BigInteger Heartbeat;  // Max seconds between updates
        public BigInteger Deviation;  // Min change in basis points
        public bool Active;
        public BigInteger CreatedAt;
    }

    public class PriceData
    {
        public string FeedId;
        public BigInteger Price;
        public byte Decimals;
        public BigInteger Timestamp;
        public BigInteger RoundId;
        public ByteString Signature;
    }
}
