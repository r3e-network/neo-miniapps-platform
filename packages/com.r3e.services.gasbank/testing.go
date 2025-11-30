package gasbank

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/testutil"
)

// Re-export centralized mocks for convenience.
type MockAccountChecker = testutil.MockAccountChecker

var NewMockAccountChecker = testutil.NewMockAccountChecker

// mockStore implements Store for testing.
type mockStore struct {
	mu sync.RWMutex

	accounts    map[string]mockAccount
	accountSeq  int
	gasAccounts map[string]GasBankAccount
	gasSeq      int
	transactions map[string]Transaction
	txSeq       int
	approvals   map[string][]WithdrawalApproval
	schedules   map[string]WithdrawalSchedule
	attempts    map[string][]SettlementAttempt
	deadLetters map[string]DeadLetter
	walletIndex map[string]string
}

type mockAccount struct {
	ID    string
	Owner string
}

func newMockStore() *mockStore {
	return &mockStore{
		accounts:     make(map[string]mockAccount),
		gasAccounts:  make(map[string]GasBankAccount),
		transactions: make(map[string]Transaction),
		approvals:    make(map[string][]WithdrawalApproval),
		schedules:    make(map[string]WithdrawalSchedule),
		attempts:     make(map[string][]SettlementAttempt),
		deadLetters:  make(map[string]DeadLetter),
		walletIndex:  make(map[string]string),
	}
}

func (s *mockStore) CreateAccount(_ context.Context, owner string) (mockAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountSeq++
	acct := mockAccount{ID: fmt.Sprintf("acct-%d", s.accountSeq), Owner: owner}
	s.accounts[acct.ID] = acct
	return acct, nil
}

func (s *mockStore) AccountExists(_ context.Context, id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.accounts[id]; !ok {
		return fmt.Errorf("account not found: %s", id)
	}
	return nil
}

func (s *mockStore) AccountTenant(_ context.Context, _ string) string { return "" }

func (s *mockStore) CreateGasAccount(_ context.Context, acct GasBankAccount) (GasBankAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	wallet := strings.ToLower(strings.TrimSpace(acct.WalletAddress))
	if wallet != "" {
		if _, exists := s.walletIndex[wallet]; exists {
			return GasBankAccount{}, ErrWalletInUse
		}
	}
	s.gasSeq++
	acct.ID = fmt.Sprintf("gas-%d", s.gasSeq)
	now := time.Now().UTC()
	acct.CreatedAt, acct.UpdatedAt = now, now
	s.gasAccounts[acct.ID] = acct
	if wallet != "" {
		s.walletIndex[wallet] = acct.ID
	}
	return acct, nil
}

func (s *mockStore) UpdateGasAccount(_ context.Context, acct GasBankAccount) (GasBankAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.gasAccounts[acct.ID]
	if !ok {
		return GasBankAccount{}, fmt.Errorf("gas account not found: %s", acct.ID)
	}
	oldWallet := strings.ToLower(strings.TrimSpace(existing.WalletAddress))
	newWallet := strings.ToLower(strings.TrimSpace(acct.WalletAddress))
	if oldWallet != newWallet {
		delete(s.walletIndex, oldWallet)
		if newWallet != "" {
			if id, exists := s.walletIndex[newWallet]; exists && id != acct.ID {
				return GasBankAccount{}, ErrWalletInUse
			}
			s.walletIndex[newWallet] = acct.ID
		}
	}
	acct.CreatedAt = existing.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	s.gasAccounts[acct.ID] = acct
	return acct, nil
}

func (s *mockStore) GetGasAccount(_ context.Context, id string) (GasBankAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	acct, ok := s.gasAccounts[id]
	if !ok {
		return GasBankAccount{}, fmt.Errorf("gas account not found: %s", id)
	}
	return acct, nil
}

func (s *mockStore) GetGasAccountByWallet(_ context.Context, wallet string) (GasBankAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wallet = strings.ToLower(strings.TrimSpace(wallet))
	id, ok := s.walletIndex[wallet]
	if !ok {
		return GasBankAccount{}, fmt.Errorf("gas account not found for wallet: %s", wallet)
	}
	return s.gasAccounts[id], nil
}

func (s *mockStore) ListGasAccounts(_ context.Context, accountID string) ([]GasBankAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []GasBankAccount
	for _, acct := range s.gasAccounts {
		if acct.AccountID == accountID {
			result = append(result, acct)
		}
	}
	return result, nil
}

func (s *mockStore) CreateGasTransaction(_ context.Context, tx Transaction) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txSeq++
	tx.ID = fmt.Sprintf("tx-%d", s.txSeq)
	now := time.Now().UTC()
	tx.CreatedAt, tx.UpdatedAt = now, now
	s.transactions[tx.ID] = tx
	return tx, nil
}

