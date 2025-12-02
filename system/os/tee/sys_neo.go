// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// sys.neo - Neo N3 Transaction Signing
//
// This file implements Neo N3 specific transaction signing within the TEE.
// The TEE only signs transactions - sending is delegated to other engines.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                         Enclave (Trusted)                                │
//	│  ┌─────────────────────────────────────────────────────────────────────┐ │
//	│  │  sys.neo.signTransaction(tx)                                         │ │
//	│  │    → Returns signed transaction bytes                                │ │
//	│  │                                                                       │ │
//	│  │  sys.neo.signInvocation(scriptHash, method, args)                    │ │
//	│  │    → Returns signed invocation transaction                           │ │
//	│  └─────────────────────────────────────────────────────────────────────┘ │
//	└─────────────────────────────────────────────────────────────────────────┘
//	                                 │
//	                                 │ Signed TX (bytes)
//	                                 ▼
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                      Go Service Engine (Untrusted)                       │
//	│  ContractsEngine.SendRawTransaction(signedTx)                           │
//	│    → Broadcasts to Neo N3 network                                        │
//	└─────────────────────────────────────────────────────────────────────────┘
package tee

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// SysNeo provides Neo N3 specific operations within the TEE.
type SysNeo interface {
	// SignTransaction signs a Neo N3 transaction.
	// Returns the signed transaction bytes ready for broadcast.
	SignTransaction(tx *NeoTransaction) (*SignedNeoTransaction, error)

	// SignInvocation creates and signs a contract invocation transaction.
	SignInvocation(req *NeoInvocationRequest) (*SignedNeoTransaction, error)

	// GetPublicKey returns the public key used for signing.
	GetPublicKey() []byte

	// GetScriptHash returns the script hash (address) derived from the public key.
	GetScriptHash() string

	// GetAddress returns the Neo N3 address.
	GetAddress() string
}

// NeoTransaction represents a Neo N3 transaction to be signed.
type NeoTransaction struct {
	// Version of the transaction (usually 0)
	Version uint8 `json:"version"`

	// Nonce for replay protection
	Nonce uint32 `json:"nonce"`

	// SystemFee in GAS (fixed8 format, multiply by 10^8)
	SystemFee int64 `json:"system_fee"`

	// NetworkFee in GAS (fixed8 format)
	NetworkFee int64 `json:"network_fee"`

	// ValidUntilBlock - transaction expires after this block
	ValidUntilBlock uint32 `json:"valid_until_block"`

	// Signers define who can use this transaction
	Signers []NeoSigner `json:"signers"`

	// Attributes for additional transaction data
	Attributes []NeoAttribute `json:"attributes,omitempty"`

	// Script is the VM script to execute
	Script []byte `json:"script"`
}

// NeoSigner represents a transaction signer.
type NeoSigner struct {
	// Account is the script hash of the signer
	Account string `json:"account"`

	// Scopes defines the signature scope
	Scopes NeoWitnessScope `json:"scopes"`

	// AllowedContracts for CustomContracts scope
	AllowedContracts []string `json:"allowed_contracts,omitempty"`

	// AllowedGroups for CustomGroups scope
	AllowedGroups []string `json:"allowed_groups,omitempty"`
}

// NeoWitnessScope defines the scope of a witness signature.
type NeoWitnessScope uint8

const (
	// NeoWitnessScopeNone - No contract can use this signature
	NeoWitnessScopeNone NeoWitnessScope = 0

	// NeoWitnessScopeCalledByEntry - Only the entry script can use this signature
	NeoWitnessScopeCalledByEntry NeoWitnessScope = 1

	// NeoWitnessScopeCustomContracts - Only specified contracts can use this signature
	NeoWitnessScopeCustomContracts NeoWitnessScope = 16

	// NeoWitnessScopeCustomGroups - Only specified groups can use this signature
	NeoWitnessScopeCustomGroups NeoWitnessScope = 32

	// NeoWitnessScopeGlobal - Any contract can use this signature (dangerous)
	NeoWitnessScopeGlobal NeoWitnessScope = 128
)

// NeoAttribute represents a transaction attribute.
type NeoAttribute struct {
	Type  uint8  `json:"type"`
	Value []byte `json:"value,omitempty"`
}

