// pkg/rbac/manager_test.go
package rbac

import (
	"context"
	"fmt"
	"testing"

	"github.com/unpackdev/fdb/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBACManager_AssignRole(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	tests := []struct {
		name        string
		role        types.Role
		permissions []types.Permission
		expectError bool
	}{
		{
			name:        "Assign Admin Role with multiple permissions",
			role:        RoleAdmin,
			permissions: []types.Permission{PermissionManageKeys, PermissionSignTransactions, PermissionVerifySignatures},
			expectError: false,
		},
		{
			name:        "Assign Validator Role with specific permissions",
			role:        RoleValidator,
			permissions: []types.Permission{PermissionApproveBlocks, PermissionViewKeys},
			expectError: false,
		},
		{
			name:        "Assign existing Role without duplication",
			role:        RoleAdmin,
			permissions: []types.Permission{PermissionFinalizeBlocks},
			expectError: false,
		},
		{
			name:        "Assign Role with no permissions",
			role:        RoleObserver,
			permissions: []types.Permission{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AssignRole(tt.role, tt.permissions...)
			if tt.expectError {
				require.Error(t, err, "Expected error while assigning role")
			} else {
				require.NoError(t, err, "Did not expect error while assigning role")
			}

			// Verify permissions are assigned correctly
			for _, perm := range tt.permissions {
				perms, err := manager.GetPermissionsForRole(tt.role)
				require.NoError(t, err, "Failed to get permissions for role")
				assert.Contains(t, perms, perm, fmt.Sprintf("Permission %s should be assigned to role %s", perm, tt.role))
			}
		})
	}
}

func TestRBACManager_RemoveRole(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Pre-assign roles and permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions, PermissionVerifySignatures)
	require.NoError(t, err, "Failed to assign initial roles and permissions")
	err = manager.AssignRole(RoleValidator, PermissionApproveBlocks, PermissionViewKeys, PermissionManageKeys) // Assign ManageKeys to Validator for test
	require.NoError(t, err, "Failed to assign Validator role with ManageKeys")
	err = manager.AssignRole(RoleObserver, PermissionViewKeys, PermissionRetrieveData, PermissionMonitorNetwork)
	require.NoError(t, err, "Failed to assign Observer role")
	// Note: RoleUser is not assigned any permissions in this setup

	tests := []struct {
		name        string
		role        types.Role
		permissions []types.Permission
		expectError bool
	}{
		{
			name:        "Remove specific permissions from Admin Role",
			role:        RoleAdmin,
			permissions: []types.Permission{PermissionSignTransactions},
			expectError: false,
		},
		{
			name:        "Remove non-existing permission from Validator Role",
			role:        RoleValidator,
			permissions: []types.Permission{PermissionManageNodes}, // Permission not assigned
			expectError: false,                                     // Should not error even if permission doesn't exist
		},
		{
			name:        "Remove permissions from non-existing Role",
			role:        "NonExistentRole",
			permissions: []types.Permission{PermissionManageKeys},
			expectError: true,
		},
		{
			name:        "Remove all permissions from Observer Role",
			role:        RoleObserver,
			permissions: []types.Permission{PermissionViewKeys, PermissionRetrieveData, PermissionMonitorNetwork},
			expectError: false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			err := manager.RemoveRole(tt.role, tt.permissions...)
			if tt.expectError {
				require.Error(t, err, "Expected error while removing role")
			} else {
				require.NoError(t, err, "Did not expect error while removing role")
			}

			// Verify permissions are removed correctly
			for _, perm := range tt.permissions {
				perms, err := manager.GetPermissionsForRole(tt.role)
				if tt.expectError && tt.role == "NonExistentRole" {
					continue // Skip further checks for non-existent role
				}
				if tt.role != "NonExistentRole" {
					if len(tt.permissions) == 0 {
						// If no permissions were specified to remove, skip checking
						continue
					}
					require.NoError(t, err, "Failed to get permissions for role")
					assert.NotContains(t, perms, perm, fmt.Sprintf("Permission %s should be removed from role %s", perm, tt.role))
				}
			}
		})
	}
}

