using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    public partial class ServiceLayerGateway
    {
        private static UInt160 GetAdmin() => (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });

        private static bool IsAdmin() => Runtime.CheckWitness(GetAdmin());

        internal static void RequireAdmin()
        {
            if (!IsAdmin()) throw new Exception("Admin only");
        }

        public static UInt160 Admin() => GetAdmin();

        public static void TransferAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (newAdmin == null || !newAdmin.IsValid) throw new Exception("Invalid address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        // Pause control
        private static bool IsPaused() => (BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }) == 1;

        internal static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract paused");
        }

        public static bool Paused() => IsPaused();

        public static void Pause()
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1);
        }

        public static void Unpause()
        {
            RequireAdmin();
            Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
        }
    }
}
