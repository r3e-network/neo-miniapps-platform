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
    /// MixerService - Privacy mixing service contract.
    ///
    /// Provides privacy-preserving transaction mixing using TEE.
    ///
    /// Flow:
    /// 1. User deposits funds into mixing pool
    /// 2. Service Layer (TEE) manages mixing logic and key derivation
    /// 3. User withdraws to new address with privacy guarantees
    ///
    /// The TEE ensures that deposit-withdrawal linkage cannot be traced.
    /// </summary>
    [DisplayName("MixerService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Mixer Service - Privacy-preserving transaction mixing")]
    [ContractPermission("*", "*")]
    public class MixerService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_DEPOSIT = 0x10;
        private const byte PREFIX_WITHDRAWAL = 0x11;
        private const byte PREFIX_POOL_BALANCE = 0x20;
        private const byte PREFIX_PAUSED = 0x30;
        private const byte PREFIX_MIN_DEPOSIT = 0x40;
        private const byte PREFIX_MIX_FEE = 0x41;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a deposit is made to the mixing pool.
        /// </summary>
        [DisplayName("MixerDeposit")]
        public static event Action<ByteString, UInt160, BigInteger, ByteString> OnMixerDeposit;
        // Parameters: depositId, depositor, amount, commitment

        /// <summary>
        /// Emitted when a withdrawal is processed.
        /// </summary>
        [DisplayName("MixerWithdrawal")]
        public static event Action<ByteString, UInt160, BigInteger, ByteString> OnMixerWithdrawal;
        // Parameters: withdrawalId, recipient, amount, nullifier

        /// <summary>
        /// Emitted when mixing is completed.
        /// </summary>
        [DisplayName("MixingComplete")]
        public static event Action<ByteString, bool> OnMixingComplete;
        // Parameters: requestId, success

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

        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ==================== Configuration ====================

        public static void SetMinDeposit(BigInteger amount)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_MIN_DEPOSIT }, amount);
        }

        public static BigInteger GetMinDeposit()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_MIN_DEPOSIT });
            return stored != null ? (BigInteger)stored : 1_00000000; // 1 GAS default
        }

        public static void SetMixFee(BigInteger feePercent)
        {
            RequireAdmin();
            if (feePercent > 1000) throw new Exception("Fee too high"); // Max 10%
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_MIX_FEE }, feePercent);
        }

        public static BigInteger GetMixFee()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_MIX_FEE });
            return stored != null ? (BigInteger)stored : 10; // 0.1% default
        }

        // ==================== Mixing Operations ====================

        /// <summary>
        /// Processes a mixing request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (MixerRequestData)StdLib.Deserialize(payload);

            if (requestData.Amount < GetMinDeposit())
                throw new Exception("Amount below minimum");

            // Store deposit
            var deposit = new MixerDeposit
            {
                DepositId = requestId,
                Depositor = requester,
                Amount = requestData.Amount,
                Commitment = requestData.Commitment,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreDeposit(requestId, deposit);

            // Update pool balance
            UpdatePoolBalance(requestData.Amount);

            OnMixerDeposit(requestId, requester, requestData.Amount, requestData.Commitment);
        }

        /// <summary>
        /// Delivers mixing response. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString withdrawalData, ByteString signature)
        {
            RequireGateway();

            var deposit = GetDeposit(requestId);
            if (deposit == null) throw new Exception("Deposit not found");
            if (deposit.Status != 0) throw new Exception("Already processed");

            deposit.Status = success ? (byte)1 : (byte)2;
            deposit.ProcessedAt = Runtime.Time;
            StoreDeposit(requestId, deposit);

            if (success && withdrawalData != null)
            {
                var withdrawal = (MixerWithdrawal)StdLib.Deserialize(withdrawalData);
                OnMixerWithdrawal(withdrawal.WithdrawalId, withdrawal.Recipient, withdrawal.Amount, withdrawal.Nullifier);
            }

            OnMixingComplete(requestId, success);
        }

        /// <summary>
        /// Gets a deposit by ID.
        /// </summary>
        public static MixerDeposit GetDeposit(ByteString depositId)
        {
            var key = GetDepositKey(depositId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (MixerDeposit)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets the current pool balance.
        /// </summary>
        public static BigInteger GetPoolBalance()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_POOL_BALANCE });
            return stored != null ? (BigInteger)stored : 0;
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

        private static byte[] GetDepositKey(ByteString depositId)
        {
            return Helper.Concat(new byte[] { PREFIX_DEPOSIT }, depositId);
        }

        private static void StoreDeposit(ByteString depositId, MixerDeposit deposit)
        {
            var key = GetDepositKey(depositId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(deposit));
        }

        private static void UpdatePoolBalance(BigInteger amount)
        {
            var current = GetPoolBalance();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_POOL_BALANCE }, current + amount);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class MixerRequestData
    {
        public BigInteger Amount;
        public ByteString Commitment;
    }

    public class MixerDeposit
    {
        public ByteString DepositId;
        public UInt160 Depositor;
        public BigInteger Amount;
        public ByteString Commitment;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
    }

    public class MixerWithdrawal
    {
        public ByteString WithdrawalId;
        public UInt160 Recipient;
        public BigInteger Amount;
        public ByteString Nullifier;
    }
}
