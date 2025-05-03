// pkg/topology/roles.go

package topology

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/types"
)

// AddPeerRole assigns a new role to an existing peer.
func (a *Actors) AddPeerRole(peerID peer.ID, role types.Role) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	actor, exists := a.peers[peerID]
	if !exists {
		return ErrPeerNotFound
	}

	// Check if the peer already has the role
	for _, r := range actor.Roles {
		if r == role {
			return fmt.Errorf("peer %s already has role %s", peerID, role)
		}
	}

	// Assign the new role
	actor.Roles = append(actor.Roles, role)

	// Update roleIndex
	if a.roleIndex[role] == nil {
		a.roleIndex[role] = make(peerSet)
	}
	a.roleIndex[role].add(peerID)

	// Notify peer availability
	if a.onPeerEvent != nil {
		a.onPeerEvent()
	}

	a.logger.Info("Assigned new role to peer", "peer_id", peerID.String(), "role", role)

	return nil
}

// RemovePeerRole removes a specific role from an existing peer.
func (a *Actors) RemovePeerRole(peerID peer.ID, role types.Role) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	actor, exists := a.peers[peerID]
	if !exists {
		return ErrPeerNotFound
	}

	// Find and remove the role
	index := -1
	for i, r := range actor.Roles {
		if r == role {
			index = i
			break
		}
	}

	if index == -1 {
		return fmt.Errorf("peer %s does not have role %s", peerID, role)
	}

	// Remove the role from the slice
	actor.Roles = append(actor.Roles[:index], actor.Roles[index+1:]...)

	// Update roleIndex
	if a.roleIndex[role] != nil {
		a.roleIndex[role].remove(peerID)
		if len(a.roleIndex[role]) == 0 {
			delete(a.roleIndex, role)
		}
	}

	// Notify peer availability
	if a.onPeerEvent != nil {
		a.onPeerEvent()
	}

	a.logger.Info("Removed role from peer", "peer_id", peerID.String(), "role", role)

	return nil
}

// GetPeersByRole retrieves all peers assigned a specific role.
func (a *Actors) GetPeersByRole(role types.Role) ([]*Actor, error) {
	// RBAC Check: Verify if self has PermissionViewTopology
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionViewTopology) {
		a.logger.Warn("Insufficient permissions to view topology", "required_permission", rbac.PermissionViewTopology)
		return nil, ErrInsufficientPermissions
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	rPeerSet, exists := a.roleIndex[role]
	if !exists {
		// No peers with the specified role
		return []*Actor{}, nil
	}

	result := make([]*Actor, 0, len(rPeerSet))
	for peerID := range rPeerSet {
		if actor, pExists := a.peers[peerID]; pExists {
			result = append(result, actor)
		}
	}

	return result, nil
}

// GetPeersByRoles retrieves all peers assigned any of the specified roles.
func (a *Actors) GetPeersByRoles(roles ...types.Role) ([]*Actor, error) {
	// RBAC Check: Verify if self has PermissionViewTopology
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionViewTopology) {
		a.logger.Warn("Insufficient permissions to view topology", "required_permission", rbac.PermissionViewTopology)
		return nil, ErrInsufficientPermissions
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Use a map to avoid duplicate peers
	resultMap := make(map[peer.ID]*Actor)

	for _, role := range roles {
		if rPeerSet, exists := a.roleIndex[role]; exists {
			for peerID := range rPeerSet {
				if actor, pExists := a.peers[peerID]; pExists {
					resultMap[peerID] = actor
				}
			}
		}
	}

	// Convert the map to a slice
	result := make([]*Actor, 0, len(resultMap))
	for _, actor := range resultMap {
		result = append(result, actor)
	}

	return result, nil
}

// GetRolesOfPeer retrieves all roles associated with a specific peer.
func (a *Actors) GetRolesOfPeer(peerID peer.ID) ([]types.Role, error) {
	// RBAC Check: Verify if self has PermissionViewTopology
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionViewTopology) {
		a.logger.Warn("Insufficient permissions to view topology", "required_permission", rbac.PermissionViewTopology)
		return nil, ErrInsufficientPermissions
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	actor, exists := a.peers[peerID]
	if !exists {
		return nil, ErrPeerNotFound
	}

	return actor.Roles, nil
}

// PeerHasRole checks if a specific peer possesses a particular role.
func (a *Actors) PeerHasRole(peerID peer.ID, role types.Role) (bool, error) {
	// RBAC Check: Verify if self has PermissionViewTopology
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionViewTopology) {
		a.logger.Warn("Insufficient permissions to view topology", "required_permission", rbac.PermissionViewTopology)
		return false, ErrInsufficientPermissions
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	actor, exists := a.peers[peerID]
	if !exists {
		return false, ErrPeerNotFound
	}

	for _, r := range actor.Roles {
		if r == role {
			return true, nil
		}
	}

	return false, nil
}
