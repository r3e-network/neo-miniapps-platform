package neoaccountsmarble

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/edgelesssys/ego/attestation"

	"github.com/R3E-Network/service_layer/internal/marble"
	neoaccountssupabase "github.com/R3E-Network/service_layer/services/neoaccounts/supabase"
)

type repoWithFailures struct {
	neoaccountssupabase.RepositoryInterface

	createErr error

	listAvailableWithBalancesErr error
	listAvailableWithBalances    []neoaccountssupabase.AccountWithBalances

	updateErrForID map[string]error

	getBalanceErr    error
	upsertBalanceErr error

	listByLockerErr error

	aggregateErrByToken map[string]error
}

func (r *repoWithFailures) Create(ctx context.Context, acc *neoaccountssupabase.Account) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.RepositoryInterface.Create(ctx, acc)
}

func (r *repoWithFailures) Update(ctx context.Context, acc *neoaccountssupabase.Account) error {
	if r.updateErrForID != nil {
		if err := r.updateErrForID[acc.ID]; err != nil {
			return err
		}
	}
	return r.RepositoryInterface.Update(ctx, acc)
}

func (r *repoWithFailures) ListAvailableWithBalances(ctx context.Context, tokenType string, minBalance *int64, limit int) ([]neoaccountssupabase.AccountWithBalances, error) {
	if r.listAvailableWithBalancesErr != nil {
		return nil, r.listAvailableWithBalancesErr
	}
	if r.listAvailableWithBalances != nil {
		if limit > 0 && len(r.listAvailableWithBalances) > limit {
			return r.listAvailableWithBalances[:limit], nil
		}
		return r.listAvailableWithBalances, nil
	}
	return r.RepositoryInterface.ListAvailableWithBalances(ctx, tokenType, minBalance, limit)
}

func (r *repoWithFailures) GetBalance(ctx context.Context, accountID, tokenType string) (*neoaccountssupabase.AccountBalance, error) {
	if r.getBalanceErr != nil {
		return nil, r.getBalanceErr
	}
	return r.RepositoryInterface.GetBalance(ctx, accountID, tokenType)
}

func (r *repoWithFailures) UpsertBalance(ctx context.Context, accountID, tokenType, scriptHash string, amount int64, decimals int) error {
	if r.upsertBalanceErr != nil {
		return r.upsertBalanceErr
	}
	return r.RepositoryInterface.UpsertBalance(ctx, accountID, tokenType, scriptHash, amount, decimals)
}

func (r *repoWithFailures) ListByLocker(ctx context.Context, lockerID string) ([]neoaccountssupabase.Account, error) {
	if r.listByLockerErr != nil {
		return nil, r.listByLockerErr
	}
	return r.RepositoryInterface.ListByLocker(ctx, lockerID)
}

func (r *repoWithFailures) AggregateTokenStats(ctx context.Context, tokenType string) (*neoaccountssupabase.TokenStats, error) {
	if r.aggregateErrByToken != nil {
		if err := r.aggregateErrByToken[tokenType]; err != nil {
			return nil, err
		}
	}
	return r.RepositoryInterface.AggregateTokenStats(ctx, tokenType)
}

func TestInitializePool_SkipsInitializationWhenDBUnavailableInDevelopment(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)
	mockRepo.simulateError = true

	t.Setenv("MARBLE_ENV", "development")

	if err := svc.initializePool(context.Background()); err != nil {
		t.Fatalf("initializePool() error = %v, want nil", err)
	}
	if len(mockRepo.accounts) != 0 {
		t.Fatalf("expected no accounts created when DB unavailable, got %d", len(mockRepo.accounts))
	}
}

func TestInitializePool_CreatesMinimumPoolAccountsFromEmpty(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	if err := svc.initializePool(context.Background()); err != nil {
		t.Fatalf("initializePool() error = %v", err)
	}
	if len(mockRepo.accounts) != MinPoolAccounts {
		t.Fatalf("pool size = %d, want %d", len(mockRepo.accounts), MinPoolAccounts)
	}
}

