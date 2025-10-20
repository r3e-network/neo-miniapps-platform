package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/internal/app/domain/trigger"
	"github.com/R3E-Network/service_layer/internal/app/storage"
)

// Store is an in-memory implementation of the storage interfaces. It is safe
// for concurrent use and is primarily intended for tests and local development.
type Store struct {
	mu                  sync.RWMutex
	nextID              int64
	accounts            map[string]account.Account
	functions           map[string]function.Definition
	triggers            map[string]trigger.Trigger
	gasAccounts         map[string]gasbank.Account
	gasAccountsByWallet map[string]string
	gasTransactions     map[string][]gasbank.Transaction
	gasTransactionsByID map[string]gasbank.Transaction
	automationJobs      map[string]automation.Job
	priceFeeds          map[string]pricefeed.Feed
	priceSnapshots      map[string][]pricefeed.Snapshot
	oracleSources       map[string]oracle.DataSource
	oracleRequests      map[string]oracle.Request
}

var _ storage.AccountStore = (*Store)(nil)
var _ storage.FunctionStore = (*Store)(nil)
var _ storage.TriggerStore = (*Store)(nil)
var _ storage.GasBankStore = (*Store)(nil)
var _ storage.AutomationStore = (*Store)(nil)
var _ storage.PriceFeedStore = (*Store)(nil)
var _ storage.OracleStore = (*Store)(nil)

// New creates an empty store.
func New() *Store {
	return &Store{
		nextID:              1,
		accounts:            make(map[string]account.Account),
		functions:           make(map[string]function.Definition),
		triggers:            make(map[string]trigger.Trigger),
		gasAccounts:         make(map[string]gasbank.Account),
		gasAccountsByWallet: make(map[string]string),
		gasTransactions:     make(map[string][]gasbank.Transaction),
		gasTransactionsByID: make(map[string]gasbank.Transaction),
		automationJobs:      make(map[string]automation.Job),
		priceFeeds:          make(map[string]pricefeed.Feed),
		priceSnapshots:      make(map[string][]pricefeed.Snapshot),
		oracleSources:       make(map[string]oracle.DataSource),
		oracleRequests:      make(map[string]oracle.Request),
	}
}

func (s *Store) nextIDLocked() string {
	id := s.nextID
	s.nextID++
	return fmt.Sprintf("%d", id)
}

// AccountStore implementation -------------------------------------------------

func (s *Store) CreateAccount(_ context.Context, acct account.Account) (account.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if acct.ID == "" {
		acct.ID = s.nextIDLocked()
	} else if _, exists := s.accounts[acct.ID]; exists {
		return account.Account{}, fmt.Errorf("account %s already exists", acct.ID)
	}

	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now
	acct.Metadata = cloneMap(acct.Metadata)

	s.accounts[acct.ID] = acct
	return cloneAccount(acct), nil
}

func (s *Store) UpdateAccount(_ context.Context, acct account.Account) (account.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.accounts[acct.ID]
	if !ok {
		return account.Account{}, fmt.Errorf("account %s not found", acct.ID)
	}

	acct.CreatedAt = original.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	acct.Metadata = cloneMap(acct.Metadata)

	s.accounts[acct.ID] = acct
	return cloneAccount(acct), nil
}

func (s *Store) GetAccount(_ context.Context, id string) (account.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acct, ok := s.accounts[id]
	if !ok {
		return account.Account{}, fmt.Errorf("account %s not found", id)
	}
	return cloneAccount(acct), nil
}

func (s *Store) ListAccounts(_ context.Context) ([]account.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]account.Account, 0, len(s.accounts))
	for _, acct := range s.accounts {
		result = append(result, cloneAccount(acct))
	}
	return result, nil
}

func (s *Store) DeleteAccount(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.accounts[id]; !ok {
		return fmt.Errorf("account %s not found", id)
	}
	delete(s.accounts, id)
	return nil
}

// FunctionStore implementation ------------------------------------------------

