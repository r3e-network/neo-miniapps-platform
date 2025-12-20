package neovrf

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"strings"

	"github.com/R3E-Network/service_layer/infrastructure/marble"
)

func computeAttestationHash(m *marble.Marble) []byte {
	if m != nil {
		if report := m.Report(); report != nil {
			if b, err := json.Marshal(report); err == nil && len(b) > 0 {
				sum := sha256.Sum256(b)
				return sum[:]
			}
		}

		if certPEM := strings.TrimSpace(os.Getenv("MARBLE_CERT")); certPEM != "" {
			sum := sha256.Sum256([]byte(certPEM))
			return sum[:]
		}

		if mt := strings.TrimSpace(m.MarbleType()); mt != "" || strings.TrimSpace(m.UUID()) != "" {
			sum := sha256.Sum256([]byte(mt + "|" + m.UUID()))
			return sum[:]
		}
	}

	sum := sha256.Sum256([]byte("neovrf:attestation:unknown"))
	return sum[:]
}