func TestInitializePool_NoOpWhenPoolAlreadySufficient(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)

	for i := 0; i < MinPoolAccounts+1; i++ {
		id := "acc-" + strconv.Itoa(i)
		mockRepo.accounts[id] = &neoaccountssupabase.Account{ID: id}
	}

	before := len(mockRepo.accounts)
	if err := svc.initializePool(context.Background()); err != nil {
		t.Fatalf("initializePool() error = %v", err)
	}
	if after := len(mockRepo.accounts); after != before {
		t.Fatalf("pool size changed from %d to %d, want unchanged", before, after)
	}
}

func TestInitializePool_ReturnsErrorWhenAccountCreationFails(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		createErr:           errors.New("create failed"),
	}

	if err := svc.initializePool(context.Background()); err == nil {
		t.Fatalf("expected initializePool to return error when createAccount fails")
	}
}

func TestCreateAccount_ReturnsErrorWhenRepoCreateFails(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		createErr:           errors.New("create failed"),
	}

	if _, err := svc.createAccount(context.Background()); err == nil {
		t.Fatalf("expected error when repo.Create fails")
	}
}

func TestRequestAccounts_RepositoryNotConfigured(t *testing.T) {
	m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, _, reqErr := svc.RequestAccounts(context.Background(), "neovault", 1, "test")
	if reqErr == nil {
		t.Fatalf("expected error when repository is not configured")
	}
}

func TestRequestAccounts_PropagatesListError(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	svc.repo = &repoWithFailures{
		RepositoryInterface:          baseRepo,
		listAvailableWithBalancesErr: errors.New("db list failed"),
	}

	_, _, err := svc.RequestAccounts(context.Background(), "neovault", 1, "test")
	if err == nil || !strings.Contains(err.Error(), "list accounts") {
		t.Fatalf("RequestAccounts() error = %v, want wrapped list error", err)
	}
}

func TestRequestAccounts_ReturnsNoAccountsAvailableWhenCreateFails(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		createErr:           errors.New("create failed"),
	}

	_, _, err := svc.RequestAccounts(context.Background(), "neovault", 1, "test")
	if err == nil || !strings.Contains(err.Error(), "no accounts available") {
		t.Fatalf("RequestAccounts() error = %v, want no accounts available", err)
	}
}

func TestRequestAccounts_SkipsLockFailuresAndReturnsOtherAccounts(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)

	acc1 := &neoaccountssupabase.Account{ID: "acc-1", Address: "NAddr1"}
	acc2 := &neoaccountssupabase.Account{ID: "acc-2", Address: "NAddr2"}
	if err := baseRepo.Create(context.Background(), acc1); err != nil {
		t.Fatalf("Create(acc1): %v", err)
	}
	if err := baseRepo.Create(context.Background(), acc2); err != nil {
		t.Fatalf("Create(acc2): %v", err)
	}

	ordered := []neoaccountssupabase.AccountWithBalances{
		*neoaccountssupabase.NewAccountWithBalances(acc1),
		*neoaccountssupabase.NewAccountWithBalances(acc2),
	}

	svc.repo = &repoWithFailures{
		RepositoryInterface:       baseRepo,
		listAvailableWithBalances: ordered,
		updateErrForID:            map[string]error{"acc-1": errors.New("update failed")},
	}

	accounts, lockID, err := svc.RequestAccounts(context.Background(), "neovault", 2, "test")
	if err != nil {
		t.Fatalf("RequestAccounts() error = %v", err)
	}
	if lockID == "" {
		t.Fatalf("expected lockID to be set")
	}
	if len(accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1 (one lock should fail)", len(accounts))
	}
	if accounts[0].ID != "acc-2" {
		t.Fatalf("returned account = %q, want acc-2", accounts[0].ID)
	}

	got1, _ := baseRepo.GetByID(context.Background(), "acc-1")
	if got1 != nil && got1.LockedBy != "" {
		t.Fatalf("acc-1 should remain unlocked when update fails, got LockedBy=%q", got1.LockedBy)
	}

	got2, _ := baseRepo.GetByID(context.Background(), "acc-2")
	if got2 == nil || got2.LockedBy != "neovault" {
		t.Fatalf("acc-2 should be locked by neovault, got %+v", got2)
	}
}

