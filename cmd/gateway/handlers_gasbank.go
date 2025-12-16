package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/infrastructure/database"
	internalhttputil "github.com/R3E-Network/service_layer/infrastructure/httputil"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
)

// =============================================================================
// Gas Bank Handlers
// =============================================================================

func getGasBankAccountHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		account, err := db.GetOrCreateGasBankAccount(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get account", http.StatusInternalServerError)
			return
		}

		internalhttputil.WriteJSON(w, http.StatusOK, account)
	}
}

func createDepositHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		var req struct {
			Amount      int64  `json:"amount"`
			FromAddress string `json:"from_address"`
			TxHash      string `json:"tx_hash"`
		}
		if !internalhttputil.DecodeJSON(w, r, &req) {
			return
		}

		account, err := db.GetOrCreateGasBankAccount(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get account", http.StatusInternalServerError)
			return
		}

		deposit := &database.DepositRequest{
			UserID:                userID,
			AccountID:             account.ID,
			Amount:                req.Amount,
			TxHash:                req.TxHash,
			FromAddress:           req.FromAddress,
			Status:                "pending",
			RequiredConfirmations: 1,
			ExpiresAt:             time.Now().Add(24 * time.Hour),
		}

		if err := db.CreateDepositRequest(r.Context(), deposit); err != nil {
			jsonError(w, "failed to create deposit", http.StatusInternalServerError)
			return
		}

		internalhttputil.WriteJSON(w, http.StatusCreated, deposit)
	}
}

func listDepositsHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		deposits, err := db.GetDepositRequests(r.Context(), userID, 50)
		if err != nil {
			jsonError(w, "failed to get deposits", http.StatusInternalServerError)
			return
		}

		internalhttputil.WriteJSON(w, http.StatusOK, deposits)
	}
}

func listTransactionsHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			jsonError(w, "missing user id", http.StatusUnauthorized)
			return
		}

		account, err := db.GetGasBankAccount(r.Context(), userID)
		if err != nil {
			jsonError(w, "account not found", http.StatusNotFound)
			return
		}

		txs, err := db.GetGasBankTransactions(r.Context(), account.ID, 50)
		if err != nil {
			jsonError(w, "failed to get transactions", http.StatusInternalServerError)
			return
		}

		internalhttputil.WriteJSON(w, http.StatusOK, txs)
	}
}

// Service endpoint configuration from environment
var serviceEndpoints = map[string]string{
	// Canonical service IDs.
	"neofeeds":    getEnvFirst([]string{"DATAFEEDS_SERVICE_URL", "NEOFEEDS_SERVICE_URL"}, "http://localhost:8083"),
	"neoflow":     getEnvFirst([]string{"AUTOMATION_SERVICE_URL", "NEOFLOW_SERVICE_URL"}, "http://localhost:8084"),
	"neoaccounts": getEnvFirst([]string{"ACCOUNTPOOL_SERVICE_URL", "NEOACCOUNTS_SERVICE_URL"}, "http://localhost:8085"),
	"neocompute":  getEnvFirst([]string{"CONFIDENTIAL_SERVICE_URL", "NEOCOMPUTE_SERVICE_URL"}, "http://localhost:8086"),
	"neooracle":   getEnvFirst([]string{"ORACLE_SERVICE_URL", "NEOORACLE_SERVICE_URL"}, "http://localhost:8088"),

	// Backward-compatible aliases.
	"oracle": getEnvFirst([]string{"ORACLE_SERVICE_URL", "NEOORACLE_SERVICE_URL"}, "http://localhost:8088"),
}

func getEnvFirst(keys []string, defaultVal string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return defaultVal
}