func TestRBACManager_HasPermission(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign roles with permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions, PermissionVerifySignatures)
	require.NoError(t, err, "Failed to assign Admin role")
	err = manager.AssignRole(RoleValidator, PermissionApproveBlocks, PermissionViewKeys)
	require.NoError(t, err, "Failed to assign Validator role")
	err = manager.AssignRole(RoleUser, PermissionViewKeys, PermissionRetrieveData)
	require.NoError(t, err, "Failed to assign User role")

	tests := []struct {
		name       string
		userRoles  []types.Role
		permission types.Permission
		expected   bool
	}{
		{
			name:       "Admin has ManageKeys permission",
			userRoles:  []types.Role{RoleAdmin},
			permission: PermissionManageKeys,
			expected:   true,
		},
		{
			name:       "Validator has ApproveBlocks permission",
			userRoles:  []types.Role{RoleValidator},
			permission: PermissionApproveBlocks,
			expected:   true,
		},
		{
			name:       "User does not have ManageKeys permission",
			userRoles:  []types.Role{RoleUser},
			permission: PermissionManageKeys,
			expected:   false,
		},
		{
			name:       "Admin has SignTransactions permission",
			userRoles:  []types.Role{RoleAdmin},
			permission: PermissionSignTransactions,
			expected:   true,
		},
		{
			name:       "User has RetrieveData permission",
			userRoles:  []types.Role{RoleUser},
			permission: PermissionRetrieveData,
			expected:   true,
		},
		{
			name:       "Multiple roles: Admin and Validator have ManageKeys permission",
			userRoles:  []types.Role{RoleAdmin, RoleValidator},
			permission: PermissionManageKeys,
			expected:   true,
		},
		{
			name:       "Multiple roles: Validator and User do not have ManageKeys permission",
			userRoles:  []types.Role{RoleValidator, RoleUser},
			permission: PermissionManageKeys,
			expected:   false,
		},
		{
			name:       "No roles assigned",
			userRoles:  []types.Role{},
			permission: PermissionViewKeys,
			expected:   false,
		},
		{
			name:       "User has ViewKeys permission",
			userRoles:  []types.Role{RoleUser},
			permission: PermissionViewKeys,
			expected:   true,
		},
		{
			name:       "Validator does not have ManageKeys permission",
			userRoles:  []types.Role{RoleValidator},
			permission: PermissionManageKeys,
			expected:   false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			hasPerm := manager.HasPermission(tt.userRoles, tt.permission)
			assert.Equal(t, tt.expected, hasPerm, fmt.Sprintf("Expected permission %v, got %v", tt.expected, hasPerm))
		})
	}
}

func TestRBACManager_GetRoles(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign roles with permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions, PermissionVerifySignatures)
	require.NoError(t, err, "Failed to assign Admin role")
	err = manager.AssignRole(RoleUser, PermissionViewKeys, PermissionRetrieveData)
	require.NoError(t, err, "Failed to assign User role")

	// Define all expected roles with their assigned permissions
	expectedRoles := map[types.Role][]types.Permission{
		RoleAdmin: {
			PermissionManageKeys,
			PermissionSignTransactions,
			PermissionVerifySignatures,
		},
		RoleUser: {
			PermissionViewKeys,
			PermissionRetrieveData,
		},
	}

	roles := manager.GetRoles()
	assert.Len(t, roles, len(expectedRoles), "Number of roles should match expected")

	for role, perms := range expectedRoles {
		retrievedPerms, exists := roles[role]
		require.True(t, exists, fmt.Sprintf("Role %s should exist", role))
		assert.ElementsMatch(t, perms, retrievedPerms, fmt.Sprintf("Permissions for role %s should match", role))
	}
}

func TestRBACManager_GetPermissionsForRole(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign roles with permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions, PermissionVerifySignatures)
	require.NoError(t, err, "Failed to assign Admin role")

	err = manager.AssignRole(RoleUser, PermissionViewKeys, PermissionRetrieveData, PermissionApproveBlocks) // Assign all expected permissions
	require.NoError(t, err, "Failed to assign User role with permissions")

	err = manager.AssignRole(RoleValidator, PermissionApproveBlocks, PermissionViewKeys, PermissionProposeBlocks, PermissionFinalizeBlocks, PermissionRetrieveData, PermissionStoreData, PermissionMonitorNetwork, PermissionUpdateValidator)
	require.NoError(t, err, "Failed to assign Validator role with default permissions")

	tests := []struct {
		name          string
		role          types.Role
		expectedPerms []types.Permission
		expectError   bool
	}{
		{
			name: "Get permissions for Admin Role",
			role: RoleAdmin,
			expectedPerms: []types.Permission{
				PermissionManageKeys,
				PermissionSignTransactions,
				PermissionVerifySignatures,
			},
			expectError: false,
		},
		{
			name: "Get permissions for User Role",
			role: RoleUser,
			expectedPerms: []types.Permission{
				PermissionViewKeys,
				PermissionRetrieveData,
				PermissionApproveBlocks, // Added in test
			},
			expectError: false,
		},
		{
			name:          "Get permissions for non-existing Role",
			role:          "NonExistentRole",
			expectedPerms: nil,
			expectError:   true,
		},
		{
			name: "Get permissions for Validator Role",
			role: RoleValidator,
			expectedPerms: []types.Permission{
				PermissionApproveBlocks,
				PermissionViewKeys,
				PermissionProposeBlocks,
				PermissionFinalizeBlocks,
				PermissionRetrieveData,
				PermissionStoreData,
				PermissionMonitorNetwork,
				PermissionUpdateValidator,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			perms, err := manager.GetPermissionsForRole(tt.role)
			if tt.expectError {
				require.Error(t, err, "Expected error for non-existing role")
			} else {
				require.NoError(t, err, "Did not expect error for existing role")
				assert.ElementsMatch(t, tt.expectedPerms, perms, "Permissions should match expected")
			}
		})
	}
}
