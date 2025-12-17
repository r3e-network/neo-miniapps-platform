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
    [DisplayName("PaymentHub")]
    [ManifestExtra("Author", "Neo MiniApp Platform")]
    [ManifestExtra("Description", "GAS-only payments & settlement hub")]
    public class PaymentHub : SmartContract
    {
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_APP = new byte[] { 0x02 };
        private static readonly byte[] PREFIX_BALANCE = new byte[] { 0x03 };
        private static readonly byte[] PREFIX_RECEIPT = new byte[] { 0x04 };
        private static readonly byte[] KEY_RECEIPT_COUNTER = new byte[] { 0x05 };

        public struct AppConfig
        {
            public UInt160 Owner;
            public UInt160[] Recipients;
            public BigInteger[] SharesBps;
            public bool Enabled;
        }

        public struct PaymentData
        {
            public string AppId;
            public string Memo;
        }

        public struct Receipt
        {
            public BigInteger Id;
            public string AppId;
            public UInt160 Payer;
            public BigInteger Amount;
            public ulong Timestamp;
            public string Memo;
        }

        [DisplayName("PaymentReceived")]
        public static event Action<BigInteger, string, UInt160, BigInteger, string> OnPaymentReceived;

        [DisplayName("AppConfigured")]
        public static event Action<string, UInt160> OnAppConfigured;

        [DisplayName("Withdrawn")]
        public static event Action<string, BigInteger> OnWithdrawn;

        public static void _deploy(object data, bool update)
        {
            if (update) return;

            Transaction tx = Runtime.Transaction;
            Storage.Put(Storage.CurrentContext, PREFIX_ADMIN, tx.Sender);
            Storage.Put(Storage.CurrentContext, PREFIX_RECEIPT.Concat(KEY_RECEIPT_COUNTER), 0);
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
        private static StorageMap BalanceMap() => new StorageMap(Storage.CurrentContext, PREFIX_BALANCE);
        private static StorageMap ReceiptMap() => new StorageMap(Storage.CurrentContext, PREFIX_RECEIPT);

        private static ByteString AppKey(string appId)
        {
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            return (ByteString)appId;
        }

        public static AppConfig GetApp(string appId)
        {
            ByteString raw = AppMap().Get(AppKey(appId));
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray.
                return new AppConfig
                {
                    Owner = null,
                    Recipients = new UInt160[0],
                    SharesBps = new BigInteger[0],
                    Enabled = false
                };
            }
            return (AppConfig)StdLib.Deserialize(raw);
        }

        public static BigInteger GetAppBalance(string appId)
        {
            ByteString raw = BalanceMap().Get(AppKey(appId));
            if (raw == null) return 0;
            return (BigInteger)raw;
        }

        private static void SetAppBalance(string appId, BigInteger amount)
        {
            BalanceMap().Put(AppKey(appId), amount);
        }

        private static BigInteger NextReceiptId()
        {
            StorageMap meta = new StorageMap(Storage.CurrentContext, PREFIX_RECEIPT);
            ByteString raw = meta.Get(KEY_RECEIPT_COUNTER);
            BigInteger current = raw == null ? 0 : (BigInteger)raw;
            BigInteger next = current + 1;
            meta.Put(KEY_RECEIPT_COUNTER, next);
            return next;
        }

        public static Receipt GetReceipt(BigInteger receiptId)
        {
            ByteString raw = ReceiptMap().Get(receiptId.ToByteArray());
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray.
                return new Receipt
                {
                    Id = 0,
                    AppId = "",
                    Payer = null,
                    Amount = 0,
                    Timestamp = 0,
                    Memo = ""
                };
            }
            return (Receipt)StdLib.Deserialize(raw);
        }

        // ============================================================================
        // Admin / App Configuration
        // ============================================================================

        public static void SetAdmin(UInt160 newAdmin)
        {
            ValidateAdmin();
            ExecutionEngine.Assert(newAdmin != null && newAdmin.IsValid, "invalid admin");
            Storage.Put(Storage.CurrentContext, PREFIX_ADMIN, newAdmin);
        }

        public static void ConfigureApp(string appId, UInt160 owner, UInt160[] recipients, BigInteger[] sharesBps, bool enabled)
        {
            ValidateAdmin();
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            ExecutionEngine.Assert(owner != null && owner.IsValid, "owner required");
            ValidateSplit(recipients, sharesBps);

            AppConfig cfg = new AppConfig
            {
                Owner = owner,
                Recipients = recipients,
                SharesBps = sharesBps,
                Enabled = enabled
            };

            AppMap().Put(AppKey(appId), StdLib.Serialize(cfg));
            OnAppConfigured(appId, owner);
        }

        public static void ConfigureSplit(string appId, UInt160[] recipients, BigInteger[] sharesBps)
        {
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            AppConfig cfg = GetApp(appId);
            ExecutionEngine.Assert(cfg.Owner != null && cfg.Owner.IsValid, "app not found");
            ExecutionEngine.Assert(Runtime.CheckWitness(cfg.Owner) || Runtime.CheckWitness(Admin()), "unauthorized");

            ValidateSplit(recipients, sharesBps);
            cfg.Recipients = recipients;
            cfg.SharesBps = sharesBps;
            AppMap().Put(AppKey(appId), StdLib.Serialize(cfg));
        }

        private static void ValidateSplit(UInt160[] recipients, BigInteger[] sharesBps)
        {
            ExecutionEngine.Assert(recipients != null && sharesBps != null, "split required");
            ExecutionEngine.Assert(recipients.Length == sharesBps.Length, "split length mismatch");
            ExecutionEngine.Assert(recipients.Length > 0 && recipients.Length <= 16, "invalid recipients");

            BigInteger total = 0;
            for (int i = 0; i < recipients.Length; i++)
            {
                ExecutionEngine.Assert(recipients[i] != null && recipients[i].IsValid, "invalid recipient");
                ExecutionEngine.Assert(sharesBps[i] >= 0, "invalid share");
                total += sharesBps[i];
            }
            ExecutionEngine.Assert(total == 10000, "shares must sum to 10000 bps");
        }

        // ============================================================================
        // Payments (GAS only)
        // ============================================================================

        public static BigInteger Pay(string appId, BigInteger amount, string memo)
        {
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            ExecutionEngine.Assert(amount > 0, "amount must be > 0");

            AppConfig cfg = GetApp(appId);
            ExecutionEngine.Assert(cfg.Owner != null && cfg.Owner.IsValid, "app not found");
            ExecutionEngine.Assert(cfg.Enabled, "app disabled");

            Transaction tx = Runtime.Transaction;
            UInt160 payer = tx.Sender;

            PaymentData data = new PaymentData { AppId = appId, Memo = memo ?? "" };
            ByteString payload = (ByteString)StdLib.Serialize(data);

            bool ok = GAS.Transfer(payer, Runtime.ExecutingScriptHash, amount, payload);
            ExecutionEngine.Assert(ok, "GAS transfer failed");

            // OnNEP17Payment records the receipt and increments the counter; return latest id.
            StorageMap meta = new StorageMap(Storage.CurrentContext, PREFIX_RECEIPT);
            ByteString raw = meta.Get(KEY_RECEIPT_COUNTER);
            return raw == null ? 0 : (BigInteger)raw;
        }

        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            // Enforce payments/settlement in GAS only.
            //
            // Note: Some token transfer paths may trigger NEP-17 payment callbacks on
            // contracts involved in the transfer (including senders). To avoid breaking
            // outbound transfers, we ignore callbacks that originate from this contract.
            if (Runtime.CallingScriptHash != GAS.Hash)
            {
                if (from == Runtime.ExecutingScriptHash) return;
                throw new Exception("Only GAS accepted");
            }
            if (amount <= 0) throw new Exception("Invalid amount");

            // Ignore sender-side hooks during outbound transfers.
            if (from == Runtime.ExecutingScriptHash) return;

            if (data == null) throw new Exception("Payment data required");
            ByteString rawData = (ByteString)data;
            PaymentData pd = (PaymentData)StdLib.Deserialize(rawData);

            ExecutionEngine.Assert(pd.AppId != null && pd.AppId.Length > 0, "app id required");

            AppConfig cfg = GetApp(pd.AppId);
            ExecutionEngine.Assert(cfg.Owner != null && cfg.Owner.IsValid, "app not found");
            ExecutionEngine.Assert(cfg.Enabled, "app disabled");

            // Update app balance.
            BigInteger bal = GetAppBalance(pd.AppId);
            SetAppBalance(pd.AppId, bal + amount);

            // Store receipt.
            BigInteger receiptId = NextReceiptId();
            Receipt receipt = new Receipt
            {
                Id = receiptId,
                AppId = pd.AppId,
                Payer = from,
                Amount = amount,
                Timestamp = Runtime.Time,
                Memo = pd.Memo ?? ""
            };
            ReceiptMap().Put(receiptId.ToByteArray(), StdLib.Serialize(receipt));

            OnPaymentReceived(receiptId, pd.AppId, from, amount, receipt.Memo);
        }

        // ============================================================================
        // Settlement
        // ============================================================================

        public static void Withdraw(string appId)
        {
            ExecutionEngine.Assert(appId != null && appId.Length > 0, "app id required");
            AppConfig cfg = GetApp(appId);
            ExecutionEngine.Assert(cfg.Owner != null && cfg.Owner.IsValid, "app not found");

            ExecutionEngine.Assert(Runtime.CheckWitness(cfg.Owner) || Runtime.CheckWitness(Admin()), "unauthorized");

            BigInteger bal = GetAppBalance(appId);
            if (bal <= 0) return;

            BigInteger remaining = bal;
            BigInteger total = 0;
            for (int i = 0; i < cfg.SharesBps.Length; i++) total += cfg.SharesBps[i];
            ExecutionEngine.Assert(total > 0, "invalid split");

            for (int i = 0; i < cfg.Recipients.Length; i++)
            {
                BigInteger share = cfg.SharesBps[i];
                if (share <= 0) continue;

                BigInteger part = bal * share / total;
                if (part <= 0) continue;

                bool ok = GAS.Transfer(Runtime.ExecutingScriptHash, cfg.Recipients[i], part, null);
                ExecutionEngine.Assert(ok, "withdraw transfer failed");
                remaining -= part;
            }

            // Remainder to owner for determinism.
            if (remaining > 0)
            {
                bool ok = GAS.Transfer(Runtime.ExecutingScriptHash, cfg.Owner, remaining, null);
                ExecutionEngine.Assert(ok, "remainder transfer failed");
            }

            SetAppBalance(appId, 0);
            OnWithdrawn(appId, bal);
        }
    }
}
