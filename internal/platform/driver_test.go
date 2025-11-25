package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockDriver implements Driver for testing.
type mockDriver struct {
	name      string
	startErr  error
	stopErr   error
	pingErr   error
	started   bool
	stopped   bool
}

func (m *mockDriver) Name() string { return m.name }

func (m *mockDriver) Start(ctx context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *mockDriver) Stop(ctx context.Context) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopped = true
	return nil
}

func (m *mockDriver) Ping(ctx context.Context) error {
	return m.pingErr
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.custom == nil {
		t.Error("custom map is nil")
	}
}

func TestRegistry_SetGetDrivers(t *testing.T) {
	r := NewRegistry()

	// Custom driver
	d := &mockDriver{name: "test"}
	r.Register("test", d)

	got, ok := r.Get("test")
	if !ok {
		t.Error("Get returned false for registered driver")
	}
	if got != d {
		t.Error("Get returned wrong driver")
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get returned true for nonexistent driver")
	}
}

func TestRegistry_StartAll(t *testing.T) {
	r := NewRegistry()

	d1 := &mockDriver{name: "d1"}
	d2 := &mockDriver{name: "d2"}
	r.Register("d1", d1)
	r.Register("d2", d2)

	ctx := context.Background()
	err := r.StartAll(ctx)
	if err != nil {
		t.Fatalf("StartAll failed: %v", err)
	}

	if !d1.started {
		t.Error("d1 was not started")
	}
	if !d2.started {
		t.Error("d2 was not started")
	}
}

func TestRegistry_StartAll_Error(t *testing.T) {
	r := NewRegistry()

	d1 := &mockDriver{name: "d1"}
	d2 := &mockDriver{name: "d2", startErr: errors.New("start failed")}
	r.Register("d1", d1)
	r.Register("d2", d2)

	ctx := context.Background()
	err := r.StartAll(ctx)
	if err == nil {
		t.Error("StartAll should fail when a driver fails")
	}
}

func TestRegistry_StopAll(t *testing.T) {
	r := NewRegistry()

	d1 := &mockDriver{name: "d1"}
	d2 := &mockDriver{name: "d2"}
	r.Register("d1", d1)
	r.Register("d2", d2)

	ctx := context.Background()
	_ = r.StartAll(ctx)
	err := r.StopAll(ctx)
	if err != nil {
		t.Fatalf("StopAll failed: %v", err)
	}

	if !d1.stopped {
		t.Error("d1 was not stopped")
	}
	if !d2.stopped {
		t.Error("d2 was not stopped")
	}
}

func TestRegistry_PingAll(t *testing.T) {
	r := NewRegistry()

	d1 := &mockDriver{name: "d1"}
	d2 := &mockDriver{name: "d2", pingErr: errors.New("ping failed")}
	r.Register("d1", d1)
	r.Register("d2", d2)

	ctx := context.Background()
	results := r.PingAll(ctx)

	if results["d1"] != nil {
		t.Errorf("d1 ping should succeed, got: %v", results["d1"])
	}
	if results["d2"] == nil {
		t.Error("d2 ping should fail")
	}
}

func TestChainIDConstants(t *testing.T) {
	chains := []ChainID{
		ChainNeoN3,
		ChainNeoX,
		ChainEthereum,
		ChainPolygon,
		ChainBSC,
		ChainAvalanche,
		ChainArbitrum,
		ChainOptimism,
		ChainBase,
		ChainSolana,
		ChainBitcoin,
	}

	for _, c := range chains {
		if c == "" {
			t.Error("chain ID should not be empty")
		}
	}
}

func TestTxStatusConstants(t *testing.T) {
	statuses := []TxStatus{
		TxStatusPending,
		TxStatusSuccess,
		TxStatusFailed,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("tx status should not be empty")
		}
	}
}

func TestKeyAlgorithmConstants(t *testing.T) {
	algorithms := []KeyAlgorithm{
		KeyAlgorithmECDSA_P256,
		KeyAlgorithmECDSA_Secp256k1,
		KeyAlgorithmEd25519,
		KeyAlgorithmRSA2048,
		KeyAlgorithmRSA4096,
		KeyAlgorithmAES256,
	}

	for _, a := range algorithms {
		if a == "" {
			t.Error("key algorithm should not be empty")
		}
	}
}

func TestBlock(t *testing.T) {
	b := Block{
		Height:       100,
		Hash:         "0x123",
		ParentHash:   "0x122",
		Timestamp:    time.Now(),
		Transactions: []string{"tx1", "tx2"},
		StateRoot:    "0xabc",
		Extra:        map[string]any{"foo": "bar"},
	}

	if b.Height != 100 {
		t.Errorf("Height = %d, want 100", b.Height)
	}
	if len(b.Transactions) != 2 {
		t.Errorf("len(Transactions) = %d, want 2", len(b.Transactions))
	}
}