func (s *Store) CreateFunction(_ context.Context, def function.Definition) (function.Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if def.ID == "" {
		def.ID = s.nextIDLocked()
	} else if _, exists := s.functions[def.ID]; exists {
		return function.Definition{}, fmt.Errorf("function %s already exists", def.ID)
	}

	now := time.Now().UTC()
	def.CreatedAt = now
	def.UpdatedAt = now
	def.Secrets = append([]string(nil), def.Secrets...)

	s.functions[def.ID] = def
	return cloneFunction(def), nil
}

func (s *Store) UpdateFunction(_ context.Context, def function.Definition) (function.Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.functions[def.ID]
	if !ok {
		return function.Definition{}, fmt.Errorf("function %s not found", def.ID)
	}

	def.CreatedAt = original.CreatedAt
	def.UpdatedAt = time.Now().UTC()
	def.Secrets = append([]string(nil), def.Secrets...)

	s.functions[def.ID] = def
	return cloneFunction(def), nil
}

func (s *Store) GetFunction(_ context.Context, id string) (function.Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, ok := s.functions[id]
	if !ok {
		return function.Definition{}, fmt.Errorf("function %s not found", id)
	}
	return cloneFunction(def), nil
}

func (s *Store) ListFunctions(_ context.Context, accountID string) ([]function.Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]function.Definition, 0)
	for _, def := range s.functions {
		if accountID == "" || def.AccountID == accountID {
			result = append(result, cloneFunction(def))
		}
	}
	return result, nil
}

// TriggerStore implementation -------------------------------------------------

func (s *Store) CreateTrigger(_ context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if trg.ID == "" {
		trg.ID = s.nextIDLocked()
	} else if _, exists := s.triggers[trg.ID]; exists {
		return trigger.Trigger{}, fmt.Errorf("trigger %s already exists", trg.ID)
	}

	now := time.Now().UTC()
	trg.CreatedAt = now
	trg.UpdatedAt = now

	trg.Config = cloneMap(trg.Config)
	s.triggers[trg.ID] = trg
	return cloneTrigger(trg), nil
}

func (s *Store) UpdateTrigger(_ context.Context, trg trigger.Trigger) (trigger.Trigger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.triggers[trg.ID]
	if !ok {
		return trigger.Trigger{}, fmt.Errorf("trigger %s not found", trg.ID)
	}

	trg.CreatedAt = original.CreatedAt
	trg.UpdatedAt = time.Now().UTC()
	trg.Config = cloneMap(trg.Config)

	s.triggers[trg.ID] = trg
	return cloneTrigger(trg), nil
}

func (s *Store) GetTrigger(_ context.Context, id string) (trigger.Trigger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trg, ok := s.triggers[id]
	if !ok {
		return trigger.Trigger{}, fmt.Errorf("trigger %s not found", id)
	}
	return cloneTrigger(trg), nil
}

func (s *Store) ListTriggers(_ context.Context, accountID string) ([]trigger.Trigger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]trigger.Trigger, 0)
	for _, trg := range s.triggers {
		if accountID == "" || trg.AccountID == accountID {
			result = append(result, cloneTrigger(trg))
		}
	}
	return result, nil
}

// Helpers --------------------------------------------------------------------

func cloneMap(src map[string]string) map[string]string {
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
	acct.Metadata = cloneMap(acct.Metadata)
	return acct
}

func cloneFunction(def function.Definition) function.Definition {
	def.Secrets = append([]string(nil), def.Secrets...)
	return def
}

func cloneTrigger(trg trigger.Trigger) trigger.Trigger {
	if trg.Config != nil {
		copyCfg := make(map[string]string, len(trg.Config))
		for k, v := range trg.Config {
			copyCfg[k] = v
		}
		trg.Config = copyCfg
	}
	return trg
}

func cloneDataSource(src oracle.DataSource) oracle.DataSource {
	src.Headers = cloneMap(src.Headers)
	return src
}

// GasBankStore implementation -------------------------------------------------

