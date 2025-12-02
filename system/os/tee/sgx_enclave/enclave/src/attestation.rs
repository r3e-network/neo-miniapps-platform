//! SGX Remote Attestation
//!
//! This module provides remote attestation capabilities using SGX's
//! quote generation and verification mechanisms.
//!
//! Attestation flow:
//! 1. Enclave generates a REPORT using EREPORT instruction
//! 2. Quoting Enclave (QE) converts REPORT to QUOTE
//! 3. Quote is sent to Intel Attestation Service (IAS) or verified locally (DCAP)
//! 4. Verifier confirms enclave identity and integrity

use std::prelude::v1::*;
use std::vec::Vec;

use sgx_types::*;
use sgx_tse::*;

use crate::types::{AttestationData, EnclaveError, EnclaveResult};

/// Generate an SGX report for local attestation.
///
/// # Arguments
/// * `target_info` - Target enclave's information (for local attestation)
/// * `report_data` - User-provided data to include in report (max 64 bytes)
///
/// # Returns
/// SGX report structure
pub fn generate_report(
    target_info: Option<&sgx_target_info_t>,
    report_data: &[u8],
) -> EnclaveResult<sgx_report_t> {
    // Prepare report data (64 bytes max)
    let mut rd = sgx_report_data_t::default();
    let len = std::cmp::min(report_data.len(), 64);
    rd.d[..len].copy_from_slice(&report_data[..len]);

    // Use provided target info or default (self-report)
    let ti = target_info.cloned().unwrap_or_default();

    // Create the report using EREPORT instruction
    rsgx_create_report(&ti, &rd)
        .map_err(|e| EnclaveError::CryptoError(format!("Report generation failed: {:?}", e)))
}

/// Generate a self-report (report targeting self).
pub fn generate_self_report(report_data: &[u8]) -> EnclaveResult<sgx_report_t> {
    generate_report(None, report_data)
}

/// Get enclave measurements from self-report.
pub fn get_enclave_measurements() -> EnclaveResult<AttestationData> {
    let report = rsgx_self_report()
        .map_err(|e| EnclaveError::CryptoError(format!("Self report failed: {:?}", e)))?;

    let mut data = AttestationData::default();

    // Copy MRENCLAVE
    data.mr_enclave.copy_from_slice(&report.body.mr_enclave.m);

    // Copy MRSIGNER
    data.mr_signer.copy_from_slice(&report.body.mr_signer.m);

    // Copy ISV product ID and SVN
    data.isv_prod_id = report.body.isv_prod_id;
    data.isv_svn = report.body.isv_svn;

    // Check debug flag
    data.is_debug = (report.body.attributes.flags & SGX_FLAGS_DEBUG) != 0;

    Ok(data)
}

/// Verify a report from another enclave (local attestation).
///
/// # Arguments
/// * `report` - Report to verify
///
/// # Returns
/// true if the report is valid
pub fn verify_report(report: &sgx_report_t) -> EnclaveResult<bool> {
    // Verify the report's MAC using our report key
    match rsgx_verify_report(report) {
        Ok(_) => Ok(true),
        Err(sgx_status_t::SGX_ERROR_MAC_MISMATCH) => Ok(false),
        Err(e) => Err(EnclaveError::CryptoError(format!("Report verification failed: {:?}", e))),
    }
}

/// Quote structure for remote attestation.
/// This is a simplified representation; actual SGX quotes are more complex.
#[derive(Clone)]
pub struct Quote {
    /// Quote version
    pub version: u16,
    /// Signature type (EPID or ECDSA)
    pub sign_type: u16,
    /// EPID group ID or QE SVN
    pub epid_group_id: [u8; 4],
    /// QE SVN
    pub qe_svn: u16,
    /// PCE SVN
    pub pce_svn: u16,
    /// Extended EPID group ID
    pub xeid: u32,
    /// Basename
    pub basename: [u8; 32],
    /// Report body
    pub report_body: ReportBody,
    /// Signature length
    pub signature_len: u32,
    /// Signature data
    pub signature: Vec<u8>,
}

/// Report body within a quote.
#[derive(Clone, Default)]
pub struct ReportBody {
    /// CPU SVN
    pub cpu_svn: [u8; 16],
    /// Misc select
    pub misc_select: u32,
    /// Reserved
    pub reserved1: [u8; 28],
    /// Attributes
    pub attributes: Attributes,
    /// MRENCLAVE
    pub mr_enclave: [u8; 32],
    /// Reserved
    pub reserved2: [u8; 32],
    /// MRSIGNER
    pub mr_signer: [u8; 32],
    /// Reserved
    pub reserved3: [u8; 96],
    /// ISV Product ID
    pub isv_prod_id: u16,
    /// ISV SVN
    pub isv_svn: u16,
    /// Reserved
    pub reserved4: [u8; 60],
    /// Report data
    pub report_data: [u8; 64],
}

/// Enclave attributes.
#[derive(Clone, Default)]
pub struct Attributes {
    /// Flags
    pub flags: u64,
    /// XFRM
    pub xfrm: u64,
}

