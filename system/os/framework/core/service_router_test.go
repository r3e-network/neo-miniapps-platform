package service

import (
	"context"
	"testing"
)

// mockAutomationService simulates a service with HTTP* methods
type mockAutomationService struct{}

func (m *mockAutomationService) HTTPGetJobs(ctx context.Context, req APIRequest) (any, error) {
	return []string{"job1", "job2"}, nil
}

func (m *mockAutomationService) HTTPPostJobs(ctx context.Context, req APIRequest) (any, error) {
	return map[string]string{"id": "new_job"}, nil
}

func (m *mockAutomationService) HTTPGetJobsById(ctx context.Context, req APIRequest) (any, error) {
	return map[string]string{"id": req.PathParams["id"]}, nil
}

func (m *mockAutomationService) HTTPPatchJobsById(ctx context.Context, req APIRequest) (any, error) {
	return map[string]string{"id": req.PathParams["id"], "updated": "true"}, nil
}

func (m *mockAutomationService) HTTPDeleteJobsById(ctx context.Context, req APIRequest) (any, error) {
	return map[string]string{"deleted": req.PathParams["id"]}, nil
}

// Non-HTTP method should be ignored
func (m *mockAutomationService) ListJobs(ctx context.Context, accountID string) ([]string, error) {
	return nil, nil
}

func TestDiscoverEndpoints(t *testing.T) {
	svc := &mockAutomationService{}
	endpoints := DiscoverEndpoints(svc)

	if len(endpoints) != 5 {
		t.Errorf("expected 5 endpoints, got %d", len(endpoints))
		for _, ep := range endpoints {
			t.Logf("  %s %s -> %s", ep.Method, ep.Path, ep.Handler)
		}
	}

	// Verify specific endpoints
	expected := map[string]string{
		"GET:jobs":       "HTTPGetJobs",
		"POST:jobs":      "HTTPPostJobs",
		"GET:jobs/{id}":  "HTTPGetJobsById",
		"PATCH:jobs/{id}": "HTTPPatchJobsById",
		"DELETE:jobs/{id}": "HTTPDeleteJobsById",
	}

	found := make(map[string]string)
	for _, ep := range endpoints {
		key := ep.Method + ":" + ep.Path
		found[key] = ep.Handler
	}

	for key, handler := range expected {
		if found[key] != handler {
			t.Errorf("expected %s -> %s, got %s", key, handler, found[key])
		}
	}
}

func TestParseMethodName(t *testing.T) {
	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
	}{
		{"HTTPGetJobs", "GET", "jobs"},
		{"HTTPPostJobs", "POST", "jobs"},
		{"HTTPGetJobsById", "GET", "jobs/{id}"},
		{"HTTPPatchJobsById", "PATCH", "jobs/{id}"},
		{"HTTPDeleteJobsById", "DELETE", "jobs/{id}"},
		{"HTTPGetUsersIdPosts", "GET", "users/{id}/posts"},
		{"HTTPPostUsersIdComments", "POST", "users/{id}/comments"},
		{"NotHTTPMethod", "", ""},
		{"HTTP", "", ""},
		{"HTTPGet", "GET", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path := parseMethodName(tt.name)
			if method != tt.wantMethod {
				t.Errorf("parseMethodName(%q) method = %q, want %q", tt.name, method, tt.wantMethod)
			}
			if path != tt.wantPath {
				t.Errorf("parseMethodName(%q) path = %q, want %q", tt.name, path, tt.wantPath)
			}
		})
	}
}

func TestConvertToPath(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Jobs", "jobs"},
		{"JobsById", "jobs/{id}"},
		{"JobsIdComments", "jobs/{id}/comments"},
		{"UsersIdPosts", "users/{id}/posts"},
		{"UsersIdPostsById", "users/{id}/posts/{id}"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToPath(tt.name)
			if got != tt.want {
				t.Errorf("convertToPath(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestServiceRouter_Register(t *testing.T) {
	router := NewServiceRouter("/accounts/{accountID}")
	svc := &mockAutomationService{}

	router.Register("automation", svc)

	services := router.Services()
	if len(services) != 1 || services[0] != "automation" {
		t.Errorf("expected [automation], got %v", services)
	}

	endpoints := router.Endpoints("automation")
	if len(endpoints) != 5 {
		t.Errorf("expected 5 endpoints, got %d", len(endpoints))
	}
}

func TestServiceRouter_Handle(t *testing.T) {
	router := NewServiceRouter("/accounts/{accountID}")
	svc := &mockAutomationService{}
	router.Register("automation", svc)

	// Test that router recognizes the service
	services := router.Services()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	endpoints := router.Endpoints("automation")
	t.Logf("Discovered %d endpoints:", len(endpoints))
	for _, ep := range endpoints {
		t.Logf("  %s %s -> %s", ep.Method, ep.Path, ep.Handler)
	}
}
