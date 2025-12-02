/**
 * SGX Bridge Header - C Interface for Go CGO
 *
 * This header defines the C interface between Go and the Rust SGX enclave.
 * The bridge library (libsgx_bridge.so) wraps the SGX SDK calls and provides
 * a simple C API that can be called from Go via CGO.
 *
 * Architecture:
 *
 *   Go (CGO) --> libsgx_bridge.so --> SGX SDK --> Enclave
 *
 * Build modes:
 *   - Hardware mode: Links against Intel SGX SDK, requires SGX hardware
 *   - Simulation mode: Links against SGX simulation libraries
 */

#ifndef SGX_BRIDGE_H
#define SGX_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* =============================================================================
 * Error Codes
 * ============================================================================= */

typedef enum {
    SGX_BRIDGE_SUCCESS = 0,
    SGX_BRIDGE_ERROR_INVALID_PARAMETER = 1,
    SGX_BRIDGE_ERROR_OUT_OF_MEMORY = 2,
    SGX_BRIDGE_ERROR_ENCLAVE_LOST = 3,
    SGX_BRIDGE_ERROR_INVALID_ENCLAVE = 4,
    SGX_BRIDGE_ERROR_ENCLAVE_NOT_INITIALIZED = 5,
    SGX_BRIDGE_ERROR_CRYPTO_FAILED = 6,
    SGX_BRIDGE_ERROR_SEAL_FAILED = 7,
    SGX_BRIDGE_ERROR_UNSEAL_FAILED = 8,
    SGX_BRIDGE_ERROR_ATTESTATION_FAILED = 9,
    SGX_BRIDGE_ERROR_KEY_NOT_FOUND = 10,
    SGX_BRIDGE_ERROR_BUFFER_TOO_SMALL = 11,
    SGX_BRIDGE_ERROR_NOT_SUPPORTED = 12,
    SGX_BRIDGE_ERROR_UNKNOWN = 255,
} sgx_bridge_status_t;

/* =============================================================================
 * Enclave Lifecycle
 * ============================================================================= */

/**
 * Initialize the SGX enclave.
 *
 * @param enclave_path  Path to the signed enclave binary (.signed.so)
 * @param debug         Enable debug mode (1) or not (0)
 * @param enclave_id    Output: Enclave ID (32 bytes)
 * @return              SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_init(
    const char* enclave_path,
    int debug,
    uint8_t* enclave_id
);

/**
 * Destroy the SGX enclave and release resources.
 *
 * @return  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_destroy(void);

/**
 * Check if the enclave is healthy.
 *
 * @return  SGX_BRIDGE_SUCCESS if healthy
 */
sgx_bridge_status_t sgx_bridge_health_check(void);

/**
 * Get enclave mode (hardware or simulation).
 *
 * @return  1 for hardware mode, 0 for simulation mode
 */
int sgx_bridge_is_hardware_mode(void);

/* =============================================================================
 * Sealing Operations (using SGX EGETKEY)
 * ============================================================================= */

/**
 * Seal data using the enclave's sealing key.
 * Uses MRSIGNER policy for key derivation.
 *
 * @param plaintext         Data to seal
 * @param plaintext_len     Length of plaintext
 * @param additional_data   Additional authenticated data (can be NULL)
 * @param additional_len    Length of additional data
 * @param sealed_out        Output buffer for sealed data
 * @param sealed_buf_len    Size of output buffer
 * @param sealed_len_out    Output: Actual sealed data length
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_seal_data(
    const uint8_t* plaintext,
    size_t plaintext_len,
    const uint8_t* additional_data,
    size_t additional_len,
    uint8_t* sealed_out,
    size_t sealed_buf_len,
    size_t* sealed_len_out
);

/**
 * Unseal data that was previously sealed.
 *
 * @param sealed            Sealed data
 * @param sealed_len        Length of sealed data
 * @param plaintext_out     Output buffer for plaintext
 * @param plaintext_buf_len Size of output buffer
 * @param plaintext_len_out Output: Actual plaintext length
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_unseal_data(
    const uint8_t* sealed,
    size_t sealed_len,
    uint8_t* plaintext_out,
    size_t plaintext_buf_len,
    size_t* plaintext_len_out
);

/**
 * Calculate the sealed data size for a given plaintext size.
 *
 * @param plaintext_len     Length of plaintext
 * @param additional_len    Length of additional data
 * @return                  Required buffer size for sealed data
 */
size_t sgx_bridge_calc_sealed_size(
    size_t plaintext_len,
    size_t additional_len
);

/* =============================================================================
 * Remote Attestation
 * ============================================================================= */

/**
 * Attestation report structure.
 */
typedef struct {
    uint8_t mr_enclave[32];     /* MRENCLAVE measurement */
    uint8_t mr_signer[32];      /* MRSIGNER measurement */
    uint8_t report_data[64];    /* User-provided report data */
    uint8_t quote[4096];        /* SGX quote (variable length) */
    size_t quote_len;           /* Actual quote length */
    int is_debug;               /* Debug enclave flag */
} sgx_bridge_attestation_t;

/**
 * Generate an attestation report/quote.
 *
 * @param report_data       User data to include in report (max 64 bytes)
 * @param report_data_len   Length of report data
 * @param attestation_out   Output: Attestation structure
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_generate_attestation(
    const uint8_t* report_data,
    size_t report_data_len,
    sgx_bridge_attestation_t* attestation_out
);

/**
 * Get enclave measurements (MRENCLAVE and MRSIGNER).
 *
 * @param mr_enclave_out    Output: MRENCLAVE (32 bytes)
 * @param mr_signer_out     Output: MRSIGNER (32 bytes)
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_get_measurements(
    uint8_t* mr_enclave_out,
    uint8_t* mr_signer_out
);

/* =============================================================================
 * Cryptographic Operations (inside enclave)
 * ============================================================================= */

