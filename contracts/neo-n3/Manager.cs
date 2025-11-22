using System;
using System.Numerics;
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

namespace ServiceLayer.Contracts
{
    // Manager holds contract hashes for modules and governs roles/pause flags.
    public class Manager : SmartContract
    {
        private static readonly StorageMap Modules = new(Storage.CurrentContext, "mod:");
        private static readonly StorageMap Roles = new(Storage.CurrentContext, "role:");
        private static readonly StorageMap Paused = new(Storage.CurrentContext, "pause:");

        // Roles are bit flags to allow multiple capabilities per account.
        public const byte RoleAdmin = 0x01;
        public const byte RoleScheduler = 0x02;
        public const byte RoleOracleRunner = 0x04;
        public const byte RoleRandomnessRunner = 0x08;
        public const byte RoleJamRunner = 0x10;
        public const byte RoleDataFeedSigner = 0x20;

        public static event Action<string, UInt160> ModuleUpgraded;
        public static event Action<UInt160, byte> RoleGranted;
        public static event Action<UInt160, byte> RoleRevoked;
        public static event Action<string, bool> Paused;

        public static void SetModule(string name, UInt160 hash)
        {
            AssertAdmin();
            if (hash is null || !hash.IsValid)
            {
                throw new Exception("invalid hash");
            }
            Modules.Put(name, hash);
            ModuleUpgraded(name, hash);
        }

        public static UInt160 GetModule(string name)
        {
            var bytes = Modules.Get(name);
            return bytes is null || bytes.Length == 0 ? UInt160.Zero : (UInt160)bytes;
        }

        public static void GrantRole(UInt160 account, byte role)
        {
            AssertAdmin();
            if (account is null || !account.IsValid)
            {
                throw new Exception("invalid account");
            }
            var existing = (byte)Roles.Get(account)!.ToByteArray()[0];
            var updated = (byte)(existing | role);
            Roles.Put(account, new byte[] { updated });
            RoleGranted(account, role);
        }

        public static void RevokeRole(UInt160 account, byte role)
        {
            AssertAdmin();
            var existing = (byte)Roles.Get(account)!.ToByteArray()[0];
            var updated = (byte)(existing & ~role);
            Roles.Put(account, new byte[] { updated });
            RoleRevoked(account, role);
        }

        public static bool HasRole(UInt160 account, byte role)
        {
            if (account is null || !account.IsValid)
            {
                return false;
            }
            var stored = Roles.Get(account);
            if (stored is null || stored.Length == 0)
            {
                return false;
            }
            var flags = (byte)stored.ToByteArray()[0];
            return (flags & role) != 0;
        }

        public static void Pause(string name, bool flag)
        {
            AssertAdmin();
            Paused.Put(name, flag ? 1 : 0);
            Paused(name, flag);
        }

        public static bool IsPaused(string name)
        {
            var val = Paused.Get(name);
            return val is not null && val.Length > 0 && val.ToByteArray()[0] != 0;
        }

        private static void AssertAdmin()
        {
            var txSender = (UInt160)Runtime.CallingScriptHash;
            if (!HasRole(txSender, RoleAdmin) && !Runtime.CheckWitness(txSender))
            {
                throw new Exception("admin required");
            }
        }
    }
}
