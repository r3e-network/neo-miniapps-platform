// Package base provides base components for all services.
package base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// SupabaseConfig holds Supabase connection configuration.
type SupabaseConfig struct {
	URL        string
	AnonKey    string
	ServiceKey string
	// Optional per-request user token for RLS; if empty, service key is used.
	AccessToken string
}

// DefaultSupabaseConfig returns config from environment variables.
func DefaultSupabaseConfig() SupabaseConfig {
	return SupabaseConfig{
		URL:        os.Getenv("SUPABASE_URL"),
		AnonKey:    os.Getenv("SUPABASE_ANON_KEY"),
		ServiceKey: os.Getenv("SUPABASE_SERVICE_KEY"),
	}
}

// SupabaseStore provides a Supabase PostgreSQL store implementation.
type SupabaseStore[T Entity] struct {
	mu        sync.RWMutex
	config    SupabaseConfig
	tableName string
	client    *http.Client
	ready     bool
}

// NewSupabaseStore creates a new Supabase store.
func NewSupabaseStore[T Entity](config SupabaseConfig, tableName string) *SupabaseStore[T] {
	return &SupabaseStore[T]{
		config:    config,
		tableName: tableName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Initialize initializes the store.
func (s *SupabaseStore[T]) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config.URL == "" || s.config.ServiceKey == "" {
		return fmt.Errorf("supabase URL and service key are required")
	}

	// Test connection
	if err := s.healthCheck(ctx); err != nil {
		return fmt.Errorf("supabase health check failed: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the store.
func (s *SupabaseStore[T]) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	return nil
}

// Health checks store health.
func (s *SupabaseStore[T]) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.healthCheck(ctx)
}

func (s *SupabaseStore[T]) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.restURL()+"?select=id&limit=1", nil)
	if err != nil {
		return err
	}
	s.setHeaders(ctx, req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase error: %s", string(body))
	}
	return nil
}

// Get retrieves an entity by ID.
func (s *SupabaseStore[T]) Get(ctx context.Context, id string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var zero T
	if !s.ready {
		return zero, fmt.Errorf("store not ready")
	}

	reqURL := fmt.Sprintf("%s?id=eq.%s&limit=1", s.restURL(), url.QueryEscape(id))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return zero, err
	}
	s.setHeaders(ctx, req)
	req.Header.Set("Accept", "application/vnd.pgrst.object+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 406 {
		return zero, fmt.Errorf("entity not found: %s", id)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("supabase error: %s", string(body))
	}

	var entity T
	if err := json.NewDecoder(resp.Body).Decode(&entity); err != nil {
		return zero, fmt.Errorf("decode error: %w", err)
	}
	return entity, nil
}

// Create creates a new entity.
func (s *SupabaseStore[T]) Create(ctx context.Context, entity T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.restURL(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	s.setHeaders(ctx, req)
	req.Header.Set("Prefer", "return=minimal")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase error: %s", string(body))
	}
	return nil
}

// Update updates an existing entity.
func (s *SupabaseStore[T]) Update(ctx context.Context, entity T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	id := entity.GetID()
	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	reqURL := fmt.Sprintf("%s?id=eq.%s", s.restURL(), url.QueryEscape(id))
	req, err := http.NewRequestWithContext(ctx, "PATCH", reqURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	s.setHeaders(ctx, req)
	req.Header.Set("Prefer", "return=minimal")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase error: %s", string(body))
	}
	return nil
}

// Delete deletes an entity.
func (s *SupabaseStore[T]) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	reqURL := fmt.Sprintf("%s?id=eq.%s", s.restURL(), url.QueryEscape(id))
	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return err
	}
	s.setHeaders(ctx, req)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase error: %s", string(body))
	}
	return nil
}

// List returns all entities.
func (s *SupabaseStore[T]) List(ctx context.Context) ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.restURL()+"?order=created_at.desc", nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(ctx, req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supabase error: %s", string(body))
	}

	var entities []T
	if err := json.NewDecoder(resp.Body).Decode(&entities); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}
	return entities, nil
}

// ListWithFilter returns entities matching a filter.
func (s *SupabaseStore[T]) ListWithFilter(ctx context.Context, filter string) ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	reqURL := s.restURL()
	if filter != "" {
		reqURL += "?" + filter
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(ctx, req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supabase error: %s", string(body))
	}

	var entities []T
	if err := json.NewDecoder(resp.Body).Decode(&entities); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}
	return entities, nil
}

// Count returns the number of entities.
func (s *SupabaseStore[T]) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return 0, fmt.Errorf("store not ready")
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", s.restURL()+"?select=id", nil)
	if err != nil {
		return 0, err
	}
	s.setHeaders(ctx, req)
	req.Header.Set("Prefer", "count=exact")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("supabase error: status %d", resp.StatusCode)
	}

	// Parse Content-Range header: "0-9/100" or "*/0"
	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return 0, nil
	}

	var count int
	_, _ = fmt.Sscanf(contentRange, "%*d-%*d/%d", &count)
	if count == 0 {
		_, _ = fmt.Sscanf(contentRange, "*/%d", &count)
	}
	return count, nil
}

// Upsert creates or updates an entity.
func (s *SupabaseStore[T]) Upsert(ctx context.Context, entity T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	data, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.restURL(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	s.setHeaders(ctx, req)
	req.Header.Set("Prefer", "resolution=merge-duplicates,return=minimal")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase error: %s", string(body))
	}
	return nil
}

func (s *SupabaseStore[T]) restURL() string {
	return fmt.Sprintf("%s/rest/v1/%s", s.config.URL, s.tableName)
}

func (s *SupabaseStore[T]) setHeaders(ctx context.Context, req *http.Request) {
	token := s.config.ServiceKey
	if tok := AccessTokenFromContext(ctx); tok != "" {
		token = tok
	} else if s.config.AccessToken != "" {
		token = s.config.AccessToken
	}
	req.Header.Set("apikey", token)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
}

// =============================================================================
// Supabase Realtime Support
// =============================================================================

// RealtimeCallback is called when a realtime event occurs.
type RealtimeCallback func(event string, payload json.RawMessage)

// SubscribeToChanges subscribes to realtime changes on the table.
// Note: This is a placeholder - full implementation requires WebSocket support.
func (s *SupabaseStore[T]) SubscribeToChanges(ctx context.Context, callback RealtimeCallback) error {
	// Supabase Realtime requires WebSocket connection
	// This would be implemented using gorilla/websocket or similar
	return fmt.Errorf("realtime subscription not yet implemented")
}

// =============================================================================
// Supabase RPC Support
// =============================================================================

// RPC calls a Supabase database function.
func (s *SupabaseStore[T]) RPC(ctx context.Context, functionName string, params map[string]any) (json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	rpcURL := fmt.Sprintf("%s/rest/v1/rpc/%s", s.config.URL, functionName)
	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	s.setHeaders(ctx, req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase rpc error: %s", string(body))
	}
	return body, nil
}
