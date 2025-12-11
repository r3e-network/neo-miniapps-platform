package httputil

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/middleware"
)

// =============================================================================
// ServiceClient Tests
// =============================================================================

func TestNewServiceClient(t *testing.T) {
	client := NewServiceClient(ServiceClientConfig{
		BaseURL:    "http://localhost:8080",
		Timeout:    10 * time.Second,
		MaxRetries: 3,
	})

	if client == nil {
		t.Fatal("NewServiceClient() returned nil")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %s, want http://localhost:8080", client.baseURL)
	}
	if client.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", client.maxRetries)
	}
}

func TestNewServiceClient_Defaults(t *testing.T) {
	client := NewServiceClient(ServiceClientConfig{
		BaseURL: "http://localhost:8080",
	})

	if client.maxRetries != 2 {
		t.Errorf("default maxRetries = %d, want 2", client.maxRetries)
	}
}

func TestNewServiceClient_WithTokenGenerator(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	client := NewServiceClient(ServiceClientConfig{
		PrivateKey: privateKey,
		ServiceID:  "gateway",
		BaseURL:    "http://localhost:8080",
	})

	if client.tokenGenerator == nil {
		t.Error("tokenGenerator should not be nil when PrivateKey and ServiceID are provided")
	}
}

func TestServiceClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		BaseURL: server.URL,
	})

	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestServiceClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "value" {
			t.Errorf("body[key] = %s, want value", body["key"])
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		BaseURL: server.URL,
	})

	resp, err := client.Post(context.Background(), "/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", resp.StatusCode)
	}
}

func TestServiceClient_WithServiceToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(middleware.ServiceTokenHeader)
		if token == "" {
			t.Error("X-Service-Token header should be set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		PrivateKey: privateKey,
		ServiceID:  "gateway",
		BaseURL:    server.URL,
	})

	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()
}

func TestServiceClient_WithUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(middleware.UserIDHeader)
		if userID != "user-123" {
			t.Errorf("X-User-ID = %s, want user-123", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		BaseURL: server.URL,
	})

	// Create context with user ID using middleware's WithUserID function
	ctx := middleware.WithUserID(context.Background(), "user-123")

	resp, err := client.Get(ctx, "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()
}

func TestServiceClient_RetryOnAuthFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		BaseURL:    server.URL,
		MaxRetries: 3,
	})

	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()

	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestServiceClient_Put(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		BaseURL: server.URL,
	})

	resp, err := client.Put(context.Background(), "/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	resp.Body.Close()
}

func TestServiceClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewServiceClient(ServiceClientConfig{
		BaseURL: server.URL,
	})

	resp, err := client.Delete(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

func TestDecodeResponse_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}

	var result map[string]string
	err = DecodeResponse(resp, &result)
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}

	if result["message"] != "hello" {
		t.Errorf("result[message] = %s, want hello", result["message"])
	}
}

func TestDecodeResponse_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}

	err = DecodeResponse(resp, nil)
	if err == nil {
		t.Error("DecodeResponse() should return error for 4xx status")
	}
}
