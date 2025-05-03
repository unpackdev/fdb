package rbac

import (
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/pkg/types"
)

// Option defines a functional option for configuring the Manager.
type Option func(*Manager) error

// WithRole allows adding a role with its permissions.
func WithRole(role types.Role, permissions ...types.Permission) Option {
	return func(m *Manager) error {
		return m.AssignRole(role, permissions...)
	}
}

// WithDefaultRoles sets up the default roles with their respective permissions.
func WithDefaultRoles() Option {
	return func(m *Manager) error {
		roles := []struct {
			role        types.Role
			permissions []types.Permission
		}{
			{
				role: RoleAdmin,
				permissions: []types.Permission{
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
					// Topology Permissions
					PermissionAddPeer,
					PermissionRemovePeer,
					PermissionViewTopology,
					PermissionProcessActorPacket,
					PermissionSendActorPacket,
				},
			},
			{
				role: RoleValidator,
				permissions: []types.Permission{
					PermissionViewKeys,
					PermissionProposeBlocks,
					PermissionApproveBlocks,
					PermissionFinalizeBlocks,
					PermissionRetrieveData,
					PermissionStoreData,
					PermissionMonitorNetwork,
					PermissionUpdateValidator,
					// Topology Permissions
					PermissionAddPeer,
					PermissionRemovePeer,
					PermissionViewTopology,
					PermissionProcessActorPacket,
					PermissionSendActorPacket,
				},
			},
			{
				role: RoleSequencer,
				permissions: []types.Permission{
					PermissionProposeBlocks,
					PermissionSequenceBlocks,
					PermissionAssignShard,
					PermissionRemoveShard,
					// Topology Permissions
					PermissionAddPeer,
					PermissionRemovePeer,
					PermissionViewTopology,
					PermissionProcessActorPacket,
					PermissionSendActorPacket,
				},
			},
			{
				role: RoleSequencerValidator,
				permissions: []types.Permission{
					PermissionViewKeys,
					PermissionProposeBlocks,
					PermissionApproveBlocks,
					PermissionFinalizeBlocks,
					PermissionRetrieveData,
					PermissionStoreData,
					PermissionMonitorNetwork,
					PermissionUpdateValidator,
					PermissionSequenceBlocks,
					PermissionAssignShard,
					PermissionRemoveShard,
					// Topology Permissions
					PermissionAddPeer,
					PermissionRemovePeer,
					PermissionViewTopology,
					PermissionProcessActorPacket,
					PermissionSendActorPacket,
				},
			},
			{
				role: RoleNode,
				permissions: []types.Permission{
					PermissionStoreData,
					PermissionRetrieveData,
					PermissionMonitorNetwork,
					PermissionManageNodes,
					// Topology Permissions
					PermissionAddPeer,
					PermissionRemovePeer,
					PermissionViewTopology,
					PermissionProcessActorPacket,
					PermissionSendActorPacket,
				},
			},
			{
				role: RoleObserver,
				permissions: []types.Permission{
					PermissionViewKeys,
					PermissionRetrieveData,
					PermissionMonitorNetwork,
					// Topology Permissions
					PermissionViewTopology,
				},
			},
			{
				role: RoleUser,
				permissions: []types.Permission{
					PermissionViewKeys,
					PermissionRetrieveData,
					PermissionSignTransactions,
					PermissionVerifySignatures,
				},
			},
		}

		for _, r := range roles {
			if err := m.AssignRole(r.role, r.permissions...); err != nil {
				return errors.Wrapf(err, "failed to assign role %s", r.role)
			}
		}

		return nil
	}
}
