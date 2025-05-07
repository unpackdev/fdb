package node

import (
	"context"
	"github.com/unpackdev/fdb/pkg/types"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/accounts"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/db"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/metrics"
	"github.com/unpackdev/fdb/pkg/networking"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/share"
	"github.com/unpackdev/fdb/pkg/state"
	"github.com/unpackdev/fdb/pkg/topology"

	"time"

	"github.com/pkg/errors"

	"go.uber.org/zap"
)

// Node encapsulates all components of a PeerDNS node.
type Node struct {
	logger      logger.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	cfg         config.Config
	obs         *observability.Observability
	rbacMgr     *rbac.Manager
	store       *accounts.Store
	account     *accounts.Account
	network     *networking.Network
	collector   *metrics.Collector
	pm          *metrics.PerformanceMonitor
	stateMgr    *state.StateManager
	actors      *share.ActorSet
	topology    *topology.Topology
	dbM         *db.Manager
	batchWriter *db.BatchWriter
	distributor *P2PDistributor // P2P record distribution component
}

// NewNode initializes and returns a new Node.
func NewNode(
	ctx context.Context, config config.Config, rbacMgr *rbac.Manager, logger logger.Logger,
	store *accounts.Store, obs *observability.Observability, stateMgr *state.StateManager,
	dbM *db.Manager, batchWriter *db.BatchWriter,
) (*Node, error) {
	nodeCtx, cancel := context.WithCancel(ctx)

	stateMgr.SetState(NodeStateType, state.Uninitialized)

	if config.Networking.PeerID == "" {
		logger.Error("PeerID not provided in the networking configuration")
		cancel()
		return nil, errors.New("peerId must be defined in the networking configuration")
	}

	// Attempt to load the identity from the manager using the PeerID
	account, err := store.GetByPeerID(config.Networking.PeerID)
	if err != nil {
		logger.Error("Failed to load account from identity manager", zap.String("peer_id", config.Networking.PeerID.String()), zap.Error(err))
		cancel()
		return nil, errors.Wrapf(err, "failed to load account from identity manager with PeerID: %s", config.Networking.PeerID)
	}

	logger.Debug("Loaded node identity", zap.String("PeerID", account.PeerID().String()))

	// Initialize the Consensus-Related ActorSet
	actors := share.NewActorSet(logger)

	//var cAccount *accounts.Account
	//var caErr error

	// Discover current node account keys...
	// At this moment RSV is first, RV second, RS third...
	// You need to tweak the node roles configuration if you wish to change the role.
	// This configuration is only for the sequencers and validators. Main master account is defined above
	// in this function in the beginning.
	//if rbac.HasRole(config.Node.Roles, rbac.RoleSequencerValidator) {
	//	cAccount, caErr = store.GetByRole(rbac.RoleSequencerValidator)
	//	if caErr != nil {
	//		cancel()
	//		return nil, errors.Wrap(caErr, "failed to load sequencer+validator account for node - did you create node key?")
	//	}
	//} else if rbac.HasRole(config.Node.Roles, rbac.RoleValidator) {
	//	cAccount, caErr = store.GetByRole(rbac.RoleValidator)
	//	if caErr != nil {
	//		cancel()
	//		return nil, errors.Wrap(caErr, "failed to load validator account for node - did you create node key?")
	//	}
	//} else if rbac.HasRole(config.Node.Roles, rbac.RoleSequencer) {
	//	cAccount, caErr = store.GetByRole(rbac.RoleSequencer)
	//	if caErr != nil {
	//		cancel()
	//		return nil, errors.Wrap(caErr, "failed to load sequencer account for node - did you create node key?")
	//	}
	//}

	// Initialize the DiscoveryService
	bootstrapAddrs, err := config.Networking.BootstrapPeersAsAddrs()
	if err != nil {
		logger.Error("Failed to parse bootstrap peers", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize networking
	ntwrk, err := networking.NewNetwork(nodeCtx, config.Networking, account, bootstrapAddrs, logger, obs, stateMgr)
	if err != nil {
		logger.Error("Failed to initialize P2P network", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize metrics collector with default weights
	// @TODO: These weights severely needs to be researched later on
	weights := share.Metrics{
		BandwidthUsage: 0.0,
		Computational:  0.0,
		Storage:        0.0,
		Uptime:         1.0,
		Responsiveness: 0.0,
		Reliability:    0.0,
	}

	emaAlpha := 0.2      // Smoothing factor for EMA (0 < emaAlpha <= 1)
	maxRespScore := 10.0 // Maximum cap for Responsiveness
	collector := metrics.NewCollector(nodeCtx, logger, weights, emaAlpha, maxRespScore)

	// Initialize PerformanceMonitor (do not start it yet)
	performanceMonitor := metrics.NewPerformanceMonitor(
		nodeCtx,
		ntwrk.Host(),
		logger,
		obs,
		collector,
		1*time.Second,
		100,
		1*time.Second,
		10*time.Second,
		10*time.Second,
		1*time.Second,
	)

	// At this moment we have all blockers satisfied to create a network topology management system
	topologyManager, tErr := topology.NewTopology(ctx, logger, account, ntwrk, rbacMgr, collector, actors)
	if tErr != nil {
		cancel()
		return nil, errors.Wrap(tErr, "failure to create new topology management system")
	}

	// Create a node instance with all components
	node := &Node{
		logger:      logger,
		ctx:         nodeCtx,
		cancel:      cancel,
		cfg:         config,
		obs:         obs,
		rbacMgr:     rbacMgr,
		store:       store,
		account:     account,
		network:     ntwrk,
		collector:   collector,
		pm:          performanceMonitor,
		stateMgr:    stateMgr,
		actors:      actors,
		topology:    topologyManager,
		dbM:         dbM,
		batchWriter: batchWriter,
	}

	// Initialize the P2P distributor with a reasonable batch size
	node.distributor = NewP2PDistributor(node, 2048)

	// Set the state to Initialized after successful node creation.
	stateMgr.SetState(NodeStateType, state.Initialized)

	return node, nil
}

// Store returns the node's account store.
func (n *Node) Store() *accounts.Store {
	return n.store
}

// Account returns the node's account.
func (n *Node) Account() *accounts.Account {
	return n.account
}

// StateManager returns the node's state manager.
func (n *Node) StateManager() *state.StateManager {
	return n.stateMgr
}

// Discovery returns the node's discovery service.
func (n *Node) Discovery() *networking.DiscoveryService {
	return n.network.Discovery()
}

// Collector returns the node's metrics collector.
func (n *Node) Collector() *metrics.Collector {
	return n.collector
}

// PerformanceMonitor returns the node's performance monitor.
func (n *Node) PerformanceMonitor() *metrics.PerformanceMonitor {
	return n.pm
}

// Network returns the node's network.
func (n *Node) Network() *networking.Network {
	return n.network
}

// ActorSet returns the node's actor set.
func (n *Node) ActorSet() *share.ActorSet {
	return n.actors
}

// Topology returns the node's topology.
func (n *Node) Topology() *topology.Topology {
	return n.topology
}

// Observability returns the node's observability.
func (n *Node) Observability() *observability.Observability {
	return n.obs
}

// RbacManager returns the node's RBAC manager.
func (n *Node) RbacManager() *rbac.Manager {
	return n.rbacMgr
}

// Distributor returns the node's P2P distributor component
func (n *Node) Distributor() *P2PDistributor {
	return n.distributor
}

// DistributeRecord adds a record to be distributed across the P2P network
func (n *Node) DistributeRecord(key [32]byte, value []byte) error {
	return n.distributor.DistributeRecord(key, value, types.PriorityNormal, types.TargetAll)
}

// DistributeRecordWithPriority adds a record with specified priority and target
func (n *Node) DistributeRecordWithPriority(key [32]byte, value []byte, priority types.Priority, target types.Target) error {
	return n.distributor.DistributeRecord(key, value, priority, target)
}

// DistributeRecordToPeer sends a record directly to a specific peer
func (n *Node) DistributeRecordToPeer(key [32]byte, value []byte, peerID peer.ID) error {
	return n.distributor.DistributeRecordToPeer(key, value, peerID, types.PriorityNormal)
}

func (n *Node) Start() error {
	n.logger.Info("Starting node")
	n.stateMgr.SetState(NodeStateType, state.Starting)

	if err := n.network.Start(); err != nil {
		n.stateMgr.SetState(NodeStateType, state.Failed)
		return errors.Wrap(err, "failed to start network")
	}

	// Start the P2P distributor
	n.distributor.Start()

	// Start a goroutine to continuously advertise the node until its advertised
	// Genesis node when started won't have any peers to connect.
	// Sequencers and validators do not need this...
	// Regular nodes do need this process.
	// For the time being, leaving this advertisement for all the nodes.
	// TODO
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := n.network.Discovery().Advertise("node", n.cfg.Networking.BootstrapNode)
				if err != nil {
					n.logger.Warn("Failed to advertise node service", zap.Error(err))
				} else {
					n.logger.Info("Successfully advertised node service")
					return
				}
			case <-n.ctx.Done():
				return
			}
		}
	}()

	// Start the performance monitor
	n.pm.Start()

	// Set the state to Started after successfully starting all components.
	n.stateMgr.SetState(NodeStateType, state.Started)
	return nil
}

// Shutdown gracefully shuts down the node.
func (n *Node) Shutdown() error {
	n.stateMgr.SetState(NodeStateType, state.Stopping)

	// Shut down txpool as fast as possible. No more transaction should be accepted...
	// RPC as well should be notified of this to start rejecting any in-flight transactions...
	//n.txpool.Shutdown()

	// Now we're going to send system-wide signal to shut down any remaining operations.
	// Cancelling sequencer at the top like this would potentially result in very bad situation of
	// block production being screwed up including actual database screwups.
	// As well we want txpool to be shutdown immediately and not through the context to ensure nothing
	// is coming in any more as fast as possible.
	n.cancel()

	// Stop the PerformanceMonitor
	if n.pm != nil {
		n.pm.Stop()
	}

	if err := n.network.Shutdown(); err != nil && !errors.Is(err, context.Canceled) {
		n.stateMgr.SetState(NodeStateType, state.Failed)
		return err
	}

	// Stop the P2P distributor
	if n.distributor != nil {
		n.distributor.Stop()
	}

	if err := n.stateMgr.WaitForState(P2PDistributorStateType, state.Stopped, 10*time.Second); err != nil {
		n.logger.Error("Failed to stop P2P distributor", zap.Error(err))
		n.stateMgr.SetState(NodeStateType, state.Failed)
		return err
	}

	n.logger.Info("Node shutdown complete")

	// Set the state to Stopped after shutting down.
	n.stateMgr.SetState(NodeStateType, state.Stopped)
	return nil
}
