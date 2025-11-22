using System;
using System.Numerics;
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

namespace ServiceLayer.Contracts
{
    // RandomnessHub tracks VRF/randomness requests and fulfillment.
    public class RandomnessHub : SmartContract
    {
        private static readonly StorageMap Requests = new(Storage.CurrentContext, "rnd:");

        public static event Action<ByteString, ByteString> RandomnessRequested;
        public static event Action<ByteString, ByteString> RandomnessFulfilled;

        public struct Request
        {
            public ByteString Id;
            public ByteString ServiceId;
            public ByteString SeedHash;
            public byte Status; // 0=pending,1=fulfilled,2=failed
            public ByteString Output;
            public BigInteger RequestedAt;
            public BigInteger FulfilledAt;
        }

        public static void Request(ByteString id, ByteString serviceId, ByteString seedHash)
        {
            if (id is null || id.Length == 0) throw new Exception("missing id");
            if (Requests.Get(id) is not null) throw new Exception("exists");
            var req = new Request
            {
                Id = id,
                ServiceId = serviceId,
                SeedHash = seedHash,
                Status = 0,
                RequestedAt = Runtime.Time
            };
            Requests.Put(id, StdLib.Serialize(req));
            RandomnessRequested(id, serviceId);
        }

        public static void Fulfill(ByteString id, ByteString output, byte status)
        {
            RequireRunner();
            var req = Load(id);
            req.Status = status;
            req.Output = output;
            req.FulfilledAt = Runtime.Time;
            Requests.Put(id, StdLib.Serialize(req));
            RandomnessFulfilled(id, output);
        }

        public static Request Get(ByteString id) => Load(id);

        private static Request Load(ByteString id)
        {
            var data = Requests.Get(id);
            if (data is null || data.Length == 0) throw new Exception("not found");
            return (Request)StdLib.Deserialize(data);
        }

        private static void RequireRunner()
        {
            if (!Runtime.CheckWitness((UInt160)Runtime.CallingScriptHash))
            {
                throw new Exception("runner required");
            }
        }
    }
}
