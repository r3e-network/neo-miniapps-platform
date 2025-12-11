using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.DataFeeds
{
    public partial class NeoFeedsService
    {
        // ============================================================================
        // Price Queries (User Contracts Read)
        // ============================================================================

        /// <summary>
        /// Get the latest price for a feed.
        /// User contracts call this to read prices.
        /// </summary>
        public static PriceData GetLatestPrice(string feedId)
        {
            return GetPriceData(feedId);
        }

        /// <summary>
        /// Get price with staleness check.
        /// Reverts if price is too old.
        /// </summary>
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
}
