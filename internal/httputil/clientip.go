package httputil

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the best-effort client IP address from the request.
//
// Security model:
//   - If the direct peer is on a private network (typical for ingress/proxy),
//     trust X-Forwarded-For / X-Real-IP.
//   - If the request comes directly from the internet, ignore spoofable forwarded
//     headers and fall back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	remoteIP := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remoteIP); err == nil {
		remoteIP = host
	}

	parsedRemote := net.ParseIP(remoteIP)
	trustForwarded := parsedRemote != nil && (parsedRemote.IsPrivate() || parsedRemote.IsLoopback() || parsedRemote.IsLinkLocalUnicast())

	if trustForwarded {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				candidate := strings.TrimSpace(parts[0])
				if host, _, err := net.SplitHostPort(candidate); err == nil {
					candidate = host
				}
				if candidate != "" {
					return candidate
				}
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if host, _, err := net.SplitHostPort(xri); err == nil {
				xri = host
			}
			if xri != "" {
				return xri
			}
		}
	}

	return remoteIP
}
