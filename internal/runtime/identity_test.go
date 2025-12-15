package runtime

import "testing"

func TestStrictIdentityMode(t *testing.T) {
	t.Run("production env", func(t *testing.T) {
		t.Setenv("MARBLE_ENV", "production")
		t.Setenv("OE_SIMULATION", "1")
		if !StrictIdentityMode() {
			t.Fatalf("StrictIdentityMode() = false, want true")
		}
	})

	t.Run("hardware mode", func(t *testing.T) {
		t.Setenv("MARBLE_ENV", "development")
		t.Setenv("OE_SIMULATION", "0")
		if !StrictIdentityMode() {
			t.Fatalf("StrictIdentityMode() = false, want true")
		}
	})

	t.Run("marblerun tls injected", func(t *testing.T) {
		t.Setenv("MARBLE_ENV", "development")
		t.Setenv("OE_SIMULATION", "1")
		t.Setenv("MARBLE_CERT", "cert")
		t.Setenv("MARBLE_KEY", "key")
		t.Setenv("MARBLE_ROOT_CA", "ca")
		if !StrictIdentityMode() {
			t.Fatalf("StrictIdentityMode() = false, want true")
		}
	})

	t.Run("dev simulation", func(t *testing.T) {
		t.Setenv("MARBLE_ENV", "development")
		t.Setenv("OE_SIMULATION", "1")
		if StrictIdentityMode() {
			t.Fatalf("StrictIdentityMode() = true, want false")
		}
	})
}
