package database

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestKeyVersions_GetActiveKeyVersion_NotFound(t *testing.T) {
	client := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v1/key_versions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	repo := NewRepository(client)

	_, err := repo.GetActiveKeyVersion(context.Background())
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestKeyVersions_GetActiveKeyVersion_Success(t *testing.T) {
	validFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	createdAt := validFrom

	client := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "eq."+KeyVersionStatusActive {
			t.Fatalf("unexpected status query: %q", r.URL.Query().Get("status"))
		}
		if r.URL.Query().Get("order") != "valid_from.desc" {
			t.Fatalf("unexpected order query: %q", r.URL.Query().Get("order"))
		}
		if r.URL.Query().Get("limit") != "1" {
			t.Fatalf("unexpected limit query: %q", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]KeyVersion{
			{
				ID:         1,
				KeyVersion: "v123",
				Status:     KeyVersionStatusActive,
				ValidFrom:  validFrom,
				CreatedAt:  createdAt,
			},
		})
	}))
	repo := NewRepository(client)

	got, err := repo.GetActiveKeyVersion(context.Background())
	if err != nil {
		t.Fatalf("GetActiveKeyVersion: %v", err)
	}
	if got.KeyVersion != "v123" {
		t.Fatalf("expected v123, got %q", got.KeyVersion)
	}
}

func TestKeyVersions_CreateAndUpdate(t *testing.T) {
	validFrom := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	validUntil := validFrom.Add(7 * 24 * time.Hour)

	var sawCreate, sawUpdate bool
	client := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			sawCreate = true
			var body KeyVersionCreate
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body.KeyVersion != "v123" || body.Status != KeyVersionStatusActive {
				t.Fatalf("unexpected create body: %+v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]KeyVersion{{
				ID:         1,
				KeyVersion: body.KeyVersion,
				Status:     body.Status,
				ValidFrom:  body.ValidFrom,
				ValidUntil: body.ValidUntil,
				CreatedAt:  validFrom,
			}})
		case r.Method == http.MethodPatch:
			sawUpdate = true
			if r.URL.Query().Get("key_version") != "eq.v123" {
				t.Fatalf("unexpected key_version query: %q", r.URL.Query().Get("key_version"))
			}
			var body KeyVersionUpdate
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			if body.Status == nil || *body.Status != KeyVersionStatusDeprecated {
				t.Fatalf("unexpected update status: %+v", body)
			}
			if body.ValidUntil == nil || !body.ValidUntil.Equal(validUntil) {
				t.Fatalf("unexpected update valid_until: %+v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]KeyVersion{{
				ID:         1,
				KeyVersion: "v123",
				Status:     KeyVersionStatusDeprecated,
				ValidFrom:  validFrom,
				ValidUntil: &validUntil,
				CreatedAt:  validFrom,
			}})
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	repo := NewRepository(client)

	created, err := repo.CreateKeyVersion(context.Background(), KeyVersionCreate{
		KeyVersion: "v123",
		Status:     KeyVersionStatusActive,
		ValidFrom:  validFrom,
	})
	if err != nil {
		t.Fatalf("CreateKeyVersion: %v", err)
	}
	if created.KeyVersion != "v123" {
		t.Fatalf("expected v123, got %q", created.KeyVersion)
	}

	deprecatedStatus := KeyVersionStatusDeprecated
	updated, err := repo.UpdateKeyVersion(context.Background(), "v123", KeyVersionUpdate{
		Status:     &deprecatedStatus,
		ValidUntil: &validUntil,
	})
	if err != nil {
		t.Fatalf("UpdateKeyVersion: %v", err)
	}
	if updated.Status != KeyVersionStatusDeprecated {
		t.Fatalf("expected deprecated, got %q", updated.Status)
	}
	if !sawCreate || !sawUpdate {
		t.Fatalf("expected create and update, saw create=%v update=%v", sawCreate, sawUpdate)
	}
}

func TestKeyVersions_GetKeyVersion_SuccessAndNotFound(t *testing.T) {
	client := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key_version") != "eq.v123" {
			t.Fatalf("unexpected key_version query: %q", r.URL.Query().Get("key_version"))
		}
		if r.URL.Query().Get("limit") != "1" {
			t.Fatalf("unexpected limit query: %q", r.URL.Query().Get("limit"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"key_version":"v123","status":"active","valid_from":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z"}]`))
	}))
	repo := NewRepository(client)

	got, err := repo.GetKeyVersion(context.Background(), "v123")
	if err != nil {
		t.Fatalf("GetKeyVersion: %v", err)
	}
	if got.KeyVersion != "v123" {
		t.Fatalf("expected v123, got %q", got.KeyVersion)
	}

	clientNotFound := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	repoNotFound := NewRepository(clientNotFound)
	_, err = repoNotFound.GetKeyVersion(context.Background(), "v123")
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestKeyVersions_ListKeyVersionsByStatus_Success(t *testing.T) {
	client := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "in.(active,deprecated)" {
			t.Fatalf("unexpected status query: %q", r.URL.Query().Get("status"))
		}
		if r.URL.Query().Get("order") != "valid_from.desc" {
			t.Fatalf("unexpected order query: %q", r.URL.Query().Get("order"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	repo := NewRepository(client)

	got, err := repo.ListKeyVersionsByStatus(context.Background(), []string{KeyVersionStatusActive, KeyVersionStatusDeprecated})
	if err != nil {
		t.Fatalf("ListKeyVersionsByStatus: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %d", len(got))
	}
}

func TestKeyVersions_UpdateKeyVersion_NotFound(t *testing.T) {
	client := newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	repo := NewRepository(client)

	statusActive := KeyVersionStatusActive
	_, err := repo.UpdateKeyVersion(context.Background(), "v123", KeyVersionUpdate{Status: &statusActive})
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestKeyVersions_ListKeyVersionsByStatus_Validation(t *testing.T) {
	repo := NewRepository(newClientWithHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	})))

	if _, err := repo.ListKeyVersionsByStatus(context.Background(), nil); err == nil {
		t.Fatal("expected error for empty statuses")
	}
	if _, err := repo.ListKeyVersionsByStatus(context.Background(), []string{"bad"}); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
