using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.DataFeeds
{
    public partial class DataFeedsService
    {
        // ============================================================================
        // TEE Account Management
        // ============================================================================

        /// <summary>Register a TEE account that can push price updates</summary>
        public static void RegisterTEEAccount(UInt160 teeAccount, ECPoint teePubKey)
        {
            RequireAdmin();
            if (teeAccount == null || !teeAccount.IsValid) throw new Exception("Invalid TEE account");
            if (teePubKey == null) throw new Exception("Invalid public key");

            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Put(Storage.CurrentContext, accountKey, 1);
            Storage.Put(Storage.CurrentContext, pubKeyKey, teePubKey);

            OnTEERegistered(teeAccount, teePubKey);
        }

        /// <summary>Remove a TEE account</summary>
        public static void RemoveTEEAccount(UInt160 teeAccount)
        {
            RequireAdmin();
            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Delete(Storage.CurrentContext, accountKey);
            Storage.Delete(Storage.CurrentContext, pubKeyKey);
        }

        /// <summary>Check if account is registered TEE</summary>
        public static bool IsTEEAccount(UInt160 account)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])account);
            return (BigInteger)Storage.Get(Storage.CurrentContext, key) == 1;
        }

        /// <summary>Get TEE public key</summary>
        public static ECPoint GetTEEPublicKey(UInt160 teeAccount)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);
            return (ECPoint)Storage.Get(Storage.CurrentContext, key);
        }

        private static void RequireTEE()
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            if (!IsTEEAccount(tx.Sender)) throw new Exception("TEE account only");
        }
    }
}