// NeoInvocationRequest represents a contract invocation request.
type NeoInvocationRequest struct {
	// ScriptHash of the contract to invoke
	ScriptHash string `json:"script_hash"`

	// Method to call
	Method string `json:"method"`

	// Args for the method call
	Args []NeoContractArg `json:"args,omitempty"`

	// SystemFee in GAS (if 0, will use default)
	SystemFee int64 `json:"system_fee,omitempty"`

	// NetworkFee in GAS (if 0, will use default)
	NetworkFee int64 `json:"network_fee,omitempty"`

	// ValidUntilBlock (if 0, will use current + 100)
	ValidUntilBlock uint32 `json:"valid_until_block,omitempty"`

	// Scope for the signature
	Scope NeoWitnessScope `json:"scope,omitempty"`
}

// NeoContractArg represents a contract method argument.
type NeoContractArg struct {
	Type  string `json:"type"`  // "Integer", "String", "ByteArray", "Hash160", "Hash256", "PublicKey", "Boolean", "Array"
	Value any    `json:"value"`
}

// SignedNeoTransaction represents a signed Neo N3 transaction.
type SignedNeoTransaction struct {
	// Hash is the transaction hash
	Hash string `json:"hash"`

	// Size in bytes
	Size int `json:"size"`

	// RawTransaction is the serialized signed transaction (hex encoded)
	RawTransaction string `json:"raw_transaction"`

	// Witnesses contains the signatures
	Witnesses []NeoWitness `json:"witnesses"`
}

// NeoWitness represents a transaction witness (signature).
type NeoWitness struct {
	// InvocationScript contains the signature
	InvocationScript string `json:"invocation_script"`

	// VerificationScript contains the public key
	VerificationScript string `json:"verification_script"`
}

// =============================================================================
// Implementation
// =============================================================================

// sysNeoImpl implements SysNeo.
type sysNeoImpl struct {
	mu         sync.RWMutex
	privateKey *ecdsa.PrivateKey
	publicKey  []byte
	scriptHash []byte
	address    string
}

// NewSysNeo creates a new SysNeo implementation.
// If privateKey is nil, a new key will be generated.
func NewSysNeo(privateKey *ecdsa.PrivateKey) (SysNeo, error) {
	impl := &sysNeoImpl{}

	if privateKey == nil {
		// Generate new key
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}
		impl.privateKey = key
	} else {
		impl.privateKey = privateKey
	}

	// Compute public key (compressed format for Neo)
	impl.publicKey = compressPublicKey(&impl.privateKey.PublicKey)

	// Compute script hash and address
	impl.scriptHash = publicKeyToScriptHash(impl.publicKey)
	impl.address = scriptHashToAddress(impl.scriptHash)

	return impl, nil
}

// SignTransaction signs a Neo N3 transaction.
func (n *sysNeoImpl) SignTransaction(tx *NeoTransaction) (*SignedNeoTransaction, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	// Serialize the transaction (without witnesses)
	txBytes, err := n.serializeTransaction(tx)
	if err != nil {
		return nil, fmt.Errorf("serialize transaction: %w", err)
	}

	// Compute transaction hash
	txHash := sha256.Sum256(txBytes)
	txHash = sha256.Sum256(txHash[:])

	// Sign the hash
	signature, err := n.sign(txHash[:])
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// Create witness
	witness := n.createWitness(signature)

	// Serialize with witness
	signedTxBytes := append(txBytes, n.serializeWitnesses([]NeoWitness{witness})...)

	return &SignedNeoTransaction{
		Hash:           hex.EncodeToString(reverseBytes(txHash[:])),
		Size:           len(signedTxBytes),
		RawTransaction: hex.EncodeToString(signedTxBytes),
		Witnesses:      []NeoWitness{witness},
	}, nil
}