func (s *Store) CreateGasAccount(_ context.Context, acct gasbank.Account) (gasbank.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if acct.ID == "" {
		acct.ID = s.nextIDLocked()
	} else if _, exists := s.gasAccounts[acct.ID]; exists {
		return gasbank.Account{}, fmt.Errorf("gas account %s already exists", acct.ID)
	}

	acct.WalletAddress = strings.TrimSpace(acct.WalletAddress)
	walletKey := strings.ToLower(acct.WalletAddress)
	if walletKey != "" {
		if existing, exists := s.gasAccountsByWallet[walletKey]; exists {
			return gasbank.Account{}, fmt.Errorf("wallet %s already assigned to gas account %s", acct.WalletAddress, existing)
		}
	}

	acct.CreatedAt = time.Now().UTC()
	acct.UpdatedAt = acct.CreatedAt

	s.gasAccounts[acct.ID] = acct
	if walletKey != "" {
		s.gasAccountsByWallet[walletKey] = acct.ID
	}
	return acct, nil
}

func (s *Store) UpdateGasAccount(_ context.Context, acct gasbank.Account) (gasbank.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.gasAccounts[acct.ID]
	if !ok {
		return gasbank.Account{}, fmt.Errorf("gas account %s not found", acct.ID)
	}

	acct.WalletAddress = strings.TrimSpace(acct.WalletAddress)
	oldKey := strings.ToLower(strings.TrimSpace(original.WalletAddress))
	newKey := strings.ToLower(acct.WalletAddress)
	if newKey != "" {
		if existing, exists := s.gasAccountsByWallet[newKey]; exists && existing != acct.ID {
			return gasbank.Account{}, fmt.Errorf("wallet %s already assigned to gas account %s", acct.WalletAddress, existing)
		}
	}

	acct.CreatedAt = original.CreatedAt
	acct.UpdatedAt = time.Now().UTC()

	s.gasAccounts[acct.ID] = acct
	if oldKey != "" && oldKey != newKey {
		delete(s.gasAccountsByWallet, oldKey)
	}
	if newKey != "" {
		s.gasAccountsByWallet[newKey] = acct.ID
	} else if oldKey != "" {
		delete(s.gasAccountsByWallet, oldKey)
	}
	return acct, nil
}

func (s *Store) GetGasAccount(_ context.Context, id string) (gasbank.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acct, ok := s.gasAccounts[id]
	if !ok {
		return gasbank.Account{}, fmt.Errorf("gas account %s not found", id)
	}
	return acct, nil
}

func (s *Store) GetGasAccountByWallet(_ context.Context, wallet string) (gasbank.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id, ok := s.gasAccountsByWallet[strings.ToLower(wallet)]; ok {
		return s.gasAccounts[id], nil
	}
	return gasbank.Account{}, fmt.Errorf("gas account for wallet %s not found", wallet)
}

func (s *Store) ListGasAccounts(_ context.Context, accountID string) ([]gasbank.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]gasbank.Account, 0)
	for _, acct := range s.gasAccounts {
		if accountID == "" || acct.AccountID == accountID {
			result = append(result, acct)
		}
	}
	return result, nil
}

func (s *Store) CreateGasTransaction(_ context.Context, tx gasbank.Transaction) (gasbank.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tx.ID == "" {
		tx.ID = s.nextIDLocked()
	}
	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = tx.CreatedAt

	s.gasTransactions[tx.AccountID] = append(s.gasTransactions[tx.AccountID], tx)
	s.gasTransactionsByID[tx.ID] = tx
	return tx, nil
}

func (s *Store) ListGasTransactions(_ context.Context, gasAccountID string) ([]gasbank.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]gasbank.Transaction(nil), s.gasTransactions[gasAccountID]...), nil
}

func (s *Store) UpdateGasTransaction(_ context.Context, tx gasbank.Transaction) (gasbank.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.gasTransactionsByID[tx.ID]
	if !ok {
		return gasbank.Transaction{}, fmt.Errorf("transaction %s not found", tx.ID)
	}

	tx.CreatedAt = original.CreatedAt
	tx.UpdatedAt = time.Now().UTC()
	s.gasTransactionsByID[tx.ID] = tx
	entries := s.gasTransactions[tx.AccountID]
	for i := range entries {
		if entries[i].ID == tx.ID {
			entries[i] = tx
			s.gasTransactions[tx.AccountID] = entries
			break
		}
	}

	return tx, nil
}

