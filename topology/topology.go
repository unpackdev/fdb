// pkg/topology/topology.go
package topology

import (
	"context"

	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/metrics"
	"github.com/unpackdev/fdb/networking"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/share"
	"github.com/unpackdev/fdb/types"

	"time"

	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"

	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Topology orchestrates the overall network topology and peer management.
type Topology struct {
	ctx           context.Context
	cancel        context.CancelFunc
	network       *networking.Network
	account       share.Account
	actors        *Actors
	metrics       *metrics.Collector
	logger        logger.Logger
	rbac          *rbac.Manager
	actorSet      *share.ActorSet
	peerEventChan chan struct{} // Channel to broadcast peer events
}

// NewTopology creates a new Topology instance with the given logger and network reference.
func NewTopology(ctx context.Context, logger logger.Logger, account share.Account, network *networking.Network, rbacMgr *rbac.Manager, collector *metrics.Collector, actorSet *share.ActorSet) (*Topology, error) {
	childCtx, cancel := context.WithCancel(ctx)

	// Initialize Topology's peerEventChan
	peerEventChan := make(chan struct{}, 100) // Buffered to prevent blocking

	// Initialize Actors with a notifier function to signal peer events
	actors, aErr := NewActors(logger, account, network, rbacMgr, collector, actorSet, func() {
		select {
		case peerEventChan <- struct{}{}:
		default:
			// Do not block if the channel is already full
		}
	})

	if aErr != nil {
		cancel()
		return nil, aErr
	}

	t := &Topology{
		ctx:           childCtx,
		cancel:        cancel,
		network:       network,
		account:       account,
		actors:        actors,
		metrics:       collector,
		logger:        logger,
		rbac:          rbacMgr,
		actorSet:      actorSet,
		peerEventChan: peerEventChan,
	}
	
	// Initialize and integrate topology metrics
	err := integrateTopologyMetrics(t, collector)
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to initialize topology metrics")
	}

	// Register packet handlers via handlerRegistry
	if err := t.RegisterHandlers(); err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to register topology handlers")
	}

	return t, nil
}

// Actors return the Actors instance managed by Topology.
func (t *Topology) Actors() *Actors {
	return t.actors
}

// ActorSet return the share.ActorSet instance managed by Topology.
// share.ActorSet is in charge of consensus-related-actors
// TODO: Rename to ConsensusSet
func (t *Topology) ActorSet() *share.ActorSet {
	return t.actorSet
}

// RegisterHandlers registers the Topology's packet handlers with the networking layer.
func (t *Topology) RegisterHandlers() error {
	// Register packet handler for ActorPacketType
	t.network.HandlerRegistry().RegisterHandler(packets.ActorPacketType, t.HandleActorPacket)

	return nil
}

