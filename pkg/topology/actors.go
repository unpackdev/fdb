// pkg/topology/actors.go
package topology

import (
	"context"
	"fmt"
	"time"

	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/pkg/accounts"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/networking"
	"github.com/unpackdev/fdb/pkg/packets"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/share"
	"github.com/unpackdev/fdb/pkg/types"
	"go.dedis.ch/kyber/v4"
	"go.uber.org/zap"
)

// peerSet is a helper type to manage a set of peer IDs.
type peerSet map[peer.ID]struct{}

// add adds a peer ID to the set.
func (ps peerSet) add(p peer.ID) {
	ps[p] = struct{}{}
}

// remove removes a peer ID from the set.
func (ps peerSet) remove(p peer.ID) {
	delete(ps, p)
}

// contains checks if a peer ID is in the set.
func (ps peerSet) contains(p peer.ID) bool {
	_, exists := ps[p]
	return exists
}

// ActorAddedCallback is a function type that is called when an actor is added.
type ActorAddedCallback func(actor *Actor) error

// ActorRemovedCallback is a function type that is called when an actor is removed.
type ActorRemovedCallback func(actor *Actor) error

// Actor represents both the node's own actor information and that of connected peers.
type Actor struct {
	ID                  peer.ID
	Address             types.Address
	Addresses           []multiaddr.Multiaddr
	Roles               []types.Role
	SupportedTransports []types.TransportType
	SupportedProtocols  []types.ProtocolType
	SupportedSigners    []types.SignerType
	ConsensusPublicKey  kyber.Point
	PublicKey           libp2pCrypto.PubKey
	ConsensusActor      *share.Actor // Optional: Link to share.Actor for consensus participation
}

// pendingPeer represents a peer awaiting verification.
type pendingPeer struct {
	addresses []multiaddr.Multiaddr
}

// Actors manages actor information within the topology.
type Actors struct {
	peers           map[peer.ID]*Actor
	pendingPeers    map[peer.ID]*pendingPeer
	roleIndex       map[types.Role]peerSet // Indexed by roles
	account         share.Account
	mutex           deadlock.RWMutex
	logger          logger.Logger
	network         *networking.Network
	rbac            *rbac.Manager
	consensusSet    *share.ActorSet
	metrics         share.Collector  // General metrics collector from metrics package
	topologyMetrics *TopologyMetrics // Topology-specific metrics
	self            *Actor
	onPeerEvent     func() // Notifier function to signal peer events

	// Callback slices
	actorAddedCallbacks   []ActorAddedCallback
	actorRemovedCallbacks []ActorRemovedCallback
}

