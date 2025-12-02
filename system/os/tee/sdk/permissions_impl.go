// Package sdk provides the Enclave SDK implementation.
package sdk

import (
	"context"
	"sync"
)

// permissionManagerImpl implements PermissionManager interface.
type permissionManagerImpl struct {
	mu          sync.RWMutex
	callerID    string
	permissions map[string][]Permission
	roles       map[string][]Role
}

// NewPermissionManager creates a new permission manager instance.
func NewPermissionManager(callerID string) PermissionManager {
	return &permissionManagerImpl{
		callerID:    callerID,
		permissions: make(map[string][]Permission),
		roles:       make(map[string][]Role),
	}
}

func (m *permissionManagerImpl) Check(ctx context.Context, permission string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perms, exists := m.permissions[m.callerID]
	if !exists {
		return false, nil
	}

	for _, p := range perms {
		if p.Resource == permission || p.Resource == "*" {
			return true, nil
		}
		for _, action := range p.Actions {
			if action == permission || action == "*" {
				return true, nil
			}
		}
	}

	return false, nil
}

func (m *permissionManagerImpl) CheckAll(ctx context.Context, permissions []string) (bool, error) {
	for _, perm := range permissions {
		has, err := m.Check(ctx, perm)
		if err != nil {
			return false, err
		}
		if !has {
			return false, nil
		}
	}
	return true, nil
}

func (m *permissionManagerImpl) CheckAny(ctx context.Context, permissions []string) (bool, error) {
	for _, perm := range permissions {
		has, err := m.Check(ctx, perm)
		if err != nil {
			return false, err
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

func (m *permissionManagerImpl) GetCallerPermissions(ctx context.Context) ([]Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perms, exists := m.permissions[m.callerID]
	if !exists {
		return []Permission{}, nil
	}

	return perms, nil
}

func (m *permissionManagerImpl) VerifyRole(ctx context.Context, role Role) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roles, exists := m.roles[m.callerID]
	if !exists {
		return false, nil
	}

	for _, r := range roles {
		if r == role || r == RoleAdmin {
			return true, nil
		}
	}

	return false, nil
}

func (m *permissionManagerImpl) GetCallerRoles(ctx context.Context) ([]Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roles, exists := m.roles[m.callerID]
	if !exists {
		return []Role{}, nil
	}

	return roles, nil
}

// SetPermissions sets permissions for a caller (used by enclave runtime).
func (m *permissionManagerImpl) SetPermissions(callerID string, perms []Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permissions[callerID] = perms
}

// SetRoles sets roles for a caller (used by enclave runtime).
func (m *permissionManagerImpl) SetRoles(callerID string, roles []Role) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[callerID] = roles
}

// GrantPermission grants a permission to a caller.
func (m *permissionManagerImpl) GrantPermission(callerID string, perm Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permissions[callerID] = append(m.permissions[callerID], perm)
}

// GrantRole grants a role to a caller.
func (m *permissionManagerImpl) GrantRole(callerID string, role Role) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[callerID] = append(m.roles[callerID], role)
}

// RevokePermission revokes a permission from a caller.
func (m *permissionManagerImpl) RevokePermission(callerID string, resource string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	perms := m.permissions[callerID]
	var newPerms []Permission
	for _, p := range perms {
		if p.Resource != resource {
			newPerms = append(newPerms, p)
		}
	}
	m.permissions[callerID] = newPerms
}

// RevokeRole revokes a role from a caller.
func (m *permissionManagerImpl) RevokeRole(callerID string, role Role) {
	m.mu.Lock()
	defer m.mu.Unlock()

	roles := m.roles[callerID]
	var newRoles []Role
	for _, r := range roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	m.roles[callerID] = newRoles
}
