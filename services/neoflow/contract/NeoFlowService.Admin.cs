using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System;

namespace ServiceLayer.Automation
{
    public partial class NeoFlowService
    {
        // ============================================================================
        // Admin Management
        // ============================================================================

        private static UInt160 GetAdmin() => (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
        private static bool IsAdmin() => Runtime.CheckWitness(GetAdmin());
        private static void RequireAdmin() { if (!IsAdmin()) throw new Exception("Admin only"); }

        public static UInt160 Admin() => GetAdmin();

        public static void TransferAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (newAdmin == null || !newAdmin.IsValid) throw new Exception("Invalid address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        // ============================================================================
        // Pause Control
        // ============================================================================

        private static bool IsPaused() => (System.Numerics.BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }) == 1;
        private static void RequireNotPaused() { if (IsPaused()) throw new Exception("Contract paused"); }
        public static bool Paused() => IsPaused();
        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ============================================================================
        // Gateway Management
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static UInt160 GetGateway()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
        }

        private static void RequireGateway()
        {
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }
    }
}
