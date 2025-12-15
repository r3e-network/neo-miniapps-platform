package neooracle

import (
	"net"
	"net/url"
	"strings"
)

// URLAllowlist defines allowed URL prefixes for outbound fetches.
// If empty, no restriction is applied (not recommended for production).
type URLAllowlist struct {
	Prefixes []string
}

type allowlistEntry struct {
	scheme     string
	host       string
	port       string
	pathPrefix string
}

func parseURLAllowlistEntry(raw string) (allowlistEntry, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return allowlistEntry{}, false
	}

	// Prevent allowing entries that contain userinfo.
	if strings.Contains(raw, "@") {
		return allowlistEntry{}, false
	}

	entry := allowlistEntry{}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return allowlistEntry{}, false
		}
		if parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
			return allowlistEntry{}, false
		}

		entry.scheme = strings.ToLower(parsed.Scheme)
		entry.host = strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		entry.port = parsed.Port()
		entry.pathPrefix = normalizePathPrefix(parsed.Path)
		return entry, entry.host != ""
	}

	hostPort := raw
	path := ""
	if idx := strings.Index(raw, "/"); idx >= 0 {
		hostPort = raw[:idx]
		path = raw[idx:]
	}
	if hostPort == "" {
		return allowlistEntry{}, false
	}

	if strings.Contains(hostPort, ":") {
		host, port, err := net.SplitHostPort(hostPort)
		if err != nil || host == "" || port == "" {
			return allowlistEntry{}, false
		}
		entry.host = strings.ToLower(strings.TrimSuffix(host, "."))
		entry.port = port
	} else {
		entry.host = strings.ToLower(strings.TrimSuffix(hostPort, "."))
	}

	if entry.host == "" {
		return allowlistEntry{}, false
	}
	entry.pathPrefix = normalizePathPrefix(path)
	return entry, true
}

func normalizePathPrefix(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func hostMatches(host, allowedHost string) bool {
	if allowedHost == "" || host == "" {
		return false
	}
	if host == allowedHost {
		return true
	}
	return strings.HasSuffix(host, "."+allowedHost)
}

func pathHasPrefix(path, prefix string) bool {
	prefix = normalizePathPrefix(prefix)
	if prefix == "" {
		return true
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

func (a URLAllowlist) Allows(rawURL string) bool {
	if len(a.Prefixes) == 0 {
		return true
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	if parsed.User != nil {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	port := parsed.Port()
	path := parsed.Path
	if path == "" {
		path = "/"
	}

	for _, rawEntry := range a.Prefixes {
		entry, ok := parseURLAllowlistEntry(rawEntry)
		if !ok {
			continue
		}
		if entry.scheme != "" && entry.scheme != scheme {
			continue
		}
		if entry.port != "" && entry.port != port {
			continue
		}
		if !hostMatches(host, entry.host) {
			continue
		}
		if entry.pathPrefix != "" && !pathHasPrefix(path, entry.pathPrefix) {
			continue
		}
		return true
	}

	return false
}
