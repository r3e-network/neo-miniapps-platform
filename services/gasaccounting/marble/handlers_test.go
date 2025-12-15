// Package gasaccounting provides GAS ledger and accounting service.
package gasaccounting

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/services/gasaccounting/supabase"
)

func newTestServiceWithMux(t *testing.T) (*Service, *http.ServeMux, *supabase.MockRepository) {
	repo := supabase.NewMockRepository()
	svc, err := New(Config{Repository: repo})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)
	return svc, mux, repo
}

func TestHandleBalance(t *testing.T) {
	_, mux, repo := newTestServiceWithMux(t)

	// Pre-populate balance
	repo.UpdateBalance(nil, 1, 50000, 10000)

	tests := []struct {
		name       string
		method     string
		query      string
		wantStatus int
	}{
		{
			name:       "valid request",
			method:     http.MethodGet,
			query:      "?user_id=1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing user_id",
			method:     http.MethodGet,
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid user_id",
			method:     http.MethodGet,
			query:      "?user_id=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodPost,
			query:      "?user_id=1",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/balance"+tt.query, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleBalance() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleDeposit(t *testing.T) {
	_, mux, _ := newTestServiceWithMux(t)

	tests := []struct {
		name       string
		method     string
		body       interface{}
		wantStatus int
	}{
		{
			name:   "valid deposit",
			method: http.MethodPost,
			body: DepositRequest{
				UserID: 1,
				Amount: 100000,
				TxHash: "0xabc123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "invalid user_id",
			method: http.MethodPost,
			body: DepositRequest{
				UserID: 0,
				Amount: 100000,
				TxHash: "0xabc123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid body",
			method:     http.MethodPost,
			body:       "invalid json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       nil,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				if s, ok := tt.body.(string); ok {
					body = []byte(s)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/deposit", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleDeposit() status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleConsume(t *testing.T) {
	_, mux, repo := newTestServiceWithMux(t)

	// Pre-populate balance
	repo.UpdateBalance(nil, 1, 100000, 0)

	tests := []struct {
		name       string
		method     string
		body       interface{}
		wantStatus int
	}{
		{
			name:   "valid consume",
			method: http.MethodPost,
			body: ConsumeRequest{
				UserID:      1,
				Amount:      5000,
				ServiceID:   "neorand",
				RequestID:   "req-001",
				Description: "VRF request",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "invalid user_id",
			method: http.MethodPost,
			body: ConsumeRequest{
				UserID:    0,
				Amount:    5000,
				ServiceID: "neorand",
				RequestID: "req-002",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid body",
			method:     http.MethodPost,
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       nil,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				if s, ok := tt.body.(string); ok {
					body = []byte(s)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/consume", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleConsume() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleReserve(t *testing.T) {
	_, mux, repo := newTestServiceWithMux(t)

	// Pre-populate balance
	repo.UpdateBalance(nil, 1, 100000, 0)

	tests := []struct {
		name       string
		method     string
		body       interface{}
		wantStatus int
	}{
		{
			name:   "valid reserve",
			method: http.MethodPost,
			body: ReserveRequest{
				UserID:    1,
				Amount:    10000,
				ServiceID: "neovault",
				RequestID: "mix-001",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "invalid user_id",
			method: http.MethodPost,
			body: ReserveRequest{
				UserID:    0,
				Amount:    10000,
				ServiceID: "neovault",
				RequestID: "mix-002",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid body",
			method:     http.MethodPost,
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       nil,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				if s, ok := tt.body.(string); ok {
					body = []byte(s)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/reserve", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleReserve() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleRelease(t *testing.T) {
	svc, mux, repo := newTestServiceWithMux(t)

	// Pre-populate balance and create reservation
	repo.UpdateBalance(nil, 1, 100000, 0)
	reserveResp, _ := svc.Reserve(nil, &ReserveRequest{
		UserID:    1,
		Amount:    10000,
		ServiceID: "neovault",
		RequestID: "mix-001",
	})

	tests := []struct {
		name       string
		method     string
		body       interface{}
		wantStatus int
	}{
		{
			name:   "valid release",
			method: http.MethodPost,
			body: ReleaseRequest{
				ReservationID: reserveResp.ReservationID,
				Consume:       false,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "non-existent reservation",
			method: http.MethodPost,
			body: ReleaseRequest{
				ReservationID: "non-existent",
				Consume:       false,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid body",
			method:     http.MethodPost,
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       nil,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				if s, ok := tt.body.(string); ok {
					body = []byte(s)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/release", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleRelease() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleHistory(t *testing.T) {
	_, mux, repo := newTestServiceWithMux(t)

	// Pre-populate
	repo.UpdateBalance(nil, 1, 100000, 0)

	tests := []struct {
		name       string
		method     string
		query      string
		wantStatus int
	}{
		{
			name:       "valid request",
			method:     http.MethodGet,
			query:      "?user_id=1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "with limit",
			method:     http.MethodGet,
			query:      "?user_id=1&limit=10",
			wantStatus: http.StatusOK,
		},
		{
			name:       "with offset",
			method:     http.MethodGet,
			query:      "?user_id=1&offset=5",
			wantStatus: http.StatusOK,
		},
		{
			name:       "with type filter",
			method:     http.MethodGet,
			query:      "?user_id=1&type=deposit",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing user_id",
			method:     http.MethodGet,
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid user_id",
			method:     http.MethodGet,
			query:      "?user_id=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong method",
			method:     http.MethodPost,
			query:      "?user_id=1",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/history"+tt.query, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("handleHistory() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleHistoryPagination(t *testing.T) {
	_, mux, _ := newTestServiceWithMux(t)

	// Test with invalid limit (should use default)
	req := httptest.NewRequest(http.MethodGet, "/history?user_id=1&limit=999", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleHistory() with large limit status = %d, want %d", w.Code, http.StatusOK)
	}

	// Test with negative offset (should use 0)
	req = httptest.NewRequest(http.MethodGet, "/history?user_id=1&offset=-5", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleHistory() with negative offset status = %d, want %d", w.Code, http.StatusOK)
	}
}
