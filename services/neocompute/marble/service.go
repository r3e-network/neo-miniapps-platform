// Package neocompute provides neocompute service.
//
// The NeoCompute service allows users to execute custom JavaScript
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
package neocomputemarble

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/runtime"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neostoreclient "github.com/R3E-Network/service_layer/services/neostore/client"
)

const (
	ServiceID   = "neocompute"
	ServiceName = "NeoCompute Service"
	Version     = "1.0.0"

	// Default execution timeout
	DefaultTimeout = 30 * time.Second

	// Max script size (100KB)
	MaxScriptSize = 100 * 1024

	// Gas accounting is approximate and based on the submitted script size.
	// This value is intended for billing/rate limiting and is not tied to VM opcodes.
	GasPerScriptByte = 10

	// Resource limits for security
	MaxInputSize      = 1 * 1024 * 1024 // 1MB max input size
	MaxOutputSize     = 1 * 1024 * 1024 // 1MB max output size
	MaxSecretRefs     = 10              // Max secrets per execution
	MaxLogEntries     = 100             // Max console.log entries
	MaxLogEntrySize   = 4096            // Max size per log entry
	MaxConcurrentJobs = 5               // Max concurrent jobs per user

	// Result retention defaults
	DefaultResultTTL       = 24 * time.Hour
	defaultCleanupInterval = time.Minute

	neocomputeResultTTLEnv = "NEOCOMPUTE_RESULT_TTL"
)

// jobEntry stores a job with its owner for authorization.
type jobEntry struct {
	UserID   string
	Response *ExecuteResponse
	storedAt time.Time
}

// Service implements the NeoCompute service.
type Service struct {
	*commonservice.BaseService
	masterKey       []byte
	signingKey      []byte // Derived key for HMAC signing
	secretClient    *neostoreclient.Client
	jobs            sync.Map // map[jobID]jobEntry
	resultTTL       time.Duration
	cleanupInterval time.Duration
}

// Config holds service configuration.
type Config struct {
	Marble *marble.Marble
	DB     database.RepositoryInterface
	// Optional overrides, primarily used for testing.
	ResultTTL       time.Duration
	CleanupInterval time.Duration
	// NeoStore configuration for secret injection.
	SecretsBaseURL    string
	SecretsHTTPClient *http.Client
}

// New creates a new NeoCompute service.
func New(cfg Config) (*Service, error) {
	if cfg.Marble == nil {
		return nil, fmt.Errorf("neocompute: marble is required")
	}

	strict := runtime.StrictIdentityMode() || cfg.Marble.IsEnclave()

	requiredSecrets := []string(nil)
	if strict {
		requiredSecrets = []string{"COMPUTE_MASTER_KEY"}
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:              ServiceID,
		Name:            ServiceName,
		Version:         Version,
		Marble:          cfg.Marble,
		DB:              cfg.DB,
		RequiredSecrets: requiredSecrets,
	})

	resultTTL := DefaultResultTTL
	if cfg.ResultTTL > 0 {
		resultTTL = cfg.ResultTTL
	} else if envTTL := loadResultTTLOverride(); envTTL > 0 {
		resultTTL = envTTL
	}

	cleanupInterval := defaultCleanupInterval
	if cfg.CleanupInterval > 0 {
		cleanupInterval = cfg.CleanupInterval
	}

	s := &Service{
		BaseService:     base,
		resultTTL:       resultTTL,
		cleanupInterval: cleanupInterval,
	}

	key, ok := cfg.Marble.Secret("COMPUTE_MASTER_KEY")
	switch {
	case ok && len(key) >= 32:
		s.masterKey = key
		signingKey, err := deriveSigningKey(key)
		if err == nil {
			s.signingKey = signingKey
		}
	case strict:
		return nil, fmt.Errorf("neocompute: COMPUTE_MASTER_KEY is required and must be at least 32 bytes")
	default:
		// Development/testing fallback: generate an ephemeral master key.
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("neocompute: generate fallback key: %w", err)
		}
		s.masterKey = buf
		if signingKey, err := deriveSigningKey(buf); err == nil {
			s.signingKey = signingKey
		}
	}

	secretsBaseURL := strings.TrimSpace(cfg.SecretsBaseURL)
	if secretsBaseURL == "" {
		secretsBaseURL = strings.TrimSpace(os.Getenv("SECRETS_BASE_URL"))
	}
	if secretsBaseURL != "" {
		httpClient := cfg.SecretsHTTPClient
		if httpClient == nil && cfg.Marble != nil {
			httpClient = cfg.Marble.HTTPClient()
		}
		client, err := neostoreclient.New(neostoreclient.Config{
			BaseURL:    secretsBaseURL,
			HTTPClient: httpClient,
			ServiceID:  ServiceID,
		})
		if err != nil {
			return nil, fmt.Errorf("neocompute: configure secret store client: %w", err)
		}
		s.secretClient = client
	}

	// Register cleanup worker using BaseService's ticker worker
	base.AddTickerWorker(s.cleanupInterval, func(ctx context.Context) error {
		s.purgeExpiredJobs()
		return nil
	})

	// Register statistics provider for /info endpoint
	base.WithStats(s.statistics)

	// Register standard routes (/health, /info) plus service-specific routes
	base.RegisterStandardRoutes()
	s.registerRoutes()

	return s, nil
}

