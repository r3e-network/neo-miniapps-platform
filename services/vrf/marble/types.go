package neovrf

type RandomRequest struct {
	RequestID string `json:"request_id,omitempty"`
}

type RandomResponse struct {
	RequestID       string `json:"request_id"`
	Randomness      string `json:"randomness"`
	Signature       string `json:"signature,omitempty"`
	PublicKey       string `json:"public_key,omitempty"`
	AttestationHash string `json:"attestation_hash,omitempty"`
	Timestamp       int64  `json:"timestamp"`
}

type PublicKeyResponse struct {
	PublicKey       string `json:"public_key"`
	AttestationHash string `json:"attestation_hash,omitempty"`
}
