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
    /// DataFeedsService - Push-based Price Feed Oracle
    ///
    /// This contract implements Pattern 2: Push / Auto-Update
    /// - TEE periodically fetches prices from multiple sources
    /// - TEE aggregates and signs the price data
    /// - TEE pushes updates to this contract
    /// - User contracts read prices directly (no callback needed)
    ///
    /// Flow:
    /// 1. TEE fetches prices from Binance, Coinbase, Kraken, etc.
    /// 2. TEE calculates weighted median price
    /// 3. TEE signs the aggregated price
    /// 4. TEE calls UpdatePrice() to push to contract
    /// 5. User contracts call GetLatestPrice() to read
    ///
    /// Security:
    /// - Only registered TEE accounts can update prices
    /// - All updates are signed and verified
    /// - Staleness check prevents using outdated prices
    /// </summary>
    [DisplayName("DataFeedsService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Push-based Price Feed Oracle Service")]
    [ManifestExtra("Version", "1.0.0")]
    [ContractPermission("*", "*")]
    public class DataFeedsService : SmartContract
    {
        // ============================================================================
        // Storage Prefixes
        // ============================================================================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_PAUSED = 0x02;
        private const byte PREFIX_TEE_ACCOUNT = 0x10;
        private const byte PREFIX_TEE_PUBKEY = 0x11;
        private const byte PREFIX_PRICE = 0x20;
        private const byte PREFIX_FEED_CONFIG = 0x30;
        private const byte PREFIX_NONCE = 0x40;

        // ============================================================================
        // Constants
        // ============================================================================

        // Maximum staleness for price data (1 hour in milliseconds)
        public static readonly ulong MAX_STALENESS = 60 * 60 * 1000;

        // Minimum update interval (10 seconds in milliseconds)
        public static readonly ulong MIN_UPDATE_INTERVAL = 10 * 1000;

        // ============================================================================
        // Events
        // ============================================================================

        /// <summary>Price updated for a feed</summary>
        [DisplayName("PriceUpdated")]
        public static event Action<string, BigInteger, BigInteger, ulong> OnPriceUpdated;
        // feedId, price, decimals, timestamp

        /// <summary>New price feed registered</summary>
        [DisplayName("FeedRegistered")]
        public static event Action<string, string, BigInteger> OnFeedRegistered;
        // feedId, description, decimals

        /// <summary>TEE account registered</summary>
        [DisplayName("TEERegistered")]
        public static event Action<UInt160, ECPoint> OnTEERegistered;

        /// <summary>Feed deactivated</summary>
        [DisplayName("FeedDeactivated")]
        public static event Action<string> OnFeedDeactivated;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, tx.Sender);

            // Register default price feeds
            RegisterFeedInternal("BTC/USD", "Bitcoin to US Dollar", 8);
            RegisterFeedInternal("ETH/USD", "Ethereum to US Dollar", 8);
            RegisterFeedInternal("NEO/USD", "Neo to US Dollar", 8);
            RegisterFeedInternal("GAS/USD", "Gas to US Dollar", 8);
            RegisterFeedInternal("NEO/GAS", "Neo to Gas", 8);
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
        private static void RequireAdmin() { if (!IsAdmin()) throw new Exception("Admin only"); }

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
        private static void RequireNotPaused() { if (IsPaused()) throw new Exception("Contract paused"); }
        public static bool Paused() => IsPaused();
        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ============================================================================
        // TEE Account Management
        // ============================================================================

        /// <summary>Register a TEE account that can push price updates</summary>
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

        /// <summary>Check if account is registered TEE</summary>
        public static bool IsTEEAccount(UInt160 account)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])account);
            return (BigInteger)Storage.Get(Storage.CurrentContext, key) == 1;
        }

        /// <summary>Get TEE public key</summary>
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
        // Feed Configuration
        // ============================================================================

        /// <summary>Register a new price feed</summary>
        public static void RegisterFeed(string feedId, string description, BigInteger decimals)
        {
            RequireAdmin();
            RegisterFeedInternal(feedId, description, decimals);
        }

        private static void RegisterFeedInternal(string feedId, string description, BigInteger decimals)
        {
            if (string.IsNullOrEmpty(feedId)) throw new Exception("Invalid feed ID");
            if (decimals < 0 || decimals > 18) throw new Exception("Invalid decimals");

            byte[] key = Helper.Concat(new byte[] { PREFIX_FEED_CONFIG }, feedId.ToByteArray());

            FeedConfig config = new FeedConfig
            {
                FeedId = feedId,
                Description = description,
                Decimals = decimals,
                Active = true,
                CreatedAt = Runtime.Time
            };

            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(config));
            OnFeedRegistered(feedId, description, decimals);
        }

        /// <summary>Deactivate a price feed</summary>
        public static void DeactivateFeed(string feedId)
        {
            RequireAdmin();
            FeedConfig config = GetFeedConfig(feedId);
            if (config == null) throw new Exception("Feed not found");

            config.Active = false;
            byte[] key = Helper.Concat(new byte[] { PREFIX_FEED_CONFIG }, feedId.ToByteArray());
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(config));

            OnFeedDeactivated(feedId);
        }

        /// <summary>Get feed configuration</summary>
        public static FeedConfig GetFeedConfig(string feedId)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_FEED_CONFIG }, feedId.ToByteArray());
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (FeedConfig)StdLib.Deserialize((ByteString)data);
        }

        // ============================================================================
        // Price Updates (TEE Push)
        // ============================================================================

        /// <summary>
        /// Update price for a feed. Called by TEE.
        ///
        /// TEE aggregates prices from multiple sources:
        /// 1. Fetch from Binance, Coinbase, Kraken, etc.
        /// 2. Calculate weighted median
        /// 3. Sign the aggregated price
        /// 4. Call this method to push on-chain
        /// </summary>
        /// <param name="feedId">Price feed identifier (e.g., "BTC/USD")</param>
        /// <param name="price">Aggregated price (scaled by decimals)</param>
        /// <param name="timestamp">Timestamp when price was fetched</param>
        /// <param name="nonce">Nonce for replay protection</param>
        /// <param name="signature">TEE signature over (feedId, price, timestamp, nonce)</param>
        public static void UpdatePrice(string feedId, BigInteger price, ulong timestamp, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            // Validate feed exists and is active
            FeedConfig config = GetFeedConfig(feedId);
            if (config == null) throw new Exception("Feed not found");
            if (!config.Active) throw new Exception("Feed not active");

            // Validate price
            if (price <= 0) throw new Exception("Invalid price");

            // Validate timestamp (not too old, not in future)
            if (timestamp > Runtime.Time) throw new Exception("Future timestamp");
            if (Runtime.Time - timestamp > MAX_STALENESS) throw new Exception("Stale price");

            // Check update interval
            PriceData currentPrice = GetPriceData(feedId);
            if (currentPrice != null && timestamp - currentPrice.Timestamp < MIN_UPDATE_INTERVAL)
                throw new Exception("Update too frequent");

            // Verify nonce
            VerifyAndMarkNonce(nonce);

            // Verify TEE signature
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            ECPoint teePubKey = GetTEEPublicKey(tx.Sender);

            byte[] message = Helper.Concat(feedId.ToByteArray(), price.ToByteArray());
            message = Helper.Concat(message, ((BigInteger)timestamp).ToByteArray());
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Store price
            PriceData priceData = new PriceData
            {
                FeedId = feedId,
                Price = price,
                Decimals = config.Decimals,
                Timestamp = timestamp,
                UpdatedBy = tx.Sender
            };

            byte[] priceKey = Helper.Concat(new byte[] { PREFIX_PRICE }, feedId.ToByteArray());
            Storage.Put(Storage.CurrentContext, priceKey, StdLib.Serialize(priceData));

            OnPriceUpdated(feedId, price, config.Decimals, timestamp);
        }

        /// <summary>
        /// Batch update multiple prices in one transaction.
        /// More gas efficient for updating multiple feeds.
        /// </summary>
        public static void UpdatePrices(string[] feedIds, BigInteger[] prices, ulong[] timestamps, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            if (feedIds.Length != prices.Length || feedIds.Length != timestamps.Length)
                throw new Exception("Array length mismatch");
            if (feedIds.Length == 0 || feedIds.Length > 10)
                throw new Exception("Invalid batch size");

            // Verify nonce
            VerifyAndMarkNonce(nonce);

            // Build message for signature verification
            byte[] message = new byte[0];
            for (int i = 0; i < feedIds.Length; i++)
            {
                message = Helper.Concat(message, feedIds[i].ToByteArray());
                message = Helper.Concat(message, prices[i].ToByteArray());
                message = Helper.Concat(message, ((BigInteger)timestamps[i]).ToByteArray());
            }
            message = Helper.Concat(message, nonce.ToByteArray());

            // Verify TEE signature
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            ECPoint teePubKey = GetTEEPublicKey(tx.Sender);

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Update each price
            for (int i = 0; i < feedIds.Length; i++)
            {
                string feedId = feedIds[i];
                BigInteger price = prices[i];
                ulong timestamp = timestamps[i];

                FeedConfig config = GetFeedConfig(feedId);
                if (config == null || !config.Active) continue;
                if (price <= 0) continue;
                if (timestamp > Runtime.Time || Runtime.Time - timestamp > MAX_STALENESS) continue;

                PriceData priceData = new PriceData
                {
                    FeedId = feedId,
                    Price = price,
                    Decimals = config.Decimals,
                    Timestamp = timestamp,
                    UpdatedBy = tx.Sender
                };

                byte[] priceKey = Helper.Concat(new byte[] { PREFIX_PRICE }, feedId.ToByteArray());
                Storage.Put(Storage.CurrentContext, priceKey, StdLib.Serialize(priceData));

                OnPriceUpdated(feedId, price, config.Decimals, timestamp);
            }
        }

        // ============================================================================
        // Price Queries (User Contracts Read)
        // ============================================================================

        /// <summary>
        /// Get the latest price for a feed.
        /// User contracts call this to read prices.
        /// </summary>
        /// <param name="feedId">Price feed identifier</param>
        /// <returns>Price data including price, decimals, and timestamp</returns>
        public static PriceData GetLatestPrice(string feedId)
        {
            return GetPriceData(feedId);
        }

        /// <summary>
        /// Get price with staleness check.
        /// Reverts if price is too old.
        /// </summary>
        /// <param name="feedId">Price feed identifier</param>
        /// <param name="maxAge">Maximum age in milliseconds</param>
        /// <returns>Price data</returns>
        public static PriceData GetLatestPriceWithCheck(string feedId, ulong maxAge)
        {
            PriceData data = GetPriceData(feedId);
            if (data == null) throw new Exception("Price not available");
            if (Runtime.Time - data.Timestamp > maxAge) throw new Exception("Price too stale");
            return data;
        }

        /// <summary>Get raw price value (for simple integrations)</summary>
        public static BigInteger GetPrice(string feedId)
        {
            PriceData data = GetPriceData(feedId);
            if (data == null) return 0;
            return data.Price;
        }

        /// <summary>Get price timestamp</summary>
        public static ulong GetPriceTimestamp(string feedId)
        {
            PriceData data = GetPriceData(feedId);
            if (data == null) return 0;
            return data.Timestamp;
        }

        /// <summary>Check if price is fresh (within max staleness)</summary>
        public static bool IsPriceFresh(string feedId)
        {
            PriceData data = GetPriceData(feedId);
            if (data == null) return false;
            return Runtime.Time - data.Timestamp <= MAX_STALENESS;
        }

        private static PriceData GetPriceData(string feedId)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_PRICE }, feedId.ToByteArray());
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (PriceData)StdLib.Deserialize((ByteString)data);
        }

        // ============================================================================
        // Nonce Management
        // ============================================================================

        private static void VerifyAndMarkNonce(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new Exception("Nonce already used");
            Storage.Put(Storage.CurrentContext, key, 1);
        }
    }

    // ============================================================================
    // Data Structures
    // ============================================================================

    /// <summary>Price feed configuration</summary>
    public class FeedConfig
    {
        public string FeedId;
        public string Description;
        public BigInteger Decimals;
        public bool Active;
        public ulong CreatedAt;
    }

    /// <summary>Price data stored on-chain</summary>
    public class PriceData
    {
        public string FeedId;
        public BigInteger Price;        // Price scaled by decimals
        public BigInteger Decimals;     // Number of decimal places
        public ulong Timestamp;         // When price was fetched
        public UInt160 UpdatedBy;       // TEE account that updated
    }
}
