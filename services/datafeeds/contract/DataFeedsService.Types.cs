using Neo;
using System.Numerics;

namespace ServiceLayer.DataFeeds
{
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
