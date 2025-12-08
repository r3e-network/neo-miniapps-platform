// Package confidential provides confidential compute service.
//
// The Confidential Compute service allows users to execute custom JavaScript
// inside the TEE enclave with access to their secrets. This enables:
// - Privacy-preserving computation on sensitive data
// - Secure execution of business logic with verifiable results
// - Integration with external APIs using protected credentials
//
// Architecture:
// - Script execution via goja JavaScript runtime
// - Secure secret injection from user's secret store
// - Signed execution results for verification
// - Gas metering and resource limits
package confidentialmarble

import (
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
)

const (
	ServiceID   = "confidential"
	ServiceName = "Confidential Compute Service"
	Version     = "1.0.0"

	// Default execution timeout
	DefaultTimeout = 30 * time.Second

	// Max script size (100KB)
	MaxScriptSize = 100 * 1024

	// Gas cost per operation (simplified)
	GasPerInstruction = 1
)

// Service implements the Confidential Compute service.
type Service struct {
	*marble.Service
	masterKey []byte
}

// Config holds service configuration.
type Config struct {
	Marble *marble.Marble
	DB     database.RepositoryInterface
}

// New creates a new Confidential Compute service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
	})

	s := &Service{Service: base}

	if key, ok := cfg.Marble.Secret("COMPUTE_MASTER_KEY"); ok {
		s.masterKey = key
	}

	s.registerRoutes()
	return s, nil
}
