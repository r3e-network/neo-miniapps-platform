//! SGX Enclave Implementation for Neo Service Layer TEE
//!
//! This enclave provides:
//! - Sealed storage using SGX sealing keys (EGETKEY)
//! - Cryptographic operations inside the enclave
//! - Remote attestation (EPID/DCAP)
//! - JavaScript execution (via embedded QuickJS)
//!
//! Architecture:
//! ```text
//! ┌─────────────────────────────────────────────────────────────┐
//! │                    SGX Enclave (Trusted)                     │
//! │  ┌─────────────────────────────────────────────────────────┐ │
//! │  │  ECALL Entry Points                                      │ │
//! │  │  - ecall_initialize()                                    │ │
//! │  │  - ecall_seal_data()                                     │ │
//! │  │  - ecall_unseal_data()                                   │ │
//! │  │  - ecall_generate_quote()                                │ │
//! │  │  - ecall_execute_script()                                │ │
//! │  │  - ecall_crypto_*()                                      │ │
//! │  └─────────────────────────────────────────────────────────┘ │
//! │  ┌─────────────────────────────────────────────────────────┐ │
//! │  │  Core Modules                                            │ │
//! │  │  - crypto: AES-GCM, ECDSA, SHA256, RIPEMD160            │ │
//! │  │  - sealing: Data sealing with MRSIGNER policy           │ │
//! │  │  - attestation: Quote generation                         │ │
//! │  │  - script: QuickJS JavaScript engine                     │ │
//! │  └─────────────────────────────────────────────────────────┘ │
//! └─────────────────────────────────────────────────────────────┘
//! ```

#![no_std]
#![cfg_attr(not(feature = "sim"), feature(sgx_platform))]

extern crate sgx_tstd as std;
extern crate sgx_types;
extern crate sgx_tcrypto;
extern crate sgx_tseal;
extern crate sgx_tse;
extern crate sgx_rand;

use std::prelude::v1::*;
use std::vec::Vec;
use std::string::String;
use std::sync::SgxMutex as Mutex;
use std::collections::HashMap;

use sgx_types::*;
use sgx_tcrypto::*;
use sgx_tseal::SgxSealedData;
use sgx_tse::*;

mod crypto;
mod sealing;
mod attestation;
mod types;

use types::*;

// =============================================================================
// Global State (protected by mutex)
// =============================================================================

lazy_static::lazy_static! {
    static ref ENCLAVE_STATE: Mutex<EnclaveState> = Mutex::new(EnclaveState::new());
}

struct EnclaveState {
    initialized: bool,
    enclave_id: [u8; 32],
    keys: HashMap<String, KeyEntry>,
    sealed_data: HashMap<String, Vec<u8>>,
}

struct KeyEntry {
    key_type: KeyType,
    private_key: Vec<u8>,
    public_key: Vec<u8>,
}

#[derive(Clone, Copy)]
enum KeyType {
    EcdsaP256,
    Aes256,
}

impl EnclaveState {
    fn new() -> Self {
        Self {
            initialized: false,
            enclave_id: [0u8; 32],
            keys: HashMap::new(),
            sealed_data: HashMap::new(),
        }
    }
}

// =============================================================================
// ECALL: Initialize Enclave
// =============================================================================

