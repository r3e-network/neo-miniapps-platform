//! SGX Sealing Operations
//!
//! This module provides data sealing using SGX's EGETKEY instruction.
//! Sealed data can only be unsealed by the same enclave (MRENCLAVE policy)
//! or any enclave signed by the same key (MRSIGNER policy).

use std::prelude::v1::*;
use std::vec::Vec;

use sgx_types::*;
use sgx_tseal::SgxSealedData;

use crate::types::{EnclaveError, EnclaveResult, SealedDataHeader};

/// Sealing policy determines which enclaves can unseal the data.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum SealingPolicy {
    /// Only the exact same enclave (same MRENCLAVE) can unseal.
    /// Most restrictive - data is tied to specific enclave version.
    MrEnclave,
    /// Any enclave signed by the same key (same MRSIGNER) can unseal.
    /// Allows enclave upgrades while maintaining access to sealed data.
    MrSigner,
}

impl Default for SealingPolicy {
    fn default() -> Self {
        SealingPolicy::MrSigner
    }
}

/// Seal data using SGX sealing key.
///
/// # Arguments
/// * `plaintext` - Data to seal
/// * `aad` - Additional authenticated data (not encrypted, but integrity protected)
/// * `policy` - Sealing policy (MRENCLAVE or MRSIGNER)
///
/// # Returns
/// Sealed data blob that can only be unsealed inside an authorized enclave.
pub fn seal_data(
    plaintext: &[u8],
    aad: &[u8],
    policy: SealingPolicy,
) -> EnclaveResult<Vec<u8>> {
    if plaintext.is_empty() {
        return Err(EnclaveError::InvalidParameter);
    }

    // Calculate required size
    let sealed_size = SgxSealedData::<[u8]>::calc_raw_sealed_data_size(
        aad.len() as u32,
        plaintext.len() as u32,
    ) as usize;

    // Seal the data
    let sealed_data = match policy {
        SealingPolicy::MrSigner => {
            SgxSealedData::<[u8]>::seal_data(aad, plaintext)
                .map_err(|e| EnclaveError::SealError(format!("Seal failed: {:?}", e)))?
        }
        SealingPolicy::MrEnclave => {
            // Use seal_data_ex with MRENCLAVE policy
            let key_policy = SGX_KEYPOLICY_MRENCLAVE;
            let attribute_mask = sgx_attributes_t {
                flags: TSEAL_DEFAULT_FLAGSMASK,
                xfrm: 0,
            };
            let misc_mask = TSEAL_DEFAULT_MISCMASK;

            SgxSealedData::<[u8]>::seal_data_ex(
                key_policy,
                attribute_mask,
                misc_mask,
                aad,
                plaintext,
            ).map_err(|e| EnclaveError::SealError(format!("Seal failed: {:?}", e)))?
        }
    };

    // Convert to raw bytes
    let raw_sealed = sealed_data.into_raw_sealed_data_t();
    let sealed_ptr = &raw_sealed as *const _ as *const u8;
    let sealed_bytes = unsafe {
        std::slice::from_raw_parts(sealed_ptr, sealed_size)
    };

    Ok(sealed_bytes.to_vec())
}

/// Unseal data that was previously sealed.
///
/// # Arguments
/// * `sealed` - Sealed data blob
///
/// # Returns
/// Tuple of (plaintext, additional_authenticated_data)
pub fn unseal_data(sealed: &[u8]) -> EnclaveResult<(Vec<u8>, Vec<u8>)> {
    if sealed.is_empty() {
        return Err(EnclaveError::InvalidParameter);
    }

    // Reconstruct sealed data structure
    let sealed_data = unsafe {
        SgxSealedData::<[u8]>::from_raw_sealed_data_t(
            sealed.as_ptr() as *mut sgx_sealed_data_t,
            sealed.len() as u32,
        )
    }.ok_or_else(|| EnclaveError::UnsealError("Invalid sealed data format".to_string()))?;

    // Unseal
    let unsealed = sealed_data.unseal_data()
        .map_err(|e| EnclaveError::UnsealError(format!("Unseal failed: {:?}", e)))?;

    let plaintext = unsealed.get_decrypt_txt().to_vec();
    let aad = unsealed.get_additional_txt().to_vec();

    Ok((plaintext, aad))
}

/// Calculate the size of sealed data for given plaintext and AAD sizes.
pub fn calc_sealed_size(plaintext_len: usize, aad_len: usize) -> usize {
    SgxSealedData::<[u8]>::calc_raw_sealed_data_size(
        aad_len as u32,
        plaintext_len as u32,
    ) as usize
}

