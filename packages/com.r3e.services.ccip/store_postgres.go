package ccip

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CCIPStore implementation

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

// rowScanner abstracts *sql.Row and *sql.Rows for scanning.
type rowScanner interface {
	Scan(dest ...any) error
}


func (s *PostgresStore) CreateLane(ctx context.Context, lane Lane) (Lane, error) {
	if lane.ID == "" {
		lane.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	lane.CreatedAt = now
	lane.UpdatedAt = now

	signerJSON, err := json.Marshal(lane.SignerSet)
	if err != nil {
		return Lane{}, err
	}
	tokensJSON, err := json.Marshal(lane.AllowedTokens)
	if err != nil {
		return Lane{}, err
	}
	policyJSON, err := json.Marshal(lane.DeliveryPolicy)
	if err != nil {
		return Lane{}, err
	}
	metaJSON, err := json.Marshal(lane.Metadata)
	if err != nil {
		return Lane{}, err
	}
	tagsJSON, err := json.Marshal(lane.Tags)
	if err != nil {
		return Lane{}, err
	}
	tenant := s.accountTenant(ctx, lane.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_ccip_lanes (id, account_id, name, source_chain, dest_chain, signer_set, allowed_tokens, delivery_policy, metadata, tags, tenant, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, lane.ID, lane.AccountID, lane.Name, lane.SourceChain, lane.DestChain, signerJSON, tokensJSON, policyJSON, metaJSON, tagsJSON, tenant, lane.CreatedAt, lane.UpdatedAt)
	if err != nil {
		return Lane{}, err
	}
	return lane, nil
}

func (s *PostgresStore) UpdateLane(ctx context.Context, lane Lane) (Lane, error) {
	existing, err := s.GetLane(ctx, lane.ID)
	if err != nil {
		return Lane{}, err
	}
	lane.AccountID = existing.AccountID
	lane.CreatedAt = existing.CreatedAt
	lane.UpdatedAt = time.Now().UTC()

	signerJSON, err := json.Marshal(lane.SignerSet)
	if err != nil {
		return Lane{}, err
	}
	tokensJSON, err := json.Marshal(lane.AllowedTokens)
	if err != nil {
		return Lane{}, err
	}
	policyJSON, err := json.Marshal(lane.DeliveryPolicy)
	if err != nil {
		return Lane{}, err
	}
	metaJSON, err := json.Marshal(lane.Metadata)
	if err != nil {
		return Lane{}, err
	}
	tagsJSON, err := json.Marshal(lane.Tags)
	if err != nil {
		return Lane{}, err
	}
	tenant := s.accountTenant(ctx, lane.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_ccip_lanes
		SET name = $2, source_chain = $3, dest_chain = $4, signer_set = $5, allowed_tokens = $6, delivery_policy = $7, metadata = $8, tags = $9, tenant = $10, updated_at = $11
		WHERE id = $1
	`, lane.ID, lane.Name, lane.SourceChain, lane.DestChain, signerJSON, tokensJSON, policyJSON, metaJSON, tagsJSON, tenant, lane.UpdatedAt)
	if err != nil {
		return Lane{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Lane{}, sql.ErrNoRows
	}
	return lane, nil
}

func (s *PostgresStore) GetLane(ctx context.Context, id string) (Lane, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, source_chain, dest_chain, signer_set, allowed_tokens, delivery_policy, metadata, tags, created_at, updated_at
		FROM app_ccip_lanes
		WHERE id = $1
	`, id)

	return scanLane(row)
}

func (s *PostgresStore) ListLanes(ctx context.Context, accountID string) ([]Lane, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, source_chain, dest_chain, signer_set, allowed_tokens, delivery_policy, metadata, tags, created_at, updated_at
		FROM app_ccip_lanes
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Lane
	for rows.Next() {
		lane, err := scanLane(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, lane)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateMessage(ctx context.Context, msg Message) (Message, error) {
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	msg.CreatedAt = now
	msg.UpdatedAt = now

	payloadJSON, err := json.Marshal(msg.Payload)
	if err != nil {
		return Message{}, err
	}
	tokensJSON, err := json.Marshal(msg.TokenTransfers)
	if err != nil {
		return Message{}, err
	}
	traceJSON, err := json.Marshal(msg.Trace)
	if err != nil {
		return Message{}, err
	}
	metaJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return Message{}, err
	}
	tagsJSON, err := json.Marshal(msg.Tags)
	if err != nil {
		return Message{}, err
	}
	tenant := s.accountTenant(ctx, msg.AccountID)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO app_ccip_messages (id, account_id, lane_id, status, payload, token_transfers, trace, error, metadata, tags, tenant, created_at, updated_at, delivered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, msg.ID, msg.AccountID, msg.LaneID, msg.Status, payloadJSON, tokensJSON, traceJSON, msg.Error, metaJSON, tagsJSON, tenant, msg.CreatedAt, msg.UpdatedAt, toNullTime(ptrTime(msg.DeliveredAt)))
	if err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (s *PostgresStore) UpdateMessage(ctx context.Context, msg Message) (Message, error) {
	existing, err := s.GetMessage(ctx, msg.ID)
	if err != nil {
		return Message{}, err
	}
	msg.AccountID = existing.AccountID
	msg.LaneID = existing.LaneID
	msg.CreatedAt = existing.CreatedAt
	msg.UpdatedAt = time.Now().UTC()

	payloadJSON, err := json.Marshal(msg.Payload)
	if err != nil {
		return Message{}, err
	}
	tokensJSON, err := json.Marshal(msg.TokenTransfers)
	if err != nil {
		return Message{}, err
	}
	traceJSON, err := json.Marshal(msg.Trace)
	if err != nil {
		return Message{}, err
	}
	metaJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return Message{}, err
	}
	tagsJSON, err := json.Marshal(msg.Tags)
	if err != nil {
		return Message{}, err
	}
	tenant := s.accountTenant(ctx, msg.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE app_ccip_messages
		SET status = $2, payload = $3, token_transfers = $4, trace = $5, error = $6, metadata = $7, tags = $8, tenant = $9, updated_at = $10, delivered_at = $11
		WHERE id = $1
	`, msg.ID, msg.Status, payloadJSON, tokensJSON, traceJSON, msg.Error, metaJSON, tagsJSON, tenant, msg.UpdatedAt, toNullTime(ptrTime(msg.DeliveredAt)))
	if err != nil {
		return Message{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Message{}, sql.ErrNoRows
	}
	return msg, nil
}

func (s *PostgresStore) GetMessage(ctx context.Context, id string) (Message, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, lane_id, status, payload, token_transfers, trace, error, metadata, tags, created_at, updated_at, delivered_at
		FROM app_ccip_messages
		WHERE id = $1
	`, id)
	return scanMessage(row)
}

func (s *PostgresStore) ListMessages(ctx context.Context, accountID string, limit int) ([]Message, error) {
	tenant := s.accountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, lane_id, status, payload, token_transfers, trace, error, metadata, tags, created_at, updated_at, delivered_at
		FROM app_ccip_messages
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, accountID, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Message
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, msg)
	}
	return result, rows.Err()
}

func scanLane(scanner rowScanner) (Lane, error) {
	var (
		lane       Lane
		signerJSON []byte
		tokensJSON []byte
		policyJSON []byte
		metaJSON   []byte
		tagsJSON   []byte
		createdAt  time.Time
		updatedAt  time.Time
	)
	if err := scanner.Scan(&lane.ID, &lane.AccountID, &lane.Name, &lane.SourceChain, &lane.DestChain, &signerJSON, &tokensJSON, &policyJSON, &metaJSON, &tagsJSON, &createdAt, &updatedAt); err != nil {
		return Lane{}, err
	}
	if len(signerJSON) > 0 {
		_ = json.Unmarshal(signerJSON, &lane.SignerSet)
	}
	if len(tokensJSON) > 0 {
		_ = json.Unmarshal(tokensJSON, &lane.AllowedTokens)
	}
	if len(policyJSON) > 0 {
		_ = json.Unmarshal(policyJSON, &lane.DeliveryPolicy)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &lane.Metadata)
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &lane.Tags)
	}
	lane.CreatedAt = createdAt.UTC()
	lane.UpdatedAt = updatedAt.UTC()
	return lane, nil
}

