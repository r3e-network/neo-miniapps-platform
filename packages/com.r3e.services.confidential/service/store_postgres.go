package confidential

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// --- ConfidentialStore ------------------------------------------------------


// PostgresStore implements Store using PostgreSQL.
// This is the service-local store implementation that uses the generic
// database connection provided by the Service Engine.
type PostgresStore struct {
	db       *sql.DB
	accounts AccountChecker
}

// NewPostgresStore creates a new PostgreSQL-backed store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

func (s *PostgresStore) accountTenant(ctx context.Context, accountID string) string {
	return s.accounts.AccountTenant(ctx, accountID)
}


func (s *PostgresStore) CreateEnclave(ctx context.Context, enclave Enclave) (Enclave, error) {
	if enclave.ID == "" {
		enclave.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	enclave.CreatedAt = now
	enclave.UpdatedAt = now
	tenant := s.accountTenant(ctx, enclave.AccountID)

	metaJSON, err := json.Marshal(enclave.Metadata)
	if err != nil {
		return Enclave{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO confidential_enclaves
			(id, account_id, name, endpoint, attestation, status, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, enclave.ID, enclave.AccountID, enclave.Name, enclave.Endpoint, enclave.Attestation, enclave.Status, metaJSON, tenant, enclave.CreatedAt, enclave.UpdatedAt)
	if err != nil {
		return Enclave{}, err
	}
	return enclave, nil
}

func (s *PostgresStore) UpdateEnclave(ctx context.Context, enclave Enclave) (Enclave, error) {
	existing, err := s.GetEnclave(ctx, enclave.ID)
	if err != nil {
		return Enclave{}, err
	}
	enclave.CreatedAt = existing.CreatedAt
	enclave.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, enclave.AccountID)

	metaJSON, err := json.Marshal(enclave.Metadata)
	if err != nil {
		return Enclave{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE confidential_enclaves
		SET name = $2, endpoint = $3, attestation = $4, status = $5, metadata = $6, tenant = $7, updated_at = $8
		WHERE id = $1
	`, enclave.ID, enclave.Name, enclave.Endpoint, enclave.Attestation, enclave.Status, metaJSON, tenant, enclave.UpdatedAt)
	if err != nil {
		return Enclave{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Enclave{}, sql.ErrNoRows
	}
	return enclave, nil
}

func (s *PostgresStore) GetEnclave(ctx context.Context, id string) (Enclave, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, endpoint, attestation, status, metadata, created_at, updated_at
		FROM confidential_enclaves
		WHERE id = $1
	`, id)
	return scanEnclave(row)
}

func (s *PostgresStore) ListEnclaves(ctx context.Context, accountID string) ([]Enclave, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, endpoint, attestation, status, metadata, created_at, updated_at
		FROM confidential_enclaves
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enclaves []Enclave
	for rows.Next() {
		enclave, err := scanEnclave(rows)
		if err != nil {
			return nil, err
		}
		enclaves = append(enclaves, enclave)
	}
	return enclaves, rows.Err()
}

func (s *PostgresStore) CreateSealedKey(ctx context.Context, key SealedKey) (SealedKey, error) {
	if key.ID == "" {
		key.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	key.CreatedAt = now
	tenant := s.accountTenant(ctx, key.AccountID)

	metaJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		return SealedKey{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO confidential_sealed_keys
			(id, account_id, enclave_id, name, blob, metadata, tenant, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
	`, key.ID, key.AccountID, key.EnclaveID, key.Name, key.Blob, metaJSON, tenant, key.CreatedAt)
	if err != nil {
		return SealedKey{}, err
	}
	return key, nil
}

func (s *PostgresStore) ListSealedKeys(ctx context.Context, accountID, enclaveID string, limit int) ([]SealedKey, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, enclave_id, name, blob, metadata, created_at
		FROM confidential_sealed_keys
		WHERE account_id = $1 AND enclave_id = $2 AND ($3 = '' OR tenant = $3)
		ORDER BY created_at DESC
		LIMIT $4
	`, accountID, enclaveID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []SealedKey
	for rows.Next() {
		key, err := scanSealedKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *PostgresStore) CreateAttestation(ctx context.Context, att Attestation) (Attestation, error) {
	if att.ID == "" {
		att.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	att.CreatedAt = now
	tenant := s.accountTenant(ctx, att.AccountID)

	metaJSON, err := json.Marshal(att.Metadata)
	if err != nil {
		return Attestation{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO confidential_attestations
			(id, account_id, enclave_id, report, valid_until, status, metadata, tenant, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, att.ID, att.AccountID, att.EnclaveID, att.Report, att.ValidUntil, att.Status, metaJSON, tenant, att.CreatedAt)
	if err != nil {
		return Attestation{}, err
	}
	return att, nil
}

func (s *PostgresStore) ListAttestations(ctx context.Context, accountID, enclaveID string, limit int) ([]Attestation, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, enclave_id, report, valid_until, status, metadata, created_at
		FROM confidential_attestations
		WHERE account_id = $1 AND enclave_id = $2 AND ($3 = '' OR tenant = $3)
		ORDER BY created_at DESC
		LIMIT $4
	`, accountID, enclaveID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []Attestation
	for rows.Next() {
		att, err := scanAttestation(rows)
		if err != nil {
			return nil, err
		}
		atts = append(atts, att)
	}
	return atts, rows.Err()
}

func (s *PostgresStore) ListAccountAttestations(ctx context.Context, accountID string, limit int) ([]Attestation, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, enclave_id, report, valid_until, status, metadata, created_at
		FROM confidential_attestations
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, accountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []Attestation
	for rows.Next() {
		att, err := scanAttestation(rows)
		if err != nil {
			return nil, err
		}
		atts = append(atts, att)
	}
	return atts, rows.Err()
}

func scanEnclave(scanner core.RowScanner) (Enclave, error) {
	var (
		enclave Enclave
		metaRaw []byte
	)
	if err := scanner.Scan(&enclave.ID, &enclave.AccountID, &enclave.Name, &enclave.Endpoint, &enclave.Attestation, &enclave.Status, &metaRaw, &enclave.CreatedAt, &enclave.UpdatedAt); err != nil {
		return Enclave{}, err
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &enclave.Metadata)
	}
	return enclave, nil
}

func scanSealedKey(scanner core.RowScanner) (SealedKey, error) {
	var (
		key     SealedKey
		metaRaw []byte
	)
	if err := scanner.Scan(&key.ID, &key.AccountID, &key.EnclaveID, &key.Name, &key.Blob, &metaRaw, &key.CreatedAt); err != nil {
		return SealedKey{}, err
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &key.Metadata)
	}
	return key, nil
}

func scanAttestation(scanner core.RowScanner) (Attestation, error) {
	var (
		att     Attestation
		valid   *time.Time
		metaRaw []byte
	)
	if err := scanner.Scan(&att.ID, &att.AccountID, &att.EnclaveID, &att.Report, &valid, &att.Status, &metaRaw, &att.CreatedAt); err != nil {
		return Attestation{}, err
	}
	att.ValidUntil = valid
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &att.Metadata)
	}
	return att, nil
}
