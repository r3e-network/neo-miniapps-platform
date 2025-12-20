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
    [DisplayName("AutomationAnchor")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Email", "dev@r3e.network")]
    [ManifestExtra("Version", "1.0.0")]
    [ManifestExtra("Description", "On-chain automation task anchoring with nonce-based anti-replay")]
    public class AutomationAnchor : SmartContract
    {
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_UPDATER = new byte[] { 0x02 };
        private static readonly byte[] PREFIX_TASK = new byte[] { 0x03 };
        private static readonly byte[] PREFIX_NONCE = new byte[] { 0x04 };
        private static readonly byte[] PREFIX_EXEC = new byte[] { 0x05 };

        public struct Task
        {
            public ByteString TaskId;
            public UInt160 Target;
            public string Method;
            public ByteString Trigger;
            public BigInteger GasLimit;
            public bool Enabled;
        }

        public struct ExecutionRecord
        {
            public ByteString TaskId;
            public BigInteger Nonce;
            public ByteString TxHash;
            public ulong Timestamp;
        }

        [DisplayName("TaskRegistered")]
        public static event Action<ByteString, UInt160, string> OnTaskRegistered;

        [DisplayName("Executed")]
        public static event Action<ByteString, BigInteger, ByteString> OnExecuted;

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

        private static StorageMap TaskMap() => new StorageMap(Storage.CurrentContext, PREFIX_TASK);
        private static StorageMap NonceMap() => new StorageMap(Storage.CurrentContext, PREFIX_NONCE);
        private static StorageMap ExecMap() => new StorageMap(Storage.CurrentContext, PREFIX_EXEC);

        public static Task GetTask(ByteString taskId)
        {
            ExecutionEngine.Assert(taskId != null && taskId.Length > 0, "taskId required");
            ByteString raw = TaskMap().Get(taskId);
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray.
                return new Task
                {
                    TaskId = (ByteString)"",
                    Target = null,
                    Method = "",
                    Trigger = (ByteString)"",
                    GasLimit = 0,
                    Enabled = false
                };
            }
            return (Task)StdLib.Deserialize(raw);
        }

        public static void RegisterTask(ByteString taskId, UInt160 target, string method, ByteString trigger, BigInteger gasLimit, bool enabled)
        {
            ValidateAdmin();

            ExecutionEngine.Assert(taskId != null && taskId.Length > 0, "taskId required");
            ExecutionEngine.Assert(target != null && target.IsValid, "target required");
            ExecutionEngine.Assert(method != null && method.Length > 0, "method required");
            ExecutionEngine.Assert(gasLimit >= 0, "invalid gasLimit");

            Task t = new Task
            {
                TaskId = taskId,
                Target = target,
                Method = method,
                Trigger = trigger ?? (ByteString)"",
                GasLimit = gasLimit,
                Enabled = enabled
            };
            TaskMap().Put(taskId, StdLib.Serialize(t));
            OnTaskRegistered(taskId, target, method);
        }

        public static bool IsNonceUsed(ByteString taskId, BigInteger nonce)
        {
            byte[] key = Helper.Concat((byte[])taskId, nonce.ToByteArray());
            return NonceMap().Get(key) != null;
        }

        public static void MarkExecuted(ByteString taskId, BigInteger nonce, ByteString txHash)
        {
            ValidateUpdater();

            Task t = GetTask(taskId);
            ExecutionEngine.Assert(t.TaskId != null && t.TaskId.Length > 0, "task not found");
            ExecutionEngine.Assert(t.Enabled, "task disabled");
            ExecutionEngine.Assert(nonce >= 0, "invalid nonce");
            ExecutionEngine.Assert(txHash != null && txHash.Length > 0, "txHash required");

            byte[] nonceKey = Helper.Concat((byte[])taskId, nonce.ToByteArray());
            ExecutionEngine.Assert(NonceMap().Get(nonceKey) == null, "nonce already used");
            NonceMap().Put(nonceKey, 1);

            ExecutionRecord rec = new ExecutionRecord
            {
                TaskId = taskId,
                Nonce = nonce,
                TxHash = txHash,
                Timestamp = Runtime.Time
            };
            ExecMap().Put(nonceKey, StdLib.Serialize(rec));
            OnExecuted(taskId, nonce, txHash);
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
