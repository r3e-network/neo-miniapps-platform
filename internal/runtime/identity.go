// Package runtime provides environment/runtime detection helpers shared across the service layer.
package runtime

import (
	"os"
	"strings"
)

// StrictIdentityMode returns true when the service should fail closed on identity/security
// boundaries (e.g. only trust identity headers protected by verified mTLS).
//
// We treat SGX hardware mode (OE_SIMULATION=0) and MarbleRun-injected TLS credentials
// as "strict" too, so a mis-set MARBLE_ENV cannot silently weaken trust boundaries.
func StrictIdentityMode() bool {
	env := Env()
	oeSimulation := strings.TrimSpace(os.Getenv("OE_SIMULATION"))
	hasMarbleTLS := strings.TrimSpace(os.Getenv("MARBLE_CERT")) != "" &&
		strings.TrimSpace(os.Getenv("MARBLE_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("MARBLE_ROOT_CA")) != ""
	return env == Production || oeSimulation == "0" || hasMarbleTLS
}