// SignInvocation creates and signs a contract invocation transaction.
func (n *sysNeoImpl) SignInvocation(req *NeoInvocationRequest) (*SignedNeoTransaction, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	// Build the invocation script
	script, err := n.buildInvocationScript(req)
	if err != nil {
		return nil, fmt.Errorf("build script: %w", err)
	}

	// Set defaults
	systemFee := req.SystemFee
	if systemFee == 0 {
		systemFee = 1000000 // 0.01 GAS default
	}

	networkFee := req.NetworkFee
	if networkFee == 0 {
		networkFee = 100000 // 0.001 GAS default
	}

	validUntilBlock := req.ValidUntilBlock
	if validUntilBlock == 0 {
		validUntilBlock = uint32(time.Now().Unix()/15) + 100 // ~25 minutes
	}

	scope := req.Scope
	if scope == 0 {
		scope = NeoWitnessScopeCalledByEntry
	}

	// Create transaction
	tx := &NeoTransaction{
		Version:         0,
		Nonce:           uint32(time.Now().UnixNano()),
		SystemFee:       systemFee,
		NetworkFee:      networkFee,
		ValidUntilBlock: validUntilBlock,
		Signers: []NeoSigner{
			{
				Account: hex.EncodeToString(n.scriptHash),
				Scopes:  scope,
			},
		},
		Script: script,
	}

	return n.SignTransaction(tx)
}

// GetPublicKey returns the public key.
func (n *sysNeoImpl) GetPublicKey() []byte {
	n.mu.RLock()
	defer n.mu.RUnlock()
	result := make([]byte, len(n.publicKey))
	copy(result, n.publicKey)
	return result
}

// GetScriptHash returns the script hash.
func (n *sysNeoImpl) GetScriptHash() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return hex.EncodeToString(n.scriptHash)
}

// GetAddress returns the Neo N3 address.
func (n *sysNeoImpl) GetAddress() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.address
}

// =============================================================================
// Internal helpers
// =============================================================================

// sign signs data using ECDSA.
func (n *sysNeoImpl) sign(hash []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, n.privateKey, hash)
	if err != nil {
		return nil, err
	}

	// Neo uses 64-byte signatures (r || s)
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return sig, nil
}

// createWitness creates a witness from a signature.
func (n *sysNeoImpl) createWitness(signature []byte) NeoWitness {
	// Invocation script: PUSHDATA1 <len> <signature>
	invocationScript := []byte{0x0C, byte(len(signature))}
	invocationScript = append(invocationScript, signature...)

	// Verification script: PUSHDATA1 <len> <pubkey> SYSCALL System.Crypto.CheckSig
	verificationScript := []byte{0x0C, byte(len(n.publicKey))}
	verificationScript = append(verificationScript, n.publicKey...)
	verificationScript = append(verificationScript, 0x41) // SYSCALL
	// System.Crypto.CheckSig hash
	verificationScript = append(verificationScript, 0x56, 0xe7, 0xb3, 0x27)

	return NeoWitness{
		InvocationScript:   hex.EncodeToString(invocationScript),
		VerificationScript: hex.EncodeToString(verificationScript),
	}
}

// serializeTransaction serializes a transaction without witnesses.
func (n *sysNeoImpl) serializeTransaction(tx *NeoTransaction) ([]byte, error) {
	var buf []byte

	// Version
	buf = append(buf, tx.Version)

	// Nonce
	nonceBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(nonceBuf, tx.Nonce)
	buf = append(buf, nonceBuf...)

	// SystemFee
	sysFee := make([]byte, 8)
	binary.LittleEndian.PutUint64(sysFee, uint64(tx.SystemFee))
	buf = append(buf, sysFee...)

	// NetworkFee
	netFee := make([]byte, 8)
	binary.LittleEndian.PutUint64(netFee, uint64(tx.NetworkFee))
	buf = append(buf, netFee...)

	// ValidUntilBlock
	validBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(validBuf, tx.ValidUntilBlock)
	buf = append(buf, validBuf...)

	// Signers
	buf = append(buf, writeVarInt(uint64(len(tx.Signers)))...)
	for _, signer := range tx.Signers {
		signerBytes, err := n.serializeSigner(signer)
		if err != nil {
			return nil, err
		}
		buf = append(buf, signerBytes...)
	}

	// Attributes
	buf = append(buf, writeVarInt(uint64(len(tx.Attributes)))...)
	for _, attr := range tx.Attributes {
		buf = append(buf, attr.Type)
		if len(attr.Value) > 0 {
			buf = append(buf, writeVarInt(uint64(len(attr.Value)))...)
			buf = append(buf, attr.Value...)
		}
	}

	// Script
	buf = append(buf, writeVarInt(uint64(len(tx.Script)))...)
	buf = append(buf, tx.Script...)

	return buf, nil
}

