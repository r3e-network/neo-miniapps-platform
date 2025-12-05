using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Automation
{
    /// <summary>
    /// AutomationService - Task automation and scheduling contract.
    ///
    /// This contract supports multiple trigger types:
    /// 1. CRON - Cron expression based scheduling
    /// 2. INTERVAL - Fixed interval execution
    /// 3. EVENT - Event-based triggers (when specific conditions are met)
    /// 4. ONCE - One-time scheduled execution
    ///
    /// Flow:
    /// 1. User Contract -> AutomationService.CreateTrigger() via Gateway
    /// 2. Service Layer monitors triggers and checks conditions
    /// 3. When trigger fires: Service Layer -> Gateway -> Target Contract
    /// </summary>
    [DisplayName("AutomationService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Task Automation Service - Scheduled and event-based triggers")]
    [ContractPermission("*", "*")]
    public class AutomationService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_TRIGGER = 0x10;
        private const byte PREFIX_OWNER_TRIGGERS = 0x20;
        private const byte PREFIX_EXECUTION_LOG = 0x30;
        private const byte PREFIX_PAUSED = 0x40;
        private const byte PREFIX_TRIGGER_COUNTER = 0x50;

        // Trigger types
        private const byte TRIGGER_CRON = 0;
        private const byte TRIGGER_INTERVAL = 1;
        private const byte TRIGGER_EVENT = 2;
        private const byte TRIGGER_ONCE = 3;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a new trigger is created.
        /// Service Layer monitors this to register the trigger.
        /// </summary>
        [DisplayName("TriggerCreated")]
        public static event Action<ByteString, UInt160, byte, UInt160, string, ByteString, BigInteger> OnTriggerCreated;
        // Parameters: triggerId, owner, triggerType, target, method, schedule, gasLimit

        /// <summary>
        /// Emitted when a trigger is executed.
        /// </summary>
        [DisplayName("TriggerExecuted")]
        public static event Action<ByteString, BigInteger, bool, BigInteger> OnTriggerExecuted;
        // Parameters: triggerId, executionId, success, gasUsed

        /// <summary>
        /// Emitted when a trigger is cancelled.
        /// </summary>
        [DisplayName("TriggerCancelled")]
        public static event Action<ByteString, UInt160> OnTriggerCancelled;

        /// <summary>
        /// Emitted when a trigger is paused/resumed.
        /// </summary>
        [DisplayName("TriggerStatusChanged")]
        public static event Action<ByteString, bool> OnTriggerStatusChanged;

        // ==================== Admin Methods ====================

        public static UInt160 GetAdmin()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
            return stored != null ? (UInt160)stored : InitialAdmin;
        }

        public static void SetAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (!newAdmin.IsValid) throw new Exception("Invalid admin");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        public static UInt160 GetGateway()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            if (!gateway.IsValid) throw new Exception("Invalid gateway");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ==================== Trigger Management ====================

        /// <summary>
        /// Creates a new automation trigger. Called via Gateway.
        ///
        /// The trigger will be monitored by the Service Layer and executed
        /// according to its schedule or conditions.
        /// </summary>
        /// <param name="triggerType">Type: 0=Cron, 1=Interval, 2=Event, 3=Once</param>
        /// <param name="target">Contract to call when triggered</param>
        /// <param name="method">Method to call on target</param>
        /// <param name="args">Arguments to pass (serialized)</param>
        /// <param name="schedule">Cron expression, interval in seconds, or event filter</param>
        /// <param name="gasLimit">Max gas per execution</param>
        /// <param name="maxExecutions">Max number of executions (0 = unlimited)</param>
        /// <returns>Trigger ID</returns>
        public static ByteString CreateTrigger(
            byte triggerType,
            UInt160 target,
            string method,
            ByteString args,
            ByteString schedule,
            BigInteger gasLimit,
            BigInteger maxExecutions)
        {
            RequireGateway();
            RequireNotPaused();

            // Validate inputs
            if (triggerType > 3) throw new Exception("Invalid trigger type");
            if (!target.IsValid) throw new Exception("Invalid target");
            if (string.IsNullOrEmpty(method)) throw new Exception("Invalid method");
            if (gasLimit <= 0) throw new Exception("Invalid gas limit");

            // Get owner (the original requester, passed through gateway)
            var owner = Runtime.CallingScriptHash; // Gateway is calling, but we track the original owner

            // Generate trigger ID
            var triggerId = GenerateTriggerId();

            // Create trigger
            var trigger = new TriggerData
            {
                TriggerId = triggerId,
                Owner = owner,
                TriggerType = triggerType,
                Target = target,
                Method = method,
                Args = args,
                Schedule = schedule,
                GasLimit = gasLimit,
                MaxExecutions = maxExecutions,
                ExecutionCount = 0,
                Active = true,
                CreatedAt = Runtime.Time,
                LastExecutedAt = 0,
                NextExecutionAt = 0 // Service Layer will calculate this
            };

            // Store trigger
            StoreTrigger(triggerId, trigger);

            // Add to owner's trigger list
            AddOwnerTrigger(owner, triggerId);

            OnTriggerCreated(triggerId, owner, triggerType, target, method, schedule, gasLimit);

            return triggerId;
        }

        /// <summary>
        /// Executes a trigger. Only callable by Gateway (from Service Layer).
        ///
        /// The Service Layer determines when to execute based on:
        /// - Cron: Matches cron schedule
        /// - Interval: Time since last execution >= interval
        /// - Event: Matching event detected
        /// - Once: Scheduled time reached
        /// </summary>
        public static void ExecuteTrigger(ByteString triggerId, ByteString signature)
        {
            RequireGateway();
            RequireNotPaused();

            var trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");
            if (!trigger.Active) throw new Exception("Trigger not active");

            // Check max executions
            if (trigger.MaxExecutions > 0 && trigger.ExecutionCount >= trigger.MaxExecutions)
            {
                trigger.Active = false;
                StoreTrigger(triggerId, trigger);
                throw new Exception("Max executions reached");
            }

            // Generate execution ID
            var executionId = trigger.ExecutionCount + 1;

            // Execute the target contract
            bool success = true;
            try
            {
                Contract.Call(trigger.Target, trigger.Method, CallFlags.All, new object[] { triggerId, trigger.Args });
            }
            catch
            {
                success = false;
            }

            // Update trigger
            trigger.ExecutionCount = executionId;
            trigger.LastExecutedAt = Runtime.Time;

            // Deactivate one-time triggers after execution
            if (trigger.TriggerType == TRIGGER_ONCE)
            {
                trigger.Active = false;
            }

            StoreTrigger(triggerId, trigger);

            // Log execution
            var log = new ExecutionLog
            {
                TriggerId = triggerId,
                ExecutionId = executionId,
                Success = success,
                ExecutedAt = Runtime.Time,
                GasUsed = 0 // TODO: Track actual gas
            };
            StoreExecutionLog(triggerId, executionId, log);

            OnTriggerExecuted(triggerId, executionId, success, 0);
        }

        /// <summary>
        /// Cancels a trigger. Only callable by owner or admin.
        /// </summary>
        public static void CancelTrigger(ByteString triggerId)
        {
            var trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");

            // Check authorization
            var caller = Runtime.CallingScriptHash;
            if (caller != trigger.Owner && !Runtime.CheckWitness(GetAdmin()))
                throw new Exception("Not authorized");

            // Mark as inactive
            trigger.Active = false;
            StoreTrigger(triggerId, trigger);

            OnTriggerCancelled(triggerId, trigger.Owner);
        }

        /// <summary>
        /// Pauses a trigger. Only callable by owner.
        /// </summary>
        public static void PauseTrigger(ByteString triggerId)
        {
            var trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");
            if (Runtime.CallingScriptHash != trigger.Owner)
                throw new Exception("Not authorized");

            trigger.Active = false;
            StoreTrigger(triggerId, trigger);
            OnTriggerStatusChanged(triggerId, false);
        }

        /// <summary>
        /// Resumes a trigger. Only callable by owner.
        /// </summary>
        public static void ResumeTrigger(ByteString triggerId)
        {
            var trigger = GetTrigger(triggerId);
            if (trigger == null) throw new Exception("Trigger not found");
            if (Runtime.CallingScriptHash != trigger.Owner)
                throw new Exception("Not authorized");

            // Check max executions not reached
            if (trigger.MaxExecutions > 0 && trigger.ExecutionCount >= trigger.MaxExecutions)
                throw new Exception("Max executions reached");

            trigger.Active = true;
            StoreTrigger(triggerId, trigger);
            OnTriggerStatusChanged(triggerId, true);
        }

        // ==================== Query Methods ====================

        /// <summary>
        /// Gets a trigger by ID.
        /// </summary>
        public static TriggerData GetTrigger(ByteString triggerId)
        {
            var key = GetTriggerKey(triggerId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (TriggerData)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets execution log for a trigger.
        /// </summary>
        public static ExecutionLog GetExecutionLog(ByteString triggerId, BigInteger executionId)
        {
            var key = GetExecutionLogKey(triggerId, executionId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (ExecutionLog)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Checks if a trigger is active.
        /// </summary>
        public static bool IsTriggerActive(ByteString triggerId)
        {
            var trigger = GetTrigger(triggerId);
            return trigger != null && trigger.Active;
        }

        // ==================== Helper Methods ====================

        private static void RequireAdmin()
        {
            if (!Runtime.CheckWitness(GetAdmin()))
                throw new Exception("Only admin");
        }

        private static void RequireGateway()
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }

        private static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract paused");
        }

        private static ByteString GenerateTriggerId()
        {
            var counterKey = new byte[] { PREFIX_TRIGGER_COUNTER };
            var counter = Storage.Get(Storage.CurrentContext, counterKey);
            BigInteger counterValue = counter != null ? (BigInteger)counter : 0;
            counterValue++;
            Storage.Put(Storage.CurrentContext, counterKey, counterValue);

            var data = Helper.Concat((ByteString)Runtime.ExecutingScriptHash, (ByteString)counterValue);
            BigInteger timeValue = Runtime.Time;
            data = Helper.Concat(data, (ByteString)timeValue);
            return CryptoLib.Sha256(data);
        }

        private static byte[] GetTriggerKey(ByteString triggerId)
        {
            return Helper.Concat(new byte[] { PREFIX_TRIGGER }, triggerId);
        }

        private static byte[] GetExecutionLogKey(ByteString triggerId, BigInteger executionId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_EXECUTION_LOG }, triggerId);
            return Helper.Concat(key, (ByteString)executionId);
        }

        private static void StoreTrigger(ByteString triggerId, TriggerData trigger)
        {
            var key = GetTriggerKey(triggerId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(trigger));
        }

        private static void StoreExecutionLog(ByteString triggerId, BigInteger executionId, ExecutionLog log)
        {
            var key = GetExecutionLogKey(triggerId, executionId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(log));
        }

        private static void AddOwnerTrigger(UInt160 owner, ByteString triggerId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_OWNER_TRIGGERS }, (ByteString)owner);
            key = Helper.Concat(key, triggerId);
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class TriggerData
    {
        public ByteString TriggerId;
        public UInt160 Owner;
        public byte TriggerType;      // 0=Cron, 1=Interval, 2=Event, 3=Once
        public UInt160 Target;
        public string Method;
        public ByteString Args;
        public ByteString Schedule;   // Cron expr, interval seconds, event filter, or timestamp
        public BigInteger GasLimit;
        public BigInteger MaxExecutions;
        public BigInteger ExecutionCount;
        public bool Active;
        public BigInteger CreatedAt;
        public BigInteger LastExecutedAt;
        public BigInteger NextExecutionAt;
    }

    public class ExecutionLog
    {
        public ByteString TriggerId;
        public BigInteger ExecutionId;
        public bool Success;
        public BigInteger ExecutedAt;
        public BigInteger GasUsed;
    }
}
