package headergate

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"sync"

	"github.com/R3E-Network/service_layer/internal/httputil"
	sllogging "github.com/R3E-Network/service_layer/internal/logging"
)

type auditEvent struct {
	ctx       context.Context
	reason    string
	method    string
	path      string
	vercelID string
	clientIP  string
	userAgent string
}

var (
	auditLogger = sllogging.NewFromEnv("gateway")
	auditOnce   sync.Once
	auditQueue  chan auditEvent
)

func enqueueAudit(event auditEvent) {
	auditOnce.Do(func() {
		auditQueue = make(chan auditEvent, 256)
		go func() {
			for auditEvent := range auditQueue {
				fields := map[string]interface{}{
					"audit":      true,
					"event_type": "header_gate_reject",
					"reason":     auditEvent.reason,
					"method":     auditEvent.method,
					"path":       auditEvent.path,
					"vercel_id": auditEvent.vercelID,
					"client_ip":  auditEvent.clientIP,
					"user_agent": auditEvent.userAgent,
				}
				auditLogger.WithContext(auditEvent.ctx).WithFields(fields).Warn("Header gate rejected request")
			}
		}()
	})

	select {
	case auditQueue <- event:
	default:
		// Never block request processing for audit logging.
	}
}

func Middleware(sharedSecret string) func(http.Handler) http.Handler {
	// Use a fixed-length digest so constant-time comparisons don't short-circuit on length.
	expectedSecretHash := sha256.Sum256([]byte(sharedSecret))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip health/metrics.
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			vercelID := r.Header.Get("X-Vercel-Id")
			receivedSecret := r.Header.Get("X-Shared-Secret")

			if vercelID == "" || receivedSecret == "" {
				enqueueAudit(auditEvent{
					ctx:       r.Context(),
					reason:    "missing_headers",
					method:    r.Method,
					path:      r.URL.Path,
					vercelID: vercelID,
					clientIP:  httputil.ClientIP(r),
					userAgent: r.UserAgent(),
				})
				httputil.Unauthorized(w, "unauthorized")
				return
			}

			receivedSecretHash := sha256.Sum256([]byte(receivedSecret))
			if subtle.ConstantTimeCompare(receivedSecretHash[:], expectedSecretHash[:]) != 1 {
				enqueueAudit(auditEvent{
					ctx:       r.Context(),
					reason:    "invalid_secret",
					method:    r.Method,
					path:      r.URL.Path,
					vercelID: vercelID,
					clientIP:  httputil.ClientIP(r),
					userAgent: r.UserAgent(),
				})
				httputil.Unauthorized(w, "unauthorized")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
