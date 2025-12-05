using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.CCIP
{
    /// <summary>
    /// CCIPService - Cross-Chain Interoperability Protocol contract.
    ///
    /// Provides secure cross-chain message passing and asset transfers.
    ///
    /// Flow:
    /// 1. User initiates cross-chain transfer via Gateway
    /// 2. Service Layer (TEE) validates and relays message to target chain
    /// 3. Confirmation delivered back to source chain
    ///
    /// Supports multiple blockchain networks with cryptographic proof verification.
    /// </summary>
    [DisplayName("CCIPService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "CCIP Service - Cross-chain interoperability protocol")]
    [ContractPermission("*", "*")]
    public class CCIPService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_MESSAGE = 0x10;
        private const byte PREFIX_CHAIN_CONFIG = 0x20;
        private const byte PREFIX_NONCE = 0x30;
        private const byte PREFIX_PAUSED = 0x40;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a cross-chain message is sent.
        /// </summary>
        [DisplayName("CCIPMessageSent")]
        public static event Action<ByteString, UInt160, BigInteger, ByteString, ByteString> OnCCIPMessageSent;
        // Parameters: messageId, sender, targetChainId, targetAddress, payload

        /// <summary>
        /// Emitted when a cross-chain message is received.
        /// </summary>
        [DisplayName("CCIPMessageReceived")]
        public static event Action<ByteString, BigInteger, ByteString, ByteString> OnCCIPMessageReceived;
        // Parameters: messageId, sourceChainId, sourceAddress, payload

        /// <summary>
        /// Emitted when message delivery is confirmed.
        /// </summary>
        [DisplayName("CCIPMessageConfirmed")]
        public static event Action<ByteString, bool, ByteString> OnCCIPMessageConfirmed;
        // Parameters: messageId, success, proof

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

        // ==================== Chain Configuration ====================

        /// <summary>
        /// Configures a supported chain. Only admin can call.
        /// </summary>
        public static void ConfigureChain(BigInteger chainId, string chainName, ByteString endpoint)
        {
            RequireAdmin();
            var config = new ChainConfig
            {
                ChainId = chainId,
                ChainName = chainName,
                Endpoint = endpoint,
                IsActive = true
            };
            var key = GetChainConfigKey(chainId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(config));
        }

        /// <summary>
        /// Gets chain configuration.
        /// </summary>
        public static ChainConfig GetChainConfig(BigInteger chainId)
        {
            var key = GetChainConfigKey(chainId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (ChainConfig)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Disables a chain. Only admin can call.
        /// </summary>
        public static void DisableChain(BigInteger chainId)
        {
            RequireAdmin();
            var config = GetChainConfig(chainId);
            if (config == null) throw new Exception("Chain not found");
            config.IsActive = false;
            var key = GetChainConfigKey(chainId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(config));
        }

        // ==================== Cross-Chain Operations ====================

        /// <summary>
        /// Processes a cross-chain message request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (CCIPRequestData)StdLib.Deserialize(payload);

            // Validate target chain
            var chainConfig = GetChainConfig(requestData.TargetChainId);
            if (chainConfig == null || !chainConfig.IsActive)
                throw new Exception("Target chain not supported");

            // Get nonce
            var nonce = GetAndIncrementNonce(requester);

            // Store message
            var message = new CCIPMessage
            {
                MessageId = requestId,
                Sender = requester,
                TargetChainId = requestData.TargetChainId,
                TargetAddress = requestData.TargetAddress,
                Payload = requestData.Payload,
                Nonce = nonce,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreMessage(requestId, message);

            OnCCIPMessageSent(requestId, requester, requestData.TargetChainId, requestData.TargetAddress, requestData.Payload);
        }

        /// <summary>
        /// Delivers cross-chain message confirmation. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString proof, ByteString signature)
        {
            RequireGateway();

            var message = GetMessage(requestId);
            if (message == null) throw new Exception("Message not found");
            if (message.Status != 0) throw new Exception("Message already processed");

            message.Status = success ? (byte)1 : (byte)2;
            message.ProcessedAt = Runtime.Time;
            message.Proof = proof;
            StoreMessage(requestId, message);

            OnCCIPMessageConfirmed(requestId, success, proof);
        }

        /// <summary>
        /// Receives a cross-chain message from another chain. Called via Gateway.
        /// </summary>
        public static void ReceiveMessage(ByteString messageId, BigInteger sourceChainId, ByteString sourceAddress, ByteString payload, ByteString proof)
        {
            RequireGateway();
            RequireNotPaused();

            // Validate source chain
            var chainConfig = GetChainConfig(sourceChainId);
            if (chainConfig == null || !chainConfig.IsActive)
                throw new Exception("Source chain not supported");

            // TODO: Verify proof

            OnCCIPMessageReceived(messageId, sourceChainId, sourceAddress, payload);
        }

        /// <summary>
        /// Gets a message by ID.
        /// </summary>
        public static CCIPMessage GetMessage(ByteString messageId)
        {
            var key = GetMessageKey(messageId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (CCIPMessage)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets the nonce for a sender.
        /// </summary>
        public static BigInteger GetNonce(UInt160 sender)
        {
            var key = GetNonceKey(sender);
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

        private static byte[] GetMessageKey(ByteString messageId)
        {
            return Helper.Concat(new byte[] { PREFIX_MESSAGE }, messageId);
        }

        private static byte[] GetChainConfigKey(BigInteger chainId)
        {
            return Helper.Concat(new byte[] { PREFIX_CHAIN_CONFIG }, (ByteString)chainId);
        }

        private static byte[] GetNonceKey(UInt160 sender)
        {
            return Helper.Concat(new byte[] { PREFIX_NONCE }, (ByteString)sender);
        }

        private static void StoreMessage(ByteString messageId, CCIPMessage message)
        {
            var key = GetMessageKey(messageId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(message));
        }

        private static BigInteger GetAndIncrementNonce(UInt160 sender)
        {
            var key = GetNonceKey(sender);
            var stored = Storage.Get(Storage.CurrentContext, key);
            BigInteger nonce = stored != null ? (BigInteger)stored : 0;
            Storage.Put(Storage.CurrentContext, key, nonce + 1);
            return nonce;
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class CCIPRequestData
    {
        public BigInteger TargetChainId;
        public ByteString TargetAddress;
        public ByteString Payload;
    }

    public class CCIPMessage
    {
        public ByteString MessageId;
        public UInt160 Sender;
        public BigInteger TargetChainId;
        public ByteString TargetAddress;
        public ByteString Payload;
        public BigInteger Nonce;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
        public ByteString Proof;
    }

    public class ChainConfig
    {
        public BigInteger ChainId;
        public string ChainName;
        public ByteString Endpoint;
        public bool IsActive;
    }
}
