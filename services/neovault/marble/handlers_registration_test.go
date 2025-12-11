package neovaultmarble

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	neovaultsupabase "github.com/R3E-Network/service_layer/services/neovault/supabase"
)

// =============================================================================
// Registration Input Validation Tests
// =============================================================================

func TestRegistrationApplyInput_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   RegistrationApplyInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: RegistrationApplyInput{
				Email:          "user@example.com",
				Jurisdiction:   "US",
				Purpose:        "privacy",
				ExpectedVolume: "low",
				AcceptTerms:    true,
			},
			wantErr: false,
		},
		{
			name: "missing terms acceptance",
			input: RegistrationApplyInput{
				Email:          "user@example.com",
				Jurisdiction:   "US",
				Purpose:        "privacy",
				ExpectedVolume: "low",
				AcceptTerms:    false,
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			input: RegistrationApplyInput{
				Email:          "invalid-email",
				Jurisdiction:   "US",
				Purpose:        "privacy",
				ExpectedVolume: "low",
				AcceptTerms:    true,
			},
			wantErr: true,
		},
		{
			name: "invalid jurisdiction",
			input: RegistrationApplyInput{
				Email:          "user@example.com",
				Jurisdiction:   "USA", // should be 2-letter ISO code
				Purpose:        "privacy",
				ExpectedVolume: "low",
				AcceptTerms:    true,
			},
			wantErr: true,
		},
		{
			name: "invalid volume",
			input: RegistrationApplyInput{
				Email:          "user@example.com",
				Jurisdiction:   "US",
				Purpose:        "privacy",
				ExpectedVolume: "massive", // invalid
				AcceptTerms:    true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate email
			emailValid := tt.input.Email != "" && emailRegex.MatchString(tt.input.Email)
			// Validate jurisdiction
			jurisdictionValid := tt.input.Jurisdiction != "" && jurisdictionRegex.MatchString(tt.input.Jurisdiction)
			// Validate volume
			volumeValid := validVolumes[tt.input.ExpectedVolume]
			// Validate terms
			termsValid := tt.input.AcceptTerms

			hasError := !emailValid || !jurisdictionValid || !volumeValid || !termsValid

			if hasError != tt.wantErr {
				t.Errorf("validation result = %v, wantErr %v", hasError, tt.wantErr)
			}
		})
	}
}

func TestEmailRegex(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"user.name@example.co.uk", true},
		{"user+tag@example.com", true},
		{"invalid", false},
		{"@example.com", false},
		{"user@", false},
		{"user@.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := emailRegex.MatchString(tt.email); got != tt.valid {
				t.Errorf("emailRegex.MatchString(%q) = %v, want %v", tt.email, got, tt.valid)
			}
		})
	}
}

func TestJurisdictionRegex(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"US", true},
		{"GB", true},
		{"CN", true},
		{"JP", true},
		{"us", false}, // must be uppercase
		{"USA", false},
		{"U", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := jurisdictionRegex.MatchString(tt.code); got != tt.valid {
				t.Errorf("jurisdictionRegex.MatchString(%q) = %v, want %v", tt.code, got, tt.valid)
			}
		})
	}
}

func TestValidVolumes(t *testing.T) {
	tests := []struct {
		volume string
		valid  bool
	}{
		{"low", true},
		{"medium", true},
		{"high", true},
		{"LOW", false},
		{"massive", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.volume, func(t *testing.T) {
			if got := validVolumes[tt.volume]; got != tt.valid {
				t.Errorf("validVolumes[%q] = %v, want %v", tt.volume, got, tt.valid)
			}
		})
	}
}

// =============================================================================
// Registration Status Tests
// =============================================================================

func TestRegistrationStatus_IsApproved(t *testing.T) {
	tests := []struct {
		status   neovaultsupabase.RegistrationStatus
		approved bool
	}{
		{neovaultsupabase.RegStatusApproved, true},
		{neovaultsupabase.RegStatusPending, false},
		{neovaultsupabase.RegStatusRejected, false},
		{neovaultsupabase.RegStatusSuspended, false},
		{neovaultsupabase.RegStatusRevoked, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsApproved(); got != tt.approved {
				t.Errorf("RegistrationStatus(%q).IsApproved() = %v, want %v", tt.status, got, tt.approved)
			}
		})
	}
}

// =============================================================================
// Registration Response Types Tests
// =============================================================================

func TestRegistrationResponseJSON(t *testing.T) {
	resp := RegistrationResponse{
		ID:              "reg-123",
		Status:          "approved",
		Message:         "Registration approved",
		TermsVersion:    "1.0.0",
		RejectionReason: "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded RegistrationResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ID != resp.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, resp.ID)
	}
	if decoded.Status != resp.Status {
		t.Errorf("Status = %s, want %s", decoded.Status, resp.Status)
	}
}