// serializeSigner serializes a signer.
func (n *sysNeoImpl) serializeSigner(signer NeoSigner) ([]byte, error) {
	var buf []byte

	// Account (20 bytes script hash)
	account, err := hex.DecodeString(signer.Account)
	if err != nil {
		return nil, fmt.Errorf("decode account: %w", err)
	}
	if len(account) != 20 {
		return nil, fmt.Errorf("invalid account length: %d", len(account))
	}
	buf = append(buf, account...)

	// Scopes
	buf = append(buf, byte(signer.Scopes))

	// AllowedContracts (if CustomContracts scope)
	if signer.Scopes&NeoWitnessScopeCustomContracts != 0 {
		buf = append(buf, writeVarInt(uint64(len(signer.AllowedContracts)))...)
		for _, contract := range signer.AllowedContracts {
			contractBytes, _ := hex.DecodeString(contract)
			buf = append(buf, contractBytes...)
		}
	}

	// AllowedGroups (if CustomGroups scope)
	if signer.Scopes&NeoWitnessScopeCustomGroups != 0 {
		buf = append(buf, writeVarInt(uint64(len(signer.AllowedGroups)))...)
		for _, group := range signer.AllowedGroups {
			groupBytes, _ := hex.DecodeString(group)
			buf = append(buf, groupBytes...)
		}
	}

	return buf, nil
}

// serializeWitnesses serializes witnesses.
func (n *sysNeoImpl) serializeWitnesses(witnesses []NeoWitness) []byte {
	var buf []byte
	buf = append(buf, writeVarInt(uint64(len(witnesses)))...)
	for _, w := range witnesses {
		invScript, _ := hex.DecodeString(w.InvocationScript)
		verScript, _ := hex.DecodeString(w.VerificationScript)

		buf = append(buf, writeVarInt(uint64(len(invScript)))...)
		buf = append(buf, invScript...)
		buf = append(buf, writeVarInt(uint64(len(verScript)))...)
		buf = append(buf, verScript...)
	}
	return buf
}

// buildInvocationScript builds a VM script for contract invocation.
func (n *sysNeoImpl) buildInvocationScript(req *NeoInvocationRequest) ([]byte, error) {
	var script []byte

	// Push arguments in reverse order
	for i := len(req.Args) - 1; i >= 0; i-- {
		arg := req.Args[i]
		argBytes, err := n.serializeContractArg(arg)
		if err != nil {
			return nil, fmt.Errorf("serialize arg %d: %w", i, err)
		}
		script = append(script, argBytes...)
	}

	// Push argument count
	script = append(script, pushInt(int64(len(req.Args)))...)

	// PACK
	script = append(script, 0xC1)

	// Push method name
	script = append(script, pushString(req.Method)...)

	// Push contract hash (reversed for little-endian)
	contractHash, err := hex.DecodeString(req.ScriptHash)
	if err != nil {
		return nil, fmt.Errorf("decode script hash: %w", err)
	}
	script = append(script, pushBytes(reverseBytes(contractHash))...)

	// SYSCALL System.Contract.Call
	script = append(script, 0x41)
	script = append(script, 0x62, 0x7d, 0x5b, 0x52) // System.Contract.Call hash

	return script, nil
}

// serializeContractArg serializes a contract argument.
func (n *sysNeoImpl) serializeContractArg(arg NeoContractArg) ([]byte, error) {
	switch arg.Type {
	case "Integer":
		val, ok := arg.Value.(float64)
		if !ok {
			if intVal, ok := arg.Value.(int); ok {
				val = float64(intVal)
			} else {
				return nil, fmt.Errorf("invalid integer value")
			}
		}
		return pushInt(int64(val)), nil

	case "String":
		str, ok := arg.Value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid string value")
		}
		return pushString(str), nil

	case "ByteArray":
		str, ok := arg.Value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid byte array value")
		}
		bytes, err := hex.DecodeString(str)
		if err != nil {
			return nil, fmt.Errorf("decode byte array: %w", err)
		}
		return pushBytes(bytes), nil

	case "Hash160":
		str, ok := arg.Value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid hash160 value")
		}
		bytes, err := hex.DecodeString(str)
		if err != nil {
			return nil, fmt.Errorf("decode hash160: %w", err)
		}
		if len(bytes) != 20 {
			return nil, fmt.Errorf("invalid hash160 length")
		}
		return pushBytes(reverseBytes(bytes)), nil

	case "Boolean":
		val, ok := arg.Value.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid boolean value")
		}
		if val {
			return []byte{0x11}, nil // PUSHT
		}
		return []byte{0x10}, nil // PUSHF

	default:
		return nil, fmt.Errorf("unsupported argument type: %s", arg.Type)
	}
}

