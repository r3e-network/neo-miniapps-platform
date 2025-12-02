//! Common types for the SGX enclave.

use std::prelude::v1::*;

/// Result type for enclave operations.
pub type EnclaveResult<T> = Result<T, EnclaveError>;

/// Enclave error types.
#[derive(Debug, Clone)]
pub enum EnclaveError {
    /// Invalid parameter provided.
    InvalidParameter,
    /// Out of memory.
    OutOfMemory,
    /// Cryptographic operation failed.
    CryptoError(String),
    /// Sealing operation failed.
    SealError(String),
    /// Unsealing operation failed.
    UnsealError(String),
    /// Key not found.
    KeyNotFound(String),
    /// Buffer too small.
    BufferTooSmall { required: usize, provided: usize },
    /// Operation not supported.
    NotSupported,
    /// Internal error.
    Internal(String),
}

impl core::fmt::Display for EnclaveError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            EnclaveError::InvalidParameter => write!(f, "invalid parameter"),
            EnclaveError::OutOfMemory => write!(f, "out of memory"),
            EnclaveError::CryptoError(msg) => write!(f, "crypto error: {}", msg),
            EnclaveError::SealError(msg) => write!(f, "seal error: {}", msg),
            EnclaveError::UnsealError(msg) => write!(f, "unseal error: {}", msg),
            EnclaveError::KeyNotFound(id) => write!(f, "key not found: {}", id),
            EnclaveError::BufferTooSmall { required, provided } => {
                write!(f, "buffer too small: required {}, provided {}", required, provided)
            }
            EnclaveError::NotSupported => write!(f, "operation not supported"),
            EnclaveError::Internal(msg) => write!(f, "internal error: {}", msg),
        }
    }
}

/// Sealed data header for versioning and metadata.
#[repr(C)]
#[derive(Clone, Copy)]
pub struct SealedDataHeader {
    /// Magic number for validation.
    pub magic: [u8; 4],
    /// Version of the sealing format.
    pub version: u32,
    /// Timestamp when sealed (Unix epoch).
    pub timestamp: u64,
    /// Length of the plaintext data.
    pub plaintext_len: u32,
    /// Length of additional authenticated data.
    pub aad_len: u32,
    /// Reserved for future use.
    pub reserved: [u8; 8],
}

impl SealedDataHeader {
    /// Magic number: "SEAL"
    pub const MAGIC: [u8; 4] = [0x53, 0x45, 0x41, 0x4C];
    /// Current version.
    pub const VERSION: u32 = 1;

    /// Create a new header.
    pub fn new(plaintext_len: u32, aad_len: u32) -> Self {
        Self {
            magic: Self::MAGIC,
            version: Self::VERSION,
            timestamp: 0, // Would be set from OCALL
            plaintext_len,
            aad_len,
            reserved: [0; 8],
        }
    }

    /// Validate the header.
    pub fn validate(&self) -> bool {
        self.magic == Self::MAGIC && self.version <= Self::VERSION
    }
}

/// Attestation report data.
#[derive(Clone)]
pub struct AttestationData {
    /// MRENCLAVE measurement.
    pub mr_enclave: [u8; 32],
    /// MRSIGNER measurement.
    pub mr_signer: [u8; 32],
    /// ISV Product ID.
    pub isv_prod_id: u16,
    /// ISV Security Version Number.
    pub isv_svn: u16,
    /// User-provided report data.
    pub report_data: [u8; 64],
    /// Debug flag.
    pub is_debug: bool,
}

impl Default for AttestationData {
    fn default() -> Self {
        Self {
            mr_enclave: [0; 32],
            mr_signer: [0; 32],
            isv_prod_id: 0,
            isv_svn: 0,
            report_data: [0; 64],
            is_debug: false,
        }
    }
}

/// Key metadata stored with keys.
#[derive(Clone)]
pub struct KeyMetadata {
    /// Key identifier.
    pub key_id: String,
    /// Key type.
    pub key_type: KeyType,
    /// Creation timestamp.
    pub created_at: u64,
    /// Whether the key can be exported.
    pub exportable: bool,
}

/// Supported key types.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum KeyType {
    /// ECDSA P-256 (secp256r1).
    EcdsaP256,
    /// ECDSA secp256k1 (Bitcoin/Ethereum).
    EcdsaSecp256k1,
    /// AES-256.
    Aes256,
    /// Ed25519.
    Ed25519,
}

impl KeyType {
    /// Get the private key size in bytes.
    pub fn private_key_size(&self) -> usize {
        match self {
            KeyType::EcdsaP256 => 32,
            KeyType::EcdsaSecp256k1 => 32,
            KeyType::Aes256 => 32,
            KeyType::Ed25519 => 32,
        }
    }

    /// Get the public key size in bytes.
    pub fn public_key_size(&self) -> usize {
        match self {
            KeyType::EcdsaP256 => 65,      // Uncompressed: 04 || x || y
            KeyType::EcdsaSecp256k1 => 65, // Uncompressed: 04 || x || y
            KeyType::Aes256 => 0,          // Symmetric key, no public key
            KeyType::Ed25519 => 32,
        }
    }

    /// Get the signature size in bytes.
    pub fn signature_size(&self) -> usize {
        match self {
            KeyType::EcdsaP256 => 64,      // r || s
            KeyType::EcdsaSecp256k1 => 64, // r || s
            KeyType::Aes256 => 0,          // Not a signing key
            KeyType::Ed25519 => 64,
        }
    }
}

/// Script execution request.
#[derive(Clone)]
pub struct ScriptRequest {
    /// JavaScript source code.
    pub script: String,
    /// Entry point function name.
    pub entry_point: String,
    /// JSON-encoded input arguments.
    pub input: Vec<u8>,
    /// Memory limit in bytes.
    pub memory_limit: u64,
    /// Execution timeout in milliseconds.
    pub timeout_ms: u64,
}

/// Script execution result.
#[derive(Clone)]
pub struct ScriptResult {
    /// JSON-encoded output.
    pub output: Vec<u8>,
    /// Error message if failed.
    pub error: Option<String>,
    /// Memory used in bytes.
    pub memory_used: u64,
    /// Execution duration in milliseconds.
    pub duration_ms: u64,
    /// Whether execution succeeded.
    pub success: bool,
}

impl ScriptResult {
    /// Create a successful result.
    pub fn success(output: Vec<u8>, memory_used: u64, duration_ms: u64) -> Self {
        Self {
            output,
            error: None,
            memory_used,
            duration_ms,
            success: true,
        }
    }

    /// Create a failed result.
    pub fn failure(error: String, memory_used: u64, duration_ms: u64) -> Self {
        Self {
            output: Vec::new(),
            error: Some(error),
            memory_used,
            duration_ms,
            success: false,
        }
    }
}