// NewActors creates a new Actors instance and registers event handlers.
// The onPeerEvent function is called whenever a peer is added or removed.
func NewActors(logger logger.Logger, account share.Account, network *networking.Network, rbacMgr *rbac.Manager, collector share.Collector, consensusSet *share.ActorSet, onPeerEvent func()) (*Actors, error) {

	a := &Actors{
		peers:        make(map[peer.ID]*Actor),
		pendingPeers: make(map[peer.ID]*pendingPeer),
		roleIndex:    make(map[types.Role]peerSet), // Initialize roleIndex
		account:      account,
		logger:       logger,
		self: &Actor{
			ID:                  account.PeerID(),
			Address:             account.Address(),
			Addresses:           network.Host().Addrs(),
			Roles:               account.Roles(),
			SupportedTransports: []types.TransportType{},   // TODO: Implement supported transports
			SupportedProtocols:  []types.ProtocolType{},    // TODO: Implement supported protocols
			PublicKey:           account.MasterPublicKey(), // Internally external PublicKey is MasterPublicKey of the account
		},
		network:      network,
		rbac:         rbacMgr,
		consensusSet: consensusSet,
		metrics:      collector,
		onPeerEvent:  onPeerEvent,
	}

	// Initialize topology metrics if we have access to a meter via network observability
	if network != nil && network.Observability != nil && network.Observability.Meter != nil {
		topologyMetrics, err := InitializeMetrics(network.Observability.Meter)
		if err != nil {
			logger.Error("Failed to initialize topology metrics", zap.Error(err))
		} else {
			a.topologyMetrics = topologyMetrics
			// Record self actor metrics
			ctx := context.Background()
			for _, role := range account.Roles() {
				topologyMetrics.RecordActorAdded(ctx, 1, role.String())
			}
		}
	}

	a.logger.Info("Initialized self actor in topology", "peer_id", a.self.ID.String())

	// Register event handlers for peer connections and disconnections.
	network.RegisterPeerConnectedHandler(a.HandlePeerConnected)
	network.RegisterPeerDisconnectedHandler(a.HandlePeerDisconnected)

	// Register PeerConnectedCallback to handle new peer connections via mDNS
	network.MdnsNotifier().SetPeerConnectedCallback(func(ctx context.Context, pi peer.AddrInfo) {
		a.logger.Info("mDNS peer connected", "peer_id", pi.ID.String())

		// Handle the peer connection through Actors
		if err := a.HandlePeerConnected(ctx, pi); err != nil && !errors.Is(err, ErrPeerAlreadyExists) {
			a.logger.Error("Actors failed to handle peer connection via mDNS", "peer_id", pi.ID.String(), "error", err.Error())
		}
	})

	// Register PeerDisconnectedCallback to handle peer disconnections via mDNS
	network.MdnsNotifier().SetPeerDisconnectedCallback(func(ctx context.Context, id peer.ID) {
		a.logger.Info("mDNS peer disconnected", "peer_id", id.String())

		// Notify Actors to handle the disconnection
		if err := a.HandlePeerDisconnected(ctx, id); err != nil {
			a.logger.Error("Actors failed to handle peer disconnection via mDNS", "peer_id", id.String(), "error", err.Error())
		}
	})

	// Add self to peers
	a.peers[a.self.ID] = a.self

	// Index self roles
	for _, role := range a.self.Roles {
		if a.roleIndex[role] == nil {
			a.roleIndex[role] = make(peerSet)
		}
		a.roleIndex[role].add(a.self.ID)
	}

	// TODO: One serious missing link here is DPoS and how to make this happen.
	if rbac.IsConsensusSupportedByRoles(a.self.Roles...) {
		a.self.ConsensusActor = share.NewActor(a.account, a.assignShareIndex(a.self.ID), a.logger)
		if err := a.consensusSet.AddActor(context.Background(), account, a.self.ConsensusActor.ShareIndex()); err != nil {
			a.logger.Error(
				"Failure to add self actor into the consensus actor set",
				zap.Error(err),
				zap.String("actor_id", a.self.ID.String()),
			)
			return nil, err
		}
	}

	// Notify peer availability
	if a.onPeerEvent != nil {
		a.onPeerEvent()
	}

	return a, nil
}

// RegisterActorAddedCallback allows external packages to register a callback
// that is invoked whenever an actor is added.
func (a *Actors) RegisterActorAddedCallback(cb ActorAddedCallback) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.actorAddedCallbacks = append(a.actorAddedCallbacks, cb)
}

// RegisterActorRemovedCallback allows external packages to register a callback
// that is invoked whenever an actor is removed.
func (a *Actors) RegisterActorRemovedCallback(cb ActorRemovedCallback) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.actorRemovedCallbacks = append(a.actorRemovedCallbacks, cb)
}

// HandlePeerConnected is invoked when a new peer connects.
func (a *Actors) HandlePeerConnected(ctx context.Context, peerInfo peer.AddrInfo) error {
	// Skip self-peer to prevent handling your own connection
	if peerInfo.ID == a.account.PeerID() {
		return nil
	}

	a.logger.Info("Handling new peer connection", "peer_id", peerInfo.ID.String())

	// RBAC Check: Verify if self has PermissionAddPeer
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionAddPeer) {
		a.logger.Warn(
			"Insufficient permissions to add peer",
			"required_permission", rbac.PermissionAddPeer,
			"roles", a.self.Roles,
		)
		return ErrInsufficientPermissions
	}

	// Check if the peer is already in the topology or pending
	if a.HasPeer(peerInfo.ID) {
		a.logger.Debug("Peer already exists in topology or is pending", "peer_id", peerInfo.ID.String())
		return ErrPeerAlreadyExists
	}

	// Mark the peer as pending before sending any messages to avoid race conditions
	a.markPeerAsPending(peerInfo.ID, peerInfo.Addrs)

	// Send an ActorPacket with Status 0 (proposed) to request actor information
	//if err := a.SendActorRequestPacket(ctx, peerInfo.ID); err != nil {
	//	if !networking.IsConnectionRefused(err) {
	//		a.logger.Error("Failed to send ActorPacket (proposed) to peer", "peer_id", peerInfo.ID.String(), "error", err.Error())
	//	}
	//	return a.removePendingPeer(peerInfo.ID)
	//}

	return nil
}