/// Seal data with a custom header for versioning.
pub fn seal_data_with_header(
    plaintext: &[u8],
    aad: &[u8],
    policy: SealingPolicy,
) -> EnclaveResult<Vec<u8>> {
    // Create header
    let header = SealedDataHeader::new(plaintext.len() as u32, aad.len() as u32);
    let header_bytes = unsafe {
        std::slice::from_raw_parts(
            &header as *const _ as *const u8,
            std::mem::size_of::<SealedDataHeader>(),
        )
    };

    // Combine header with AAD
    let mut combined_aad = Vec::with_capacity(header_bytes.len() + aad.len());
    combined_aad.extend_from_slice(header_bytes);
    combined_aad.extend_from_slice(aad);

    seal_data(plaintext, &combined_aad, policy)
}

/// Unseal data and validate header.
pub fn unseal_data_with_header(sealed: &[u8]) -> EnclaveResult<(Vec<u8>, Vec<u8>)> {
    let (plaintext, combined_aad) = unseal_data(sealed)?;

    // Validate header
    if combined_aad.len() < std::mem::size_of::<SealedDataHeader>() {
        return Err(EnclaveError::UnsealError("Missing header".to_string()));
    }

    let header = unsafe {
        &*(combined_aad.as_ptr() as *const SealedDataHeader)
    };

    if !header.validate() {
        return Err(EnclaveError::UnsealError("Invalid header".to_string()));
    }

    // Extract original AAD
    let aad = combined_aad[std::mem::size_of::<SealedDataHeader>()..].to_vec();

    Ok((plaintext, aad))
}

/// Key derivation using SGX sealing key.
/// Derives a deterministic key that is unique to this enclave.
pub fn derive_key(
    key_id: &[u8],
    key_len: usize,
    policy: SealingPolicy,
) -> EnclaveResult<Vec<u8>> {
    // Seal a known value with the key_id as AAD
    // The sealing key is derived from EGETKEY, making it deterministic
    let dummy_data = [0u8; 32];
    let sealed = seal_data(&dummy_data, key_id, policy)?;

    // Use part of the sealed data as the derived key
    // The MAC in the sealed data is derived from the sealing key
    if sealed.len() < key_len {
        return Err(EnclaveError::BufferTooSmall {
            required: key_len,
            provided: sealed.len(),
        });
    }

    // Extract key material from the sealed blob
    // In production, would use proper KDF
    Ok(sealed[..key_len].to_vec())
}

// SGX sealing constants
const SGX_KEYPOLICY_MRENCLAVE: u16 = 0x0001;
const TSEAL_DEFAULT_FLAGSMASK: u64 = 0xFFFFFFFFFFFFFFFF;
const TSEAL_DEFAULT_MISCMASK: u32 = 0xFFFFFFFF;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_seal_unseal_roundtrip() {
        let plaintext = b"secret data to seal";
        let aad = b"additional authenticated data";

        let sealed = seal_data(plaintext, aad, SealingPolicy::MrSigner).unwrap();
        let (unsealed_plaintext, unsealed_aad) = unseal_data(&sealed).unwrap();

        assert_eq!(plaintext.as_slice(), unsealed_plaintext.as_slice());
        assert_eq!(aad.as_slice(), unsealed_aad.as_slice());
    }

    #[test]
    fn test_seal_with_header() {
        let plaintext = b"versioned secret";
        let aad = b"metadata";

        let sealed = seal_data_with_header(plaintext, aad, SealingPolicy::MrSigner).unwrap();
        let (unsealed_plaintext, unsealed_aad) = unseal_data_with_header(&sealed).unwrap();

        assert_eq!(plaintext.as_slice(), unsealed_plaintext.as_slice());
        assert_eq!(aad.as_slice(), unsealed_aad.as_slice());
    }

    #[test]
    fn test_derive_key() {
        let key1 = derive_key(b"key-1", 32, SealingPolicy::MrSigner).unwrap();
        let key2 = derive_key(b"key-2", 32, SealingPolicy::MrSigner).unwrap();
        let key1_again = derive_key(b"key-1", 32, SealingPolicy::MrSigner).unwrap();

        // Different key IDs should produce different keys
        assert_ne!(key1, key2);
        // Same key ID should produce same key (deterministic)
        assert_eq!(key1, key1_again);
    }
}
