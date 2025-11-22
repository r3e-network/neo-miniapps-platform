using System;
using System.Numerics;
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

namespace ServiceLayer.Contracts
{
    // JAMInbox stores content-addressed receipts and accumulator roots per service.
    public class JAMInbox : SmartContract
    {
        private static readonly StorageMap Receipts = new(Storage.CurrentContext, "rcpt:");
        private static readonly StorageMap Roots = new(Storage.CurrentContext, "root:");
        private static readonly StorageMap Seq = new(Storage.CurrentContext, "seq:");

        public const byte EntryTypePackage = 0x01;
        public const byte EntryTypeReport = 0x02;

        public static event Action<ByteString, ByteString, BigInteger, ByteString> ReceiptAppended;

        public static void AppendReceipt(ByteString hash, ByteString serviceId, byte entryType, ByteString prevRoot, ByteString newRoot, byte status, BigInteger processedAt)
        {
            RequireRunner();
            if (hash is null || hash.Length == 0) throw new Exception("missing hash");
            if (serviceId is null || serviceId.Length == 0) throw new Exception("missing service");
            var seq = NextSeq(serviceId);
            var payload = StdLib.Serialize(new Receipt
            {
                Hash = hash,
                ServiceId = serviceId,
                EntryType = entryType,
                Seq = seq,
                PrevRoot = prevRoot,
                NewRoot = newRoot,
                Status = status,
                ProcessedAt = processedAt
            });
            Receipts.Put(hash, payload);
            Roots.Put(serviceId, newRoot);
            ReceiptAppended(hash, serviceId, seq, newRoot);
        }

        public static Receipt GetReceipt(ByteString hash)
        {
            var data = Receipts.Get(hash);
            if (data is null || data.Length == 0) return default;
            return (Receipt)StdLib.Deserialize(data);
        }

        public static ByteString GetRoot(ByteString serviceId)
        {
            return Roots.Get(serviceId);
        }

        private static BigInteger NextSeq(ByteString serviceId)
        {
            var existing = Seq.Get(serviceId);
            BigInteger current = 0;
            if (existing is not null && existing.Length > 0)
            {
                current = existing.ToBigInteger();
            }
            var next = current + 1;
            Seq.Put(serviceId, next);
            return next;
        }

        private static void RequireRunner()
        {
            // Use Manager role gate: this assumes Manager is calling with caller script hash == runner.
            if (!Runtime.CheckWitness((UInt160)Runtime.CallingScriptHash))
            {
                throw new Exception("runner required");
            }
        }
    }

    public struct Receipt
    {
        public ByteString Hash;
        public ByteString ServiceId;
        public byte EntryType;
        public BigInteger Seq;
        public ByteString PrevRoot;
        public ByteString NewRoot;
        public byte Status;
        public BigInteger ProcessedAt;
    }
}