// =============================================================================
// Neo-specific crypto helpers
// =============================================================================

// compressPublicKey compresses an ECDSA public key.
func compressPublicKey(pub *ecdsa.PublicKey) []byte {
	x := pub.X.Bytes()
	// Pad to 32 bytes
	for len(x) < 32 {
		x = append([]byte{0}, x...)
	}

	// Prefix: 0x02 if Y is even, 0x03 if Y is odd
	prefix := byte(0x02)
	if pub.Y.Bit(0) == 1 {
		prefix = 0x03
	}

	return append([]byte{prefix}, x...)
}

// publicKeyToScriptHash converts a public key to a Neo script hash.
func publicKeyToScriptHash(pubKey []byte) []byte {
	// Verification script: PUSHDATA1 <len> <pubkey> SYSCALL System.Crypto.CheckSig
	script := []byte{0x0C, byte(len(pubKey))}
	script = append(script, pubKey...)
	script = append(script, 0x41, 0x56, 0xe7, 0xb3, 0x27)

	// SHA256 then RIPEMD160
	hash := sha256.Sum256(script)
	return ripemd160Hash(hash[:])
}

// scriptHashToAddress converts a script hash to a Neo N3 address.
func scriptHashToAddress(scriptHash []byte) string {
	// Neo N3 address version
	version := byte(0x35) // 'N' prefix

	// Add version prefix
	data := append([]byte{version}, scriptHash...)

	// Double SHA256 for checksum
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])
	checksum := hash2[:4]

	// Append checksum
	data = append(data, checksum...)

	// Base58 encode
	return base58Encode(data)
}

