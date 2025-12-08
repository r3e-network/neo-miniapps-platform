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
    /// AutomationService - Trigger-based Automation Service
    ///
    /// This contract implements Pattern 3: Trigger-Based
    /// - Users register triggers with conditions
    /// - TEE monitors conditions continuously
    /// - When conditions are met, TEE executes callbacks
    ///
    /// Trigger Types:
    /// 1. Time-based: Cron expressions (e.g., "Every Friday 00:00 UTC")
    /// 2. Price-based: Price thresholds (e.g., "When BTC > $100,000")
    /// 3. Event-based: On-chain events (e.g., "When contract X emits event Y")
    /// 4. Threshold-based: Balance/value thresholds (e.g., "When balance < 10 GAS")
    ///
    /// Flow:
    /// 1. User registers trigger via Gateway.RequestService("automation", ...)
    /// 2. This contract stores trigger configuration
    /// 3. TEE monitors all registered triggers
    /// 4. When condition met, TEE calls ExecuteTrigger()
    /// 5. Contract executes callback to user contract
    ///
    /// Security:
    /// - Only registered TEE accounts can execute triggers
    /// - Users prepay for executions via GasBank
    /// - Triggers can be paused/cancelled by owner
    /// </summary>
    [DisplayName("AutomationService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Trigger-based Automation Service")]
    [ManifestExtra("Version", "1.0.0")]
    [ContractPermission("*", "*")]
    public class AutomationService : SmartContract
    {
        // ============================================================================
        // Storage Prefixes
        // ============================================================================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_PAUSED = 0x02;
        private const byte PREFIX_GATEWAY = 0x03;
        private const byte PREFIX_TEE_ACCOUNT = 0x10;
        private const byte PREFIX_TEE_PUBKEY = 0x11;
        private const byte PREFIX_TRIGGER = 0x20;
        private const byte PREFIX_TRIGGER_COUNT = 0x21;
        private const byte PREFIX_USER_TRIGGERS = 0x22;
        private const byte PREFIX_NONCE = 0x30;
        private const byte PREFIX_EXECUTION = 0x40;

        // ============================================================================
        // Constants
        // ============================================================================

        // Trigger types
        public const byte TRIGGER_TIME = 1;      // Cron-based time trigger
        public const byte TRIGGER_PRICE = 2;     // Price threshold trigger
        public const byte TRIGGER_EVENT = 3;     // On-chain event trigger
        public const byte TRIGGER_THRESHOLD = 4; // Balance/value threshold

        // Trigger status
        public const byte STATUS_ACTIVE = 1;
        public const byte STATUS_PAUSED = 2;
        public const byte STATUS_CANCELLED = 3;
        public const byte STATUS_EXPIRED = 4;

        // Maximum triggers per user
        public static readonly int MAX_TRIGGERS_PER_USER = 100;

        // ============================================================================
        // Events
        // ============================================================================

        /// <summary>New trigger registered</summary>
        [DisplayName("TriggerRegistered")]
        public static event Action<BigInteger, UInt160, UInt160, byte, string> OnTriggerRegistered;
        // triggerId, owner, targetContract, triggerType, condition

        /// <summary>Trigger executed by TEE</summary>
        [DisplayName("TriggerExecuted")]
        public static event Action<BigInteger, UInt160, bool, ulong> OnTriggerExecuted;
        // triggerId, targetContract, success, timestamp

        /// <summary>Trigger paused</summary>
        [DisplayName("TriggerPaused")]
        public static event Action<BigInteger> OnTriggerPaused;

        /// <summary>Trigger resumed</summary>
        [DisplayName("TriggerResumed")]
        public static event Action<BigInteger> OnTriggerResumed;

        /// <summary>Trigger cancelled</summary>
        [DisplayName("TriggerCancelled")]
        public static event Action<BigInteger> OnTriggerCancelled;

        /// <summary>TEE account registered</summary>
        [DisplayName("TEERegistered")]
        public static event Action<UInt160, ECPoint> OnTEERegistered;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, tx.Sender);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest);
        }

        // ============================================================================
        // Admin Management
        // ============================================================================

        private static UInt160 GetAdmin() => (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
        private static bool IsAdmin() => Runtime.CheckWitness(GetAdmin());
        private static void RequireAdmin() { if (!IsAdmin()) throw new Exception("Admin only"); }

        public static UInt160 Admin() => GetAdmin();

        public static void TransferAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (newAdmin == null || !newAdmin.IsValid) throw new Exception("Invalid address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        // ============================================================================
        // Pause Control
        // ============================================================================

        private static bool IsPaused() => (BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }) == 1;
        private static void RequireNotPaused() { if (IsPaused()) throw new Exception("Contract paused"); }
        public static bool Paused() => IsPaused();
        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ============================================================================
        // Gateway Management
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static UInt160 GetGateway()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
        }

        private static void RequireGateway()
        {
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }

        // ============================================================================
        // TEE Account Management
        // ============================================================================

        public static void RegisterTEEAccount(UInt160 teeAccount, ECPoint teePubKey)
        {
            RequireAdmin();
            if (teeAccount == null || !teeAccount.IsValid) throw new Exception("Invalid TEE account");
            if (teePubKey == null) throw new Exception("Invalid public key");

            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Put(Storage.CurrentContext, accountKey, 1);
            Storage.Put(Storage.CurrentContext, pubKeyKey, teePubKey);

            OnTEERegistered(teeAccount, teePubKey);
        }

        public static void RemoveTEEAccount(UInt160 teeAccount)
        {
            RequireAdmin();
            byte[] accountKey = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])teeAccount);
            byte[] pubKeyKey = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);

            Storage.Delete(Storage.CurrentContext, accountKey);
            Storage.Delete(Storage.CurrentContext, pubKeyKey);
        }

        public static bool IsTEEAccount(UInt160 account)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_ACCOUNT }, (byte[])account);
            return (BigInteger)Storage.Get(Storage.CurrentContext, key) == 1;
        }

        public static ECPoint GetTEEPublicKey(UInt160 teeAccount)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_TEE_PUBKEY }, (byte[])teeAccount);
            return (ECPoint)Storage.Get(Storage.CurrentContext, key);
        }

        private static void RequireTEE()
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            if (!IsTEEAccount(tx.Sender)) throw new Exception("TEE account only");
        }

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

    // ============================================================================
    // Data Structures
    // ============================================================================

    /// <summary>Trigger configuration (passed in payload)</summary>
    public class TriggerConfig
    {
        public UInt160 TargetContract;      // Contract to call when triggered
        public string CallbackMethod;       // Method to call
        public byte TriggerType;            // TRIGGER_TIME, TRIGGER_PRICE, etc.
        public string Condition;            // Condition expression (cron, price threshold, etc.)
        public byte[] CallbackData;         // Optional data to pass to callback
        public BigInteger MaxExecutions;    // Max times to execute (0 = unlimited)
        public ulong ExpiresAt;             // Expiration timestamp (0 = never)
    }

    /// <summary>Stored trigger data</summary>
    public class Trigger
    {
        public BigInteger TriggerId;
        public BigInteger RequestId;        // Original request ID from Gateway
        public UInt160 Owner;               // Trigger owner
        public UInt160 TargetContract;      // Contract to call
        public string CallbackMethod;       // Method to call
        public byte TriggerType;            // Type of trigger
        public string Condition;            // Condition expression
        public byte[] CallbackData;         // Data to pass to callback
        public BigInteger MaxExecutions;    // Max executions (0 = unlimited)
        public BigInteger ExecutionCount;   // Times executed
        public byte Status;                 // Current status
        public ulong CreatedAt;             // Creation timestamp
        public ulong LastExecutedAt;        // Last execution timestamp
        public ulong ExpiresAt;             // Expiration timestamp
    }

    /// <summary>Execution record</summary>
    public class ExecutionRecord
    {
        public BigInteger TriggerId;
        public BigInteger ExecutionNumber;
        public ulong Timestamp;
        public bool Success;
        public UInt160 ExecutedBy;          // TEE account that executed
    }
}
