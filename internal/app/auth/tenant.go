package auth

import "strings"

// TenantBinding associates a user to a tenant/project with a role.
type TenantBinding struct {
	TenantID string
	Role     string
}

// TenantResolver resolves tenant context from headers or query parameters.
func ResolveTenant(headerVal, queryVal string) string {
	if t := strings.TrimSpace(headerVal); t != "" {
		return t
	}
	return strings.TrimSpace(queryVal)
}
