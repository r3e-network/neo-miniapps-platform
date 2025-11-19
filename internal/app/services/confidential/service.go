package confidential

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domainconf "github.com/R3E-Network/service_layer/internal/app/domain/confidential"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages enclave registrations and sealed keys.
type Service struct {
	base           *core.Base
	store          storage.ConfidentialStore
	log            *logger.Logger
	sealedKeyHooks core.ObservationHooks
	attestHooks    core.ObservationHooks
}

const (
	defaultAttestationLimit = 25
	maxAttestationLimit     = 500
)

func clampAttestationLimit(limit int) int {
	return core.ClampLimit(limit, defaultAttestationLimit, maxAttestationLimit)
}

// New constructs a confidential compute service.
func New(accounts storage.AccountStore, store storage.ConfidentialStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("confidential")
	}
	return &Service{
		base:           core.NewBase(accounts),
		store:          store,
		log:            log,
		sealedKeyHooks: core.NoopObservationHooks,
		attestHooks:    core.NoopObservationHooks,
	}
}

// WithSealedKeyHooks configures observability hooks for sealed key storage.
func (s *Service) WithSealedKeyHooks(h core.ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		s.sealedKeyHooks = core.NoopObservationHooks
		return
	}
	s.sealedKeyHooks = h
}

// WithAttestationHooks configures observability hooks for attestation storage.
func (s *Service) WithAttestationHooks(h core.ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		s.attestHooks = core.NoopObservationHooks
		return
	}
	s.attestHooks = h
}

// CreateEnclave registers a new enclave for an account.
func (s *Service) CreateEnclave(ctx context.Context, enclave domainconf.Enclave) (domainconf.Enclave, error) {
	if err := s.base.EnsureAccount(ctx, enclave.AccountID); err != nil {
		return domainconf.Enclave{}, err
	}
	if err := s.normalizeEnclave(&enclave); err != nil {
		return domainconf.Enclave{}, err
	}
	created, err := s.store.CreateEnclave(ctx, enclave)
	if err != nil {
		return domainconf.Enclave{}, err
	}
	s.log.WithField("enclave_id", created.ID).WithField("account_id", created.AccountID).Info("enclave registered")
	return created, nil
}

// UpdateEnclave updates enclave metadata/status.
func (s *Service) UpdateEnclave(ctx context.Context, enclave domainconf.Enclave) (domainconf.Enclave, error) {
	stored, err := s.store.GetEnclave(ctx, enclave.ID)
	if err != nil {
		return domainconf.Enclave{}, err
	}
	if stored.AccountID != enclave.AccountID {
		return domainconf.Enclave{}, fmt.Errorf("enclave %s does not belong to account %s", enclave.ID, enclave.AccountID)
	}
	enclave.AccountID = stored.AccountID
	if err := s.normalizeEnclave(&enclave); err != nil {
		return domainconf.Enclave{}, err
	}
	updated, err := s.store.UpdateEnclave(ctx, enclave)
	if err != nil {
		return domainconf.Enclave{}, err
	}
	s.log.WithField("enclave_id", enclave.ID).WithField("account_id", enclave.AccountID).Info("enclave updated")
	return updated, nil
}

// ListEnclaves lists an account's enclaves.
func (s *Service) ListEnclaves(ctx context.Context, accountID string) ([]domainconf.Enclave, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListEnclaves(ctx, accountID)
}

// GetEnclave fetches a single enclave.
func (s *Service) GetEnclave(ctx context.Context, accountID, enclaveID string) (domainconf.Enclave, error) {
	enclave, err := s.store.GetEnclave(ctx, enclaveID)
	if err != nil {
		return domainconf.Enclave{}, err
	}
	if enclave.AccountID != accountID {
		return domainconf.Enclave{}, fmt.Errorf("enclave %s does not belong to account %s", enclaveID, accountID)
	}
	return enclave, nil
}