// HandlePeerDisconnected is invoked when a peer disconnects.
func (a *Actors) HandlePeerDisconnected(ctx context.Context, peerID peer.ID) error {
	a.logger.Info("Handling peer disconnection", "peer_id", peerID.String())

	// RBAC Check: Verify if self has PermissionRemovePeer
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionRemovePeer) {
		a.logger.Warn("Insufficient permissions to remove peer", "required_permission", rbac.PermissionRemovePeer)
		return ErrInsufficientPermissions
	}

	// Attempt to remove the peer
	err := a.RemovePeer(ctx, peerID, true)
	if err != nil {
		if errors.Is(err, ErrPeerNotFound) {
			// Peer already removed; log info and return
			a.logger.Debug("Peer already removed from topology", "peer_id", peerID.String())
			// Remove from pendingPeers if present
			return a.removePendingPeer(peerID)
		}
		// Other errors
		a.logger.Error("Failed to remove disconnected peer from topology", "peer_id", peerID.String(), "error", err.Error())
		return errors.Wrapf(err, "failed to remove disconnected peer %s from topology", peerID.String())
	}

	// Remove from pendingPeers if present
	return a.removePendingPeer(peerID)
}

// AddPeer adds a peer to the topology with verified information.
func (a *Actors) AddPeer(peerID peer.ID, address types.Address, addresses []multiaddr.Multiaddr, roles []types.Role, transports []types.TransportType, protocols []types.ProtocolType, signers []types.SignerType, pubKey libp2pCrypto.PubKey) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// RBAC Check
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionAddPeer) {
		a.logger.Warn("Insufficient permissions to add peer", "required_permission", rbac.PermissionAddPeer)
		return ErrInsufficientPermissions
	}

	if _, exists := a.peers[peerID]; exists {
		return ErrPeerAlreadyExists
	}

	// Validate PublicKey
	if pubKey == nil {
		return fmt.Errorf("invalid public key for peer %s: public key cannot be nil", peerID)
	}

	// Create the Actor struct
	actor := &Actor{
		ID:                  peerID,
		Address:             address,
		Addresses:           addresses,
		Roles:               roles,
		SupportedTransports: transports,
		SupportedProtocols:  protocols,
		SupportedSigners:    signers,
		PublicKey:           pubKey,
	}

	// Add the peer to the peers map
	a.peers[peerID] = actor

	// Update roleIndex
	for _, role := range roles {
		if a.roleIndex[role] == nil {
			a.roleIndex[role] = make(peerSet)
		}
		a.roleIndex[role].add(peerID)
	}

	// Record metrics for added peer
	if a.topologyMetrics != nil {
		// Use background context for metrics
		ctx := context.Background()

		// Record actor addition for each role
		for _, role := range roles {
			a.topologyMetrics.RecordActorAdded(ctx, 1, role.String())
		}
		// Record actor connection
		a.topologyMetrics.RecordActorConnection(ctx, 1)

		// If this is a consensus actor, record that too
		if actor.ConsensusActor != nil {
			a.topologyMetrics.RecordConsensusActor(ctx, 1)
		}
	}

	// Notify peer availability
	if a.onPeerEvent != nil {
		a.onPeerEvent()
	}

	a.logger.Info("Added peer to topology", "peer_id", peerID.String(), "num_addresses", len(addresses))

	// Invoke ActorAddedCallbacks outside the lock
	a.mutex.Unlock()
	a.invokeActorAddedCallbacks(actor)
	a.mutex.Lock()

	return nil
}

