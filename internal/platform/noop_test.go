package platform

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"
)

// TestNoopDriversImplementInterfaces verifies all noop drivers implement their interfaces.
// This test ensures compile-time interface compliance.
func TestNoopDriversImplementInterfaces(t *testing.T) {
	ctx := context.Background()

	var _ RPCDriver = (*NoopRPCDriver)(nil)
	var _ StorageDriver = (*NoopStorageDriver)(nil)
	var _ CacheDriver = (*NoopCacheDriver)(nil)
	var _ QueueDriver = (*NoopQueueDriver)(nil)
	var _ CryptoDriver = (*NoopCryptoDriver)(nil)
	var _ HTTPDriver = (*NoopHTTPDriver)(nil)
	var _ OracleDriver = (*NoopOracleDriver)(nil)

	// Runtime verification
	drivers := []Driver{
		NewNoopRPCDriver(),
		NewNoopStorageDriver(),
		NewNoopCacheDriver(),
		NewNoopQueueDriver(),
		NewNoopCryptoDriver(),
		NewNoopHTTPDriver(),
		NewNoopOracleDriver(),
	}

	for _, d := range drivers {
		if d.Name() == "" {
			t.Errorf("%T: Name() returned empty string", d)
		}
		if err := d.Start(ctx); err != nil {
			t.Errorf("%T: Start() failed: %v", d, err)
		}
		if err := d.Ping(ctx); err != nil {
			t.Errorf("%T: Ping() failed: %v", d, err)
		}
		if err := d.Stop(ctx); err != nil {
			t.Errorf("%T: Stop() failed: %v", d, err)
		}
	}
}

func TestNoopRPCDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopRPCDriver()

	if d.Name() != "noop-rpc" {
		t.Errorf("Name() = %s, want noop-rpc", d.Name())
	}

	chains := d.SupportedChains()
	if len(chains) != 0 {
		t.Errorf("SupportedChains() returned %d chains, want 0", len(chains))
	}

	_, err := d.GetBlockHeight(ctx, ChainEthereum)
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("GetBlockHeight() error = %v, want ErrNotImplemented", err)
	}

	_, err = d.GetBlock(ctx, ChainEthereum, "0x123")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("GetBlock() error = %v, want ErrNotImplemented", err)
	}

	balance, err := d.GetBalance(ctx, ChainEthereum, "0xaddr")
	if err != nil {
		t.Errorf("GetBalance() unexpected error: %v", err)
	}
	if balance.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("GetBalance() = %v, want 0", balance)
	}
}

func TestNoopStorageDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopStorageDriver()

	if d.Type() != "noop" {
		t.Errorf("Type() = %s, want noop", d.Type())
	}

	if d.DB() != nil {
		t.Errorf("DB() = %v, want nil", d.DB())
	}

	stats := d.Stats()
	if stats.OpenConnections != 0 {
		t.Errorf("Stats().OpenConnections = %d, want 0", stats.OpenConnections)
	}

	err := d.Transaction(ctx, func(tx StorageTx) error { return nil })
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Transaction() error = %v, want ErrNotImplemented", err)
	}
}

func TestNoopCacheDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopCacheDriver()

	exists, err := d.Exists(ctx, "key")
	if err != nil {
		t.Errorf("Exists() unexpected error: %v", err)
	}
	if exists {
		t.Errorf("Exists() = true, want false")
	}

	keys, err := d.Keys(ctx, "*")
	if err != nil {
		t.Errorf("Keys() unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("Keys() returned %d keys, want 0", len(keys))
	}

	err = d.Flush(ctx)
	if err != nil {
		t.Errorf("Flush() unexpected error: %v", err)
	}

	_, err = d.Get(ctx, "key")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Get() error = %v, want ErrNotImplemented", err)
	}
}

func TestNoopQueueDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopQueueDriver()

	stats, err := d.TopicStats(ctx, "test-topic")
	if err != nil {
		t.Errorf("TopicStats() unexpected error: %v", err)
	}
	if stats == nil {
		t.Error("TopicStats() returned nil")
	}

	err = d.Publish(ctx, "topic", []byte("msg"))
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Publish() error = %v, want ErrNotImplemented", err)
	}
}

func TestNoopCryptoDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopCryptoDriver()

	keys, err := d.ListKeys(ctx)
	if err != nil {
		t.Errorf("ListKeys() unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("ListKeys() returned %d keys, want 0", len(keys))
	}

	_, err = d.GenerateKey(ctx, KeyAlgorithmECDSA_P256)
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("GenerateKey() error = %v, want ErrNotImplemented", err)
	}

	_, err = d.Sign(ctx, "key-id", []byte("data"))
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Sign() error = %v, want ErrNotImplemented", err)
	}
}

func TestNoopHTTPDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopHTTPDriver()

	// SetTimeout and SetRetry should not panic
	d.SetTimeout(10 * time.Second)
	d.SetRetry(3, time.Second)

	resp, err := d.Get(ctx, "https://example.com", nil)
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Get() error = %v, want ErrNotImplemented", err)
	}
	if resp == nil {
		t.Error("Get() returned nil response")
	}
	if resp != nil && resp.StatusCode != 503 {
		t.Errorf("Get() StatusCode = %d, want 503", resp.StatusCode)
	}
}

func TestNoopOracleDriver(t *testing.T) {
	ctx := context.Background()
	d := NewNoopOracleDriver()

	feeds, err := d.ListFeeds(ctx)
	if err != nil {
		t.Errorf("ListFeeds() unexpected error: %v", err)
	}
	if len(feeds) != 0 {
		t.Errorf("ListFeeds() returned %d feeds, want 0", len(feeds))
	}

	prices, err := d.FetchMultiplePrices(ctx, []string{"BTC/USD"})
	if err != nil {
		t.Errorf("FetchMultiplePrices() unexpected error: %v", err)
	}
	if len(prices) != 0 {
		t.Errorf("FetchMultiplePrices() returned %d prices, want 0", len(prices))
	}

	_, err = d.FetchPrice(ctx, "BTC/USD")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("FetchPrice() error = %v, want ErrNotImplemented", err)
	}
}

func TestErrNotImplemented(t *testing.T) {
	if ErrNotImplemented == nil {
		t.Error("ErrNotImplemented is nil")
	}
	if ErrNotImplemented.Error() == "" {
		t.Error("ErrNotImplemented.Error() is empty")
	}
}

func TestNoopDriversInRegistry(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	// Register all noop drivers
	r.SetRPC(NewNoopRPCDriver())
	r.SetStorage(NewNoopStorageDriver())
	r.SetCache(NewNoopCacheDriver())
	r.SetQueue(NewNoopQueueDriver())
	r.SetCrypto(NewNoopCryptoDriver())
	r.SetHTTP(NewNoopHTTPDriver())
	r.SetOracle(NewNoopOracleDriver())

	// Verify they are registered
	if r.RPC() == nil {
		t.Error("RPC driver is nil")
	}
	if r.Storage() == nil {
		t.Error("Storage driver is nil")
	}
	if r.Cache() == nil {
		t.Error("Cache driver is nil")
	}
	if r.Queue() == nil {
		t.Error("Queue driver is nil")
	}
	if r.Crypto() == nil {
		t.Error("Crypto driver is nil")
	}
	if r.HTTP() == nil {
		t.Error("HTTP driver is nil")
	}
	if r.Oracle() == nil {
		t.Error("Oracle driver is nil")
	}

	// Test StartAll and StopAll
	if err := r.StartAll(ctx); err != nil {
		t.Errorf("StartAll() failed: %v", err)
	}

	if err := r.StopAll(ctx); err != nil {
		t.Errorf("StopAll() failed: %v", err)
	}

	// Test PingAll
	results := r.PingAll(ctx)
	for name, err := range results {
		if err != nil {
			t.Errorf("Ping failed for %s: %v", name, err)
		}
	}
}
