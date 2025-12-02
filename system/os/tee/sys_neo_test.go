package tee

import (
	"strings"
	"testing"
)

func TestNewSysNeo(t *testing.T) {
	neo, err := NewSysNeo(nil)
	if err != nil {
		t.Fatalf("NewSysNeo() error = %v", err)
	}
	if neo == nil {
		t.Fatal("expected non-nil SysNeo")
	}
}

func TestSysNeo_GetPublicKey(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	pubKey := neo.GetPublicKey()
	if len(pubKey) != 33 {
		t.Errorf("GetPublicKey() len = %d, want 33 (compressed)", len(pubKey))
	}

	// Should be compressed format (0x02 or 0x03 prefix)
	if pubKey[0] != 0x02 && pubKey[0] != 0x03 {
		t.Errorf("GetPublicKey() prefix = 0x%02x, want 0x02 or 0x03", pubKey[0])
	}
}

func TestSysNeo_GetScriptHash(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	scriptHash := neo.GetScriptHash()
	if len(scriptHash) != 40 { // 20 bytes = 40 hex chars
		t.Errorf("GetScriptHash() len = %d, want 40", len(scriptHash))
	}
}

func TestSysNeo_GetAddress(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	address := neo.GetAddress()
	if !strings.HasPrefix(address, "N") {
		t.Errorf("GetAddress() = %s, should start with 'N'", address)
	}

	// Neo N3 addresses are typically 34 characters
	if len(address) < 30 || len(address) > 40 {
		t.Errorf("GetAddress() len = %d, expected ~34", len(address))
	}
}

func TestSysNeo_SignTransaction(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	tx := &NeoTransaction{
		Version:         0,
		Nonce:           12345,
		SystemFee:       1000000,
		NetworkFee:      100000,
		ValidUntilBlock: 1000000,
		Signers: []NeoSigner{
			{
				Account: neo.GetScriptHash(),
				Scopes:  NeoWitnessScopeCalledByEntry,
			},
		},
		Script: []byte{0x01, 0x02, 0x03}, // Dummy script
	}

	signed, err := neo.SignTransaction(tx)
	if err != nil {
		t.Fatalf("SignTransaction() error = %v", err)
	}

	if signed.Hash == "" {
		t.Error("SignTransaction() Hash should not be empty")
	}

	if signed.RawTransaction == "" {
		t.Error("SignTransaction() RawTransaction should not be empty")
	}

	if len(signed.Witnesses) != 1 {
		t.Errorf("SignTransaction() Witnesses len = %d, want 1", len(signed.Witnesses))
	}

	if signed.Witnesses[0].InvocationScript == "" {
		t.Error("SignTransaction() InvocationScript should not be empty")
	}

	if signed.Witnesses[0].VerificationScript == "" {
		t.Error("SignTransaction() VerificationScript should not be empty")
	}
}

func TestSysNeo_SignTransaction_NilTx(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	_, err := neo.SignTransaction(nil)
	if err == nil {
		t.Error("SignTransaction() should return error for nil transaction")
	}
}

func TestSysNeo_SignInvocation(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	req := &NeoInvocationRequest{
		ScriptHash: "d2a4cff31913016155e38e474a2c06d08be276cf", // Example contract
		Method:     "transfer",
		Args: []NeoContractArg{
			{Type: "Hash160", Value: neo.GetScriptHash()},
			{Type: "Hash160", Value: "0000000000000000000000000000000000000000"},
			{Type: "Integer", Value: 100},
			{Type: "String", Value: "test"},
		},
		Scope: NeoWitnessScopeCalledByEntry,
	}

	signed, err := neo.SignInvocation(req)
	if err != nil {
		t.Fatalf("SignInvocation() error = %v", err)
	}

	if signed.Hash == "" {
		t.Error("SignInvocation() Hash should not be empty")
	}

	if signed.RawTransaction == "" {
		t.Error("SignInvocation() RawTransaction should not be empty")
	}
}

func TestSysNeo_SignInvocation_NilReq(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	_, err := neo.SignInvocation(nil)
	if err == nil {
		t.Error("SignInvocation() should return error for nil request")
	}
}

