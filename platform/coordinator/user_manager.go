package coordinator

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
)

// UserManager handles user authentication and authorization.
// It implements certificate-based authentication and role-based access control.
type UserManager struct {
	mu sync.RWMutex

	// Users (name -> User)
	users map[string]*UserEntry

	// Roles (name -> Role)
	roles map[string]*os.Role

	// Certificate to user mapping (cert fingerprint -> user name)
	certToUser map[string]string
}

// UserEntry holds user information with parsed certificate.
type UserEntry struct {
	Name        string
	Certificate *x509.Certificate
	Roles       []string
}

// NewUserManager creates a new UserManager.
func NewUserManager() *UserManager {
	return &UserManager{
		users:      make(map[string]*UserEntry),
		roles:      make(map[string]*os.Role),
		certToUser: make(map[string]string),
	}
}

// SetUsersAndRoles configures users and roles from the manifest.
func (um *UserManager) SetUsersAndRoles(users map[string]os.User, roles map[string]os.Role) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Clear existing
	um.users = make(map[string]*UserEntry, len(users))
	um.roles = make(map[string]*os.Role, len(roles))
	um.certToUser = make(map[string]string, len(users))

	// Set roles
	for name, role := range roles {
		r := role // Copy
		um.roles[name] = &r
	}

	// Set users
	for name, user := range users {
		cert, err := parseCertificate(user.Certificate)
		if err != nil {
			return fmt.Errorf("parse certificate for user %q: %w", name, err)
		}

		// Verify all roles exist
		for _, roleName := range user.Roles {
			if _, ok := um.roles[roleName]; !ok {
				return fmt.Errorf("user %q references unknown role %q", name, roleName)
			}
		}

		entry := &UserEntry{
			Name:        name,
			Certificate: cert,
			Roles:       user.Roles,
		}
		um.users[name] = entry

		// Map certificate fingerprint to user
		fingerprint := certFingerprint(cert)
		um.certToUser[fingerprint] = name
	}

	return nil
}

// AuthenticateCertificate authenticates a user by their certificate.
func (um *UserManager) AuthenticateCertificate(cert *x509.Certificate) (string, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	fingerprint := certFingerprint(cert)
	userName, ok := um.certToUser[fingerprint]
	if !ok {
		return "", fmt.Errorf("unknown certificate")
	}

	return userName, nil
}

// GetUser retrieves a user by name.
func (um *UserManager) GetUser(name string) (*UserEntry, bool) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, ok := um.users[name]
	return user, ok
}

// GetUserRoles returns the roles assigned to a user.
func (um *UserManager) GetUserRoles(userName string) []string {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, ok := um.users[userName]
	if !ok {
		return nil
	}

	return user.Roles
}

// HasPermission checks if a user has permission for an action on a resource.
func (um *UserManager) HasPermission(userName, resourceType, resourceName, action string) bool {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, ok := um.users[userName]
	if !ok {
		return false
	}

	for _, roleName := range user.Roles {
		role, ok := um.roles[roleName]
		if !ok {
			continue
		}

		// Check resource type
		if role.ResourceType != resourceType && role.ResourceType != "*" {
			continue
		}

		// Check resource name (empty = all resources)
		if len(role.ResourceNames) > 0 {
			found := false
			for _, rn := range role.ResourceNames {
				if rn == resourceName || rn == "*" {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check action
		for _, a := range role.Actions {
			if a == action || a == "*" {
				return true
			}
		}
	}

	return false
}

// HasRole checks if a user has a specific role.
func (um *UserManager) HasRole(userName, roleName string) bool {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, ok := um.users[userName]
	if !ok {
		return false
	}

	for _, r := range user.Roles {
		if r == roleName {
			return true
		}
	}

	return false
}

// GetAllowedActions returns all actions a user can perform on a resource.
func (um *UserManager) GetAllowedActions(userName, resourceType, resourceName string) []string {
	um.mu.RLock()
	defer um.mu.RUnlock()

	user, ok := um.users[userName]
	if !ok {
		return nil
	}

	actionSet := make(map[string]bool)

	for _, roleName := range user.Roles {
		role, ok := um.roles[roleName]
		if !ok {
			continue
		}

		// Check resource type
		if role.ResourceType != resourceType && role.ResourceType != "*" {
			continue
		}

		// Check resource name
		if len(role.ResourceNames) > 0 {
			found := false
			for _, rn := range role.ResourceNames {
				if rn == resourceName || rn == "*" {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Collect actions
		for _, a := range role.Actions {
			actionSet[a] = true
		}
	}

	actions := make([]string, 0, len(actionSet))
	for a := range actionSet {
		actions = append(actions, a)
	}

	return actions
}

// ListUsers returns all user names.
func (um *UserManager) ListUsers() []string {
	um.mu.RLock()
	defer um.mu.RUnlock()

	names := make([]string, 0, len(um.users))
	for name := range um.users {
		names = append(names, name)
	}
	return names
}

// ListRoles returns all role names.
func (um *UserManager) ListRoles() []string {
	um.mu.RLock()
	defer um.mu.RUnlock()

	names := make([]string, 0, len(um.roles))
	for name := range um.roles {
		names = append(names, name)
	}
	return names
}

// =============================================================================
// Helper Functions
// =============================================================================

func parseCertificate(pemData string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

func certFingerprint(cert *x509.Certificate) string {
	// Use SHA256 of the raw certificate as fingerprint
	h := sha256.Sum256(cert.Raw)
	return fmt.Sprintf("%x", h[:])
}