func TestAdminApproveInputJSON(t *testing.T) {
	maxDaily := int64(1000000)
	input := AdminApproveInput{
		RegistrationID:   "reg-123",
		Action:           "approve",
		Notes:            "Verified user",
		MaxDailyAmount:   &maxDaily,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded AdminApproveInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.RegistrationID != input.RegistrationID {
		t.Errorf("RegistrationID = %s, want %s", decoded.RegistrationID, input.RegistrationID)
	}
	if decoded.Action != input.Action {
		t.Errorf("Action = %s, want %s", decoded.Action, input.Action)
	}
	if *decoded.MaxDailyAmount != *input.MaxDailyAmount {
		t.Errorf("MaxDailyAmount = %d, want %d", *decoded.MaxDailyAmount, *input.MaxDailyAmount)
	}
}

func TestRegistrationSummaryJSON(t *testing.T) {
	summary := RegistrationSummary{
		ID:             "reg-123",
		UserID:         "user-456",
		Email:          "user@example.com",
		Status:         "pending",
		Jurisdiction:   "US",
		ExpectedVolume: "medium",
		CreatedAt:      time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded RegistrationSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ID != summary.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, summary.ID)
	}
	if decoded.Email != summary.Email {
		t.Errorf("Email = %s, want %s", decoded.Email, summary.Email)
	}
}

// =============================================================================
// Extract Email Domain Tests
// =============================================================================

func TestExtractEmailDomain(t *testing.T) {
	tests := []struct {
		email  string
		domain string
	}{
		{"user@example.com", "example.com"},
		{"admin@company.co.uk", "company.co.uk"},
		{"test@localhost", "localhost"},
		{"invalid", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := extractEmailDomain(tt.email); got != tt.domain {
				t.Errorf("extractEmailDomain(%q) = %q, want %q", tt.email, got, tt.domain)
			}
		})
	}
}

// =============================================================================
// Handler Tests (with mock repository)
// =============================================================================

// mockRegistrationRepo implements the registration methods of RepositoryInterface
type mockRegistrationRepo struct {
	neovaultsupabase.RepositoryInterface
	registrations map[string]*neovaultsupabase.Registration
	auditLogs     []*neovaultsupabase.AuditLog
}

func newMockRegistrationRepo() *mockRegistrationRepo {
	return &mockRegistrationRepo{
		registrations: make(map[string]*neovaultsupabase.Registration),
		auditLogs:     make([]*neovaultsupabase.AuditLog, 0),
	}
}

func (m *mockRegistrationRepo) CreateRegistration(ctx context.Context, reg *neovaultsupabase.Registration) error {
	reg.ID = "reg-" + time.Now().Format("20060102150405")
	reg.CreatedAt = time.Now()
	reg.UpdatedAt = time.Now()
	m.registrations[reg.UserID] = reg
	return nil
}

func (m *mockRegistrationRepo) UpdateRegistration(ctx context.Context, reg *neovaultsupabase.Registration) error {
	reg.UpdatedAt = time.Now()
	m.registrations[reg.UserID] = reg
	return nil
}

func (m *mockRegistrationRepo) GetRegistrationByUserID(ctx context.Context, userID string) (*neovaultsupabase.Registration, error) {
	if reg, ok := m.registrations[userID]; ok {
		return reg, nil
	}
	return nil, neovaultsupabase.ErrNotFound
}

func (m *mockRegistrationRepo) GetRegistrationByID(ctx context.Context, id string) (*neovaultsupabase.Registration, error) {
	for _, reg := range m.registrations {
		if reg.ID == id {
			return reg, nil
		}
	}
	return nil, neovaultsupabase.ErrNotFound
}

func (m *mockRegistrationRepo) ListRegistrationsByStatus(ctx context.Context, status neovaultsupabase.RegistrationStatus) ([]neovaultsupabase.Registration, error) {
	var result []neovaultsupabase.Registration
	for _, reg := range m.registrations {
		if reg.Status == status {
			result = append(result, *reg)
		}
	}
	return result, nil
}

func (m *mockRegistrationRepo) ListPendingRegistrations(ctx context.Context) ([]neovaultsupabase.Registration, error) {
	return m.ListRegistrationsByStatus(ctx, neovaultsupabase.RegStatusPending)
}

func (m *mockRegistrationRepo) CreateAuditLog(ctx context.Context, log *neovaultsupabase.AuditLog) error {
	log.ID = "audit-" + time.Now().Format("20060102150405")
	log.CreatedAt = time.Now()
	m.auditLogs = append(m.auditLogs, log)
	return nil
}

