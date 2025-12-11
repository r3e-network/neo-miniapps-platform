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
    }
}
