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
    public partial class DataFeedsService : SmartContract
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
    }
}
