using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Examples
{
    /// <summary>
    /// MixerClient - A client contract for privacy mixing using Service Layer Mixer.
    ///
    /// Features:
    /// - Deposit GAS/NEO for mixing
    /// - Create mix requests with encrypted targets
    /// - Receive mixed funds at target addresses
    ///
    /// Flow:
    /// 1. User deposits tokens (GAS or NEO)
    /// 2. User creates mix request with encrypted target addresses
    /// 3. TEE claims request and executes mixing
    /// 4. Mixed funds arrive at target addresses (off-chain delivery)
    /// 5. User receives confirmation callback
    /// </summary>
    [DisplayName("MixerClient")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Privacy Mixing Client using Service Layer Mixer")]
    [ManifestExtra("Version", "1.0.0")]
    [ContractPermission("*", "*")]
    public class MixerClient : SmartContract
    {
        private const byte PREFIX_OWNER = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_MIX_REQUEST = 0x10;
        private const byte PREFIX_DEPOSIT = 0x20;
        private const byte PREFIX_MIXER_SERVICE = 0x03;

        // Minimum amounts
        private const long MIN_GAS_AMOUNT = 10000000;   // 0.1 GAS
        private const long MIN_NEO_AMOUNT = 1;          // 1 NEO

        // Events
        [DisplayName("DepositReceived")]
        public static event Action<UInt160, string, BigInteger> OnDepositReceived;

        [DisplayName("MixRequestCreated")]
        public static event Action<BigInteger, UInt160, string, BigInteger> OnMixRequestCreated;

        [DisplayName("MixCompleted")]
        public static event Action<BigInteger, byte[]> OnMixCompleted;

        [DisplayName("MixFailed")]
        public static event Action<BigInteger, string> OnMixFailed;

        [DisplayName("RefundIssued")]
        public static event Action<BigInteger, UInt160, BigInteger> OnRefundIssued;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_OWNER }, tx.Sender);
        }

        // ============================================================================
        // Configuration
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            RequireOwner();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static void SetMixerService(UInt160 mixerService)
        {
            RequireOwner();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_MIXER_SERVICE }, mixerService);
        }

        public static UInt160 GetGateway() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });

        public static UInt160 GetMixerService() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_MIXER_SERVICE });

        private static UInt160 GetOwner() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_OWNER });

        private static void RequireOwner()
        {
            if (!Runtime.CheckWitness(GetOwner())) throw new Exception("Owner only");
        }

        // ============================================================================
        // Deposit Handling
        // ============================================================================

        /// <summary>Receive deposits (GAS or NEO)</summary>
        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            UInt160 callingContract = Runtime.CallingScriptHash;
            string tokenType;

            if (callingContract == GAS.Hash)
            {
                if (amount < MIN_GAS_AMOUNT) throw new Exception("Below minimum GAS amount");
                tokenType = "GAS";
            }
            else if (callingContract == NEO.Hash)
            {
                if (amount < MIN_NEO_AMOUNT) throw new Exception("Below minimum NEO amount");
                tokenType = "NEO";
            }
            else
            {
                throw new Exception("Only GAS or NEO accepted");
            }

            // Store deposit info
            DepositInfo deposit = new DepositInfo
            {
                User = from,
                TokenType = tokenType,
                Amount = amount,
                Timestamp = Runtime.Time,
                Used = false
            };

            StorageMap depositMap = new StorageMap(Storage.CurrentContext, PREFIX_DEPOSIT);
            BigInteger depositId = GetNextDepositId();
            depositMap.Put(depositId.ToByteArray(), StdLib.Serialize(deposit));

            OnDepositReceived(from, tokenType, amount);
        }

        // ============================================================================
        // Mix Request Creation
        // ============================================================================

        /// <summary>
        /// Create a mix request using deposited funds.
        ///
        /// The encryptedTargets should be encrypted with the TEE's public key:
        /// - List of target addresses
        /// - Amount per target
        /// - Optional: delay preferences
        /// </summary>
        public static BigInteger CreateMixRequest(
            BigInteger depositId,
            byte[] encryptedTargets,
            BigInteger mixOption)
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;

            // Verify deposit
            StorageMap depositMap = new StorageMap(Storage.CurrentContext, PREFIX_DEPOSIT);
            ByteString depositData = depositMap.Get(depositId.ToByteArray());
            if (depositData == null) throw new Exception("Deposit not found");

            DepositInfo deposit = (DepositInfo)StdLib.Deserialize((ByteString)depositData);
            if (deposit.Used) throw new Exception("Deposit already used");
            if (deposit.User != tx.Sender) throw new Exception("Not deposit owner");

            // Verify gateway is set
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");

            // Build mixer payload
            MixerPayload payload = new MixerPayload
            {
                DepositId = depositId,
                TokenType = deposit.TokenType,
                Amount = deposit.Amount,
                EncryptedTargets = encryptedTargets,
                MixOption = mixOption
            };

            byte[] payloadBytes = (byte[])StdLib.Serialize(payload);

            // Call Gateway to request Mixer service
            BigInteger requestId = (BigInteger)Contract.Call(gateway, "requestService", CallFlags.All,
                new object[] { "mixer", payloadBytes, "onMixCallback" });

            // Mark deposit as used
            deposit.Used = true;
            deposit.MixRequestId = requestId;
            depositMap.Put(depositId.ToByteArray(), StdLib.Serialize(deposit));

            // Store mix request
            MixRequestInfo mixRequest = new MixRequestInfo
            {
                RequestId = requestId,
                User = tx.Sender,
                DepositId = depositId,
                TokenType = deposit.TokenType,
                Amount = deposit.Amount,
                Status = MixStatus.Pending,
                CreatedAt = Runtime.Time
            };

            StorageMap mixMap = new StorageMap(Storage.CurrentContext, PREFIX_MIX_REQUEST);
            mixMap.Put(requestId.ToByteArray(), StdLib.Serialize(mixRequest));

            OnMixRequestCreated(requestId, tx.Sender, deposit.TokenType, deposit.Amount);

            return requestId;
        }

        // ============================================================================
        // Mix Callback
        // ============================================================================

        /// <summary>Callback from Service Layer Mixer</summary>
        public static void OnMixCallback(BigInteger requestId, bool success, byte[] result, string error)
        {
            UInt160 gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can callback");

            StorageMap mixMap = new StorageMap(Storage.CurrentContext, PREFIX_MIX_REQUEST);
            ByteString mixData = mixMap.Get(requestId.ToByteArray());
            if (mixData == null) throw new Exception("Unknown mix request");

            MixRequestInfo mixRequest = (MixRequestInfo)StdLib.Deserialize((ByteString)mixData);

            if (success)
            {
                mixRequest.Status = MixStatus.Completed;
                mixRequest.OutputsHash = result;
                mixRequest.CompletedAt = Runtime.Time;

                OnMixCompleted(requestId, result);
            }
            else
            {
                mixRequest.Status = MixStatus.Failed;
                mixRequest.Error = error;

                // Issue refund
                IssueRefund(mixRequest);

                OnMixFailed(requestId, error);
            }

            mixMap.Put(requestId.ToByteArray(), StdLib.Serialize(mixRequest));
        }

        private static void IssueRefund(MixRequestInfo mixRequest)
        {
            UInt160 tokenContract = mixRequest.TokenType == "GAS" ? GAS.Hash : NEO.Hash;

            Contract.Call(tokenContract, "transfer", CallFlags.All,
                new object[] { Runtime.ExecutingScriptHash, mixRequest.User, mixRequest.Amount, null });

            OnRefundIssued(mixRequest.RequestId, mixRequest.User, mixRequest.Amount);
        }

        // ============================================================================
        // User Refund Request (Timeout)
        // ============================================================================

        /// <summary>Request refund if mix times out (after 24 hours)</summary>
        public static void RequestRefund(BigInteger requestId)
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;

            StorageMap mixMap = new StorageMap(Storage.CurrentContext, PREFIX_MIX_REQUEST);
            ByteString mixData = mixMap.Get(requestId.ToByteArray());
            if (mixData == null) throw new Exception("Mix request not found");

            MixRequestInfo mixRequest = (MixRequestInfo)StdLib.Deserialize((ByteString)mixData);
            if (mixRequest.User != tx.Sender) throw new Exception("Not request owner");
            if (mixRequest.Status != MixStatus.Pending) throw new Exception("Request not pending");

            // Check timeout (24 hours = 86400000 ms)
            if (Runtime.Time < mixRequest.CreatedAt + 86400000)
                throw new Exception("Timeout not reached");

            mixRequest.Status = MixStatus.Refunded;
            IssueRefund(mixRequest);

            mixMap.Put(requestId.ToByteArray(), StdLib.Serialize(mixRequest));
        }

        // ============================================================================
        // Query Functions
        // ============================================================================

        public static MixRequestInfo GetMixRequest(BigInteger requestId)
        {
            StorageMap mixMap = new StorageMap(Storage.CurrentContext, PREFIX_MIX_REQUEST);
            ByteString data = mixMap.Get(requestId.ToByteArray());
            if (data == null) return null;
            return (MixRequestInfo)StdLib.Deserialize(data);
        }

        public static DepositInfo GetDeposit(BigInteger depositId)
        {
            StorageMap depositMap = new StorageMap(Storage.CurrentContext, PREFIX_DEPOSIT);
            ByteString data = depositMap.Get(depositId.ToByteArray());
            if (data == null) return null;
            return (DepositInfo)StdLib.Deserialize(data);
        }

        private static BigInteger GetNextDepositId()
        {
            byte[] key = new byte[] { 0xFF };
            BigInteger id = (BigInteger)Storage.Get(Storage.CurrentContext, key);
            id += 1;
            Storage.Put(Storage.CurrentContext, key, id);
            return id;
        }

        public static BigInteger GetMinGasAmount() => MIN_GAS_AMOUNT;
        public static BigInteger GetMinNeoAmount() => MIN_NEO_AMOUNT;
    }

    // ============================================================================
    // Data Structures
    // ============================================================================

    public enum MixStatus : byte
    {
        Pending = 0,
        Processing = 1,
        Completed = 2,
        Failed = 3,
        Refunded = 4
    }

    public class DepositInfo
    {
        public UInt160 User;
        public string TokenType;
        public BigInteger Amount;
        public ulong Timestamp;
        public bool Used;
        public BigInteger MixRequestId;
    }

    public class MixRequestInfo
    {
        public BigInteger RequestId;
        public UInt160 User;
        public BigInteger DepositId;
        public string TokenType;
        public BigInteger Amount;
        public MixStatus Status;
        public ulong CreatedAt;
        public ulong CompletedAt;
        public byte[] OutputsHash;
        public string Error;
    }

    public class MixerPayload
    {
        public BigInteger DepositId;
        public string TokenType;
        public BigInteger Amount;
        public byte[] EncryptedTargets;
        public BigInteger MixOption;
    }
}