func (s *mockStore) UpdateGasTransaction(_ context.Context, tx Transaction) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.transactions[tx.ID]
	if !ok {
		return Transaction{}, fmt.Errorf("transaction not found: %s", tx.ID)
	}
	tx.CreatedAt = existing.CreatedAt
	tx.UpdatedAt = time.Now().UTC()
	s.transactions[tx.ID] = tx
	return tx, nil
}

func (s *mockStore) GetGasTransaction(_ context.Context, id string) (Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, ok := s.transactions[id]
	if !ok {
		return Transaction{}, fmt.Errorf("transaction not found: %s", id)
	}
	return tx, nil
}

func (s *mockStore) ListGasTransactions(_ context.Context, gasAccountID string, limit int) ([]Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Transaction
	for _, tx := range s.transactions {
		if tx.AccountID == gasAccountID {
			result = append(result, tx)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *mockStore) ListPendingWithdrawals(_ context.Context) ([]Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Transaction
	for _, tx := range s.transactions {
		if tx.Type == TransactionWithdrawal && tx.Status == StatusPending {
			result = append(result, tx)
		}
	}
	return result, nil
}

func (s *mockStore) UpsertWithdrawalApproval(_ context.Context, a WithdrawalApproval) (WithdrawalApproval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	a.UpdatedAt = now
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	approvals := s.approvals[a.TransactionID]
	found := false
	for i, existing := range approvals {
		if existing.Approver == a.Approver {
			approvals[i] = a
			found = true
			break
		}
	}
	if !found {
		approvals = append(approvals, a)
	}
	s.approvals[a.TransactionID] = approvals
	return a, nil
}

func (s *mockStore) ListWithdrawalApprovals(_ context.Context, txID string) ([]WithdrawalApproval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.approvals[txID], nil
}

func (s *mockStore) SaveWithdrawalSchedule(_ context.Context, sch WithdrawalSchedule) (WithdrawalSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	sch.UpdatedAt = now
	if sch.CreatedAt.IsZero() {
		sch.CreatedAt = now
	}
	s.schedules[sch.TransactionID] = sch
	return sch, nil
}

func (s *mockStore) GetWithdrawalSchedule(_ context.Context, txID string) (WithdrawalSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sch, ok := s.schedules[txID]
	if !ok {
		return WithdrawalSchedule{}, fmt.Errorf("schedule not found: %s", txID)
	}
	return sch, nil
}

func (s *mockStore) DeleteWithdrawalSchedule(_ context.Context, txID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, txID)
	return nil
}

func (s *mockStore) ListDueWithdrawalSchedules(_ context.Context, before time.Time, limit int) ([]WithdrawalSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []WithdrawalSchedule
	for _, sch := range s.schedules {
		if !sch.NextRunAt.After(before) {
			result = append(result, sch)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *mockStore) RecordSettlementAttempt(_ context.Context, a SettlementAttempt) (SettlementAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts[a.TransactionID] = append(s.attempts[a.TransactionID], a)
	return a, nil
}

func (s *mockStore) ListSettlementAttempts(_ context.Context, txID string, limit int) ([]SettlementAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	attempts := s.attempts[txID]
	if limit > 0 && len(attempts) > limit {
		return attempts[:limit], nil
	}
	return attempts, nil
}

func (s *mockStore) UpsertDeadLetter(_ context.Context, d DeadLetter) (DeadLetter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	d.UpdatedAt = now
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	s.deadLetters[d.TransactionID] = d
	return d, nil
}

func (s *mockStore) GetDeadLetter(_ context.Context, txID string) (DeadLetter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deadLetters[txID]
	if !ok {
		return DeadLetter{}, fmt.Errorf("dead letter not found: %s", txID)
	}
	return d, nil
}

func (s *mockStore) ListDeadLetters(_ context.Context, accountID string, limit int) ([]DeadLetter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []DeadLetter
	for _, d := range s.deadLetters {
		if d.AccountID == accountID {
			result = append(result, d)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *mockStore) RemoveDeadLetter(_ context.Context, txID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.deadLetters, txID)
	return nil
}

// failingStore wraps mockStore to simulate failures.
type failingStore struct {
	*mockStore
	failCreateTx bool
}

func (s *failingStore) CreateGasTransaction(ctx context.Context, tx Transaction) (Transaction, error) {
	if s.failCreateTx {
		return Transaction{}, fmt.Errorf("stub create gas transaction failure")
	}
	return s.mockStore.CreateGasTransaction(ctx, tx)
}
