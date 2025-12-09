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
	"crypto/sha256"
	"io"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"golang.org/x/crypto/hkdf"
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

	// Resource limits for security
	MaxInputSize      = 1 * 1024 * 1024  // 1MB max input size
	MaxOutputSize     = 1 * 1024 * 1024  // 1MB max output size
	MaxSecretRefs     = 10               // Max secrets per execution
	MaxLogEntries     = 100              // Max console.log entries
	MaxLogEntrySize   = 4096             // Max size per log entry
	MaxConcurrentJobs = 5                // Max concurrent jobs per user
)

// jobEntry stores a job with its owner for authorization.
type jobEntry struct {
	UserID   string
	Response *ExecuteResponse
}

// Service implements the Confidential Compute service.
type Service struct {
	*marble.Service
	masterKey  []byte
	signingKey []byte   // Derived key for HMAC signing
	jobs       sync.Map // map[jobID]jobEntry
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
		// Derive signing key from master key using HKDF
		signingKey, err := deriveSigningKey(key)
		if err == nil {
			s.signingKey = signingKey
		}
	}

	s.registerRoutes()
	return s, nil
}

// deriveSigningKey derives a signing key from the master key using HKDF.
func deriveSigningKey(masterKey []byte) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, nil
	}
	// Use HKDF to derive a separate key for signing
	// This ensures encryption and signing use different keys
	salt := []byte("confidential-signing-key")
	info := "tee-result-signing"

	hkdfReader := hkdf.New(sha256.New, masterKey, salt, []byte(info))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// storeJob stores a job result for later retrieval.
func (s *Service) storeJob(userID string, response *ExecuteResponse) {
	s.jobs.Store(response.JobID, jobEntry{
		UserID:   userID,
		Response: response,
	})
}

// getJob retrieves a job by ID, returning nil if not found or not owned by user.
func (s *Service) getJob(userID, jobID string) *ExecuteResponse {
	val, ok := s.jobs.Load(jobID)
	if !ok {
		return nil
	}
	entry := val.(jobEntry)
	if entry.UserID != userID {
		return nil // User doesn't own this job
	}
	return entry.Response
}

// listJobs returns all jobs for a user.
func (s *Service) listJobs(userID string) []*ExecuteResponse {
	var jobs []*ExecuteResponse
	s.jobs.Range(func(key, value interface{}) bool {
		entry := value.(jobEntry)
		if entry.UserID == userID {
			jobs = append(jobs, entry.Response)
		}
		return true
	})
	return jobs
}

// countRunningJobs counts the number of running jobs for a user.
func (s *Service) countRunningJobs(userID string) int {
	count := 0
	s.jobs.Range(func(key, value interface{}) bool {
		entry := value.(jobEntry)
		if entry.UserID == userID && entry.Response.Status == "running" {
			count++
		}
		return true
	})
	return count
}
