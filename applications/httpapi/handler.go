package httpapi

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	app "github.com/R3E-Network/service_layer/applications"
	"github.com/R3E-Network/service_layer/applications/auth"
	"github.com/R3E-Network/service_layer/pkg/metrics"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts/service"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle/service"
	engine "github.com/R3E-Network/service_layer/system/engine/core"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// handler bundles HTTP endpoints for the application services.
type handler struct {
	services          app.ServiceProvider
	serviceRouter     *core.ServiceRouter // Auto-discovered service routes
	neo               neoProvider
	supabaseGoTrueURL string
	authManager       authManager
	audit             *auditLog
	modulesFn         ModuleProvider
	busPub            BusPublisher
	busPush           BusPusher
	invoke            ComputeInvoker
	listenAddr        func() string
	slowMS            float64
	busMaxBytes       int64
	rpcEngines        func() []engine.RPCEngine
	rpcPolicy         *rpcPolicy
	rpcMu             sync.Mutex
	rpcSeq            map[string]int
	adminConfigStore  adminConfigStore
	extraRoutes       []RouteRegistrar
}

type authManager interface {
	HasUsers() bool
	Authenticate(username, password string) (auth.User, error)
	Issue(user auth.User, ttl time.Duration) (string, time.Time, error)
	Validate(token string) (*auth.Claims, error)
	IssueWalletChallenge(wallet string, ttl time.Duration) (string, time.Time, error)
	VerifyWalletSignature(wallet, signature, pubKey string) (auth.User, error)
}

func parseMaxBytes(value string, def int64) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return def
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
		return n
	}
	return def
}

// HandlerOption customizes handler behaviour.
type HandlerOption func(*handler)

// WithBusMaxBytes caps request bodies for bus endpoints.
func WithBusMaxBytes(limit int64) HandlerOption {
	return func(h *handler) {
		if limit > 0 {
			h.busMaxBytes = limit
		}
	}
}

// WithBusEndpoints wires engine fan-out helpers for /system bus endpoints.
func WithBusEndpoints(publish BusPublisher, push BusPusher, invoke ComputeInvoker) HandlerOption {
	return func(h *handler) {
		h.busPub = publish
		h.busPush = push
		h.invoke = invoke
	}
}

// WithListenAddrProvider injects a function returning the current listen address.
func WithListenAddrProvider(provider func() string) HandlerOption {
	return func(h *handler) {
		h.listenAddr = provider
	}
}

// WithHandlerSupabaseGoTrueURL wires the configured self-hosted GoTrue base URL for refresh proxying.
func WithHandlerSupabaseGoTrueURL(url string) HandlerOption {
	return func(h *handler) {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			h.supabaseGoTrueURL = trimmed
		}
	}
}

// WithSlowThreshold overrides the slow module threshold (milliseconds) for status responses.
func WithSlowThreshold(ms float64) HandlerOption {
	return func(h *handler) {
		if ms > 0 {
			h.slowMS = ms
		}
	}
}

// WithRPCEngines injects a lookup for available RPC hubs.
func WithRPCEngines(fn func() []engine.RPCEngine) HandlerOption {
	return func(h *handler) {
		h.rpcEngines = fn
	}
}

// WithRPCPolicy enforces tenancy/rate limits on /system/rpc.
func WithRPCPolicy(policy *RPCPolicy) HandlerOption {
	return func(h *handler) {
		if policy != nil {
			h.rpcPolicy = newRPCPolicy(*policy)
		}
	}
}

// WithAdminConfigStore sets the admin configuration store.
func WithAdminConfigStore(store adminConfigStore) HandlerOption {
	return func(h *handler) {
		h.adminConfigStore = store
	}
}

// RouteRegistrar is a function that registers routes on a ServeMux.
type RouteRegistrar func(*http.ServeMux)

