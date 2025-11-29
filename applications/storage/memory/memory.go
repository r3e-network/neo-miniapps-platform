package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/applications/storage"
	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/domain/automation"
	"github.com/R3E-Network/service_layer/domain/ccip"
	"github.com/R3E-Network/service_layer/domain/confidential"
	"github.com/R3E-Network/service_layer/domain/cre"
	"github.com/R3E-Network/service_layer/domain/datafeeds"
	"github.com/R3E-Network/service_layer/domain/datalink"
	"github.com/R3E-Network/service_layer/domain/datastreams"
	"github.com/R3E-Network/service_layer/domain/dta"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/domain/gasbank"
	"github.com/R3E-Network/service_layer/domain/oracle"
	"github.com/R3E-Network/service_layer/domain/secret"
	"github.com/R3E-Network/service_layer/domain/vrf"
)

// Store is an in-memory implementation of the storage interfaces. It is safe
// for concurrent use and is primarily intended for tests and local development.
type Store struct {
	mu                    sync.RWMutex
	nextID                int64
	accounts              map[string]account.Account
	functions             map[string]function.Definition
	functionExecutions    map[string]function.Execution
	gasAccounts           map[string]gasbank.Account
	gasAccountsByWallet   map[string]string
	gasTransactions       map[string][]gasbank.Transaction
	gasTransactionsByID   map[string]gasbank.Transaction
	gasApprovals          map[string]map[string]gasbank.WithdrawalApproval
	gasSchedules          map[string]gasbank.WithdrawalSchedule
	gasSettlementAttempts map[string][]gasbank.SettlementAttempt
	gasDeadLetters        map[string]gasbank.DeadLetter
	automationJobs        map[string]automation.Job
	oracleSources         map[string]oracle.DataSource
	oracleRequests        map[string]oracle.Request
	secrets               map[string]secret.Secret
	playbooks             map[string]cre.Playbook
	runs                  map[string]cre.Run
	executors             map[string]cre.Executor
	ccipLanes             map[string]ccip.Lane
	ccipMessages          map[string]ccip.Message
	dataFeeds             map[string]datafeeds.Feed
	dataFeedUpdates       map[string][]datafeeds.Update
	vrfKeys               map[string]vrf.Key
	vrfRequests           map[string]vrf.Request
	dataStreams           map[string]datastreams.Stream
	dataStreamFrames      map[string][]datastreams.Frame
	dataLinkChannels      map[string]datalink.Channel
	dataLinkDeliveries    map[string]datalink.Delivery
	dtaProducts           map[string]dta.Product
	dtaOrders             map[string]dta.Order
	confEnclaves          map[string]confidential.Enclave
	confSealedKeys        map[string][]confidential.SealedKey
	confAttestations      map[string][]confidential.Attestation
	workspaceWallets      map[string]account.WorkspaceWallet
	workspaceWalletsByWS  map[string][]string
}

var _ storage.AccountStore = (*Store)(nil)
var _ storage.FunctionStore = (*Store)(nil)
var _ storage.GasBankStore = (*Store)(nil)
var _ storage.AutomationStore = (*Store)(nil)
var _ storage.OracleStore = (*Store)(nil)
var _ storage.SecretStore = (*Store)(nil)
var _ storage.CREStore = (*Store)(nil)
var _ storage.CCIPStore = (*Store)(nil)
var _ storage.VRFStore = (*Store)(nil)
var _ storage.WorkspaceWalletStore = (*Store)(nil)
var _ storage.DataFeedStore = (*Store)(nil)
var _ storage.VRFStore = (*Store)(nil)
var _ storage.DataStreamStore = (*Store)(nil)
var _ storage.DataLinkStore = (*Store)(nil)
var _ storage.DTAStore = (*Store)(nil)
var _ storage.ConfidentialStore = (*Store)(nil)

// New creates an empty store.
func New() *Store {
	return &Store{
		nextID:                1,
		accounts:              make(map[string]account.Account),
		functions:             make(map[string]function.Definition),
		functionExecutions:    make(map[string]function.Execution),
		gasAccounts:           make(map[string]gasbank.Account),
		gasAccountsByWallet:   make(map[string]string),
		gasTransactions:       make(map[string][]gasbank.Transaction),
		gasTransactionsByID:   make(map[string]gasbank.Transaction),
		gasApprovals:          make(map[string]map[string]gasbank.WithdrawalApproval),
		gasSchedules:          make(map[string]gasbank.WithdrawalSchedule),
		gasSettlementAttempts: make(map[string][]gasbank.SettlementAttempt),
		gasDeadLetters:        make(map[string]gasbank.DeadLetter),
		automationJobs:        make(map[string]automation.Job),
		oracleSources:         make(map[string]oracle.DataSource),
		oracleRequests:        make(map[string]oracle.Request),
		secrets:               make(map[string]secret.Secret),
		playbooks:             make(map[string]cre.Playbook),
		runs:                  make(map[string]cre.Run),
		executors:             make(map[string]cre.Executor),
		ccipLanes:             make(map[string]ccip.Lane),
		ccipMessages:          make(map[string]ccip.Message),
		dataFeeds:             make(map[string]datafeeds.Feed),
		dataFeedUpdates:       make(map[string][]datafeeds.Update),
		vrfKeys:               make(map[string]vrf.Key),
		vrfRequests:           make(map[string]vrf.Request),
		workspaceWallets:      make(map[string]account.WorkspaceWallet),
		workspaceWalletsByWS:  make(map[string][]string),
		dataStreams:           make(map[string]datastreams.Stream),
		dataStreamFrames:      make(map[string][]datastreams.Frame),
		dataLinkChannels:      make(map[string]datalink.Channel),
		dataLinkDeliveries:    make(map[string]datalink.Delivery),
		dtaProducts:           make(map[string]dta.Product),
		dtaOrders:             make(map[string]dta.Order),
		confEnclaves:          make(map[string]confidential.Enclave),
		confSealedKeys:        make(map[string][]confidential.SealedKey),
		confAttestations:      make(map[string][]confidential.Attestation),
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

func (s *Store) CreateExecution(_ context.Context, exec function.Execution) (function.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exec.ID == "" {
		exec.ID = s.nextIDLocked()
	} else if _, exists := s.functionExecutions[exec.ID]; exists {
		return function.Execution{}, fmt.Errorf("function execution %s already exists", exec.ID)
	}

	exec.StartedAt = exec.StartedAt.UTC()
	exec.CompletedAt = exec.CompletedAt.UTC()
	if exec.Input == nil {
		exec.Input = map[string]any{}
	}
	if exec.Output == nil {
		exec.Output = map[string]any{}
	}
	exec.Logs = append([]string(nil), exec.Logs...)
	if exec.Actions == nil {
		exec.Actions = []function.ActionResult{}
	} else {
		exec.Actions = cloneActionResults(exec.Actions)
	}

	s.functionExecutions[exec.ID] = cloneExecution(exec)
	return cloneExecution(exec), nil
}

func (s *Store) GetExecution(_ context.Context, id string) (function.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exec, ok := s.functionExecutions[id]
	if !ok {
		return function.Execution{}, fmt.Errorf("function execution %s not found", id)
	}
	return cloneExecution(exec), nil
}

func (s *Store) ListFunctionExecutions(_ context.Context, functionID string, limit int) ([]function.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]function.Execution, 0)
	for _, exec := range s.functionExecutions {
		if exec.FunctionID == functionID {
			result = append(result, cloneExecution(exec))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
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

func cloneBoolMap(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneGasAccount(acct gasbank.Account) gasbank.Account {
	acct.Flags = cloneBoolMap(acct.Flags)
	acct.Metadata = cloneMap(acct.Metadata)
	return acct
}

func cloneApprovalPolicy(policy gasbank.ApprovalPolicy) gasbank.ApprovalPolicy {
	policy.Approvers = append([]string(nil), policy.Approvers...)
	return policy
}

func cloneWithdrawalApprovals(items []gasbank.WithdrawalApproval) []gasbank.WithdrawalApproval {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]gasbank.WithdrawalApproval, len(items))
	copy(cloned, items)
	return cloned
}

func cloneWithdrawalApproval(approval gasbank.WithdrawalApproval) gasbank.WithdrawalApproval {
	return approval
}

func cloneWithdrawalSchedule(schedule gasbank.WithdrawalSchedule) gasbank.WithdrawalSchedule {
	return schedule
}

func cloneSettlementAttempt(attempt gasbank.SettlementAttempt) gasbank.SettlementAttempt {
	return attempt
}

func cloneDeadLetter(entry gasbank.DeadLetter) gasbank.DeadLetter {
	return entry
}

func cloneGasTransaction(tx gasbank.Transaction) gasbank.Transaction {
	tx.ApprovalPolicy = cloneApprovalPolicy(tx.ApprovalPolicy)
	tx.Approvals = cloneWithdrawalApprovals(tx.Approvals)
	tx.Metadata = cloneMap(tx.Metadata)
	return tx
}

func (s *Store) cloneAndHydrateTransactionLocked(tx gasbank.Transaction) gasbank.Transaction {
	cloned := cloneGasTransaction(tx)
	if approvals := s.gasApprovals[tx.ID]; len(approvals) > 0 {
		list := make([]gasbank.WithdrawalApproval, 0, len(approvals))
		for _, approval := range approvals {
			list = append(list, cloneWithdrawalApproval(approval))
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].UpdatedAt.After(list[j].UpdatedAt)
		})
		cloned.Approvals = list
	} else {
		cloned.Approvals = nil
	}
	return cloned
}

