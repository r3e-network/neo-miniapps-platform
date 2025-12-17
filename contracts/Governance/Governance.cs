using System;
using System.ComponentModel;
using System.Numerics;
using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace NeoMiniAppPlatform.Contracts
{
    [DisplayName("Governance")]
    [ManifestExtra("Author", "Neo MiniApp Platform")]
    [ManifestExtra("Description", "NEO-only staking and voting for platform governance")]
    public class Governance : SmartContract
    {
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_STAKE = new byte[] { 0x02 };
        private static readonly byte[] PREFIX_PROPOSAL = new byte[] { 0x03 };
        private static readonly byte[] PREFIX_VOTE = new byte[] { 0x04 };

        public struct Proposal
        {
            public string ProposalId;
            public string Description;
            public ulong StartTime;
            public ulong EndTime;
            public BigInteger Yes;
            public BigInteger No;
            public bool Finalized;
        }

        [DisplayName("Staked")]
        public static event Action<UInt160, BigInteger> OnStaked;

        [DisplayName("Unstaked")]
        public static event Action<UInt160, BigInteger> OnUnstaked;

        [DisplayName("Voted")]
        public static event Action<UInt160, string, bool, BigInteger> OnVoted;

        public static void _deploy(object data, bool update)
        {
            if (update) return;

            Transaction tx = Runtime.Transaction;
            Storage.Put(Storage.CurrentContext, PREFIX_ADMIN, tx.Sender);
        }

        public static UInt160 Admin()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, PREFIX_ADMIN);
        }

        private static void ValidateAdmin()
        {
            UInt160 admin = Admin();
            ExecutionEngine.Assert(admin != null, "admin not set");
            ExecutionEngine.Assert(Runtime.CheckWitness(admin), "unauthorized");
        }

        private static StorageMap StakeMap() => new StorageMap(Storage.CurrentContext, PREFIX_STAKE);
        private static StorageMap ProposalMap() => new StorageMap(Storage.CurrentContext, PREFIX_PROPOSAL);
        private static StorageMap VoteMap() => new StorageMap(Storage.CurrentContext, PREFIX_VOTE);

        private static ByteString ProposalKey(string proposalId)
        {
            ExecutionEngine.Assert(proposalId != null && proposalId.Length > 0, "proposal id required");
            return (ByteString)proposalId;
        }

        public static BigInteger GetStake(UInt160 account)
        {
            ByteString raw = StakeMap().Get(account);
            if (raw == null) return 0;
            return (BigInteger)raw;
        }

        public static void Stake(BigInteger amount)
        {
            ExecutionEngine.Assert(amount > 0, "amount must be > 0");

            Transaction tx = Runtime.Transaction;
            UInt160 from = tx.Sender;

            // NEO-only: deposit into this contract.
            bool ok = NEO.Transfer(from, Runtime.ExecutingScriptHash, amount, (ByteString)"stake");
            ExecutionEngine.Assert(ok, "NEO transfer failed");
        }

        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            // Enforce governance staking in NEO only.
            // NOTE: NEO transfers may trigger internal GAS bonus distribution. Some
            // GAS transfers can therefore touch this contract even though governance
            // stake accounting must remain NEO-only. We ignore non-NEO callbacks and
            // only process explicit NEO stake deposits.
            if (Runtime.CallingScriptHash != NEO.Hash) return;
            if (amount <= 0) throw new Exception("Invalid amount");

            // Ignore sender-side hooks during outbound transfers.
            if (from == Runtime.ExecutingScriptHash) return;

            // Only accept deposits coming from the explicit `Stake()` flow.
            if (data == null) throw new Exception("Stake data required");
            if ((ByteString)data != (ByteString)"stake") throw new Exception("Invalid stake data");

            BigInteger current = GetStake(from);
            StakeMap().Put(from, current + amount);
            OnStaked(from, amount);
        }

        public static void Unstake(BigInteger amount)
        {
            ExecutionEngine.Assert(amount > 0, "amount must be > 0");

            Transaction tx = Runtime.Transaction;
            UInt160 from = tx.Sender;
            ExecutionEngine.Assert(Runtime.CheckWitness(from), "unauthorized");

            BigInteger current = GetStake(from);
            ExecutionEngine.Assert(current >= amount, "insufficient stake");

            StakeMap().Put(from, current - amount);

            bool ok = NEO.Transfer(Runtime.ExecutingScriptHash, from, amount, null);
            ExecutionEngine.Assert(ok, "NEO transfer failed");

            OnUnstaked(from, amount);
        }

        // ============================================================================
        // Proposals (minimal skeleton)
        // ============================================================================

        public static void CreateProposal(string proposalId, string description, ulong startTime, ulong endTime)
        {
            ValidateAdmin();
            ExecutionEngine.Assert(proposalId != null && proposalId.Length > 0, "proposal id required");
            ExecutionEngine.Assert(endTime > startTime, "invalid window");

            Proposal p = new Proposal
            {
                ProposalId = proposalId,
                Description = description ?? "",
                StartTime = startTime,
                EndTime = endTime,
                Yes = 0,
                No = 0,
                Finalized = false
            };
            ProposalMap().Put(ProposalKey(proposalId), StdLib.Serialize(p));
        }

        public static Proposal GetProposal(string proposalId)
        {
            ByteString raw = ProposalMap().Get(ProposalKey(proposalId));
            if (raw == null)
            {
                // Avoid returning `default` struct which may be represented as an empty VMArray.
                return new Proposal
                {
                    ProposalId = "",
                    Description = "",
                    StartTime = 0,
                    EndTime = 0,
                    Yes = 0,
                    No = 0,
                    Finalized = false
                };
            }
            return (Proposal)StdLib.Deserialize(raw);
        }

        public static void Vote(string proposalId, bool support, BigInteger amount)
        {
            ExecutionEngine.Assert(proposalId != null && proposalId.Length > 0, "proposal id required");
            ExecutionEngine.Assert(amount > 0, "amount must be > 0");

            Transaction tx = Runtime.Transaction;
            UInt160 voter = tx.Sender;
            ExecutionEngine.Assert(Runtime.CheckWitness(voter), "unauthorized");

            Proposal p = GetProposal(proposalId);
            ExecutionEngine.Assert(p.ProposalId != null && p.ProposalId.Length > 0, "proposal not found");
            ExecutionEngine.Assert(!p.Finalized, "finalized");
            ExecutionEngine.Assert(Runtime.Time >= p.StartTime && Runtime.Time <= p.EndTime, "voting closed");

            BigInteger stake = GetStake(voter);
            ExecutionEngine.Assert(stake >= amount, "insufficient stake");

            // One vote record per voter+proposal (simple model).
            byte[] voteKey = Helper.Concat((byte[])ProposalKey(proposalId), (byte[])voter);
            ByteString prev = VoteMap().Get(voteKey);
            ExecutionEngine.Assert(prev == null, "already voted");
            VoteMap().Put(voteKey, amount.ToByteArray());

            if (support) p.Yes += amount;
            else p.No += amount;

            ProposalMap().Put(ProposalKey(proposalId), StdLib.Serialize(p));
            OnVoted(voter, proposalId, support, amount);
        }

        public static void Finalize(string proposalId)
        {
            ValidateAdmin();
            Proposal p = GetProposal(proposalId);
            ExecutionEngine.Assert(p.ProposalId != null && p.ProposalId.Length > 0, "proposal not found");
            ExecutionEngine.Assert(!p.Finalized, "already finalized");
            ExecutionEngine.Assert(Runtime.Time > p.EndTime, "voting not ended");

            p.Finalized = true;
            ProposalMap().Put(ProposalKey(proposalId), StdLib.Serialize(p));
        }
    }
}