func TestSysNeo_SignInvocation_SimpleMethod(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	req := &NeoInvocationRequest{
		ScriptHash: "d2a4cff31913016155e38e474a2c06d08be276cf",
		Method:     "symbol",
		Args:       []NeoContractArg{},
	}

	signed, err := neo.SignInvocation(req)
	if err != nil {
		t.Fatalf("SignInvocation() error = %v", err)
	}

	if signed.Size <= 0 {
		t.Error("SignInvocation() Size should be positive")
	}
}

func TestSysNeo_ConsistentAddress(t *testing.T) {
	neo, _ := NewSysNeo(nil)

	addr1 := neo.GetAddress()
	addr2 := neo.GetAddress()

	if addr1 != addr2 {
		t.Error("GetAddress() should return consistent results")
	}
}

func TestSysNeo_UniqueKeys(t *testing.T) {
	neo1, _ := NewSysNeo(nil)
	neo2, _ := NewSysNeo(nil)

	if neo1.GetAddress() == neo2.GetAddress() {
		t.Error("Different instances should have different addresses")
	}
}

func TestWriteVarInt(t *testing.T) {
	tests := []struct {
		val      uint64
		expected int // expected length
	}{
		{0, 1},
		{0xFC, 1},
		{0xFD, 3},
		{0xFFFF, 3},
		{0x10000, 5},
		{0xFFFFFFFF, 5},
		{0x100000000, 9},
	}

	for _, tt := range tests {
		result := writeVarInt(tt.val)
		if len(result) != tt.expected {
			t.Errorf("writeVarInt(%d) len = %d, want %d", tt.val, len(result), tt.expected)
		}
	}
}

func TestPushInt(t *testing.T) {
	tests := []struct {
		val         int64
		minLen      int
	}{
		{-1, 1},   // PUSHM1
		{0, 1},    // PUSH0
		{1, 1},    // PUSH1
		{16, 1},   // PUSH16
		{17, 2},   // PUSHINT8
		{127, 2},  // PUSHINT8
		{128, 2},  // PUSHINT8 (128 fits in 1 byte unsigned)
		{256, 3},  // PUSHINT16
	}

	for _, tt := range tests {
		result := pushInt(tt.val)
		if len(result) < tt.minLen {
			t.Errorf("pushInt(%d) len = %d, want >= %d", tt.val, len(result), tt.minLen)
		}
	}
}

func TestPushString(t *testing.T) {
	tests := []struct {
		str         string
		expectedLen int
	}{
		{"", 2},           // PUSHDATA1 + len(0)
		{"a", 3},          // PUSHDATA1 + len(1) + "a"
		{"hello", 7},      // PUSHDATA1 + len(5) + "hello"
	}

	for _, tt := range tests {
		result := pushString(tt.str)
		if len(result) != tt.expectedLen {
			t.Errorf("pushString(%q) len = %d, want %d", tt.str, len(result), tt.expectedLen)
		}
	}
}

func TestBase58Encode(t *testing.T) {
	// Test with known values
	tests := []struct {
		input    []byte
		expected string
	}{
		{[]byte{0}, "1"},
		{[]byte{0, 0}, "11"},
	}

	for _, tt := range tests {
		result := base58Encode(tt.input)
		if result != tt.expected {
			t.Errorf("base58Encode(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestReverseBytes(t *testing.T) {
	input := []byte{1, 2, 3, 4}
	expected := []byte{4, 3, 2, 1}
	result := reverseBytes(input)

	for i, b := range result {
		if b != expected[i] {
			t.Errorf("reverseBytes() = %v, want %v", result, expected)
			break
		}
	}
}

func TestCompressPublicKey(t *testing.T) {
	neo, _ := NewSysNeo(nil)
	impl := neo.(*sysNeoImpl)

	compressed := compressPublicKey(&impl.privateKey.PublicKey)

	if len(compressed) != 33 {
		t.Errorf("compressPublicKey() len = %d, want 33", len(compressed))
	}

	if compressed[0] != 0x02 && compressed[0] != 0x03 {
		t.Errorf("compressPublicKey() prefix = 0x%02x, want 0x02 or 0x03", compressed[0])
	}
}
