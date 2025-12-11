using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Mixer
{
    /// <summary>
    /// NeoVaultService v5.1 - Privacy-First Off-Chain Mixing with On-Chain Dispute Resolution
    ///
    /// Architecture: Off-Chain Mixing with On-Chain Dispute Only
    /// - User requests mix via CLI/API → Mixer service directly (NO on-chain)
    /// - Mixer returns RequestProof (requestHash + TEE signature) + deposit address
    /// - User deposits DIRECTLY to pool account on-chain (NOT gasbank, NOT any known service layer address)
    /// - Mixer processes off-chain (HD pool accounts, random mixing)
    /// - When done, Mixer delivers NetAmount (TotalAmount - Fee) to target addresses
    /// - Fee remains in pool accounts (no explicit fee transfer to any known address)
    /// - Normal path: User happy, nothing on-chain links user to service layer
    /// - Dispute path: User submits dispute → TEE submits CompletionProof on-chain
    ///
    /// Privacy-First Fee Model:
    /// - User deposits TotalAmount to anonymous pool account
    /// - User receives NetAmount (TotalAmount - ServiceFee) at target addresses
    /// - ServiceFee stays distributed in pool (no explicit fee collection address)
    /// - User NEVER connects to any known service layer account
    ///
    /// Contract Role (Minimal):
    /// - Service registration and bond management
    /// - Dispute submission by user
    /// - Dispute resolution by TEE (completion proof)
    /// - Refund if TEE fails to resolve within deadline
    /// </summary>
    [DisplayName("NeoVaultService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Off-Chain Privacy Mixer with On-Chain Dispute Resolution")]
    [ManifestExtra("Version", "5.0.0")]
    [ContractPermission("*", "*")]
    public partial class NeoVaultService : SmartContract
    {
        // ============================================================================
        // Storage Prefixes
        // ============================================================================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_PAUSED = 0x02;
        private const byte PREFIX_SERVICE = 0x10;
        private const byte PREFIX_DISPUTE = 0x20;
        private const byte PREFIX_RESOLVED = 0x21;
        private const byte PREFIX_NONCE = 0x30;

        // ============================================================================
        // Constants
        // ============================================================================

        // Minimum bond required (10 GAS)
        public static readonly BigInteger MIN_BOND = 10_00000000;

        // Dispute resolution deadline (7 days in milliseconds)
        public static readonly ulong DISPUTE_DEADLINE = 7 * 24 * 60 * 60 * 1000;

        // Dispute status
        public const byte DISPUTE_PENDING = 0;   // User submitted, waiting for TEE
        public const byte DISPUTE_RESOLVED = 1;  // TEE submitted completion proof
        public const byte DISPUTE_REFUNDED = 2;  // TEE failed, user refunded

        // ============================================================================
        // Events
        // ============================================================================

        /// <summary>Service registered with TEE public key</summary>
        [DisplayName("ServiceRegistered")]
        public static event Action<byte[], ECPoint> OnServiceRegistered;

        /// <summary>Bond deposited by service</summary>
        [DisplayName("BondDeposited")]
        public static event Action<byte[], BigInteger, BigInteger> OnBondDeposited;

        /// <summary>User submitted dispute for an off-chain mix request</summary>
        [DisplayName("DisputeSubmitted")]
        public static event Action<byte[], UInt160, BigInteger, ulong> OnDisputeSubmitted;
        // requestHash, user, amount, deadline

        /// <summary>TEE resolved dispute with completion proof</summary>
        [DisplayName("DisputeResolved")]
        public static event Action<byte[], byte[], byte[]> OnDisputeResolved;
        // requestHash, serviceId, completionProof

        /// <summary>User refunded after dispute deadline passed</summary>
        [DisplayName("DisputeRefunded")]
        public static event Action<byte[], UInt160, BigInteger> OnDisputeRefunded;

        /// <summary>Bond slashed due to service failure</summary>
        [DisplayName("BondSlashed")]
        public static event Action<byte[], BigInteger, BigInteger> OnBondSlashed;

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