// ripemd160Hash computes RIPEMD-160 hash.
// This is a complete implementation of the RIPEMD-160 algorithm as specified
// in the original paper by Hans Dobbertin, Antoon Bosselaers, and Bart Preneel.
func ripemd160Hash(data []byte) []byte {
	// Initial hash values
	h0 := uint32(0x67452301)
	h1 := uint32(0xEFCDAB89)
	h2 := uint32(0x98BADCFE)
	h3 := uint32(0x10325476)
	h4 := uint32(0xC3D2E1F0)

	// Pre-processing: adding padding bits
	msgLen := uint64(len(data)) * 8
	data = append(data, 0x80)
	for len(data)%64 != 56 {
		data = append(data, 0x00)
	}
	// Append length in bits as 64-bit little-endian
	for i := 0; i < 8; i++ {
		data = append(data, byte(msgLen>>(8*i)))
	}

	// Process each 512-bit block
	for i := 0; i < len(data); i += 64 {
		// Break block into sixteen 32-bit little-endian words
		var x [16]uint32
		for j := 0; j < 16; j++ {
			x[j] = uint32(data[i+j*4]) | uint32(data[i+j*4+1])<<8 |
				uint32(data[i+j*4+2])<<16 | uint32(data[i+j*4+3])<<24
		}

		// Initialize working variables
		al, bl, cl, dl, el := h0, h1, h2, h3, h4
		ar, br, cr, dr, er := h0, h1, h2, h3, h4

		// Round constants
		kl := [5]uint32{0x00000000, 0x5A827999, 0x6ED9EBA1, 0x8F1BBCDC, 0xA953FD4E}
		kr := [5]uint32{0x50A28BE6, 0x5C4DD124, 0x6D703EF3, 0x7A6D76E9, 0x00000000}

		// Message schedule (selection of message word)
		rl := [80]int{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
			7, 4, 13, 1, 10, 6, 15, 3, 12, 0, 9, 5, 2, 14, 11, 8,
			3, 10, 14, 4, 9, 15, 8, 1, 2, 7, 0, 6, 13, 11, 5, 12,
			1, 9, 11, 10, 0, 8, 12, 4, 13, 3, 7, 15, 14, 5, 6, 2,
			4, 0, 5, 9, 7, 12, 2, 10, 14, 1, 3, 8, 11, 6, 15, 13,
		}
		rr := [80]int{
			5, 14, 7, 0, 9, 2, 11, 4, 13, 6, 15, 8, 1, 10, 3, 12,
			6, 11, 3, 7, 0, 13, 5, 10, 14, 15, 8, 12, 4, 9, 1, 2,
			15, 5, 1, 3, 7, 14, 6, 9, 11, 8, 12, 2, 10, 0, 4, 13,
			8, 6, 4, 1, 3, 11, 15, 0, 5, 12, 2, 13, 9, 7, 10, 14,
			12, 15, 10, 4, 1, 5, 8, 7, 6, 2, 13, 14, 0, 3, 9, 11,
		}

		// Rotation amounts
		sl := [80]uint{
			11, 14, 15, 12, 5, 8, 7, 9, 11, 13, 14, 15, 6, 7, 9, 8,
			7, 6, 8, 13, 11, 9, 7, 15, 7, 12, 15, 9, 11, 7, 13, 12,
			11, 13, 6, 7, 14, 9, 13, 15, 14, 8, 13, 6, 5, 12, 7, 5,
			11, 12, 14, 15, 14, 15, 9, 8, 9, 14, 5, 6, 8, 6, 5, 12,
			9, 15, 5, 11, 6, 8, 13, 12, 5, 12, 13, 14, 11, 8, 5, 6,
		}
		sr := [80]uint{
			8, 9, 9, 11, 13, 15, 15, 5, 7, 7, 8, 11, 14, 14, 12, 6,
			9, 13, 15, 7, 12, 8, 9, 11, 7, 7, 12, 7, 6, 15, 13, 11,
			9, 7, 15, 11, 8, 6, 6, 14, 12, 13, 5, 14, 13, 13, 7, 5,
			15, 5, 8, 11, 14, 14, 6, 14, 6, 9, 12, 9, 12, 5, 15, 8,
			8, 5, 12, 9, 12, 5, 14, 6, 8, 13, 6, 5, 15, 13, 11, 11,
		}

		// 80 rounds
		for j := 0; j < 80; j++ {
			var fl, fr uint32
			round := j / 16

			// Left line
			switch round {
			case 0:
				fl = bl ^ cl ^ dl
			case 1:
				fl = (bl & cl) | (^bl & dl)
			case 2:
				fl = (bl | ^cl) ^ dl
			case 3:
				fl = (bl & dl) | (cl & ^dl)
			case 4:
				fl = bl ^ (cl | ^dl)
			}

			// Right line
			switch round {
			case 0:
				fr = br ^ (cr | ^dr)
			case 1:
				fr = (br & dr) | (cr & ^dr)
			case 2:
				fr = (br | ^cr) ^ dr
			case 3:
				fr = (br & cr) | (^br & dr)
			case 4:
				fr = br ^ cr ^ dr
			}

			tl := rotateLeft(al+fl+x[rl[j]]+kl[round], sl[j]) + el
			al = el
			el = dl
			dl = rotateLeft(cl, 10)
			cl = bl
			bl = tl

			tr := rotateLeft(ar+fr+x[rr[j]]+kr[round], sr[j]) + er
			ar = er
			er = dr
			dr = rotateLeft(cr, 10)
			cr = br
			br = tr
		}

		// Final addition
		t := h1 + cl + dr
		h1 = h2 + dl + er
		h2 = h3 + el + ar
		h3 = h4 + al + br
		h4 = h0 + bl + cr
		h0 = t
	}

	// Produce the final hash value (little-endian)
	result := make([]byte, 20)
	for i, v := range []uint32{h0, h1, h2, h3, h4} {
		result[i*4] = byte(v)
		result[i*4+1] = byte(v >> 8)
		result[i*4+2] = byte(v >> 16)
		result[i*4+3] = byte(v >> 24)
	}
	return result
}

