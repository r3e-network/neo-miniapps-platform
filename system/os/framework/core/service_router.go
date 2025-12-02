package service

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// ServiceRouter provides automatic HTTP route registration for services.
// It discovers API endpoints from services that implement APIProvider or
// have methods following the HTTP* naming convention.
type ServiceRouter struct {
	services map[string]serviceEntry
	basePath string
}

type serviceEntry struct {
	name      string
	service   any
	endpoints []APIEndpoint
}

// NewServiceRouter creates a new service router.
// basePath is the prefix for all routes (e.g., "/accounts/{accountID}").
func NewServiceRouter(basePath string) *ServiceRouter {
	return &ServiceRouter{
		services: make(map[string]serviceEntry),
		basePath: strings.TrimSuffix(basePath, "/"),
	}
}

// Register adds a service to the router.
// The service name is used as the URL path segment (e.g., "automation" -> /accounts/{id}/automation).
// Endpoints are discovered from:
// 1. APIProvider interface (if implemented)
// 2. HTTP* method naming convention (via reflection)
func (r *ServiceRouter) Register(name string, service any) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" || service == nil {
		return
	}

	entry := serviceEntry{
		name:    name,
		service: service,
	}

	// Try APIProvider interface first
	if provider, ok := service.(APIProvider); ok {
		entry.endpoints = provider.APIEndpoints()
	}

	// Also discover HTTP* methods via reflection
	discovered := DiscoverEndpoints(service)
	if len(discovered) > 0 {
		// Merge with declared endpoints, preferring declared ones
		existing := make(map[string]bool)
		for _, ep := range entry.endpoints {
			key := ep.Method + ":" + ep.Path
			existing[key] = true
		}
		for _, ep := range discovered {
			key := ep.Method + ":" + ep.Path
			if !existing[key] {
				entry.endpoints = append(entry.endpoints, ep)
			}
		}
	}

	r.services[name] = entry
}

// Mount registers all discovered routes on the given ServeMux.
// Routes are registered as: {basePath}/{serviceName}/{endpoint.Path}
func (r *ServiceRouter) Mount(mux *http.ServeMux) {
	for _, entry := range r.services {
		for _, ep := range entry.endpoints {
			pattern := r.buildPattern(entry.name, ep.Path)
			handler := r.createHandler(entry.service, ep)
			mux.HandleFunc(pattern, handler)
		}
	}
}

// Handle routes a request to the appropriate service handler.
// This is for integration with existing handler dispatch logic.
// path should be the remaining path after /accounts/{accountID}/ (e.g., "automation/jobs/123")
func (r *ServiceRouter) Handle(w http.ResponseWriter, req *http.Request, accountID string, path string) bool {
	parts := strings.SplitN(strings.Trim(path, "/"), "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return false
	}

	serviceName := strings.ToLower(parts[0])
	entry, ok := r.services[serviceName]
	if !ok {
		return false
	}

	// Build the resource path (everything after service name)
	resourcePath := ""
	if len(parts) > 1 {
		resourcePath = parts[1]
	}

	// Find matching endpoint
	endpoint, pathParams := r.matchEndpoint(entry.endpoints, req.Method, resourcePath)
	if endpoint == nil {
		return false
	}

	// Parse request using shared parser
	apiReq, err := ParseRequest(req, accountID, pathParams)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return true
	}

	// Call the handler method
	result, err := r.invokeHandler(entry.service, endpoint.Handler, req.Context(), apiReq)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return true
	}

	WriteJSON(w, http.StatusOK, result)
	return true
}

// Services returns the list of registered service names.
func (r *ServiceRouter) Services() []string {
	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}
	return names
}

// Endpoints returns all endpoints for a service.
func (r *ServiceRouter) Endpoints(serviceName string) []APIEndpoint {
	if entry, ok := r.services[strings.ToLower(serviceName)]; ok {
		return entry.endpoints
	}
	return nil
}

// buildPattern constructs the full URL pattern.
func (r *ServiceRouter) buildPattern(serviceName, path string) string {
	parts := []string{r.basePath, serviceName}
	if path != "" {
		parts = append(parts, path)
	}
	return strings.Join(parts, "/")
}

// matchEndpoint finds the endpoint matching the HTTP method and path.
// Returns the endpoint and extracted path parameters.
func (r *ServiceRouter) matchEndpoint(endpoints []APIEndpoint, method, path string) (*APIEndpoint, map[string]string) {
	path = strings.Trim(path, "/")
	pathParts := strings.Split(path, "/")
	if path == "" {
		pathParts = []string{}
	}

	for i := range endpoints {
		ep := &endpoints[i]
		if ep.Method != method {
			continue
		}

		epPath := strings.Trim(ep.Path, "/")
		epParts := strings.Split(epPath, "/")
		if epPath == "" {
			epParts = []string{}
		}

		if len(epParts) != len(pathParts) {
			continue
		}

		params := make(map[string]string)
		match := true
		for j, epPart := range epParts {
			if strings.HasPrefix(epPart, "{") && strings.HasSuffix(epPart, "}") {
				// Path parameter
				paramName := epPart[1 : len(epPart)-1]
				params[paramName] = pathParts[j]
			} else if epPart != pathParts[j] {
				match = false
				break
			}
		}

		if match {
			return ep, params
		}
	}

	return nil, nil
}

