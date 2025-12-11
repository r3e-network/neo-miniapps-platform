using Neo;
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
    }
}
