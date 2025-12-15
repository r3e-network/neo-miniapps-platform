// Package middleware provides HTTP middleware for the service layer
package middleware

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
type CORSMiddleware struct {
	cfg      CORSConfig
	allowAll bool
}

// CORSConfig configures CORS behavior.
type CORSConfig struct {
	AllowedOrigins         []string
	AllowedMethods         []string
	AllowedHeaders         []string
	ExposedHeaders         []string
	AllowCredentials       bool
	MaxAgeSeconds          int
	PreflightStatus        int
	RejectDisallowedOrigin bool
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(cfg *CORSConfig) *CORSMiddleware {
	cfgValue := CORSConfig{}
	if cfg != nil {
		cfgValue = *cfg
	}

	if len(cfgValue.AllowedMethods) == 0 {
		cfgValue.AllowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions}
	}
	if len(cfgValue.AllowedHeaders) == 0 {
		cfgValue.AllowedHeaders = []string{"Content-Type", "Authorization", "X-Trace-ID"}
	}
	if len(cfgValue.ExposedHeaders) == 0 {
		cfgValue.ExposedHeaders = []string{"X-Trace-ID"}
	}
	if cfgValue.MaxAgeSeconds == 0 {
		cfgValue.MaxAgeSeconds = 3600
	}
	if cfgValue.PreflightStatus == 0 {
		cfgValue.PreflightStatus = http.StatusNoContent
	}

	allowAll := false
	for _, origin := range cfgValue.AllowedOrigins {
		if origin == "*" {
			allowAll = true
			break
		}
	}

	return &CORSMiddleware{
		cfg:      cfgValue,
		allowAll: allowAll,
	}
}

// Handler returns the CORS middleware handler
func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := origin != "" && (m.allowAll || m.isOriginAllowed(origin))
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.cfg.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.cfg.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.cfg.ExposedHeaders, ", "))
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.cfg.MaxAgeSeconds))
			if m.cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		} else if origin != "" && m.cfg.RejectDisallowedOrigin {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(m.cfg.PreflightStatus)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if an origin is in the allowed list
func (m *CORSMiddleware) isOriginAllowed(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}

	for _, allowed := range m.cfg.AllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == origin {
			return true
		}
		if strings.HasPrefix(allowed, ".") {
			suffix := strings.TrimPrefix(allowed, ".")
			if suffix == "" {
				continue
			}
			if strings.HasSuffix(host, suffix) {
				idx := len(host) - len(suffix)
				if idx > 0 && host[idx-1] == '.' {
					return true
				}
			}
			continue
		}
	}
	return false
}
