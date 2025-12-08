using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Common
{
    /// <summary>
    /// Base contract for all Service Layer contracts.
    /// Provides common functionality: admin management, pause control, TEE verification.
    /// </summary>
    public abstract class ServiceLayerBase : SmartContract
    {
        // Storage prefixes
        protected const byte PREFIX_ADMIN = 0x01;
        protected const byte PREFIX_PAUSED = 0x02;
        protected const byte PREFIX_TEE_PUBKEY = 0x03;
        protected const byte PREFIX_NONCE = 0x04;
        protected const byte PREFIX_FEE_COLLECTOR = 0x05;

        // Events
        [DisplayName("AdminChanged")]
        public static event Action<UInt160, UInt160> OnAdminChanged;

        [DisplayName("Paused")]
        public static event Action<UInt160> OnPaused;

        [DisplayName("Unpaused")]
        public static event Action<UInt160> OnUnpaused;

        [DisplayName("TEEKeyUpdated")]
        public static event Action<ECPoint> OnTEEKeyUpdated;

        // ============================================================================
        // Admin Management
        // ============================================================================

        protected static UInt160 GetAdmin()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
        }

        protected static void SetAdmin(UInt160 admin)
        {
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, admin);
        }

        protected static bool IsAdmin()
        {
            UInt160 admin = GetAdmin();
            if (admin == null) return false;
            return Runtime.CheckWitness(admin);
        }

        protected static void RequireAdmin()
        {
            if (!IsAdmin()) throw new Exception("Not authorized: admin only");
        }

        public static UInt160 Admin() => GetAdmin();

        public static void TransferAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (newAdmin == null || !newAdmin.IsValid) throw new Exception("Invalid admin address");

            UInt160 oldAdmin = GetAdmin();
            SetAdmin(newAdmin);
            OnAdminChanged(oldAdmin, newAdmin);
        }

        // ============================================================================
        // Pause Control
        // ============================================================================

        protected static bool IsPaused()
        {
            return (BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }) == 1;
        }

        protected static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract is paused");
        }

        public static bool Paused() => IsPaused();

        public static void Pause()
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1);
            OnPaused(GetAdmin());
        }

        public static void Unpause()
        {
            RequireAdmin();
            Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            OnUnpaused(GetAdmin());
        }

        // ============================================================================
        // TEE Verification
        // ============================================================================

        protected static ECPoint GetTEEPublicKey()
        {
            return (ECPoint)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_TEE_PUBKEY });
        }

        protected static void SetTEEPublicKey(ECPoint pubKey)
        {
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_TEE_PUBKEY }, pubKey);
        }

        public static ECPoint TEEPublicKey() => GetTEEPublicKey();

        public static void UpdateTEEPublicKey(ECPoint newPubKey)
        {
            RequireAdmin();
            if (newPubKey == null) throw new Exception("Invalid public key");
            SetTEEPublicKey(newPubKey);
            OnTEEKeyUpdated(newPubKey);
        }

        /// <summary>
        /// Verify a signature from the TEE enclave.
        /// </summary>
        protected static bool VerifyTEESignature(byte[] message, byte[] signature)
        {
            ECPoint pubKey = GetTEEPublicKey();
            if (pubKey == null) throw new Exception("TEE public key not set");
            return CryptoLib.VerifyWithECDsa(message, pubKey, signature, NamedCurve.secp256r1);
        }

        /// <summary>
        /// Verify TEE signature with nonce to prevent replay attacks.
        /// </summary>
        protected static bool VerifyTEESignatureWithNonce(byte[] message, byte[] signature, BigInteger nonce)
        {
            // Check nonce
            byte[] nonceKey = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, nonceKey) != null)
                throw new Exception("Nonce already used");

            // Mark nonce as used
            Storage.Put(Storage.CurrentContext, nonceKey, 1);

            // Verify signature
            return VerifyTEESignature(message, signature);
        }

        // ============================================================================
        // Fee Management
        // ============================================================================

        protected static UInt160 GetFeeCollector()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_FEE_COLLECTOR });
        }

        public static UInt160 FeeCollector() => GetFeeCollector();

        public static void SetFeeCollector(UInt160 collector)
        {
            RequireAdmin();
            if (collector == null || !collector.IsValid) throw new Exception("Invalid collector address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_FEE_COLLECTOR }, collector);
        }

        // ============================================================================
        // Utility Functions
        // ============================================================================

        protected static byte[] GetStorageKey(byte prefix, byte[] key)
        {
            return Helper.Concat(new byte[] { prefix }, key);
        }

        protected static byte[] GetStorageKey(byte prefix, UInt160 address)
        {
            return Helper.Concat(new byte[] { prefix }, (byte[])address);
        }

        protected static byte[] GetStorageKey(byte prefix, BigInteger id)
        {
            return Helper.Concat(new byte[] { prefix }, id.ToByteArray());
        }

        protected static void RequireValidAddress(UInt160 address)
        {
            if (address == null || !address.IsValid) throw new Exception("Invalid address");
        }

        protected static void RequirePositiveAmount(BigInteger amount)
        {
            if (amount <= 0) throw new Exception("Amount must be positive");
        }
    }
}