/**
 * Generate an ECDSA P-256 key pair inside the enclave.
 *
 * @param key_id            Unique identifier for the key
 * @param key_id_len        Length of key ID
 * @param public_key_out    Output: Public key (65 bytes, uncompressed)
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_generate_ecdsa_keypair(
    const char* key_id,
    size_t key_id_len,
    uint8_t* public_key_out
);

/**
 * Sign data using ECDSA P-256.
 *
 * @param key_id            Key identifier
 * @param key_id_len        Length of key ID
 * @param data              Data to sign
 * @param data_len          Length of data
 * @param signature_out     Output: Signature (64 bytes, r||s)
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_ecdsa_sign(
    const char* key_id,
    size_t key_id_len,
    const uint8_t* data,
    size_t data_len,
    uint8_t* signature_out
);

/**
 * Verify an ECDSA P-256 signature.
 *
 * @param public_key        Public key (65 bytes, uncompressed)
 * @param data              Original data
 * @param data_len          Length of data
 * @param signature         Signature to verify (64 bytes)
 * @param valid_out         Output: 1 if valid, 0 if invalid
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_ecdsa_verify(
    const uint8_t* public_key,
    const uint8_t* data,
    size_t data_len,
    const uint8_t* signature,
    int* valid_out
);

/**
 * Compute SHA-256 hash inside the enclave.
 *
 * @param data              Data to hash
 * @param data_len          Length of data
 * @param hash_out          Output: Hash (32 bytes)
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_sha256(
    const uint8_t* data,
    size_t data_len,
    uint8_t* hash_out
);

/**
 * AES-256-GCM encryption inside the enclave.
 *
 * @param key               Encryption key (32 bytes)
 * @param iv                Initialization vector (12 bytes)
 * @param plaintext         Data to encrypt
 * @param plaintext_len     Length of plaintext
 * @param aad               Additional authenticated data (can be NULL)
 * @param aad_len           Length of AAD
 * @param ciphertext_out    Output: Ciphertext (same length as plaintext)
 * @param tag_out           Output: Authentication tag (16 bytes)
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_aes_gcm_encrypt(
    const uint8_t* key,
    const uint8_t* iv,
    const uint8_t* plaintext,
    size_t plaintext_len,
    const uint8_t* aad,
    size_t aad_len,
    uint8_t* ciphertext_out,
    uint8_t* tag_out
);

/**
 * AES-256-GCM decryption inside the enclave.
 *
 * @param key               Decryption key (32 bytes)
 * @param iv                Initialization vector (12 bytes)
 * @param ciphertext        Data to decrypt
 * @param ciphertext_len    Length of ciphertext
 * @param aad               Additional authenticated data (can be NULL)
 * @param aad_len           Length of AAD
 * @param tag               Authentication tag (16 bytes)
 * @param plaintext_out     Output: Plaintext
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_aes_gcm_decrypt(
    const uint8_t* key,
    const uint8_t* iv,
    const uint8_t* ciphertext,
    size_t ciphertext_len,
    const uint8_t* aad,
    size_t aad_len,
    const uint8_t* tag,
    uint8_t* plaintext_out
);

/**
 * Generate cryptographically secure random bytes inside the enclave.
 *
 * @param buffer            Output buffer
 * @param length            Number of random bytes to generate
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_random_bytes(
    uint8_t* buffer,
    size_t length
);

/* =============================================================================
 * Script Execution (JavaScript in enclave)
 * ============================================================================= */

/**
 * Script execution request.
 */
typedef struct {
    const char* script;         /* JavaScript source code */
    size_t script_len;          /* Length of script */
    const char* entry_point;    /* Function to call */
    size_t entry_point_len;     /* Length of entry point name */
    const uint8_t* input;       /* JSON-encoded input */
    size_t input_len;           /* Length of input */
    uint64_t memory_limit;      /* Memory limit in bytes */
    uint64_t timeout_ms;        /* Execution timeout in milliseconds */
} sgx_bridge_script_request_t;

/**
 * Script execution result.
 */
typedef struct {
    uint8_t* output;            /* JSON-encoded output (caller must free) */
    size_t output_len;          /* Length of output */
    char* error;                /* Error message if failed (caller must free) */
    size_t error_len;           /* Length of error message */
    uint64_t memory_used;       /* Memory used in bytes */
    uint64_t duration_ms;       /* Execution duration in milliseconds */
    int success;                /* 1 if successful, 0 if failed */
} sgx_bridge_script_result_t;

/**
 * Execute JavaScript inside the enclave.
 *
 * @param request           Execution request
 * @param result_out        Output: Execution result
 * @return                  SGX_BRIDGE_SUCCESS on success
 */
sgx_bridge_status_t sgx_bridge_execute_script(
    const sgx_bridge_script_request_t* request,
    sgx_bridge_script_result_t* result_out
);

/**
 * Free script result resources.
 *
 * @param result            Result to free
 */
void sgx_bridge_free_script_result(sgx_bridge_script_result_t* result);

#ifdef __cplusplus
}
#endif

#endif /* SGX_BRIDGE_H */
