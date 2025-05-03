// pkg/rbac/user_test.go
package rbac

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/unpackdev/fdb/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAuthorize(t *testing.T) {
	// Initialize RBAC Manager without default roles
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign roles with permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions, PermissionVerifySignatures)
	require.NoError(t, err, "Failed to assign Admin role")

	err = manager.AssignRole(RoleValidator, PermissionApproveBlocks, PermissionViewKeys)
	require.NoError(t, err, "Failed to assign Validator role")

	// Create users with different roles
	tests := []struct {
		name        string
		userRoles   []types.Role
		permission  types.Permission
		expectError bool
	}{
		{
			name:        "Admin has ManageKeys permission",
			userRoles:   []types.Role{RoleAdmin},
			permission:  PermissionManageKeys,
			expectError: false,
		},
		{
			name:        "Validator has ApproveBlocks permission",
			userRoles:   []types.Role{RoleValidator},
			permission:  PermissionApproveBlocks,
			expectError: false,
		},
		{
			name:        "Validator does not have ManageKeys permission",
			userRoles:   []types.Role{RoleValidator},
			permission:  PermissionManageKeys,
			expectError: true,
		},
		{
			name:        "User with no roles",
			userRoles:   []types.Role{},
			permission:  PermissionViewKeys,
			expectError: true,
		},
		{
			name:        "Admin has VerifySignatures permission",
			userRoles:   []types.Role{RoleAdmin},
			permission:  PermissionVerifySignatures,
			expectError: false,
		},
		{
			name:        "User has ApproveBlocks permission without role",
			userRoles:   []types.Role{RoleUser},
			permission:  PermissionApproveBlocks,
			expectError: true,
		},
		{
			name:        "Validator has ViewKeys permission",
			userRoles:   []types.Role{RoleValidator},
			permission:  PermissionViewKeys,
			expectError: false,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser("testuser", manager, tt.userRoles...)
			err := user.Authorize(tt.permission)
			if tt.expectError {
				assert.Error(t, err, "Expected authorization to fail")
			} else {
				assert.NoError(t, err, "Expected authorization to succeed")
			}
		})
	}
}

func TestUserAddRemoveRole(t *testing.T) {
	// Initialize RBAC Manager without default roles
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign roles with permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions)
	require.NoError(t, err, "Failed to assign Admin role")

	// Create a user with initial roles
	user := NewUser("user1", manager, RoleUser)

	tests := []struct {
		name          string
		action        func()
		expectedRoles []types.Role
	}{
		{
			name: "Add Admin Role to User",
			action: func() {
				user.AddRole(RoleAdmin)
			},
			expectedRoles: []types.Role{RoleUser, RoleAdmin},
		},
		{
			name: "Add Validator Role to User",
			action: func() {
				user.AddRole(RoleValidator)
			},
			expectedRoles: []types.Role{RoleUser, RoleAdmin, RoleValidator},
		},
		{
			name: "Add existing Admin Role to User (should not duplicate)",
			action: func() {
				user.AddRole(RoleAdmin)
			},
			expectedRoles: []types.Role{RoleUser, RoleAdmin, RoleValidator},
		},
		{
			name: "Remove Admin Role from User",
			action: func() {
				user.RemoveRole(RoleAdmin)
			},
			expectedRoles: []types.Role{RoleUser, RoleValidator},
		},
		{
			name: "Remove non-assigned Observer Role from User",
			action: func() {
				user.RemoveRole(RoleObserver)
			},
			expectedRoles: []types.Role{RoleUser, RoleValidator},
		},
		{
			name: "Remove Validator Role from User",
			action: func() {
				user.RemoveRole(RoleValidator)
			},
			expectedRoles: []types.Role{RoleUser},
		},
		{
			name: "Remove User Role from User",
			action: func() {
				user.RemoveRole(RoleUser)
			},
			expectedRoles: []types.Role{},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			tt.action()
			assert.ElementsMatch(t, tt.expectedRoles, user.Roles, "User roles should match expected roles after action")
		})
	}
}

