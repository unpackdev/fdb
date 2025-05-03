package suite

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/client"
	"github.com/unpackdev/fdb/pkg/accounts"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/db"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/node"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/protocols/rpc"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/share"
	"github.com/unpackdev/fdb/pkg/state"
	"github.com/unpackdev/fdb/pkg/transports"
	"github.com/unpackdev/fdb/pkg/transports/tcp"
	"github.com/unpackdev/fdb/pkg/types"
	"go.uber.org/zap"
)

type TestNodes []*TestNode

func (tn TestNodes) GetBootstrapNode() *TestNode {
	return tn[0]
}

func (tn TestNodes) GetNodeByRole(role types.Role) *TestNode {
	for _, tNode := range tn {
		if tNode.IsRole(role) {
			return tNode
		}
	}
	return nil
}

func (tn TestNodes) GetNodesByRole(role types.Role) []*TestNode {
	toReturn := make([]*TestNode, 0)
	for _, tNode := range tn {
		if tNode.IsRole(role) {
			toReturn = append(toReturn, tNode)
		}
	}
	return toReturn
}

func (tn TestNodes) GetNodesByRoles(roles ...types.Role) []*TestNode {
	toReturn := make([]*TestNode, 0)
	for _, tNode := range tn {
	rolegoto:
		for _, role := range roles {
			if tNode.IsRole(role) {
				toReturn = append(toReturn, tNode)
				break rolegoto
			}
		}
	}
	return toReturn
}

func (t TestNodes) GetNodeByIndex(index int) *TestNode {
	if index < 0 || index >= len(t) {
		return nil
	}
	return t[index]
}

// TestNode represents a test node with its configuration and instance.
type TestNode struct {
	ctx     context.Context
	node    *node.Node
	logger  logger.Logger
	config  config.Config
	peerID  peer.ID
	dir     string
	state   *state.StateManager
	account share.Account
	rpc     *rpc.RPC
	role    types.Role
	dbM     *db.Manager
	fDb     *fdb.FDB
	client  *client.Client
}

func (t *TestNode) Ctx() context.Context {
	return t.ctx
}

func (t *TestNode) IsNode() bool {
	return t.role == rbac.RoleNode
}

func (t *TestNode) Node() *node.Node {
	return t.node
}

func (t *TestNode) Config() config.Config {
	return t.config
}

func (t *TestNode) Logger() logger.Logger {
	return t.logger
}

func (t *TestNode) PeerID() peer.ID {
	return t.peerID
}

func (t *TestNode) Dir() string {
	return t.dir
}

func (t *TestNode) Account() share.Account {
	return t.account
}

func (t *TestNode) IsValidator() bool {
	return t.role == rbac.RoleValidator || t.role == rbac.RoleSequencerValidator
}

func (t *TestNode) IsSequencer() bool {
	return t.role == rbac.RoleSequencer || t.role == rbac.RoleSequencerValidator
}

func (t *TestNode) Role() types.Role {
	return t.role
}

func (t *TestNode) IsRole(role types.Role) bool {
	return t.role == role
}

func (t *TestNode) State() *state.StateManager {
	return t.state
}

func (t *TestNode) RequiresDKG() bool {
	return t.IsValidator()
}

func (t *TestNode) RPC() *rpc.RPC {
	return t.rpc
}

func (t *TestNode) DBM() *db.Manager {
	return t.dbM
}

func (t *TestNode) FDB() *fdb.FDB {
	return t.fDb
}

func (t *TestNode) WaitForPeersConnected(expectedPeerCount int, timeout time.Duration) error {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for validator %s to connect to peers", t.PeerID())
		case <-ticker.C:
			peers := t.node.Network().Host().Network().Peers()
			if len(peers) >= expectedPeerCount {
				t.logger.Info(
					"Node connected to all peers expected peers",
					zap.String("peer_id", t.PeerID().String()),
					zap.Int("connected_peers", len(peers)),
					zap.Int("expected_peers", expectedPeerCount),
				)
				return nil
			}
		}
	}
}