// statistics returns runtime statistics for the /info endpoint.
func (s *Service) statistics() map[string]any {
	jobCount := 0
	runningCount := 0
	now := time.Now()

	s.jobs.Range(func(key, value interface{}) bool {
		entry, ok := value.(jobEntry)
		if !ok {
			return true
		}
		if !s.isJobExpired(entry, now) {
			jobCount++
			if entry.Response.Status == "running" {
				runningCount++
			}
		}
		return true
	})

	return map[string]any{
		"total_jobs":       jobCount,
		"running_jobs":     runningCount,
		"result_ttl":       s.resultTTL.String(),
		"cleanup_interval": s.cleanupInterval.String(),
	}
}

// deriveSigningKey derives a signing key from the master key using HKDF.
func deriveSigningKey(masterKey []byte) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, nil
	}
	// Use HKDF to derive a separate key for signing
	// This ensures encryption and signing use different keys
	salt := []byte("neocompute-signing-key")
	info := "tee-result-signing"

	hkdfReader := hkdf.New(sha256.New, masterKey, salt, []byte(info))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func loadResultTTLOverride() time.Duration {
	value := os.Getenv(neocomputeResultTTLEnv)
	if value == "" {
		return 0
	}
	ttl, err := time.ParseDuration(value)
	if err != nil || ttl <= 0 {
		return 0
	}
	return ttl
}

// storeJob stores a job result for later retrieval.
func (s *Service) storeJob(userID string, response *ExecuteResponse) {
	s.jobs.Store(response.JobID, jobEntry{
		UserID:   userID,
		Response: response,
		storedAt: time.Now(),
	})
}

// getJob retrieves a job by ID, returning nil if not found or not owned by user.
func (s *Service) getJob(userID, jobID string) *ExecuteResponse {
	val, ok := s.jobs.Load(jobID)
	if !ok {
		return nil
	}
	entry, ok := val.(jobEntry)
	if !ok {
		s.jobs.Delete(jobID)
		return nil
	}
	if s.isJobExpired(entry, time.Now()) {
		s.jobs.Delete(jobID)
		return nil
	}
	if entry.UserID != userID {
		return nil // User doesn't own this job
	}
	return entry.Response
}

// listJobs returns all jobs for a user.
func (s *Service) listJobs(userID string) []*ExecuteResponse {
	var jobs []*ExecuteResponse
	now := time.Now()
	s.jobs.Range(func(key, value interface{}) bool {
		entry, ok := value.(jobEntry)
		if !ok {
			return true
		}
		if s.isJobExpired(entry, now) {
			s.jobs.Delete(key)
			return true
		}
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
	now := time.Now()
	s.jobs.Range(func(key, value interface{}) bool {
		entry, ok := value.(jobEntry)
		if !ok {
			return true
		}
		if s.isJobExpired(entry, now) {
			s.jobs.Delete(key)
			return true
		}
		if entry.UserID == userID && entry.Response.Status == "running" {
			count++
		}
		return true
	})
	return count
}

func (s *Service) purgeExpiredJobs() {
	if s.resultTTL <= 0 {
		return
	}
	now := time.Now()
	s.jobs.Range(func(key, value interface{}) bool {
		entry, ok := value.(jobEntry)
		if !ok {
			s.jobs.Delete(key)
			return true
		}
		if s.isJobExpired(entry, now) {
			s.jobs.Delete(key)
		}
		return true
	})
}

func (s *Service) isJobExpired(entry jobEntry, now time.Time) bool {
	if s.resultTTL <= 0 {
		return false
	}
	expiration := entry.storedAt.Add(s.resultTTL)
	return expiration.Before(now)
}
