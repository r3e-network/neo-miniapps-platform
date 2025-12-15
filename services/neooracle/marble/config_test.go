package neooracle

import "testing"

func TestURLAllowlist_AllowsEmptyList(t *testing.T) {
	allowlist := URLAllowlist{}
	if !allowlist.Allows("https://example.com/data") {
		t.Fatalf("expected empty allowlist to allow URL")
	}
}

func TestURLAllowlist_AllowsDomainAndSubdomain(t *testing.T) {
	allowlist := URLAllowlist{Prefixes: []string{"example.com"}}
	if !allowlist.Allows("https://api.example.com/v1") {
		t.Fatalf("expected allowlist to allow subdomain")
	}
	if allowlist.Allows("https://evil.com") {
		t.Fatalf("expected allowlist to block unrelated domain")
	}
}

func TestURLAllowlist_AllowsSchemeHostAndPathPrefix(t *testing.T) {
	allowlist := URLAllowlist{Prefixes: []string{"https://example.com/v1"}}
	if !allowlist.Allows("https://example.com/v1/resource") {
		t.Fatalf("expected allowlist to allow matching path prefix")
	}
	if allowlist.Allows("https://example.com/v2/resource") {
		t.Fatalf("expected allowlist to block non-matching path prefix")
	}
	if allowlist.Allows("https://example.com/v11/resource") {
		t.Fatalf("expected allowlist to block partial segment match")
	}
	if allowlist.Allows("http://example.com/v1/resource") {
		t.Fatalf("expected allowlist to enforce scheme when provided")
	}
}

func TestURLAllowlist_AllowsHostPathPrefixWithoutScheme(t *testing.T) {
	allowlist := URLAllowlist{Prefixes: []string{"example.com/v1"}}
	if !allowlist.Allows("https://example.com/v1/resource") {
		t.Fatalf("expected allowlist to allow host path prefix")
	}
	if allowlist.Allows("https://example.com/v11/resource") {
		t.Fatalf("expected allowlist to block partial segment match")
	}
}

func TestURLAllowlist_AllowsPortRestriction(t *testing.T) {
	allowlist := URLAllowlist{Prefixes: []string{"example.com:8443"}}
	if !allowlist.Allows("https://example.com:8443/v1") {
		t.Fatalf("expected allowlist to allow matching port")
	}
	if allowlist.Allows("https://example.com:443/v1") {
		t.Fatalf("expected allowlist to block non-matching port")
	}
}

func TestURLAllowlist_DoesNotAllowUserinfoBypass(t *testing.T) {
	allowlist := URLAllowlist{Prefixes: []string{"https://allowed.example"}}
	if allowlist.Allows("https://allowed.example@evil.example/path") {
		t.Fatalf("expected allowlist to block userinfo bypass")
	}
}

func TestParseURLAllowlistEntry_InvalidValues(t *testing.T) {
	if _, ok := parseURLAllowlistEntry(""); ok {
		t.Fatalf("expected empty entry to be invalid")
	}
	if _, ok := parseURLAllowlistEntry("http://"); ok {
		t.Fatalf("expected malformed URL entry to be invalid")
	}
	if _, ok := parseURLAllowlistEntry("user:pass@example.com"); ok {
		t.Fatalf("expected userinfo entry to be invalid")
	}
}

func TestPathHasPrefix_SegmentBoundaries(t *testing.T) {
	if !pathHasPrefix("/v1/resource", "/v1") {
		t.Fatalf("expected /v1 to match /v1/resource")
	}
	if pathHasPrefix("/v11/resource", "/v1") {
		t.Fatalf("expected /v1 not to match /v11/resource")
	}
	if !pathHasPrefix("/v1", "/v1") {
		t.Fatalf("expected exact match")
	}
}
