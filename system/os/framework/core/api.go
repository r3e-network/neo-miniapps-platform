package service

import "context"

// API method naming convention:
//
// Services expose HTTP APIs by implementing methods with specific signatures.
// The service engine automatically discovers and registers these methods.
//
// Naming pattern: HTTP{Method}{Path}
//   - HTTPGetJobs      -> GET    /jobs
//   - HTTPPostJobs     -> POST   /jobs
//   - HTTPGetJobsById  -> GET    /jobs/{id}
//   - HTTPPatchJobsById -> PATCH /jobs/{id}
//   - HTTPDeleteJobsById -> DELETE /jobs/{id}
//
// Method signature must be:
//   func (s *Service) HTTPGetJobs(ctx context.Context, req APIRequest) (any, error)
//
// Example:
//
//	// GET /accounts/{accountID}/automation/jobs
//	func (s *Service) HTTPGetJobs(ctx context.Context, req APIRequest) (any, error) {
//	    return s.ListJobs(ctx, req.AccountID)
//	}
//
//	// POST /accounts/{accountID}/automation/jobs
//	func (s *Service) HTTPPostJobs(ctx context.Context, req APIRequest) (any, error) {
//	    return s.CreateJob(ctx, req.AccountID, req.Body)
//	}
//
//	// GET /accounts/{accountID}/automation/jobs/{id}
//	func (s *Service) HTTPGetJobsById(ctx context.Context, req APIRequest) (any, error) {
//	    return s.GetJob(ctx, req.PathParams["id"])
//	}

// APIRequest contains all request data passed to API handlers.
type APIRequest struct {
	AccountID  string            // From URL path /accounts/{accountID}/...
	PathParams map[string]string // URL path parameters (e.g., {"id": "job_123"})
	Query      map[string]string // URL query parameters
	Body       map[string]any    // Parsed JSON body
}

// APIHandlerFunc is the standard signature for API handler methods.
// All HTTP* methods must follow this signature.
type APIHandlerFunc func(ctx context.Context, req APIRequest) (any, error)

// APIEndpoint represents a discovered API endpoint (for documentation/introspection).
type APIEndpoint struct {
	Method      string // HTTP method
	Path        string // URL path relative to service
	Handler     string // Method name
	Description string // From method doc comment (if available)
}

// APIProvider can be optionally implemented for explicit endpoint declaration.
// If not implemented, endpoints are discovered via HTTP* method naming convention.
type APIProvider interface {
	APIEndpoints() []APIEndpoint
}

// HTTP method constants
const (
	GET    = "GET"
	POST   = "POST"
	PUT    = "PUT"
	PATCH  = "PATCH"
	DELETE = "DELETE"
)
