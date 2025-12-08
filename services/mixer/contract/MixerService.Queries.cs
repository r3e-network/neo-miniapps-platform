using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace ServiceLayer.Mixer
{
    public partial class MixerService
    {
        // ============================================================================
        // Query Functions
        // ============================================================================

        public static ServiceData GetService(byte[] serviceId)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceId);
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (ServiceData)StdLib.Deserialize((ByteString)data);
        }

        public static DisputeRecord GetDispute(byte[] requestHash)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_DISPUTE }, requestHash);
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (DisputeRecord)StdLib.Deserialize((ByteString)data);
        }

        public static bool IsRequestResolved(byte[] requestHash)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_RESOLVED }, requestHash);
            return Storage.Get(Storage.CurrentContext, key) != null;
        }

        public static bool CanClaimDisputeRefund(byte[] requestHash)
        {
            DisputeRecord dispute = GetDispute(requestHash);
            if (dispute == null) return false;
            if (dispute.Status != DISPUTE_PENDING) return false;
            return Runtime.Time > dispute.Deadline;
        }

        // ============================================================================
        // Internal Helpers
        // ============================================================================

        private static void SaveService(ServiceData service)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, service.ServiceId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(service));
        }

        private static void VerifyAndMarkNonce(System.Numerics.BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new System.Exception("Nonce already used");
            Storage.Put(Storage.CurrentContext, key, 1);
        }
    }
}