func proxyHandler(service string, m *marble.Marble) http.HandlerFunc {
	useMTLS := m != nil && m.TLSConfig() != nil
	transport := http.DefaultTransport
	if useMTLS {
		if base, ok := http.DefaultTransport.(*http.Transport); ok {
			cloned := base.Clone()
			cloned.TLSClientConfig = m.TLSConfig().Clone()
			transport = cloned
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		targetURL, ok := serviceEndpoints[service]
		if !ok {
			jsonError(w, "unknown service", http.StatusNotFound)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			jsonError(w, "invalid service URL", http.StatusInternalServerError)
			return
		}

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = transport

		// Use Rewrite so we fully control forwarded headers and avoid the
		// automatic X-Forwarded-For append logic in Director mode.
		proxy.Director = nil
		proxy.Rewrite = func(pr *httputil.ProxyRequest) {
			// Never forward identity/privileged headers provided by public clients.
			// The gateway is the trust boundary: it authenticates the user, then
			// forwards only the derived identity and tracing metadata to internal
			// services.
			pr.Out.Header.Del("X-Service-ID")
			pr.Out.Header.Del("X-Service-Token")
			pr.Out.Header.Del("X-User-ID")
			pr.Out.Header.Del("X-User-Role")
			pr.Out.Header.Del("X-API-Key")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Del("Cookie")

			// Strip spoofable proxy headers before the default reverse proxy
			// director appends forwarding information.
			pr.Out.Header.Del("X-Forwarded-For")
			pr.Out.Header.Del("X-Real-IP")
			pr.Out.Header.Del("Forwarded")
			pr.Out.Header.Del("X-Forwarded-Host")
			pr.Out.Header.Del("X-Forwarded-Proto")

			upstream := *target
			if useMTLS {
				upstream.Scheme = "https"
			}
			pr.SetURL(&upstream)
			pr.Out.Host = pr.In.Host

			// When running inside MarbleRun with injected TLS credentials, enforce
			// HTTPS+mTLS for all upstream service calls regardless of the configured
			// URL scheme. This keeps service discovery flexible while ensuring the
			// security properties expected by the mesh.
			path := mux.Vars(pr.In)["path"]
			if path != "" {
				pr.Out.URL.Path = "/" + path
			} else {
				pr.Out.URL.Path = "/"
			}
			pr.Out.URL.RawPath = ""

			// Forward authentication headers
			if userID := pr.In.Header.Get("X-User-ID"); userID != "" {
				pr.Out.Header.Set("X-User-ID", userID)
			}
			if role := pr.In.Header.Get("X-User-Role"); role != "" {
				pr.Out.Header.Set("X-User-Role", role)
			}
			if traceID := pr.In.Header.Get("X-Trace-ID"); traceID != "" {
				pr.Out.Header.Set("X-Trace-ID", traceID)
			}

			// Forward the derived client IP (sanitized). This preserves accurate
			// audit logging and rate limiting inside the service mesh while
			// preventing spoofing from untrusted internet clients.
			if clientIP := internalhttputil.ClientIP(pr.In); clientIP != "" {
				pr.Out.Header.Set("X-Forwarded-For", clientIP)
				pr.Out.Header.Set("X-Real-IP", clientIP)
			}

			// Set forwarding headers
			pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
			proto := pr.In.Header.Get("X-Forwarded-Proto")
			if proto == "" {
				if pr.In.TLS != nil {
					proto = "https"
				} else {
					proto = "http"
				}
			}
			pr.Out.Header.Set("X-Forwarded-Proto", proto)
			pr.Out.Header.Set("X-Gateway-Service", service)
		}

		// Custom error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("gateway proxy error: service=%s err=%v", service, err)

			details := map[string]string{
				"service": service,
			}
			if !internalhttputil.StrictIdentityMode() {
				details["detail"] = err.Error()
			}
			internalhttputil.WriteErrorResponse(w, r, http.StatusBadGateway, "BAD_GATEWAY", "service unavailable", details)
		}

		// Custom response modifier to handle streaming
		proxy.ModifyResponse = func(resp *http.Response) error {
			// Add CORS headers if needed
			resp.Header.Set("X-Proxied-By", "gateway")
			return nil
		}

		// Forward the request
		proxy.ServeHTTP(w, r)
	}
}
