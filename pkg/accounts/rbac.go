// pkg/accounts/rbac.go
package accounts

import (
	"fmt"

	"github.com/unpackdev/fdb/pkg/types"
)

// Roles returns the list of roles assigned to the account.
func (a *Account) Roles() []types.Role {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.roles
}

// AssignRole assigns a new role to the account.
func (a *Account) AssignRole(role types.Role, permissions ...types.Permission) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Assign the role using RBAC Manager
	if err := a.rbacMgr.AssignRole(role, permissions...); err != nil {
		return err
	}

	// Add the role to the account's Roles slice if not already present
	for _, r := range a.roles {
		if r == role {
			// Role already assigned
			return nil
		}
	}
	a.roles = append(a.roles, role)
	return nil
}

// RemoveRole removes a role from the account.
func (a *Account) RemoveRole(role types.Role) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Remove the role from RBAC Manager
	if err := a.rbacMgr.RemoveRole(role); err != nil {
		return err
	}

	// Remove the role from the account's Roles slice
	for i, r := range a.roles {
		if r == role {
			a.roles = append(a.roles[:i], a.roles[i+1:]...)
			break
		}
	}
	return nil
}

// HasPermission checks if the account has the specified permission.
func (a *Account) HasPermission(permission types.Permission) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.rbacMgr.HasPermission(a.roles, permission)
}

// Authorize ensures the account has the required permission.
func (a *Account) Authorize(permission types.Permission) error {
	if a.HasPermission(permission) {
		return nil
	}
	return fmt.Errorf("account %s does not have permission %s", a.name, permission)
}
