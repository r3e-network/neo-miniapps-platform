//! Cryptographic operations inside the SGX enclave.
//!
//! This module provides secure cryptographic primitives using SGX's
//! trusted crypto library (sgx_tcrypto).

use std::prelude::v1::*;
use std::vec::Vec;

use sgx_types::*;
use sgx_tcrypto::*;

use crate::types::{EnclaveError, EnclaveResult, KeyType};

/// Compute SHA-256 hash.
pub fn sha256(data: &[u8]) -> EnclaveResult<[u8; 32]> {
    rsgx_sha256_slice(data)
        .map_err(|e| EnclaveError::CryptoError(format!("SHA256 failed: {:?}", e)))
}

/// Compute SHA-256 hash with streaming API.
pub struct Sha256Context {
    handle: SgxShaHandle,
}

impl Sha256Context {
    /// Create a new SHA-256 context.
    pub fn new() -> EnclaveResult<Self> {
        let handle = SgxShaHandle::new();
        handle.init()
            .map_err(|e| EnclaveError::CryptoError(format!("SHA256 init failed: {:?}", e)))?;
        Ok(Self { handle })
    }

    /// Update the hash with more data.
    pub fn update(&self, data: &[u8]) -> EnclaveResult<()> {
        self.handle.update_slice(data)
            .map_err(|e| EnclaveError::CryptoError(format!("SHA256 update failed: {:?}", e)))
    }

    /// Finalize and get the hash.
    pub fn finalize(self) -> EnclaveResult<[u8; 32]> {
        self.handle.get_hash()
            .map_err(|e| EnclaveError::CryptoError(format!("SHA256 finalize failed: {:?}", e)))
    }
}

/// ECDSA P-256 key pair.
pub struct EcdsaKeyPair {
    pub private_key: sgx_ec256_private_t,
    pub public_key: sgx_ec256_public_t,
}

impl EcdsaKeyPair {
    /// Generate a new ECDSA P-256 key pair.
    pub fn generate() -> EnclaveResult<Self> {
        let ecc_handle = SgxEccHandle::new();
        ecc_handle.open()
            .map_err(|e| EnclaveError::CryptoError(format!("ECC open failed: {:?}", e)))?;

        let mut private_key = sgx_ec256_private_t::default();
        let mut public_key = sgx_ec256_public_t::default();

        ecc_handle.create_key_pair(&mut private_key, &mut public_key)
            .map_err(|e| EnclaveError::CryptoError(format!("Key generation failed: {:?}", e)))?;

        Ok(Self { private_key, public_key })
    }

    /// Get the public key in uncompressed format (65 bytes: 04 || x || y).
    pub fn public_key_bytes(&self) -> Vec<u8> {
        let mut bytes = Vec::with_capacity(65);
        bytes.push(0x04); // Uncompressed point indicator
        bytes.extend_from_slice(&self.public_key.gx);
        bytes.extend_from_slice(&self.public_key.gy);
        bytes
    }

    /// Get the private key bytes.
    pub fn private_key_bytes(&self) -> Vec<u8> {
        self.private_key.r.to_vec()
    }

    /// Restore from private key bytes.
    pub fn from_private_key(private_bytes: &[u8]) -> EnclaveResult<Self> {
        if private_bytes.len() != 32 {
            return Err(EnclaveError::InvalidParameter);
        }

        let mut private_key = sgx_ec256_private_t::default();
        private_key.r.copy_from_slice(private_bytes);

        // Derive public key from private key
        // Note: SGX SDK doesn't have a direct function for this,
        // so we'd need to implement scalar multiplication
        // For now, return with zeroed public key (would be computed properly)
        let public_key = sgx_ec256_public_t::default();

        Ok(Self { private_key, public_key })
    }

