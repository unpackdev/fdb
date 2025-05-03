// pkg/share/actor_set.go

package share

import (
	"context"
	"fmt"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/types"

	"sort"
	"time"

	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"

	"github.com/libp2p/go-libp2p/core/peer"
)

type ActorCallbackFn func(ctx context.Context, actor *Actor, force bool) error

// ActorSet manages the collection of Actor's participating in consensus.
type ActorSet struct {
	actors          map[peer.ID]*Actor
	leaders         map[types.Role]peer.ID
	mutex           deadlock.RWMutex
	quorumThreshold int
	logger          logger.Logger

	// Callback functions
	onActorAdded   []ActorCallbackFn
	onActorRemoved []ActorCallbackFn
}

// NewActorSet creates a new set of Actor's.
func NewActorSet(logger logger.Logger) *ActorSet {
	actorSet := &ActorSet{
		actors:         make(map[peer.ID]*Actor),
		leaders:        make(map[types.Role]peer.ID),
		logger:         logger,
		onActorAdded:   make([]ActorCallbackFn, 0),
		onActorRemoved: make([]ActorCallbackFn, 0),
	}

	return actorSet
}

// AppendActorAddedCallback sets the callback to be invoked when an Actor is added.
func (vs *ActorSet) AppendActorAddedCallback(callback ActorCallbackFn) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.onActorAdded = append(vs.onActorAdded, callback)
}

// AppendActorRemovedCallback sets the callback to be invoked when an Actor is removed.
func (vs *ActorSet) AppendActorRemovedCallback(callback ActorCallbackFn) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.onActorRemoved = append(vs.onActorRemoved, callback)
}

// AddActor now not only adds the Actor to the set but also initiates a connection.
func (vs *ActorSet) AddActor(ctx context.Context, account Account, shareIndex int) error {
	vs.mutex.Lock()

	// Check if the Actor already exists
	if _, exists := vs.actors[account.PeerID()]; exists {
		vs.mutex.Unlock()
		vs.logger.Debug("Attempted to add an existing actor", zap.String("actor_id", account.PeerID().String()))
		return ActorAlreadyExists
	}

	actor := NewActor(account, shareIndex, vs.logger)
	vs.actors[account.PeerID()] = actor
	vs.logger.Info("Added actor to actor set", zap.String("actor_id", account.PeerID().String()))

	vs.mutex.Unlock()

	// Invoke the callback if set
	if vs.onActorAdded != nil {
		for _, callback := range vs.onActorAdded {
			if err := callback(ctx, actor, false); err != nil {
				vs.logger.Error(
					"Failed to add actor to actor set",
					zap.Error(err),
					zap.String("actor_id", actor.account.PeerID().String()),
				)
				return err
			}
		}
	}

	return nil
}

// RemoveActor removes an Actor from the set.
func (vs *ActorSet) RemoveActor(ctx context.Context, id peer.ID, force bool) error {
	vs.mutex.Lock()

	actor, exists := vs.actors[id]
	if !exists {
		vs.logger.Warn("Attempted to remove a non-existent actor", zap.String("actor_id", id.String()))
		vs.mutex.Unlock()
		return fmt.Errorf("actor does not exist")
	}

	delete(vs.actors, id)
	vs.logger.Debug("Removed actor", zap.String("actor_id", id.String()))

	// Remove leader if this actor was a leader for any role
	for role, leaderID := range vs.leaders {
		if leaderID == id {
			delete(vs.leaders, role)
			vs.logger.Info("Removed leader for role due to actor removal", zap.String("role", string(role)), zap.String("actor_id", id.String()))

			// Retrieve constituent roles if the role is composite
			compositeRoles := rbac.GetCompositeRolesByRole(role)
			for _, constituentRole := range compositeRoles {
				delete(vs.leaders, constituentRole)
				vs.logger.Info("Removed leader for constituent role due to composite role leader removal", zap.String("role", string(constituentRole)), zap.String("actor_id", id.String()))
			}
		}
	}

	// Copy the callbacks to avoid holding the lock during callback execution
	callbacks := make([]ActorCallbackFn, len(vs.onActorRemoved))
	copy(callbacks, vs.onActorRemoved)

	vs.mutex.Unlock()

	// Invoke the callbacks outside the lock
	if callbacks != nil {
		for _, callback := range callbacks {
			if err := callback(ctx, actor, force); err != nil {
				vs.logger.Error(
					"Failed to remove actor from actor set",
					zap.Error(err),
					zap.String("actor_id", actor.account.PeerID().String()),
				)
				return err
			}
		}
	}

	return nil
}

