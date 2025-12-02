// Package datastreams provides the Data Streams Service as a ServicePackage.
package datastreams

import (
	"context"
	"database/sql"
	"encoding/json"
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

// NewPostgresStore creates a new PostgreSQL-backed data streams store.
func NewPostgresStore(db *sql.DB, accounts AccountChecker) *PostgresStore {
	return &PostgresStore{db: db, accounts: accounts}
}

// CreateStream creates a new data stream.
func (s *PostgresStore) CreateStream(ctx context.Context, stream Stream) (Stream, error) {
	if stream.ID == "" {
		stream.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	stream.CreatedAt = now
	stream.UpdatedAt = now
	tenant := s.accounts.AccountTenant(ctx, stream.AccountID)

	metaJSON, err := json.Marshal(stream.Metadata)
	if err != nil {
		return Stream{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_datastreams
			(id, account_id, name, symbol, description, frequency, sla_ms, status, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, stream.ID, stream.AccountID, stream.Name, stream.Symbol, stream.Description, stream.Frequency, stream.SLAms, stream.Status, metaJSON, tenant, stream.CreatedAt, stream.UpdatedAt)
	if err != nil {
		return Stream{}, err
	}
	return stream, nil
}

// UpdateStream updates an existing data stream.
func (s *PostgresStore) UpdateStream(ctx context.Context, stream Stream) (Stream, error) {
	existing, err := s.GetStream(ctx, stream.ID)
	if err != nil {
		return Stream{}, err
	}
	stream.CreatedAt = existing.CreatedAt
	stream.UpdatedAt = time.Now().UTC()

	metaJSON, err := json.Marshal(stream.Metadata)
	if err != nil {
		return Stream{}, err
	}
	tenant := s.accounts.AccountTenant(ctx, stream.AccountID)

	result, err := s.db.ExecContext(ctx, `
		UPDATE chainlink_datastreams
		SET name = $2, symbol = $3, description = $4, frequency = $5, sla_ms = $6, status = $7, metadata = $8, tenant = $9, updated_at = $10
		WHERE id = $1
	`, stream.ID, stream.Name, stream.Symbol, stream.Description, stream.Frequency, stream.SLAms, stream.Status, metaJSON, tenant, stream.UpdatedAt)
	if err != nil {
		return Stream{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return Stream{}, sql.ErrNoRows
	}
	return stream, nil
}

// GetStream retrieves a data stream by ID.
func (s *PostgresStore) GetStream(ctx context.Context, id string) (Stream, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, name, symbol, description, frequency, sla_ms, status, metadata, created_at, updated_at
		FROM chainlink_datastreams
		WHERE id = $1
	`, id)
	return s.scanDataStream(row)
}

// ListStreams lists data streams for an account.
func (s *PostgresStore) ListStreams(ctx context.Context, accountID string) ([]Stream, error) {
	tenant := s.accounts.AccountTenant(ctx, accountID)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, symbol, description, frequency, sla_ms, status, metadata, created_at, updated_at
		FROM chainlink_datastreams
		WHERE account_id = $1 AND ($2 = '' OR tenant = $2)
		ORDER BY created_at DESC
	`, accountID, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []Stream
	for rows.Next() {
		stream, err := s.scanDataStream(rows)
		if err != nil {
			return nil, err
		}
		streams = append(streams, stream)
	}
	return streams, rows.Err()
}

// CreateFrame creates a new data stream frame.
func (s *PostgresStore) CreateFrame(ctx context.Context, frame Frame) (Frame, error) {
	if frame.ID == "" {
		frame.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	frame.CreatedAt = now
	tenant := s.accounts.AccountTenant(ctx, frame.AccountID)

	payloadJSON, err := json.Marshal(frame.Payload)
	if err != nil {
		return Frame{}, err
	}
	metaJSON, err := json.Marshal(frame.Metadata)
	if err != nil {
		return Frame{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chainlink_datastream_frames
			(id, account_id, stream_id, sequence, payload, latency_ms, status, metadata, tenant, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, frame.ID, frame.AccountID, frame.StreamID, frame.Sequence, payloadJSON, frame.LatencyMS, frame.Status, metaJSON, tenant, frame.CreatedAt, now)
	if err != nil {
		return Frame{}, err
	}
	return frame, nil
}

// ListFrames lists frames for a stream.
func (s *PostgresStore) ListFrames(ctx context.Context, streamID string, limit int) ([]Frame, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, stream_id, sequence, payload, latency_ms, status, metadata, created_at, updated_at
		FROM chainlink_datastream_frames
		WHERE stream_id = $1
		ORDER BY sequence DESC
		LIMIT $2
	`, streamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var frames []Frame
	for rows.Next() {
		frame, err := s.scanDataStreamFrame(rows)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	return frames, rows.Err()
}

// GetLatestFrame retrieves the latest frame for a stream.
func (s *PostgresStore) GetLatestFrame(ctx context.Context, streamID string) (Frame, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, stream_id, sequence, payload, latency_ms, status, metadata, created_at, updated_at
		FROM chainlink_datastream_frames
		WHERE stream_id = $1
		ORDER BY sequence DESC
		LIMIT 1
	`, streamID)
	return s.scanDataStreamFrame(row)
}


func (s *PostgresStore) scanDataStream(scanner core.RowScanner) (Stream, error) {
	var (
		stream  Stream
		metaRaw []byte
	)
	if err := scanner.Scan(&stream.ID, &stream.AccountID, &stream.Name, &stream.Symbol, &stream.Description, &stream.Frequency, &stream.SLAms, &stream.Status, &metaRaw, &stream.CreatedAt, &stream.UpdatedAt); err != nil {
		return Stream{}, err
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &stream.Metadata)
	}
	return stream, nil
}

func (s *PostgresStore) scanDataStreamFrame(scanner core.RowScanner) (Frame, error) {
	var (
		frame   Frame
		payload []byte
		metaRaw []byte
		updated time.Time
	)
	if err := scanner.Scan(&frame.ID, &frame.AccountID, &frame.StreamID, &frame.Sequence, &payload, &frame.LatencyMS, &frame.Status, &metaRaw, &frame.CreatedAt, &updated); err != nil {
		return Frame{}, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &frame.Payload)
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &frame.Metadata)
	}
	return frame, nil
}