func cloneAccount(acct account.Account) account.Account {
	acct.Metadata = cloneMap(acct.Metadata)
	return acct
}

func cloneFunction(def function.Definition) function.Definition {
	def.Secrets = append([]string(nil), def.Secrets...)
	return def
}

func cloneExecution(exec function.Execution) function.Execution {
	exec.Input = cloneAnyMap(exec.Input)
	exec.Output = cloneAnyMap(exec.Output)
	if exec.Logs != nil {
		exec.Logs = append([]string(nil), exec.Logs...)
	}
	if exec.Actions != nil {
		exec.Actions = cloneActionResults(exec.Actions)
	}
	return exec
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneActionResults(actions []function.ActionResult) []function.ActionResult {
	if len(actions) == 0 {
		return nil
	}
	copied := make([]function.ActionResult, len(actions))
	for i, act := range actions {
		copied[i] = cloneActionResult(act)
	}
	return copied
}

func cloneActionResult(action function.ActionResult) function.ActionResult {
	action.Params = cloneAnyMap(action.Params)
	action.Result = cloneAnyMap(action.Result)
	action.Meta = cloneAnyMap(action.Meta)
	return action
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

	acct.Flags = cloneBoolMap(acct.Flags)
	acct.Metadata = cloneMap(acct.Metadata)
	acct.CreatedAt = time.Now().UTC()
	acct.UpdatedAt = acct.CreatedAt

	s.gasAccounts[acct.ID] = acct
	if walletKey != "" {
		s.gasAccountsByWallet[walletKey] = acct.ID
	}
	return cloneGasAccount(acct), nil
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
	acct.Flags = cloneBoolMap(acct.Flags)
	acct.Metadata = cloneMap(acct.Metadata)

	s.gasAccounts[acct.ID] = acct
	if oldKey != "" && oldKey != newKey {
		delete(s.gasAccountsByWallet, oldKey)
	}
	if newKey != "" {
		s.gasAccountsByWallet[newKey] = acct.ID
	} else if oldKey != "" {
		delete(s.gasAccountsByWallet, oldKey)
	}
	return cloneGasAccount(acct), nil
}

func (s *Store) GetGasAccount(_ context.Context, id string) (gasbank.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acct, ok := s.gasAccounts[id]
	if !ok {
		return gasbank.Account{}, fmt.Errorf("gas account %s not found", id)
	}
	return cloneGasAccount(acct), nil
}

func (s *Store) GetGasAccountByWallet(_ context.Context, wallet string) (gasbank.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id, ok := s.gasAccountsByWallet[strings.ToLower(wallet)]; ok {
		return cloneGasAccount(s.gasAccounts[id]), nil
	}
	return gasbank.Account{}, fmt.Errorf("gas account for wallet %s not found", wallet)
}

func (s *Store) ListGasAccounts(_ context.Context, accountID string) ([]gasbank.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]gasbank.Account, 0)
	for _, acct := range s.gasAccounts {
		if accountID == "" || acct.AccountID == accountID {
			result = append(result, cloneGasAccount(acct))
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
	tx.ApprovalPolicy = cloneApprovalPolicy(tx.ApprovalPolicy)
	tx.Approvals = cloneWithdrawalApprovals(tx.Approvals)
	tx.Metadata = cloneMap(tx.Metadata)
	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = tx.CreatedAt

	stored := cloneGasTransaction(tx)
	s.gasTransactions[tx.AccountID] = append(s.gasTransactions[tx.AccountID], stored)
	s.gasTransactionsByID[tx.ID] = stored
	return s.cloneAndHydrateTransactionLocked(stored), nil
}

func (s *Store) ListGasTransactions(_ context.Context, gasAccountID string, limit int) ([]gasbank.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.gasTransactions[gasAccountID]
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	result := make([]gasbank.Transaction, 0, len(entries))
	for _, tx := range entries {
		result = append(result, s.cloneAndHydrateTransactionLocked(tx))
	}
	return result, nil
}

func (s *Store) UpdateGasTransaction(_ context.Context, tx gasbank.Transaction) (gasbank.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original, ok := s.gasTransactionsByID[tx.ID]
	if !ok {
		return gasbank.Transaction{}, fmt.Errorf("transaction %s not found", tx.ID)
	}

	tx.ApprovalPolicy = cloneApprovalPolicy(tx.ApprovalPolicy)
	tx.Approvals = cloneWithdrawalApprovals(tx.Approvals)
	tx.Metadata = cloneMap(tx.Metadata)
	tx.CreatedAt = original.CreatedAt
	tx.UpdatedAt = time.Now().UTC()
	stored := cloneGasTransaction(tx)
	s.gasTransactionsByID[tx.ID] = stored
	entries := s.gasTransactions[tx.AccountID]
	for i := range entries {
		if entries[i].ID == tx.ID {
			entries[i] = stored
			s.gasTransactions[tx.AccountID] = entries
			break
		}
	}

	return s.cloneAndHydrateTransactionLocked(stored), nil
}

func (s *Store) GetGasTransaction(_ context.Context, id string) (gasbank.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.gasTransactionsByID[id]
	if !ok {
		return gasbank.Transaction{}, fmt.Errorf("transaction %s not found", id)
	}
	return s.cloneAndHydrateTransactionLocked(tx), nil
}

func (s *Store) ListPendingWithdrawals(_ context.Context) ([]gasbank.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]gasbank.Transaction, 0)
	for _, entries := range s.gasTransactions {
		for _, tx := range entries {
			if tx.Type == gasbank.TransactionWithdrawal && tx.Status == gasbank.StatusPending {
				result = append(result, s.cloneAndHydrateTransactionLocked(tx))
			}
		}
	}
	return result, nil
}