// HandleActorPacket handles incoming ActorPacket based on its Status.
func (t *Topology) HandleActorPacket(ctx context.Context, msg *packets.NetworkPacket, sender peer.ID) error {
	t.logger.Info("Received ActorPacket", "peer_id", sender.String(), "packet_type", msg.Type)

	// Deserialize ActorPacket
	ap, err := packets.DeserializeActorPacket(msg.Payload)
	if err != nil {
		t.logger.Error("Failed to deserialize ActorPacket", "peer_id", sender.String(), "error", err.Error())
		return errors.Wrapf(err, "failed to deserialize ActorPacket from peer %s", sender.String())
	}

	// Unmarshal and set the PublicKey
	publicKey, err := libp2pCrypto.UnmarshalPublicKey(ap.PublicKey)
	if err != nil {
		t.logger.Error("Failed to unmarshal PublicKey", "peer_id", sender.String(), "error", err.Error())
		return errors.Wrapf(ErrInvalidMessageType, "invalid public key for peer %s: %w", sender.String(), err)
	}

	// Serialize the message without signature for verification
	serialized, err := msg.SerializeWithoutSignature()
	if err != nil {
		t.logger.Error("Failed to serialize message without signature", "peer_id", sender.String(), "error", err.Error())
		return errors.Wrapf(err, "failed to serialize message without signature for peer %s", sender.String())
	}

	// Verify the signature
	verified, err := publicKey.Verify(serialized, msg.Signature)
	if err != nil {
		t.logger.Error("Error during signature verification", "peer_id", sender.String(), "error", err.Error())
		return errors.Wrapf(ErrInvalidSignature, "error verifying signature for peer %s: %w", sender.String(), err)
	}

	if !verified {
		t.logger.Error("Invalid signature detected", "peer_id", sender.String())
		return errors.Wrapf(ErrInvalidSignature, "invalid signature for peer %s", sender.String())
	}

	// RBAC Check: Verify if self has PermissionProcessActorPacket
	if !t.rbac.HasPermission(t.actors.self.Roles, rbac.PermissionProcessActorPacket) {
		t.logger.Warn("Insufficient permissions to process actor packet", "required_permission", rbac.PermissionProcessActorPacket)
		return ErrInsufficientPermissions
	}

	// Process the ActorPacket via the Actors, passing the context
	//if err := t.actors.ProcessActorPacket(ctx, sender, ap); err != nil {
	//	t.logger.Error("Failed to process ActorPacket", "peer_id", sender.String(), "error", err.Error())
	//	return errors.Wrapf(err, "failed to process ActorPacket from peer %s", sender.String())
	//}

	t.logger.Info("Processed ActorPacket", "peer_id", sender.String(), "status", ap.Status)
	return nil
}

// WaitForPeer waits until the specified peer is available in the topology or the context times out.
func (t *Topology) WaitForPeer(ctx context.Context, targetPeerID peer.ID, timeout time.Duration) error {
	// Start timing the wait operation
	startTime := time.Now()
	
	// Record the wait operation start
	if t.actors != nil && t.actors.topologyMetrics != nil {
		t.actors.topologyMetrics.RecordWaitOperation(ctx, 1, "peer")
	}
	
	// Create a context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok && timeout > 0 { // Changed 'deadline' to '_'
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		// Check if the peer is available
		_, err := t.Actors().GetPeer(targetPeerID)
		if err == nil {
			// Peer found, record success metrics
			if t.actors != nil && t.actors.topologyMetrics != nil {
				t.actors.topologyMetrics.RecordWaitOperationSuccess(ctx, 1, "peer")
				t.actors.topologyMetrics.RecordWaitOperationLatency(ctx, time.Since(startTime), "peer")
			}
			return nil
		} else if err != nil && !errors.Is(err, ErrPeerNotFound) && !errors.Is(err, ErrInsufficientPermissions) {
			// An unexpected error occurred
			return err
		}

		// Wait for a peer event or context cancellation
		select {
		case <-t.peerEventChan:
			// Peer list has changed; recheck
		case <-ctx.Done():
			// Record timeout or cancellation metrics
			if t.actors != nil && t.actors.topologyMetrics != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.actors.topologyMetrics.RecordWaitOperationTimeout(ctx, 1, "peer")
				}
				t.actors.topologyMetrics.RecordWaitOperationLatency(ctx, time.Since(startTime), "peer")
			}
			
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ErrTimeoutExceeded
			}
			return ctx.Err()
		}
	}
}