// RemovePeer removes a peer from the topology.
func (a *Actors) RemovePeer(ctx context.Context, peerID peer.ID, force bool) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// RBAC Check: Verify if self has PermissionRemovePeer
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionRemovePeer) {
		a.logger.Warn("Insufficient permissions to remove peer", "required_permission", rbac.PermissionRemovePeer)
		return ErrInsufficientPermissions
	}

	actor, exists := a.peers[peerID]
	if !exists {
		return ErrPeerNotFound
	}

	// Remove the peer from the peers map
	delete(a.peers, peerID)

	// Update roleIndex by removing the peer from all associated roles
	for _, role := range actor.Roles {
		if rPeerSet, rExists := a.roleIndex[role]; rExists {
			rPeerSet.remove(peerID)
			// If no peers remain for this role, delete the role entry
			if len(rPeerSet) == 0 {
				delete(a.roleIndex, role)
			}
		}
	}

	// Remove from share.ActorSet if it was a consensus peer
	if actor.ConsensusActor != nil {
		if raErr := a.consensusSet.RemoveActor(ctx, peerID, force); raErr != nil {
			return errors.Wrapf(raErr, "failed to remove actor from consensus set: %s", peerID)
		}
	}

	// Record metrics for removed peer
	if a.topologyMetrics != nil {
		// Use the provided context for metrics
		// Record actor removal for each role
		for _, role := range actor.Roles {
			a.topologyMetrics.RecordActorRemoved(ctx, 1, role.String())
		}
		// Record actor disconnection
		a.topologyMetrics.RecordActorDisconnection(ctx, 1)

		// If this was a consensus actor, record that too
		if actor.ConsensusActor != nil {
			a.topologyMetrics.RecordConsensusActor(ctx, -1)
		}

		// Simplified topology change latency - just record current operation time
		a.topologyMetrics.RecordTopologyChangeLatency(ctx, time.Millisecond*10) // Use a nominal value
	}

	// Notify peer availability
	if a.onPeerEvent != nil {
		a.onPeerEvent()
	}

	a.logger.Info("Removed peer from topology", "peer_id", peerID.String())

	// Invoke ActorRemovedCallbacks outside the lock
	a.mutex.Unlock()
	a.invokeActorRemovedCallbacks(actor)
	a.mutex.Lock()

	return nil
}

// GetPeer retrieves a peer's information from the topology.
func (a *Actors) GetPeer(peerID peer.ID) (*Actor, error) {
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

	return actor, nil
}

// GetPeers returns a list of all peers in the topology.
func (a *Actors) GetPeers() ([]*Actor, error) {
	// RBAC Check: Verify if self has PermissionViewTopology
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionViewTopology) {
		a.logger.Warn("Insufficient permissions to view topology", "required_permission", rbac.PermissionViewTopology)
		return nil, ErrInsufficientPermissions
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	peers := make([]*Actor, 0, len(a.peers))
	for _, actor := range a.peers {
		peers = append(peers, actor)
	}
	return peers, nil
}

// GetPeersCount returns a count of all peers in the topology.
func (a *Actors) GetPeersCount() (int, error) {
	// RBAC Check: Verify if self has PermissionViewTopology
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionViewTopology) {
		a.logger.Warn("Insufficient permissions to view topology", "required_permission", rbac.PermissionViewTopology)
		return 0, ErrInsufficientPermissions
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return len(a.peers), nil
}

// HasPeer checks if a peer is already in the topology or pending.
func (a *Actors) HasPeer(peerID peer.ID) bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	_, exists := a.peers[peerID]
	if exists {
		return true
	}

	_, pending := a.pendingPeers[peerID]
	return pending
}

// markPeerAsPending adds a peer to the pendingPeers map.
func (a *Actors) markPeerAsPending(peerID peer.ID, addresses []multiaddr.Multiaddr) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.pendingPeers == nil {
		a.pendingPeers = make(map[peer.ID]*pendingPeer)
	}

	a.pendingPeers[peerID] = &pendingPeer{
		addresses: addresses,
	}

	// Record pending peer metrics
	if a.topologyMetrics != nil {
		a.topologyMetrics.RecordPendingPeer(context.Background(), 1)
	}

	a.logger.Info("Marked peer as pending", "peer_id", peerID.String())
}

