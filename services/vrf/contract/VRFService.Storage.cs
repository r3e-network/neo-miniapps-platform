using Neo;
using Neo.SmartContract.Framework.Services;
using System.Numerics;

namespace ServiceLayer.VRF
{
    public partial class VRFService
    {
        // ============================================================================
        // Storage Prefixes
        // ============================================================================

        // PREFIX_GATEWAY inherited from ServiceContractBase
        private const byte PREFIX_VRF_PUBKEY = 0x02;
        private const byte PREFIX_REQUEST = 0x10;
        private const byte PREFIX_RANDOMNESS = 0x20;
        private const byte PREFIX_PROOF = 0x21;

        // ============================================================================
        // Storage Access Methods
        // ============================================================================

        /// <summary>Set VRF public key for proof verification</summary>
        public static void SetVRFPublicKey(ECPoint pubKey)
        {
            RequireGateway();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_VRF_PUBKEY }, pubKey);
        }

        public static ECPoint GetVRFPublicKey()
        {
            return (ECPoint)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_VRF_PUBKEY });
        }
    }
}
