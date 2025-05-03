// pkg/rbac/manager.go
package rbac

import (
	"context"
	"fmt"

	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/pkg/types"
)

// Manager manages roles and permissions.
type Manager struct {
	ctx             context.Context
	rolePermissions map[types.Role]map[types.Permission]struct{}
	mu              deadlock.RWMutex
}

// NewManager initializes a new RBAC Manager.
// If withDefaults is true, it initializes with predefined roles and permissions.
// Additional roles and permissions can be provided via variadic options.
func NewManager(ctx context.Context, opts ...Option) (*Manager, error) {
	manager := &Manager{
		ctx:             ctx,
		rolePermissions: make(map[types.Role]map[types.Permission]struct{}),
	}
	for _, opt := range opts {
		if err := opt(manager); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

// AssignRole assigns one or more permissions to a role.
// If the role does not exist, it is created.
func (m *Manager) AssignRole(role types.Role, permissions ...types.Permission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rolePermissions[role]; !exists {
		m.rolePermissions[role] = make(map[types.Permission]struct{})
	}

	for _, perm := range permissions {
		m.rolePermissions[role][perm] = struct{}{}
	}

	return nil
}

// RemoveRole removes one or more permissions from a role.
// If the role does not exist, an error is returned.
func (m *Manager) RemoveRole(role types.Role, permissions ...types.Permission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	perms, exists := m.rolePermissions[role]
	if !exists {
		return fmt.Errorf("role %s does not exist", role)
	}

	for _, perm := range permissions {
		delete(perms, perm)
	}

	return nil
}

// HasPermission checks if any of the user's roles grant the specified permission.
func (m *Manager) HasPermission(userRoles []types.Role, permission types.Permission) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, role := range userRoles {
		if perms, exists := m.rolePermissions[role]; exists {
			if _, hasPerm := perms[permission]; hasPerm {
				return true
			}
		}
	}
	return false
}

// GetRoles returns all roles and their associated permissions.
func (m *Manager) GetRoles() map[types.Role][]types.Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[types.Role][]types.Permission)
	for role, perms := range m.rolePermissions {
		for perm := range perms {
			result[role] = append(result[role], perm)
		}
	}
	return result
}

// GetPermissionsForRole returns all permissions assigned to a specific role.
// Returns an error if the role does not exist.
func (m *Manager) GetPermissionsForRole(role types.Role) ([]types.Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perms, exists := m.rolePermissions[role]
	if !exists {
		return nil, fmt.Errorf("role %s does not exist", role)
	}

	var permissions []types.Permission
	for perm := range perms {
		permissions = append(permissions, perm)
	}
	return permissions, nil
}