// removePendingPeer removes a peer from the pendingPeers map.
func (a *Actors) removePendingPeer(peerID peer.ID) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.pendingPeers != nil {
		// Check if peer was actually pending before removal
		_, wasPending := a.pendingPeers[peerID]
		delete(a.pendingPeers, peerID)

		// If the peer was pending and is now being removed, it's likely a timeout
		if wasPending {
			a.logger.Info("Removed peer from pendingPeers", "peer_id", peerID.String())

			if a.topologyMetrics != nil {
				a.topologyMetrics.RecordPendingPeerTimeout(context.Background(), 1)
			}
		}
	}

	return nil
}

// verifyAndAddPeer moves a peer from pendingPeers to peers after verification.
func (a *Actors) verifyAndAddPeer(peerID peer.ID, actorInfo *packets.ActorPacket) error {
	startTime := time.Now() // Start timing the verification process

	// Record verification attempt
	if a.topologyMetrics != nil {
		a.topologyMetrics.RecordActorVerification(context.Background(), 1)
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if the peer is in pendingPeers
	pending, exists := a.pendingPeers[peerID]
	if !exists {
		return errors.Wrapf(ErrPeerNotInPendingPeers, "peer %s not found in pendingPeers", peerID.String())
	}

	// RBAC Check: Verify if self has PermissionProcessActorPacket
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionProcessActorPacket) {
		a.logger.Warn("Insufficient permissions to process actor packet", "required_permission", rbac.PermissionProcessActorPacket)
		return ErrInsufficientPermissions
	}

	// Unmarshal and set the PublicKey
	publicKey, err := libp2pCrypto.UnmarshalPublicKey(actorInfo.PublicKey)
	if err != nil {
		return errors.Wrapf(ErrInvalidMessageType, "invalid PublicKey for peer %s: %w", peerID.String(), err)
	}

	// Create the Actor struct with verified information
	actor := &Actor{
		ID:                  peerID,
		Address:             actorInfo.Address,
		Addresses:           pending.addresses,
		Roles:               actorInfo.Roles,
		SupportedTransports: actorInfo.SupportedTransports,
		SupportedProtocols:  actorInfo.SupportedProtocols,
		SupportedSigners:    actorInfo.SupportedSigners,
		PublicKey:           publicKey,
	}

	// Add the peer to the peers map
	a.peers[peerID] = actor

	// Update roleIndex
	for _, role := range actor.Roles {
		if a.roleIndex[role] == nil {
			a.roleIndex[role] = make(peerSet)
		}
		a.roleIndex[role].add(peerID)
	}

	// TODO: One serious missing link here is DPoS and how to make this happen.
	// Basically right now in the system you can create new account, say it's validator and start the
	// validator without any type of the extra security mechanism such as DPoS to ensure that peers cannot
	// just like that access consensus protocol...
	if rbac.IsConsensusSupportedByRoles(actor.Roles...) {
		//// At this moment, we need to check if this actor is staked or not.
		//// If not, we are not going to add actor to the consensus ruleset...
		//// If current node is not part of the consensus, it should not check for staked amounts as it's not
		//// relevant for it...
		//if rbac.IsConsensusSupportedByRoles(a.self.Roles...) {
		//	if stakedAmount, saErr := a.stakingMgr.GetStakedAmount(actor.Address); saErr != nil {
		//		a.logger.Error(
		//			"Failed to check for incoming actor staked amount",
		//			zap.Error(saErr),
		//			zap.String("actor_id", actor.ID.String()),
		//			zap.Any("actor_roles", actor.Roles),
		//			zap.String("actor_address", actor.Address.Hex()),
		//		)
		//		return errors.Wrap(saErr, "failed to check for incoming actor staked amount")
		//	} else if stakedAmount.Cmp(uint256.NewInt(0).SetUint64(staking.MinimalStake)) < 0 {
		//		a.logger.Error(
		//			"Failed to check for minimal actor staked amount - not enough to participate in consensus protocol",
		//			zap.Error(saErr),
		//			zap.String("actor_id", actor.ID.String()),
		//			zap.Any("actor_roles", actor.Roles),
		//			zap.String("actor_address", actor.Address.Hex()),
		//			zap.Any("staked_amount", stakedAmount),
		//		)
		//		return fmt.Errorf("not enough staked balance to participate in consensus protocol: %d", stakedAmount)
		//	}
		//}

		actor.ConsensusActor = share.NewActor(a.account, a.assignShareIndex(peerID), a.logger)

		account, aErr := accounts.NewConsensusAccount(a.logger, peerID, actorInfo.Address, publicKey, actor.Roles)
		if aErr != nil {
			a.logger.Error(
				"Failed to create consensus topology account",
				zap.Error(aErr),
				zap.String("actor_id", peerID.String()),
			)
			return errors.Wrap(aErr, "failed to create new topology consensus account")
		}

		shareIndex := a.assignShareIndex(peerID)
		if aaErr := a.consensusSet.AddActor(context.Background(), account, shareIndex); aaErr != nil {
			if !errors.Is(aaErr, share.ActorAlreadyExists) {
				a.logger.Error(
					"Failure to add actor into the consensus actor set",
					zap.Error(aaErr),
					zap.String("actor_id", a.self.ID.String()),
				)
			}
			return errors.Wrap(aaErr, "failed to add actor into the consensus actor set")
		}

	}

	// Remove from pendingPeers
	delete(a.pendingPeers, peerID)

	// Notify peer availability
	if a.onPeerEvent != nil {
		a.onPeerEvent()
	}

	a.logger.Info("Peer verified and added to topology", "peer_id", peerID.String())

	// Record successful verification metrics
	if a.topologyMetrics != nil {
		ctx := context.Background()
		// Record verification latency
		a.topologyMetrics.RecordActorVerificationLatency(ctx, time.Since(startTime))

		// Record actor addition for each role
		for _, role := range actorInfo.Roles {
			a.topologyMetrics.RecordActorAdded(ctx, 1, role.String())
		}

		// Record connection
		a.topologyMetrics.RecordActorConnection(ctx, 1)

		// If this is a consensus actor, record that too
		if actor.ConsensusActor != nil {
			a.topologyMetrics.RecordConsensusActor(ctx, 1)
		}
	}

	// Invoke ActorAddedCallbacks outside the lock
	a.mutex.Unlock()
	a.invokeActorAddedCallbacks(actor)
	a.mutex.Lock()

	return nil
}

