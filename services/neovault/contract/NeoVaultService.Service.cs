using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Mixer
{
    public partial class NeoVaultService
    {
        // ============================================================================
        // Service Registration & Bond Management
        // ============================================================================

        /// <summary>
        /// Register a mixing service with TEE public key.
        /// The TEE public key is used to verify dispute resolution signatures.
        /// </summary>
        public static void RegisterService(byte[] serviceId, ECPoint teePubKey)
        {
            RequireAdmin();
            if (serviceId == null || serviceId.Length == 0) throw new Exception("Invalid serviceId");
            if (teePubKey == null) throw new Exception("Invalid TEE public key");

            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceId);
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new Exception("Service already exists");

            ServiceData service = new ServiceData
            {
                ServiceId = serviceId,
                TeePubKey = teePubKey,
                BondAmount = 0,
                OutstandingAmount = 0,
                Status = 1,
                RegisteredAt = Runtime.Time
            };

            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(service));
            OnServiceRegistered(serviceId, teePubKey);
        }

        /// <summary>
        /// Handle incoming GAS payments for bond deposits.
        /// Note: Mix requests are handled off-chain - users deposit directly to anonymous pool accounts.
        /// No on-chain request creation. Fee is deducted from delivery (privacy-first model).
        /// </summary>
        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash)
                throw new Exception("Only GAS accepted");

            if (data == null) throw new Exception("Missing data");

            object[] dataArray = (object[])data;
            string operation = (string)dataArray[0];

            if (operation == "depositBond")
            {
                byte[] serviceId = (byte[])dataArray[1];
                DepositBondInternal(serviceId, amount);
            }
            else if (operation == "submitDispute")
            {
                // User submits dispute with GAS amount matching their mix request
                byte[] requestHash = (byte[])dataArray[1];
                byte[] requestProof = (byte[])dataArray[2];
                byte[] serviceId = (byte[])dataArray[3];
                SubmitDisputeInternal(from, amount, requestHash, requestProof, serviceId);
            }
            else
            {
                throw new Exception("Unknown operation");
            }
        }

        private static void DepositBondInternal(byte[] serviceId, BigInteger amount)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceId);
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) throw new Exception("Service not found");

            ServiceData service = (ServiceData)StdLib.Deserialize((ByteString)data);
            service.BondAmount += amount;

            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(service));
            OnBondDeposited(serviceId, amount, service.BondAmount);
        }
    }
}
