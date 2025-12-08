using Neo;
using System.Numerics;

namespace ServiceLayer.Automation
{
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