func TestReleaseAccounts_SkipsMissingAndUpdateFailures(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)

	acc1 := &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neovault", LockedAt: time.Now()}
	acc2 := &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neovault", LockedAt: time.Now()}
	_ = baseRepo.Create(context.Background(), acc1)
	_ = baseRepo.Create(context.Background(), acc2)

	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		updateErrForID:      map[string]error{"acc-1": errors.New("update failed")},
	}

	released, err := svc.ReleaseAccounts(context.Background(), "neovault", []string{"missing", "acc-1", "acc-2"})
	if err != nil {
		t.Fatalf("ReleaseAccounts() error = %v", err)
	}
	if released != 1 {
		t.Fatalf("released = %d, want 1", released)
	}

	got2, _ := baseRepo.GetByID(context.Background(), "acc-2")
	if got2 == nil || got2.LockedBy != "" {
		t.Fatalf("acc-2 should be unlocked, got LockedBy=%q", got2.LockedBy)
	}
}

func TestReleaseAllByService_ReturnsErrorOnListFailure(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		listByLockerErr:     errors.New("list by locker failed"),
	}

	if _, err := svc.ReleaseAllByService(context.Background(), "neovault"); err == nil {
		t.Fatalf("expected error when ListByLocker fails")
	}
}

func TestReleaseAllByService_SkipsUpdateFailures(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)

	acc1 := &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neovault", LockedAt: time.Now()}
	acc2 := &neoaccountssupabase.Account{ID: "acc-2", LockedBy: "neovault", LockedAt: time.Now()}
	_ = baseRepo.Create(context.Background(), acc1)
	_ = baseRepo.Create(context.Background(), acc2)

	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		updateErrForID:      map[string]error{"acc-1": errors.New("update failed")},
	}

	released, err := svc.ReleaseAllByService(context.Background(), "neovault")
	if err != nil {
		t.Fatalf("ReleaseAllByService() error = %v", err)
	}
	if released != 1 {
		t.Fatalf("released = %d, want 1 (one update should fail)", released)
	}

	got1, _ := baseRepo.GetByID(context.Background(), "acc-1")
	if got1 == nil || got1.LockedBy == "" {
		t.Fatalf("acc-1 should remain locked when update fails, got %+v", got1)
	}

	got2, _ := baseRepo.GetByID(context.Background(), "acc-2")
	if got2 == nil || got2.LockedBy != "" {
		t.Fatalf("acc-2 should be unlocked, got %+v", got2)
	}
}

func TestUpdateBalance_PropagatesBalanceLookupError(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	_ = baseRepo.Create(context.Background(), &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neovault"})

	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		getBalanceErr:       errors.New("get balance failed"),
	}

	_, _, _, err := svc.UpdateBalance(context.Background(), "neovault", "acc-1", TokenTypeGAS, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "get balance") {
		t.Fatalf("UpdateBalance() error = %v, want wrapped get balance error", err)
	}
}

func TestUpdateBalance_PropagatesUpsertError(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	_ = baseRepo.Create(context.Background(), &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neovault"})

	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		upsertBalanceErr:    errors.New("upsert failed"),
	}

	_, _, _, err := svc.UpdateBalance(context.Background(), "neovault", "acc-1", TokenTypeGAS, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "upsert balance") {
		t.Fatalf("UpdateBalance() error = %v, want wrapped upsert balance error", err)
	}
}