func (s *Store) GetGasTransaction(_ context.Context, id string) (gasbank.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.gasTransactionsByID[id]
	if !ok {
		return gasbank.Transaction{}, fmt.Errorf("transaction %s not found", id)
	}
	return tx, nil
}

func (s *Store) ListPendingWithdrawals(_ context.Context) ([]gasbank.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]gasbank.Transaction, 0)
	for _, entries := range s.gasTransactions {
		for _, tx := range entries {
			if tx.Type == gasbank.TransactionWithdrawal && tx.Status == gasbank.StatusPending {
				result = append(result, tx)
			}
		}
	}
	return result, nil
}

// AutomationStore implementation --------------------------------------------

func (s *Store) CreateAutomationJob(_ context.Context, job automation.Job) (automation.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		job.ID = s.nextIDLocked()
	} else if _, exists := s.automationJobs[job.ID]; exists {
		return automation.Job{}, fmt.Errorf("automation job %s already exists", job.ID)
	}

	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now

	s.automationJobs[job.ID] = job
	return job, nil
}

func (s *Store) UpdateAutomationJob(_ context.Context, job automation.Job) (automation.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.automationJobs[job.ID]
	if !ok {
		return automation.Job{}, fmt.Errorf("automation job %s not found", job.ID)
	}

	job.CreatedAt = original.CreatedAt
	job.UpdatedAt = time.Now().UTC()

	s.automationJobs[job.ID] = job
	return job, nil
}

func (s *Store) GetAutomationJob(_ context.Context, id string) (automation.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.automationJobs[id]
	if !ok {
		return automation.Job{}, fmt.Errorf("automation job %s not found", id)
	}
	return job, nil
}

func (s *Store) ListAutomationJobs(_ context.Context, accountID string) ([]automation.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]automation.Job, 0)
	for _, job := range s.automationJobs {
		if accountID == "" || job.AccountID == accountID {
			result = append(result, job)
		}
	}
	return result, nil
}

// PriceFeedStore implementation ---------------------------------------------

func (s *Store) CreatePriceFeed(_ context.Context, feed pricefeed.Feed) (pricefeed.Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if feed.ID == "" {
		feed.ID = s.nextIDLocked()
	} else if _, exists := s.priceFeeds[feed.ID]; exists {
		return pricefeed.Feed{}, fmt.Errorf("price feed %s already exists", feed.ID)
	}

	now := time.Now().UTC()
	feed.CreatedAt = now
	feed.UpdatedAt = now

	s.priceFeeds[feed.ID] = feed
	return feed, nil
}

func (s *Store) UpdatePriceFeed(_ context.Context, feed pricefeed.Feed) (pricefeed.Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.priceFeeds[feed.ID]
	if !ok {
		return pricefeed.Feed{}, fmt.Errorf("price feed %s not found", feed.ID)
	}

	feed.CreatedAt = original.CreatedAt
	feed.UpdatedAt = time.Now().UTC()

	s.priceFeeds[feed.ID] = feed
	return feed, nil
}

func (s *Store) GetPriceFeed(_ context.Context, id string) (pricefeed.Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	feed, ok := s.priceFeeds[id]
	if !ok {
		return pricefeed.Feed{}, fmt.Errorf("price feed %s not found", id)
	}
	return feed, nil
}

func (s *Store) ListPriceFeeds(_ context.Context, accountID string) ([]pricefeed.Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]pricefeed.Feed, 0)
	for _, feed := range s.priceFeeds {
		if accountID == "" || feed.AccountID == accountID {
			result = append(result, feed)
		}
	}
	return result, nil
}