// CreateSealedKey stores sealed key material for an enclave.
func (s *Service) CreateSealedKey(ctx context.Context, key domainconf.SealedKey) (domainconf.SealedKey, error) {
	if err := s.base.EnsureAccount(ctx, key.AccountID); err != nil {
		return domainconf.SealedKey{}, err
	}
	if _, err := s.GetEnclave(ctx, key.AccountID, key.EnclaveID); err != nil {
		return domainconf.SealedKey{}, err
	}
	key.Name = strings.TrimSpace(key.Name)
	if key.Name == "" {
		return domainconf.SealedKey{}, fmt.Errorf("name is required")
	}
	key.Metadata = core.NormalizeMetadata(key.Metadata)
	attrs := map[string]string{"account_id": key.AccountID, "enclave_id": key.EnclaveID, "resource": "sealed_key"}
	finish := core.StartObservation(ctx, s.sealedKeyHooks, attrs)
	created, err := s.store.CreateSealedKey(ctx, key)
	if err == nil && created.ID != "" {
		attrs["sealed_key_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return domainconf.SealedKey{}, err
	}
	s.log.WithField("sealed_key_id", created.ID).WithField("enclave_id", created.EnclaveID).Info("sealed key stored")
	return created, nil
}

// ListSealedKeys lists keys for an account/enclave.
func (s *Service) ListSealedKeys(ctx context.Context, accountID, enclaveID string, limit int) ([]domainconf.SealedKey, error) {
	if _, err := s.GetEnclave(ctx, accountID, enclaveID); err != nil {
		return nil, err
	}
	return s.store.ListSealedKeys(ctx, accountID, enclaveID, clampAttestationLimit(limit))
}

// CreateAttestation stores an attestation proof for an enclave.
func (s *Service) CreateAttestation(ctx context.Context, att domainconf.Attestation) (domainconf.Attestation, error) {
	if err := s.base.EnsureAccount(ctx, att.AccountID); err != nil {
		return domainconf.Attestation{}, err
	}
	if _, err := s.GetEnclave(ctx, att.AccountID, att.EnclaveID); err != nil {
		return domainconf.Attestation{}, err
	}
	att.Report = strings.TrimSpace(att.Report)
	if att.Report == "" {
		return domainconf.Attestation{}, fmt.Errorf("report is required")
	}
	att.Status = strings.TrimSpace(att.Status)
	if att.Status == "" {
		att.Status = "pending"
	}
	att.Metadata = core.NormalizeMetadata(att.Metadata)
	attrs := map[string]string{"account_id": att.AccountID, "enclave_id": att.EnclaveID, "resource": "attestation"}
	finish := core.StartObservation(ctx, s.attestHooks, attrs)
	created, err := s.store.CreateAttestation(ctx, att)
	if err == nil && created.ID != "" {
		attrs["attestation_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return domainconf.Attestation{}, err
	}
	s.log.WithField("attestation_id", created.ID).WithField("enclave_id", created.EnclaveID).Info("attestation recorded")
	return created, nil
}

// ListAttestations lists proofs for an enclave.
func (s *Service) ListAttestations(ctx context.Context, accountID, enclaveID string, limit int) ([]domainconf.Attestation, error) {
	if _, err := s.GetEnclave(ctx, accountID, enclaveID); err != nil {
		return nil, err
	}
	return s.store.ListAttestations(ctx, accountID, enclaveID, clampAttestationLimit(limit))
}

// ListAccountAttestations aggregates attestations across all enclaves for an account.
func (s *Service) ListAccountAttestations(ctx context.Context, accountID string, limit int) ([]domainconf.Attestation, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListAccountAttestations(ctx, accountID, clampAttestationLimit(limit))
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "confidential",
		Domain:       "confidential",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"enclaves", "sealed_keys", "attestations"},
	}
}

func (s *Service) normalizeEnclave(enclave *domainconf.Enclave) error {
	enclave.Name = strings.TrimSpace(enclave.Name)
	enclave.Endpoint = strings.TrimSpace(enclave.Endpoint)
	enclave.Attestation = strings.TrimSpace(enclave.Attestation)
	enclave.Metadata = core.NormalizeMetadata(enclave.Metadata)
	if enclave.Name == "" {
		return fmt.Errorf("name is required")
	}
	if enclave.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	status := domainconf.EnclaveStatus(strings.ToLower(strings.TrimSpace(string(enclave.Status))))
	if status == "" {
		status = domainconf.EnclaveStatusInactive
	}
	switch status {
	case domainconf.EnclaveStatusInactive, domainconf.EnclaveStatusActive, domainconf.EnclaveStatusRevoked:
		enclave.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}
