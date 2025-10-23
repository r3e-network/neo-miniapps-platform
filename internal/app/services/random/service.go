package random

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	domain "github.com/R3E-Network/service_layer/internal/app/domain/random"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service provides random number generation utilities.
type Service struct {
	log *logger.Logger
}

// New constructs a random service.
func New(log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("random")
	}
	return &Service{log: log}
}

// Generate returns cryptographically secure random bytes of the requested length.
func (s *Service) Generate(ctx context.Context, length int) (domain.Result, error) {
	_ = ctx
	if length <= 0 || length > 1024 {
		return domain.Result{}, fmt.Errorf("length must be between 1 and 1024")
	}

	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return domain.Result{}, fmt.Errorf("read randomness: %w", err)
	}

	s.log.Debugf("generated %d random bytes", length)
	return domain.Result{Value: buf, CreatedAt: time.Now().UTC()}, nil
}

// EncodeResult encodes the random bytes using base64 for transport.
func EncodeResult(res domain.Result) string {
	return base64.StdEncoding.EncodeToString(res.Value)
}
