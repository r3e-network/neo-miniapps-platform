using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System.Numerics;

namespace ServiceLayer.VRF
{
    public partial class NeoRandService
    {
        // ============================================================================
        // Query and Verification Functions
        // ============================================================================

        /// <summary>Get stored randomness for a request</summary>
        public static byte[] GetRandomness(BigInteger requestId)
        {
            StorageMap randomMap = new StorageMap(Storage.CurrentContext, PREFIX_RANDOMNESS);
            return (byte[])randomMap.Get(requestId.ToByteArray());
        }

        /// <summary>Get stored proof for a request</summary>
        public static byte[] GetProof(BigInteger requestId)
        {
            StorageMap proofMap = new StorageMap(Storage.CurrentContext, PREFIX_PROOF);
            return (byte[])proofMap.Get(requestId.ToByteArray());
        }

        /// <summary>Verify a VRF proof</summary>
        public static bool VerifyProof(byte[] seed, byte[] randomWords, byte[] proof)
        {
            ECPoint vrfPubKey = GetVRFPublicKey();
            if (vrfPubKey == null) return false;

            byte[] message = Helper.Concat(seed, randomWords);
            return CryptoLib.VerifyWithECDsa((ByteString)message, vrfPubKey, (ByteString)proof, NamedCurve.secp256r1);
        }
    }
}
