package tee

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryScriptStore implements ScriptStore using in-memory storage.
// This is useful for testing and development.
type MemoryScriptStore struct {
	mu      sync.RWMutex
	scripts map[string]ScriptDefinition
	runs    map[string]ScriptRun
}

// NewMemoryScriptStore creates a new in-memory script store.
func NewMemoryScriptStore() *MemoryScriptStore {
	return &MemoryScriptStore{
		scripts: make(map[string]ScriptDefinition),
		runs:    make(map[string]ScriptRun),
	}
}

// CreateScript stores a new script definition.
func (s *MemoryScriptStore) CreateScript(ctx context.Context, def ScriptDefinition) (ScriptDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	def.CreatedAt = now
	def.UpdatedAt = now

	s.scripts[def.ID] = def
	return def, nil
}

// UpdateScript modifies an existing script definition.
func (s *MemoryScriptStore) UpdateScript(ctx context.Context, def ScriptDefinition) (ScriptDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.scripts[def.ID]
	if !ok {
		return ScriptDefinition{}, fmt.Errorf("script not found: %s", def.ID)
	}

	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now().UTC()

	s.scripts[def.ID] = def
	return def, nil
}

// GetScript retrieves a script by ID.
func (s *MemoryScriptStore) GetScript(ctx context.Context, id string) (ScriptDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, ok := s.scripts[id]
	if !ok {
		return ScriptDefinition{}, fmt.Errorf("script not found: %s", id)
	}
	return def, nil
}

// ListScripts returns all scripts for an account.
func (s *MemoryScriptStore) ListScripts(ctx context.Context, accountID string) ([]ScriptDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ScriptDefinition
	for _, def := range s.scripts {
		if def.AccountID == accountID {
			result = append(result, def)
		}
	}
	return result, nil
}

// DeleteScript removes a script.
func (s *MemoryScriptStore) DeleteScript(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.scripts[id]; !ok {
		return fmt.Errorf("script not found: %s", id)
	}
	delete(s.scripts, id)
	return nil
}

// CreateScriptRun stores an execution record.
func (s *MemoryScriptStore) CreateScriptRun(ctx context.Context, run ScriptRun) (ScriptRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if run.ID == "" {
		run.ID = uuid.New().String()
	}

	s.runs[run.ID] = run
	return run, nil
}

// GetScriptRun retrieves an execution record.
func (s *MemoryScriptStore) GetScriptRun(ctx context.Context, id string) (ScriptRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.runs[id]
	if !ok {
		return ScriptRun{}, fmt.Errorf("script run not found: %s", id)
	}
	return run, nil
}

// ListScriptRuns returns execution history for a script.
func (s *MemoryScriptStore) ListScriptRuns(ctx context.Context, scriptID string, limit int) ([]ScriptRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ScriptRun
	for _, run := range s.runs {
		if run.ScriptID == scriptID {
			result = append(result, run)
		}
	}

	// Sort by started_at descending (most recent first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].StartedAt.After(result[i].StartedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// Ensure MemoryScriptStore implements ScriptStore
var _ ScriptStore = (*MemoryScriptStore)(nil)
