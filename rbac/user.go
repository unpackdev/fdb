// pkg/rbac/user.go
package rbac

import (
	"fmt"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/types"
)

// User represents a system user with roles and permissions.
type User struct {
	ID      string
	Roles   []types.Role
	mu      deadlock.RWMutex // Mutex to ensure thread-safe access
	manager *Manager
}

// NewUser creates a new User with the given ID and roles.
func NewUser(id string, manager *Manager, roles ...types.Role) *User {
	return &User{
		ID:      id,
		Roles:   roles,
		manager: manager,
	}
}

// HasPermission checks if the user has the specified permission using RBACManager.
func (u *User) HasPermission(permission types.Permission) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return u.manager.HasPermission(u.Roles, permission)
}

// Authorize ensures the user has the required permission.
// Returns nil if authorized, otherwise an error.
func (u *User) Authorize(permission types.Permission) error {
	if u.HasPermission(permission) {
		return nil
	}
	return fmt.Errorf("user %s does not have permission %s", u.ID, permission)
}

// AddRole adds a role to the user if it's not already assigned.
func (u *User) AddRole(role types.Role) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Check if the role already exists
	for _, r := range u.Roles {
		if r == role {
			// Role already assigned; do nothing
			return
		}
	}

	// Add the new role
	u.Roles = append(u.Roles, role)
}

// RemoveRole removes a role from the user if it exists.
func (u *User) RemoveRole(role types.Role) {
	u.mu.Lock()
	defer u.mu.Unlock()

	newRoles := make([]types.Role, 0, len(u.Roles))
	for _, r := range u.Roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	u.Roles = newRoles
}

// HasRole checks if the user possesses a specific role.
func (u *User) HasRole(role types.Role) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}
