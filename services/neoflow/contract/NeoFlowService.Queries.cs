using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Automation
{
    public partial class NeoFlowService
    {
        // ============================================================================
        // Query Functions
        // ============================================================================

        /// <summary>Get trigger by ID</summary>
        public static Trigger GetTrigger(BigInteger triggerId)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TRIGGER }, triggerId.ToByteArray());
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (Trigger)StdLib.Deserialize((ByteString)data);
        }

        /// <summary>Get execution record</summary>
        public static ExecutionRecord GetExecution(BigInteger triggerId, BigInteger executionNumber)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_EXECUTION }, triggerId.ToByteArray());
            key = Helper.Concat(key, executionNumber.ToByteArray());
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (ExecutionRecord)StdLib.Deserialize((ByteString)data);
        }

        /// <summary>Check if trigger is active and can be executed</summary>
        public static bool CanExecute(BigInteger triggerId)
        {
            Trigger trigger = GetTrigger(triggerId);
            if (trigger == null) return false;
            if (trigger.Status != STATUS_ACTIVE) return false;
            if (trigger.ExpiresAt > 0 && Runtime.Time > trigger.ExpiresAt) return false;
            if (trigger.MaxExecutions > 0 && trigger.ExecutionCount >= trigger.MaxExecutions) return false;
            return true;
        }

        // ============================================================================
        // Internal Helpers
        // ============================================================================

        private static void SaveTrigger(BigInteger triggerId, Trigger trigger)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TRIGGER }, triggerId.ToByteArray());
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(trigger));
        }

        private static BigInteger GetNextTriggerId()
        {
            byte[] key = new byte[] { PREFIX_TRIGGER_COUNT };
            BigInteger id = (BigInteger)Storage.Get(Storage.CurrentContext, key);
            id += 1;
            Storage.Put(Storage.CurrentContext, key, id);
            return id;
        }

        private static void VerifyAndMarkNonce(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new Exception("Nonce already used");
            Storage.Put(Storage.CurrentContext, key, 1);
        }
    }
}
