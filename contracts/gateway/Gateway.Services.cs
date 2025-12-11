using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System;

namespace ServiceLayer.Gateway
{
    public partial class ServiceLayerGateway
    {
        public static void RegisterService(string serviceType, UInt160 serviceContract)
        {
            RequireAdmin();
            if (string.IsNullOrEmpty(serviceType)) throw new Exception("Invalid service type");
            if (serviceContract == null || !serviceContract.IsValid) throw new Exception("Invalid contract");

            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceType.ToByteArray());
            Storage.Put(Storage.CurrentContext, key, serviceContract);

            OnServiceRegistered(serviceType, serviceContract);
        }

        public static void RemoveService(string serviceType)
        {
            RequireAdmin();
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceType.ToByteArray());
            Storage.Delete(Storage.CurrentContext, key);
        }

        public static UInt160 GetServiceContract(string serviceType)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceType.ToByteArray());
            return (UInt160)Storage.Get(Storage.CurrentContext, key);
        }
    }
}
