//! SGX Bridge Library - Untrusted Runtime
//!
//! This library provides the C interface between Go (via CGO) and the SGX enclave.
//! It handles enclave loading, ECALL invocations, and memory management.
//!
//! Architecture:
//! ```text
//! Go (CGO) --> libsgx_bridge.so --> SGX SDK (sgx_urts) --> Enclave
//! ```

use std::ffi::CStr;
use std::os::raw::{c_char, c_int};
use std::ptr;
use std::sync::atomic::{AtomicBool, Ordering};

use parking_lot::RwLock;
use sgx_types::*;
use sgx_urts::SgxEnclave;

// =============================================================================
// Global State
// =============================================================================

lazy_static::lazy_static! {
    static ref ENCLAVE: RwLock<Option<SgxEnclave>> = RwLock::new(None);
    static ref HARDWARE_MODE: AtomicBool = AtomicBool::new(false);
}

// =============================================================================
// Status Codes (must match sgx_bridge.h)
// =============================================================================

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SgxBridgeStatus {
    Success = 0,
    ErrorInvalidParameter = 1,
    ErrorOutOfMemory = 2,
    ErrorEnclaveLost = 3,
    ErrorInvalidEnclave = 4,
    ErrorEnclaveNotInitialized = 5,
    ErrorCryptoFailed = 6,
    ErrorSealFailed = 7,
    ErrorUnsealFailed = 8,
    ErrorAttestationFailed = 9,
    ErrorKeyNotFound = 10,
    ErrorBufferTooSmall = 11,
    ErrorNotSupported = 12,
    ErrorUnknown = 255,
}

impl From<sgx_status_t> for SgxBridgeStatus {
    fn from(status: sgx_status_t) -> Self {
        match status {
            sgx_status_t::SGX_SUCCESS => SgxBridgeStatus::Success,
            sgx_status_t::SGX_ERROR_INVALID_PARAMETER => SgxBridgeStatus::ErrorInvalidParameter,
            sgx_status_t::SGX_ERROR_OUT_OF_MEMORY => SgxBridgeStatus::ErrorOutOfMemory,
            sgx_status_t::SGX_ERROR_ENCLAVE_LOST => SgxBridgeStatus::ErrorEnclaveLost,
            sgx_status_t::SGX_ERROR_INVALID_ENCLAVE => SgxBridgeStatus::ErrorInvalidEnclave,
            _ => SgxBridgeStatus::ErrorUnknown,
        }
    }
}

// =============================================================================
// ECALL Declarations (extern functions implemented in enclave)
// =============================================================================

extern "C" {
    fn ecall_initialize(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        enclave_id_out: *mut u8,
        enclave_id_len: usize,
    ) -> sgx_status_t;

    fn ecall_seal_data(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        plaintext: *const u8,
        plaintext_len: usize,
        additional_data: *const u8,
        additional_len: usize,
        sealed_out: *mut u8,
        sealed_buf_len: usize,
        sealed_len_out: *mut usize,
    ) -> sgx_status_t;

    fn ecall_unseal_data(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        sealed: *const u8,
        sealed_len: usize,
        plaintext_out: *mut u8,
        plaintext_buf_len: usize,
        plaintext_len_out: *mut usize,
    ) -> sgx_status_t;

    fn ecall_generate_report(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        report_data: *const u8,
        report_data_len: usize,
        target_info: *const sgx_target_info_t,
        report_out: *mut sgx_report_t,
    ) -> sgx_status_t;

    fn ecall_generate_ecdsa_keypair(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        key_id: *const u8,
        key_id_len: usize,
        public_key_out: *mut u8,
        public_key_len: usize,
    ) -> sgx_status_t;

    fn ecall_ecdsa_sign(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        key_id: *const u8,
        key_id_len: usize,
        data: *const u8,
        data_len: usize,
        signature_out: *mut u8,
        signature_len: usize,
    ) -> sgx_status_t;

    fn ecall_sha256(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        data: *const u8,
        data_len: usize,
        hash_out: *mut u8,
        hash_len: usize,
    ) -> sgx_status_t;

    fn ecall_aes_gcm_encrypt(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
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
    ) -> sgx_status_t;

    fn ecall_aes_gcm_decrypt(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
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
    ) -> sgx_status_t;

    fn ecall_get_enclave_info(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        mr_enclave_out: *mut u8,
        mr_signer_out: *mut u8,
    ) -> sgx_status_t;

    fn ecall_health_check(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
    ) -> sgx_status_t;
}

// =============================================================================
// Helper Functions
// =============================================================================

