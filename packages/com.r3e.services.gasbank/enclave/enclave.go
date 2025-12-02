// Package enclave provides TEE-protected gas bank operations.
// Balance management, fee calculations, and settlements run inside
// the enclave to ensure integrity and prevent manipulation.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// EnclaveGasBank handles all gas bank operations within the TEE enclave.
// Critical operations:
// - Balance calculations
// - Fee deductions
// - Settlement signing
type EnclaveGasBank struct {
	*sdk.BaseEnclave
	balances   map[string]*big.Int
	pendingTxs map[string]*GasTransaction
	feeRate    *big.Int
}

// GasTransaction represents a gas transaction.
type GasTransaction struct {
	TxID      string
	From      string
	Amount    *big.Int
	Fee       *big.Int
	Timestamp int64
	Status    string
}

// SettlementProof represents a signed settlement proof.
type SettlementProof struct {
	SettlementID string
	TotalAmount  *big.Int
	FeeCollected *big.Int
	Signature    []byte
	PublicKey    []byte
}

// GasBankConfig holds configuration for the gas bank enclave.
type GasBankConfig struct {
	ServiceID  string
	RequestID  string
	CallerID   string
	AccountID  string
	SealKey    []byte
	FeeRateBps int64
}

// NewEnclaveGasBank creates a new enclave gas bank handler.
func NewEnclaveGasBank(feeRateBps int64) (*EnclaveGasBank, error) {
	base, err := sdk.NewBaseEnclave("gasbank")
	if err != nil {
		return nil, err
	}

	return &EnclaveGasBank{
		BaseEnclave: base,
		balances:    make(map[string]*big.Int),
		pendingTxs:  make(map[string]*GasTransaction),
		feeRate:     big.NewInt(feeRateBps), // basis points
	}, nil
}

// NewEnclaveGasBankWithSDK creates a gas bank handler with full SDK integration.
func NewEnclaveGasBankWithSDK(cfg *GasBankConfig) (*EnclaveGasBank, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "gasbank",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveGasBank{
		BaseEnclave: base,
		balances:    make(map[string]*big.Int),
		pendingTxs:  make(map[string]*GasTransaction),
		feeRate:     big.NewInt(cfg.FeeRateBps),
	}, nil
}

// InitializeWithSDK initializes the gas bank handler with an existing SDK instance.
func (e *EnclaveGasBank) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// CalculateFee calculates the fee for a given amount within the enclave.
func (e *EnclaveGasBank) CalculateFee(amount *big.Int) *big.Int {
	e.RLock()
	defer e.RUnlock()

	// Fee = amount * feeRate / 10000 (basis points)
	fee := new(big.Int).Mul(amount, e.feeRate)
	fee.Div(fee, big.NewInt(10000))
	return fee
}

// ProcessDeduction processes a gas deduction within the enclave.
func (e *EnclaveGasBank) ProcessDeduction(userID string, amount *big.Int) (*GasTransaction, error) {
	e.Lock()
	defer e.Unlock()

	balance, exists := e.balances[userID]
	if !exists {
		balance = big.NewInt(0)
	}

	fee := e.CalculateFee(amount)
	totalDeduction := new(big.Int).Add(amount, fee)

	if balance.Cmp(totalDeduction) < 0 {
		return nil, errors.New("insufficient balance")
	}

	// Deduct from balance
	newBalance := new(big.Int).Sub(balance, totalDeduction)
	e.balances[userID] = newBalance

	// Create transaction record
	txID := generateTxID(userID, amount)
	tx := &GasTransaction{
		TxID:      txID,
		From:      userID,
		Amount:    amount,
		Fee:       fee,
		Timestamp: getCurrentTimestamp(),
		Status:    "completed",
	}

	e.pendingTxs[txID] = tx
	return tx, nil
}

// CreateSettlementProof creates a signed settlement proof.
func (e *EnclaveGasBank) CreateSettlementProof(settlementID string, transactions []*GasTransaction) (*SettlementProof, error) {
	e.Lock()
	defer e.Unlock()

	totalAmount := big.NewInt(0)
	totalFee := big.NewInt(0)

	for _, tx := range transactions {
		totalAmount.Add(totalAmount, tx.Amount)
		totalFee.Add(totalFee, tx.Fee)
	}

	// Create message to sign
	message := sha256.New()
	message.Write([]byte(settlementID))
	message.Write(totalAmount.Bytes())
	message.Write(totalFee.Bytes())
	hash := message.Sum(nil)

	// Sign the settlement
	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash)
	if err != nil {
		return nil, err
	}

	signature := append(r.Bytes(), s.Bytes()...)
	pubKey := e.GetPublicKey()

	return &SettlementProof{
		SettlementID: settlementID,
		TotalAmount:  totalAmount,
		FeeCollected: totalFee,
		Signature:    signature,
		PublicKey:    pubKey,
	}, nil
}

// VerifySettlement verifies a settlement proof signature.
func VerifySettlement(proof *SettlementProof) (bool, error) {
	if len(proof.Signature) < 64 {
		return false, errors.New("invalid signature length")
	}

	return sdk.VerifySignature(proof.PublicKey, append(append([]byte(proof.SettlementID), proof.TotalAmount.Bytes()...), proof.FeeCollected.Bytes()...), proof.Signature)
}

// GetBalance returns the balance for a user (encrypted outside enclave).
func (e *EnclaveGasBank) GetBalance(userID string) *big.Int {
	e.RLock()
	defer e.RUnlock()

	if balance, exists := e.balances[userID]; exists {
		return new(big.Int).Set(balance)
	}
	return big.NewInt(0)
}

// SetBalance sets the balance for a user within the enclave.
func (e *EnclaveGasBank) SetBalance(userID string, amount *big.Int) {
	e.Lock()
	defer e.Unlock()
	e.balances[userID] = new(big.Int).Set(amount)
}

func generateTxID(userID string, amount *big.Int) string {
	h := sha256.New()
	h.Write([]byte(userID))
	h.Write(amount.Bytes())
	h.Write(big.NewInt(getCurrentTimestamp()).Bytes())
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func getCurrentTimestamp() int64 {
	return 0 // In production, use time.Now().Unix()
}
