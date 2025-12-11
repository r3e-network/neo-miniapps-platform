using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.VRF
{
    public partial class NeoRandService
    {
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
    }
}