func (s *Store) CreatePriceSnapshot(_ context.Context, snap pricefeed.Snapshot) (pricefeed.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snap.ID == "" {
		snap.ID = s.nextIDLocked()
	} else {
		for _, existing := range s.priceSnapshots[snap.FeedID] {
			if existing.ID == snap.ID {
				return pricefeed.Snapshot{}, fmt.Errorf("snapshot %s already exists for feed %s", snap.ID, snap.FeedID)
			}
		}
	}

	now := time.Now().UTC()
	snap.CreatedAt = now
	if snap.CollectedAt.IsZero() {
		snap.CollectedAt = now
	}

	s.priceSnapshots[snap.FeedID] = append(s.priceSnapshots[snap.FeedID], snap)
	return snap, nil
}

func (s *Store) ListPriceSnapshots(_ context.Context, feedID string) ([]pricefeed.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]pricefeed.Snapshot(nil), s.priceSnapshots[feedID]...), nil
}

// OracleStore implementation -------------------------------------------------

func (s *Store) CreateDataSource(_ context.Context, src oracle.DataSource) (oracle.DataSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if src.ID == "" {
		src.ID = s.nextIDLocked()
	} else if _, exists := s.oracleSources[src.ID]; exists {
		return oracle.DataSource{}, fmt.Errorf("oracle data source %s already exists", src.ID)
	}

	now := time.Now().UTC()
	src.CreatedAt = now
	src.UpdatedAt = now
	src.Headers = cloneMap(src.Headers)

	s.oracleSources[src.ID] = src
	return cloneDataSource(src), nil
}

func (s *Store) UpdateDataSource(_ context.Context, src oracle.DataSource) (oracle.DataSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.oracleSources[src.ID]
	if !ok {
		return oracle.DataSource{}, fmt.Errorf("oracle data source %s not found", src.ID)
	}

	src.CreatedAt = original.CreatedAt
	src.UpdatedAt = time.Now().UTC()
	src.Headers = cloneMap(src.Headers)

	s.oracleSources[src.ID] = src
	return cloneDataSource(src), nil
}

func (s *Store) GetDataSource(_ context.Context, id string) (oracle.DataSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src, ok := s.oracleSources[id]
	if !ok {
		return oracle.DataSource{}, fmt.Errorf("oracle data source %s not found", id)
	}
	return cloneDataSource(src), nil
}

func (s *Store) ListDataSources(_ context.Context, accountID string) ([]oracle.DataSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]oracle.DataSource, 0)
	for _, src := range s.oracleSources {
		if accountID == "" || src.AccountID == accountID {
			result = append(result, cloneDataSource(src))
		}
	}
	return result, nil
}

func (s *Store) CreateRequest(_ context.Context, req oracle.Request) (oracle.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.ID == "" {
		req.ID = s.nextIDLocked()
	} else if _, exists := s.oracleRequests[req.ID]; exists {
		return oracle.Request{}, fmt.Errorf("oracle request %s already exists", req.ID)
	}

	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now

	s.oracleRequests[req.ID] = req
	return req, nil
}

func (s *Store) UpdateRequest(_ context.Context, req oracle.Request) (oracle.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.oracleRequests[req.ID]
	if !ok {
		return oracle.Request{}, fmt.Errorf("oracle request %s not found", req.ID)
	}

	req.CreatedAt = original.CreatedAt
	if req.CompletedAt.IsZero() {
		req.CompletedAt = original.CompletedAt
	}
	req.UpdatedAt = time.Now().UTC()

	s.oracleRequests[req.ID] = req
	return req, nil
}

func (s *Store) GetRequest(_ context.Context, id string) (oracle.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.oracleRequests[id]
	if !ok {
		return oracle.Request{}, fmt.Errorf("oracle request %s not found", id)
	}
	return req, nil
}

func (s *Store) ListRequests(_ context.Context, accountID string) ([]oracle.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]oracle.Request, 0)
	for _, req := range s.oracleRequests {
		if accountID == "" || req.AccountID == accountID {
			result = append(result, req)
		}
	}
	return result, nil
}

func (s *Store) ListPendingRequests(_ context.Context) ([]oracle.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]oracle.Request, 0)
	for _, req := range s.oracleRequests {
		if req.Status == oracle.StatusPending || req.Status == oracle.StatusRunning {
			result = append(result, req)
		}
	}
	return result, nil
}
