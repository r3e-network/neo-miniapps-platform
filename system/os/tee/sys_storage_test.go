package tee

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"sync"
	"testing"
)

func TestSealedStorage_BasicOperations(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	ctx := context.Background()

	t.Run("set and get", func(t *testing.T) {
		key := "test-key"
		value := []byte("test-value")

		if err := storage.Set(ctx, key, value); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, err := storage.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if !bytes.Equal(got, value) {
			t.Errorf("Get() = %v, want %v", got, value)
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		_, err := storage.Get(ctx, "non-existent")
		if err == nil {
			t.Error("Get() expected error for non-existent key")
		}
	})

	t.Run("delete", func(t *testing.T) {
		key := "delete-test"
		value := []byte("to-be-deleted")

		if err := storage.Set(ctx, key, value); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if err := storage.Delete(ctx, key); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := storage.Get(ctx, key)
		if err == nil {
			t.Error("Get() expected error after delete")
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		key := "overwrite-test"
		value1 := []byte("first-value")
		value2 := []byte("second-value")

		if err := storage.Set(ctx, key, value1); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		if err := storage.Set(ctx, key, value2); err != nil {
			t.Fatalf("Set() overwrite error = %v", err)
		}

		got, err := storage.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if !bytes.Equal(got, value2) {
			t.Errorf("Get() = %v, want %v", got, value2)
		}
	})
}

func TestSealedStorage_Encryption(t *testing.T) {
	// Create two storage instances with different enclave IDs
	config1 := SealedStorageConfig{
		EnclaveID:    "enclave-1",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}
	config2 := SealedStorageConfig{
		EnclaveID:    "enclave-2",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage1, err := NewSealedStorage(config1)
	if err != nil {
		t.Fatalf("NewSealedStorage(1) error = %v", err)
	}

	storage2, err := NewSealedStorage(config2)
	if err != nil {
		t.Fatalf("NewSealedStorage(2) error = %v", err)
	}

	ctx := context.Background()
	key := "shared-key"
	value := []byte("secret-data")

	// Store in storage1
	if err := storage1.Set(ctx, key, value); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify storage1 can read it
	got, err := storage1.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() from storage1 error = %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get() from storage1 = %v, want %v", got, value)
	}

	// Storage2 should NOT be able to read it (different sealing key)
	// Since they use local cache, storage2 won't find the key at all
	_, err = storage2.Get(ctx, key)
	if err == nil {
		t.Error("Get() from storage2 should fail - different enclave")
	}
}

func TestSealedStorage_LargeData(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20, // 1MB
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	ctx := context.Background()

	t.Run("large value", func(t *testing.T) {
		key := "large-data"
		value := make([]byte, 100*1024) // 100KB
		if _, err := rand.Read(value); err != nil {
			t.Fatalf("rand.Read() error = %v", err)
		}

		if err := storage.Set(ctx, key, value); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, err := storage.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if !bytes.Equal(got, value) {
			t.Error("Get() returned different data for large value")
		}
	})

	t.Run("value too large", func(t *testing.T) {
		key := "too-large"
		value := make([]byte, 2*1024*1024) // 2MB > 1MB limit

		err := storage.Set(ctx, key, value)
		if err == nil {
			t.Error("Set() expected error for value exceeding max size")
		}
	})
}

func TestSealedStorage_Validation(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 10, // Short key limit for testing
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	ctx := context.Background()

	t.Run("empty key", func(t *testing.T) {
		err := storage.Set(ctx, "", []byte("value"))
		if err == nil {
			t.Error("Set() expected error for empty key")
		}
	})

	t.Run("key too long", func(t *testing.T) {
		err := storage.Set(ctx, "this-key-is-way-too-long", []byte("value"))
		if err == nil {
			t.Error("Set() expected error for key exceeding max length")
		}
	})
}

func TestSealedStorage_Concurrency(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	ctx := context.Background()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "concurrent-key"
				value := []byte("value-from-goroutine")

				_ = storage.Set(ctx, key, value)
				_, _ = storage.Get(ctx, key)
			}
		}(i)
	}

	wg.Wait()
}

