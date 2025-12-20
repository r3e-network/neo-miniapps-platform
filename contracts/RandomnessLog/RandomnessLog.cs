using System;
using System.ComponentModel;
using System.Numerics;
using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace NeoMiniAppPlatform.Contracts
{
    [DisplayName("RandomnessLog")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Email", "dev@r3e.network")]
    [ManifestExtra("Version", "1.0.0")]
    [ManifestExtra("Description", "On-chain randomness anchoring with attestation hash")]
    public class RandomnessLog : SmartContract
    {
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_UPDATER = new byte[] { 0x02 };
        private static readonly byte[] PREFIX_RECORD = new byte[] { 0x03 };

        public struct RandomRecord
        {
            public string RequestId;
            public ByteString Randomness;
            public ByteString AttestationHash;
            public ulong Timestamp;
        }

        [DisplayName("RandomnessRecorded")]
        public static event Action<string, ByteString, ByteString, ulong> OnRandomnessRecorded;

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

        public static void SetUpdater(UInt160 updater)
        {
            ValidateAdmin();
            ExecutionEngine.Assert(updater != null && updater.IsValid, "invalid updater");
            Storage.Put(Storage.CurrentContext, PREFIX_UPDATER, updater);
        }

        public static UInt160 Updater()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, PREFIX_UPDATER);
        }

        private static void ValidateUpdater()
        {
            UInt160 updater = Updater();
            ExecutionEngine.Assert(updater != null && updater.IsValid, "updater not set");
            ExecutionEngine.Assert(Runtime.CheckWitness(updater), "unauthorized");
        }

        private static StorageMap RecordMap() => new StorageMap(Storage.CurrentContext, PREFIX_RECORD);

        private static ByteString RequestKey(string requestId)
        {
            ExecutionEngine.Assert(requestId != null && requestId.Length > 0, "requestId required");
            return (ByteString)requestId;
        }

        public static RandomRecord Get(string requestId)
        {
            ByteString raw = RecordMap().Get(RequestKey(requestId));
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray.
                return new RandomRecord
                {
                    RequestId = "",
                    Randomness = (ByteString)"",
                    AttestationHash = (ByteString)"",
                    Timestamp = 0
                };
            }
            return (RandomRecord)StdLib.Deserialize(raw);
        }

        public static void Record(string requestId, ByteString randomness, ByteString attestationHash, ulong timestamp)
        {
            ValidateUpdater();

            ExecutionEngine.Assert(requestId != null && requestId.Length > 0, "requestId required");
            ExecutionEngine.Assert(randomness != null && randomness.Length > 0, "randomness required");
            ExecutionEngine.Assert(attestationHash != null && attestationHash.Length > 0, "attestationHash required");
            ExecutionEngine.Assert(timestamp > 0, "timestamp required");

            ByteString key = RequestKey(requestId);
            ByteString existing = RecordMap().Get(key);
            ExecutionEngine.Assert(existing == null, "request already recorded");

            RandomRecord rec = new RandomRecord
            {
                RequestId = requestId,
                Randomness = randomness,
                AttestationHash = attestationHash,
                Timestamp = timestamp
            };

            RecordMap().Put(key, StdLib.Serialize(rec));
            OnRandomnessRecorded(requestId, randomness, attestationHash, timestamp);
        }

        public static void SetAdmin(UInt160 newAdmin)
        {
            ValidateAdmin();
            ExecutionEngine.Assert(newAdmin != null && newAdmin.IsValid, "invalid admin");
            Storage.Put(Storage.CurrentContext, PREFIX_ADMIN, newAdmin);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            ValidateAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }
}