// WaitForPeers waits until at least one peer is available in the topology or the context times out.
func (t *Topology) WaitForPeers(ctx context.Context, timeout time.Duration) error {
	// Start timing the wait operation
	startTime := time.Now()
	
	// Record the wait operation start
	if t.actors != nil && t.actors.topologyMetrics != nil {
		t.actors.topologyMetrics.RecordWaitOperation(ctx, 1, "any_peer")
	}
	
	// Create a context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok && timeout > 0 { // Changed 'deadline' to '_'
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		// Check if any peers are available
		peers, err := t.Actors().GetPeers()
		if err == nil && len(peers) > 0 {
			// Peers found, record success metrics
			if t.actors != nil && t.actors.topologyMetrics != nil {
				t.actors.topologyMetrics.RecordWaitOperationSuccess(ctx, 1, "any_peer")
				t.actors.topologyMetrics.RecordWaitOperationLatency(ctx, time.Since(startTime), "any_peer")
			}
			return nil // At least one peer is available
		} else if err != nil && !errors.Is(err, ErrInsufficientPermissions) {
			// An unexpected error occurred
			return err
		}

		// Wait for a peer event or context cancellation
		select {
		case <-t.peerEventChan:
			// Peer list has changed; recheck
		case <-ctx.Done():
			// Record timeout or cancellation metrics
			if t.actors != nil && t.actors.topologyMetrics != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.actors.topologyMetrics.RecordWaitOperationTimeout(ctx, 1, "any_peer")
				}
				t.actors.topologyMetrics.RecordWaitOperationLatency(ctx, time.Since(startTime), "any_peer")
			}
			
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ErrTimeoutExceeded
			}
			return ctx.Err()
		}
	}
}

// WaitForPeersWithRole waits until at least one peer with the specified role is available or the context times out.
func (t *Topology) WaitForPeersWithRole(ctx context.Context, role types.Role, timeout time.Duration) error {
	// Create a context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok && timeout > 0 { // Changed 'deadline' to '_'
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		// Check if any peers with the specified role are available
		peers, err := t.Actors().GetPeersByRole(role)
		if err == nil && len(peers) > 0 {
			return nil // At least one peer with the role is available
		} else if err != nil && !errors.Is(err, ErrInsufficientPermissions) {
			// An unexpected error occurred
			return err
		}

		// Wait for a peer event or context cancellation
		select {
		case <-t.peerEventChan:
			// Peer list has changed; recheck
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ErrTimeoutExceeded
			}
			return ctx.Err()
		}
	}
}

// WaitForPeersWithRoles waits until at least one peer with any of the specified roles is available or the context times out.
func (t *Topology) WaitForPeersWithRoles(ctx context.Context, roles []types.Role, timeout time.Duration) error {
	// Start timing the wait operation
	startTime := time.Now()
	
	// Record the wait operation start
	if t.actors != nil && t.actors.topologyMetrics != nil {
		t.actors.topologyMetrics.RecordWaitOperation(ctx, 1, "peers_with_roles")
	}
	
	// Create a context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok && timeout > 0 { // Changed 'deadline' to '_'
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		// Check if any peers with the specified roles are available
		peers, err := t.Actors().GetPeersByRoles(roles...)
		if err == nil && len(peers) > 0 {
			// Peers with roles found, record success metrics
			if t.actors != nil && t.actors.topologyMetrics != nil {
				t.actors.topologyMetrics.RecordWaitOperationSuccess(ctx, 1, "peers_with_roles")
				t.actors.topologyMetrics.RecordWaitOperationLatency(ctx, time.Since(startTime), "peers_with_roles")
			}
			return nil // At least one peer with any of the roles is available
		} else if err != nil && !errors.Is(err, ErrInsufficientPermissions) {
			// An unexpected error occurred
			return err
		}

		// Wait for a peer event or context cancellation
		select {
		case <-t.peerEventChan:
			// Peer list has changed; recheck
		case <-ctx.Done():
			// Record timeout or cancellation metrics
			if t.actors != nil && t.actors.topologyMetrics != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.actors.topologyMetrics.RecordWaitOperationTimeout(ctx, 1, "peers_with_roles")
				}
				t.actors.topologyMetrics.RecordWaitOperationLatency(ctx, time.Since(startTime), "peers_with_roles")
			}
			
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return ErrTimeoutExceeded
			}
			return ctx.Err()
		}
	}
}

// Shutdown gracefully shuts down the topology.
func (t *Topology) Shutdown() {
	t.cancel()
	t.logger.Info("Topology shut down successfully")
}
