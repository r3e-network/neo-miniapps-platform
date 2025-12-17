using System;
using System.ComponentModel;
using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace NeoMiniAppPlatform.Contracts
{
    public enum AppStatus : byte
    {
        Pending = 0,
        Approved = 1,
        Disabled = 2
    }

    [DisplayName("AppRegistry")]
    [ManifestExtra("Author", "Neo MiniApp Platform")]
    [ManifestExtra("Description", "On-chain miniapp registry (manifest hash + status + allowlist anchors)")]
    public class AppRegistry : SmartContract
    {
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_APP = new byte[] { 0x02 };

        public struct AppInfo
        {
            public ByteString AppId;
            public UInt160 Developer;
            public ByteString DeveloperPubKey;
            public ByteString EntryUrl;
            public ByteString ManifestHash;
            public AppStatus Status;
            public ByteString AllowlistHash;
        }

        [DisplayName("AppRegistered")]
        public static event Action<ByteString, UInt160> OnAppRegistered;

        [DisplayName("AppUpdated")]
        public static event Action<ByteString, ByteString> OnAppUpdated;

        [DisplayName("StatusChanged")]
        public static event Action<ByteString, AppStatus> OnStatusChanged;

        public static void _deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = Runtime.Transaction;
            Storage.Put(Storage.CurrentContext, PREFIX_ADMIN, tx.Sender);
        }

        public static UInt160 Admin()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, PREFIX_ADMIN);
        }

        private static void ValidateAdmin()
        {
            UInt160 admin = Admin();
            ExecutionEngine.Assert(admin != null, "admin not set");
            ExecutionEngine.Assert(Runtime.CheckWitness(admin), "unauthorized");
        }

        private static StorageMap AppMap() => new StorageMap(Storage.CurrentContext, PREFIX_APP);

        public static AppInfo GetApp(ByteString appId)
        {
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            ByteString raw = AppMap().Get(appId);
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray.
                return new AppInfo
                {
                    AppId = (ByteString)"",
                    Developer = null,
                    DeveloperPubKey = (ByteString)"",
                    EntryUrl = (ByteString)"",
                    ManifestHash = (ByteString)"",
                    Status = AppStatus.Pending,
                    AllowlistHash = (ByteString)""
                };
            }
            return (AppInfo)StdLib.Deserialize(raw);
        }

        public static void Register(ByteString appId, ByteString manifestHash, ByteString entryUrl, ByteString developerPubKey)
        {
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            ExecutionEngine.Assert(manifestHash != null && manifestHash.Length > 0, "manifest hash required");
            ExecutionEngine.Assert(entryUrl != null && entryUrl.Length > 0, "entry url required");
            ExecutionEngine.Assert(developerPubKey != null && developerPubKey.Length > 0, "developer pubkey required");

            ByteString existing = AppMap().Get(appId);
            ExecutionEngine.Assert(existing == null, "already registered");

            Transaction tx = Runtime.Transaction;

            AppInfo info = new AppInfo
            {
                AppId = appId,
                Developer = tx.Sender,
                DeveloperPubKey = developerPubKey,
                EntryUrl = entryUrl,
                ManifestHash = manifestHash,
                Status = AppStatus.Pending,
                AllowlistHash = (ByteString)""
            };

            AppMap().Put(appId, StdLib.Serialize(info));
            OnAppRegistered(appId, info.Developer);
        }

        public static void UpdateManifest(ByteString appId, ByteString manifestHash, ByteString entryUrl)
        {
            AppInfo info = GetApp(appId);
            ExecutionEngine.Assert(info.AppId != null && info.AppId.Length > 0, "app not found");
            ExecutionEngine.Assert(Runtime.CheckWitness(info.Developer), "unauthorized");

            ExecutionEngine.Assert(manifestHash != null && manifestHash.Length > 0, "manifest hash required");
            ExecutionEngine.Assert(entryUrl != null && entryUrl.Length > 0, "entry url required");

            info.ManifestHash = manifestHash;
            info.EntryUrl = entryUrl;
            info.Status = AppStatus.Pending; // require re-approval
            AppMap().Put(appId, StdLib.Serialize(info));
            OnAppUpdated(appId, manifestHash);
        }

        public static void SetAllowlistHash(ByteString appId, ByteString allowlistHash)
        {
            AppInfo info = GetApp(appId);
            ExecutionEngine.Assert(info.AppId != null && info.AppId.Length > 0, "app not found");
            ExecutionEngine.Assert(Runtime.CheckWitness(info.Developer) || Runtime.CheckWitness(Admin()), "unauthorized");
            ExecutionEngine.Assert(allowlistHash != null, "allowlist hash required");

            info.AllowlistHash = allowlistHash;
            AppMap().Put(appId, StdLib.Serialize(info));
        }

        public static void SetStatus(ByteString appId, AppStatus status)
        {
            ValidateAdmin();
            AppInfo info = GetApp(appId);
            ExecutionEngine.Assert(info.AppId != null && info.AppId.Length > 0, "app not found");
            info.Status = status;
            AppMap().Put(appId, StdLib.Serialize(info));
            OnStatusChanged(appId, status);
        }
    }
}
