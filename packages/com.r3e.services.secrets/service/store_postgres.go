package secrets

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)


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


// SecretStore implementation

func (s *PostgresStore) CreateSecret(ctx context.Context, sec Secret) (Secret, error) {
	if sec.ID == "" {
		sec.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	sec.CreatedAt = now
	sec.UpdatedAt = now
	sec.Version = 1
	tenant := s.accountTenant(ctx, sec.AccountID)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_secrets (id, account_id, tenant, name, value, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sec.ID, sec.AccountID, tenant, sec.Name, sec.Value, sec.Version, sec.CreatedAt, sec.UpdatedAt)
	if err != nil {
		return Secret{}, err
	}
	return sec, nil
}

func (s *PostgresStore) UpdateSecret(ctx context.Context, sec Secret) (Secret, error) {
	existing, err := s.GetSecret(ctx, sec.AccountID, sec.Name)
	if err != nil {
		return Secret{}, err
	}

	sec.ID = existing.ID
	sec.AccountID = existing.AccountID
	sec.Name = existing.Name
	sec.CreatedAt = existing.CreatedAt
	sec.Version = existing.Version + 1
	sec.UpdatedAt = time.Now().UTC()
	tenant := s.accountTenant(ctx, sec.AccountID)

	_, err = s.db.ExecContext(ctx, `
		UPDATE app_secrets
		SET value = $1, version = $2, updated_at = $3, tenant = $4
		WHERE id = $5
	`, sec.Value, sec.Version, sec.UpdatedAt, tenant, sec.ID)
	if err != nil {
		return Secret{}, err
	}
	return sec, nil
}

func (s *PostgresStore) GetSecret(ctx context.Context, accountID, name string) (Secret, error) {
	tenant := s.accountTenant(ctx, accountID)
	query := `
		SELECT id, account_id, name, value, version, created_at, updated_at
		FROM app_secrets
		WHERE account_id = $1 AND lower(name) = lower($2)
	`
	args := []any{accountID, name}
	if tenant != "" {
		query += " AND tenant = $3"
		args = append(args, tenant)
	}
	row := s.db.QueryRowContext(ctx, query, args...)

	sec, err := scanSecret(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Secret{}, fmt.Errorf("secret %s not found for account %s", name, accountID)
		}
		return Secret{}, err
	}
	return sec, nil
}

func (s *PostgresStore) ListSecrets(ctx context.Context, accountID string) ([]Secret, error) {
	tenant := s.accountTenant(ctx, accountID)
	query := `
		SELECT id, account_id, name, value, version, created_at, updated_at
		FROM app_secrets
		WHERE account_id = $1
	`
	args := []any{accountID}
	if tenant != "" {
		query += " AND tenant = $2"
		args = append(args, tenant)
	}
	query += " ORDER BY created_at"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Secret
	for rows.Next() {
		sec, err := scanSecret(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, sec)
	}
	return result, rows.Err()
}

func (s *PostgresStore) DeleteSecret(ctx context.Context, accountID, name string) error {
	tenant := s.accountTenant(ctx, accountID)
	query := `
		DELETE FROM app_secrets
		WHERE account_id = $1 AND lower(name) = lower($2)
	`
	args := []any{accountID, name}
	if tenant != "" {
		query += " AND tenant = $3"
		args = append(args, tenant)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("secret %s not found for account %s", name, accountID)
	}
	return nil
}

func scanSecret(scanner core.RowScanner) (Secret, error) {
	var sec Secret
	if err := scanner.Scan(&sec.ID, &sec.AccountID, &sec.Name, &sec.Value, &sec.Version, &sec.CreatedAt, &sec.UpdatedAt); err != nil {
		return Secret{}, err
	}
	return sec, nil
}
