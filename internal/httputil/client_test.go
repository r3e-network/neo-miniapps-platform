package httputil

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newResponse(statusCode int, payload []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

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
		t.Fatalf("rsa.GenerateKey: %v", err)
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
	client := NewServiceClient(ServiceClientConfig{
		BaseURL: "http://example",
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		payload, _ := json.Marshal(map[string]string{"status": "ok"})
		return newResponse(http.StatusOK, payload), nil
	})

	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestServiceClient_Post(t *testing.T) {
	client := NewServiceClient(ServiceClientConfig{
		BaseURL: "http://example",
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}

		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "value" {
			t.Errorf("body[key] = %s, want value", body["key"])
		}

		return newResponse(http.StatusCreated, nil), nil
	})

	resp, err := client.Post(context.Background(), "/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", resp.StatusCode)
	}
}

func TestServiceClient_WithServiceToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	client := NewServiceClient(ServiceClientConfig{
		PrivateKey: privateKey,
		ServiceID:  "gateway",
		BaseURL:    "http://example",
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		token := r.Header.Get(serviceauth.ServiceTokenHeader)
		if token == "" {
			t.Error("X-Service-Token header should be set")
		}
		return newResponse(http.StatusOK, nil), nil
	})

	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()
}

func TestServiceClient_WithUserID(t *testing.T) {
	client := NewServiceClient(ServiceClientConfig{
		BaseURL: "http://example",
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		userID := r.Header.Get(serviceauth.UserIDHeader)
		if userID != "user-123" {
			t.Errorf("X-User-ID = %s, want user-123", userID)
		}
		return newResponse(http.StatusOK, nil), nil
	})

	ctx := serviceauth.WithUserID(context.Background(), "user-123")

	resp, err := client.Get(ctx, "/test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	resp.Body.Close()
}

func TestServiceClient_RetryOnAuthFailure(t *testing.T) {
	attempts := 0
	client := NewServiceClient(ServiceClientConfig{
		BaseURL:    "http://example",
		MaxRetries: 3,
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return newResponse(http.StatusUnauthorized, nil), nil
		}
		return newResponse(http.StatusOK, nil), nil
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
	client := NewServiceClient(ServiceClientConfig{
		BaseURL: "http://example",
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %s, want PUT", r.Method)
		}
		return newResponse(http.StatusOK, nil), nil
	})

	resp, err := client.Put(context.Background(), "/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	resp.Body.Close()
}

func TestServiceClient_Delete(t *testing.T) {
	client := NewServiceClient(ServiceClientConfig{
		BaseURL: "http://example",
	})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %s, want DELETE", r.Method)
		}
		return newResponse(http.StatusNoContent, nil), nil
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
	payload, _ := json.Marshal(map[string]string{"message": "hello"})
	resp := newResponse(http.StatusOK, payload)

	var result map[string]string
	if err := DecodeResponse(resp, &result); err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}

	if result["message"] != "hello" {
		t.Errorf("result[message] = %s, want hello", result["message"])
	}
}

func TestDecodeResponse_Error(t *testing.T) {
	resp := newResponse(http.StatusBadRequest, []byte("bad request"))

	err := DecodeResponse(resp, nil)
	if err == nil {
		t.Error("DecodeResponse() should return error for 4xx status")
	}
}

func TestDecodeResponse_InvalidJSON(t *testing.T) {
	resp := newResponse(http.StatusOK, []byte("{invalid json"))
	var out map[string]string
	if err := DecodeResponse(resp, &out); err == nil {
		t.Fatalf("expected DecodeResponse() to fail for invalid JSON")
	}
}

func TestServiceClient_Do_TransportError(t *testing.T) {
	client := NewServiceClient(ServiceClientConfig{BaseURL: "http://example"})
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	_, err := client.Get(context.Background(), "/test")
	if err == nil {
		t.Fatalf("expected transport error")
	}
}
