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
    public partial class AutomationService : SmartContract
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
    }
}
