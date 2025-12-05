using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.VRF
{
    /// <summary>
    /// VRFService - Verifiable Random Function service contract.
    ///
    /// Provides cryptographically secure, verifiable randomness.
    ///
    /// Flow:
    /// 1. User Contract -> Gateway.Request("vrf", ...) with seed and numWords
    /// 2. Service Layer (TEE) generates VRF proof and random values
    /// 3. Service Layer -> Gateway.Callback() -> User Contract callback
    ///
    /// The VRF proof can be verified on-chain to ensure randomness integrity.
    /// </summary>
    [DisplayName("VRFService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "VRF Service - Verifiable Random Function")]
    [ContractPermission("*", "*")]
    public class VRFService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_REQUEST = 0x10;
        private const byte PREFIX_VRF_KEY = 0x20;
        private const byte PREFIX_PAUSED = 0x30;
        private const byte PREFIX_NONCE = 0x40;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a VRF request is created.
        /// </summary>
        [DisplayName("VRFRequest")]
        public static event Action<ByteString, UInt160, ByteString, BigInteger> OnVRFRequest;
        // Parameters: requestId, requester, seed, numWords

        /// <summary>
        /// Emitted when VRF response is delivered.
        /// </summary>
        [DisplayName("VRFResponse")]
        public static event Action<ByteString, bool, ByteString, ByteString> OnVRFResponse;
        // Parameters: requestId, success, randomWords, proof

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

        // ==================== VRF Key Management ====================

        /// <summary>
        /// Sets the VRF public key. Only admin can call.
        /// This is the public key used to verify VRF proofs.
        /// </summary>
        public static void SetVRFPublicKey(ByteString publicKey)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_VRF_KEY }, publicKey);
        }

        /// <summary>
        /// Gets the VRF public key.
        /// </summary>
        public static ByteString GetVRFPublicKey()
        {
            return Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_VRF_KEY });
        }

        // ==================== VRF Request ====================

        /// <summary>
        /// Creates a VRF request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            // Deserialize payload
            var requestData = (VRFRequestData)StdLib.Deserialize(payload);

            // Validate
            if (requestData.NumWords == 0 || requestData.NumWords > 10)
                throw new Exception("Invalid numWords (1-10)");

            // Get nonce for this requester
            var nonce = GetAndIncrementNonce(requester);

            // Store request
            var request = new VRFRequest
            {
                RequestId = requestId,
                Requester = requester,
                Seed = requestData.Seed,
                NumWords = requestData.NumWords,
                Nonce = nonce,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreRequest(requestId, request);

            OnVRFRequest(requestId, requester, requestData.Seed, requestData.NumWords);
        }

        /// <summary>
        /// Delivers VRF response. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString randomWords, ByteString proof, ByteString signature)
        {
            RequireGateway();

            var request = GetRequest(requestId);
            if (request == null) throw new Exception("Request not found");
            if (request.Status != 0) throw new Exception("Request already processed");

            // Verify VRF proof (optional on-chain verification)
            if (success)
            {
                var vrfKey = GetVRFPublicKey();
                if (vrfKey != null)
                {
                    // TODO: Implement VRF proof verification
                    // VerifyVRFProof(vrfKey, request.Seed, randomWords, proof);
                }
            }

            // Update request
            request.Status = success ? (byte)1 : (byte)2;
            request.ProcessedAt = Runtime.Time;
            request.RandomWords = randomWords;
            request.Proof = proof;
            StoreRequest(requestId, request);

            OnVRFResponse(requestId, success, randomWords, proof);
        }

        /// <summary>
        /// Gets a request by ID.
        /// </summary>
        public static VRFRequest GetRequest(ByteString requestId)
        {
            var key = GetRequestKey(requestId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (VRFRequest)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Gets the nonce for a requester.
        /// </summary>
        public static BigInteger GetNonce(UInt160 requester)
        {
            var key = GetNonceKey(requester);
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

        private static byte[] GetRequestKey(ByteString requestId)
        {
            return Helper.Concat(new byte[] { PREFIX_REQUEST }, requestId);
        }

        private static byte[] GetNonceKey(UInt160 requester)
        {
            return Helper.Concat(new byte[] { PREFIX_NONCE }, (ByteString)requester);
        }

        private static void StoreRequest(ByteString requestId, VRFRequest request)
        {
            var key = GetRequestKey(requestId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(request));
        }

        private static BigInteger GetAndIncrementNonce(UInt160 requester)
        {
            var key = GetNonceKey(requester);
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

    public class VRFRequestData
    {
        public ByteString Seed;
        public BigInteger NumWords;
    }

    public class VRFRequest
    {
        public ByteString RequestId;
        public UInt160 Requester;
        public ByteString Seed;
        public BigInteger NumWords;
        public BigInteger Nonce;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
        public ByteString RandomWords;
        public ByteString Proof;
    }
}