func TestSealedStorage_JSONData(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	ctx := context.Background()

	type TestData struct {
		Name    string   `json:"name"`
		Count   int      `json:"count"`
		Tags    []string `json:"tags"`
		Enabled bool     `json:"enabled"`
	}

	original := TestData{
		Name:    "test-object",
		Count:   42,
		Tags:    []string{"tag1", "tag2", "tag3"},
		Enabled: true,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Store
	if err := storage.Set(ctx, "json-data", jsonData); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Retrieve
	got, err := storage.Get(ctx, "json-data")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Deserialize
	var retrieved TestData
	if err := json.Unmarshal(got, &retrieved); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify
	if retrieved.Name != original.Name {
		t.Errorf("Name = %v, want %v", retrieved.Name, original.Name)
	}
	if retrieved.Count != original.Count {
		t.Errorf("Count = %v, want %v", retrieved.Count, original.Count)
	}
	if len(retrieved.Tags) != len(original.Tags) {
		t.Errorf("Tags length = %v, want %v", len(retrieved.Tags), len(original.Tags))
	}
	if retrieved.Enabled != original.Enabled {
		t.Errorf("Enabled = %v, want %v", retrieved.Enabled, original.Enabled)
	}
}

func TestSealedStorage_WithOCALLHandler(t *testing.T) {
	// Create a storage backend
	backend := NewMemoryStorageBackend()

	// Create OCALL handler with the backend
	ocallConfig := OCALLHandlerConfig{
		StorageBackend: backend,
	}
	ocallHandler := NewOCALLHandler(ocallConfig)

	// Create sealed storage with OCALL handler
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		OCALLHandler: ocallHandler,
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	ctx := context.Background()

	// Test basic operations through OCALL
	key := "ocall-test-key"
	value := []byte("ocall-test-value")

	if err := storage.Set(ctx, key, value); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := storage.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !bytes.Equal(got, value) {
		t.Errorf("Get() = %v, want %v", got, value)
	}
}

func TestMemoryStorageBackend(t *testing.T) {
	backend := NewMemoryStorageBackend()
	ctx := context.Background()

	t.Run("set and get", func(t *testing.T) {
		key := "test-key"
		value := []byte("test-value")

		if err := backend.Set(ctx, key, value); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, found, err := backend.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !found {
			t.Error("Get() found = false, want true")
		}
		if !bytes.Equal(got, value) {
			t.Errorf("Get() = %v, want %v", got, value)
		}
	})

	t.Run("get non-existent", func(t *testing.T) {
		_, found, err := backend.Get(ctx, "non-existent")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if found {
			t.Error("Get() found = true for non-existent key")
		}
	})

	t.Run("delete", func(t *testing.T) {
		key := "delete-key"
		value := []byte("to-delete")

		_ = backend.Set(ctx, key, value)
		if err := backend.Delete(ctx, key); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, found, _ := backend.Get(ctx, key)
		if found {
			t.Error("Get() found = true after delete")
		}
	})

	t.Run("list", func(t *testing.T) {
		// Clear and add some keys
		backend2 := NewMemoryStorageBackend()
		_ = backend2.Set(ctx, "key1", []byte("v1"))
		_ = backend2.Set(ctx, "key2", []byte("v2"))
		_ = backend2.Set(ctx, "key3", []byte("v3"))

		keys, err := backend2.List(ctx, "")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(keys) != 3 {
			t.Errorf("List() returned %d keys, want 3", len(keys))
		}
	})

	t.Run("data isolation", func(t *testing.T) {
		// Verify that modifications to returned data don't affect stored data
		key := "isolation-test"
		original := []byte("original-value")

		_ = backend.Set(ctx, key, original)

		got, _, _ := backend.Get(ctx, key)
		got[0] = 'X' // Modify returned data

		got2, _, _ := backend.Get(ctx, key)
		if got2[0] == 'X' {
			t.Error("Modification to returned data affected stored data")
		}
	})
}

func TestHandleStorageOCALL(t *testing.T) {
	backend := NewMemoryStorageBackend()
	ctx := context.Background()

	t.Run("set operation", func(t *testing.T) {
		req := StorageOCALLRequest{
			Operation: "set",
			Key:       "ocall-key",
			Value:     []byte("ocall-value"),
		}
		payload, _ := json.Marshal(req)

		resp, err := HandleStorageOCALL(ctx, backend, payload)
		if err != nil {
			t.Fatalf("HandleStorageOCALL() error = %v", err)
		}
		if !resp.Success {
			t.Errorf("HandleStorageOCALL() success = false, error = %s", resp.Error)
		}
	})

	t.Run("get operation", func(t *testing.T) {
		// First set a value
		_ = backend.Set(ctx, "get-test", []byte("get-value"))

		req := StorageOCALLRequest{
			Operation: "get",
			Key:       "get-test",
		}
		payload, _ := json.Marshal(req)

		resp, err := HandleStorageOCALL(ctx, backend, payload)
		if err != nil {
			t.Fatalf("HandleStorageOCALL() error = %v", err)
		}
		if !resp.Success {
			t.Errorf("HandleStorageOCALL() success = false, error = %s", resp.Error)
		}

		var storageResp StorageOCALLResponse
		_ = json.Unmarshal(resp.Payload, &storageResp)
		if !storageResp.Found {
			t.Error("HandleStorageOCALL() found = false")
		}
		if !bytes.Equal(storageResp.Value, []byte("get-value")) {
			t.Errorf("HandleStorageOCALL() value = %v, want %v", storageResp.Value, []byte("get-value"))
		}
	})

	t.Run("delete operation", func(t *testing.T) {
		_ = backend.Set(ctx, "delete-test", []byte("to-delete"))

		req := StorageOCALLRequest{
			Operation: "delete",
			Key:       "delete-test",
		}
		payload, _ := json.Marshal(req)

		resp, err := HandleStorageOCALL(ctx, backend, payload)
		if err != nil {
			t.Fatalf("HandleStorageOCALL() error = %v", err)
		}
		if !resp.Success {
			t.Errorf("HandleStorageOCALL() success = false, error = %s", resp.Error)
		}

		// Verify deleted
		_, found, _ := backend.Get(ctx, "delete-test")
		if found {
			t.Error("Key still exists after delete")
		}
	})

	t.Run("unknown operation", func(t *testing.T) {
		req := StorageOCALLRequest{
			Operation: "unknown",
			Key:       "test",
		}
		payload, _ := json.Marshal(req)

		resp, err := HandleStorageOCALL(ctx, backend, payload)
		if err != nil {
			t.Fatalf("HandleStorageOCALL() error = %v", err)
		}
		if resp.Success {
			t.Error("HandleStorageOCALL() should fail for unknown operation")
		}
	})

	t.Run("invalid payload", func(t *testing.T) {
		resp, err := HandleStorageOCALL(ctx, backend, []byte("invalid json"))
		if err != nil {
			t.Fatalf("HandleStorageOCALL() error = %v", err)
		}
		if resp.Success {
			t.Error("HandleStorageOCALL() should fail for invalid payload")
		}
	})
}

func TestSealUnseal(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	impl := storage.(*sealedStorageImpl)

	testCases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello")},
		{"medium", make([]byte, 1024)},
		{"with nulls", []byte("hello\x00world\x00")},
		{"binary", []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fill medium test case with random data
			if tc.name == "medium" {
				rand.Read(tc.data)
			}

			sealed, err := impl.seal(tc.data)
			if err != nil {
				t.Fatalf("seal() error = %v", err)
			}

			// Sealed data should be different from original (unless empty)
			if len(tc.data) > 0 && bytes.Equal(sealed, tc.data) {
				t.Error("seal() returned same data as input")
			}

			// Sealed data should be larger (nonce + tag overhead)
			if len(sealed) <= len(tc.data) {
				t.Error("seal() output should be larger than input")
			}

			unsealed, err := impl.unseal(sealed)
			if err != nil {
				t.Fatalf("unseal() error = %v", err)
			}

			if !bytes.Equal(unsealed, tc.data) {
				t.Errorf("unseal() = %v, want %v", unsealed, tc.data)
			}
		})
	}
}

