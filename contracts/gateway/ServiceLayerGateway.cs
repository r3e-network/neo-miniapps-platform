using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    /// <summary>
    /// ServiceLayerGateway - entry point and router for all Service Layer services.
    /// </summary>
    [DisplayName("ServiceLayerGateway")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Email", "dev@r3e.network")]
    [ManifestExtra("Description", "Service Layer Gateway - Main entry for all services")]
    [ManifestExtra("Version", "3.0.1")]
    // SECURITY: Broad permission required for router functionality.
    [ContractPermission("*", "*")]
    public partial class ServiceLayerGateway : SmartContract
    {
        // ============================================================================
        // Storage Prefixes
        // ============================================================================
        internal const byte PREFIX_ADMIN = 0x01;
        internal const byte PREFIX_PAUSED = 0x02;
        internal const byte PREFIX_TEE_ACCOUNT = 0x10;
        internal const byte PREFIX_TEE_PUBKEY = 0x11;
        internal const byte PREFIX_SERVICE = 0x20;
        internal const byte PREFIX_REQUEST = 0x40;
        internal const byte PREFIX_REQUEST_COUNT = 0x41;
        internal const byte PREFIX_NONCE = 0x50;

        // Request status
        internal const byte STATUS_PENDING = 0;
        internal const byte STATUS_PROCESSING = 1;
        internal const byte STATUS_COMPLETED = 2;
        internal const byte STATUS_FAILED = 3;

        // ============================================================================
        // Events
        // ============================================================================

        [DisplayName("ServiceRequest")]
        public static event Action<BigInteger, UInt160, UInt160, string, byte[]> OnServiceRequest;

        [DisplayName("RequestFulfilled")]
        public static event Action<BigInteger, byte[]> OnRequestFulfilled;

        [DisplayName("RequestFailed")]
        public static event Action<BigInteger, string> OnRequestFailed;

        [DisplayName("CallbackExecuted")]
        public static event Action<BigInteger, UInt160, string, bool> OnCallbackExecuted;

        [DisplayName("TEERegistered")]
        public static event Action<UInt160, ECPoint> OnTEERegistered;

        [DisplayName("ServiceRegistered")]
        public static event Action<string, UInt160> OnServiceRegistered;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;

            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, tx.Sender);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest);
        }
    }
}
