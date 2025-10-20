package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
)

// Memory is a thread-safe in-memory persistence layer implementing the storage
// interfaces defined in this package. It is intended for tests and prototyping
// and deliberately keeps the implementation simple.
type Memory struct {
	mu        sync.RWMutex
	nextID    int64
	accounts  map[string]account.Account
	functions map[string]function.Definition
	triggers  map[string]trigger.Trigger
}

// NewMemory creates an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		nextID:    1,
		accounts:  make(map[string]account.Account),
		functions: make(map[string]function.Definition),
		triggers:  make(map[string]trigger.Trigger),
	}
}

func (m *Memory) nextIDLocked() string {
	id := m.nextID
	m.nextID++
	return fmtID(id)
}

func fmtID(id int64) string {
	return fmt.Sprintf("%d", id)
}

// AccountStore implementation -------------------------------------------------

func (m *Memory) CreateAccount(_ context.Context, acct account.Account) (account.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if acct.ID == "" {
		acct.ID = m.nextIDLocked()
	} else {
		if _, exists := m.accounts[acct.ID]; exists {
			return account.Account{}, fmt.Errorf("account %s already exists", acct.ID)
		}
	}

	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now
	acct.Metadata = copyMap(acct.Metadata)

	m.accounts[acct.ID] = acct
	return cloneAccount(acct), nil
}

func (m *Memory) UpdateAccount(_ context.Context, acct account.Account) (account.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	original, ok := m.accounts[acct.ID]
	if !ok {
		return account.Account{}, fmt.Errorf("account %s not found", acct.ID)
	}

	acct.CreatedAt = original.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	acct.Metadata = copyMap(acct.Metadata)

	m.accounts[acct.ID] = acct
	return cloneAccount(acct), nil
}

func (m *Memory) GetAccount(_ context.Context, id string) (account.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	acct, ok := m.accounts[id]
	if !ok {
		return account.Account{}, fmt.Errorf("account %s not found", id)
	}
	return cloneAccount(acct), nil
}

func (m *Memory) ListAccounts(_ context.Context) ([]account.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]account.Account, 0, len(m.accounts))
	for _, acct := range m.accounts {
		result = append(result, cloneAccount(acct))
	}
	return result, nil
}

func (m *Memory) DeleteAccount(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.accounts[id]; !ok {
		return fmt.Errorf("account %s not found", id)
	}
	delete(m.accounts, id)
	return nil
}

// FunctionStore implementation ------------------------------------------------

func (m *Memory) CreateFunction(_ context.Context, def function.Definition) (function.Definition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if def.ID == "" {
		def.ID = m.nextIDLocked()
	} else if _, exists := m.functions[def.ID]; exists {
		return function.Definition{}, fmt.Errorf("function %s already exists", def.ID)
	}

	now := time.Now().UTC()
	def.CreatedAt = now
	def.UpdatedAt = now
	def.Secrets = append([]string(nil), def.Secrets...)

	m.functions[def.ID] = def
	return cloneFunction(def), nil
}

func (m *Memory) UpdateFunction(_ context.Context, def function.Definition) (function.Definition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	original, ok := m.functions[def.ID]
	if !ok {
		return function.Definition{}, fmt.Errorf("function %s not found", def.ID)
	}

	def.CreatedAt = original.CreatedAt
	def.UpdatedAt = time.Now().UTC()
	def.Secrets = append([]string(nil), def.Secrets...)

	m.functions[def.ID] = def
	return cloneFunction(def), nil
}

func (m *Memory) GetFunction(_ context.Context, id string) (function.Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	def, ok := m.functions[id]
	if !ok {
		return function.Definition{}, fmt.Errorf("function %s not found", id)
	}
	return cloneFunction(def), nil
}

func (m *Memory) ListFunctions(_ context.Context, accountID string) ([]function.Definition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]function.Definition, 0)
	for _, def := range m.functions {
		if accountID == "" || def.AccountID == accountID {
			result = append(result, cloneFunction(def))
		}
	}
	return result, nil
}

// TriggerStore implementation -------------------------------------------------

func (m *Memory) CreateTrigger(_ context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if trg.ID == "" {
		trg.ID = m.nextIDLocked()
	} else if _, exists := m.triggers[trg.ID]; exists {
		return trigger.Trigger{}, fmt.Errorf("trigger %s already exists", trg.ID)
	}

	now := time.Now().UTC()
	trg.CreatedAt = now
	trg.UpdatedAt = now

	m.triggers[trg.ID] = trg
	return cloneTrigger(trg), nil
}

func (m *Memory) UpdateTrigger(_ context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	original, ok := m.triggers[trg.ID]
	if !ok {
		return trigger.Trigger{}, fmt.Errorf("trigger %s not found", trg.ID)
	}

	trg.CreatedAt = original.CreatedAt
	trg.UpdatedAt = time.Now().UTC()

	m.triggers[trg.ID] = trg
	return cloneTrigger(trg), nil
}

func (m *Memory) GetTrigger(_ context.Context, id string) (trigger.Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trg, ok := m.triggers[id]
	if !ok {
		return trigger.Trigger{}, fmt.Errorf("trigger %s not found", id)
	}
	return cloneTrigger(trg), nil
}

func (m *Memory) ListTriggers(_ context.Context, accountID string) ([]trigger.Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]trigger.Trigger, 0)
	for _, trg := range m.triggers {
		if accountID == "" || trg.AccountID == accountID {
			result = append(result, cloneTrigger(trg))
		}
	}
	return result, nil
}

// Helpers ---------------------------------------------------------------------

func copyMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneAccount(acct account.Account) account.Account {
	acct.Metadata = copyMap(acct.Metadata)
	return acct
}

func cloneFunction(def function.Definition) function.Definition {
	def.Secrets = append([]string(nil), def.Secrets...)
	return def
}

func cloneTrigger(trg trigger.Trigger) trigger.Trigger {
	return trg
}