func TestSealedStorage_DifferentNonces(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	impl := storage.(*sealedStorageImpl)
	data := []byte("same data")

	// Seal the same data multiple times
	sealed1, _ := impl.seal(data)
	sealed2, _ := impl.seal(data)
	sealed3, _ := impl.seal(data)

	// Each sealed output should be different (different nonces)
	if bytes.Equal(sealed1, sealed2) {
		t.Error("seal() produced same output for same input (nonce reuse)")
	}
	if bytes.Equal(sealed2, sealed3) {
		t.Error("seal() produced same output for same input (nonce reuse)")
	}
	if bytes.Equal(sealed1, sealed3) {
		t.Error("seal() produced same output for same input (nonce reuse)")
	}

	// But all should unseal to the same data
	unsealed1, _ := impl.unseal(sealed1)
	unsealed2, _ := impl.unseal(sealed2)
	unsealed3, _ := impl.unseal(sealed3)

	if !bytes.Equal(unsealed1, data) || !bytes.Equal(unsealed2, data) || !bytes.Equal(unsealed3, data) {
		t.Error("unseal() returned different data")
	}
}

func TestSealedStorage_TamperedData(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	impl := storage.(*sealedStorageImpl)
	data := []byte("sensitive data")

	sealed, err := impl.seal(data)
	if err != nil {
		t.Fatalf("seal() error = %v", err)
	}

	// Tamper with the sealed data
	tampered := make([]byte, len(sealed))
	copy(tampered, sealed)
	tampered[len(tampered)-1] ^= 0xFF // Flip bits in the last byte

	// Unseal should fail
	_, err = impl.unseal(tampered)
	if err == nil {
		t.Error("unseal() should fail for tampered data")
	}
}

func TestSealedStorage_TruncatedData(t *testing.T) {
	config := SealedStorageConfig{
		EnclaveID:    "test-enclave",
		MaxValueSize: 1 << 20,
		MaxKeyLength: 256,
	}

	storage, err := NewSealedStorage(config)
	if err != nil {
		t.Fatalf("NewSealedStorage() error = %v", err)
	}

	impl := storage.(*sealedStorageImpl)

	// Try to unseal data that's too short
	_, err = impl.unseal([]byte{0x01, 0x02})
	if err == nil {
		t.Error("unseal() should fail for truncated data")
	}
}
