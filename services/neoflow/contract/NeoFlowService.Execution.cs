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
        // Trigger Execution (Called by TEE)
        // ============================================================================

        /// <summary>
        /// Execute a trigger when its condition is met.
        /// Called by TEE after monitoring and detecting condition.
        /// </summary>
        /// <param name="triggerId">Trigger to execute</param>
        /// <param name="executionData">Data to pass to callback (e.g., current price)</param>
        /// <param name="nonce">Nonce for replay protection</param>
        /// <param name="signature">TEE signature</param>
        public static void ExecuteTrigger(BigInteger triggerId, byte[] executionData, BigInteger nonce, byte[] signature)
        {
            RequireNotPaused();
            RequireTEE();

            // Get trigger
            Trigger trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");
            if (trigger.Status != STATUS_ACTIVE) throw new Exception("Trigger not active");

            // Check expiration
            if (trigger.ExpiresAt > 0 && Runtime.Time > trigger.ExpiresAt)
            {
                trigger.Status = STATUS_EXPIRED;
                SaveTrigger(triggerId, trigger);
                throw new Exception("Trigger expired");
            }

            // Check max executions
            if (trigger.MaxExecutions > 0 && trigger.ExecutionCount >= trigger.MaxExecutions)
            {
                trigger.Status = STATUS_EXPIRED;
                SaveTrigger(triggerId, trigger);
                throw new Exception("Max executions reached");
            }

            // Verify nonce
            VerifyAndMarkNonce(nonce);

            // Verify TEE signature
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            ECPoint teePubKey = GetTEEPublicKey(tx.Sender);

            byte[] message = Helper.Concat(triggerId.ToByteArray(), executionData);
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, teePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Execute callback
            bool success = false;
            try
            {
                // Prepare callback data
                byte[] callbackPayload = trigger.CallbackData != null
                    ? Helper.Concat(trigger.CallbackData, executionData)
                    : executionData;

                Contract.Call(trigger.TargetContract, trigger.CallbackMethod, CallFlags.All,
                    new object[] { triggerId, callbackPayload });
                success = true;
            }
            catch
            {
                success = false;
            }

            // Update trigger
            trigger.ExecutionCount += 1;
            trigger.LastExecutedAt = Runtime.Time;

            // Check if max executions reached after this execution
            if (trigger.MaxExecutions > 0 && trigger.ExecutionCount >= trigger.MaxExecutions)
            {
                trigger.Status = STATUS_EXPIRED;
            }

            SaveTrigger(triggerId, trigger);

            // Record execution
            ExecutionRecord record = new ExecutionRecord
            {
                TriggerId = triggerId,
                ExecutionNumber = trigger.ExecutionCount,
                Timestamp = Runtime.Time,
                Success = success,
                ExecutedBy = tx.Sender
            };

            byte[] execKey = Helper.Concat(new byte[] { PREFIX_EXECUTION }, triggerId.ToByteArray());
            execKey = Helper.Concat(execKey, trigger.ExecutionCount.ToByteArray());
            Storage.Put(Storage.CurrentContext, execKey, StdLib.Serialize(record));

            OnTriggerExecuted(triggerId, trigger.TargetContract, success, Runtime.Time);
        }
    }
}