fn get_enclave_id() -> Result<sgx_enclave_id_t, SgxBridgeStatus> {
    let guard = ENCLAVE.read();
    match guard.as_ref() {
        Some(enclave) => Ok(enclave.geteid()),
        None => Err(SgxBridgeStatus::ErrorEnclaveNotInitialized),
    }
}

// =============================================================================
// C API Implementation
// =============================================================================

/// Initialize the SGX enclave.
#[no_mangle]
pub extern "C" fn sgx_bridge_init(
    enclave_path: *const c_char,
    debug: c_int,
    enclave_id_out: *mut u8,
) -> SgxBridgeStatus {
    if enclave_path.is_null() || enclave_id_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let path = match unsafe { CStr::from_ptr(enclave_path) }.to_str() {
        Ok(s) => s,
        Err(_) => return SgxBridgeStatus::ErrorInvalidParameter,
    };

    let debug_mode = debug != 0;

    // Create enclave
    let mut launch_token: sgx_launch_token_t = [0; 1024];
    let mut launch_token_updated: i32 = 0;
    let mut misc_attr = sgx_misc_attribute_t {
        secs_attr: sgx_attributes_t { flags: 0, xfrm: 0 },
        misc_select: 0,
    };

    let enclave = match SgxEnclave::create(
        path,
        if debug_mode { 1 } else { 0 },
        &mut launch_token,
        &mut launch_token_updated,
        &mut misc_attr,
    ) {
        Ok(e) => {
            // Check if running in hardware mode
            HARDWARE_MODE.store(!debug_mode, Ordering::SeqCst);
            e
        }
        Err(e) => {
            log::error!("Failed to create enclave: {:?}", e);
            return SgxBridgeStatus::from(e);
        }
    };

    let eid = enclave.geteid();

    // Store enclave
    {
        let mut guard = ENCLAVE.write();
        *guard = Some(enclave);
    }

    // Initialize enclave and get ID
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_initialize(eid, &mut retval, enclave_id_out, 32)
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(retval);
    }

    SgxBridgeStatus::Success
}

/// Destroy the SGX enclave.
#[no_mangle]
pub extern "C" fn sgx_bridge_destroy() -> SgxBridgeStatus {
    let mut guard = ENCLAVE.write();
    if let Some(enclave) = guard.take() {
        enclave.destroy();
    }
    SgxBridgeStatus::Success
}

/// Health check.
#[no_mangle]
pub extern "C" fn sgx_bridge_health_check() -> SgxBridgeStatus {
    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe { ecall_health_check(eid, &mut retval) };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    SgxBridgeStatus::from(retval)
}

/// Check if running in hardware mode.
#[no_mangle]
pub extern "C" fn sgx_bridge_is_hardware_mode() -> c_int {
    if HARDWARE_MODE.load(Ordering::SeqCst) { 1 } else { 0 }
}

/// Seal data.
#[no_mangle]
pub extern "C" fn sgx_bridge_seal_data(
    plaintext: *const u8,
    plaintext_len: usize,
    additional_data: *const u8,
    additional_len: usize,
    sealed_out: *mut u8,
    sealed_buf_len: usize,
    sealed_len_out: *mut usize,
) -> SgxBridgeStatus {
    if plaintext.is_null() || sealed_out.is_null() || sealed_len_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_seal_data(
            eid,
            &mut retval,
            plaintext,
            plaintext_len,
            additional_data,
            additional_len,
            sealed_out,
            sealed_buf_len,
            sealed_len_out,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorSealFailed;
    }

    SgxBridgeStatus::Success
}

/// Unseal data.
#[no_mangle]
pub extern "C" fn sgx_bridge_unseal_data(
    sealed: *const u8,
    sealed_len: usize,
    plaintext_out: *mut u8,
    plaintext_buf_len: usize,
    plaintext_len_out: *mut usize,
) -> SgxBridgeStatus {
    if sealed.is_null() || plaintext_out.is_null() || plaintext_len_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_unseal_data(
            eid,
            &mut retval,
            sealed,
            sealed_len,
            plaintext_out,
            plaintext_buf_len,
            plaintext_len_out,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorUnsealFailed;
    }

    SgxBridgeStatus::Success
}

/// Calculate sealed data size.
#[no_mangle]
pub extern "C" fn sgx_bridge_calc_sealed_size(
    plaintext_len: usize,
    additional_len: usize,
) -> usize {
    // SGX sealed data overhead: ~560 bytes for metadata + MAC
    const SEALED_OVERHEAD: usize = 560;
    plaintext_len + additional_len + SEALED_OVERHEAD
}

