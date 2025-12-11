package neoaccountsmarble

import (
	"encoding/base64"
	"time"

	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/enclave"
)

// MasterKeyAttestation is a non-sensitive bundle proving the master key hash
// is bound to enclave report data. The quote is intended for off-chain
// verification; the account pool does not parse or validate it here.
type MasterKeyAttestation struct {
	Hash      string `json:"hash"`
	PubKey    string `json:"pubkey,omitempty"`
	Quote     string `json:"quote,omitempty"`
	MRENCLAVE string `json:"mrenclave,omitempty"`
	MRSIGNER  string `json:"mrsigner,omitempty"`
	ProdID    uint16 `json:"prod_id,omitempty"`
	ISVSVN    uint16 `json:"isvsvn,omitempty"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Simulated bool   `json:"simulated"`
}

func (s *Service) buildMasterKeyAttestation() MasterKeyAttestation {
	summary := s.masterKeySummary()
	att := MasterKeyAttestation{
		Hash:      summary.Hash,
		PubKey:    summary.PubKeyHex,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "neoaccounts",
		Simulated: !s.Marble().IsEnclave(),
	}

	// Only produce a quote when running inside an enclave.
	if !s.Marble().IsEnclave() {
		return att
	}

	// Use master key hash as report data to bind the key to the attestation.
	report, quote, err := getQuote([]byte(summary.Hash))
	if err != nil {
		return att
	}

	att.Quote = base64.StdEncoding.EncodeToString(quote)
	att.MRENCLAVE = base64.StdEncoding.EncodeToString(report.UniqueID)
	att.MRSIGNER = base64.StdEncoding.EncodeToString(report.SignerID)
	if len(report.ProductID) >= 2 {
		att.ProdID = uint16(report.ProductID[1])<<8 | uint16(report.ProductID[0])
	}
	att.ISVSVN = uint16(report.SecurityVersion)
	return att
}

// getQuote returns the SGX report and raw quote with the given user data.
func getQuote(userData []byte) (*attestation.Report, []byte, error) {
	if len(userData) > 64 {
		userData = userData[:64]
	}
	if len(userData) < 64 {
		padded := make([]byte, 64)
		copy(padded, userData)
		userData = padded
	}

	quote, err := enclave.GetRemoteReport(userData)
	if err != nil {
		return nil, nil, err
	}
	report, err := enclave.VerifyRemoteReport(quote)
	if err != nil {
		return nil, nil, err
	}
	return &report, quote, nil
}
