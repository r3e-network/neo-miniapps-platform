package oracle

// URLAllowlist defines allowed URL prefixes for outbound fetches.
// If empty, no restriction is applied (not recommended for production).
type URLAllowlist struct {
	Prefixes []string
}

func (a URLAllowlist) Allows(url string) bool {
	if len(a.Prefixes) == 0 {
		return true
	}
	for _, p := range a.Prefixes {
		if len(url) >= len(p) && url[:len(p)] == p {
			return true
		}
	}
	return false
}
