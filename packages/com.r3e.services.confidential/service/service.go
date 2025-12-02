package confidential

import (
	"context"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// Service manages enclave registrations and sealed keys.
type Service struct {
	*framework.SandboxedServiceEngine
	store          Store
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
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:        "confidential",
				Description: "Confidential compute enclaves, keys, and attestations",
				Accounts:    accounts,
				Logger:      log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
				sandbox.CapCryptoSign,
				sandbox.CapCryptoEncrypt,
			},
			StorageQuota: 10 * 1024 * 1024,
		}),
		store:          store,
		sealedKeyHooks: core.NoopObservationHooks,
		attestHooks:    core.NoopObservationHooks,
	}
}

// WithSealedKeyHooks configures observability hooks for sealed key storage.
func (s *Service) WithSealedKeyHooks(h core.ObservationHooks) {
	s.sealedKeyHooks = core.NormalizeHooks(h)
}

// WithAttestationHooks configures observability hooks for attestation storage.
func (s *Service) WithAttestationHooks(h core.ObservationHooks) {
	s.attestHooks = core.NormalizeHooks(h)
}

// CreateEnclave registers a new enclave for an account.
func (s *Service) CreateEnclave(ctx context.Context, enclave Enclave) (Enclave, error) {
	if err := s.ValidateAccountExists(ctx, enclave.AccountID); err != nil {
		return Enclave{}, err
	}
	if err := s.normalizeEnclave(&enclave); err != nil {
		return Enclave{}, err
	}
	attrs := map[string]string{"account_id": enclave.AccountID, "resource": "enclave"}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateEnclave(ctx, enclave)
	if err == nil && created.ID != "" {
		attrs["enclave_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Enclave{}, err
	}
	s.Logger().WithField("enclave_id", created.ID).WithField("account_id", created.AccountID).Info("enclave registered")
	s.LogCreated("enclave", created.ID, created.AccountID)
	s.IncrementCounter("confidential_enclaves_created_total", map[string]string{"account_id": created.AccountID})
	return created, nil
}

// UpdateEnclave updates enclave metadata/status.
func (s *Service) UpdateEnclave(ctx context.Context, enclave Enclave) (Enclave, error) {
	stored, err := s.store.GetEnclave(ctx, enclave.ID)
	if err != nil {
		return Enclave{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, enclave.AccountID, "enclave", enclave.ID); err != nil {
		return Enclave{}, err
	}
	enclave.AccountID = stored.AccountID
	if err := s.normalizeEnclave(&enclave); err != nil {
		return Enclave{}, err
	}
	attrs := map[string]string{"account_id": enclave.AccountID, "enclave_id": enclave.ID, "resource": "enclave"}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdateEnclave(ctx, enclave)
	finish(err)
	if err != nil {
		return Enclave{}, err
	}
	s.Logger().WithField("enclave_id", enclave.ID).WithField("account_id", enclave.AccountID).Info("enclave updated")
	s.LogUpdated("enclave", enclave.ID, enclave.AccountID)
	s.IncrementCounter("confidential_enclaves_updated_total", map[string]string{"account_id": enclave.AccountID})
	return updated, nil
}

// ListEnclaves lists an account's enclaves.
func (s *Service) ListEnclaves(ctx context.Context, accountID string) ([]Enclave, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListEnclaves(ctx, accountID)
}

// GetEnclave fetches a single enclave.
func (s *Service) GetEnclave(ctx context.Context, accountID, enclaveID string) (Enclave, error) {
	enclave, err := s.store.GetEnclave(ctx, enclaveID)
	if err != nil {
		return Enclave{}, err
	}
	if err := core.EnsureOwnership(enclave.AccountID, accountID, "enclave", enclaveID); err != nil {
		return Enclave{}, err
	}
	return enclave, nil
}

// CreateSealedKey stores sealed key material for an enclave.
func (s *Service) CreateSealedKey(ctx context.Context, key SealedKey) (SealedKey, error) {
	if err := s.ValidateAccountExists(ctx, key.AccountID); err != nil {
		return SealedKey{}, err
	}
	if _, err := s.GetEnclave(ctx, key.AccountID, key.EnclaveID); err != nil {
		return SealedKey{}, err
	}
	key.Name = strings.TrimSpace(key.Name)
	if key.Name == "" {
		return SealedKey{}, core.RequiredError("name")
	}
	key.Metadata = core.NormalizeMetadata(key.Metadata)
	attrs := map[string]string{"account_id": key.AccountID, "enclave_id": key.EnclaveID, "resource": "sealed_key"}
	ctx, engineFinish := s.StartObservation(ctx, attrs)
	customFinish := core.StartObservation(ctx, s.sealedKeyHooks, attrs)
	created, err := s.store.CreateSealedKey(ctx, key)
	if err == nil && created.ID != "" {
		attrs["sealed_key_id"] = created.ID
	}
	customFinish(err)
	engineFinish(err)
	if err != nil {
		return SealedKey{}, err
	}
	s.Logger().WithField("sealed_key_id", created.ID).WithField("enclave_id", created.EnclaveID).Info("sealed key stored")
	s.LogCreated("sealed_key", created.ID, created.AccountID)
	s.IncrementCounter("confidential_sealed_keys_created_total", map[string]string{"account_id": created.AccountID, "enclave_id": created.EnclaveID})
	return created, nil
}

// ListSealedKeys lists keys for an account/enclave.
func (s *Service) ListSealedKeys(ctx context.Context, accountID, enclaveID string, limit int) ([]SealedKey, error) {
	if _, err := s.GetEnclave(ctx, accountID, enclaveID); err != nil {
		return nil, err
	}
	return s.store.ListSealedKeys(ctx, accountID, enclaveID, clampAttestationLimit(limit))
}

// CreateAttestation stores an attestation proof for an enclave.
func (s *Service) CreateAttestation(ctx context.Context, att Attestation) (Attestation, error) {
	if err := s.ValidateAccountExists(ctx, att.AccountID); err != nil {
		return Attestation{}, err
	}
	if _, err := s.GetEnclave(ctx, att.AccountID, att.EnclaveID); err != nil {
		return Attestation{}, err
	}
	att.Report = strings.TrimSpace(att.Report)
	if att.Report == "" {
		return Attestation{}, core.RequiredError("report")
	}
	att.Status = strings.TrimSpace(att.Status)
	if att.Status == "" {
		att.Status = "pending"
	}
	att.Metadata = core.NormalizeMetadata(att.Metadata)
	attrs := map[string]string{"account_id": att.AccountID, "enclave_id": att.EnclaveID, "resource": "attestation"}
	ctx, engineFinish := s.StartObservation(ctx, attrs)
	customFinish := core.StartObservation(ctx, s.attestHooks, attrs)
	created, err := s.store.CreateAttestation(ctx, att)
	if err == nil && created.ID != "" {
		attrs["attestation_id"] = created.ID
	}
	customFinish(err)
	engineFinish(err)
	if err != nil {
		return Attestation{}, err
	}
	s.Logger().WithField("attestation_id", created.ID).WithField("enclave_id", created.EnclaveID).Info("attestation recorded")
	s.LogCreated("attestation", created.ID, created.AccountID)
	s.IncrementCounter("confidential_attestations_created_total", map[string]string{"account_id": created.AccountID, "enclave_id": created.EnclaveID})
	return created, nil
}

// ListAttestations lists proofs for an enclave.
func (s *Service) ListAttestations(ctx context.Context, accountID, enclaveID string, limit int) ([]Attestation, error) {
	if _, err := s.GetEnclave(ctx, accountID, enclaveID); err != nil {
		return nil, err
	}
	return s.store.ListAttestations(ctx, accountID, enclaveID, clampAttestationLimit(limit))
}

// ListAccountAttestations aggregates attestations across all enclaves for an account.
func (s *Service) ListAccountAttestations(ctx context.Context, accountID string, limit int) ([]Attestation, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListAccountAttestations(ctx, accountID, clampAttestationLimit(limit))
}

func (s *Service) normalizeEnclave(enclave *Enclave) error {
	enclave.Name = strings.TrimSpace(enclave.Name)
	enclave.Endpoint = strings.TrimSpace(enclave.Endpoint)
	enclave.Attestation = strings.TrimSpace(enclave.Attestation)
	enclave.Metadata = core.NormalizeMetadata(enclave.Metadata)
	if enclave.Name == "" {
		return core.RequiredError("name")
	}
	if enclave.Endpoint == "" {
		return core.RequiredError("endpoint")
	}
	status := EnclaveStatus(strings.ToLower(strings.TrimSpace(string(enclave.Status))))
	if status == "" {
		status = EnclaveStatusInactive
	}
	switch status {
	case EnclaveStatusInactive, EnclaveStatusActive, EnclaveStatusRevoked:
		enclave.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetEnclaves handles GET /enclaves - list all enclaves for an account.
func (s *Service) HTTPGetEnclaves(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListEnclaves(ctx, req.AccountID)
}

// HTTPPostEnclaves handles POST /enclaves - create a new enclave.
func (s *Service) HTTPPostEnclaves(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	endpoint, _ := req.Body["endpoint"].(string)
	attestation, _ := req.Body["attestation"].(string)
	status, _ := req.Body["status"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	enclave := Enclave{
		AccountID:   req.AccountID,
		Name:        name,
		Endpoint:    endpoint,
		Attestation: attestation,
		Status:      EnclaveStatus(status),
		Metadata:    metadata,
	}

	return s.CreateEnclave(ctx, enclave)
}

// HTTPGetEnclavesById handles GET /enclaves/{id} - get a specific enclave.
func (s *Service) HTTPGetEnclavesById(ctx context.Context, req core.APIRequest) (any, error) {
	enclaveID := req.PathParams["id"]
	return s.GetEnclave(ctx, req.AccountID, enclaveID)
}

// HTTPPatchEnclavesById handles PATCH /enclaves/{id} - update an enclave.
func (s *Service) HTTPPatchEnclavesById(ctx context.Context, req core.APIRequest) (any, error) {
	enclaveID := req.PathParams["id"]

	// Get existing enclave first
	existing, err := s.GetEnclave(ctx, req.AccountID, enclaveID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if name, ok := req.Body["name"].(string); ok {
		existing.Name = name
	}
	if endpoint, ok := req.Body["endpoint"].(string); ok {
		existing.Endpoint = endpoint
	}
	if attestation, ok := req.Body["attestation"].(string); ok {
		existing.Attestation = attestation
	}
	if status, ok := req.Body["status"].(string); ok {
		existing.Status = EnclaveStatus(status)
	}

	existing.AccountID = req.AccountID
	return s.UpdateEnclave(ctx, existing)
}

// HTTPGetEnclavesIdKeys handles GET /enclaves/{id}/keys - list sealed keys.
func (s *Service) HTTPGetEnclavesIdKeys(ctx context.Context, req core.APIRequest) (any, error) {
	enclaveID := req.PathParams["id"]
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListSealedKeys(ctx, req.AccountID, enclaveID, limit)
}

// HTTPPostEnclavesIdKeys handles POST /enclaves/{id}/keys - create a sealed key.
func (s *Service) HTTPPostEnclavesIdKeys(ctx context.Context, req core.APIRequest) (any, error) {
	enclaveID := req.PathParams["id"]
	name, _ := req.Body["name"].(string)
	blobStr, _ := req.Body["blob"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	key := SealedKey{
		AccountID: req.AccountID,
		EnclaveID: enclaveID,
		Name:      name,
		Blob:      []byte(blobStr),
		Metadata:  metadata,
	}

	return s.CreateSealedKey(ctx, key)
}

// HTTPGetEnclavesIdAttestations handles GET /enclaves/{id}/attestations - list attestations.
func (s *Service) HTTPGetEnclavesIdAttestations(ctx context.Context, req core.APIRequest) (any, error) {
	enclaveID := req.PathParams["id"]
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListAttestations(ctx, req.AccountID, enclaveID, limit)
}

// HTTPPostEnclavesIdAttestations handles POST /enclaves/{id}/attestations - create attestation.
func (s *Service) HTTPPostEnclavesIdAttestations(ctx context.Context, req core.APIRequest) (any, error) {
	enclaveID := req.PathParams["id"]
	report, _ := req.Body["report"].(string)
	status, _ := req.Body["status"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	att := Attestation{
		AccountID: req.AccountID,
		EnclaveID: enclaveID,
		Report:    report,
		Status:    status,
		Metadata:  metadata,
	}

	return s.CreateAttestation(ctx, att)
}

// HTTPGetAttestations handles GET /attestations - list all attestations for an account.
func (s *Service) HTTPGetAttestations(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListAccountAttestations(ctx, req.AccountID, limit)
}
