package chain

import (
	"encoding/hex"
	"math/big"
)

// =============================================================================
// Contract Parameter Types
// =============================================================================

// ContractParam represents a contract parameter.
type ContractParam struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// NewStringParam creates a string parameter.
func NewStringParam(value string) ContractParam {
	return ContractParam{Type: "String", Value: value}
}

// NewIntegerParam creates an integer parameter.
func NewIntegerParam(value *big.Int) ContractParam {
	return ContractParam{Type: "Integer", Value: value.String()}
}

// NewBoolParam creates a boolean parameter.
func NewBoolParam(value bool) ContractParam {
	return ContractParam{Type: "Boolean", Value: value}
}

// NewByteArrayParam creates a byte array parameter.
func NewByteArrayParam(value []byte) ContractParam {
	return ContractParam{Type: "ByteArray", Value: hex.EncodeToString(value)}
}

// NewHash160Param creates a Hash160 (address) parameter.
func NewHash160Param(value string) ContractParam {
	return ContractParam{Type: "Hash160", Value: value}
}

// NewHash256Param creates a Hash256 parameter.
func NewHash256Param(value string) ContractParam {
	return ContractParam{Type: "Hash256", Value: value}
}

// NewPublicKeyParam creates a public key parameter.
func NewPublicKeyParam(value string) ContractParam {
	return ContractParam{Type: "PublicKey", Value: value}
}

// NewArrayParam creates an array parameter.
func NewArrayParam(values []ContractParam) ContractParam {
	return ContractParam{Type: "Array", Value: values}
}
