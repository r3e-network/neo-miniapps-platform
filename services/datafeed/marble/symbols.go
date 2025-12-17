package neofeeds

import "strings"

// normalizePair normalizes user-facing pair symbols.
//
// Canonical form is `BASE-QUOTE` (e.g. `BTC-USD`). For backward compatibility,
// the service also accepts legacy `BASE/QUOTE` (and `BASE_QUOTE`) inputs.
func normalizePair(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	normalized := strings.ToUpper(trimmed)
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.Trim(normalized, "-")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}

	return normalized
}

func parseBaseQuoteFromPair(value string) (base, quote string) {
	pair := normalizePair(value)
	if pair == "" {
		return "", ""
	}

	if strings.Contains(pair, "-") {
		parts := strings.Split(pair, "-")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return strings.ToLower(parts[0]), strings.ToLower(parts[1])
		}
	}

	// Legacy heuristic for delimiter-less symbols (e.g. BTCUSDT).
	if len(pair) >= 6 {
		return strings.ToLower(pair[:3]), strings.ToLower(pair[3:])
	}

	return "", ""
}
