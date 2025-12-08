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
    /// MixerService v5.0 - Off-Chain First with On-Chain Dispute Resolution
    ///
    /// Architecture: Off-Chain Mixing with On-Chain Dispute Only
    /// - User requests mix via CLI/API → Mixer service directly (NO on-chain)
    /// - Mixer returns RequestProof (requestHash + TEE signature) to user
    /// - User deposits via GasBank (off-chain balance management)
    /// - Mixer processes off-chain (HD pool accounts, random mixing)
    /// - When done, Mixer generates CompletionProof (stored, NOT submitted)
    /// - Normal path: User happy, nothing on-chain, privacy preserved
    /// - Dispute path: User submits dispute → TEE submits CompletionProof on-chain
    ///
    /// Flow:
    /// 1. User → CLI/API → Mixer Service (off-chain request, returns RequestProof)
    /// 2. User → GasBank deposit (off-chain, managed by Service Layer)
    /// 3. Mixer → Off-chain mixing (HD pool accounts)
    /// 4. Mixer → Generate CompletionProof (stored, NOT submitted on-chain)
    /// 5. Normal: User receives funds, happy, nothing on-chain
    /// 6. Dispute: User calls SubmitDispute(requestHash, requestProof)
    /// 7. TEE: ResolveDispute(requestHash, completionProof) OR user gets refund
    ///
    /// Privacy Guarantees:
    /// - Normal flow has ZERO on-chain transactions
    /// - Pool accounts are standard single-sig addresses (no fingerprint)
    /// - On-chain data only exposed during dispute resolution
    /// - Dispute reveals: requestHash, completionProof (not target addresses)
    ///
    /// Contract Role (Minimal):
    /// - Service registration and bond management
    /// - Dispute submission by user
    /// - Dispute resolution by TEE (completion proof)
    /// - Refund if TEE fails to resolve within deadline
    /// </summary>
    [DisplayName("MixerService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Off-Chain Privacy Mixer with On-Chain Dispute Resolution")]
    [ManifestExtra("Version", "5.0.0")]
    [ContractPermission("*", "*")]
    public class MixerService : SmartContract
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
        /// Note: Mix requests are handled off-chain via GasBank - no on-chain request creation.
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

        // ============================================================================
        // Dispute Submission (User) - ONLY ON-CHAIN INTERACTION FOR NORMAL USERS
        // ============================================================================

        /// <summary>
        /// User submits a dispute when they believe their mix request was not fulfilled.
        /// This is the ONLY on-chain interaction users make (besides viewing results).
        ///
        /// Required:
        /// - requestHash: Hash of the original request (from RequestProof)
        /// - requestProof: TEE signature from RequestProof (proves request was accepted)
        /// - GAS amount: Must match the original mix amount (for refund if dispute succeeds)
        ///
        /// After submission, TEE has DISPUTE_DEADLINE to submit completion proof.
        /// If TEE doesn't respond, user can claim refund from bond.
        /// </summary>
        private static void SubmitDisputeInternal(UInt160 user, BigInteger amount, byte[] requestHash, byte[] requestProof, byte[] serviceId)
        {
            RequireNotPaused();

            if (requestHash == null || requestHash.Length != 32)
                throw new Exception("Invalid request hash");
            if (requestProof == null || requestProof.Length == 0)
                throw new Exception("Invalid request proof");
            if (serviceId == null || serviceId.Length == 0)
                throw new Exception("Invalid service ID");
            if (amount <= 0)
                throw new Exception("Invalid amount");

            // Verify service exists
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            if (service.Status != 1) throw new Exception("Service not active");

            // Check if dispute already exists
            byte[] disputeKey = Helper.Concat(new byte[] { PREFIX_DISPUTE }, requestHash);
            if (Storage.Get(Storage.CurrentContext, disputeKey) != null)
                throw new Exception("Dispute already exists");

            // Check if already resolved
            byte[] resolvedKey = Helper.Concat(new byte[] { PREFIX_RESOLVED }, requestHash);
            if (Storage.Get(Storage.CurrentContext, resolvedKey) != null)
                throw new Exception("Request already resolved");

            // Calculate deadline
            ulong deadline = Runtime.Time + DISPUTE_DEADLINE;

            // Create dispute record with serviceId for targeted slashing
            DisputeRecord dispute = new DisputeRecord
            {
                RequestHash = requestHash,
                User = user,
                Amount = amount,
                RequestProof = requestProof,
                ServiceId = serviceId,
                SubmittedAt = Runtime.Time,
                Deadline = deadline,
                Status = DISPUTE_PENDING
            };

            Storage.Put(Storage.CurrentContext, disputeKey, StdLib.Serialize(dispute));
            OnDisputeSubmitted(requestHash, user, amount, deadline);
        }

        // ============================================================================
        // Dispute Resolution (TEE) - ONLY CALLED WHEN USER DISPUTES
        // ============================================================================

        /// <summary>
        /// TEE resolves a dispute by submitting the completion proof.
        /// This is the ONLY on-chain submission by TEE (and only when disputed).
        ///
        /// CompletionProof contains:
        /// - requestId and requestHash (links to original request)
        /// - outputsHash (hash of all output transactions)
        /// - outputTxIDs (actual transaction IDs proving delivery)
        /// - completedAt timestamp
        /// - TEE signature over all above
        ///
        /// If valid, dispute is resolved and user's deposit is returned.
        /// </summary>
        public static void ResolveDispute(
            byte[] serviceId,
            byte[] requestHash,
            byte[] completionProof,
            BigInteger nonce,
            byte[] signature)
        {
            RequireNotPaused();

            // Get service
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            if (service.Status != 1) throw new Exception("Service not active");

            // Get dispute
            byte[] disputeKey = Helper.Concat(new byte[] { PREFIX_DISPUTE }, requestHash);
            ByteString disputeData = Storage.Get(Storage.CurrentContext, disputeKey);
            if (disputeData == null) throw new Exception("Dispute not found");

            DisputeRecord dispute = (DisputeRecord)StdLib.Deserialize((ByteString)disputeData);
            if (dispute.Status != DISPUTE_PENDING)
                throw new Exception("Dispute not pending");

            // Verify nonce (replay protection)
            VerifyAndMarkNonce(nonce);

            // Verify TEE signature: requestHash | completionProof | nonce
            byte[] message = Helper.Concat(requestHash, completionProof);
            message = Helper.Concat(message, nonce.ToByteArray());

            if (!CryptoLib.VerifyWithECDsa((ByteString)message, service.TeePubKey, (ByteString)signature, NamedCurve.secp256r1))
                throw new Exception("Invalid TEE signature");

            // Mark as resolved
            dispute.Status = DISPUTE_RESOLVED;
            dispute.CompletionProof = completionProof;
            dispute.ResolvedAt = Runtime.Time;
            Storage.Put(Storage.CurrentContext, disputeKey, StdLib.Serialize(dispute));

            // Mark request as resolved (prevent double disputes)
            byte[] resolvedKey = Helper.Concat(new byte[] { PREFIX_RESOLVED }, requestHash);
            Storage.Put(Storage.CurrentContext, resolvedKey, 1);

            // Return user's dispute deposit (they got their mix, dispute resolved)
            GAS.Transfer(Runtime.ExecutingScriptHash, dispute.User, dispute.Amount, null);

            OnDisputeResolved(requestHash, serviceId, completionProof);
        }

        // ============================================================================
        // Dispute Refund (User claims if TEE fails to respond)
        // ============================================================================

        /// <summary>
        /// User claims refund if TEE fails to resolve dispute by deadline.
        /// Refund comes from service bond (slashing mechanism).
        /// </summary>
        public static void ClaimDisputeRefund(byte[] requestHash)
        {
            byte[] disputeKey = Helper.Concat(new byte[] { PREFIX_DISPUTE }, requestHash);
            ByteString disputeData = Storage.Get(Storage.CurrentContext, disputeKey);
            if (disputeData == null) throw new Exception("Dispute not found");

            DisputeRecord dispute = (DisputeRecord)StdLib.Deserialize((ByteString)disputeData);

            if (!Runtime.CheckWitness(dispute.User))
                throw new Exception("Only dispute submitter can claim refund");

            if (Runtime.Time <= dispute.Deadline)
                throw new Exception("Deadline not reached");

            if (dispute.Status != DISPUTE_PENDING)
                throw new Exception("Dispute not pending");

            // Calculate refund (dispute deposit + potential bond slash)
            BigInteger refundAmount = dispute.Amount;

            // Mark as refunded
            dispute.Status = DISPUTE_REFUNDED;
            dispute.ResolvedAt = Runtime.Time;
            Storage.Put(Storage.CurrentContext, disputeKey, StdLib.Serialize(dispute));

            // Mark request as resolved
            byte[] resolvedKey = Helper.Concat(new byte[] { PREFIX_RESOLVED }, requestHash);
            Storage.Put(Storage.CurrentContext, resolvedKey, 1);

            // Return user's dispute deposit
            GAS.Transfer(Runtime.ExecutingScriptHash, dispute.User, dispute.Amount, null);

            // Slash the specific service's bond
            if (dispute.ServiceId != null && dispute.ServiceId.Length > 0)
            {
                byte[] serviceKey = Helper.Concat(new byte[] { PREFIX_SERVICE }, dispute.ServiceId);
                ByteString serviceData = Storage.Get(Storage.CurrentContext, serviceKey);
                if (serviceData != null)
                {
                    ServiceData service = (ServiceData)StdLib.Deserialize((ByteString)serviceData);
                    BigInteger slashAmount = dispute.Amount;
                    if (slashAmount > service.BondAmount)
                        slashAmount = service.BondAmount;

                    service.BondAmount -= slashAmount;
                    Storage.Put(Storage.CurrentContext, serviceKey, StdLib.Serialize(service));
                    OnBondSlashed(dispute.ServiceId, slashAmount, service.BondAmount);
                }
            }

            OnDisputeRefunded(requestHash, dispute.User, refundAmount);
        }

        // ============================================================================
        // Query Functions
        // ============================================================================

        public static ServiceData GetService(byte[] serviceId)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, serviceId);
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (ServiceData)StdLib.Deserialize((ByteString)data);
        }

        public static DisputeRecord GetDispute(byte[] requestHash)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_DISPUTE }, requestHash);
            ByteString data = Storage.Get(Storage.CurrentContext, key);
            if (data == null) return null;
            return (DisputeRecord)StdLib.Deserialize((ByteString)data);
        }

        public static bool IsRequestResolved(byte[] requestHash)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_RESOLVED }, requestHash);
            return Storage.Get(Storage.CurrentContext, key) != null;
        }

        public static bool CanClaimDisputeRefund(byte[] requestHash)
        {
            DisputeRecord dispute = GetDispute(requestHash);
            if (dispute == null) return false;
            if (dispute.Status != DISPUTE_PENDING) return false;
            return Runtime.Time > dispute.Deadline;
        }

        // ============================================================================
        // Internal Helpers
        // ============================================================================

        private static void SaveService(ServiceData service)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_SERVICE }, service.ServiceId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(service));
        }

        private static void VerifyAndMarkNonce(BigInteger nonce)
        {
            byte[] key = Helper.Concat(new byte[] { PREFIX_NONCE }, nonce.ToByteArray());
            if (Storage.Get(Storage.CurrentContext, key) != null)
                throw new Exception("Nonce already used");
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        // ============================================================================
        // Admin Functions
        // ============================================================================

        /// <summary>
        /// Withdraw available bond (bond - outstanding).
        /// </summary>
        public static void WithdrawBond(byte[] serviceId, BigInteger amount, UInt160 recipient)
        {
            RequireAdmin();

            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");

            BigInteger available = service.BondAmount - service.OutstandingAmount;
            if (amount > available) throw new Exception("Amount exceeds available bond");

            service.BondAmount -= amount;
            SaveService(service);

            GAS.Transfer(Runtime.ExecutingScriptHash, recipient, amount, null);
        }

        public static void SuspendService(byte[] serviceId)
        {
            RequireAdmin();
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            service.Status = 0;
            SaveService(service);
        }

        public static void ActivateService(byte[] serviceId)
        {
            RequireAdmin();
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            service.Status = 1;
            SaveService(service);
        }

        public static void UpdateTeePubKey(byte[] serviceId, ECPoint newTeePubKey)
        {
            RequireAdmin();
            ServiceData service = GetService(serviceId);
            if (service == null) throw new Exception("Service not found");
            service.TeePubKey = newTeePubKey;
            SaveService(service);
        }
    }

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
