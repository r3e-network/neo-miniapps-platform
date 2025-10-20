package runtime

import "testing"

func TestParseEncryptionKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		ok      bool
	}{
		{"raw-16", "1234567890abcdef", 16, true},
		{"raw-32", "0123456789abcdef0123456789abcdef", 32, true},
		{"base64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=", 32, true},
		{"hex", "3031323334353637383961626364656630313233343536373839616263646566", 32, true},
		{"invalid-length", "short", 0, false},
		{"invalid-format", "zzzz", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := parseEncryptionKey(tt.input)
			if tt.ok {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if len(key) != tt.wantLen {
					t.Fatalf("unexpected length: got %d want %d", len(key), tt.wantLen)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
			}
		})
	}
}
