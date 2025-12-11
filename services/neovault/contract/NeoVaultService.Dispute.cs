using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Mixer
{
    public partial class NeoVaultService
    {
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
    }
}
