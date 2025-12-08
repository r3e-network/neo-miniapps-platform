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
    /// VRFService - On-chain logic for VRF (Verifiable Random Function) requests.
    ///
    /// This contract ONLY interacts with ServiceLayerGateway:
    /// - Receives requests via onRequest() from Gateway
    /// - Emits events for TEE to monitor
    /// - Receives fulfillment via onFulfill() from Gateway
    /// - Stores VRF proofs for on-chain verification
    ///
    /// Flow:
    /// 1. UserContract → Gateway.RequestService("vrf", payload) → VRFService.onRequest()
    /// 2. VRFService emits VRFRequest event
    /// 3. TEE monitors event, generates VRF, signs result
    /// 4. TEE → Gateway.FulfillRequest() → VRFService.onFulfill()
    /// 5. Gateway → UserContract.callback()
    /// </summary>
    [DisplayName("VRFService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "VRF Service Contract - Verifiable Random Numbers")]
    [ManifestExtra("Version", "2.0.0")]
    [ContractPermission("*", "*")]
    public class VRFService : SmartContract
    {
        // Storage prefixes
        private const byte PREFIX_GATEWAY = 0x01;
        private const byte PREFIX_VRF_PUBKEY = 0x02;
        private const byte PREFIX_REQUEST = 0x10;
        private const byte PREFIX_RANDOMNESS = 0x20;
        private const byte PREFIX_PROOF = 0x21;

        // ============================================================================
        // Events - Monitored by TEE
        // ============================================================================

        /// <summary>
        /// Emitted when a new VRF request is created.
        /// TEE monitors this event to generate randomness.
        /// </summary>
        [DisplayName("VRFRequest")]
        public static event Action<BigInteger, UInt160, byte[], BigInteger> OnVRFRequest;
        // requestId, userContract, seed, numWords

        /// <summary>Emitted when VRF is fulfilled with proof</summary>
        [DisplayName("VRFFulfilled")]
        public static event Action<BigInteger, byte[], byte[]> OnVRFFulfilled;
        // requestId, randomWords, proof

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
        }

        // ============================================================================
        // Gateway Management
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            UInt160 currentGateway = GetGateway();
            if (currentGateway != null)
            {
                if (Runtime.CallingScriptHash != currentGateway)
                    throw new Exception("Only gateway can update");
            }
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static UInt160 GetGateway()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
        }

        private static void RequireGateway()
        {
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }

        /// <summary>Set VRF public key for proof verification</summary>
        public static void SetVRFPublicKey(ECPoint pubKey)
        {
            RequireGateway();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_VRF_PUBKEY }, pubKey);
        }

        public static ECPoint GetVRFPublicKey()
        {
            return (ECPoint)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_VRF_PUBKEY });
        }

        // ============================================================================
        // Request Handling (Called by Gateway)
        // ============================================================================

        /// <summary>
        /// Called by ServiceLayerGateway when a user contract requests VRF service.
        /// </summary>
        public static void OnRequest(BigInteger requestId, UInt160 userContract, byte[] payload)
        {
            RequireGateway();

            // Parse payload
            VRFRequestPayload request = (VRFRequestPayload)StdLib.Deserialize((ByteString)payload);

            if (request.Seed == null || request.Seed.Length == 0)
                throw new Exception("Seed required");

            if (request.NumWords <= 0 || request.NumWords > 10)
                throw new Exception("NumWords must be 1-10");

            // Enhance seed with block info for additional entropy
            byte[] enhancedSeed = Helper.Concat(request.Seed, (byte[])Ledger.CurrentHash);
            enhancedSeed = Helper.Concat(enhancedSeed, requestId.ToByteArray());

            // Store request
            VRFStoredRequest stored = new VRFStoredRequest
            {
                Seed = enhancedSeed,
                NumWords = request.NumWords,
                UserContract = userContract
            };

            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Put(requestId.ToByteArray(), StdLib.Serialize(stored));

            // Emit event for TEE
            OnVRFRequest(requestId, userContract, enhancedSeed, request.NumWords);
        }

        /// <summary>
        /// Called by ServiceLayerGateway when TEE fulfills the VRF request.
        /// Result contains: randomWords + proof
        /// </summary>
        public static void OnFulfill(BigInteger requestId, byte[] result)
        {
            RequireGateway();

            // Parse result (randomWords || proof)
            VRFResultPayload vrfResult = (VRFResultPayload)StdLib.Deserialize((ByteString)result);

            // Store randomness and proof for future verification
            StorageMap randomMap = new StorageMap(Storage.CurrentContext, PREFIX_RANDOMNESS);
            StorageMap proofMap = new StorageMap(Storage.CurrentContext, PREFIX_PROOF);

            randomMap.Put(requestId.ToByteArray(), vrfResult.RandomWords);
            proofMap.Put(requestId.ToByteArray(), vrfResult.Proof);

            // Clean up request
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Delete(requestId.ToByteArray());

            OnVRFFulfilled(requestId, vrfResult.RandomWords, vrfResult.Proof);
        }

        // ============================================================================
        // Query and Verification Functions
        // ============================================================================

        /// <summary>Get stored randomness for a request</summary>
        public static byte[] GetRandomness(BigInteger requestId)
        {
            StorageMap randomMap = new StorageMap(Storage.CurrentContext, PREFIX_RANDOMNESS);
            return (byte[])randomMap.Get(requestId.ToByteArray());
        }

        /// <summary>Get stored proof for a request</summary>
        public static byte[] GetProof(BigInteger requestId)
        {
            StorageMap proofMap = new StorageMap(Storage.CurrentContext, PREFIX_PROOF);
            return (byte[])proofMap.Get(requestId.ToByteArray());
        }

        /// <summary>Verify a VRF proof</summary>
        public static bool VerifyProof(byte[] seed, byte[] randomWords, byte[] proof)
        {
            ECPoint vrfPubKey = GetVRFPublicKey();
            if (vrfPubKey == null) return false;

            byte[] message = Helper.Concat(seed, randomWords);
            return CryptoLib.VerifyWithECDsa((ByteString)message, vrfPubKey, (ByteString)proof, NamedCurve.secp256r1);
        }
    }

    /// <summary>VRF request payload from user contract</summary>
    public class VRFRequestPayload
    {
        public byte[] Seed;         // User-provided seed
        public BigInteger NumWords; // Number of random words (1-10)
    }

    /// <summary>Stored VRF request</summary>
    public class VRFStoredRequest
    {
        public byte[] Seed;
        public BigInteger NumWords;
        public UInt160 UserContract;
    }

    /// <summary>VRF result from TEE</summary>
    public class VRFResultPayload
    {
        public byte[] RandomWords;
        public byte[] Proof;
    }
}
