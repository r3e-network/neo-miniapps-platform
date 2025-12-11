using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    public partial class ServiceLayerGateway
    {
        private static void VerifyAndMarkNonce(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new Exception("Nonce already used");
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        public static bool IsNonceUsed(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            return Storage.Get(Storage.CurrentContext, key) != null;
        }
    }
}