// rotateLeft performs a left rotation on a 32-bit value.
func rotateLeft(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}

// =============================================================================
// Encoding helpers
// =============================================================================

// writeVarInt writes a variable-length integer.
func writeVarInt(val uint64) []byte {
	if val < 0xFD {
		return []byte{byte(val)}
	} else if val <= 0xFFFF {
		buf := make([]byte, 3)
		buf[0] = 0xFD
		binary.LittleEndian.PutUint16(buf[1:], uint16(val))
		return buf
	} else if val <= 0xFFFFFFFF {
		buf := make([]byte, 5)
		buf[0] = 0xFE
		binary.LittleEndian.PutUint32(buf[1:], uint32(val))
		return buf
	}
	buf := make([]byte, 9)
	buf[0] = 0xFF
	binary.LittleEndian.PutUint64(buf[1:], val)
	return buf
}

// pushInt pushes an integer onto the VM stack.
func pushInt(val int64) []byte {
	if val == -1 {
		return []byte{0x0F} // PUSHM1
	}
	if val >= 0 && val <= 16 {
		return []byte{byte(0x10 + val)} // PUSH0-PUSH16
	}

	// Use PUSHINT
	bytes := big.NewInt(val).Bytes()
	if val < 0 {
		// Handle negative numbers
		bytes = twosComplement(val)
	}

	// Reverse for little-endian
	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}

	if len(bytes) <= 1 {
		return append([]byte{0x00}, bytes...) // PUSHINT8
	} else if len(bytes) <= 2 {
		return append([]byte{0x01}, padRight(bytes, 2)...) // PUSHINT16
	} else if len(bytes) <= 4 {
		return append([]byte{0x02}, padRight(bytes, 4)...) // PUSHINT32
	} else if len(bytes) <= 8 {
		return append([]byte{0x03}, padRight(bytes, 8)...) // PUSHINT64
	}
	return append([]byte{0x04}, padRight(bytes, 16)...) // PUSHINT128
}

// pushString pushes a string onto the VM stack.
func pushString(s string) []byte {
	return pushBytes([]byte(s))
}

// pushBytes pushes bytes onto the VM stack.
func pushBytes(data []byte) []byte {
	l := len(data)
	if l <= 255 {
		return append([]byte{0x0C, byte(l)}, data...) // PUSHDATA1
	} else if l <= 65535 {
		buf := []byte{0x0D}
		lenBuf := make([]byte, 2)
		binary.LittleEndian.PutUint16(lenBuf, uint16(l))
		buf = append(buf, lenBuf...)
		return append(buf, data...)
	}
	buf := []byte{0x0E}
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(l))
	buf = append(buf, lenBuf...)
	return append(buf, data...)
}

// reverseBytes reverses a byte slice.
func reverseBytes(data []byte) []byte {
	result := make([]byte, len(data))
	for i, b := range data {
		result[len(data)-1-i] = b
	}
	return result
}

// padRight pads bytes to the right with zeros.
func padRight(data []byte, size int) []byte {
	if len(data) >= size {
		return data[:size]
	}
	result := make([]byte, size)
	copy(result, data)
	return result
}

// twosComplement returns two's complement representation.
func twosComplement(val int64) []byte {
	if val >= 0 {
		return big.NewInt(val).Bytes()
	}
	// For negative numbers
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(val))
	// Trim trailing 0xFF bytes but keep at least one
	for len(bytes) > 1 && bytes[len(bytes)-1] == 0xFF && bytes[len(bytes)-2]&0x80 != 0 {
		bytes = bytes[:len(bytes)-1]
	}
	return bytes
}

// base58Encode encodes data to Base58.
func base58Encode(data []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Count leading zeros
	zeros := 0
	for _, b := range data {
		if b == 0 {
			zeros++
		} else {
			break
		}
	}

	// Convert to big integer
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	mod := new(big.Int)

	var result []byte
	for num.Sign() > 0 {
		num.DivMod(num, base, mod)
		result = append([]byte{alphabet[mod.Int64()]}, result...)
	}

	// Add leading '1's for zeros
	for i := 0; i < zeros; i++ {
		result = append([]byte{'1'}, result...)
	}

	return string(result)
}