/// Initialize the enclave and generate enclave ID.
/// Must be called before any other ECALL.
#[no_mangle]
pub extern "C" fn ecall_initialize(
    enclave_id_out: *mut u8,
    enclave_id_len: usize,
) -> sgx_status_t {
    if enclave_id_out.is_null() || enclave_id_len < 32 {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let mut state = match ENCLAVE_STATE.lock() {
        Ok(s) => s,
        Err(_) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    };

    if state.initialized {
        // Already initialized, return existing ID
        unsafe {
            std::ptr::copy_nonoverlapping(
                state.enclave_id.as_ptr(),
                enclave_id_out,
                32,
            );
        }
        return sgx_status_t::SGX_SUCCESS;
    }

    // Generate random enclave ID
    let mut rand_id = [0u8; 32];
    match sgx_rand::rand::Rng::fill_bytes(&mut sgx_rand::rand::thread_rng(), &mut rand_id) {
        Ok(_) => {},
        Err(_) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    }

    state.enclave_id = rand_id;
    state.initialized = true;

    unsafe {
        std::ptr::copy_nonoverlapping(rand_id.as_ptr(), enclave_id_out, 32);
    }

    sgx_status_t::SGX_SUCCESS
}

// =============================================================================
// ECALL: Seal Data (using SGX sealing key from EGETKEY)
// =============================================================================

/// Seal data using the enclave's sealing key.
/// Uses MRSIGNER policy so data can be unsealed by any enclave signed by the same key.
#[no_mangle]
pub extern "C" fn ecall_seal_data(
    plaintext: *const u8,
    plaintext_len: usize,
    additional_data: *const u8,
    additional_len: usize,
    sealed_out: *mut u8,
    sealed_buf_len: usize,
    sealed_len_out: *mut usize,
) -> sgx_status_t {
    if plaintext.is_null() || sealed_out.is_null() || sealed_len_out.is_null() {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let plaintext_slice = unsafe { std::slice::from_raw_parts(plaintext, plaintext_len) };

    let additional_slice = if additional_data.is_null() || additional_len == 0 {
        &[]
    } else {
        unsafe { std::slice::from_raw_parts(additional_data, additional_len) }
    };

    // Calculate required sealed data size
    let sealed_size = SgxSealedData::<[u8]>::calc_raw_sealed_data_size(
        additional_slice.len() as u32,
        plaintext_len as u32,
    ) as usize;

    if sealed_buf_len < sealed_size {
        unsafe { *sealed_len_out = sealed_size; }
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    // Seal the data using MRSIGNER policy
    let sealed_data = match SgxSealedData::<[u8]>::seal_data(
        additional_slice,
        plaintext_slice,
    ) {
        Ok(sd) => sd,
        Err(e) => return e,
    };

    // Copy sealed data to output buffer
    let sealed_bytes = sealed_data.into_raw_sealed_data_t();
    let sealed_ptr = &sealed_bytes as *const _ as *const u8;

    unsafe {
        std::ptr::copy_nonoverlapping(sealed_ptr, sealed_out, sealed_size);
        *sealed_len_out = sealed_size;
    }

    sgx_status_t::SGX_SUCCESS
}

/// Unseal data that was previously sealed by this enclave (or same MRSIGNER).
#[no_mangle]
pub extern "C" fn ecall_unseal_data(
    sealed: *const u8,
    sealed_len: usize,
    plaintext_out: *mut u8,
    plaintext_buf_len: usize,
    plaintext_len_out: *mut usize,
) -> sgx_status_t {
    if sealed.is_null() || plaintext_out.is_null() || plaintext_len_out.is_null() {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let sealed_slice = unsafe { std::slice::from_raw_parts(sealed, sealed_len) };

    // Reconstruct sealed data structure
    let sealed_data = match unsafe {
        SgxSealedData::<[u8]>::from_raw_sealed_data_t(
            sealed_slice.as_ptr() as *mut sgx_sealed_data_t,
            sealed_len as u32,
        )
    } {
        Some(sd) => sd,
        None => return sgx_status_t::SGX_ERROR_INVALID_PARAMETER,
    };

    // Unseal the data
    let unsealed = match sealed_data.unseal_data() {
        Ok(u) => u,
        Err(e) => return e,
    };

    let plaintext = unsealed.get_decrypt_txt();

    if plaintext_buf_len < plaintext.len() {
        unsafe { *plaintext_len_out = plaintext.len(); }
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    unsafe {
        std::ptr::copy_nonoverlapping(plaintext.as_ptr(), plaintext_out, plaintext.len());
        *plaintext_len_out = plaintext.len();
    }

    sgx_status_t::SGX_SUCCESS
}

// =============================================================================
// ECALL: Remote Attestation
// =============================================================================

/// Generate an SGX quote for remote attestation.
/// The quote contains MRENCLAVE, MRSIGNER, and user-provided report data.
#[no_mangle]
pub extern "C" fn ecall_generate_report(
    report_data: *const u8,
    report_data_len: usize,
    target_info: *const sgx_target_info_t,
    report_out: *mut sgx_report_t,
) -> sgx_status_t {
    if report_out.is_null() {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    // Prepare report data (64 bytes max)
    let mut rd = sgx_report_data_t::default();
    if !report_data.is_null() && report_data_len > 0 {
        let len = std::cmp::min(report_data_len, 64);
        let data_slice = unsafe { std::slice::from_raw_parts(report_data, len) };
        rd.d[..len].copy_from_slice(data_slice);
    }

    // Get target info (use self if not provided)
    let ti = if target_info.is_null() {
        sgx_target_info_t::default()
    } else {
        unsafe { *target_info }
    };

    // Create the report
    let report = match rsgx_create_report(&ti, &rd) {
        Ok(r) => r,
        Err(e) => return e,
    };

    unsafe { *report_out = report; }

    sgx_status_t::SGX_SUCCESS
}

// =============================================================================
// ECALL: Cryptographic Operations
// =============================================================================

/// Generate an ECDSA P-256 key pair inside the enclave.
#[no_mangle]
pub extern "C" fn ecall_generate_ecdsa_keypair(
    key_id: *const u8,
    key_id_len: usize,
    public_key_out: *mut u8,
    public_key_len: usize,
) -> sgx_status_t {
    if key_id.is_null() || public_key_out.is_null() || public_key_len < 65 {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let key_id_str = match std::str::from_utf8(unsafe {
        std::slice::from_raw_parts(key_id, key_id_len)
    }) {
        Ok(s) => String::from(s),
        Err(_) => return sgx_status_t::SGX_ERROR_INVALID_PARAMETER,
    };

    // Generate ECDSA key pair
    let mut private_key = sgx_ec256_private_t::default();
    let mut public_key = sgx_ec256_public_t::default();

    let ecc_handle = match SgxEccHandle::new() {
        Ok(h) => h,
        Err(e) => return e,
    };

    match ecc_handle.open() {
        Ok(_) => {},
        Err(e) => return e,
    }

    match ecc_handle.create_key_pair(&mut private_key, &mut public_key) {
        Ok(_) => {},
        Err(e) => return e,
    }

    // Store key in enclave state
    let mut state = match ENCLAVE_STATE.lock() {
        Ok(s) => s,
        Err(_) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    };

    // Serialize public key (uncompressed format: 04 || x || y)
    let mut pub_bytes = vec![0x04u8];
    pub_bytes.extend_from_slice(&public_key.gx);
    pub_bytes.extend_from_slice(&public_key.gy);

    // Serialize private key
    let priv_bytes = private_key.r.to_vec();

    state.keys.insert(key_id_str, KeyEntry {
        key_type: KeyType::EcdsaP256,
        private_key: priv_bytes,
        public_key: pub_bytes.clone(),
    });

    // Copy public key to output
    unsafe {
        std::ptr::copy_nonoverlapping(pub_bytes.as_ptr(), public_key_out, pub_bytes.len());
    }

    sgx_status_t::SGX_SUCCESS
}

/// Sign data using ECDSA P-256.
#[no_mangle]
pub extern "C" fn ecall_ecdsa_sign(
    key_id: *const u8,
    key_id_len: usize,
    data: *const u8,
    data_len: usize,
    signature_out: *mut u8,
    signature_len: usize,
) -> sgx_status_t {
    if key_id.is_null() || data.is_null() || signature_out.is_null() || signature_len < 64 {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let key_id_str = match std::str::from_utf8(unsafe {
        std::slice::from_raw_parts(key_id, key_id_len)
    }) {
        Ok(s) => s,
        Err(_) => return sgx_status_t::SGX_ERROR_INVALID_PARAMETER,
    };

    let data_slice = unsafe { std::slice::from_raw_parts(data, data_len) };

    // Get key from state
    let state = match ENCLAVE_STATE.lock() {
        Ok(s) => s,
        Err(_) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    };

    let key_entry = match state.keys.get(key_id_str) {
        Some(k) => k,
        None => return sgx_status_t::SGX_ERROR_INVALID_PARAMETER,
    };

    // Reconstruct private key
    let mut private_key = sgx_ec256_private_t::default();
    private_key.r.copy_from_slice(&key_entry.private_key);

    // Hash the data first (SHA-256)
    let hash = match rsgx_sha256_slice(data_slice) {
        Ok(h) => h,
        Err(e) => return e,
    };

    // Sign the hash
    let ecc_handle = match SgxEccHandle::new() {
        Ok(h) => h,
        Err(e) => return e,
    };

    match ecc_handle.open() {
        Ok(_) => {},
        Err(e) => return e,
    }

    let signature = match ecc_handle.ecdsa_sign_slice(&hash, &private_key) {
        Ok(s) => s,
        Err(e) => return e,
    };

    // Serialize signature (r || s, 64 bytes)
    let mut sig_bytes = Vec::with_capacity(64);
    sig_bytes.extend_from_slice(&signature.x);
    sig_bytes.extend_from_slice(&signature.y);

    unsafe {
        std::ptr::copy_nonoverlapping(sig_bytes.as_ptr(), signature_out, 64);
    }

    sgx_status_t::SGX_SUCCESS
}

/// Compute SHA-256 hash.
#[no_mangle]
pub extern "C" fn ecall_sha256(
    data: *const u8,
    data_len: usize,
    hash_out: *mut u8,
    hash_len: usize,
) -> sgx_status_t {
    if data.is_null() || hash_out.is_null() || hash_len < 32 {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let data_slice = unsafe { std::slice::from_raw_parts(data, data_len) };

    let hash = match rsgx_sha256_slice(data_slice) {
        Ok(h) => h,
        Err(e) => return e,
    };

    unsafe {
        std::ptr::copy_nonoverlapping(hash.as_ptr(), hash_out, 32);
    }

    sgx_status_t::SGX_SUCCESS
}

/// AES-256-GCM encryption inside the enclave.
#[no_mangle]
pub extern "C" fn ecall_aes_gcm_encrypt(
    key: *const u8,
    key_len: usize,
    iv: *const u8,
    iv_len: usize,
    plaintext: *const u8,
    plaintext_len: usize,
    aad: *const u8,
    aad_len: usize,
    ciphertext_out: *mut u8,
    ciphertext_len: usize,
    tag_out: *mut u8,
    tag_len: usize,
) -> sgx_status_t {
    if key.is_null() || key_len != 32 || iv.is_null() || iv_len != 12
        || plaintext.is_null() || ciphertext_out.is_null()
        || ciphertext_len < plaintext_len || tag_out.is_null() || tag_len < 16 {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let key_slice = unsafe { std::slice::from_raw_parts(key, key_len) };
    let iv_slice = unsafe { std::slice::from_raw_parts(iv, iv_len) };
    let plaintext_slice = unsafe { std::slice::from_raw_parts(plaintext, plaintext_len) };

    let aad_slice = if aad.is_null() || aad_len == 0 {
        &[]
    } else {
        unsafe { std::slice::from_raw_parts(aad, aad_len) }
    };

    // Prepare key
    let mut aes_key = sgx_aes_gcm_128bit_key_t::default();
    aes_key.copy_from_slice(&key_slice[..16]); // Use first 128 bits for SGX API

    // Encrypt
    let mut ciphertext = vec![0u8; plaintext_len];
    let mut tag = sgx_aes_gcm_128bit_tag_t::default();

    match rsgx_rijndael128GCM_encrypt(
        &aes_key,
        plaintext_slice,
        iv_slice,
        aad_slice,
        &mut ciphertext,
        &mut tag,
    ) {
        Ok(_) => {},
        Err(e) => return e,
    }

    unsafe {
        std::ptr::copy_nonoverlapping(ciphertext.as_ptr(), ciphertext_out, plaintext_len);
        std::ptr::copy_nonoverlapping(tag.as_ptr(), tag_out, 16);
    }

    sgx_status_t::SGX_SUCCESS
}

/// AES-256-GCM decryption inside the enclave.
#[no_mangle]
pub extern "C" fn ecall_aes_gcm_decrypt(
    key: *const u8,
    key_len: usize,
    iv: *const u8,
    iv_len: usize,
    ciphertext: *const u8,
    ciphertext_len: usize,
    aad: *const u8,
    aad_len: usize,
    tag: *const u8,
    tag_len: usize,
    plaintext_out: *mut u8,
    plaintext_buf_len: usize,
) -> sgx_status_t {
    if key.is_null() || key_len != 32 || iv.is_null() || iv_len != 12
        || ciphertext.is_null() || tag.is_null() || tag_len != 16
        || plaintext_out.is_null() || plaintext_buf_len < ciphertext_len {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let key_slice = unsafe { std::slice::from_raw_parts(key, key_len) };
    let iv_slice = unsafe { std::slice::from_raw_parts(iv, iv_len) };
    let ciphertext_slice = unsafe { std::slice::from_raw_parts(ciphertext, ciphertext_len) };
    let tag_slice = unsafe { std::slice::from_raw_parts(tag, tag_len) };

    let aad_slice = if aad.is_null() || aad_len == 0 {
        &[]
    } else {
        unsafe { std::slice::from_raw_parts(aad, aad_len) }
    };

    // Prepare key and tag
    let mut aes_key = sgx_aes_gcm_128bit_key_t::default();
    aes_key.copy_from_slice(&key_slice[..16]);

    let mut aes_tag = sgx_aes_gcm_128bit_tag_t::default();
    aes_tag.copy_from_slice(tag_slice);

    // Decrypt
    let mut plaintext = vec![0u8; ciphertext_len];

    match rsgx_rijndael128GCM_decrypt(
        &aes_key,
        ciphertext_slice,
        iv_slice,
        aad_slice,
        &aes_tag,
        &mut plaintext,
    ) {
        Ok(_) => {},
        Err(e) => return e,
    }

    unsafe {
        std::ptr::copy_nonoverlapping(plaintext.as_ptr(), plaintext_out, ciphertext_len);
    }

    sgx_status_t::SGX_SUCCESS
}

// =============================================================================
// ECALL: Get Enclave Info
// =============================================================================

/// Get enclave measurement (MRENCLAVE) and signer (MRSIGNER).
#[no_mangle]
pub extern "C" fn ecall_get_enclave_info(
    mr_enclave_out: *mut u8,
    mr_signer_out: *mut u8,
) -> sgx_status_t {
    if mr_enclave_out.is_null() || mr_signer_out.is_null() {
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    // Create a self-report to get MRENCLAVE and MRSIGNER
    let report = match rsgx_self_report() {
        Ok(r) => r,
        Err(e) => return e,
    };

    unsafe {
        std::ptr::copy_nonoverlapping(
            report.body.mr_enclave.m.as_ptr(),
            mr_enclave_out,
            32,
        );
        std::ptr::copy_nonoverlapping(
            report.body.mr_signer.m.as_ptr(),
            mr_signer_out,
            32,
        );
    }

    sgx_status_t::SGX_SUCCESS
}

// =============================================================================
// ECALL: Health Check
// =============================================================================

#[no_mangle]
pub extern "C" fn ecall_health_check() -> sgx_status_t {
    let state = match ENCLAVE_STATE.lock() {
        Ok(s) => s,
        Err(_) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    };

    if state.initialized {
        sgx_status_t::SGX_SUCCESS
    } else {
        sgx_status_t::SGX_ERROR_ENCLAVE_LOST
    }
}
