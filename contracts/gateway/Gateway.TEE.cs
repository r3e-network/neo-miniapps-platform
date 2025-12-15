using Neo;
using Neo.Cryptography.ECC;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    public partial class ServiceLayerGateway
    {
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

        public static void RemoveTEEAccount(UInt160 teeAccount)
        {
            RequireAdmin();
            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Delete(Storage.CurrentContext, accountKey);
            Storage.Delete(Storage.CurrentContext, pubKeyKey);
        }

        public static bool IsTEEAccount(UInt160 account)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])account);
            return (BigInteger)Storage.Get(Storage.CurrentContext, key) == 1;
        }

        public static ECPoint GetTEEPublicKey(UInt160 teeAccount)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);
            return (ECPoint)Storage.Get(Storage.CurrentContext, key);
        }

        internal static void RequireTEE()
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            if (!IsTEEAccount(tx.Sender)) throw new Exception("TEE account only");
        }

        // ============================================================================
        // TEE Master Key Management
        // ============================================================================

        /// <summary>
        /// Sets the TEE master key with attestation proof.
        /// Can only be called by a registered TEE account.
        /// </summary>
        public static void SetTEEMasterKey(byte[] pubKey, byte[] pubKeyHash, byte[] attestHash, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            if (pubKey == null || pubKey.Length == 0) throw new Exception("Invalid public key");
            if (pubKeyHash == null || pubKeyHash.Length == 0) throw new Exception("Invalid public key hash");
            if (attestHash == null || attestHash.Length == 0) throw new Exception("Invalid attestation hash");

            VerifyAndMarkNonce(nonce);

            // Verify TEE signature
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            var teePubKey = GetTEEPublicKey(tx.Sender);
            if (teePubKey == null) throw new Exception("TEE key not found");

            byte[] message = Helper.Concat(pubKey, pubKeyHash);
            message = Helper.Concat(message, attestHash);
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Store master key data
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_TEE_MASTER_PUBKEY }, pubKey);
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_TEE_MASTER_PUBKEY_HASH }, pubKeyHash);
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_TEE_MASTER_ATTEST_HASH }, attestHash);

            OnTEEMasterKeySet(pubKey, pubKeyHash, attestHash);
        }

        /// <summary>
        /// Gets the TEE master public key.
        /// </summary>
        public static byte[] GetTEEMasterPubKey()
        {
            return (byte[])Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_TEE_MASTER_PUBKEY });
        }

        /// <summary>
        /// Gets the SHA-256 hash of the TEE master public key.
        /// </summary>
        public static byte[] GetTEEMasterPubKeyHash()
        {
            return (byte[])Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_TEE_MASTER_PUBKEY_HASH });
        }

        /// <summary>
        /// Gets the attestation hash/CID for the TEE master key.
        /// </summary>
        public static byte[] GetTEEMasterAttestationHash()
        {
            return (byte[])Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_TEE_MASTER_ATTEST_HASH });
        }
    }
}
