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
        // Trigger Registration (Called by Gateway)
        // ============================================================================

        /// <summary>
        /// Called by Gateway when user requests automation service.
        /// Registers a new trigger based on the payload.
        /// </summary>
        public static void OnRequest(BigInteger requestId, UInt160 userContract, byte[] payload)
        {
            RequireGateway();
            RequireNotPaused();

            // Parse trigger configuration from payload
            TriggerConfig config = (TriggerConfig)StdLib.Deserialize((ByteString)payload);

            if (config.TargetContract == null || !config.TargetContract.IsValid)
                throw new Exception("Invalid target contract");
            if (string.IsNullOrEmpty(config.CallbackMethod))
                throw new Exception("Invalid callback method");
            if (config.TriggerType < TRIGGER_TIME || config.TriggerType > TRIGGER_THRESHOLD)
                throw new Exception("Invalid trigger type");
            if (string.IsNullOrEmpty(config.Condition))
                throw new Exception("Invalid condition");

            // Get trigger owner (the user contract or its owner)
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            UInt160 owner = tx.Sender;

            // Generate trigger ID
            BigInteger triggerId = GetNextTriggerId();

            // Create trigger
            Trigger trigger = new Trigger
            {
                TriggerId = triggerId,
                RequestId = requestId,
                Owner = owner,
                TargetContract = config.TargetContract,
                CallbackMethod = config.CallbackMethod,
                TriggerType = config.TriggerType,
                Condition = config.Condition,
                CallbackData = config.CallbackData,
                MaxExecutions = config.MaxExecutions > 0 ? config.MaxExecutions : 0, // 0 = unlimited
                ExecutionCount = 0,
                Status = STATUS_ACTIVE,
                CreatedAt = Runtime.Time,
                LastExecutedAt = 0,
                ExpiresAt = config.ExpiresAt
            };

            SaveTrigger(triggerId, trigger);

            OnTriggerRegistered(triggerId, owner, config.TargetContract, config.TriggerType, config.Condition);
        }

        // ============================================================================
        // Trigger Management (Called by Owner)
        // ============================================================================

        /// <summary>Pause a trigger</summary>
        public static void PauseTrigger(BigInteger triggerId)
        {
            Trigger trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");

            if (!Runtime.CheckWitness(trigger.Owner) && !IsAdmin())
                throw new Exception("Not authorized");

            if (trigger.Status != STATUS_ACTIVE) throw new Exception("Trigger not active");

            trigger.Status = STATUS_PAUSED;
            SaveTrigger(triggerId, trigger);

            OnTriggerPaused(triggerId);
        }

        /// <summary>Resume a paused trigger</summary>
        public static void ResumeTrigger(BigInteger triggerId)
        {
            Trigger trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");

            if (!Runtime.CheckWitness(trigger.Owner) && !IsAdmin())
                throw new Exception("Not authorized");

            if (trigger.Status != STATUS_PAUSED) throw new Exception("Trigger not paused");

            trigger.Status = STATUS_ACTIVE;
            SaveTrigger(triggerId, trigger);

            OnTriggerResumed(triggerId);
        }

        /// <summary>Cancel a trigger permanently</summary>
        public static void CancelTrigger(BigInteger triggerId)
        {
            Trigger trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");

            if (!Runtime.CheckWitness(trigger.Owner) && !IsAdmin())
                throw new Exception("Not authorized");

            trigger.Status = STATUS_CANCELLED;
            SaveTrigger(triggerId, trigger);

            OnTriggerCancelled(triggerId);
        }
    }
}