// InitializeTestNodes initializes 'count' number of nodes for testing.
// Each node is assigned a unique port starting from 'basePort'.
// DIDs are created with persistence disabled (non-persistent keys).
func InitializeNodes(
	ctx context.Context,
	testLogger logger.Logger,
	signerType types.SignerType,
	nodeRoles []types.Role,
	basePort int,
	transportTypes ...types.TransportType,
) (TestNodes, error) {
	var nodes []*TestNode

	for i, role := range nodeRoles {
		// Create a unique temporary directory for each node
		dir, err := os.MkdirTemp("", fmt.Sprintf("peerdns_node_%d", i))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory for node %d: %w", i, err)
		}

		// Use a free port.
		rpcPort, rpErr := tcp.GetFreePort()
		if rpErr != nil {
			return nil, fmt.Errorf("failed to acquire a free port for node %d: %w", i, rpErr)
		}

		tcpPort, tpErr := tcp.GetFreePort()
		if tpErr != nil {
			return nil, fmt.Errorf("failed to acquire a free port for node %d: %w", i, tpErr)
		}

		// Configuration of mdbx databases that are used by the chain and graphmdbx packages.
		// This system utilises a DAG database approach.
		mdbxNodes := []config.MdbxNode{
			{
				Name:            "fdb",
				Path:            filepath.Join(dir, "fdb.mdbx"),
				MaxReaders:      4096,
				MaxSize:         1,    // in GB for testing purposes
				MinSize:         1,    // in MB
				GrowthStep:      4096, // 4KB for testing
				FilePermissions: 0600,
			},
		}

		// Assign a unique port for the node
		listenPort := basePort + i

		// Determine bootstrap peers
		var bootstrapPeers []string
		if i > 0 {
			// Use the first node as the bootstrap node
			bootstrapPeerID := nodes[0].peerID
			bootstrapAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", basePort, bootstrapPeerID.String())
			bootstrapPeers = append(bootstrapPeers, bootstrapAddr)
		}

		// Build the node configuration
		nodeConfig := config.Config{
			Id: fmt.Sprintf("playground_node_%d", i),
			// Logger: config.Logger{
			// 	Enabled:     true,
			// 	Environment: "development",
			// 	Level:       logLevel.String(),
			// },
			Mdbx: config.Mdbx{
				Enabled: true,
				Nodes:   mdbxNodes,
			},
			Pprof: []config.Pprof{
				{
					Name:    "fdb",
					Enabled: false,
				},
			},
			Networking: config.Networking{
				ListenAddrs:    []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
				ProtocolID:     "/peerdns/1.0.0",
				BootstrapPeers: bootstrapPeers,
				BootstrapNode:  i == 0, // First node is the bootstrap node
				EnableMDNS:     true,
				EnableRelay:    true,
			},
			Identity: config.Identity{
				Enabled:  true,
				BasePath: dir,
				Keys:     []config.Key{}, // Keys are generated dynamically...
			},
			Observability: config.Observability{
				Metrics: config.MetricsConfig{
					Enable:         false,
					Exporter:       "prometheus",
					Endpoint:       "0.0.0.0:9090",
					Headers:        map[string]string{},
					ExportInterval: 15 * time.Second,
					SampleRate:     1.0,
				},
				Tracing: config.TracingConfig{
					Enable:         false,
					Exporter:       "otlp",
					Endpoint:       "localhost:4317",
					Headers:        map[string]string{},
					Sampler:        "always_on",
					SamplingRate:   1.0,
					ExportInterval: 15 * time.Second,
				},
			},
			Transports: []config.Transport{
				{
					Type:    types.TCPTransportType,
					Enabled: true,
					Config: &config.TcpTransport{
						Type:    types.TCPTransportType,
						Enabled: true,
						IPv4:    "127.0.0.1",
						Port:    tcpPort,
						// TLS: &config.TLS{
						// 	Insecure: true,
						// 	Key:      "../data/certs/key.pem",
						// 	Cert:     "../data/certs/cert.pem",
						// },
					},
				},
			},
			Rpc: config.Rpc{
				PoolMaxSize: 5,
				Transport: config.TcpTransport{
					Enabled: true,
					IPv4:    net.ParseIP("127.0.0.1").String(),
					Port:    rpcPort,
					Type:    types.TCPTransportType,
					TLS:     nil,
				},
			},
		}

		// Initialize metrics and tracing system (opentelemetry & prometheus)
		obs, err := observability.Initialize(ctx, nodeConfig, testLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize observability for node %d: %w", i, err)
		}

		// Initialize the RBAC manager
		rbacMgr, rbmErr := rbac.NewManager(ctx, rbac.WithDefaultRoles())
		if rbmErr != nil {
			return nil, fmt.Errorf("failed to initialize RBAC manager for node %d: %w", i, rbmErr)
		}

		// Initialize the identity manager
		identityMgr, err := accounts.NewStore(nodeConfig.Identity, testLogger, rbacMgr)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize identity manager for node %d: %w", i, err)
		}

		// Create a DID with persistence disabled (non-persistent keys)
		account, err := identityMgr.Create("Node Account", fmt.Sprintf("This is account for node %d", i), signerType, false, role)
		if err != nil {
			return nil, fmt.Errorf("failed to create account for node %d: %w", i, err)
		}

		// Assign the newly created account as a peer id in the networking configuration
		// Without this system will fail as network peer id will not be known.
		nodeConfig.Networking.PeerID = account.PeerID()

		stateMgr, smErr := state.NewManager(testLogger, obs)
		if smErr != nil {
			return nil, errors.Wrapf(smErr, "failure to initialize state manager for node %d", i)
		}

		dbM, dbmErr := db.NewManager(ctx, nodeConfig.Mdbx)
		if dbmErr != nil {
			return nil, fmt.Errorf("failed to create database manager for node %d: %w", i, err)
		}

		// Create the RPC instance.
		rpcInstance, err := rpc.NewRPC(ctx, nodeConfig.Rpc, testLogger, obs, stateMgr)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RPC for node %d: %w", i, err)
		}

		tManager := transports.NewManager()

		fDb, fdbErr := fdb.NewWithArgs(ctx, nodeConfig, testLogger, obs, tManager, dbM, rbacMgr, identityMgr, stateMgr, rpcInstance)
		if fdbErr != nil {
			return nil, fmt.Errorf("failed to initialize fdb for node %d: %w", i, fdbErr)
		}

		nodes = append(nodes, &TestNode{
			ctx:     ctx,
			node:    fDb.GetNode(),
			logger:  testLogger,
			config:  nodeConfig,
			peerID:  account.PeerID(),
			dir:     dir,
			account: account,
			role:    role,
			state:   stateMgr,
			rpc:     rpcInstance,
			dbM:     dbM,
			fDb:     fDb,
		})

		// Construct the multiaddress of this node and store it for others test node to use
		nodeAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", listenPort, account.PeerID().String())

		testLogger.Info(
			"Initialized test node",
			zap.Int("index", i),
			zap.String("address", nodeAddr),
		)
	}

	for _, tNode := range nodes {
		go func() {
			startErr := tNode.fDb.Start(ctx, transportTypes...)
			if startErr != nil {
				tNode.logger.Error("failed to start fdb for node", zap.Error(startErr))
				return
			}
		}()

		// Wait for the node to start
		stateErr := tNode.state.WaitForState(node.NodeStateType, state.Started, 15*time.Second)
		if stateErr != nil {
			return nil, fmt.Errorf("failed to start node %s: %w", tNode.peerID, stateErr)
		}

		// // Wait for the RPC to start
		stateErr = tNode.state.WaitForState(rpc.RpcStateType, state.Started, 5*time.Second)
		if stateErr != nil {
			return nil, fmt.Errorf("failed to start rpc for node %s: %w", tNode.peerID, stateErr)
		}

		// Initialize client to raw tcp socket, not actual RPC client.
		client, err := CreateClient(ctx, tNode.logger, tNode.config.GetTransportByType(types.TCPTransportType).Config.(*config.TcpTransport).Port)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for node %s: %w", tNode.peerID, err)
		}

		// Connect to the node
		connectErr := client.Start(ctx)
		if connectErr != nil {
			return nil, fmt.Errorf("failed to connect to node %s: %w", tNode.peerID, connectErr)
		}

		tNode.client = client
	}

	for _, tNode := range nodes {
		// Minus one because own peer needs to be excluded
		wpcErr := tNode.WaitForPeersConnected(len(nodeRoles)-1, 10*time.Second)
		if wpcErr != nil {
			return nil, fmt.Errorf("failure to establish mutual node connectivity: %w", wpcErr)
		}
	}

	return nodes, nil
}

