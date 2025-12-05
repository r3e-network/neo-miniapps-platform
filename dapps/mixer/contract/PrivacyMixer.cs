using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.DApps
{
    /// <summary>
    /// PrivacyMixer - Privacy-preserving transaction mixer powered by Service Layer.
    ///
    /// Features:
    /// - Fixed denomination pools for anonymity
    /// - TEE-based mixing for privacy
    /// - Commitment-nullifier scheme
    /// - Time-delayed withdrawals
    ///
    /// Flow:
    /// 1. User deposits fixed amount with commitment hash
    /// 2. Service Layer (TEE) processes and mixes deposits
    /// 3. User withdraws to new address with nullifier proof
    /// </summary>
    [DisplayName("PrivacyMixer")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Privacy Mixer DApp powered by Service Layer TEE")]
    [ContractPermission("*", "*")]
    public class PrivacyMixer : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_POOL = 0x10;
        private const byte PREFIX_COMMITMENT = 0x20;
        private const byte PREFIX_NULLIFIER = 0x30;
        private const byte PREFIX_PENDING_WITHDRAWAL = 0x40;
        private const byte PREFIX_CONFIG = 0x50;
        private const byte PREFIX_STATS = 0x60;

        // Pool denominations (in GAS with 8 decimals)
        private static readonly BigInteger POOL_1_GAS = 1_00000000;    // 1 GAS
        private static readonly BigInteger POOL_10_GAS = 10_00000000;  // 10 GAS
        private static readonly BigInteger POOL_100_GAS = 100_00000000; // 100 GAS

        [InitialValue("NZHf1NJvz1tvELGLWZjhpb3NqZJFFqMSbR", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        [DisplayName("Deposit")]
        public static event Action<ByteString, BigInteger, BigInteger> OnDeposit;
        // commitment, poolId, timestamp

        [DisplayName("WithdrawalRequested")]
        public static event Action<ByteString, UInt160, BigInteger> OnWithdrawalRequested;
        // nullifier, recipient, poolId

        [DisplayName("WithdrawalCompleted")]
        public static event Action<ByteString, UInt160, BigInteger> OnWithdrawalCompleted;
        // nullifier, recipient, amount

        [DisplayName("MixCompleted")]
        public static event Action<BigInteger, BigInteger> OnMixCompleted;
        // poolId, mixedCount

        // ==================== Admin Methods ====================

        public static UInt160 GetAdmin()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
            return stored != null ? (UInt160)stored : InitialAdmin;
        }

        public static void SetAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
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
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        // ==================== Configuration ====================

        public static void SetConfig(BigInteger minDelay, BigInteger maxDelay, BigInteger fee)
        {
            RequireAdmin();
            var config = new MixerConfig
            {
                MinWithdrawalDelay = minDelay,  // Minimum delay in ms
                MaxWithdrawalDelay = maxDelay,  // Maximum delay in ms
                FeePercent = fee                 // Fee in basis points (100 = 1%)
            };
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_CONFIG }, StdLib.Serialize(config));
        }

        public static MixerConfig GetConfig()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_CONFIG });
            if (stored == null)
            {
                return new MixerConfig
                {
                    MinWithdrawalDelay = 3600000,   // 1 hour
                    MaxWithdrawalDelay = 86400000,  // 24 hours
                    FeePercent = 50                  // 0.5%
                };
            }
            return (MixerConfig)StdLib.Deserialize(stored);
        }

        // ==================== Pool Management ====================

        /// <summary>
        /// Gets pool information by denomination.
        /// </summary>
        public static MixerPool GetPool(BigInteger poolId)
        {
            var key = GetPoolKey(poolId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null)
            {
                return new MixerPool
                {
                    PoolId = poolId,
                    Denomination = GetDenomination(poolId),
                    TotalDeposits = 0,
                    TotalWithdrawals = 0,
                    PendingCount = 0,
                    Active = true
                };
            }
            return (MixerPool)StdLib.Deserialize(stored);
        }

        private static BigInteger GetDenomination(BigInteger poolId)
        {
            if (poolId == 1) return POOL_1_GAS;
            if (poolId == 2) return POOL_10_GAS;
            if (poolId == 3) return POOL_100_GAS;
            throw new Exception("Invalid pool");
        }

        // ==================== Deposit ====================

        /// <summary>
        /// Deposits funds into the mixer with a commitment.
        /// The commitment is hash(secret || nullifier) where:
        /// - secret: random value known only to depositor
        /// - nullifier: unique identifier to prevent double-spending
        /// </summary>
        public static void Deposit(BigInteger poolId, ByteString commitment)
        {
            if (poolId < 1 || poolId > 3) throw new Exception("Invalid pool");

            var pool = GetPool(poolId);
            if (!pool.Active) throw new Exception("Pool not active");

            // Check commitment not already used
            if (IsCommitmentUsed(commitment)) throw new Exception("Commitment already exists");

            // Store commitment
            var commitmentData = new CommitmentData
            {
                Commitment = commitment,
                PoolId = poolId,
                DepositTime = Runtime.Time,
                Withdrawn = false
            };
            StoreCommitment(commitment, commitmentData);

            // Update pool stats
            pool.TotalDeposits += 1;
            pool.PendingCount += 1;
            StorePool(poolId, pool);

            // Update global stats
            UpdateStats(poolId, true);

            OnDeposit(commitment, poolId, Runtime.Time);
        }

        /// <summary>
        /// NEP-17 payment handler for deposits.
        /// Data should contain: [poolId, commitment]
        /// </summary>
        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash) return;
            if (data == null) return;

            var params_ = (object[])data;
            if (params_.Length < 2) return;

            var poolId = (BigInteger)params_[0];
            var commitment = (ByteString)params_[1];

            var denomination = GetDenomination(poolId);
            if (amount < denomination) throw new Exception("Insufficient amount");

            // Process deposit
            Deposit(poolId, commitment);

            // Refund excess
            var excess = amount - denomination;
            if (excess > 0)
            {
                GAS.Transfer(Runtime.ExecutingScriptHash, from, excess, null);
            }
        }

        // ==================== Withdrawal ====================

        /// <summary>
        /// Requests a withdrawal from the mixer.
        /// The nullifier proves knowledge of the secret without revealing it.
        /// </summary>
        public static void RequestWithdrawal(
            ByteString nullifier,
            ByteString commitment,
            UInt160 recipient,
            ByteString proof)
        {
            // Check nullifier not already used
            if (IsNullifierUsed(nullifier)) throw new Exception("Nullifier already used");

            // Get commitment data
            var commitmentData = GetCommitment(commitment);
            if (commitmentData == null) throw new Exception("Commitment not found");
            if (commitmentData.Withdrawn) throw new Exception("Already withdrawn");

            // Request Service Layer to verify proof
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");

            var payload = StdLib.Serialize(new WithdrawalRequest
            {
                Nullifier = nullifier,
                Commitment = commitment,
                Recipient = recipient,
                Proof = proof,
                PoolId = commitmentData.PoolId
            });

            Contract.Call(gateway, "request", CallFlags.All, new object[]
            {
                "mixer",
                Runtime.ExecutingScriptHash,
                "OnWithdrawalVerified",
                payload,
                2_00000000
            });

            OnWithdrawalRequested(nullifier, recipient, commitmentData.PoolId);
        }

        /// <summary>
        /// Callback from Service Layer after proof verification.
        /// </summary>
        public static void OnWithdrawalVerified(ByteString requestId, ByteString result)
        {
            RequireGateway();

            var verificationResult = (WithdrawalVerification)StdLib.Deserialize(result);

            if (!verificationResult.Valid)
            {
                // Verification failed
                return;
            }

            // Mark nullifier as used
            MarkNullifierUsed(verificationResult.Nullifier);

            // Mark commitment as withdrawn
            var commitmentData = GetCommitment(verificationResult.Commitment);
            commitmentData.Withdrawn = true;
            StoreCommitment(verificationResult.Commitment, commitmentData);

            // Calculate amount after fee
            var config = GetConfig();
            var pool = GetPool(commitmentData.PoolId);
            var fee = pool.Denomination * config.FeePercent / 10000;
            var amount = pool.Denomination - fee;

            // Create pending withdrawal with delay
            var delay = CalculateDelay(config.MinWithdrawalDelay, config.MaxWithdrawalDelay);
            var pendingWithdrawal = new PendingWithdrawal
            {
                Nullifier = verificationResult.Nullifier,
                Recipient = verificationResult.Recipient,
                Amount = amount,
                PoolId = commitmentData.PoolId,
                UnlockTime = Runtime.Time + (ulong)delay,
                Completed = false
            };
            StorePendingWithdrawal(verificationResult.Nullifier, pendingWithdrawal);

            // Update pool stats
            pool.TotalWithdrawals += 1;
            pool.PendingCount -= 1;
            StorePool(commitmentData.PoolId, pool);

            UpdateStats(commitmentData.PoolId, false);
        }

        /// <summary>
        /// Completes a pending withdrawal after the delay period.
        /// </summary>
        public static void CompleteWithdrawal(ByteString nullifier)
        {
            var pending = GetPendingWithdrawal(nullifier);
            if (pending == null) throw new Exception("Withdrawal not found");
            if (pending.Completed) throw new Exception("Already completed");
            if (Runtime.Time < pending.UnlockTime) throw new Exception("Withdrawal locked");

            // Transfer funds
            GAS.Transfer(Runtime.ExecutingScriptHash, pending.Recipient, pending.Amount, null);

            // Mark as completed
            pending.Completed = true;
            StorePendingWithdrawal(nullifier, pending);

            OnWithdrawalCompleted(nullifier, pending.Recipient, pending.Amount);
        }

        // ==================== Query Methods ====================

        public static bool IsCommitmentUsed(ByteString commitment)
        {
            var key = GetCommitmentKey(commitment);
            return Storage.Get(Storage.CurrentContext, key) != null;
        }

        public static bool IsNullifierUsed(ByteString nullifier)
        {
            var key = GetNullifierKey(nullifier);
            return Storage.Get(Storage.CurrentContext, key) != null;
        }

        public static CommitmentData GetCommitment(ByteString commitment)
        {
            var key = GetCommitmentKey(commitment);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (CommitmentData)StdLib.Deserialize(stored);
        }

        public static PendingWithdrawal GetPendingWithdrawal(ByteString nullifier)
        {
            var key = GetPendingWithdrawalKey(nullifier);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (PendingWithdrawal)StdLib.Deserialize(stored);
        }

        public static MixerStats GetStats()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_STATS });
            if (stored == null)
            {
                return new MixerStats
                {
                    TotalDeposits = 0,
                    TotalWithdrawals = 0,
                    TotalVolume = 0,
                    Pool1Deposits = 0,
                    Pool2Deposits = 0,
                    Pool3Deposits = 0
                };
            }
            return (MixerStats)StdLib.Deserialize(stored);
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

        private static byte[] GetPoolKey(BigInteger poolId)
        {
            return Helper.Concat(new byte[] { PREFIX_POOL }, (ByteString)poolId);
        }

        private static byte[] GetCommitmentKey(ByteString commitment)
        {
            return Helper.Concat(new byte[] { PREFIX_COMMITMENT }, commitment);
        }

        private static byte[] GetNullifierKey(ByteString nullifier)
        {
            return Helper.Concat(new byte[] { PREFIX_NULLIFIER }, nullifier);
        }

        private static byte[] GetPendingWithdrawalKey(ByteString nullifier)
        {
            return Helper.Concat(new byte[] { PREFIX_PENDING_WITHDRAWAL }, nullifier);
        }

        private static void StorePool(BigInteger poolId, MixerPool pool)
        {
            var key = GetPoolKey(poolId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(pool));
        }

        private static void StoreCommitment(ByteString commitment, CommitmentData data)
        {
            var key = GetCommitmentKey(commitment);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(data));
        }

        private static void MarkNullifierUsed(ByteString nullifier)
        {
            var key = GetNullifierKey(nullifier);
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        private static void StorePendingWithdrawal(ByteString nullifier, PendingWithdrawal pending)
        {
            var key = GetPendingWithdrawalKey(nullifier);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(pending));
        }

        private static void UpdateStats(BigInteger poolId, bool isDeposit)
        {
            var stats = GetStats();
            var denomination = GetDenomination(poolId);

            if (isDeposit)
            {
                stats.TotalDeposits += 1;
                stats.TotalVolume += denomination;
                if (poolId == 1) stats.Pool1Deposits += 1;
                else if (poolId == 2) stats.Pool2Deposits += 1;
                else if (poolId == 3) stats.Pool3Deposits += 1;
            }
            else
            {
                stats.TotalWithdrawals += 1;
            }

            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_STATS }, StdLib.Serialize(stats));
        }

        private static BigInteger CalculateDelay(BigInteger min, BigInteger max)
        {
            // Use block time as pseudo-random source for delay
            var seed = Runtime.Time % (max - min);
            return min + seed;
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    // ==================== Data Structures ====================

    public class MixerConfig
    {
        public BigInteger MinWithdrawalDelay;
        public BigInteger MaxWithdrawalDelay;
        public BigInteger FeePercent;
    }

    public class MixerPool
    {
        public BigInteger PoolId;
        public BigInteger Denomination;
        public BigInteger TotalDeposits;
        public BigInteger TotalWithdrawals;
        public BigInteger PendingCount;
        public bool Active;
    }

    public class CommitmentData
    {
        public ByteString Commitment;
        public BigInteger PoolId;
        public BigInteger DepositTime;
        public bool Withdrawn;
    }

    public class PendingWithdrawal
    {
        public ByteString Nullifier;
        public UInt160 Recipient;
        public BigInteger Amount;
        public BigInteger PoolId;
        public BigInteger UnlockTime;
        public bool Completed;
    }

    public class WithdrawalRequest
    {
        public ByteString Nullifier;
        public ByteString Commitment;
        public UInt160 Recipient;
        public ByteString Proof;
        public BigInteger PoolId;
    }

    public class WithdrawalVerification
    {
        public bool Valid;
        public ByteString Nullifier;
        public ByteString Commitment;
        public UInt160 Recipient;
    }

    public class MixerStats
    {
        public BigInteger TotalDeposits;
        public BigInteger TotalWithdrawals;
        public BigInteger TotalVolume;
        public BigInteger Pool1Deposits;
        public BigInteger Pool2Deposits;
        public BigInteger Pool3Deposits;
    }
}
