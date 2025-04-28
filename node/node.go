package node

import (
	"context"
	"github.com/unpackdev/fdb/accounts"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/metrics"
	"github.com/unpackdev/fdb/networking"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/share"
	"github.com/unpackdev/fdb/state"
	"github.com/unpackdev/fdb/topology"

	"github.com/pkg/errors"
	"time"

	"go.uber.org/zap"
)

// Node encapsulates all components of a PeerDNS node.
type Node struct {
	logger    logger.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	cfg       config.Config
	obs       *observability.Observability
	rbacMgr   *rbac.Manager
	store     *accounts.Store
	account   *accounts.Account
	network   *networking.Network
	collector *metrics.Collector
	pm        *metrics.PerformanceMonitor
	stateMgr  *state.StateManager
	actors    *share.ActorSet
	topology  *topology.Topology
}

// NewNode initializes and returns a new Node.
func NewNode(ctx context.Context, config config.Config, rbacMgr *rbac.Manager, logger logger.Logger, store *accounts.Store, obs *observability.Observability, stateMgr *state.StateManager) (*Node, error) {
	// Create a child context for the node
	nodeCtx, cancel := context.WithCancel(ctx)

	// Set initial state as Uninitialized
	stateMgr.SetState(NodeStateType, state.Uninitialized)

	// Ensure the PeerID is set in the configuration
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

	// Initialize sharding
	//shardManager := sharding.NewShardManager(config.Sharding.ShardCount, logger)

	// Initialize metrics collector with default weights
	// This thing here is used to decide (including with staking...) the leader of the consensus
	// whenever it is sequencer or validator.
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

	// Set the state to Initialized after successful node creation.
	stateMgr.SetState(NodeStateType, state.Initialized)

	return &Node{
		store:     store,
		account:   account,
		cfg:       config,
		obs:       obs,
		rbacMgr:   rbacMgr,
		network:   ntwrk,
		collector: collector,
		pm:        performanceMonitor,
		logger:    logger,
		ctx:       nodeCtx,
		cancel:    cancel,
		stateMgr:  stateMgr,
		actors:    actors,
		topology:  topologyManager,
	}, nil
}

func (n *Node) Store() *accounts.Store {
	return n.store
}

func (n *Node) Account() *accounts.Account {
	return n.account
}

func (n *Node) StateManager() *state.StateManager {
	return n.stateMgr
}

func (n *Node) Discovery() *networking.DiscoveryService {
	return n.network.Discovery()
}

func (n *Node) Collector() *metrics.Collector {
	return n.collector
}

func (n *Node) PerformanceMonitor() *metrics.PerformanceMonitor {
	return n.pm
}

func (n *Node) Network() *networking.Network {
	return n.network
}

func (n *Node) ActorSet() *share.ActorSet {
	return n.actors
}

func (n *Node) Topology() *topology.Topology {
	return n.topology
}

func (n *Node) Observability() *observability.Observability {
	return n.obs
}

func (n *Node) RbacManager() *rbac.Manager {
	return n.rbacMgr
}

// Start begins the node's operations.
func (n *Node) Start() error {
	n.logger.Info("Starting node")
	n.stateMgr.SetState(NodeStateType, state.Starting)

	if err := n.network.Start(); err != nil {
		return errors.Wrap(err, "failure to start P2P network")
	}

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

	n.logger.Info("Node shutdown complete")

	// Set the state to Stopped after shutting down.
	n.stateMgr.SetState(NodeStateType, state.Stopped)
	return nil
}
