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
    [DisplayName("PriceFeed")]
    [ManifestExtra("Author", "Neo MiniApp Platform")]
    [ManifestExtra("Description", "On-chain price feed anchoring with attestation hash")]
    public class PriceFeed : SmartContract
    {
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_UPDATER = new byte[] { 0x02 };
        private static readonly byte[] PREFIX_PRICE = new byte[] { 0x03 };

        public struct PriceRecord
        {
            public BigInteger RoundId;
            public BigInteger Price;
            public ulong Timestamp;
            public ByteString AttestationHash;
            public BigInteger SourceSetId;
        }

        [DisplayName("PriceUpdated")]
        public static event Action<string, BigInteger, BigInteger, ulong, ByteString, BigInteger> OnPriceUpdated;

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

        private static StorageMap PriceMap() => new StorageMap(Storage.CurrentContext, PREFIX_PRICE);

        public static PriceRecord GetLatest(string symbol)
        {
            ExecutionEngine.Assert(symbol != null && symbol.Length > 0, "symbol required");
            ByteString raw = PriceMap().Get(symbol);
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray,
                // causing field access to throw (index out of range) in Neo VM.
                return new PriceRecord
                {
                    RoundId = 0,
                    Price = 0,
                    Timestamp = 0,
                    AttestationHash = (ByteString)"",
                    SourceSetId = 0
                };
            }
            return (PriceRecord)StdLib.Deserialize(raw);
        }

        public static void Update(string symbol, BigInteger roundId, BigInteger price, ulong timestamp, ByteString attestationHash, BigInteger sourceSetId)
        {
            ValidateUpdater();

            ExecutionEngine.Assert(symbol != null && symbol.Length > 0, "symbol required");
            ExecutionEngine.Assert(roundId > 0, "roundId required");
            ExecutionEngine.Assert(price > 0, "price required");
            ExecutionEngine.Assert(timestamp > 0, "timestamp required");
            ExecutionEngine.Assert(attestationHash != null && attestationHash.Length > 0, "attestation hash required");

            PriceRecord current = GetLatest(symbol);
            if (current.RoundId > 0)
            {
                ExecutionEngine.Assert(roundId > current.RoundId, "roundId must be monotonic");
            }

            PriceRecord next = new PriceRecord
            {
                RoundId = roundId,
                Price = price,
                Timestamp = timestamp,
                AttestationHash = attestationHash,
                SourceSetId = sourceSetId
            };

            PriceMap().Put(symbol, StdLib.Serialize(next));
            OnPriceUpdated(symbol, roundId, price, timestamp, attestationHash, sourceSetId);
        }
    }
}
