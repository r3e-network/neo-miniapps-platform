using Neo.SmartContract.Framework;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    public class RequestData
    {
        public BigInteger Id;
        public UInt160 UserContract;
        public UInt160 Caller;
        public string ServiceType;
        public UInt160 ServiceContract;
        public byte[] Payload;
        public string CallbackMethod;
        public byte Status;
        public ulong CreatedAt;
        public byte[] Result;
        public string Error;
        public ulong CompletedAt;
    }
}
