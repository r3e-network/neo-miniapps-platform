using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Gateway
{
    public partial class ServiceLayerGateway
    {
        public static BigInteger RequestService(string serviceType, byte[] payload, string callbackMethod)
        {
            RequireNotPaused();

            UInt160 userContract = Runtime.CallingScriptHash;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            UInt160 caller = tx.Sender;

            UInt160 serviceContract = GetServiceContract(serviceType);
            if (serviceContract == null) throw new Exception("Service not registered");

            BigInteger requestId = GetNextRequestId();

            RequestData request = new RequestData
            {
                Id = requestId,
                UserContract = userContract,
                Caller = caller,
                ServiceType = serviceType,
                ServiceContract = serviceContract,
                Payload = payload,
                CallbackMethod = callbackMethod ?? "",
                Status = STATUS_PENDING,
                CreatedAt = Runtime.Time
            };

            SaveRequest(requestId, request);

            Contract.Call(serviceContract, "onRequest", CallFlags.All, new object[] { requestId, userContract, payload });
            OnServiceRequest(requestId, userContract, caller, serviceType, payload);

            return requestId;
        }

        public static void FulfillRequest(BigInteger requestId, byte[] result, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            VerifyAndMarkNonce(nonce);

            RequestData request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != STATUS_PENDING && request.Status != STATUS_PROCESSING)
                throw new Exception("Request already processed");

            Transaction tx = (Transaction)Runtime.ScriptContainer;
            var teePubKey = GetTEEPublicKey(tx.Sender);
            if (teePubKey == null) throw new Exception("TEE key not found");

            byte[] message = Helper.Concat(requestId.ToByteArray(), result);
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            request.Status = STATUS_COMPLETED;
            request.Result = result;
            request.CompletedAt = Runtime.Time;
            SaveRequest(requestId, request);

            Contract.Call(request.ServiceContract, "onFulfill", CallFlags.All, new object[] { requestId, result });
            OnRequestFulfilled(requestId, result);

            if (!string.IsNullOrEmpty(request.CallbackMethod))
            {
                ExecuteCallback(requestId, request.UserContract, request.CallbackMethod, result, true, "");
            }
        }

        public static void FailRequest(BigInteger requestId, string reason, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            VerifyAndMarkNonce(nonce);

            RequestData request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != STATUS_PENDING && request.Status != STATUS_PROCESSING)
                throw new Exception("Request already processed");

            Transaction tx = (Transaction)Runtime.ScriptContainer;
            var teePubKey = GetTEEPublicKey(tx.Sender);

            byte[] message = Helper.Concat(requestId.ToByteArray(), reason.ToByteArray());
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            request.Status = STATUS_FAILED;
            request.Error = reason;
            request.CompletedAt = Runtime.Time;
            SaveRequest(requestId, request);

            OnRequestFailed(requestId, reason);

            if (!string.IsNullOrEmpty(request.CallbackMethod))
            {
                ExecuteCallback(requestId, request.UserContract, request.CallbackMethod, null, false, reason);
            }
        }

        private static void ExecuteCallback(BigInteger requestId, UInt160 userContract, string method, byte[] result, bool success, string error)
        {
            bool callbackSuccess = false;
            try
            {
                Contract.Call(userContract, method, CallFlags.All, new object[] { requestId, success, result, error });
                callbackSuccess = true;
            }
            catch
            {
                callbackSuccess = false;
            }

            OnCallbackExecuted(requestId, userContract, method, callbackSuccess);
        }

        public static RequestData GetRequest(BigInteger requestId)
        {
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            ByteString data = requestMap.Get(requestId.ToByteArray());
            if (data == null) return null;
            return (RequestData)StdLib.Deserialize(data);
        }

        public static BigInteger GetRequestCount()
        {
            byte[] key = new byte[] { PREFIX_REQUEST_COUNT };
            return (BigInteger)Storage.Get(Storage.CurrentContext, key);
        }

        private static void SaveRequest(BigInteger requestId, RequestData request)
        {
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Put(requestId.ToByteArray(), StdLib.Serialize(request));
        }

        private static BigInteger GetNextRequestId()
        {
            byte[] key = new byte[] { PREFIX_REQUEST_COUNT };
            BigInteger id = (BigInteger)Storage.Get(Storage.CurrentContext, key);
            id += 1;
            Storage.Put(Storage.CurrentContext, key, id);
            return id;
        }
    }
}