// GetActor retrieves an Actor by peer ID.
func (vs *ActorSet) GetActor(id peer.ID) *Actor {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	return vs.actors[id]
}

// GetActors returns a list of all Actors.
func (vs *ActorSet) GetActors() []*Actor {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	actors := make([]*Actor, 0, len(vs.actors))
	for _, v := range vs.actors {
		actors = append(actors, v)
	}
	return actors
}

// GetActorIDs returns a slice of all Actor peer IDs.
func (vs *ActorSet) GetActorIDs() []peer.ID {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	ids := make([]peer.ID, 0, len(vs.actors))
	for id := range vs.actors {
		ids = append(ids, id)
	}
	return ids
}

// ElectLeader selects a leader for a specific role based on utility scores.
// If the role is a composite role, it also sets the leader for its constituent roles.
func (vs *ActorSet) ElectLeader(role types.Role, metricsCollector Collector) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	// Filter actors that have the specified role
	eligibleActors := make(map[peer.ID]*Actor)
	for id, actor := range vs.actors {
		if rbac.HasRole(actor.account.Roles(), role) {
			eligibleActors[id] = actor
		}
	}

	if len(eligibleActors) == 0 {
		vs.logger.Debug("No available actors to elect the leader for role", zap.String("role", string(role)))
		return
	}

	var highestScore float64
	var selectedLeaderID peer.ID
	var leaderFound bool

	for id := range eligibleActors {
		// Calculate utility score adjusted by role weight
		score := metricsCollector.CalculateUtilityScore(id)
		vs.logger.Debug("Calculated weighted utility score", zap.String("actor_id", id.String()), zap.Float64("score", score))

		// Select the actor with the highest weighted utility score
		if !leaderFound || score > highestScore {
			highestScore = score
			selectedLeaderID = id
			leaderFound = true
		}
	}

	if !leaderFound {
		vs.logger.Warn("No leader found based on utility scores for role", zap.String("role", string(role)))
		return
	}

	// Assign leader to the specified role
	vs.leaders[role] = selectedLeaderID
	vs.logger.Info("Elected leader for role based on weighted utility score and roles", zap.String("role", string(role)), zap.String("leader_id", selectedLeaderID.String()), zap.Float64("score", highestScore))

	// Retrieve constituent roles if the role is composite
	compositeRoles := rbac.GetCompositeRolesByRole(role)
	for _, constituentRole := range compositeRoles {
		vs.leaders[constituentRole] = selectedLeaderID
		vs.logger.Info("Elected leader for constituent role based on composite role leader", zap.String("role", string(constituentRole)), zap.String("leader_id", selectedLeaderID.String()))
	}
}

// SetLeader sets the leader for a specific role (useful for tests or dynamic leader changes).
// If the role is a composite role, it also sets the leader for its constituent roles.
func (vs *ActorSet) SetLeader(role types.Role, id peer.ID) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	if _, exists := vs.actors[id]; exists {
		vs.leaders[role] = id
		vs.logger.Debug("Set leader for role", zap.String("role", string(role)), zap.String("leader_id", id.String()))

		// Retrieve constituent roles if the role is composite
		compositeRoles := rbac.GetCompositeRolesByRole(role)
		for _, constituentRole := range compositeRoles {
			vs.leaders[constituentRole] = id
			vs.logger.Debug("Set leader for constituent role based on composite role leader", zap.String("role", string(constituentRole)), zap.String("leader_id", id.String()))
		}
	} else {
		vs.logger.Warn("Attempted to set unknown actor as leader for role", zap.String("role", string(role)), zap.String("actor_id", id.String()))
	}
}

