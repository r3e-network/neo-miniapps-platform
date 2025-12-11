using Neo;

namespace ServiceLayer.Confidential
{
    /// <summary>Confidential request payload from user contract</summary>
    public class ConfidentialRequestPayload
    {
        public string ComputationType;   // Type of computation (aggregate, compare, auction, vote)
        public byte[] EncryptedInput;    // Input encrypted with TEE public key
        public bool OutputPublic;        // Whether output should be public or encrypted
        public byte[] UserPublicKey;     // User's public key for encrypted output
    }

    /// <summary>Stored Confidential request</summary>
    public class ConfidentialStoredRequest
    {
        public string ComputationType;
        public byte[] EncryptedInput;
        public byte[] InputCommitment;
        public bool OutputPublic;
        public UInt160 UserContract;
    }

    /// <summary>Confidential result from TEE</summary>
    public class ConfidentialResultPayload
    {
        public byte[] EncryptedOutput;    // Output (encrypted or public based on request)
        public byte[] OutputCommitment;   // Hash of output for verification
        public byte[] Proof;              // Optional ZK proof of correct computation
    }
}