// SendActorRequestPacket sends an ActorPacket with Status 0 (proposed) to request actor information.
//func (a *Actors) SendActorRequestPacket(ctx context.Context, peerID peer.ID) error {
//	// RBAC Check: Verify if self has PermissionSendActorPacket
//	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionSendActorPacket) {
//		a.logger.Warn("Insufficient permissions to send actor packet", "required_permission", rbac.PermissionSendActorPacket)
//		return ErrInsufficientPermissions
//	}
//
//	// Create an ActorPacket with Status 0 (proposed) and marshal PublicKey
//	apk, apkErr := a.account.MarshalPublicKey()
//	if apkErr != nil {
//		return errors.Wrap(apkErr, "cannot marshal actor (account) public key")
//	}
//
//	dkgPkBytes, dpbErr := a.account.DkgPublicKey().MarshalBinary()
//	if dpbErr != nil {
//		return errors.Wrap(dpbErr, "failed to marshal account consensus DKG public key")
//	}
//
//	ap := &packets.ActorPacket{
//		Address:             a.self.Address,
//		Status:              0, // 0 => proposed
//		Message:             "Requesting actor information.",
//		Roles:               a.self.Roles,
//		SupportedProtocols:  a.self.SupportedProtocols,
//		SupportedTransports: a.self.SupportedTransports,
//		SupportedSigners:    a.self.SupportedSigners,
//		ConsensusPublicKey:  dkgPkBytes,
//		PublicKey:           apk,
//	}
//
//	// Serialize ActorPacket
//	payload, err := ap.Serialize()
//	if err != nil {
//		return errors.Wrap(err, "failed to serialize ActorPacket")
//	}
//
//	// Create a NetworkPacket with PacketTypeActorPacket
//	networkPacket := &packets.NetworkPacket{
//		Type:       packets.ActorPacketType,
//		SenderID:   a.self.ID, // Use self Actor's ID
//		ReceiverID: peerID,
//		Payload:    payload,
//	}
//
//	// Serialize without signature for signing
//	signatureSerializedBytes, sbErr := networkPacket.SerializeWithoutSignature()
//	if sbErr != nil {
//		return errors.Wrap(sbErr, "failed to serialize network packet without signature")
//	}
//
//	// Sign the packet
//	signature, sErr := a.account.MasterPrivateKey().Sign(signatureSerializedBytes)
//	if sErr != nil {
//		return errors.Wrap(sErr, "failed to sign network packet")
//	}
//	networkPacket.Signature = signature
//
//	// Serialize the full NetworkPacket
//	serializedPacket, err := networkPacket.Serialize()
//	if err != nil {
//		return errors.Wrap(err, "failed to serialize NetworkPacket")
//	}
//
//	// Send the serialized NetworkPacket using networking.SendMessage
//	if err := a.network.SendMessage(ctx, a.network.ProtocolID, peerID, serializedPacket); err != nil {
//		return errors.Wrapf(err, "failed to send ActorPacket (proposed) to peer %s", peerID.String())
//	}
//
//	a.logger.Info("Sent ActorPacket (proposed)", "peer_id", peerID.String())
//	return nil
//}

