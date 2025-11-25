package runtime

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseEncryptionKeyAcceptsRawLengths(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		valid bool
	}{
		{"empty", "", false},
		{"short", "short", false},
		{"16-bytes", strings.Repeat("a", 16), true},
		{"24-bytes", strings.Repeat("b", 24), true},
		{"32-bytes", strings.Repeat("c", 32), true},
	}
	for _, tc := range cases {
		_, err := parseEncryptionKey(tc.key)
		if tc.valid && err != nil {
			t.Fatalf("%s expected success, got %v", tc.name, err)
		}
		if !tc.valid && err == nil {
			t.Fatalf("%s expected error", tc.name)
		}
	}

	decoded := strings.Repeat("d", 32)
	encoded := base64.StdEncoding.EncodeToString([]byte(decoded))
	if out, err := parseEncryptionKey(encoded); err != nil || string(out) != decoded {
		t.Fatalf("expected base64 key to decode correctly")
	}
}

func TestParseEncryptionKeyDecodesEncodings(t *testing.T) {
	base64Key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	hexKey := "3031323334353637383961626364656630313233343536373839616263646566"
	if _, err := parseEncryptionKey(base64Key); err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	if _, err := parseEncryptionKey(hexKey); err != nil {
		t.Fatalf("hex decode failed: %v", err)
	}
}