impl Quote {
    /// Create a quote from a report.
    /// Note: In production, this would call the Quoting Enclave.
    pub fn from_report(report: &sgx_report_t) -> Self {
        let mut quote = Quote {
            version: 3,
            sign_type: 0, // EPID
            epid_group_id: [0; 4],
            qe_svn: 0,
            pce_svn: 0,
            xeid: 0,
            basename: [0; 32],
            report_body: ReportBody::default(),
            signature_len: 0,
            signature: Vec::new(),
        };

        // Copy report body fields
        quote.report_body.cpu_svn.copy_from_slice(&report.body.cpu_svn.svn);
        quote.report_body.misc_select = report.body.misc_select;
        quote.report_body.attributes.flags = report.body.attributes.flags;
        quote.report_body.attributes.xfrm = report.body.attributes.xfrm;
        quote.report_body.mr_enclave.copy_from_slice(&report.body.mr_enclave.m);
        quote.report_body.mr_signer.copy_from_slice(&report.body.mr_signer.m);
        quote.report_body.isv_prod_id = report.body.isv_prod_id;
        quote.report_body.isv_svn = report.body.isv_svn;
        quote.report_body.report_data.copy_from_slice(&report.body.report_data.d);

        quote
    }

    /// Serialize quote to bytes.
    pub fn to_bytes(&self) -> Vec<u8> {
        let mut bytes = Vec::new();

        // Version and sign type
        bytes.extend_from_slice(&self.version.to_le_bytes());
        bytes.extend_from_slice(&self.sign_type.to_le_bytes());

        // EPID group ID
        bytes.extend_from_slice(&self.epid_group_id);

        // QE and PCE SVN
        bytes.extend_from_slice(&self.qe_svn.to_le_bytes());
        bytes.extend_from_slice(&self.pce_svn.to_le_bytes());

        // XEID
        bytes.extend_from_slice(&self.xeid.to_le_bytes());

        // Basename
        bytes.extend_from_slice(&self.basename);

        // Report body
        bytes.extend_from_slice(&self.report_body.cpu_svn);
        bytes.extend_from_slice(&self.report_body.misc_select.to_le_bytes());
        bytes.extend_from_slice(&self.report_body.reserved1);
        bytes.extend_from_slice(&self.report_body.attributes.flags.to_le_bytes());
        bytes.extend_from_slice(&self.report_body.attributes.xfrm.to_le_bytes());
        bytes.extend_from_slice(&self.report_body.mr_enclave);
        bytes.extend_from_slice(&self.report_body.reserved2);
        bytes.extend_from_slice(&self.report_body.mr_signer);
        bytes.extend_from_slice(&self.report_body.reserved3);
        bytes.extend_from_slice(&self.report_body.isv_prod_id.to_le_bytes());
        bytes.extend_from_slice(&self.report_body.isv_svn.to_le_bytes());
        bytes.extend_from_slice(&self.report_body.reserved4);
        bytes.extend_from_slice(&self.report_body.report_data);

        // Signature
        bytes.extend_from_slice(&self.signature_len.to_le_bytes());
        bytes.extend_from_slice(&self.signature);

        bytes
    }
}

/// Attestation evidence for remote verification.
#[derive(Clone)]
pub struct AttestationEvidence {
    /// The quote
    pub quote: Quote,
    /// Platform certificate chain (for DCAP)
    pub cert_chain: Option<Vec<u8>>,
    /// Collateral data
    pub collateral: Option<Vec<u8>>,
}

impl AttestationEvidence {
    /// Create attestation evidence from a report.
    pub fn from_report(report: &sgx_report_t) -> Self {
        Self {
            quote: Quote::from_report(report),
            cert_chain: None,
            collateral: None,
        }
    }

    /// Get the quote bytes.
    pub fn quote_bytes(&self) -> Vec<u8> {
        self.quote.to_bytes()
    }
}

/// Channel binding data for TLS integration.
/// Used to bind attestation to a TLS session.
#[derive(Clone)]
pub struct ChannelBinding {
    /// TLS session hash
    pub session_hash: [u8; 32],
    /// Timestamp
    pub timestamp: u64,
    /// Nonce
    pub nonce: [u8; 32],
}

impl ChannelBinding {
    /// Create channel binding data.
    pub fn new(session_hash: [u8; 32], nonce: [u8; 32]) -> Self {
        Self {
            session_hash,
            timestamp: 0, // Would be set from trusted time source
            nonce,
        }
    }

    /// Serialize to bytes for inclusion in report data.
    pub fn to_bytes(&self) -> [u8; 64] {
        let mut bytes = [0u8; 64];
        bytes[..32].copy_from_slice(&self.session_hash);
        bytes[32..64].copy_from_slice(&self.nonce);
        bytes
    }
}

// SGX flags
const SGX_FLAGS_DEBUG: u64 = 0x0000000000000002;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_get_measurements() {
        let data = get_enclave_measurements().unwrap();
        // MRENCLAVE and MRSIGNER should be non-zero in a real enclave
        assert_eq!(data.mr_enclave.len(), 32);
        assert_eq!(data.mr_signer.len(), 32);
    }

    #[test]
    fn test_generate_self_report() {
        let report_data = b"test data for report";
        let report = generate_self_report(report_data).unwrap();

        // Verify the report data was included
        assert_eq!(&report.body.report_data.d[..report_data.len()], report_data);
    }

    #[test]
    fn test_quote_serialization() {
        let report = generate_self_report(b"test").unwrap();
        let quote = Quote::from_report(&report);
        let bytes = quote.to_bytes();

        // Quote should have reasonable size
        assert!(bytes.len() > 400);
    }
}