// ProcessActorPacket processes the received ActorPacket based on its Status.
//func (a *Actors) ProcessActorPacket(ctx context.Context, peerID peer.ID, ap *packets.ActorPacket) error {
//	// RBAC Check: Verify if self has PermissionProcessActorPacket
//	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionProcessActorPacket) {
//		a.logger.Warn("Insufficient permissions to process actor packet", "required_permission", rbac.PermissionProcessActorPacket)
//		return ErrInsufficientPermissions
//	}
//
//	switch ap.Status {
//	case 0: // proposed
//		a.logger.Info("Received ActorPacket with Status 'proposed' from peer", "peer_id", peerID.String())
//
//		// Retrieve own PublicKey outside the lock
//		ownPublicKey, err := a.account.MarshalPublicKey()
//		if err != nil {
//			return errors.Wrap(err, "failed to marshal own PublicKey")
//		}
//
//		dkgPkBytes, dpbErr := a.account.DkgPublicKey().MarshalBinary()
//		if dpbErr != nil {
//			return errors.Wrap(dpbErr, "failed to marshal account consensus DKG public key")
//		}
//
//		// Create an approved ActorPacket
//		approvedPacket := &packets.ActorPacket{
//			Address:             a.account.Address(),
//			Status:              1, // approved
//			Message:             "Approved",
//			Roles:               a.account.Roles(),
//			SupportedTransports: a.account.SupportedTransports(),
//			SupportedProtocols:  a.account.SupportedProtocols(),
//			SupportedSigners:    a.account.SupportedSigners(),
//			ConsensusPublicKey:  dkgPkBytes,
//			PublicKey:           ownPublicKey,
//		}
//
//		// Send the approved ActorPacket using the provided context
//		if err := a.SendActorResponsePacket(ctx, peerID, approvedPacket); err != nil {
//			a.logger.Error("Failed to send approved ActorPacket", "peer_id", peerID.String(), "error", err.Error())
//			// Optionally, remove from pendingPeers if sending fails
//			a.removePendingPeer(peerID)
//			return errors.Wrapf(err, "failed to send approved ActorPacket to peer %s", peerID.String())
//		}
//
//		a.logger.Info("Sent approved ActorPacket to peer", "peer_id", peerID.String())
//
//	case 1: // approved
//
//		if err := a.verifyAndAddPeer(peerID, ap); err != nil && !errors.Is(err, share.ActorAlreadyExists) {
//			a.logger.Info("Peer not found in pendingPeers, handling as new verification", "peer_id", peerID.String(), "error", err.Error())
//
//			// Handle the case where the peer is not in pendingPeers
//			// This could happen due to simultaneous connections or race conditions
//			a.markPeerAsPending(peerID, a.self.Addresses)
//
//			// Retry verification and addition
//			err = a.verifyAndAddPeer(peerID, ap)
//			if err != nil && !errors.Is(err, share.ActorAlreadyExists) {
//				a.logger.Error("Failed to verify and add peer after marking as pending", "peer_id", peerID.String(), "error", err.Error())
//				return errors.Wrapf(err, "failed to verify and add peer %s after marking as pending", peerID.String())
//			}
//		}
//
//	case 2: // rejected
//		// Handle rejection
//		a.logger.Warn("Peer actor info rejected", "peer_id", peerID.String(), "message", ap.Message)
//		// RBAC Check: Verify if self has PermissionRemovePeer to disconnect
//		if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionRemovePeer) {
//			a.logger.Warn("Insufficient permissions to remove rejected peer", "required_permission", rbac.PermissionRemovePeer)
//			return ErrInsufficientPermissions
//		}
//
//		// Disconnect the peer
//		if err := a.network.Host().Network().ClosePeer(peerID); err != nil {
//			a.logger.Error("Failed to disconnect peer after rejection", "peer_id", peerID.String(), "error", err.Error())
//		}
//		// Force removal from peers if present
//		if err := a.RemovePeer(ctx, peerID, true); err != nil {
//			a.logger.Error("Failed to remove rejected peer from topology", "peer_id", peerID.String(), "error", err.Error())
//		}
//		// Also remove from pendingPeers
//		a.removePendingPeer(peerID)
//		return errors.Wrapf(ErrPeerNotFound, "peer %s rejected: %s", peerID.String(), ap.Message)
//
//	default:
//		a.logger.Warn("Received ActorPacket with unknown Status", "peer_id", peerID.String(), "status", ap.Status)
//		return errors.Wrapf(ErrUnknownActorPacketStatus, "unknown ActorPacket status: %d", ap.Status)
//	}
//
//	return nil
//}

