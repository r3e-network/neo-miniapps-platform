package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	KeyVersionStatusActive     = "active"
	KeyVersionStatusDeprecated = "deprecated"
	KeyVersionStatusExpired    = "expired"
)

var keyVersionStatuses = []string{
	KeyVersionStatusActive,
	KeyVersionStatusDeprecated,
	KeyVersionStatusExpired,
}

// KeyVersion represents a key lifecycle record persisted in Supabase.
type KeyVersion struct {
	ID         int64      `json:"id"`
	KeyVersion string     `json:"key_version"`
	Status     string     `json:"status"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type KeyVersionCreate struct {
	KeyVersion string     `json:"key_version"`
	Status     string     `json:"status"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

type KeyVersionUpdate struct {
	Status     *string    `json:"status,omitempty"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

func (c KeyVersionCreate) validate() error {
	kv := strings.TrimSpace(c.KeyVersion)
	if kv == "" {
		return fmt.Errorf("%w: key_version cannot be empty", ErrInvalidInput)
	}
	if err := ValidateStatus(c.Status, keyVersionStatuses); err != nil {
		return err
	}
	if c.ValidFrom.IsZero() {
		return fmt.Errorf("%w: valid_from is required", ErrInvalidInput)
	}
	return nil
}

func (u KeyVersionUpdate) validate() error {
	if u.Status != nil {
		if err := ValidateStatus(*u.Status, keyVersionStatuses); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetKeyVersion(ctx context.Context, keyVersion string) (*KeyVersion, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("%w: repository not initialized", ErrInvalidInput)
	}
	keyVersion = strings.TrimSpace(keyVersion)
	if keyVersion == "" {
		return nil, fmt.Errorf("%w: keyVersion cannot be empty", ErrInvalidInput)
	}

	query := "key_version=eq." + url.QueryEscape(keyVersion) + "&limit=1"
	data, err := r.client.request(ctx, "GET", "key_versions", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get key version: %v", ErrDatabaseError, err)
	}

	var versions []KeyVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("%w: unmarshal key_versions: %v", ErrDatabaseError, err)
	}
	if len(versions) == 0 {
		return nil, NewNotFoundError("key_version", keyVersion)
	}
	return &versions[0], nil
}

func (r *Repository) GetActiveKeyVersion(ctx context.Context) (*KeyVersion, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("%w: repository not initialized", ErrInvalidInput)
	}

	query := "status=eq." + url.QueryEscape(KeyVersionStatusActive) + "&order=valid_from.desc&limit=1"
	data, err := r.client.request(ctx, "GET", "key_versions", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get active key version: %v", ErrDatabaseError, err)
	}

	var versions []KeyVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("%w: unmarshal key_versions: %v", ErrDatabaseError, err)
	}
	if len(versions) == 0 {
		return nil, NewNotFoundError("key_version", "active")
	}
	return &versions[0], nil
}

func (r *Repository) ListKeyVersionsByStatus(ctx context.Context, statuses []string) ([]KeyVersion, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("%w: repository not initialized", ErrInvalidInput)
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("%w: statuses cannot be empty", ErrInvalidInput)
	}

	cleaned := make([]string, 0, len(statuses))
	for _, status := range statuses {
		status = strings.TrimSpace(status)
		if status == "" {
			continue
		}
		if err := ValidateStatus(status, keyVersionStatuses); err != nil {
			return nil, err
		}
		cleaned = append(cleaned, status)
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("%w: statuses cannot be empty", ErrInvalidInput)
	}

	query := "status=in.(" + strings.Join(cleaned, ",") + ")&order=valid_from.desc"
	data, err := r.client.request(ctx, "GET", "key_versions", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: list key versions: %v", ErrDatabaseError, err)
	}

	var versions []KeyVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("%w: unmarshal key_versions: %v", ErrDatabaseError, err)
	}
	return versions, nil
}

func (r *Repository) CreateKeyVersion(ctx context.Context, create KeyVersionCreate) (*KeyVersion, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("%w: repository not initialized", ErrInvalidInput)
	}
	if err := create.validate(); err != nil {
		return nil, err
	}

	data, err := r.client.request(ctx, "POST", "key_versions", create, "")
	if err != nil {
		return nil, fmt.Errorf("%w: create key_version: %v", ErrDatabaseError, err)
	}

	var versions []KeyVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("%w: unmarshal key_versions: %v", ErrDatabaseError, err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("%w: create key_version returned empty response", ErrDatabaseError)
	}
	return &versions[0], nil
}

func (r *Repository) UpdateKeyVersion(ctx context.Context, keyVersion string, update KeyVersionUpdate) (*KeyVersion, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("%w: repository not initialized", ErrInvalidInput)
	}
	keyVersion = strings.TrimSpace(keyVersion)
	if keyVersion == "" {
		return nil, fmt.Errorf("%w: keyVersion cannot be empty", ErrInvalidInput)
	}
	if err := update.validate(); err != nil {
		return nil, err
	}

	query := "key_version=eq." + url.QueryEscape(keyVersion)
	data, err := r.client.request(ctx, "PATCH", "key_versions", update, query)
	if err != nil {
		return nil, fmt.Errorf("%w: update key_version: %v", ErrDatabaseError, err)
	}

	var versions []KeyVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("%w: unmarshal key_versions: %v", ErrDatabaseError, err)
	}
	if len(versions) == 0 {
		return nil, NewNotFoundError("key_version", keyVersion)
	}
	return &versions[0], nil
}