// WithExtraRoutes allows registering additional routes on the handler's mux.
// This is useful for integrating external API handlers (e.g., system/api).
func WithExtraRoutes(registrars ...RouteRegistrar) HandlerOption {
	return func(h *handler) {
		h.extraRoutes = append(h.extraRoutes, registrars...)
	}
}

// WithServiceRouter sets the service router for automatic API endpoint discovery.
// Services registered with the router will have their HTTP* methods automatically
// invoked when matching requests arrive.
func WithServiceRouter(router *core.ServiceRouter) HandlerOption {
	return func(h *handler) {
		h.serviceRouter = router
	}
}

// NewHandler returns a mux exposing the core REST API.
func NewHandler(
	services app.ServiceProvider,
	tokens []string,
	authMgr authManager,
	audit *auditLog,
	neo neoProvider,
	modules ModuleProvider,
	opts ...HandlerOption,
) http.Handler {
	h := &handler{services: services, authManager: authMgr, audit: audit, neo: neo, modulesFn: modules, busMaxBytes: parseMaxBytes(os.Getenv("BUS_MAX_BYTES"), 1<<20)}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	// Auto-wire ServiceRouter from ServiceProvider if not explicitly set
	if h.serviceRouter == nil && services != nil {
		h.serviceRouter = services.GetServiceRouter()
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mountRoutes(mux,
		route{pattern: "/healthz", method: http.MethodGet, handler: h.health},
		route{pattern: "/readyz", method: http.MethodGet, handler: h.readyz},
		route{pattern: "/livez", method: http.MethodGet, handler: h.livez},
		route{pattern: "/system/descriptors", method: http.MethodGet, handler: h.systemDescriptors},
		route{pattern: "/system/descriptors.html", method: http.MethodGet, handler: h.systemDescriptorsHTML},
		route{pattern: "/system/version", method: http.MethodGet, handler: h.systemVersion},
		route{pattern: "/system/tenant", method: http.MethodGet, handler: h.systemTenant},
		route{pattern: "/system/status", method: http.MethodGet, handler: h.systemStatus},
		route{pattern: "/system/events", method: http.MethodPost, handler: h.systemEvents},
		route{pattern: "/system/data", method: http.MethodPost, handler: h.systemData},
		route{pattern: "/system/compute", method: http.MethodPost, handler: h.systemCompute},
		route{pattern: "/system/rpc", method: http.MethodPost, handler: h.handleChainRPC},
		route{pattern: "/neo/status", method: http.MethodGet, handler: h.neoStatus},
		route{pattern: "/neo/checkpoint", method: http.MethodGet, handler: h.neoCheckpoint},
		route{pattern: "/neo/blocks", method: http.MethodGet, handler: h.neoBlocks},
		route{pattern: "/neo/blocks/", method: http.MethodGet, handler: h.neoBlock},
		route{pattern: "/neo/snapshots", method: http.MethodGet, handler: h.neoSnapshots},
		route{pattern: "/neo/snapshots/", method: http.MethodGet, handler: h.neoSnapshot},
		route{pattern: "/neo/storage/", method: http.MethodGet, handler: h.neoStorage},
		route{pattern: "/neo/storage-diff/", method: http.MethodGet, handler: h.neoStorageDiff},
		route{pattern: "/neo/storage-summary/", method: http.MethodGet, handler: h.neoStorageSummary},
		route{pattern: "/auth/login", method: http.MethodPost, handler: h.login},
		route{pattern: "/auth/refresh", method: http.MethodPost, handler: h.refresh},
		route{pattern: "/auth/wallet/challenge", method: http.MethodPost, handler: h.walletChallenge},
		route{pattern: "/auth/wallet/login", method: http.MethodPost, handler: h.walletLogin},
		route{pattern: "/auth/whoami", method: http.MethodGet, handler: h.whoami},
		route{pattern: "/admin/audit", method: http.MethodGet, handler: h.adminAudit},
	)
	mux.HandleFunc("/accounts", h.accounts)
	mux.HandleFunc("/accounts/", h.accountResources)

	h.maybeMountAdminConfig(mux)
	h.mountExtraRoutes(mux)
	return mux
}

// mountExtraRoutes registers any additional routes configured via WithExtraRoutes.
func (h *handler) mountExtraRoutes(mux *http.ServeMux) {
	for _, registrar := range h.extraRoutes {
		if registrar != nil {
			registrar(mux)
		}
	}
}


func (h *handler) accounts(w http.ResponseWriter, r *http.Request) {
	tenant := tenantFromCtx(r.Context())
	if tenant == "" {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("tenant required"))
		return
	}
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			Owner    string            `json:"owner"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			core.WriteError(w, http.StatusBadRequest, err)
			return
		}
		if tenant != "" {
			if payload.Metadata == nil {
				payload.Metadata = map[string]string{}
			}
			payload.Metadata["tenant"] = tenant
		}

		acct, err := h.services.AccountsService().Create(r.Context(), payload.Owner, payload.Metadata)
		if err != nil {
			core.WriteError(w, http.StatusBadRequest, err)
			return
		}
		if tenant != "" {
			acct.Metadata["tenant"] = tenant
		}
		writeJSON(w, http.StatusCreated, acct)

	case http.MethodGet:
		accts, err := h.services.AccountsService().List(r.Context())
		if err != nil {
			core.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		filtered := make([]accounts.Account, 0, len(accts))
		for _, a := range accts {
			accountTenant := strings.TrimSpace(tenantFromMetadata(a.Metadata))
			if accountTenant == tenant {
				filtered = append(filtered, a)
			}
		}
		writeJSON(w, http.StatusOK, filtered)

	default:
		methodNotAllowed(w, http.MethodPost, http.MethodGet)
	}
}

func (h *handler) accountResources(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/accounts"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	accountID := parts[0]
	accountTenant, err := h.accountTenant(r.Context(), accountID)
	if err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		core.WriteError(w, status, err)
		return
	}
	requestTenant := strings.TrimSpace(tenantFromCtx(r.Context()))
	if requestTenant == "" {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden: tenant required"))
		return
	}
	if accountTenant != "" && accountTenant != requestTenant {
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("forbidden: tenant mismatch"))
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			acct, err := h.services.AccountsService().Get(r.Context(), accountID)
			if err != nil {
				core.WriteError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, acct)
		case http.MethodDelete:
			if err := h.services.AccountsService().Delete(r.Context(), accountID); err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, sql.ErrNoRows) {
					status = http.StatusNotFound
				}
				core.WriteError(w, status, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodDelete)
		}
		return
	}

	// Route to ServiceRouter for auto-discovered endpoints
	resourcePath := strings.Join(parts[1:], "/")
	if h.serviceRouter != nil && h.serviceRouter.Handle(w, r, accountID, resourcePath) {
		return
	}

	// No matching endpoint found
	w.WriteHeader(http.StatusNotFound)
}

// accountTenant returns the tenant string for an account (from metadata) or an empty string if none.
func (h *handler) accountTenant(ctx context.Context, accountID string) (string, error) {
	acct, err := h.services.AccountsService().Get(ctx, accountID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(tenantFromMetadata(acct.Metadata)), nil
}

func tenantFromMetadata(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta["tenant"])
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func (h *handler) accountOracle(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.services.OracleService() == nil {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("oracle service not configured"))
		return
	}

	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch rest[0] {
	case "sources":
		h.accountOracleSources(w, r, accountID, rest[1:])
	case "requests":
		h.accountOracleRequests(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountOracleSources(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			sources, err := h.services.OracleService().ListSources(r.Context(), accountID)
			if err != nil {
				core.WriteError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, sources)
		case http.MethodPost:
			var payload struct {
				Name        string            `json:"name"`
				URL         string            `json:"url"`
				Method      string            `json:"method"`
				Description string            `json:"description"`
				Headers     map[string]string `json:"headers"`
				Body        string            `json:"body"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
			src, err := h.services.OracleService().CreateSource(r.Context(), accountID, payload.Name, payload.URL, payload.Method, payload.Description, payload.Headers, payload.Body)
			if err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, src)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}

	sourceID := rest[0]
	src, err := h.services.OracleService().GetSource(r.Context(), sourceID)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	if src.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if len(rest) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, src)
	case http.MethodPatch:
		var payload struct {
			Name        *string           `json:"name"`
			URL         *string           `json:"url"`
			Method      *string           `json:"method"`
			Description *string           `json:"description"`
			Headers     map[string]string `json:"headers"`
			Body        *string           `json:"body"`
			Enabled     *bool             `json:"enabled"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			core.WriteError(w, http.StatusBadRequest, err)
			return
		}

		updated := src
		if payload.Name != nil || payload.URL != nil || payload.Method != nil || payload.Description != nil || payload.Headers != nil || payload.Body != nil {
			updated, err = h.services.OracleService().UpdateSource(r.Context(), sourceID, payload.Name, payload.URL, payload.Method, payload.Description, payload.Headers, payload.Body)
			if err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
		}
		if payload.Enabled != nil {
			updated, err = h.services.OracleService().SetSourceEnabled(r.Context(), sourceID, *payload.Enabled)
			if err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPatch)
	}
}

func (h *handler) accountOracleRequests(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			status := strings.TrimSpace(r.URL.Query().Get("status"))
			cursorID := strings.TrimSpace(r.URL.Query().Get("cursor"))
			limit, err := parseLimitParam(r.URL.Query().Get("limit"), 100)
			if err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
			fetchLimit := limit
			if cursorID != "" && fetchLimit < 500 {
				fetchLimit = 500
			}
			reqs, err := h.services.OracleService().ListRequests(r.Context(), accountID, fetchLimit, status)
			if err != nil {
				core.WriteError(w, http.StatusInternalServerError, err)
				return
			}
			if cursorID != "" {
				start := 0
				for i, req := range reqs {
					if req.ID == cursorID {
						start = i + 1
						break
					}
				}
				if start < len(reqs) {
					reqs = reqs[start:]
				} else {
					reqs = []oraclesvc.Request{}
				}
			}
			if len(reqs) > limit {
				reqs = reqs[:limit]
			}
			if len(reqs) == limit {
				w.Header().Set("X-Next-Cursor", reqs[len(reqs)-1].ID)
			}
			writeJSON(w, http.StatusOK, reqs)
		case http.MethodPost:
			var payload struct {
				DataSourceID string `json:"data_source_id"`
				Payload      string `json:"payload"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
			req, err := h.services.OracleService().CreateRequest(r.Context(), accountID, payload.DataSourceID, payload.Payload)
			if err != nil {
				core.WriteError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, req)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	requestID := rest[0]
	req, err := h.services.OracleService().GetRequest(r.Context(), requestID)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	if req.AccountID != accountID {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if len(rest) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, req)
	case http.MethodPatch:
		var payload struct {
			Status *string `json:"status"`
			Result *string `json:"result"`
			Error  *string `json:"error"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			core.WriteError(w, http.StatusBadRequest, err)
			return
		}
		if payload.Status == nil {
			core.WriteError(w, http.StatusBadRequest, fmt.Errorf("status is required"))
			return
		}
		status := strings.ToLower(strings.TrimSpace(*payload.Status))
		var updated oraclesvc.Request
		switch status {
		case "running":
			if !h.requireOracleRunner(r) {
				core.WriteError(w, http.StatusUnauthorized, fmt.Errorf("oracle runner token required"))
				return
			}
			updated, err = h.services.OracleService().MarkRunning(r.Context(), requestID)
		case "succeeded", "completed":
			if !h.requireOracleRunner(r) {
				core.WriteError(w, http.StatusUnauthorized, fmt.Errorf("oracle runner token required"))
				return
			}
			if payload.Result == nil {
				core.WriteError(w, http.StatusBadRequest, fmt.Errorf("result is required for succeeded status"))
				return
			}
			updated, err = h.services.OracleService().CompleteRequest(r.Context(), requestID, *payload.Result)
		case "failed":
			if !h.requireOracleRunner(r) {
				core.WriteError(w, http.StatusUnauthorized, fmt.Errorf("oracle runner token required"))
				return
			}
			errMsg := ""
			if payload.Error != nil {
				errMsg = *payload.Error
			}
			updated, err = h.services.OracleService().FailRequest(r.Context(), requestID, errMsg)
		case "retry":
			updated, err = h.services.OracleService().RetryRequest(r.Context(), requestID)
		default:
			core.WriteError(w, http.StatusBadRequest, fmt.Errorf("unsupported status %s", status))
			return
		}
		if err != nil {
			core.WriteError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func decodeJSON(body io.ReadCloser, dst interface{}) error {
	defer body.Close()
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}


func (h *handler) requireOracleRunner(r *http.Request) bool {
	tokens := h.services.OracleRunnerTokensValue()
	if len(tokens) == 0 {
		return true
	}
	header := strings.TrimSpace(r.Header.Get("X-Oracle-Runner-Token"))
	if header == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			header = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
		}
	}
	if header == "" {
		return false
	}
	for _, token := range tokens {
		if subtle.ConstantTimeCompare([]byte(header), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

func (h *handler) adminAudit(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil {
		writeJSON(w, http.StatusOK, []auditEntry{})
		return
	}
	limit, err := parseLimitParam(r.URL.Query().Get("limit"), 200)
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		val, convErr := strconv.Atoi(raw)
		if convErr != nil || val < 0 {
			core.WriteError(w, http.StatusBadRequest, fmt.Errorf("offset must be a non-negative integer"))
			return
		}
		offset = val
	}
	entries := h.audit.listLimit(limit + offset)
	q := r.URL.Query()
	user := strings.ToLower(strings.TrimSpace(q.Get("user")))
	role := strings.ToLower(strings.TrimSpace(q.Get("role")))
	tenant := strings.ToLower(strings.TrimSpace(q.Get("tenant")))
	method := strings.ToLower(strings.TrimSpace(q.Get("method")))
	pathContains := strings.ToLower(strings.TrimSpace(q.Get("contains")))
	statusStr := strings.TrimSpace(q.Get("status"))
	var statusFilter *int
	if statusStr != "" {
		if v, convErr := strconv.Atoi(statusStr); convErr == nil && v > 0 {
			statusFilter = &v
		} else {
			core.WriteError(w, http.StatusBadRequest, fmt.Errorf("status must be a positive integer"))
			return
		}
	}

	var filtered []auditEntry
	for _, e := range entries {
		if user != "" && strings.ToLower(e.User) != user {
			continue
		}
		if role != "" && strings.ToLower(e.Role) != role {
			continue
		}
		if tenant != "" && strings.ToLower(e.Tenant) != tenant {
			continue
		}
		if method != "" && strings.ToLower(e.Method) != method {
			continue
		}
		if pathContains != "" && !strings.Contains(strings.ToLower(e.Path), pathContains) {
			continue
		}
		if statusFilter != nil && e.Status != *statusFilter {
			continue
		}
		filtered = append(filtered, e)
	}
	if offset > 0 && offset < len(filtered) {
		filtered = filtered[offset:]
	} else if offset >= len(filtered) {
		filtered = []auditEntry{}
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	writeJSON(w, http.StatusOK, filtered)
}

// maybeMountAdminConfig mounts admin configuration endpoints if a store is provided.
func (h *handler) maybeMountAdminConfig(mux *http.ServeMux) {
	if h.adminConfigStore == nil {
		return
	}
	adminHandler := NewAdminConfigHandler(h.adminConfigStore)
	adminHandler.RegisterRoutes(mux)
}