// CurrentLeader returns the current leader for any of the specified roles.
func (vs *ActorSet) CurrentLeader(roles ...types.Role) *Actor {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	for _, role := range roles {
		leaderID, exists := vs.leaders[role]
		if exists {
			return vs.actors[leaderID]
		}
		// No warning here since we only need a leader for one of the roles.
	}

	// Log a warning if no leader is found for any of the roles
	vs.logger.Debug("No leader elected for any of the specified roles")
	return nil
}

// ElectLeaders elects leaders for all eligible roles.
func (vs *ActorSet) ElectLeaders(metricsCollector Collector) {
	for _, role := range rbac.EligibleConsensusRoles {
		vs.ElectLeader(role, metricsCollector)
	}
}

// IsLeader checks if a given Actor is the current leader for any of the specified roles.
func (vs *ActorSet) IsLeader(id peer.ID, roles ...types.Role) bool {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	for _, role := range roles {
		leaderID, exists := vs.leaders[role]
		if exists && leaderID == id {
			return true
		}
	}

	return false
}

// IsLeaderForAnyRole checks if a given Actor is the leader for any role.
func (vs *ActorSet) IsLeaderForAnyRole(id peer.ID) bool {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	for role, leaderID := range vs.leaders {
		if leaderID == id {
			vs.logger.Debug("Actor is leader for role", zap.String("role", string(role)), zap.String("actor_id", id.String()))
			return true
		}
	}
	return false
}

// GetRolesForLeader returns a slice of roles for which the given Actor is the leader.
func (vs *ActorSet) GetRolesForLeader(id peer.ID) []types.Role {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	var roles []types.Role
	for role, leaderID := range vs.leaders {
		if leaderID == id {
			roles = append(roles, role)
		}
	}
	return roles
}

// QuorumSize returns the number of Actor's required for quorum.
func (vs *ActorSet) QuorumSize() int {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	if vs.quorumThreshold > 0 {
		return vs.quorumThreshold
	}
	// Default quorum threshold is two-thirds of the total Actor's plus one
	return (2*len(vs.actors))/3 + 1
}

// SetQuorumThreshold sets the quorum threshold for block finalization.
func (vs *ActorSet) SetQuorumThreshold(threshold int) {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()
	vs.quorumThreshold = threshold
	vs.logger.Debug("Set quorum threshold", zap.Int("threshold", threshold))
}

// IsSequencer checks if a given peer ID is a sequencer.
func (vs *ActorSet) IsSequencer(id peer.ID) bool {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	_, exists := vs.actors[id]
	return exists
}

// IsValidator checks if a given peer ID is a Actor's.
func (vs *ActorSet) IsValidator(id peer.ID) bool {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	_, exists := vs.actors[id]
	return exists
}

// GetActorByUserID retrieves a Actor by user ID.
func (vs *ActorSet) GetActorByUserID(userID string) *Actor {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	for _, actor := range vs.actors {
		if actor.account.ID() == userID { // Assuming account.ID is the user ID
			return actor
		}
	}
	return nil
}

// AssignShareIndex helper function used to assign unique share indices to Actor's
func (vs *ActorSet) AssignShareIndex(id peer.ID) int {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	// Get a sorted list of Actor IDs
	ids := make([]string, 0, len(vs.actors))
	for vid := range vs.actors {
		ids = append(ids, vid.String())
	}
	sort.Strings(ids)

	// Assign share indices based on sorted order
	for index, vid := range ids {
		if vid == id.String() {
			return index + 1 // Share indices start from 1
		}
	}
	// If the ID is not found, return -1 (should not happen)
	return -1
}

// GetActorsCount returns the current number of Actor's in the ActorSet.
func (vs *ActorSet) GetActorsCount() int {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()
	return len(vs.actors)
}

// WaitForActor waits until the specified Actor is present in the ActorSet or times out.
func (vs *ActorSet) WaitForActor(pid peer.ID, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		vs.mutex.RLock()
		_, exists := vs.actors[pid]
		vs.mutex.RUnlock()
		if exists {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("actor %s not found in actor set within timeout", pid.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}
