package httpapi

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/auth"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/jam"
	"github.com/R3E-Network/service_layer/internal/app/metrics"
	"github.com/R3E-Network/service_layer/internal/platform/database"
	"github.com/R3E-Network/service_layer/internal/platform/migrations"
	"github.com/R3E-Network/service_layer/internal/version"
)

// handler bundles HTTP endpoints for the application services.
type handler struct {
	app         *app.Application
	jamCfg      jam.Config
	jamAuth     []string
	jamStore    jam.PackageStore
	authManager authManager
	audit       *auditLog
}

type authManager interface {
	HasUsers() bool
	Authenticate(username, password string) (auth.User, error)
	Issue(user auth.User, ttl time.Duration) (string, time.Time, error)
	Validate(token string) (*auth.Claims, error)
	IssueWalletChallenge(wallet string, ttl time.Duration) (string, time.Time, error)
	VerifyWalletSignature(wallet, signature, pubKey string) (auth.User, error)
}

// NewHandler returns a mux exposing the core REST API.
func NewHandler(application *app.Application, jamCfg jam.Config, tokens []string, authMgr authManager, audit *auditLog) http.Handler {
	jamCfg.Normalize()
	h := &handler{app: application, jamCfg: jamCfg, jamAuth: tokens, authManager: authMgr, audit: audit}
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", h.health)
	mux.HandleFunc("/system/descriptors", h.systemDescriptors)
	mux.HandleFunc("/system/descriptors.html", h.systemDescriptorsHTML)
	mux.HandleFunc("/system/version", h.systemVersion)
	mux.HandleFunc("/system/status", h.systemStatus)
	mux.HandleFunc("/auth/login", h.login)
	mux.HandleFunc("/auth/wallet/challenge", h.walletChallenge)
	mux.HandleFunc("/auth/wallet/login", h.walletLogin)
	mux.HandleFunc("/auth/whoami", h.whoami)
	mux.HandleFunc("/accounts", h.accounts)
	mux.HandleFunc("/accounts/", h.accountResources)
	mux.HandleFunc("/admin/audit", h.adminAudit)

	h.maybeMountJAM(mux)
	return mux
}

func (h *handler) maybeMountJAM(mux *http.ServeMux) {
	if !h.jamCfg.Enabled {
		return
	}

	storeChoice := strings.ToLower(strings.TrimSpace(h.jamCfg.Store))
	var (
		pkgStore  jam.PackageStore  = jam.NewInMemoryStore()
		blobStore jam.PreimageStore = jam.NewMemPreimageStore()
	)

	if storeChoice == "postgres" {
		dsn := strings.TrimSpace(h.jamCfg.PGDSN)
		if dsn == "" {
			dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
		}
		if dsn != "" {
			db, err := database.Open(context.Background(), dsn)
			if err != nil {
				log.Printf("jam: postgres open failed, falling back to memory: %v", err)
			} else {
				if err := migrations.Apply(context.Background(), db); err != nil {
					log.Printf("jam: migration failed, falling back to memory: %v", err)
					db.Close()
				} else {
					pkgStore = jam.NewPGStore(db)
					blobStore = jam.NewPGPreimageStore(db)
				}
			}
		} else {
			log.Printf("jam: JAM_STORE=postgres set but no JAM_PG_DSN or DATABASE_URL; using memory")
		}
	}

	allowedTokens := h.jamCfg.AllowedTokens
	if len(allowedTokens) == 0 {
		allowedTokens = h.jamAuth
	}

	if memStore, ok := pkgStore.(*jam.InMemoryStore); ok {
		memStore.SetAccumulatorHash(h.jamCfg.AccumulatorHash)
		memStore.SetAccumulatorsEnabled(h.jamCfg.AccumulatorsEnabled)
	}
	if pgStore, ok := pkgStore.(*jam.PGStore); ok {
		pgStore.SetAccumulatorHash(h.jamCfg.AccumulatorHash)
		pgStore.SetAccumulatorsEnabled(h.jamCfg.AccumulatorsEnabled)
	}

	h.jamStore = pkgStore

	engine := jam.Engine{
		Preimages:   blobStore,
		Refiner:     jam.HashRefiner{},
		Attestors:   []jam.Attestor{jam.StaticAttestor{WorkerID: "local", Weight: 1}},
		Accumulator: jam.NoopAccumulator{},
		Threshold:   1,
	}
	coord := jam.Coordinator{
		Store:               pkgStore,
		Engine:              engine,
		AccumulatorsEnabled: h.jamCfg.AccumulatorsEnabled,
	}
	mux.Handle("/jam/", jam.NewHTTPHandler(pkgStore, blobStore, coord, h.jamCfg, allowedTokens))
}

