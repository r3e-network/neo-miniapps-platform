package main

import (
	"os"
	"strings"
)

var (
	adminUserIDs      = map[string]struct{}{}
	superAdminUserIDs = map[string]struct{}{}
)

func loadAdminAllowlistsFromEnv() {
	adminUserIDs = parseCSVSet(os.Getenv("ADMIN_USER_IDS"))
	superAdminUserIDs = parseCSVSet(os.Getenv("SUPER_ADMIN_USER_IDS"))
}

func resolveUserRole(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	if _, ok := superAdminUserIDs[userID]; ok {
		return "super_admin"
	}
	if _, ok := adminUserIDs[userID]; ok {
		return "admin"
	}
	return ""
}

func parseCSVSet(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}