// ShutdownTestNodes gracefully shuts down all test nodes and cleans up temporary directories.
func ShutdownTestNodes(nodes []*TestNode) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(nodes))

	for _, nodeInst := range nodes {
		// Capture nodeInst for the goroutine
		nodeInst := nodeInst

		wg.Add(1)
		go func(n *TestNode) {
			defer wg.Done()

			// First shut down the RPC to gracefully stop any inbound traffic
			if err := n.fDb.Stop(); err != nil {
				errChan <- errors.Wrap(err, "failed to stop (f)db server")
				return
			}

			rpcStateErr := n.State().WaitForState(rpc.RpcStateType, state.Stopped, 15*time.Second)
			if rpcStateErr != nil {
				errChan <- fmt.Errorf("failed to stop rpc for node %s: %w", n.peerID, rpcStateErr)
				return
			}

			if err := n.node.Shutdown(); err != nil {
				errChan <- fmt.Errorf("failed to shutdown node %s: %w", n.peerID, err)
				return
			}

			nodeStateErr := n.State().WaitForState(node.NodeStateType, state.Stopped, 15*time.Second)
			if nodeStateErr != nil {
				errChan <- fmt.Errorf("failed to stop node %s: %w", n.peerID, nodeStateErr)
				return
			}

			// Remove temporary directory
			if err := os.RemoveAll(n.dir); err != nil {
				errChan <- fmt.Errorf("failed to remove temp directory %s: %w", n.dir, err)
				return
			}

		}(nodeInst)
	}

	wg.Wait()

	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}