func TestUpdateBalance_PropagatesAccountUpdateError(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	_ = baseRepo.Create(context.Background(), &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neovault"})

	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		updateErrForID:      map[string]error{"acc-1": errors.New("update failed")},
	}

	_, _, _, err := svc.UpdateBalance(context.Background(), "neovault", "acc-1", TokenTypeGAS, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("UpdateBalance() error = %v, want update error", err)
	}
}

func TestGetPoolInfo_IgnoresTokenStatsErrors(t *testing.T) {
	svc, baseRepo := newTestServiceWithMock(t)
	_ = baseRepo.Create(context.Background(), &neoaccountssupabase.Account{ID: "acc-1"})

	svc.repo = &repoWithFailures{
		RepositoryInterface: baseRepo,
		aggregateErrByToken: map[string]error{TokenTypeNEO: errors.New("stats failed")},
	}

	info, err := svc.GetPoolInfo(context.Background())
	if err != nil {
		t.Fatalf("GetPoolInfo() error = %v", err)
	}
	if info.TokenStats == nil {
		t.Fatalf("expected TokenStats map to be initialized")
	}
	if _, ok := info.TokenStats[TokenTypeNEO]; ok {
		t.Fatalf("expected NEO stats to be omitted when repository errors")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("rng failed") }

func TestSignTransaction_ReturnsErrorWhenSigningFails(t *testing.T) {
	svc, mockRepo := newTestServiceWithMock(t)
	_ = mockRepo.Create(context.Background(), &neoaccountssupabase.Account{ID: "acc-1", LockedBy: "neovault"})

	origRand := rand.Reader
	rand.Reader = failingReader{}
	defer func() { rand.Reader = origRand }()

	_, err := svc.SignTransaction(context.Background(), "neovault", "acc-1", []byte("hash"))
	if err == nil || !strings.Contains(err.Error(), "sign") {
		t.Fatalf("SignTransaction() error = %v, want signing error", err)
	}
}

func TestSignTransaction_RepositoryNotConfigured(t *testing.T) {
	m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	m.SetTestSecret("POOL_MASTER_KEY", []byte("test-master-key-32-bytes-long!!!"))

	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, signErr := svc.SignTransaction(context.Background(), "neovault", "acc-1", []byte("hash"))
	if signErr == nil || !strings.Contains(signErr.Error(), "repository not configured") {
		t.Fatalf("SignTransaction() error = %v, want repository error", signErr)
	}
}

func TestHandleSignTransaction_MissingFieldsReturnsBadRequest(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	body, err := json.Marshal(map[string]any{
		"service_id": "neovault",
		"account_id": "acc-1",
		// tx_hash omitted
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleSignTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateBalance_MissingAccountIDReturnsBadRequest(t *testing.T) {
	svc, _ := newTestServiceWithMock(t)

	body, err := json.Marshal(map[string]any{
		"service_id": "neovault",
		"delta":      1,
		// account_id omitted
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/balance", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleUpdateBalance(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestBuildMasterKeyAttestation_EnclaveModeMarksAsNotSimulated(t *testing.T) {
	m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	key := []byte("test-master-key-32-bytes-long!!!")
	m.SetTestSecret(secretPoolMasterKey, key)

	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.loadMasterKey(m); err != nil {
		t.Fatalf("loadMasterKey: %v", err)
	}

	m.SetTestReport(&attestation.Report{})

	att := svc.buildMasterKeyAttestation()
	if att.Simulated {
		t.Fatalf("expected Simulated=false when IsEnclave=true")
	}
	if att.Source != "neoaccounts" {
		t.Fatalf("Source = %q, want neoaccounts", att.Source)
	}
	if att.Hash != svc.masterKeySummary().Hash {
		t.Fatalf("Hash = %q, want %q", att.Hash, svc.masterKeySummary().Hash)
	}
	if _, err := time.Parse(time.RFC3339, att.Timestamp); err != nil {
		t.Fatalf("Timestamp %q is not RFC3339: %v", att.Timestamp, err)
	}
}
