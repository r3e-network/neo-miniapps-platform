package jam

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewHTTPHandler returns a ServeMux exposing minimal JAM endpoints.
func NewHTTPHandler(store PackageStore, preimages PreimageStore, coord Coordinator, cfg Config, allowedTokens []string) http.Handler {
	cfg.Normalize()
	h := &httpHandler{
		store:        store,
		preimages:    preimages,
		coord:        coord,
		cfg:          cfg,
		allowed:      make(map[string]struct{}),
		rateLimiter:  newTokenLimiter(cfg.RateLimitPerMinute),
		pendingLimit: cfg.MaxPendingPackages,
	}
	for _, t := range allowedTokens {
		h.allowed[t] = struct{}{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jam/status", h.statusHandler)
	mux.HandleFunc("/jam/preimages/", h.preimagesHandler)
	mux.HandleFunc("/jam/packages", h.packagesHandler)
	mux.HandleFunc("/jam/packages/", h.packageResource)
	mux.HandleFunc("/jam/reports", h.reportsHandler)
	mux.HandleFunc("/jam/process", h.processHandler)
	return mux
}

type httpHandler struct {
	store        PackageStore
	preimages    PreimageStore
	coord        Coordinator
	cfg          Config
	allowed      map[string]struct{}
	rateLimiter  *tokenLimiter
	pendingLimit int
}

func (h *httpHandler) statusHandler(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if !h.checkRate(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	serviceID := strings.TrimSpace(r.URL.Query().Get("service_id"))
	resp := map[string]any{
		"enabled":              h.cfg.Enabled,
		"store":                h.cfg.Store,
		"rate_limit_per_min":   h.cfg.RateLimitPerMinute,
		"max_preimage_bytes":   h.cfg.MaxPreimageBytes,
		"max_pending_packages": h.cfg.MaxPendingPackages,
		"auth_required":        h.cfg.AuthRequired,
		"legacy_list_response": h.cfg.LegacyListResponse,
		"accumulators_enabled": h.cfg.AccumulatorsEnabled,
		"accumulator_hash":     h.cfg.AccumulatorHash,
	}
	if serviceID != "" {
		root, err := h.store.AccumulatorRoot(r.Context(), serviceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		resp["accumulator_root"] = root
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *httpHandler) appendReceiptIfNeeded(ctx context.Context, obj any) (Receipt, error) {
	if !h.cfg.AccumulatorsEnabled {
		return Receipt{}, nil
	}
	recorder, ok := h.store.(interface {
		AppendReceipt(context.Context, ReceiptInput) (Receipt, error)
	})
	if !ok {
		return Receipt{}, nil
	}
	switch v := obj.(type) {
	case WorkPackage:
		return recorder.AppendReceipt(ctx, ReceiptInput{
			Hash:         v.ID,
			ServiceID:    v.ServiceID,
			EntryType:    ReceiptTypePackage,
			Status:       string(v.Status),
			ProcessedAt:  time.Now().UTC(),
			MetadataHash: packageMetadataHash(v, accumulatorHash(h.store)),
		})
	case WorkReport:
		return recorder.AppendReceipt(ctx, ReceiptInput{
			Hash:         v.RefineOutputHash,
			ServiceID:    v.ServiceID,
			EntryType:    ReceiptTypeReport,
			Status:       string(PackageStatusApplied),
			ProcessedAt:  v.CreatedAt,
			MetadataHash: reportMetadataHash(v, accumulatorHash(h.store)),
		})
	default:
		return Receipt{}, nil
	}
}

func (h *httpHandler) preimagesHandler(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	hash := strings.TrimPrefix(r.URL.Path, "/jam/preimages/")
	if hash == "" {
		writeError(w, http.StatusBadRequest, Err("missing preimage hash"))
		return
	}
	switch r.Method {
	case http.MethodPut:
		if !h.checkRate(w, r) {
			return
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if h.cfg.MaxPreimageBytes > 0 && int64(len(data)) > h.cfg.MaxPreimageBytes {
			writeError(w, http.StatusRequestEntityTooLarge, errors.New("preimage too large"))
			return
		}
		mediaType := r.Header.Get("Content-Type")
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		meta, err := h.preimages.Put(r.Context(), hash, mediaType, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, meta)
	case http.MethodHead:
		if !h.checkRate(w, r) {
			return
		}
		meta, err := h.preimages.Stat(r.Context(), hash)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		w.Header().Set("Content-Type", meta.MediaType)
		if meta.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
		}
		w.Header().Set("X-Preimage-Hash", meta.Hash)
		w.Header().Set("X-Preimage-Size", strconv.FormatInt(meta.Size, 10))
		w.Header().Set("X-Preimage-Media-Type", meta.MediaType)
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		if !h.checkRate(w, r) {
			return
		}
		if strings.HasSuffix(r.URL.Path, "/meta") {
			hash = strings.TrimSuffix(hash, "/meta")
			meta, err := h.preimages.Stat(r.Context(), hash)
			if err != nil {
				status := http.StatusInternalServerError
				if err == ErrNotFound {
					status = http.StatusNotFound
				}
				writeError(w, status, err)
				return
			}
			writeJSON(w, http.StatusOK, meta)
			return
		}
		meta, err := h.preimages.Stat(r.Context(), hash)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		rc, err := h.preimages.Get(r.Context(), hash)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		defer rc.Close()
		w.Header().Set("Content-Type", meta.MediaType)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *httpHandler) packagesHandler(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	switch r.Method {
	case http.MethodPost:
		if !h.checkRate(w, r) {
			return
		}
		var pkg WorkPackage
		if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if pkg.ID == "" {
			pkg.ID = uuid.NewString()
		}
		if pkg.Status == "" {
			pkg.Status = PackageStatusPending
		}
		if pkg.CreatedAt.IsZero() {
			pkg.CreatedAt = time.Now().UTC()
		}
		for i := range pkg.Items {
			if pkg.Items[i].ID == "" {
				pkg.Items[i].ID = uuid.NewString()
			}
			pkg.Items[i].PackageID = pkg.ID
		}
		if h.pendingLimit > 0 {
			if count, err := h.store.PendingCount(r.Context()); err == nil && count >= h.pendingLimit {
				writeError(w, http.StatusConflict, errors.New("pending package limit reached"))
				return
			}
		}
		if err := h.store.EnqueuePackage(r.Context(), pkg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if includeReceipt := strings.EqualFold(r.URL.Query().Get("include_receipt"), "true"); includeReceipt {
			resp := map[string]any{"package": pkg}
			if rcpt, err := h.appendReceiptIfNeeded(r.Context(), pkg); err == nil && rcpt.Hash != "" {
				resp["receipt"] = rcpt
			}
			writeJSON(w, http.StatusCreated, resp)
			return
		}
		writeJSON(w, http.StatusCreated, pkg)
	case http.MethodGet:
		if !h.checkRate(w, r) {
			return
		}
		filter := PackageFilter{
			Status:    PackageStatus(r.URL.Query().Get("status")),
			ServiceID: r.URL.Query().Get("service_id"),
		}
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				filter.Limit = parsed
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				filter.Offset = parsed
			}
		}
		pkgs, err := h.store.ListPackages(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		resp := map[string]any{
			"items": pkgs,
		}
		if includeReceipt := strings.EqualFold(r.URL.Query().Get("include_receipt"), "true"); includeReceipt {
			for i := range pkgs {
				rcpt, err := h.appendReceiptIfNeeded(r.Context(), pkgs[i])
				if err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
				if rcpt.Hash != "" {
					if resp["receipts"] == nil {
						resp["receipts"] = []Receipt{rcpt}
					} else {
						resp["receipts"] = append(resp["receipts"].([]Receipt), rcpt)
					}
				}
			}
		}
		if !h.cfg.LegacyListResponse {
			resp["next_offset"] = filter.Offset + len(pkgs)
			writeJSON(w, http.StatusOK, resp)
		} else {
			writeJSON(w, http.StatusOK, pkgs)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *httpHandler) packageResource(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if !h.checkRate(w, r) {
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/jam/packages/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pkgID := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		pkg, err := h.store.GetPackage(r.Context(), pkgID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		if includeReceipt := strings.EqualFold(r.URL.Query().Get("include_receipt"), "true"); includeReceipt {
			resp := map[string]any{"package": pkg}
			if rcpt, err := h.appendReceiptIfNeeded(r.Context(), pkg); err == nil && rcpt.Hash != "" {
				resp["receipt"] = rcpt
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeJSON(w, http.StatusOK, pkg)
		return
	}

	if parts[1] == "report" && r.Method == http.MethodGet {
		report, attns, err := h.store.GetReportByPackage(r.Context(), pkgID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == ErrNotFound {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"report":       report,
			"attestations": attns,
		})
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *httpHandler) processHandler(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if !h.checkRate(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ok, err := h.coord.ProcessNext(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"processed": true})
}

func (h *httpHandler) reportsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if !h.checkRate(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	filter := ReportFilter{
		ServiceID: r.URL.Query().Get("service_id"),
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			filter.Limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			filter.Offset = parsed
		}
	}
	reports, err := h.store.ListReports(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := map[string]any{
		"items": reports,
	}
	if includeReceipt := strings.EqualFold(r.URL.Query().Get("include_receipt"), "true"); includeReceipt {
		var rcpts []Receipt
		for _, rpt := range reports {
			rcpt, err := h.appendReceiptIfNeeded(r.Context(), rpt)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if rcpt.Hash != "" {
				rcpts = append(rcpts, rcpt)
			}
		}
		if len(rcpts) > 0 {
			resp["receipts"] = rcpts
		}
	}
	if !h.cfg.LegacyListResponse {
		resp["next_offset"] = filter.Offset + len(reports)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	code := "jam_internal"
	switch status {
	case http.StatusBadRequest:
		code = "jam_bad_request"
	case http.StatusUnauthorized:
		code = "jam_auth_missing"
	case http.StatusForbidden:
		code = "jam_auth_forbidden"
	case http.StatusNotFound:
		code = "jam_not_found"
	case http.StatusConflict:
		code = "jam_pending_limit"
	case http.StatusRequestEntityTooLarge:
		code = "jam_preimage_too_large"
	case http.StatusTooManyRequests:
		code = "jam_rate_limit"
	}
	writeJSON(w, status, map[string]string{"error": err.Error(), "code": code})
}

// authorize enforces JAM-specific auth if configured.
func (h *httpHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if !h.cfg.AuthRequired {
		return true
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, errors.New("missing bearer token"))
		return false
	}
	if len(h.allowed) > 0 {
		if _, ok := h.allowed[token]; !ok {
			writeError(w, http.StatusForbidden, errors.New("token not allowed for JAM"))
			return false
		}
	}
	return true
}

// checkRate enforces per-token rate limiting when configured.
func (h *httpHandler) checkRate(w http.ResponseWriter, r *http.Request) bool {
	if h.rateLimiter == nil {
		return true
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = r.RemoteAddr
	}
	allowed, retry := h.rateLimiter.Allow(token)
	if !allowed {
		if retry > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
		}
		writeError(w, http.StatusTooManyRequests, errors.New("rate limit exceeded"))
		return false
	}
	return true
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}
	return ""
}

type tokenLimiter struct {
	limit   int
	mu      sync.Mutex
	buckets map[string]bucket
}

type bucket struct {
	count int
	start time.Time
}

func newTokenLimiter(limit int) *tokenLimiter {
	if limit <= 0 {
		return nil
	}
	return &tokenLimiter{limit: limit, buckets: make(map[string]bucket)}
}

// Allow returns whether a token may proceed and the retry-after duration if limited.
func (t *tokenLimiter) Allow(key string) (bool, time.Duration) {
	if t == nil {
		return true, 0
	}
	now := time.Now()
	window := now.Truncate(time.Minute)

	t.mu.Lock()
	defer t.mu.Unlock()

	b := t.buckets[key]
	if b.start.Before(window) {
		b = bucket{start: window, count: 0}
	}
	if b.count >= t.limit {
		retry := b.start.Add(time.Minute).Sub(now)
		t.buckets[key] = b
		return false, retry
	}
	b.count++
	t.buckets[key] = b
	return true, 0
}
