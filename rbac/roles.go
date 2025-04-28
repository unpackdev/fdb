// pkg/rbac/roles.go
package rbac

import "github.com/unpackdev/fdb/types"

const (
	RoleAdmin              types.Role = "admin"
	RoleValidator          types.Role = "validator"
	RoleSequencer          types.Role = "sequencer"
	RoleNode               types.Role = "node"
	RoleObserver           types.Role = "observer"
	RoleUser               types.Role = "user" // Added a generic user role
	RoleSequencerValidator types.Role = "sequencer_validator"
)

const (
	PermissionManageKeys        types.Permission = "manage_keys"
	PermissionViewKeys          types.Permission = "view_keys"
	PermissionProposeBlocks     types.Permission = "propose_blocks"
	PermissionApproveBlocks     types.Permission = "approve_blocks"
	PermissionFinalizeBlocks    types.Permission = "finalize_blocks"
	PermissionStoreData         types.Permission = "store_data"       // Example additional permission
	PermissionRetrieveData      types.Permission = "retrieve_data"    // Example additional permission
	PermissionAssignShard       types.Permission = "assign_shard"     // Example additional permission
	PermissionRemoveShard       types.Permission = "remove_shard"     // Example additional permission
	PermissionMonitorNetwork    types.Permission = "monitor_network"  // Example additional permission
	PermissionUpdateValidator   types.Permission = "update_validator" // Example additional permission
	PermissionSequenceBlocks    types.Permission = "sequence_blocks"  // Specific to Sequencers
	PermissionManageNodes       types.Permission = "manage_nodes"     // Specific to Nodes
	PermissionSignTransactions  types.Permission = "sign_transactions"
	PermissionVerifySignatures  types.Permission = "verify_signatures"
	PermissionCollectSignatures types.Permission = "collect_signatures"

	// Topology-Specific Permissions
	PermissionAddPeer            types.Permission = "add_peer"
	PermissionRemovePeer         types.Permission = "remove_peer"
	PermissionViewTopology       types.Permission = "view_topology"
	PermissionProcessActorPacket types.Permission = "process_actor_packet"
	PermissionSendActorPacket    types.Permission = "send_actor_packet"
)

// RolePermissions maps roles to their permissions
var RolePermissions = map[types.Role][]types.Permission{
	RoleAdmin: {
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
	RoleSequencerValidator: {
		PermissionViewKeys,
		PermissionProposeBlocks,
		PermissionApproveBlocks,
		PermissionFinalizeBlocks,
		PermissionRetrieveData,
		PermissionStoreData,
		PermissionMonitorNetwork,
		PermissionUpdateValidator,
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
	RoleValidator: {
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
	RoleSequencer: {
		PermissionProposeBlocks,
		PermissionSequenceBlocks, // Ability to order or sequence blocks
		PermissionAssignShard,    // Manage shard assignments
		PermissionRemoveShard,    // Manage shard removals
		// Topology Permissions
		PermissionAddPeer,
		PermissionRemovePeer,
		PermissionViewTopology,
		PermissionProcessActorPacket,
		PermissionSendActorPacket,
	},
	RoleNode: {
		PermissionStoreData,
		PermissionRetrieveData,
		PermissionMonitorNetwork, // Monitor network health and status
		PermissionManageNodes,    // Manage node configurations or statuses
		// Topology Permissions
		PermissionAddPeer,
		PermissionRemovePeer,
		PermissionViewTopology,
		PermissionProcessActorPacket,
		PermissionSendActorPacket,
	},
	RoleObserver: {
		PermissionViewKeys,
		PermissionRetrieveData,
		PermissionMonitorNetwork, // Ability to monitor network without making changes
		// Topology Permissions
		PermissionViewTopology,
	},
	RoleUser: {
		PermissionViewKeys,
		PermissionRetrieveData,
		PermissionSignTransactions,
		PermissionVerifySignatures,
	},
}

// EligibleConsensusRoles Defines consensus eligible leadership roles
var EligibleConsensusRoles = []types.Role{
	RoleSequencerValidator,
	RoleValidator,
	RoleSequencer,
}

func GetCompositeRolesByRole(role types.Role) []types.Role {
	roles := make([]types.Role, 0)

	switch role {
	case RoleSequencerValidator:
		roles = append(roles, RoleSequencer)
		roles = append(roles, RoleValidator)
	case RoleValidator:
		roles = append(roles, RoleValidator)
	case RoleSequencer:
		roles = append(roles, RoleSequencer)
	}

	return roles
}

// HasRole check if a role is part of a subset or roles
func HasRole(roles []types.Role, role types.Role) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsConsensusSupported a helper function to check if the current role has consensus capabilities
// or if it should be used in the consensus...
func IsConsensusSupported(role types.Role) bool {
	return HasRole(EligibleConsensusRoles, role)
}

// IsConsensusSupportedByRoles a helper function to check if the current role has consensus capabilities
// or if it should be used in the consensus...
func IsConsensusSupportedByRoles(role ...types.Role) bool {
	for _, r := range role {
		if found := HasRole(EligibleConsensusRoles, r); found {
			return true
		}
	}

	return false
}

// IsPermissionGranted checks if a role has a specific permission
func IsPermissionGranted(role types.Role, permission types.Permission) bool {
	perms, exists := RolePermissions[role]
	if !exists {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}