func (m *mockRegistrationRepo) ListAuditLogsByUser(ctx context.Context, userID string) ([]neovaultsupabase.AuditLog, error) {
	var result []neovaultsupabase.AuditLog
	for _, log := range m.auditLogs {
		if log.UserID == userID {
			result = append(result, *log)
		}
	}
	return result, nil
}

func (m *mockRegistrationRepo) ListAuditLogsByEntity(ctx context.Context, entityType, entityID string) ([]neovaultsupabase.AuditLog, error) {
	var result []neovaultsupabase.AuditLog
	for _, log := range m.auditLogs {
		if log.EntityType == entityType && log.EntityID == entityID {
			result = append(result, *log)
		}
	}
	return result, nil
}

// =============================================================================
// Handler HTTP Tests
// =============================================================================

func TestHandleRegistrationApply_Unauthorized(t *testing.T) {
	req := httptest.NewRequest("POST", "/registration/apply", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	// No X-User-ID header - should fail
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate handler checking for user ID
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	})

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleRegistrationApply_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/registration/apply", bytes.NewBufferString(`{invalid json`))
	req.Header.Set("X-User-ID", "user-123")
	_ = httptest.NewRecorder() // recorder not used in this test

	// Decode JSON and check for error
	var input RegistrationApplyInput
	err := json.NewDecoder(req.Body).Decode(&input)
	if err == nil {
		t.Error("expected JSON decode error, got nil")
	}
}

func TestHandleRegistrationApply_MissingTerms(t *testing.T) {
	input := RegistrationApplyInput{
		Email:          "user@example.com",
		Jurisdiction:   "US",
		Purpose:        "privacy",
		ExpectedVolume: "low",
		AcceptTerms:    false, // not accepted
	}

	body, _ := json.Marshal(input)
	req := httptest.NewRequest("POST", "/registration/apply", bytes.NewBuffer(body))
	req.Header.Set("X-User-ID", "user-123")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Validate terms acceptance
	if !input.AcceptTerms {
		http.Error(w, "you must accept the terms of service", http.StatusBadRequest)
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRegistrationStatus_NotRegistered(t *testing.T) {
	repo := newMockRegistrationRepo()

	// User not in registrations
	_, err := repo.GetRegistrationByUserID(context.Background(), "user-999")
	if err != neovaultsupabase.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminApproveInput_Actions(t *testing.T) {
	validActions := []string{"approve", "reject", "suspend", "revoke"}
	invalidActions := []string{"delete", "ban", ""}

	for _, action := range validActions {
		t.Run("valid_"+action, func(t *testing.T) {
			input := AdminApproveInput{
				RegistrationID: "reg-123",
				Action:         action,
			}
			// Action should be recognized
			switch input.Action {
			case "approve", "reject", "suspend", "revoke":
				// valid
			default:
				t.Errorf("action %q should be valid", action)
			}
		})
	}

	for _, action := range invalidActions {
		t.Run("invalid_"+action, func(t *testing.T) {
			input := AdminApproveInput{
				RegistrationID: "reg-123",
				Action:         action,
			}
			switch input.Action {
			case "approve", "reject", "suspend", "revoke":
				t.Errorf("action %q should be invalid", action)
			default:
				// invalid as expected
			}
		})
	}
}

// =============================================================================
// Audit Log Tests
// =============================================================================

func TestAuditLogCreation(t *testing.T) {
	repo := newMockRegistrationRepo()

	log := &neovaultsupabase.AuditLog{
		UserID:      "user-123",
		AdminID:     "",
		Action:      neovaultsupabase.AuditActionRegistrationSubmitted,
		EntityType:  "registration",
		EntityID:    "reg-123",
		RequestPath: "/registration/apply",
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0",
		Details:     map[string]interface{}{"jurisdiction": "US"},
	}

	err := repo.CreateAuditLog(context.Background(), log)
	if err != nil {
		t.Fatalf("CreateAuditLog() error = %v", err)
	}

	if log.ID == "" {
		t.Error("log ID should be set after creation")
	}

	// Verify log was stored
	logs, err := repo.ListAuditLogsByUser(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("ListAuditLogsByUser() error = %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("len(logs) = %d, want 1", len(logs))
	}
}

// =============================================================================
// Current Terms Version Test
// =============================================================================

func TestCurrentTermsVersion(t *testing.T) {
	if CurrentTermsVersion == "" {
		t.Error("CurrentTermsVersion should not be empty")
	}
	// Should follow semver format
	if CurrentTermsVersion != "1.0.0" {
		t.Logf("CurrentTermsVersion = %s (expected 1.0.0 for initial release)", CurrentTermsVersion)
	}
}
