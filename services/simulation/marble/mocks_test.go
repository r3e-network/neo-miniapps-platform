// Package neosimulation provides simulation service for automated transaction testing.
package neosimulation

import (
	"context"
	"sync"
	"time"

	neoaccountsclient "github.com/R3E-Network/service_layer/infrastructure/accountpool/client"
)

// =============================================================================
// Mock Pool Client
// =============================================================================

// mockPoolClient implements a mock version of the account pool client for testing.
type mockPoolClient struct {
	mu sync.Mutex

	// Configurable responses
	requestAccountsResp *neoaccountsclient.RequestAccountsResponse
	requestAccountsErr  error

	releaseAccountsResp *neoaccountsclient.ReleaseAccountsResponse
	releaseAccountsErr  error

	invokeContractResp *neoaccountsclient.InvokeContractResponse
	invokeContractErr  error

	invokeMasterResp *neoaccountsclient.InvokeContractResponse
	invokeMasterErr  error

	fundAccountResp *neoaccountsclient.FundAccountResponse
	fundAccountErr  error

	// Call tracking
	requestAccountsCalls  []requestAccountsCall
	releaseAccountsCalls  []releaseAccountsCall
	invokeContractCalls   []invokeContractCall
	invokeMasterCalls     []invokeMasterCall
	fundAccountCalls      []fundAccountCall
}

type requestAccountsCall struct {
	Count   int
	Purpose string
}

type releaseAccountsCall struct {
	AccountIDs []string
}

type invokeContractCall struct {
	AccountID    string
	ContractHash string
	Method       string
	Params       []neoaccountsclient.ContractParam
	Scope        string
}

type invokeMasterCall struct {
	ContractHash string
	Method       string
	Params       []neoaccountsclient.ContractParam
	Scope        string
}

type fundAccountCall struct {
	ToAddress string
	Amount    int64
}

func newMockPoolClient() *mockPoolClient {
	return &mockPoolClient{
		requestAccountsResp: &neoaccountsclient.RequestAccountsResponse{
			Accounts: []neoaccountsclient.AccountInfo{
				{
					ID:      "test-account-1",
					Address: "NTestAddress1234567890123456789012",
					Balances: map[string]neoaccountsclient.TokenBalance{
						"GAS": {Amount: 1000000000}, // 10 GAS
					},
				},
			},
			LockID: "test-lock-1",
		},
		releaseAccountsResp: &neoaccountsclient.ReleaseAccountsResponse{
			ReleasedCount: 1,
		},
		invokeContractResp: &neoaccountsclient.InvokeContractResponse{
			TxHash:      "0xtest-tx-hash-invoke-contract",
			State:       "HALT",
			GasConsumed: "1000000",
			AccountID:   "test-account-1",
		},
		invokeMasterResp: &neoaccountsclient.InvokeContractResponse{
			TxHash:      "0xtest-tx-hash-invoke-master",
			State:       "HALT",
			GasConsumed: "500000",
			AccountID:   "master",
		},
		fundAccountResp: &neoaccountsclient.FundAccountResponse{
			TxHash:      "0xtest-tx-hash-fund",
			FromAddress: "NMasterAddress",
			ToAddress:   "NTestAddress",
			Amount:      1000000000,
		},
	}
}

func (m *mockPoolClient) RequestAccounts(ctx context.Context, count int, purpose string) (*neoaccountsclient.RequestAccountsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestAccountsCalls = append(m.requestAccountsCalls, requestAccountsCall{Count: count, Purpose: purpose})
	return m.requestAccountsResp, m.requestAccountsErr
}

func (m *mockPoolClient) ReleaseAccounts(ctx context.Context, accountIDs []string) (*neoaccountsclient.ReleaseAccountsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseAccountsCalls = append(m.releaseAccountsCalls, releaseAccountsCall{AccountIDs: accountIDs})
	return m.releaseAccountsResp, m.releaseAccountsErr
}

func (m *mockPoolClient) InvokeContract(ctx context.Context, accountID, contractHash, method string, params []neoaccountsclient.ContractParam, scope string) (*neoaccountsclient.InvokeContractResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invokeContractCalls = append(m.invokeContractCalls, invokeContractCall{
		AccountID:    accountID,
		ContractHash: contractHash,
		Method:       method,
		Params:       params,
		Scope:        scope,
	})
	return m.invokeContractResp, m.invokeContractErr
}

func (m *mockPoolClient) InvokeMaster(ctx context.Context, contractHash, method string, params []neoaccountsclient.ContractParam, scope string) (*neoaccountsclient.InvokeContractResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invokeMasterCalls = append(m.invokeMasterCalls, invokeMasterCall{
		ContractHash: contractHash,
		Method:       method,
		Params:       params,
		Scope:        scope,
	})
	return m.invokeMasterResp, m.invokeMasterErr
}

func (m *mockPoolClient) FundAccount(ctx context.Context, toAddress string, amount int64) (*neoaccountsclient.FundAccountResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fundAccountCalls = append(m.fundAccountCalls, fundAccountCall{ToAddress: toAddress, Amount: amount})
	return m.fundAccountResp, m.fundAccountErr
}

// Helper methods for test assertions
func (m *mockPoolClient) getRequestAccountsCalls() []requestAccountsCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]requestAccountsCall{}, m.requestAccountsCalls...)
}

