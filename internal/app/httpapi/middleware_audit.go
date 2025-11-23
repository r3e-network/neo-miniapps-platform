package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// wrapWithAudit logs basic request metadata for admin/user visibility.
func wrapWithAudit(next http.Handler, log *auditLog) http.Handler {
	if log == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		user, _ := r.Context().Value(ctxUserKey).(string)
		role, _ := r.Context().Value(ctxRoleKey).(string)
		tenant, _ := r.Context().Value(ctxTenantKey).(string)
		log.add(auditEntry{
			Time:       start.UTC(),
			User:       user,
			Role:       role,
			Tenant:     tenant,
			Path:       r.URL.Path,
			Method:     r.Method,
			Status:     rec.status,
			RemoteAddr: clientIP(r),
			UserAgent:  r.UserAgent(),
		})
	})
}

func clientIP(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if h != "" {
		parts := strings.Split(h, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host := strings.TrimSpace(r.RemoteAddr)
	return host
}