// SendActorResponsePacket sends an ActorPacket (e.g., approved) to a peer.
func (a *Actors) SendActorResponsePacket(ctx context.Context, peerID peer.ID, ap *packets.ActorPacket) error {
	// RBAC Check: Verify if self has PermissionSendActorPacket
	if !a.rbac.HasPermission(a.self.Roles, rbac.PermissionSendActorPacket) {
		a.logger.Warn("Insufficient permissions to send actor packet", "required_permission", rbac.PermissionSendActorPacket)
		return ErrInsufficientPermissions
	}

	// Serialize ActorPacket
	payload, err := ap.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize ActorPacket")
	}

	// Create a NetworkPacket with PacketTypeActorPacket
	networkPacket := &packets.NetworkPacket{
		Type:       packets.ActorPacketType,
		SenderID:   a.self.ID, // Use self Actor's ID
		ReceiverID: peerID,
		Payload:    payload,
	}

	// Serialize without signature for signing
	signatureSerializedBytes, sbErr := networkPacket.SerializeWithoutSignature()
	if sbErr != nil {
		return errors.Wrap(sbErr, "failed to serialize network packet without signature")
	}

	// Sign the packet
	signature, sErr := a.account.MasterPrivateKey().Sign(signatureSerializedBytes)
	if sErr != nil {
		return errors.Wrap(sErr, "failed to sign network packet")
	}
	networkPacket.Signature = signature

	// Serialize the full NetworkPacket
	serializedPacket, err := networkPacket.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize NetworkPacket")
	}

	// Send the serialized NetworkPacket using networking.SendMessage
	if err := a.network.SendMessage(ctx, a.network.ProtocolID, peerID, serializedPacket); err != nil {
		return errors.Wrapf(err, "failed to send ActorPacket to peer %s", peerID.String())
	}

	a.logger.Info("Sent ActorPacket", "peer_id", peerID.String(), "status", ap.Status)
	return nil
}

// Helper method to assign a share index
func (a *Actors) assignShareIndex(peerID peer.ID) int {
	return a.consensusSet.AssignShareIndex(peerID)
}

// invokeActorAddedCallbacks invokes all registered ActorAddedCallback functions.
func (a *Actors) invokeActorAddedCallbacks(actor *Actor) {
	a.mutex.RLock()
	callbacks := make([]ActorAddedCallback, len(a.actorAddedCallbacks))
	copy(callbacks, a.actorAddedCallbacks)
	a.mutex.RUnlock()

	for _, cb := range callbacks {
		// Execute each callback in a separate goroutine to prevent blocking
		go cb(actor)
	}
}

// invokeActorRemovedCallbacks invokes all registered ActorRemovedCallback functions.
func (a *Actors) invokeActorRemovedCallbacks(actor *Actor) {
	a.mutex.RLock()
	callbacks := make([]ActorRemovedCallback, len(a.actorRemovedCallbacks))
	copy(callbacks, a.actorRemovedCallbacks)
	a.mutex.RUnlock()

	for _, cb := range callbacks {
		// Execute each callback in a separate goroutine to prevent blocking
		go cb(actor)
	}
}