func (m *mockPoolClient) getReleaseAccountsCalls() []releaseAccountsCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]releaseAccountsCall{}, m.releaseAccountsCalls...)
}

func (m *mockPoolClient) getInvokeContractCalls() []invokeContractCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]invokeContractCall{}, m.invokeContractCalls...)
}

func (m *mockPoolClient) getInvokeMasterCalls() []invokeMasterCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]invokeMasterCall{}, m.invokeMasterCalls...)
}

func (m *mockPoolClient) getFundAccountCalls() []fundAccountCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fundAccountCall{}, m.fundAccountCalls...)
}

func (m *mockPoolClient) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestAccountsCalls = nil
	m.releaseAccountsCalls = nil
	m.invokeContractCalls = nil
	m.invokeMasterCalls = nil
	m.fundAccountCalls = nil
}

// =============================================================================
// Mock Contract Invoker
// =============================================================================

// mockContractInvoker implements a mock version of ContractInvoker for testing.
type mockContractInvoker struct {
	mu sync.Mutex

	// Configurable responses
	updatePriceFeedResp string
	updatePriceFeedErr  error

	recordRandomnessResp string
	recordRandomnessErr  error

	payToAppResp string
	payToAppErr  error

	// Call tracking
	updatePriceFeedCalls  []updatePriceFeedCall
	recordRandomnessCalls []recordRandomnessCall
	payToAppCalls         []payToAppCall

	// Stats
	stats map[string]interface{}
}

type updatePriceFeedCall struct {
	Symbol string
}

type recordRandomnessCall struct{}

type payToAppCall struct {
	AppID  string
	Amount int64
	Memo   string
}

func newMockContractInvoker() *mockContractInvoker {
	return &mockContractInvoker{
		updatePriceFeedResp:  "0xtest-pricefeed-tx",
		recordRandomnessResp: "0xtest-randomness-tx",
		payToAppResp:         "0xtest-payment-tx",
		stats: map[string]interface{}{
			"price_feed_updates":  int64(0),
			"randomness_records":  int64(0),
			"payment_hub_pays":    int64(0),
			"contract_errors":     int64(0),
			"locked_accounts":     0,
		},
	}
}

func (m *mockContractInvoker) UpdatePriceFeed(ctx context.Context, symbol string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatePriceFeedCalls = append(m.updatePriceFeedCalls, updatePriceFeedCall{Symbol: symbol})
	if m.updatePriceFeedErr == nil {
		m.stats["price_feed_updates"] = m.stats["price_feed_updates"].(int64) + 1
	}
	return m.updatePriceFeedResp, m.updatePriceFeedErr
}

func (m *mockContractInvoker) RecordRandomness(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordRandomnessCalls = append(m.recordRandomnessCalls, recordRandomnessCall{})
	if m.recordRandomnessErr == nil {
		m.stats["randomness_records"] = m.stats["randomness_records"].(int64) + 1
	}
	return m.recordRandomnessResp, m.recordRandomnessErr
}

func (m *mockContractInvoker) PayToApp(ctx context.Context, appID string, amount int64, memo string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payToAppCalls = append(m.payToAppCalls, payToAppCall{AppID: appID, Amount: amount, Memo: memo})
	if m.payToAppErr == nil {
		m.stats["payment_hub_pays"] = m.stats["payment_hub_pays"].(int64) + 1
	}
	return m.payToAppResp, m.payToAppErr
}

func (m *mockContractInvoker) GetStats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]interface{})
	for k, v := range m.stats {
		result[k] = v
	}
	return result
}

func (m *mockContractInvoker) GetPriceSymbols() []string {
	return []string{"BTCUSD", "ETHUSD", "NEOUSD", "GASUSD"}
}

func (m *mockContractInvoker) GetLockedAccountCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats["locked_accounts"].(int)
}

func (m *mockContractInvoker) ReleaseAllAccounts(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats["locked_accounts"] = 0
}

func (m *mockContractInvoker) Close() {
	m.ReleaseAllAccounts(context.Background())
}

// Helper methods for test assertions
func (m *mockContractInvoker) getUpdatePriceFeedCalls() []updatePriceFeedCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]updatePriceFeedCall{}, m.updatePriceFeedCalls...)
}

func (m *mockContractInvoker) getRecordRandomnessCalls() []recordRandomnessCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]recordRandomnessCall{}, m.recordRandomnessCalls...)
}

func (m *mockContractInvoker) getPayToAppCalls() []payToAppCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]payToAppCall{}, m.payToAppCalls...)
}

func (m *mockContractInvoker) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatePriceFeedCalls = nil
	m.recordRandomnessCalls = nil
	m.payToAppCalls = nil
	m.stats = map[string]interface{}{
		"price_feed_updates":  int64(0),
		"randomness_records":  int64(0),
		"payment_hub_pays":    int64(0),
		"contract_errors":     int64(0),
		"locked_accounts":     0,
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// testClock provides a controllable clock for testing time-dependent code.
type testClock struct {
	now time.Time
}

func newTestClock(t time.Time) *testClock {
	return &testClock{now: t}
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// Verify interface compliance
var _ PoolClientInterface = (*mockPoolClient)(nil)
var _ ContractInvokerInterface = (*mockContractInvoker)(nil)
