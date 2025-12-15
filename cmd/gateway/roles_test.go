package main

import "testing"

func TestResolveUserRole(t *testing.T) {
	adminUserIDs = map[string]struct{}{"admin": {}}
	superAdminUserIDs = map[string]struct{}{"root": {}}

	if got := resolveUserRole(""); got != "" {
		t.Fatalf("resolveUserRole(empty) = %q, want empty", got)
	}
	if got := resolveUserRole("user"); got != "" {
		t.Fatalf("resolveUserRole(user) = %q, want empty", got)
	}
	if got := resolveUserRole("admin"); got != "admin" {
		t.Fatalf("resolveUserRole(admin) = %q, want admin", got)
	}
	if got := resolveUserRole("root"); got != "super_admin" {
		t.Fatalf("resolveUserRole(root) = %q, want super_admin", got)
	}
}