func (h *handler) accounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			Owner    string            `json:"owner"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		acct, err := h.app.Accounts.Create(r.Context(), payload.Owner, payload.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, acct)

	case http.MethodGet:
		accts, err := h.app.Accounts.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, accts)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
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

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			acct, err := h.app.Accounts.Get(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, acct)
		case http.MethodDelete:
			if err := h.app.Accounts.Delete(r.Context(), accountID); err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, sql.ErrNoRows) {
					status = http.StatusNotFound
				}
				writeError(w, status, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	resource := parts[1]
	switch resource {
	case "functions":
		h.accountFunctions(w, r, accountID, parts[2:])
	case "triggers":
		h.accountTriggers(w, r, accountID)
	case "gasbank":
		h.accountGasBank(w, r, accountID, parts[2:])
	case "automation":
		h.accountAutomation(w, r, accountID, parts[2:])
	case "pricefeeds":
		h.accountPriceFeeds(w, r, accountID, parts[2:])
	case "datafeeds":
		h.accountDataFeeds(w, r, accountID, parts[2:])
	case "oracle":
		h.accountOracle(w, r, accountID, parts[2:])
	case "secrets":
		h.accountSecrets(w, r, accountID, parts[2:])
	case "random":
		h.accountRandom(w, r, accountID, parts[2:])
	case "cre":
		h.accountCRE(w, r, accountID, parts[2:])
	case "ccip":
		h.accountCCIP(w, r, accountID, parts[2:])
	case "vrf":
		h.accountVRF(w, r, accountID, parts[2:])
	case "datastreams":
		h.accountDataStreams(w, r, accountID, parts[2:])
	case "datalink":
		h.accountDataLink(w, r, accountID, parts[2:])
	case "dta":
		h.accountDTA(w, r, accountID, parts[2:])
	case "confcompute":
		h.accountConfCompute(w, r, accountID, parts[2:])
	case "workspace-wallets":
		h.accountWorkspaceWallets(w, r, accountID, parts[2:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) systemVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    version.Version,
		"commit":     version.GitCommit,
		"built_at":   version.BuildTime,
		"go_version": version.GoVersion,
	})
}

func (h *handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	jamStatus := map[string]any{
		"enabled":              h.jamCfg.Enabled,
		"store":                h.jamCfg.Store,
		"rate_limit_per_min":   h.jamCfg.RateLimitPerMinute,
		"max_preimage_bytes":   h.jamCfg.MaxPreimageBytes,
		"max_pending_packages": h.jamCfg.MaxPendingPackages,
		"auth_required":        h.jamCfg.AuthRequired,
		"legacy_list_response": h.jamCfg.LegacyListResponse,
		"accumulators_enabled": h.jamCfg.AccumulatorsEnabled,
		"accumulator_hash":     h.jamCfg.AccumulatorHash,
	}
	if h.jamCfg.AccumulatorsEnabled && h.jamStore != nil {
		if lister, ok := h.jamStore.(interface {
			AccumulatorRoots(context.Context) ([]jam.AccumulatorRoot, error)
		}); ok {
			if roots, err := lister.AccumulatorRoots(r.Context()); err == nil && len(roots) > 0 {
				jamStatus["accumulator_roots"] = roots
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"version": map[string]string{
			"version":    version.Version,
			"commit":     version.GitCommit,
			"built_at":   version.BuildTime,
			"go_version": version.GoVersion,
		},
		"services": h.app.Descriptors(),
		"jam":      jamStatus,
	})
}

func (h *handler) accountOracle(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.Oracle == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("oracle service not configured"))
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
			sources, err := h.app.Oracle.ListSources(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
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
				writeError(w, http.StatusBadRequest, err)
				return
			}
			src, err := h.app.Oracle.CreateSource(r.Context(), accountID, payload.Name, payload.URL, payload.Method, payload.Description, payload.Headers, payload.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, src)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	sourceID := rest[0]
	src, err := h.app.Oracle.GetSource(r.Context(), sourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
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
			writeError(w, http.StatusBadRequest, err)
			return
		}

		updated := src
		if payload.Name != nil || payload.URL != nil || payload.Method != nil || payload.Description != nil || payload.Headers != nil || payload.Body != nil {
			updated, err = h.app.Oracle.UpdateSource(r.Context(), sourceID, payload.Name, payload.URL, payload.Method, payload.Description, payload.Headers, payload.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		if payload.Enabled != nil {
			updated, err = h.app.Oracle.SetSourceEnabled(r.Context(), sourceID, *payload.Enabled)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
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
				writeError(w, http.StatusBadRequest, err)
				return
			}
			fetchLimit := limit
			if cursorID != "" && fetchLimit < 500 {
				fetchLimit = 500
			}
			reqs, err := h.app.Oracle.ListRequests(r.Context(), accountID, fetchLimit, status)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
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
					reqs = []oracle.Request{}
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
				writeError(w, http.StatusBadRequest, err)
				return
			}
			req, err := h.app.Oracle.CreateRequest(r.Context(), accountID, payload.DataSourceID, payload.Payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, req)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	requestID := rest[0]
	req, err := h.app.Oracle.GetRequest(r.Context(), requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
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
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if payload.Status == nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("status is required"))
			return
		}
		status := strings.ToLower(strings.TrimSpace(*payload.Status))
		var updated oracle.Request
		switch status {
		case "running":
			if !h.requireOracleRunner(r) {
				writeError(w, http.StatusUnauthorized, fmt.Errorf("oracle runner token required"))
				return
			}
			updated, err = h.app.Oracle.MarkRunning(r.Context(), requestID)
		case "succeeded", "completed":
			if !h.requireOracleRunner(r) {
				writeError(w, http.StatusUnauthorized, fmt.Errorf("oracle runner token required"))
				return
			}
			if payload.Result == nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("result is required for succeeded status"))
				return
			}
			updated, err = h.app.Oracle.CompleteRequest(r.Context(), requestID, *payload.Result)
		case "failed":
			if !h.requireOracleRunner(r) {
				writeError(w, http.StatusUnauthorized, fmt.Errorf("oracle runner token required"))
				return
			}
			errMsg := ""
			if payload.Error != nil {
				errMsg = *payload.Error
			}
			updated, err = h.app.Oracle.FailRequest(r.Context(), requestID, errMsg)
		case "retry":
			updated, err = h.app.Oracle.RetryRequest(r.Context(), requestID)
		default:
			writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported status %s", status))
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
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

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (h *handler) requireOracleRunner(r *http.Request) bool {
	tokens := h.app.OracleRunnerTokens
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

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) adminAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.audit == nil {
		writeJSON(w, http.StatusOK, []auditEntry{})
		return
	}
	limit, err := parseLimitParam(r.URL.Query().Get("limit"), 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		val, convErr := strconv.Atoi(raw)
		if convErr != nil || val < 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("offset must be a non-negative integer"))
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
			writeError(w, http.StatusBadRequest, fmt.Errorf("status must be a positive integer"))
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