func (s *Store) UpsertWithdrawalApproval(_ context.Context, approval gasbank.WithdrawalApproval) (gasbank.WithdrawalApproval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if approval.TransactionID == "" || strings.TrimSpace(approval.Approver) == "" {
		return gasbank.WithdrawalApproval{}, fmt.Errorf("transaction_id and approver required")
	}
	now := time.Now().UTC()
	if approval.CreatedAt.IsZero() {
		approval.CreatedAt = now
	}
	approval.UpdatedAt = now
	approvals := s.gasApprovals[approval.TransactionID]
	if approvals == nil {
		approvals = make(map[string]gasbank.WithdrawalApproval)
	}
	if existing, ok := approvals[approval.Approver]; ok && !existing.CreatedAt.IsZero() {
		approval.CreatedAt = existing.CreatedAt
	}
	approvals[approval.Approver] = cloneWithdrawalApproval(approval)
	s.gasApprovals[approval.TransactionID] = approvals
	return cloneWithdrawalApproval(approval), nil
}

func (s *Store) ListWithdrawalApprovals(_ context.Context, transactionID string) ([]gasbank.WithdrawalApproval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	approvals := s.gasApprovals[transactionID]
	if len(approvals) == 0 {
		return nil, nil
	}
	result := make([]gasbank.WithdrawalApproval, 0, len(approvals))
	for _, approval := range approvals {
		result = append(result, cloneWithdrawalApproval(approval))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result, nil
}

func (s *Store) SaveWithdrawalSchedule(_ context.Context, schedule gasbank.WithdrawalSchedule) (gasbank.WithdrawalSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if schedule.TransactionID == "" {
		return gasbank.WithdrawalSchedule{}, fmt.Errorf("transaction_id required")
	}
	now := time.Now().UTC()
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}
	schedule.UpdatedAt = now
	s.gasSchedules[schedule.TransactionID] = cloneWithdrawalSchedule(schedule)
	return cloneWithdrawalSchedule(schedule), nil
}

func (s *Store) GetWithdrawalSchedule(_ context.Context, transactionID string) (gasbank.WithdrawalSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.gasSchedules[transactionID]
	if !ok {
		return gasbank.WithdrawalSchedule{}, fmt.Errorf("withdrawal schedule for %s not found", transactionID)
	}
	return cloneWithdrawalSchedule(schedule), nil
}

func (s *Store) DeleteWithdrawalSchedule(_ context.Context, transactionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.gasSchedules, transactionID)
	return nil
}

