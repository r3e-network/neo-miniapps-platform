// Package neocompute provides neocompute service.
package neocomputemarble

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/gorilla/mux"
)

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, err := New(Config{Marble: m})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if svc.ID() != ServiceID {
		t.Errorf("ID() = %s, want %s", svc.ID(), ServiceID)
	}
	if svc.Name() != ServiceName {
		t.Errorf("Name() = %s, want %s", svc.Name(), ServiceName)
	}
}

func TestServiceConstants(t *testing.T) {
	if ServiceID != "neocompute" {
		t.Errorf("ServiceID = %s, want neocompute", ServiceID)
	}
	if ServiceName != "NeoCompute Service" {
		t.Errorf("ServiceName = %s, want NeoCompute Service", ServiceName)
	}
}

func TestResultTTLConfiguredViaEnv(t *testing.T) {
	t.Setenv("NEOCOMPUTE_RESULT_TTL", "2h")

	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	if svc.resultTTL != 2*time.Hour {
		t.Fatalf("resultTTL = %v, want %v", svc.resultTTL, 2*time.Hour)
	}
}

func TestGetJobRespectsTTL(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m, ResultTTL: time.Millisecond})

	jobID := "job-expired"
	svc.jobs.Store(jobID, jobEntry{
		UserID:   "user-123",
		Response: &ExecuteResponse{JobID: jobID},
		storedAt: time.Now().Add(-time.Second),
	})

	if resp := svc.getJob("user-123", jobID); resp != nil {
		t.Fatal("expected expired job to be purged")
	}
	if _, ok := svc.jobs.Load(jobID); ok {
		t.Fatal("expired job should be removed from storage")
	}
}

func TestCleanupWorkerRemovesExpiredJobs(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{
		Marble:          m,
		ResultTTL:       5 * time.Millisecond,
		CleanupInterval: 5 * time.Millisecond,
	})

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer svc.Stop()

	jobID := "job-cleanup"
	svc.jobs.Store(jobID, jobEntry{
		UserID:   "user-123",
		Response: &ExecuteResponse{JobID: jobID},
		storedAt: time.Now().Add(-time.Second),
	})

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, ok := svc.jobs.Load(jobID); !ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("cleanup worker did not remove expired job within timeout")
}

func TestExecute(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	req := &ExecuteRequest{
		Script:     "function main() { return 42; }",
		EntryPoint: "main",
		Input:      map[string]interface{}{"key": "value"},
	}

	resp, err := svc.Execute(ctx, "user-123", req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("Status = %s, want completed", resp.Status)
	}
	if resp.JobID == "" {
		t.Error("JobID should not be empty")
	}
	if resp.Output == nil {
		t.Error("Output should not be nil")
	}
}

func TestExecuteEmptyScript(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	ctx := context.Background()
	req := &ExecuteRequest{Script: ""}

	resp, err := svc.Execute(ctx, "user-123", req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("Status = %s, want failed", resp.Status)
	}
}

func TestExecuteWithSecretRefs(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	mockDB := database.NewMockRepository()
	// Pre-populate mock with secrets
	mockDB.CreateSecret(context.Background(), &database.Secret{
		UserID:         "user-123",
		Name:           "API_KEY",
		EncryptedValue: []byte("encrypted-api-key"),
	})
	mockDB.CreateSecret(context.Background(), &database.Secret{
		UserID:         "user-123",
		Name:           "DB_PASSWORD",
		EncryptedValue: []byte("encrypted-db-password"),
	})
	svc, _ := New(Config{Marble: m, DB: mockDB})

	ctx := context.Background()
	req := &ExecuteRequest{
		Script:     "function main() { return secret; }",
		SecretRefs: []string{"API_KEY", "DB_PASSWORD"},
	}

	resp, _ := svc.Execute(ctx, "user-123", req)
	if len(resp.Logs) < 2 {
		t.Error("Should have logs for secret loading")
	}
}

func TestHandleExecuteUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/execute", nil)
	rr := httptest.NewRecorder()
	svc.handleExecute(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleExecuteInvalidBody(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/execute", bytes.NewReader([]byte("invalid")))
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()
	svc.handleExecute(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleExecuteMissingScript(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(ExecuteRequest{})
	req := httptest.NewRequest("POST", "/execute", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()
	svc.handleExecute(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleExecuteSuccess(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(ExecuteRequest{Script: "return 42"})
	req := httptest.NewRequest("POST", "/execute", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()
	svc.handleExecute(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleGetJob(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	job := &ExecuteResponse{JobID: "job-123", Status: "completed"}
	svc.storeJob("user-123", job)

	req := httptest.NewRequest("GET", "/jobs/job-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "job-123"})
	rr := httptest.NewRecorder()
	svc.handleGetJob(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListJobs(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})

	svc.storeJob("user-123", &ExecuteResponse{JobID: "job-123", Status: "completed"})

	req := httptest.NewRequest("GET", "/jobs", nil)
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()
	svc.handleListJobs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestGetMapKeys(t *testing.T) {
	m := map[string]interface{}{"a": 1, "b": 2, "c": 3}
	keys := getMapKeys(m)
	if len(keys) != 3 {
		t.Errorf("len(keys) = %d, want 3", len(keys))
	}
}

func TestGetMapKeysEmpty(t *testing.T) {
	m := map[string]interface{}{}
	keys := getMapKeys(m)
	if len(keys) != 0 {
		t.Errorf("len(keys) = %d, want 0", len(keys))
	}
}

func TestExecuteRequestJSON(t *testing.T) {
	req := ExecuteRequest{
		Script:     "code",
		EntryPoint: "main",
		Timeout:    30,
	}
	data, _ := json.Marshal(req)
	var decoded ExecuteRequest
	json.Unmarshal(data, &decoded)
	if decoded.Script != req.Script {
		t.Errorf("Script = %s, want %s", decoded.Script, req.Script)
	}
}

func TestExecuteResponseJSON(t *testing.T) {
	resp := ExecuteResponse{
		JobID:   "job-123",
		Status:  "completed",
		GasUsed: 1000,
	}
	data, _ := json.Marshal(resp)
	var decoded ExecuteResponse
	json.Unmarshal(data, &decoded)
	if decoded.JobID != resp.JobID {
		t.Errorf("JobID = %s, want %s", decoded.JobID, resp.JobID)
	}
}

func BenchmarkExecute(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	svc, _ := New(Config{Marble: m})
	ctx := context.Background()
	req := &ExecuteRequest{Script: "return 42", EntryPoint: "main"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Execute(ctx, "user-123", req)
	}
}
