using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Mixer
{
    public partial class MixerService
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

        private static bool IsPaused() => (BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }) == 1;
        private static void RequireNotPaused() { if (IsPaused()) throw new Exception("Contract paused"); }
        public static bool Paused() => IsPaused();
        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ============================================================================
        // Admin Functions
        // ============================================================================

        /// <summary>
        /// Withdraw available bond (bond - outstanding).
        /// </summary>
        public static void WithdrawBond(byte[] serviceId, BigInteger amount, UInt160 recipient)
        {
            RequireAdmin();

            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");

            BigInteger available = service.BondAmount - service.OutstandingAmount;
            if (amount > available) throw new Exception("Amount exceeds available bond");

            service.BondAmount -= amount;
            SaveService(service);

            GAS.Transfer(Runtime.ExecutingScriptHash, recipient, amount, null);
        }

        public static void SuspendService(byte[] serviceId)
        {
            RequireAdmin();
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            service.Status = 0;
            SaveService(service);
        }

        public static void ActivateService(byte[] serviceId)
        {
            RequireAdmin();
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            service.Status = 1;
            SaveService(service);
        }

        public static void UpdateTeePubKey(byte[] serviceId, ECPoint newTeePubKey)
        {
            RequireAdmin();
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            service.TeePubKey = newTeePubKey;
            SaveService(service);
        }
    }
}