func (s *Store) ListDueWithdrawalSchedules(_ context.Context, before time.Time, limit int) ([]gasbank.WithdrawalSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]gasbank.WithdrawalSchedule, 0)
	for _, schedule := range s.gasSchedules {
		if !schedule.ScheduleAt.IsZero() && !schedule.ScheduleAt.After(before) {
			result = append(result, cloneWithdrawalSchedule(schedule))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ScheduleAt.Before(result[j].ScheduleAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) RecordSettlementAttempt(_ context.Context, attempt gasbank.SettlementAttempt) (gasbank.SettlementAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if attempt.TransactionID == "" {
		return gasbank.SettlementAttempt{}, fmt.Errorf("transaction_id required")
	}
	if attempt.Attempt <= 0 {
		attempt.Attempt = len(s.gasSettlementAttempts[attempt.TransactionID]) + 1
	}
	now := time.Now().UTC()
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = now
	}
	if attempt.CompletedAt.IsZero() {
		attempt.CompletedAt = now
	}
	if attempt.Latency == 0 && !attempt.CompletedAt.IsZero() {
		attempt.Latency = attempt.CompletedAt.Sub(attempt.StartedAt)
	}
	list := s.gasSettlementAttempts[attempt.TransactionID]
	if attempt.Attempt-1 < len(list) {
		list[attempt.Attempt-1] = cloneSettlementAttempt(attempt)
	} else {
		list = append(list, cloneSettlementAttempt(attempt))
	}
	s.gasSettlementAttempts[attempt.TransactionID] = list
	return cloneSettlementAttempt(attempt), nil
}

func (s *Store) ListSettlementAttempts(_ context.Context, transactionID string, limit int) ([]gasbank.SettlementAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	attempts := s.gasSettlementAttempts[transactionID]
	if len(attempts) == 0 {
		return nil, nil
	}
	result := make([]gasbank.SettlementAttempt, len(attempts))
	for i, attempt := range attempts {
		result[i] = cloneSettlementAttempt(attempt)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Attempt > result[j].Attempt
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) UpsertDeadLetter(_ context.Context, entry gasbank.DeadLetter) (gasbank.DeadLetter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.TransactionID == "" {
		return gasbank.DeadLetter{}, fmt.Errorf("transaction_id required")
	}
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	s.gasDeadLetters[entry.TransactionID] = cloneDeadLetter(entry)
	return cloneDeadLetter(entry), nil
}

func (s *Store) GetDeadLetter(_ context.Context, transactionID string) (gasbank.DeadLetter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.gasDeadLetters[transactionID]
	if !ok {
		return gasbank.DeadLetter{}, fmt.Errorf("dead letter %s not found", transactionID)
	}
	return cloneDeadLetter(entry), nil
}

func (s *Store) ListDeadLetters(_ context.Context, accountID string, limit int) ([]gasbank.DeadLetter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]gasbank.DeadLetter, 0, len(s.gasDeadLetters))
	for _, entry := range s.gasDeadLetters {
		if accountID == "" || entry.AccountID == accountID {
			result = append(result, cloneDeadLetter(entry))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) RemoveDeadLetter(_ context.Context, transactionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.gasDeadLetters, transactionID)
	return nil
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

	req.AccountID = original.AccountID
	req.DataSourceID = original.DataSourceID
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

func (s *Store) ListRequests(_ context.Context, accountID string, limit int, status string) ([]oracle.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	max := limit
	if max <= 0 || max > 500 {
		max = 100
	}

	result := make([]oracle.Request, 0, max)
	for _, req := range s.oracleRequests {
		if accountID != "" && req.AccountID != accountID {
			continue
		}
		if status != "" && !strings.EqualFold(string(req.Status), status) {
			continue
		}
		result = append(result, req)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if len(result) > max {
		result = result[:max]
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

// SecretStore implementation -------------------------------------------------

func (s *Store) CreateSecret(_ context.Context, sec secret.Secret) (secret.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := secretKey(sec.AccountID, sec.Name)
	if _, exists := s.secrets[key]; exists {
		return secret.Secret{}, fmt.Errorf("secret %s already exists for account %s", sec.Name, sec.AccountID)
	}

	if sec.ID == "" {
		sec.ID = s.nextIDLocked()
	}
	now := time.Now().UTC()
	sec.CreatedAt = now
	sec.UpdatedAt = now
	sec.Version = 1

	s.secrets[key] = sec
	return sec, nil
}

func (s *Store) UpdateSecret(_ context.Context, sec secret.Secret) (secret.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := secretKey(sec.AccountID, sec.Name)
	existing, ok := s.secrets[key]
	if !ok {
		return secret.Secret{}, fmt.Errorf("secret %s not found for account %s", sec.Name, sec.AccountID)
	}

	sec.ID = existing.ID
	sec.CreatedAt = existing.CreatedAt
	sec.Version = existing.Version + 1
	sec.UpdatedAt = time.Now().UTC()

	s.secrets[key] = sec
	return sec, nil
}

func (s *Store) GetSecret(_ context.Context, accountID, name string) (secret.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sec, ok := s.secrets[secretKey(accountID, name)]
	if !ok {
		return secret.Secret{}, fmt.Errorf("secret %s not found for account %s", name, accountID)
	}
	return sec, nil
}

func (s *Store) ListSecrets(_ context.Context, accountID string) ([]secret.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]secret.Secret, 0)
	for _, sec := range s.secrets {
		if sec.AccountID == accountID {
			result = append(result, sec)
		}
	}
	return result, nil
}

func (s *Store) DeleteSecret(_ context.Context, accountID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := secretKey(accountID, name)
	if _, ok := s.secrets[key]; !ok {
		return fmt.Errorf("secret %s not found for account %s", name, accountID)
	}
	delete(s.secrets, key)
	return nil
}

func secretKey(accountID, name string) string {
	return accountID + "|" + strings.ToLower(name)
}

// CREStore implementation ----------------------------------------------------

func (s *Store) CreatePlaybook(_ context.Context, pb cre.Playbook) (cre.Playbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pb.ID == "" {
		pb.ID = s.nextIDLocked()
	} else if _, exists := s.playbooks[pb.ID]; exists {
		return cre.Playbook{}, fmt.Errorf("playbook %s already exists", pb.ID)
	}

	now := time.Now().UTC()
	pb.CreatedAt = now
	pb.UpdatedAt = now
	pb.Tags = append([]string(nil), pb.Tags...)
	pb.Metadata = cloneMap(pb.Metadata)
	pb.Steps = cloneSteps(pb.Steps)

	s.playbooks[pb.ID] = pb
	return clonePlaybook(pb), nil
}

func (s *Store) UpdatePlaybook(_ context.Context, pb cre.Playbook) (cre.Playbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.playbooks[pb.ID]
	if !ok {
		return cre.Playbook{}, fmt.Errorf("playbook %s not found", pb.ID)
	}

	pb.CreatedAt = existing.CreatedAt
	pb.UpdatedAt = time.Now().UTC()
	pb.Tags = append([]string(nil), pb.Tags...)
	pb.Metadata = cloneMap(pb.Metadata)
	pb.Steps = cloneSteps(pb.Steps)

	s.playbooks[pb.ID] = pb
	return clonePlaybook(pb), nil
}

func (s *Store) GetPlaybook(_ context.Context, id string) (cre.Playbook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pb, ok := s.playbooks[id]
	if !ok {
		return cre.Playbook{}, fmt.Errorf("playbook %s not found", id)
	}
	return clonePlaybook(pb), nil
}

func (s *Store) ListPlaybooks(_ context.Context, accountID string) ([]cre.Playbook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]cre.Playbook, 0)
	for _, pb := range s.playbooks {
		if pb.AccountID == accountID {
			result = append(result, clonePlaybook(pb))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateRun(_ context.Context, run cre.Run) (cre.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if run.ID == "" {
		run.ID = s.nextIDLocked()
	} else if _, exists := s.runs[run.ID]; exists {
		return cre.Run{}, fmt.Errorf("run %s already exists", run.ID)
	}

	now := time.Now().UTC()
	run.CreatedAt = now
	run.UpdatedAt = now
	run.Parameters = cloneAnyMap(run.Parameters)
	run.Results = cloneStepResults(run.Results)
	run.Metadata = cloneMap(run.Metadata)
	run.Tags = append([]string(nil), run.Tags...)

	s.runs[run.ID] = run
	return cloneRun(run), nil
}

func (s *Store) UpdateRun(_ context.Context, run cre.Run) (cre.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.runs[run.ID]
	if !ok {
		return cre.Run{}, fmt.Errorf("run %s not found", run.ID)
	}

	run.CreatedAt = existing.CreatedAt
	if run.CompletedAt != nil {
		completed := run.CompletedAt.UTC()
		run.CompletedAt = &completed
	}
	run.UpdatedAt = time.Now().UTC()
	run.Parameters = cloneAnyMap(run.Parameters)
	run.Results = cloneStepResults(run.Results)
	run.Metadata = cloneMap(run.Metadata)
	run.Tags = append([]string(nil), run.Tags...)

	s.runs[run.ID] = run
	return cloneRun(run), nil
}

func (s *Store) GetRun(_ context.Context, id string) (cre.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.runs[id]
	if !ok {
		return cre.Run{}, fmt.Errorf("run %s not found", id)
	}
	return cloneRun(run), nil
}

func (s *Store) ListRuns(_ context.Context, accountID string, limit int) ([]cre.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]cre.Run, 0)
	for _, run := range s.runs {
		if run.AccountID == accountID {
			result = append(result, cloneRun(run))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) CreateExecutor(_ context.Context, exec cre.Executor) (cre.Executor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exec.ID == "" {
		exec.ID = s.nextIDLocked()
	} else if _, exists := s.executors[exec.ID]; exists {
		return cre.Executor{}, fmt.Errorf("executor %s already exists", exec.ID)
	}
	now := time.Now().UTC()
	exec.CreatedAt = now
	exec.UpdatedAt = now
	exec.Metadata = cloneMap(exec.Metadata)
	exec.Tags = append([]string(nil), exec.Tags...)

	s.executors[exec.ID] = exec
	return cloneExecutor(exec), nil
}

func (s *Store) UpdateExecutor(_ context.Context, exec cre.Executor) (cre.Executor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.executors[exec.ID]
	if !ok {
		return cre.Executor{}, fmt.Errorf("executor %s not found", exec.ID)
	}

	exec.CreatedAt = existing.CreatedAt
	exec.UpdatedAt = time.Now().UTC()
	exec.Metadata = cloneMap(exec.Metadata)
	exec.Tags = append([]string(nil), exec.Tags...)

	s.executors[exec.ID] = exec
	return cloneExecutor(exec), nil
}

func (s *Store) GetExecutor(_ context.Context, id string) (cre.Executor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exec, ok := s.executors[id]
	if !ok {
		return cre.Executor{}, fmt.Errorf("executor %s not found", id)
	}
	return cloneExecutor(exec), nil
}

func (s *Store) ListExecutors(_ context.Context, accountID string) ([]cre.Executor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]cre.Executor, 0)
	for _, exec := range s.executors {
		if exec.AccountID == accountID {
			result = append(result, cloneExecutor(exec))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func clonePlaybook(pb cre.Playbook) cre.Playbook {
	pb.Tags = append([]string(nil), pb.Tags...)
	pb.Metadata = cloneMap(pb.Metadata)
	pb.Steps = cloneSteps(pb.Steps)
	return pb
}

func cloneSteps(steps []cre.Step) []cre.Step {
	if len(steps) == 0 {
		return nil
	}
	result := make([]cre.Step, len(steps))
	for i, step := range steps {
		result[i] = cre.Step{
			Name:           step.Name,
			Type:           step.Type,
			Config:         cloneAnyMap(step.Config),
			TimeoutSeconds: step.TimeoutSeconds,
			RetryLimit:     step.RetryLimit,
			Metadata:       cloneMap(step.Metadata),
			Tags:           append([]string(nil), step.Tags...),
		}
	}
	return result
}

func cloneRun(run cre.Run) cre.Run {
	run.Parameters = cloneAnyMap(run.Parameters)
	run.Metadata = cloneMap(run.Metadata)
	run.Tags = append([]string(nil), run.Tags...)
	run.ExecutorID = strings.TrimSpace(run.ExecutorID)
	if run.CompletedAt != nil {
		completed := run.CompletedAt.UTC()
		run.CompletedAt = &completed
	}
	run.Results = cloneStepResults(run.Results)
	return run
}

func cloneStepResults(results []cre.StepResult) []cre.StepResult {
	if len(results) == 0 {
		return nil
	}
	cloned := make([]cre.StepResult, len(results))
	for i, res := range results {
		cloned[i] = cre.StepResult{
			RunID:       res.RunID,
			StepIndex:   res.StepIndex,
			Name:        res.Name,
			Type:        res.Type,
			Status:      res.Status,
			Logs:        append([]string(nil), res.Logs...),
			Error:       res.Error,
			StartedAt:   res.StartedAt,
			CompletedAt: res.CompletedAt,
			Metadata:    cloneMap(res.Metadata),
		}
		if res.CompletedAt != nil {
			t := res.CompletedAt.UTC()
			cloned[i].CompletedAt = &t
		}
	}
	return cloned
}

func cloneExecutor(exec cre.Executor) cre.Executor {
	exec.Metadata = cloneMap(exec.Metadata)
	exec.Tags = append([]string(nil), exec.Tags...)
	return exec
}

// CCIPStore implementation ---------------------------------------------------

func (s *Store) CreateLane(_ context.Context, lane ccip.Lane) (ccip.Lane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lane.ID == "" {
		lane.ID = s.nextIDLocked()
	} else if _, exists := s.ccipLanes[lane.ID]; exists {
		return ccip.Lane{}, fmt.Errorf("lane %s already exists", lane.ID)
	}
	now := time.Now().UTC()
	lane.CreatedAt = now
	lane.UpdatedAt = now
	lane.AllowedTokens = append([]string(nil), lane.AllowedTokens...)
	lane.Metadata = cloneMap(lane.Metadata)
	lane.Tags = append([]string(nil), lane.Tags...)
	lane.DeliveryPolicy = cloneAnyMap(lane.DeliveryPolicy)

	s.ccipLanes[lane.ID] = lane
	return cloneLane(lane), nil
}

func (s *Store) UpdateLane(_ context.Context, lane ccip.Lane) (ccip.Lane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.ccipLanes[lane.ID]
	if !ok {
		return ccip.Lane{}, fmt.Errorf("lane %s not found", lane.ID)
	}

	lane.CreatedAt = existing.CreatedAt
	lane.UpdatedAt = time.Now().UTC()
	lane.AllowedTokens = append([]string(nil), lane.AllowedTokens...)
	lane.Metadata = cloneMap(lane.Metadata)
	lane.Tags = append([]string(nil), lane.Tags...)
	lane.DeliveryPolicy = cloneAnyMap(lane.DeliveryPolicy)

	s.ccipLanes[lane.ID] = lane
	return cloneLane(lane), nil
}

func (s *Store) GetLane(_ context.Context, id string) (ccip.Lane, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lane, ok := s.ccipLanes[id]
	if !ok {
		return ccip.Lane{}, fmt.Errorf("lane %s not found", id)
	}
	return cloneLane(lane), nil
}

func (s *Store) ListLanes(_ context.Context, accountID string) ([]ccip.Lane, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ccip.Lane, 0)
	for _, lane := range s.ccipLanes {
		if lane.AccountID == accountID {
			result = append(result, cloneLane(lane))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateMessage(_ context.Context, msg ccip.Message) (ccip.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ID == "" {
		msg.ID = s.nextIDLocked()
	} else if _, exists := s.ccipMessages[msg.ID]; exists {
		return ccip.Message{}, fmt.Errorf("message %s already exists", msg.ID)
	}
	now := time.Now().UTC()
	msg.CreatedAt = now
	msg.UpdatedAt = now
	msg.Payload = cloneAnyMap(msg.Payload)
	msg.Metadata = cloneMap(msg.Metadata)
	msg.Tags = append([]string(nil), msg.Tags...)
	msg.Trace = append([]string(nil), msg.Trace...)
	msg.TokenTransfers = cloneTokenTransfers(msg.TokenTransfers)

	s.ccipMessages[msg.ID] = msg
	return cloneMessage(msg), nil
}

func (s *Store) UpdateMessage(_ context.Context, msg ccip.Message) (ccip.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.ccipMessages[msg.ID]
	if !ok {
		return ccip.Message{}, fmt.Errorf("message %s not found", msg.ID)
	}

	msg.CreatedAt = existing.CreatedAt
	msg.Payload = cloneAnyMap(msg.Payload)
	msg.Metadata = cloneMap(msg.Metadata)
	msg.Tags = append([]string(nil), msg.Tags...)
	msg.Trace = append([]string(nil), msg.Trace...)
	msg.TokenTransfers = cloneTokenTransfers(msg.TokenTransfers)
	if msg.DeliveredAt != nil {
		del := msg.DeliveredAt.UTC()
		msg.DeliveredAt = &del
	}
	msg.UpdatedAt = time.Now().UTC()

	s.ccipMessages[msg.ID] = msg
	return cloneMessage(msg), nil
}

func (s *Store) GetMessage(_ context.Context, id string) (ccip.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msg, ok := s.ccipMessages[id]
	if !ok {
		return ccip.Message{}, fmt.Errorf("message %s not found", id)
	}
	return cloneMessage(msg), nil
}

func (s *Store) ListMessages(_ context.Context, accountID string, limit int) ([]ccip.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ccip.Message, 0)
	for _, msg := range s.ccipMessages {
		if msg.AccountID == accountID {
			result = append(result, cloneMessage(msg))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func cloneLane(lane ccip.Lane) ccip.Lane {
	lane.AllowedTokens = append([]string(nil), lane.AllowedTokens...)
	lane.Tags = append([]string(nil), lane.Tags...)
	lane.Metadata = cloneMap(lane.Metadata)
	lane.DeliveryPolicy = cloneAnyMap(lane.DeliveryPolicy)
	return lane
}

func cloneMessage(msg ccip.Message) ccip.Message {
	msg.Payload = cloneAnyMap(msg.Payload)
	msg.Metadata = cloneMap(msg.Metadata)
	msg.Tags = append([]string(nil), msg.Tags...)
	msg.Trace = append([]string(nil), msg.Trace...)
	msg.TokenTransfers = cloneTokenTransfers(msg.TokenTransfers)
	if msg.DeliveredAt != nil {
		del := msg.DeliveredAt.UTC()
		msg.DeliveredAt = &del
	}
	return msg
}

func cloneTokenTransfers(transfers []ccip.TokenTransfer) []ccip.TokenTransfer {
	if len(transfers) == 0 {
		return nil
	}
	out := make([]ccip.TokenTransfer, len(transfers))
	copy(out, transfers)
	return out
}

// DataFeedStore implementation -----------------------------------------------

func (s *Store) CreateDataFeed(_ context.Context, feed datafeeds.Feed) (datafeeds.Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if feed.ID == "" {
		feed.ID = s.nextIDLocked()
	} else if _, exists := s.dataFeeds[feed.ID]; exists {
		return datafeeds.Feed{}, fmt.Errorf("data feed %s already exists", feed.ID)
	}
	if strings.TrimSpace(feed.Aggregation) == "" {
		feed.Aggregation = "median"
	}
	now := time.Now().UTC()
	feed.CreatedAt = now
	feed.UpdatedAt = now
	feed.Metadata = cloneMap(feed.Metadata)
	feed.Tags = append([]string(nil), feed.Tags...)
	feed.SignerSet = append([]string(nil), feed.SignerSet...)

	s.dataFeeds[feed.ID] = feed
	return cloneDataFeed(feed), nil
}

func (s *Store) UpdateDataFeed(_ context.Context, feed datafeeds.Feed) (datafeeds.Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.dataFeeds[feed.ID]
	if !ok {
		return datafeeds.Feed{}, fmt.Errorf("data feed %s not found", feed.ID)
	}

	if strings.TrimSpace(feed.Aggregation) == "" {
		feed.Aggregation = existing.Aggregation
	}
	feed.CreatedAt = existing.CreatedAt
	feed.UpdatedAt = time.Now().UTC()
	feed.Metadata = cloneMap(feed.Metadata)
	feed.Tags = append([]string(nil), feed.Tags...)
	feed.SignerSet = append([]string(nil), feed.SignerSet...)

	s.dataFeeds[feed.ID] = feed
	return cloneDataFeed(feed), nil
}

func (s *Store) GetDataFeed(_ context.Context, id string) (datafeeds.Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	feed, ok := s.dataFeeds[id]
	if !ok {
		return datafeeds.Feed{}, fmt.Errorf("data feed %s not found", id)
	}
	return cloneDataFeed(feed), nil
}

func (s *Store) ListDataFeeds(_ context.Context, accountID string) ([]datafeeds.Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]datafeeds.Feed, 0)
	for _, feed := range s.dataFeeds {
		if feed.AccountID == accountID {
			result = append(result, cloneDataFeed(feed))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateDataFeedUpdate(_ context.Context, upd datafeeds.Update) (datafeeds.Update, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if upd.ID == "" {
		upd.ID = s.nextIDLocked()
	}
	for _, existing := range s.dataFeedUpdates[upd.FeedID] {
		if existing.ID == upd.ID {
			return datafeeds.Update{}, fmt.Errorf("data feed update %s already exists", upd.ID)
		}
		if existing.RoundID == upd.RoundID && strings.EqualFold(existing.Signer, upd.Signer) {
			return datafeeds.Update{}, fmt.Errorf("data feed update for signer %s round %d already exists", upd.Signer, upd.RoundID)
		}
	}
	now := time.Now().UTC()
	upd.CreatedAt = now
	upd.UpdatedAt = now
	upd.Metadata = cloneMap(upd.Metadata)

	list := s.dataFeedUpdates[upd.FeedID]
	list = append([]datafeeds.Update{upd}, list...)
	s.dataFeedUpdates[upd.FeedID] = list
	return cloneDataFeedUpdate(upd), nil
}

func (s *Store) ListDataFeedUpdates(_ context.Context, feedID string, limit int) ([]datafeeds.Update, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.dataFeedUpdates[feedID]
	if len(list) == 0 {
		return nil, nil
	}
	result := make([]datafeeds.Update, len(list))
	copy(result, list)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return cloneDataFeedUpdates(result), nil
}

func (s *Store) GetLatestDataFeedUpdate(_ context.Context, feedID string) (datafeeds.Update, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.dataFeedUpdates[feedID]
	if len(list) == 0 {
		return datafeeds.Update{}, fmt.Errorf("no updates for feed %s", feedID)
	}
	return cloneDataFeedUpdate(list[0]), nil
}

func (s *Store) ListDataFeedUpdatesByRound(_ context.Context, feedID string, roundID int64) ([]datafeeds.Update, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.dataFeedUpdates[feedID]
	result := make([]datafeeds.Update, 0)
	for i := len(list) - 1; i >= 0; i-- { // preserve chronological order
		if list[i].RoundID == roundID {
			result = append(result, cloneDataFeedUpdate(list[i]))
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	// restore ascending creation order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

func cloneDataFeed(feed datafeeds.Feed) datafeeds.Feed {
	feed.Metadata = cloneMap(feed.Metadata)
	feed.Tags = append([]string(nil), feed.Tags...)
	feed.SignerSet = append([]string(nil), feed.SignerSet...)
	feed.Aggregation = strings.TrimSpace(feed.Aggregation)
	return feed
}

func cloneDataFeedUpdate(upd datafeeds.Update) datafeeds.Update {
	upd.Metadata = cloneMap(upd.Metadata)
	return upd
}

func cloneDataFeedUpdates(list []datafeeds.Update) []datafeeds.Update {
	if len(list) == 0 {
		return nil
	}
	result := make([]datafeeds.Update, len(list))
	for i, upd := range list {
		result[i] = cloneDataFeedUpdate(upd)
	}
	return result
}

// VRFStore implementation ----------------------------------------------------

func (s *Store) CreateVRFKey(_ context.Context, key vrf.Key) (vrf.Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key.ID == "" {
		key.ID = s.nextIDLocked()
	} else if _, exists := s.vrfKeys[key.ID]; exists {
		return vrf.Key{}, fmt.Errorf("vrf key %s already exists", key.ID)
	}
	now := time.Now().UTC()
	key.CreatedAt = now
	key.UpdatedAt = now
	key.Metadata = cloneMap(key.Metadata)

	s.vrfKeys[key.ID] = key
	return cloneVRFKey(key), nil
}

func (s *Store) UpdateVRFKey(_ context.Context, key vrf.Key) (vrf.Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.vrfKeys[key.ID]
	if !ok {
		return vrf.Key{}, fmt.Errorf("vrf key %s not found", key.ID)
	}

	key.CreatedAt = existing.CreatedAt
	key.UpdatedAt = time.Now().UTC()
	key.Metadata = cloneMap(key.Metadata)

	s.vrfKeys[key.ID] = key
	return cloneVRFKey(key), nil
}

func (s *Store) GetVRFKey(_ context.Context, id string) (vrf.Key, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.vrfKeys[id]
	if !ok {
		return vrf.Key{}, fmt.Errorf("vrf key %s not found", id)
	}
	return cloneVRFKey(key), nil
}

func (s *Store) ListVRFKeys(_ context.Context, accountID string) ([]vrf.Key, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]vrf.Key, 0)
	for _, key := range s.vrfKeys {
		if key.AccountID == accountID {
			result = append(result, cloneVRFKey(key))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateVRFRequest(_ context.Context, req vrf.Request) (vrf.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.ID == "" {
		req.ID = s.nextIDLocked()
	} else if _, exists := s.vrfRequests[req.ID]; exists {
		return vrf.Request{}, fmt.Errorf("vrf request %s already exists", req.ID)
	}
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	req.Metadata = cloneMap(req.Metadata)

	s.vrfRequests[req.ID] = req
	return cloneVRFRequest(req), nil
}

func (s *Store) GetVRFRequest(_ context.Context, id string) (vrf.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.vrfRequests[id]
	if !ok {
		return vrf.Request{}, fmt.Errorf("vrf request %s not found", id)
	}
	return cloneVRFRequest(req), nil
}

func (s *Store) ListVRFRequests(_ context.Context, accountID string, limit int) ([]vrf.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]vrf.Request, 0)
	for _, req := range s.vrfRequests {
		if req.AccountID == accountID {
			result = append(result, cloneVRFRequest(req))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// WorkspaceWalletStore implementation ----------------------------------------

func (s *Store) CreateWorkspaceWallet(_ context.Context, wallet account.WorkspaceWallet) (account.WorkspaceWallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if wallet.ID == "" {
		wallet.ID = s.nextIDLocked()
	} else if _, exists := s.workspaceWallets[wallet.ID]; exists {
		return account.WorkspaceWallet{}, fmt.Errorf("workspace wallet %s already exists", wallet.ID)
	}
	if err := account.ValidateWalletAddress(wallet.WalletAddress); err != nil {
		return account.WorkspaceWallet{}, err
	}
	wallet.WalletAddress = account.NormalizeWalletAddress(wallet.WalletAddress)
	now := time.Now().UTC()
	wallet.CreatedAt = now
	wallet.UpdatedAt = now

	s.workspaceWallets[wallet.ID] = wallet
	s.workspaceWalletsByWS[wallet.WorkspaceID] = append(s.workspaceWalletsByWS[wallet.WorkspaceID], wallet.ID)
	return wallet, nil
}

func (s *Store) GetWorkspaceWallet(_ context.Context, id string) (account.WorkspaceWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallet, ok := s.workspaceWallets[id]
	if !ok {
		return account.WorkspaceWallet{}, fmt.Errorf("workspace wallet %s not found", id)
	}
	return wallet, nil
}

func (s *Store) ListWorkspaceWallets(_ context.Context, workspaceID string) ([]account.WorkspaceWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.workspaceWalletsByWS[workspaceID]
	result := make([]account.WorkspaceWallet, 0, len(ids))
	for _, id := range ids {
		result = append(result, s.workspaceWallets[id])
	}
	return result, nil
}

func (s *Store) FindWorkspaceWalletByAddress(_ context.Context, workspaceID, wallet string) (account.WorkspaceWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	target := account.NormalizeWalletAddress(wallet)
	for _, id := range s.workspaceWalletsByWS[workspaceID] {
		w := s.workspaceWallets[id]
		if account.NormalizeWalletAddress(w.WalletAddress) == target {
			return w, nil
		}
	}
	return account.WorkspaceWallet{}, fmt.Errorf("workspace wallet not found")
}

func cloneVRFKey(key vrf.Key) vrf.Key {
	key.Metadata = cloneMap(key.Metadata)
	return key
}

func cloneVRFRequest(req vrf.Request) vrf.Request {
	req.Metadata = cloneMap(req.Metadata)
	return req
}

// DataStreamStore implementation ---------------------------------------------

func (s *Store) CreateStream(_ context.Context, stream datastreams.Stream) (datastreams.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stream.ID == "" {
		stream.ID = s.nextIDLocked()
	} else if _, exists := s.dataStreams[stream.ID]; exists {
		return datastreams.Stream{}, fmt.Errorf("data stream %s already exists", stream.ID)
	}
	now := time.Now().UTC()
	stream.CreatedAt = now
	stream.UpdatedAt = now
	stream.Metadata = cloneMap(stream.Metadata)

	s.dataStreams[stream.ID] = stream
	return cloneStream(stream), nil
}

func (s *Store) UpdateStream(_ context.Context, stream datastreams.Stream) (datastreams.Stream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.dataStreams[stream.ID]
	if !ok {
		return datastreams.Stream{}, fmt.Errorf("data stream %s not found", stream.ID)
	}
	stream.CreatedAt = existing.CreatedAt
	stream.UpdatedAt = time.Now().UTC()
	stream.Metadata = cloneMap(stream.Metadata)

	s.dataStreams[stream.ID] = stream
	return cloneStream(stream), nil
}

func (s *Store) GetStream(_ context.Context, id string) (datastreams.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stream, ok := s.dataStreams[id]
	if !ok {
		return datastreams.Stream{}, fmt.Errorf("data stream %s not found", id)
	}
	return cloneStream(stream), nil
}

func (s *Store) ListStreams(_ context.Context, accountID string) ([]datastreams.Stream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]datastreams.Stream, 0)
	for _, stream := range s.dataStreams {
		if stream.AccountID == accountID {
			result = append(result, cloneStream(stream))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateFrame(_ context.Context, frame datastreams.Frame) (datastreams.Frame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if frame.ID == "" {
		frame.ID = s.nextIDLocked()
	} else {
		for _, existing := range s.dataStreamFrames[frame.StreamID] {
			if existing.ID == frame.ID {
				return datastreams.Frame{}, fmt.Errorf("frame %s already exists", frame.ID)
			}
		}
	}
	frame.CreatedAt = time.Now().UTC()
	frame.Payload = cloneAnyMap(frame.Payload)
	frame.Metadata = cloneMap(frame.Metadata)

	list := s.dataStreamFrames[frame.StreamID]
	list = append([]datastreams.Frame{frame}, list...)
	s.dataStreamFrames[frame.StreamID] = list
	return cloneFrame(frame), nil
}

func (s *Store) ListFrames(_ context.Context, streamID string, limit int) ([]datastreams.Frame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.dataStreamFrames[streamID]
	if len(list) == 0 {
		return nil, nil
	}
	result := make([]datastreams.Frame, len(list))
	copy(result, list)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return cloneFrames(result), nil
}

func (s *Store) GetLatestFrame(_ context.Context, streamID string) (datastreams.Frame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.dataStreamFrames[streamID]
	if len(list) == 0 {
		return datastreams.Frame{}, fmt.Errorf("no frames for stream %s", streamID)
	}
	return cloneFrame(list[0]), nil
}

func cloneStream(stream datastreams.Stream) datastreams.Stream {
	stream.Metadata = cloneMap(stream.Metadata)
	return stream
}

func cloneFrame(frame datastreams.Frame) datastreams.Frame {
	frame.Payload = cloneAnyMap(frame.Payload)
	frame.Metadata = cloneMap(frame.Metadata)
	return frame
}

func cloneFrames(frames []datastreams.Frame) []datastreams.Frame {
	if len(frames) == 0 {
		return nil
	}
	result := make([]datastreams.Frame, len(frames))
	for i, frame := range frames {
		result[i] = cloneFrame(frame)
	}
	return result
}

// DataLinkStore implementation -----------------------------------------------

func (s *Store) CreateChannel(_ context.Context, ch datalink.Channel) (datalink.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch.ID == "" {
		ch.ID = s.nextIDLocked()
	} else if _, exists := s.dataLinkChannels[ch.ID]; exists {
		return datalink.Channel{}, fmt.Errorf("datalink channel %s already exists", ch.ID)
	}
	now := time.Now().UTC()
	ch.CreatedAt = now
	ch.UpdatedAt = now
	ch.Metadata = cloneMap(ch.Metadata)

	s.dataLinkChannels[ch.ID] = ch
	return cloneChannel(ch), nil
}

func (s *Store) UpdateChannel(_ context.Context, ch datalink.Channel) (datalink.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.dataLinkChannels[ch.ID]
	if !ok {
		return datalink.Channel{}, fmt.Errorf("datalink channel %s not found", ch.ID)
	}
	ch.CreatedAt = existing.CreatedAt
	ch.UpdatedAt = time.Now().UTC()
	ch.Metadata = cloneMap(ch.Metadata)

	s.dataLinkChannels[ch.ID] = ch
	return cloneChannel(ch), nil
}

func (s *Store) GetChannel(_ context.Context, id string) (datalink.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.dataLinkChannels[id]
	if !ok {
		return datalink.Channel{}, fmt.Errorf("datalink channel %s not found", id)
	}
	return cloneChannel(ch), nil
}

func (s *Store) ListChannels(_ context.Context, accountID string) ([]datalink.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]datalink.Channel, 0)
	for _, ch := range s.dataLinkChannels {
		if ch.AccountID == accountID {
			result = append(result, cloneChannel(ch))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateDelivery(_ context.Context, del datalink.Delivery) (datalink.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if del.ID == "" {
		del.ID = s.nextIDLocked()
	} else if _, exists := s.dataLinkDeliveries[del.ID]; exists {
		return datalink.Delivery{}, fmt.Errorf("datalink delivery %s already exists", del.ID)
	}
	now := time.Now().UTC()
	del.CreatedAt = now
	del.UpdatedAt = now
	del.Metadata = cloneMap(del.Metadata)
	del.Payload = cloneAnyMap(del.Payload)

	s.dataLinkDeliveries[del.ID] = del
	return cloneDelivery(del), nil
}

func (s *Store) GetDelivery(_ context.Context, id string) (datalink.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	del, ok := s.dataLinkDeliveries[id]
	if !ok {
		return datalink.Delivery{}, fmt.Errorf("datalink delivery %s not found", id)
	}
	return cloneDelivery(del), nil
}

func (s *Store) ListDeliveries(_ context.Context, accountID string, limit int) ([]datalink.Delivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]datalink.Delivery, 0)
	for _, del := range s.dataLinkDeliveries {
		if del.AccountID == accountID {
			result = append(result, cloneDelivery(del))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func cloneChannel(ch datalink.Channel) datalink.Channel {
	ch.Metadata = cloneMap(ch.Metadata)
	ch.SignerSet = cloneAndNormalizeStrings(ch.SignerSet)
	return ch
}

func cloneDelivery(del datalink.Delivery) datalink.Delivery {
	del.Metadata = cloneMap(del.Metadata)
	del.Payload = cloneAnyMap(del.Payload)
	return del
}

func cloneAndNormalizeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// ConfidentialStore implementation ------------------------------------------

func (s *Store) CreateEnclave(_ context.Context, enclave confidential.Enclave) (confidential.Enclave, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if enclave.ID == "" {
		enclave.ID = s.nextIDLocked()
	} else if _, exists := s.confEnclaves[enclave.ID]; exists {
		return confidential.Enclave{}, fmt.Errorf("enclave %s already exists", enclave.ID)
	}
	now := time.Now().UTC()
	enclave.CreatedAt = now
	enclave.UpdatedAt = now
	enclave.Metadata = cloneMap(enclave.Metadata)

	s.confEnclaves[enclave.ID] = enclave
	return cloneEnclave(enclave), nil
}

func (s *Store) UpdateEnclave(_ context.Context, enclave confidential.Enclave) (confidential.Enclave, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.confEnclaves[enclave.ID]
	if !ok {
		return confidential.Enclave{}, fmt.Errorf("enclave %s not found", enclave.ID)
	}
	enclave.CreatedAt = existing.CreatedAt
	enclave.UpdatedAt = time.Now().UTC()
	enclave.Metadata = cloneMap(enclave.Metadata)

	s.confEnclaves[enclave.ID] = enclave
	return cloneEnclave(enclave), nil
}

func (s *Store) GetEnclave(_ context.Context, id string) (confidential.Enclave, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enclave, ok := s.confEnclaves[id]
	if !ok {
		return confidential.Enclave{}, fmt.Errorf("enclave %s not found", id)
	}
	return cloneEnclave(enclave), nil
}

func (s *Store) ListEnclaves(_ context.Context, accountID string) ([]confidential.Enclave, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]confidential.Enclave, 0)
	for _, enclave := range s.confEnclaves {
		if enclave.AccountID == accountID {
			result = append(result, cloneEnclave(enclave))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateSealedKey(_ context.Context, key confidential.SealedKey) (confidential.SealedKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key.ID == "" {
		key.ID = s.nextIDLocked()
	} else {
		for _, existing := range s.confSealedKeys[key.EnclaveID] {
			if existing.ID == key.ID {
				return confidential.SealedKey{}, fmt.Errorf("sealed key %s already exists", key.ID)
			}
		}
	}
	key.CreatedAt = time.Now().UTC()
	key.Metadata = cloneMap(key.Metadata)

	list := s.confSealedKeys[key.EnclaveID]
	list = append([]confidential.SealedKey{key}, list...)
	s.confSealedKeys[key.EnclaveID] = list
	return cloneSealedKey(key), nil
}

func (s *Store) ListSealedKeys(_ context.Context, accountID, enclaveID string, limit int) ([]confidential.SealedKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.confSealedKeys[enclaveID]
	result := make([]confidential.SealedKey, 0, len(list))
	for _, key := range list {
		if key.AccountID == accountID {
			result = append(result, cloneSealedKey(key))
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) CreateAttestation(_ context.Context, att confidential.Attestation) (confidential.Attestation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if att.ID == "" {
		att.ID = s.nextIDLocked()
	} else {
		for _, existing := range s.confAttestations[att.EnclaveID] {
			if existing.ID == att.ID {
				return confidential.Attestation{}, fmt.Errorf("attestation %s already exists", att.ID)
			}
		}
	}
	att.CreatedAt = time.Now().UTC()
	att.Metadata = cloneMap(att.Metadata)
	list := s.confAttestations[att.EnclaveID]
	list = append([]confidential.Attestation{att}, list...)
	s.confAttestations[att.EnclaveID] = list
	return cloneAttestation(att), nil
}

func (s *Store) ListAttestations(_ context.Context, accountID, enclaveID string, limit int) ([]confidential.Attestation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.confAttestations[enclaveID]
	result := make([]confidential.Attestation, 0, len(list))
	for _, att := range list {
		if att.AccountID == accountID {
			result = append(result, cloneAttestation(att))
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *Store) ListAccountAttestations(_ context.Context, accountID string, limit int) ([]confidential.Attestation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []confidential.Attestation
	for _, list := range s.confAttestations {
		for _, att := range list {
			if att.AccountID == accountID {
				result = append(result, cloneAttestation(att))
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func cloneEnclave(enclave confidential.Enclave) confidential.Enclave {
	enclave.Metadata = cloneMap(enclave.Metadata)
	return enclave
}

func cloneSealedKey(key confidential.SealedKey) confidential.SealedKey {
	key.Metadata = cloneMap(key.Metadata)
	if key.Blob != nil {
		key.Blob = append([]byte(nil), key.Blob...)
	}
	return key
}

func cloneAttestation(att confidential.Attestation) confidential.Attestation {
	att.Metadata = cloneMap(att.Metadata)
	if att.ValidUntil != nil {
		value := att.ValidUntil.UTC()
		att.ValidUntil = &value
	}
	return att
}

// DTAStore implementation ----------------------------------------------------

func (s *Store) CreateProduct(_ context.Context, product dta.Product) (dta.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if product.ID == "" {
		product.ID = s.nextIDLocked()
	} else if _, exists := s.dtaProducts[product.ID]; exists {
		return dta.Product{}, fmt.Errorf("dta product %s already exists", product.ID)
	}
	now := time.Now().UTC()
	product.CreatedAt = now
	product.UpdatedAt = now
	product.Metadata = cloneMap(product.Metadata)

	s.dtaProducts[product.ID] = product
	return cloneProduct(product), nil
}

func (s *Store) UpdateProduct(_ context.Context, product dta.Product) (dta.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.dtaProducts[product.ID]
	if !ok {
		return dta.Product{}, fmt.Errorf("dta product %s not found", product.ID)
	}
	product.CreatedAt = existing.CreatedAt
	product.UpdatedAt = time.Now().UTC()
	product.Metadata = cloneMap(product.Metadata)

	s.dtaProducts[product.ID] = product
	return cloneProduct(product), nil
}

func (s *Store) GetProduct(_ context.Context, id string) (dta.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, ok := s.dtaProducts[id]
	if !ok {
		return dta.Product{}, fmt.Errorf("dta product %s not found", id)
	}
	return cloneProduct(product), nil
}

func (s *Store) ListProducts(_ context.Context, accountID string) ([]dta.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]dta.Product, 0)
	for _, product := range s.dtaProducts {
		if product.AccountID == accountID {
			result = append(result, cloneProduct(product))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *Store) CreateOrder(_ context.Context, order dta.Order) (dta.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order.ID == "" {
		order.ID = s.nextIDLocked()
	} else if _, exists := s.dtaOrders[order.ID]; exists {
		return dta.Order{}, fmt.Errorf("dta order %s already exists", order.ID)
	}
	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now
	order.Metadata = cloneMap(order.Metadata)

	s.dtaOrders[order.ID] = order
	return cloneOrder(order), nil
}

func (s *Store) GetOrder(_ context.Context, id string) (dta.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.dtaOrders[id]
	if !ok {
		return dta.Order{}, fmt.Errorf("dta order %s not found", id)
	}
	return cloneOrder(order), nil
}

func (s *Store) ListOrders(_ context.Context, accountID string, limit int) ([]dta.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]dta.Order, 0)
	for _, order := range s.dtaOrders {
		if order.AccountID == accountID {
			result = append(result, cloneOrder(order))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func cloneProduct(product dta.Product) dta.Product {
	product.Metadata = cloneMap(product.Metadata)
	return product
}

func cloneOrder(order dta.Order) dta.Order {
	order.Metadata = cloneMap(order.Metadata)
	return order
}
