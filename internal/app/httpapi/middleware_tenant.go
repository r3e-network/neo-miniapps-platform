package httpapi

import "context"

// withTenantContext ensures tenant is set in context for downstream handlers.
func withTenantContext(ctx context.Context, tenant string) context.Context {
	if tenant == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxTenantKey, tenant)
}
