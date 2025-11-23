package httpapi

import "context"

// withTenantContext ensures tenant is set in context for downstream handlers.
func withTenantContext(ctx context.Context, tenant string) context.Context {
	if tenant == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxTenantKey, tenant)
}

// tenantFromCtx extracts the tenant string from context.
func tenantFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	tenant, _ := ctx.Value(ctxTenantKey).(string)
	return tenant
}
