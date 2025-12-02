package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/pkg/metrics"
	engine "github.com/R3E-Network/service_layer/system/os/core"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

type rpcRequest struct {
	Chain  string          `json:"chain"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (h *handler) handleChainRPC(w http.ResponseWriter, r *http.Request) {
	if h.rpcEngines == nil {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("rpc hub not configured"))
		return
	}
	rpcs := h.rpcEngines()
	if len(rpcs) == 0 {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("rpc hub not available"))
		return
	}
	start := time.Now()
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.RecordRPCCall("unknown", "bad_request", time.Since(start))
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("decode rpc request: %w", err))
		return
	}
	req.Chain = strings.TrimSpace(req.Chain)
	req.Method = strings.TrimSpace(req.Method)
	if req.Chain == "" || req.Method == "" {
		metrics.RecordRPCCall(req.Chain, "bad_request", time.Since(start))
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("chain and method are required"))
		return
	}
	if h.rpcPolicy != nil && !h.rpcPolicy.methodAllowed(req.Chain, req.Method) {
		metrics.RecordRPCCall(req.Chain, "blocked", time.Since(start))
		core.WriteError(w, http.StatusForbidden, fmt.Errorf("rpc method %q not allowed for chain %q", req.Method, req.Chain))
		return
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  req.Method,
	}
	if len(req.Params) > 0 && string(req.Params) != "null" {
		var params any
		if err := json.Unmarshal(req.Params, &params); err != nil {
			metrics.RecordRPCCall(req.Chain, "bad_request", time.Since(start))
			core.WriteError(w, http.StatusBadRequest, fmt.Errorf("invalid params: %w", err))
			return
		}
		payload["params"] = params
	}

	if allowed, retry, reason := h.checkRPCPolicy(r, req.Chain); !allowed {
		status := http.StatusForbidden
		if reason == "tenant-required" {
			status = http.StatusForbidden
		} else {
			status = http.StatusTooManyRequests
		}
		if retry > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retry.Seconds()))
		}
		metrics.RecordRPCCall(req.Chain, "limited", time.Since(start))
		core.WriteError(w, status, fmt.Errorf("rpc request blocked: %s", reason))
		return
	}

	resp, err := h.rpcFanout(r.Context(), rpcs, req.Chain, payload)
	status := "ok"
	if err != nil {
		status = "error"
	}
	metrics.RecordRPCCall(req.Chain, status, time.Since(start))
	if err != nil {
		var cfgErr rpcConfigError
		if errors.As(err, &cfgErr) {
			core.WriteError(w, http.StatusBadRequest, err)
			return
		}
		core.WriteError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// rpcFanout sends a JSON-RPC request to the configured chain endpoint via the RPC engine.
// Minimal implementation: uses RPC endpoints exposed by the engine.
func (h *handler) rpcFanout(ctx context.Context, rpcs []engine.RPCEngine, chain string, payload map[string]any) (map[string]any, error) {
	chainKey := strings.ToLower(strings.TrimSpace(chain))
	var chainURLs []string
	var available []string
	for _, rpcEng := range rpcs {
		if rpcEng == nil {
			continue
		}
		endpoints := rpcEng.RPCEndpoints()
		for k, v := range endpoints {
			k = strings.ToLower(strings.TrimSpace(k))
			if k != "" {
				available = append(available, k)
			}
			if k == chainKey && strings.TrimSpace(v) != "" {
				chainURLs = append(chainURLs, strings.TrimSpace(v))
			}
		}
	}
	if len(chainURLs) == 0 {
		if len(available) == 0 {
			return nil, rpcConfigError{msg: "rpc endpoints not configured"}
		}
		sort.Strings(available)
		return nil, rpcConfigError{msg: fmt.Sprintf("no rpc endpoint configured for chain %q (available: %s)", chain, strings.Join(available, ","))}
	}

	url := h.selectRPCEndpoint(chainKey, chainURLs)

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("build rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rpc request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rpc status %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode rpc response: %w", err)
	}
	return out, nil
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, strings.TrimSpace(k))
	}
	sort.Strings(keys)
	return keys
}

type rpcConfigError struct {
	msg string
}

func (e rpcConfigError) Error() string { return e.msg }

func (h *handler) checkRPCPolicy(r *http.Request, chain string) (bool, time.Duration, string) {
	if h == nil || h.rpcPolicy == nil {
		return true, 0, ""
	}
	tenant := tenantFromCtx(r.Context())
	token := tokenFromCtx(r.Context())
	return h.rpcPolicy.allow(tenant, token)
}

// selectRPCEndpoint rotates through the provided endpoints per chain to avoid sticky-first routing.
func (h *handler) selectRPCEndpoint(chain string, urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	if len(urls) == 1 || h == nil {
		return urls[0]
	}
	h.rpcMu.Lock()
	defer h.rpcMu.Unlock()
	if h.rpcSeq == nil {
		h.rpcSeq = make(map[string]int)
	}
	idx := h.rpcSeq[chain] % len(urls)
	h.rpcSeq[chain] = (h.rpcSeq[chain] + 1) % len(urls)
	return urls[idx]
}
