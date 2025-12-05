package base

import (
	"context"
)

// StoreContext carries the Supabase access token for RLS-aware operations.
type StoreContext struct {
	context.Context
	AccessToken string
}

// WithAccessToken returns a wrapped context carrying an access token.
func WithAccessToken(ctx context.Context, token string) context.Context {
	return &StoreContext{Context: ctx, AccessToken: token}
}

// AccessTokenFromContext extracts an access token from StoreContext if present.
func AccessTokenFromContext(ctx context.Context) string {
	if sc, ok := ctx.(*StoreContext); ok {
		return sc.AccessToken
	}
	return ""
}