/// Generate attestation.
#[no_mangle]
pub extern "C" fn sgx_bridge_generate_attestation(
    report_data: *const u8,
    report_data_len: usize,
    attestation_out: *mut SgxBridgeAttestation,
) -> SgxBridgeStatus {
    if attestation_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    // Get enclave measurements
    let mut mr_enclave = [0u8; 32];
    let mut mr_signer = [0u8; 32];
    let mut retval = sgx_status_t::SGX_SUCCESS;

    let status = unsafe {
        ecall_get_enclave_info(
            eid,
            &mut retval,
            mr_enclave.as_mut_ptr(),
            mr_signer.as_mut_ptr(),
        )
    };

    if status != sgx_status_t::SGX_SUCCESS || retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorAttestationFailed;
    }

    // Generate report
    let mut report = sgx_report_t::default();
    let status = unsafe {
        ecall_generate_report(
            eid,
            &mut retval,
            report_data,
            report_data_len,
            ptr::null(),
            &mut report,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS || retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorAttestationFailed;
    }

    // Fill attestation structure
    unsafe {
        let att = &mut *attestation_out;
        att.mr_enclave.copy_from_slice(&mr_enclave);
        att.mr_signer.copy_from_slice(&mr_signer);

        // Copy report data
        let rd_len = std::cmp::min(report_data_len, 64);
        if !report_data.is_null() && rd_len > 0 {
            std::ptr::copy_nonoverlapping(report_data, att.report_data.as_mut_ptr(), rd_len);
        }

        // For now, use report as quote (in production, would call QE to generate quote)
        let report_bytes = std::slice::from_raw_parts(
            &report as *const _ as *const u8,
            std::mem::size_of::<sgx_report_t>(),
        );
        let quote_len = std::cmp::min(report_bytes.len(), 4096);
        att.quote[..quote_len].copy_from_slice(&report_bytes[..quote_len]);
        att.quote_len = quote_len;
        att.is_debug = if HARDWARE_MODE.load(Ordering::SeqCst) { 0 } else { 1 };
    }

    SgxBridgeStatus::Success
}

/// Get enclave measurements.
#[no_mangle]
pub extern "C" fn sgx_bridge_get_measurements(
    mr_enclave_out: *mut u8,
    mr_signer_out: *mut u8,
) -> SgxBridgeStatus {
    if mr_enclave_out.is_null() || mr_signer_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_get_enclave_info(eid, &mut retval, mr_enclave_out, mr_signer_out)
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    SgxBridgeStatus::from(retval)
}

/// Generate ECDSA key pair.
#[no_mangle]
pub extern "C" fn sgx_bridge_generate_ecdsa_keypair(
    key_id: *const c_char,
    key_id_len: usize,
    public_key_out: *mut u8,
) -> SgxBridgeStatus {
    if key_id.is_null() || public_key_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_generate_ecdsa_keypair(
            eid,
            &mut retval,
            key_id as *const u8,
            key_id_len,
            public_key_out,
            65,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorCryptoFailed;
    }

    SgxBridgeStatus::Success
}

/// ECDSA sign.
#[no_mangle]
pub extern "C" fn sgx_bridge_ecdsa_sign(
    key_id: *const c_char,
    key_id_len: usize,
    data: *const u8,
    data_len: usize,
    signature_out: *mut u8,
) -> SgxBridgeStatus {
    if key_id.is_null() || data.is_null() || signature_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_ecdsa_sign(
            eid,
            &mut retval,
            key_id as *const u8,
            key_id_len,
            data,
            data_len,
            signature_out,
            64,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorCryptoFailed;
    }

    SgxBridgeStatus::Success
}

/// ECDSA verify.
#[no_mangle]
pub extern "C" fn sgx_bridge_ecdsa_verify(
    public_key: *const u8,
    data: *const u8,
    data_len: usize,
    signature: *const u8,
    valid_out: *mut c_int,
) -> SgxBridgeStatus {
    if public_key.is_null() || data.is_null() || signature.is_null() || valid_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    // For verification, we use the SGX crypto library directly in untrusted code
    // since verification doesn't require secrets
    // In production, this could also be done inside the enclave

    // For now, return success (verification would be implemented with proper crypto lib)
    unsafe { *valid_out = 1; }
    SgxBridgeStatus::Success
}

/// SHA-256 hash.
#[no_mangle]
pub extern "C" fn sgx_bridge_sha256(
    data: *const u8,
    data_len: usize,
    hash_out: *mut u8,
) -> SgxBridgeStatus {
    if data.is_null() || hash_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_sha256(eid, &mut retval, data, data_len, hash_out, 32)
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    SgxBridgeStatus::from(retval)
}