func scanMessage(scanner rowScanner) (Message, error) {
	var (
		msg         Message
		payloadJSON []byte
		tokensJSON  []byte
		traceJSON   []byte
		metaJSON    []byte
		tagsJSON    []byte
		createdAt   time.Time
		updatedAt   time.Time
		deliveredAt sql.NullTime
	)
	if err := scanner.Scan(&msg.ID, &msg.AccountID, &msg.LaneID, &msg.Status, &payloadJSON, &tokensJSON, &traceJSON, &msg.Error, &metaJSON, &tagsJSON, &createdAt, &updatedAt, &deliveredAt); err != nil {
		return Message{}, err
	}
	if len(payloadJSON) > 0 {
		_ = json.Unmarshal(payloadJSON, &msg.Payload)
	}
	if len(tokensJSON) > 0 {
		_ = json.Unmarshal(tokensJSON, &msg.TokenTransfers)
	}
	if len(traceJSON) > 0 {
		_ = json.Unmarshal(traceJSON, &msg.Trace)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &msg.Metadata)
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &msg.Tags)
	}
	msg.CreatedAt = createdAt.UTC()
	msg.UpdatedAt = updatedAt.UTC()
	if deliveredAt.Valid {
		t := deliveredAt.Time.UTC()
		msg.DeliveredAt = &t
	}
	return msg, nil
}

func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func ptrTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