func TestUserConcurrency(t *testing.T) {
	// Initialize RBAC Manager without default roles
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign multiple roles with permissions
	err = manager.AssignRole(RoleAdmin, PermissionManageKeys, PermissionSignTransactions)
	require.NoError(t, err, "Failed to assign Admin role")
	err = manager.AssignRole(RoleValidator, PermissionApproveBlocks, PermissionViewKeys)
	require.NoError(t, err, "Failed to assign Validator role")

	user := NewUser("concurrentUser", manager, RoleUser)

	var wg sync.WaitGroup
	numRoutines := 100

	// Concurrently add and remove roles
	for i := 0; i < numRoutines; i++ {
		wg.Add(2)

		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				user.AddRole(RoleAdmin)
			} else {
				user.RemoveRole(RoleValidator)
			}
		}(i)

		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				user.AddRole(RoleValidator)
			} else {
				user.RemoveRole(RoleAdmin)
			}
		}(i)
	}

	wg.Wait()

	// Final roles should be consistent and no data races should have occurred
	// Since operations are random, we can only assert that roles are a subset of possible roles
	for _, role := range user.Roles {
		assert.Contains(t, []types.Role{RoleUser, RoleAdmin, RoleValidator}, role, "User has an unexpected role")
	}
}

func TestUserHasRole(t *testing.T) {
	// Initialize RBAC Manager without default roles
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Assign roles with permissions
	err = manager.AssignRole(RoleSequencer, PermissionSequenceBlocks, PermissionAssignShard)
	require.NoError(t, err, "Failed to assign Sequencer role")

	// Create users with different roles
	tests := []struct {
		name        string
		userRoles   []types.Role
		roleToCheck types.Role
		expected    bool
	}{
		{
			name:        "User has Sequencer role",
			userRoles:   []types.Role{RoleUser, RoleSequencer},
			roleToCheck: RoleSequencer,
			expected:    true,
		},
		{
			name:        "User does not have Admin role",
			userRoles:   []types.Role{RoleUser, RoleValidator},
			roleToCheck: RoleAdmin,
			expected:    false,
		},
		{
			name:        "User has only User role",
			userRoles:   []types.Role{RoleUser},
			roleToCheck: RoleUser,
			expected:    true,
		},
		{
			name:        "User has no roles",
			userRoles:   []types.Role{},
			roleToCheck: RoleObserver,
			expected:    false,
		},
		{
			name:        "User has Validator role",
			userRoles:   []types.Role{RoleValidator},
			roleToCheck: RoleValidator,
			expected:    true,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser("roleCheckUser", manager, tt.userRoles...)
			hasRole := user.HasRole(tt.roleToCheck)
			assert.Equal(t, tt.expected, hasRole, fmt.Sprintf("Expected HasRole(%s) to be %v", tt.roleToCheck, tt.expected))
		})
	}
}

func TestRBACManager_Concurrency(t *testing.T) {
	ctx := context.Background()
	manager, err := NewManager(ctx)
	require.NoError(t, err, "Failed to create RBAC Manager")

	// Define roles and permissions to be used in concurrency tests
	roles := []types.Role{RoleAdmin, RoleValidator, RoleSequencer, RoleNode, RoleObserver, RoleUser}
	permissions := []types.Permission{
		PermissionManageKeys,
		PermissionViewKeys,
		PermissionProposeBlocks,
		PermissionApproveBlocks,
		PermissionFinalizeBlocks,
		PermissionStoreData,
		PermissionRetrieveData,
		PermissionAssignShard,
		PermissionRemoveShard,
		PermissionMonitorNetwork,
		PermissionUpdateValidator,
		PermissionSequenceBlocks,
		PermissionManageNodes,
		PermissionSignTransactions,
		PermissionVerifySignatures,
		PermissionCollectSignatures,
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrently assign and remove permissions from roles
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)

		go func(i int) {
			defer wg.Done()
			role := roles[i%len(roles)]
			perm := permissions[i%len(permissions)]
			manager.AssignRole(role, perm)
		}(i)

		go func(i int) {
			defer wg.Done()
			role := roles[(i+1)%len(roles)]
			perm := permissions[(i+2)%len(permissions)]
			manager.RemoveRole(role, perm)
		}(i)
	}

	wg.Wait()

	// Verify that the RBACManager is in a consistent state
	// This is a basic check; more thorough checks can be added as needed
	for _, role := range roles {
		perms, err := manager.GetPermissionsForRole(role)
		if err != nil {
			// Role might have been removed all permissions
			continue
		}
		for _, perm := range perms {
			assert.Contains(t, permissions, perm, fmt.Sprintf("Permission %s should be valid", perm))
		}
	}
}
