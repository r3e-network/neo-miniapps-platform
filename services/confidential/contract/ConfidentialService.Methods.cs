using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.Confidential
{
    public partial class ConfidentialService
    {
        // ============================================================================
        // TEE Key Management
        // ============================================================================

        /// <summary>Set TEE public key for encryption</summary>
        public static void SetTEEPublicKey(ECPoint pubKey)
        {
            RequireGateway();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_TEE_PUBKEY }, pubKey);
        }

        public static ECPoint GetTEEPublicKey()
        {
            return (ECPoint)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_TEE_PUBKEY });
        }

        // ============================================================================
        // Request Handling (Called by Gateway)
        // ============================================================================

        /// <summary>
        /// Called by ServiceLayerGateway when a user contract requests Confidential service.
        /// </summary>
        public static void OnRequest(BigInteger requestId, UInt160 userContract, byte[] payload)
        {
            RequireGateway();

            // Parse payload
            ConfidentialRequestPayload request = (ConfidentialRequestPayload)StdLib.Deserialize((ByteString)payload);

            if (request.EncryptedInput == null || request.EncryptedInput.Length == 0)
                throw new Exception("Encrypted input required");

            if (string.IsNullOrEmpty(request.ComputationType))
                throw new Exception("Computation type required");

            // Calculate input commitment (hash of encrypted input for verification)
            byte[] inputCommitment = CryptoLib.Sha256((ByteString)request.EncryptedInput);

            // Store request
            ConfidentialStoredRequest stored = new ConfidentialStoredRequest
            {
                ComputationType = request.ComputationType,
                EncryptedInput = request.EncryptedInput,
                InputCommitment = inputCommitment,
                OutputPublic = request.OutputPublic,
                UserContract = userContract
            };

            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Put(requestId.ToByteArray(), StdLib.Serialize(stored));

            // Store commitment for later verification
            StorageMap commitmentMap = new StorageMap(Storage.CurrentContext, PREFIX_COMMITMENT);
            commitmentMap.Put(requestId.ToByteArray(), inputCommitment);

            // Emit event for TEE
            OnConfidentialRequest(requestId, userContract, request.ComputationType, request.EncryptedInput, inputCommitment);
        }

        /// <summary>
        /// Called by ServiceLayerGateway when TEE fulfills the Confidential request.
        /// Result contains: encryptedOutput + outputCommitment
        /// </summary>
        public static void OnFulfill(BigInteger requestId, byte[] result)
        {
            RequireGateway();

            // Parse result
            ConfidentialResultPayload confidentialResult = (ConfidentialResultPayload)StdLib.Deserialize((ByteString)result);

            // Store result and output commitment
            StorageMap resultMap = new StorageMap(Storage.CurrentContext, PREFIX_RESULT);
            resultMap.Put(requestId.ToByteArray(), result);

            // Clean up request
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_REQUEST);
            requestMap.Delete(requestId.ToByteArray());

            OnConfidentialFulfilled(requestId, confidentialResult.EncryptedOutput, confidentialResult.OutputCommitment);
        }

        // ============================================================================
        // Query and Verification Functions
        // ============================================================================

        /// <summary>Get stored result for a request</summary>
        public static byte[] GetResult(BigInteger requestId)
        {
            StorageMap resultMap = new StorageMap(Storage.CurrentContext, PREFIX_RESULT);
            return (byte[])resultMap.Get(requestId.ToByteArray());
        }

        /// <summary>Get input commitment for verification</summary>
        public static byte[] GetInputCommitment(BigInteger requestId)
        {
            StorageMap commitmentMap = new StorageMap(Storage.CurrentContext, PREFIX_COMMITMENT);
            return (byte[])commitmentMap.Get(requestId.ToByteArray());
        }

        /// <summary>Verify that encrypted input matches stored commitment</summary>
        public static bool VerifyInputCommitment(BigInteger requestId, byte[] encryptedInput)
        {
            byte[] storedCommitment = GetInputCommitment(requestId);
            if (storedCommitment == null) return false;

            byte[] computedCommitment = CryptoLib.Sha256((ByteString)encryptedInput);
            return storedCommitment == computedCommitment;
        }
    }
}
