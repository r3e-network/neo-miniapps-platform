package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/gorilla/mux"
)

// =============================================================================
// Gas Bank Handlers
// =============================================================================

func getGasBankAccountHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		account, err := db.GetOrCreateGasBankAccount(r.Context(), userID)
		if err != nil {
			jsonError(w, "failed to get account", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(account)
	}
}

func createDepositHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")

		var req struct {
			Amount      int64  `json:"amount"`
			FromAddress string `json:"from_address"`
			TxHash      string `json:"tx_hash"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(deposit)
	}
}

func listDepositsHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		deposits, err := db.GetDepositRequests(r.Context(), userID, 50)
		if err != nil {
			jsonError(w, "failed to get deposits", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deposits)
	}
}

func listTransactionsHandler(db *database.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(txs)
	}
}

// Service endpoint configuration from environment
var serviceEndpoints = map[string]string{
	"vrf":          getEnvOrDefault("VRF_SERVICE_URL", "http://localhost:8081"),
	"mixer":        getEnvOrDefault("MIXER_SERVICE_URL", "http://localhost:8082"),
	"datafeeds":    getEnvOrDefault("DATAFEEDS_SERVICE_URL", "http://localhost:8083"),
	"automation":   getEnvOrDefault("AUTOMATION_SERVICE_URL", "http://localhost:8084"),
	"confidential": getEnvOrDefault("CONFIDENTIAL_SERVICE_URL", "http://localhost:8085"),
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func proxyHandler(service string) http.HandlerFunc {
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

		// Customize the director to set the path
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			path := mux.Vars(r)["path"]
			if path != "" {
				req.URL.Path = "/" + path
			} else {
				req.URL.Path = "/"
			}
			req.URL.RawQuery = r.URL.RawQuery

			// Forward authentication headers
			if userID := r.Header.Get("X-User-ID"); userID != "" {
				req.Header.Set("X-User-ID", userID)
			}
			if auth := r.Header.Get("Authorization"); auth != "" {
				req.Header.Set("Authorization", auth)
			}

			// Set forwarding headers
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Gateway-Service", service)
		}

		// Custom error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "service unavailable",
				"service": service,
				"detail":  err.Error(),
			})
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

// proxyWithBody handles POST/PUT requests with body forwarding
func proxyWithBody(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetURL, ok := serviceEndpoints[service]
		if !ok {
			jsonError(w, "unknown service", http.StatusNotFound)
			return
		}

		path := mux.Vars(r)["path"]
		fullURL := targetURL
		if path != "" {
			fullURL = strings.TrimRight(targetURL, "/") + "/" + path
		}
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}

		// Create new request
		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, fullURL, r.Body)
		if err != nil {
			jsonError(w, "failed to create proxy request", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}
		proxyReq.Header.Set("X-Gateway-Service", service)

		// Send request
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			jsonError(w, "service unavailable: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Set("X-Proxied-By", "gateway")
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		io.Copy(w, resp.Body)
	}
}
