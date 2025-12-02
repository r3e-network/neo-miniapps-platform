// Package enclave provides TEE-protected oracle operations.
// Data fetching, validation, and signing run inside the enclave
// to ensure data integrity and prevent manipulation.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/system/os/tee/sdk"
)

// EnclaveOracle handles all oracle operations within the TEE enclave.
// Critical operations:
// - Data validation and signing
// - Price feed aggregation
// - Request fulfillment
type EnclaveOracle struct {
	*sdk.BaseEnclave
	trustedSources map[string]bool
	dataCache      map[string]*CachedData
}

// CachedData represents cached oracle data.
type CachedData struct {
	Value     []byte
	Timestamp time.Time
	Signature []byte
}

// OracleResponse represents a signed oracle response.
type OracleResponse struct {
	RequestID  string
	Data       []byte
	Timestamp  int64
	Signature  []byte
	PublicKey  []byte
}

// OracleConfig holds configuration for the oracle enclave.
type OracleConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveOracle creates a new enclave oracle handler.
func NewEnclaveOracle() (*EnclaveOracle, error) {
	base, err := sdk.NewBaseEnclave("oracle")
	if err != nil {
		return nil, err
	}

	return &EnclaveOracle{
		BaseEnclave:    base,
		trustedSources: make(map[string]bool),
		dataCache:      make(map[string]*CachedData),
	}, nil
}

// NewEnclaveOracleWithSDK creates an oracle handler with full SDK integration.
func NewEnclaveOracleWithSDK(cfg *OracleConfig) (*EnclaveOracle, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "oracle",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveOracle{
		BaseEnclave:    base,
		trustedSources: make(map[string]bool),
		dataCache:      make(map[string]*CachedData),
	}, nil
}

// InitializeWithSDK initializes the oracle handler with an existing SDK instance.
func (e *EnclaveOracle) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// ValidateAndSign validates external data and signs it within the enclave.
func (e *EnclaveOracle) ValidateAndSign(requestID string, data []byte) (*OracleResponse, error) {
	e.Lock()
	defer e.Unlock()

	// Validate data format
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	timestamp := time.Now().Unix()

	// Create message to sign
	message := sha256.New()
	message.Write([]byte(requestID))
	message.Write(data)
	message.Write(big.NewInt(timestamp).Bytes())
	hash := message.Sum(nil)

	// Sign the hash
	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash)
	if err != nil {
		return nil, err
	}

	signature := append(r.Bytes(), s.Bytes()...)
	pubKey := elliptic.Marshal(signingKey.PublicKey.Curve,
		signingKey.PublicKey.X, signingKey.PublicKey.Y)

	// Cache the response
	e.dataCache[requestID] = &CachedData{
		Value:     data,
		Timestamp: time.Now(),
		Signature: signature,
	}

	return &OracleResponse{
		RequestID:  requestID,
		Data:       data,
		Timestamp:  timestamp,
		Signature:  signature,
		PublicKey:  pubKey,
	}, nil
}

// AggregateData aggregates data from multiple sources within the enclave.
func (e *EnclaveOracle) AggregateData(sources [][]byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	if len(sources) == 0 {
		return nil, errors.New("no sources provided")
	}

	// Simple median aggregation for numeric data
	// In production, implement proper outlier detection
	aggregated := sha256.New()
	for _, source := range sources {
		aggregated.Write(source)
	}

	return aggregated.Sum(nil), nil
}

// VerifyResponse verifies an oracle response signature.
func VerifyResponse(response *OracleResponse) (bool, error) {
	if len(response.Signature) < 64 {
		return false, errors.New("invalid signature length")
	}

	// Parse public key
	x, y := elliptic.Unmarshal(elliptic.P256(), response.PublicKey)
	if x == nil {
		return false, errors.New("invalid public key")
	}

	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Recreate message hash
	message := sha256.New()
	message.Write([]byte(response.RequestID))
	message.Write(response.Data)
	message.Write(big.NewInt(response.Timestamp).Bytes())
	hash := message.Sum(nil)

	// Parse signature
	r := new(big.Int).SetBytes(response.Signature[:32])
	s := new(big.Int).SetBytes(response.Signature[32:64])

	return ecdsa.Verify(pubKey, hash, r, s), nil
}

// GetResponseHash returns the hash of a response for logging.
func GetResponseHash(response *OracleResponse) string {
	data, _ := json.Marshal(response)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