func TestTransaction(t *testing.T) {
	tx := Transaction{
		Hash:        "0xtx",
		BlockHash:   "0xblock",
		BlockHeight: 100,
		From:        "0xfrom",
		To:          "0xto",
		Status:      TxStatusSuccess,
		Timestamp:   time.Now(),
	}

	if tx.Status != TxStatusSuccess {
		t.Errorf("Status = %s, want success", tx.Status)
	}
}

func TestLog(t *testing.T) {
	l := Log{
		Address:  "0xcontract",
		Topics:   []string{"topic1", "topic2"},
		Data:     []byte("data"),
		LogIndex: 0,
		TxHash:   "0xtx",
		TxIndex:  5,
	}

	if len(l.Topics) != 2 {
		t.Errorf("len(Topics) = %d, want 2", len(l.Topics))
	}
}

func TestContractCall(t *testing.T) {
	call := ContractCall{
		To:   "0xcontract",
		From: "0xcaller",
		Data: []byte("data"),
		Gas:  100000,
	}

	if call.To != "0xcontract" {
		t.Errorf("To = %s, want 0xcontract", call.To)
	}
}

func TestLogFilter(t *testing.T) {
	filter := LogFilter{
		Addresses: []string{"0x1", "0x2"},
		Topics:    [][]string{{"topic1"}, {"topic2"}},
		FromBlock: 100,
		ToBlock:   200,
	}

	if len(filter.Addresses) != 2 {
		t.Errorf("len(Addresses) = %d, want 2", len(filter.Addresses))
	}
}

func TestStorageStats(t *testing.T) {
	stats := StorageStats{
		OpenConnections: 10,
		InUse:           5,
		Idle:            5,
		MaxOpen:         20,
		WaitCount:       100,
		WaitDuration:    time.Second,
	}

	if stats.OpenConnections != 10 {
		t.Errorf("OpenConnections = %d, want 10", stats.OpenConnections)
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Body:      []byte("message body"),
		Timestamp: time.Now(),
		Headers:   map[string]string{"key": "value"},
		Attempts:  1,
	}

	if msg.Topic != "test-topic" {
		t.Errorf("Topic = %s, want test-topic", msg.Topic)
	}
}

func TestTopicStats(t *testing.T) {
	stats := TopicStats{
		MessageCount:   1000,
		ConsumerCount:  5,
		ProducerCount:  2,
		MessagesPerSec: 100.5,
	}

	if stats.MessageCount != 1000 {
		t.Errorf("MessageCount = %d, want 1000", stats.MessageCount)
	}
}

func TestKeyPair(t *testing.T) {
	kp := KeyPair{
		ID:        "key-1",
		Algorithm: KeyAlgorithmECDSA_Secp256k1,
		PublicKey: []byte("pubkey"),
		CreatedAt: time.Now(),
	}

	if kp.Algorithm != KeyAlgorithmECDSA_Secp256k1 {
		t.Errorf("Algorithm = %s, want secp256k1", kp.Algorithm)
	}
}

func TestKeyInfo(t *testing.T) {
	info := KeyInfo{
		ID:        "key-1",
		Algorithm: KeyAlgorithmEd25519,
		CreatedAt: time.Now(),
		Usage:     []string{"sign", "verify"},
	}

	if len(info.Usage) != 2 {
		t.Errorf("len(Usage) = %d, want 2", len(info.Usage))
	}
}

func TestHTTPRequest(t *testing.T) {
	req := HTTPRequest{
		Method:  "GET",
		URL:     "https://example.com",
		Headers: map[string]string{"Accept": "application/json"},
		Timeout: 30 * time.Second,
	}

	if req.Method != "GET" {
		t.Errorf("Method = %s, want GET", req.Method)
	}
}

func TestHTTPResponse(t *testing.T) {
	resp := HTTPResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"ok": true}`),
		Duration:   100 * time.Millisecond,
	}

	if resp.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestPriceData(t *testing.T) {
	pd := PriceData{
		Pair:       "BTC/USD",
		Price:      50000.0,
		Bid:        49999.0,
		Ask:        50001.0,
		Volume24h:  1000000.0,
		Change24h:  2.5,
		Timestamp:  time.Now(),
		Source:     "binance",
		Confidence: 0.99,
	}

	if pd.Pair != "BTC/USD" {
		t.Errorf("Pair = %s, want BTC/USD", pd.Pair)
	}
}

func TestFeedConfig(t *testing.T) {
	fc := FeedConfig{
		ID:          "feed-1",
		Name:        "BTC Price",
		Source:      "coinbase",
		Endpoint:    "https://api.coinbase.com/v2/prices/BTC-USD",
		RefreshRate: time.Minute,
		Timeout:     10 * time.Second,
		Aggregation: "median",
		Enabled:     true,
	}

	if fc.Aggregation != "median" {
		t.Errorf("Aggregation = %s, want median", fc.Aggregation)
	}
}