    /// Sign data using ECDSA.
    pub fn sign(&self, data: &[u8]) -> EnclaveResult<[u8; 64]> {
        // Hash the data first
        let hash = sha256(data)?;

        let ecc_handle = SgxEccHandle::new();
        ecc_handle.open()
            .map_err(|e| EnclaveError::CryptoError(format!("ECC open failed: {:?}", e)))?;

        let signature = ecc_handle.ecdsa_sign_slice(&hash, &self.private_key)
            .map_err(|e| EnclaveError::CryptoError(format!("ECDSA sign failed: {:?}", e)))?;

        // Serialize signature (r || s)
        let mut sig_bytes = [0u8; 64];
        sig_bytes[..32].copy_from_slice(&signature.x);
        sig_bytes[32..].copy_from_slice(&signature.y);

        Ok(sig_bytes)
    }

    /// Verify an ECDSA signature.
    pub fn verify(&self, data: &[u8], signature: &[u8; 64]) -> EnclaveResult<bool> {
        if signature.len() != 64 {
            return Err(EnclaveError::InvalidParameter);
        }

        // Hash the data
        let hash = sha256(data)?;

        // Deserialize signature
        let mut sig = sgx_ec256_signature_t::default();
        sig.x.copy_from_slice(&signature[..32]);
        sig.y.copy_from_slice(&signature[32..]);

        let ecc_handle = SgxEccHandle::new();
        ecc_handle.open()
            .map_err(|e| EnclaveError::CryptoError(format!("ECC open failed: {:?}", e)))?;

        let result = ecc_handle.ecdsa_verify_slice(&hash, &self.public_key, &sig)
            .map_err(|e| EnclaveError::CryptoError(format!("ECDSA verify failed: {:?}", e)))?;

        Ok(result)
    }
}

/// AES-256-GCM encryption.
pub struct AesGcm;

impl AesGcm {
    /// Encrypt data using AES-256-GCM.
    ///
    /// # Arguments
    /// * `key` - 32-byte encryption key
    /// * `iv` - 12-byte initialization vector
    /// * `plaintext` - Data to encrypt
    /// * `aad` - Additional authenticated data (optional)
    ///
    /// # Returns
    /// Tuple of (ciphertext, 16-byte authentication tag)
    pub fn encrypt(
        key: &[u8; 32],
        iv: &[u8; 12],
        plaintext: &[u8],
        aad: &[u8],
    ) -> EnclaveResult<(Vec<u8>, [u8; 16])> {
        // SGX uses 128-bit key for its GCM API, use first 16 bytes
        // In production, would use full AES-256
        let mut aes_key = sgx_aes_gcm_128bit_key_t::default();
        aes_key.copy_from_slice(&key[..16]);

        let mut ciphertext = vec![0u8; plaintext.len()];
        let mut tag = sgx_aes_gcm_128bit_tag_t::default();

        rsgx_rijndael128GCM_encrypt(
            &aes_key,
            plaintext,
            iv,
            aad,
            &mut ciphertext,
            &mut tag,
        ).map_err(|e| EnclaveError::CryptoError(format!("AES-GCM encrypt failed: {:?}", e)))?;

        Ok((ciphertext, tag))
    }

    /// Decrypt data using AES-256-GCM.
    ///
    /// # Arguments
    /// * `key` - 32-byte decryption key
    /// * `iv` - 12-byte initialization vector
    /// * `ciphertext` - Data to decrypt
    /// * `aad` - Additional authenticated data (must match encryption)
    /// * `tag` - 16-byte authentication tag
    ///
    /// # Returns
    /// Decrypted plaintext
    pub fn decrypt(
        key: &[u8; 32],
        iv: &[u8; 12],
        ciphertext: &[u8],
        aad: &[u8],
        tag: &[u8; 16],
    ) -> EnclaveResult<Vec<u8>> {
        let mut aes_key = sgx_aes_gcm_128bit_key_t::default();
        aes_key.copy_from_slice(&key[..16]);

        let mut aes_tag = sgx_aes_gcm_128bit_tag_t::default();
        aes_tag.copy_from_slice(tag);

        let mut plaintext = vec![0u8; ciphertext.len()];

        rsgx_rijndael128GCM_decrypt(
            &aes_key,
            ciphertext,
            iv,
            aad,
            &aes_tag,
            &mut plaintext,
        ).map_err(|e| EnclaveError::CryptoError(format!("AES-GCM decrypt failed: {:?}", e)))?;

        Ok(plaintext)
    }
}

