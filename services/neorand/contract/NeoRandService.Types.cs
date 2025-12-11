using Neo;
using System.Numerics;

namespace ServiceLayer.VRF
{
    /// <summary>VRF request payload from user contract</summary>
    public class VRFRequestPayload
    {
        public byte[] Seed;         // User-provided seed
        public BigInteger NumWords; // Number of random words (1-10)
    }

    /// <summary>Stored VRF request</summary>
    public class VRFStoredRequest
    {
        public byte[] Seed;
        public BigInteger NumWords;
        public UInt160 UserContract;
    }

    /// <summary>VRF result from TEE</summary>
    public class VRFResultPayload
    {
        public byte[] RandomWords;
        public byte[] Proof;
    }
}