// invokeHandler calls the handler method on the service.
func (r *ServiceRouter) invokeHandler(service any, handlerName string, ctx context.Context, req APIRequest) (any, error) {
	v := reflect.ValueOf(service)
	method := v.MethodByName(handlerName)
	if !method.IsValid() {
		return nil, fmt.Errorf("handler method %s not found", handlerName)
	}

	// Check if it's an HTTP* method (takes ctx, APIRequest) or a business method
	methodType := method.Type()
	if methodType.NumIn() == 2 && methodType.NumOut() == 2 {
		// Check if first param is context.Context
		ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
		if methodType.In(0).Implements(ctxType) {
			// Check if second param is APIRequest
			if methodType.In(1) == reflect.TypeOf(APIRequest{}) {
				// Direct HTTP* method call
				results := method.Call([]reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(req),
				})
				if !results[1].IsNil() {
					return nil, results[1].Interface().(error)
				}
				return results[0].Interface(), nil
			}
		}
	}

	// For business methods referenced in APIEndpoints(), we need to adapt
	// This is a simplified approach - real implementation would need more sophisticated mapping
	return nil, fmt.Errorf("handler %s has unsupported signature", handlerName)
}

// DiscoverEndpoints uses reflection to find HTTP* methods on a service.
// Methods must follow the naming convention: HTTP{Method}{Path}
// e.g., HTTPGetJobs, HTTPPostJobsById, HTTPDeleteJobsIdCancel
func DiscoverEndpoints(service any) []APIEndpoint {
	var endpoints []APIEndpoint
	v := reflect.ValueOf(service)
	t := v.Type()

	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		if !strings.HasPrefix(method.Name, "HTTP") {
			continue
		}

		// Parse method name to extract HTTP method and path
		httpMethod, path := parseMethodName(method.Name)
		if httpMethod == "" {
			continue
		}

		endpoints = append(endpoints, APIEndpoint{
			Method:  httpMethod,
			Path:    path,
			Handler: method.Name,
		})
	}

	return endpoints
}

// parseMethodName extracts HTTP method and path from a method name.
// e.g., "HTTPGetJobs" -> "GET", "jobs"
// e.g., "HTTPPostJobsById" -> "POST", "jobs/{id}"
// e.g., "HTTPDeleteJobsIdCancel" -> "DELETE", "jobs/{id}/cancel"
func parseMethodName(name string) (string, string) {
	if !strings.HasPrefix(name, "HTTP") {
		return "", ""
	}
	name = name[4:] // Remove "HTTP" prefix

	// Extract HTTP method
	var httpMethod string
	for _, m := range []string{"Get", "Post", "Put", "Patch", "Delete"} {
		if strings.HasPrefix(name, m) {
			httpMethod = strings.ToUpper(m)
			name = name[len(m):]
			break
		}
	}
	if httpMethod == "" {
		return "", ""
	}

	// Convert remaining name to path
	path := convertToPath(name)
	return httpMethod, path
}

// convertToPath converts a camelCase method suffix to a URL path.
// e.g., "Jobs" -> "jobs"
// e.g., "JobsById" -> "jobs/{id}"
// e.g., "JobsIdCancel" -> "jobs/{id}/cancel"
func convertToPath(name string) string {
	if name == "" {
		return ""
	}

	var parts []string
	var current strings.Builder

	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	// Convert parts to path segments
	var pathParts []string
	for i, part := range parts {
		lower := strings.ToLower(part)
		// Check if this is an "Id" or "By" marker for path parameters
		if lower == "id" && i > 0 {
			// Previous part becomes the resource, this becomes {id}
			pathParts = append(pathParts, "{id}")
		} else if lower == "by" {
			// Skip "By" - it's just a connector
			continue
		} else {
			pathParts = append(pathParts, lower)
		}
	}

	return strings.Join(pathParts, "/")
}

// createHandler creates an HTTP handler for a specific endpoint.
func (r *ServiceRouter) createHandler(service any, ep APIEndpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != ep.Method {
			w.Header().Set("Allow", ep.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Extract accountID and path parameters
		accountID := ExtractAccountID(req.URL.Path)
		pathParams := MatchPathParams(req.URL.Path, ep.Path)

		// Parse request using shared parser
		apiReq, err := ParseRequest(req, accountID, pathParams)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}

		result, err := r.invokeHandler(service, ep.Handler, req.Context(), apiReq)
		if err != nil {
			WriteError(w, http.StatusBadRequest, err)
			return
		}

		WriteJSON(w, http.StatusOK, result)
	}
}