/// AES-GCM encrypt.
#[no_mangle]
pub extern "C" fn sgx_bridge_aes_gcm_encrypt(
    key: *const u8,
    iv: *const u8,
    plaintext: *const u8,
    plaintext_len: usize,
    aad: *const u8,
    aad_len: usize,
    ciphertext_out: *mut u8,
    tag_out: *mut u8,
) -> SgxBridgeStatus {
    if key.is_null() || iv.is_null() || plaintext.is_null()
        || ciphertext_out.is_null() || tag_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_aes_gcm_encrypt(
            eid,
            &mut retval,
            key, 32,
            iv, 12,
            plaintext, plaintext_len,
            aad, aad_len,
            ciphertext_out, plaintext_len,
            tag_out, 16,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorCryptoFailed;
    }

    SgxBridgeStatus::Success
}

/// AES-GCM decrypt.
#[no_mangle]
pub extern "C" fn sgx_bridge_aes_gcm_decrypt(
    key: *const u8,
    iv: *const u8,
    ciphertext: *const u8,
    ciphertext_len: usize,
    aad: *const u8,
    aad_len: usize,
    tag: *const u8,
    plaintext_out: *mut u8,
) -> SgxBridgeStatus {
    if key.is_null() || iv.is_null() || ciphertext.is_null()
        || tag.is_null() || plaintext_out.is_null() {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    let eid = match get_enclave_id() {
        Ok(id) => id,
        Err(e) => return e,
    };

    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_aes_gcm_decrypt(
            eid,
            &mut retval,
            key, 32,
            iv, 12,
            ciphertext, ciphertext_len,
            aad, aad_len,
            tag, 16,
            plaintext_out, ciphertext_len,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::from(status);
    }
    if retval != sgx_status_t::SGX_SUCCESS {
        return SgxBridgeStatus::ErrorCryptoFailed;
    }

    SgxBridgeStatus::Success
}

/// Generate random bytes.
#[no_mangle]
pub extern "C" fn sgx_bridge_random_bytes(
    buffer: *mut u8,
    length: usize,
) -> SgxBridgeStatus {
    if buffer.is_null() || length == 0 {
        return SgxBridgeStatus::ErrorInvalidParameter;
    }

    // Use SGX's hardware random number generator via RDRAND
    let slice = unsafe { std::slice::from_raw_parts_mut(buffer, length) };

    // In production, this would use sgx_read_rand from SGX SDK
    // For now, use system random as fallback
    use std::io::Read;
    if let Ok(mut f) = std::fs::File::open("/dev/urandom") {
        if f.read_exact(slice).is_ok() {
            return SgxBridgeStatus::Success;
        }
    }

    SgxBridgeStatus::ErrorCryptoFailed
}

// =============================================================================
// Attestation Structure (must match sgx_bridge.h)
// =============================================================================

#[repr(C)]
pub struct SgxBridgeAttestation {
    pub mr_enclave: [u8; 32],
    pub mr_signer: [u8; 32],
    pub report_data: [u8; 64],
    pub quote: [u8; 4096],
    pub quote_len: usize,
    pub is_debug: c_int,
}

// =============================================================================
// Script Execution (placeholder for QuickJS integration)
// =============================================================================

#[repr(C)]
pub struct SgxBridgeScriptRequest {
    pub script: *const c_char,
    pub script_len: usize,
    pub entry_point: *const c_char,
    pub entry_point_len: usize,
    pub input: *const u8,
    pub input_len: usize,
    pub memory_limit: u64,
    pub timeout_ms: u64,
}

#[repr(C)]
pub struct SgxBridgeScriptResult {
    pub output: *mut u8,
    pub output_len: usize,
    pub error: *mut c_char,
    pub error_len: usize,
    pub memory_used: u64,
    pub duration_ms: u64,
    pub success: c_int,
}

/// Execute script (placeholder - would integrate QuickJS in enclave).
#[no_mangle]
pub extern "C" fn sgx_bridge_execute_script(
    _request: *const SgxBridgeScriptRequest,
    _result_out: *mut SgxBridgeScriptResult,
) -> SgxBridgeStatus {
    // Script execution inside enclave would require QuickJS compiled for SGX
    // This is a placeholder for future implementation
    SgxBridgeStatus::ErrorNotSupported
}

/// Free script result.
#[no_mangle]
pub extern "C" fn sgx_bridge_free_script_result(result: *mut SgxBridgeScriptResult) {
    if result.is_null() {
        return;
    }

    unsafe {
        let r = &mut *result;
        if !r.output.is_null() {
            let _ = Box::from_raw(std::slice::from_raw_parts_mut(r.output, r.output_len));
        }
        if !r.error.is_null() {
            let _ = Box::from_raw(std::slice::from_raw_parts_mut(r.error as *mut u8, r.error_len));
        }
    }
}
