using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Accounts
{
    /// <summary>
    /// AccountsService - Account management contract.
    ///
    /// Provides secure account creation, management, and authentication.
    ///
    /// Flow:
    /// 1. User requests account creation via Gateway
    /// 2. Service Layer (TEE) generates secure credentials
    /// 3. Account metadata stored on-chain, secrets in TEE
    ///
    /// Supports multi-signature accounts and hierarchical key derivation.
    /// </summary>
    [DisplayName("AccountsService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Accounts Service - Secure account management")]
    [ContractPermission("*", "*")]
    public class AccountsService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_ACCOUNT = 0x10;
        private const byte PREFIX_ACCOUNT_NONCE = 0x11;
        private const byte PREFIX_ACCOUNT_COUNT = 0x20;
        private const byte PREFIX_PAUSED = 0x30;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when an account creation request is made.
        /// </summary>
        [DisplayName("AccountRequest")]
        public static event Action<ByteString, UInt160, string, BigInteger> OnAccountRequest;
        // Parameters: requestId, owner, accountType, derivationIndex

        /// <summary>
        /// Emitted when an account is created.
        /// </summary>
        [DisplayName("AccountCreated")]
        public static event Action<ByteString, UInt160, UInt160, string> OnAccountCreated;
        // Parameters: accountId, owner, accountAddress, accountType

        /// <summary>
        /// Emitted when account metadata is updated.
        /// </summary>
        [DisplayName("AccountUpdated")]
        public static event Action<ByteString, UInt160> OnAccountUpdated;
        // Parameters: accountId, updatedBy

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

        // ==================== Account Operations ====================

        /// <summary>
        /// Processes an account creation request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (AccountRequestData)StdLib.Deserialize(payload);

            // Validate account type
            if (requestData.AccountType != "standard" &&
                requestData.AccountType != "multisig" &&
                requestData.AccountType != "hd")
                throw new Exception("Invalid account type");

            // Get nonce for this owner
            var nonce = GetAndIncrementNonce(requester);

            // Store request
            var request = new AccountRequest
            {
                RequestId = requestId,
                Owner = requester,
                AccountType = requestData.AccountType,
                DerivationIndex = nonce,
                Metadata = requestData.Metadata,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreAccountRequest(requestId, request);

            OnAccountRequest(requestId, requester, requestData.AccountType, nonce);
        }

        /// <summary>
        /// Delivers account creation response. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString accountData, ByteString signature)
        {
            RequireGateway();

            var request = GetAccountRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != 0) throw new Exception("Request already processed");

            request.Status = success ? (byte)1 : (byte)2;
            request.ProcessedAt = Runtime.Time;
            StoreAccountRequest(requestId, request);

            if (success && accountData != null)
            {
                var account = (AccountData)StdLib.Deserialize(accountData);

                // Store account
                var accountRecord = new Account
                {
                    AccountId = requestId,
                    Owner = request.Owner,
                    AccountAddress = account.AccountAddress,
                    AccountType = request.AccountType,
                    PublicKey = account.PublicKey,
                    Metadata = request.Metadata,
                    CreatedAt = Runtime.Time,
                    IsActive = true
                };
                StoreAccount(requestId, accountRecord);

                // Increment account count
                IncrementAccountCount();

                OnAccountCreated(requestId, request.Owner, account.AccountAddress, request.AccountType);
            }
        }

        /// <summary>
        /// Gets an account by ID.
        /// </summary>
        public static Account GetAccount(ByteString accountId)
        {
            var key = GetAccountKey(accountId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (Account)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets an account request by ID.
        /// </summary>
        public static AccountRequest GetAccountRequest(ByteString requestId)
        {
            var key = GetAccountRequestKey(requestId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (AccountRequest)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Updates account metadata. Only owner can call.
        /// </summary>
        public static void UpdateAccountMetadata(ByteString accountId, ByteString newMetadata)
        {
            var account = GetAccount(accountId);
            if (account == null) throw new Exception("Account not found");
            if (!Runtime.CheckWitness(account.Owner)) throw new Exception("Only owner");

            account.Metadata = newMetadata;
            StoreAccount(accountId, account);

            OnAccountUpdated(accountId, account.Owner);
        }

        /// <summary>
        /// Deactivates an account. Only owner can call.
        /// </summary>
        public static void DeactivateAccount(ByteString accountId)
        {
            var account = GetAccount(accountId);
            if (account == null) throw new Exception("Account not found");
            if (!Runtime.CheckWitness(account.Owner)) throw new Exception("Only owner");

            account.IsActive = false;
            StoreAccount(accountId, account);
        }

        /// <summary>
        /// Gets the total account count.
        /// </summary>
        public static BigInteger GetAccountCount()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ACCOUNT_COUNT });
            return stored != null ? (BigInteger)stored : 0;
        }

        /// <summary>
        /// Gets the nonce for an owner.
        /// </summary>
        public static BigInteger GetNonce(UInt160 owner)
        {
            var key = GetNonceKey(owner);
            var stored = Storage.Get(Storage.CurrentContext, key);
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

        private static byte[] GetAccountKey(ByteString accountId)
        {
            return Helper.Concat(new byte[] { PREFIX_ACCOUNT }, accountId);
        }

        private static byte[] GetAccountRequestKey(ByteString requestId)
        {
            return Helper.Concat(new byte[] { PREFIX_ACCOUNT }, requestId);
        }

        private static byte[] GetNonceKey(UInt160 owner)
        {
            return Helper.Concat(new byte[] { PREFIX_ACCOUNT_NONCE }, (ByteString)owner);
        }

        private static void StoreAccount(ByteString accountId, Account account)
        {
            var key = GetAccountKey(accountId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(account));
        }

        private static void StoreAccountRequest(ByteString requestId, AccountRequest request)
        {
            var key = GetAccountRequestKey(requestId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(request));
        }

        private static BigInteger GetAndIncrementNonce(UInt160 owner)
        {
            var key = GetNonceKey(owner);
            var stored = Storage.Get(Storage.CurrentContext, key);
            BigInteger nonce = stored != null ? (BigInteger)stored : 0;
            Storage.Put(Storage.CurrentContext, key, nonce + 1);
            return nonce;
        }

        private static void IncrementAccountCount()
        {
            var count = GetAccountCount();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ACCOUNT_COUNT }, count + 1);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class AccountRequestData
    {
        public string AccountType;
        public ByteString Metadata;
    }

    public class AccountRequest
    {
        public ByteString RequestId;
        public UInt160 Owner;
        public string AccountType;
        public BigInteger DerivationIndex;
        public ByteString Metadata;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
    }

    public class AccountData
    {
        public UInt160 AccountAddress;
        public ByteString PublicKey;
    }

    public class Account
    {
        public ByteString AccountId;
        public UInt160 Owner;
        public UInt160 AccountAddress;
        public string AccountType;
        public ByteString PublicKey;
        public ByteString Metadata;
        public BigInteger CreatedAt;
        public bool IsActive;
    }
}