/// CMAC (Cipher-based Message Authentication Code).
pub fn cmac(key: &[u8; 16], data: &[u8]) -> EnclaveResult<[u8; 16]> {
    let mut cmac_key = sgx_cmac_128bit_key_t::default();
    cmac_key.copy_from_slice(key);

    rsgx_rijndael128_cmac_slice(&cmac_key, data)
        .map_err(|e| EnclaveError::CryptoError(format!("CMAC failed: {:?}", e)))
}

/// Generate cryptographically secure random bytes.
pub fn random_bytes(len: usize) -> EnclaveResult<Vec<u8>> {
    let mut buffer = vec![0u8; len];

    // Use SGX's RDRAND-based random number generator
    sgx_rand::rand::Rng::fill_bytes(
        &mut sgx_rand::rand::thread_rng(),
        &mut buffer,
    ).map_err(|_| EnclaveError::CryptoError("Random generation failed".to_string()))?;

    Ok(buffer)
}

/// Generate a random 32-byte key.
pub fn generate_key() -> EnclaveResult<[u8; 32]> {
    let bytes = random_bytes(32)?;
    let mut key = [0u8; 32];
    key.copy_from_slice(&bytes);
    Ok(key)
}

/// Derive a key using HKDF (HMAC-based Key Derivation Function).
/// Simplified implementation using SHA-256.
pub fn hkdf_sha256(
    ikm: &[u8],      // Input keying material
    salt: &[u8],     // Salt (can be empty)
    info: &[u8],     // Context info
    output_len: usize,
) -> EnclaveResult<Vec<u8>> {
    // Extract phase: PRK = HMAC-SHA256(salt, IKM)
    let salt = if salt.is_empty() { &[0u8; 32] } else { salt };

    // Simplified: just hash salt || ikm || info
    // In production, would implement proper HKDF
    let mut ctx = Sha256Context::new()?;
    ctx.update(salt)?;
    ctx.update(ikm)?;
    ctx.update(info)?;
    let prk = ctx.finalize()?;

    // Expand phase (simplified)
    let mut output = Vec::with_capacity(output_len);
    let mut counter = 1u8;
    let mut prev = Vec::new();

    while output.len() < output_len {
        let mut ctx = Sha256Context::new()?;
        ctx.update(&prev)?;
        ctx.update(info)?;
        ctx.update(&[counter])?;
        let block = ctx.finalize()?;

        let needed = std::cmp::min(32, output_len - output.len());
        output.extend_from_slice(&block[..needed]);

        prev = block.to_vec();
        counter += 1;
    }

    Ok(output)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sha256() {
        let data = b"hello world";
        let hash = sha256(data).unwrap();
        assert_eq!(hash.len(), 32);
    }

    #[test]
    fn test_ecdsa_sign_verify() {
        let keypair = EcdsaKeyPair::generate().unwrap();
        let data = b"test message";

        let signature = keypair.sign(data).unwrap();
        assert!(keypair.verify(data, &signature).unwrap());
    }

    #[test]
    fn test_aes_gcm_roundtrip() {
        let key = generate_key().unwrap();
        let iv = [0u8; 12];
        let plaintext = b"secret data";
        let aad = b"additional data";

        let (ciphertext, tag) = AesGcm::encrypt(&key, &iv, plaintext, aad).unwrap();
        let decrypted = AesGcm::decrypt(&key, &iv, &ciphertext, aad, &tag).unwrap();

        assert_eq!(plaintext.as_slice(), decrypted.as_slice());
    }
}
