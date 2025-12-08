using Neo;
using System.Numerics;

namespace ServiceLayer.Mixer
{
    // ============================================================================
    // Data Structures
    // ============================================================================

    /// <summary>
    /// Mixing service data.
    /// </summary>
    public class ServiceData
    {
        public byte[] ServiceId;
        public ECPoint TeePubKey;            // TEE public key for signature verification
        public BigInteger BondAmount;        // Total bond deposited
        public BigInteger OutstandingAmount; // Amount at risk (pending disputes)
        public byte Status;                  // 0=suspended, 1=active
        public ulong RegisteredAt;
    }

    /// <summary>
    /// Dispute record - only created when user disputes off-chain mix.
    /// </summary>
    public class DisputeRecord
    {
        public byte[] RequestHash;           // Hash of original request
        public UInt160 User;                 // User who submitted dispute
        public BigInteger Amount;            // Mix amount being disputed
        public byte[] RequestProof;          // TEE signature from original request
        public byte[] ServiceId;             // Service ID for targeted slashing
        public ulong SubmittedAt;            // When dispute was submitted
        public ulong Deadline;               // TEE must respond by this time
        public byte Status;                  // DISPUTE_PENDING/RESOLVED/REFUNDED
        public byte[] CompletionProof;       // TEE's completion proof (if resolved)
        public ulong ResolvedAt;             // When dispute was resolved/refunded
    }
}
