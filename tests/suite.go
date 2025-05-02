// tests/suite.go

package tests

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/accounts"
	"github.com/unpackdev/fdb/client"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/node"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/protocols/rpc"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/share"
	"github.com/unpackdev/fdb/state"
	"github.com/unpackdev/fdb/transports"
	"github.com/unpackdev/fdb/transports/tcp"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
func InitializeTestNodes(
	t *testing.T,
	ctx context.Context,
	logLevel zapcore.Level,
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
		rpcPort := tcp.GetFreePort(t)
		tcpPort := tcp.GetFreePort(t)

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
			Logger: config.Logger{
				Enabled:     true,
				Environment: "development",
				Level:       logLevel.String(),
			},
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
				// {
				// 	Type:    types.DummyTransportType,
				// 	Enabled: true,
				// 	Config: &config.DummyTransport{
				// 		Type:    types.DummyTransportType,
				// 		Enabled: true,
				// 		IPv4:    "127.0.0.1",
				// 		Port:    4434,
				// 	},
				// },
				// {
				// 	Type:    types.QUICTransportType,
				// 	Enabled: true,
				// 	Config: &config.QuicTransport{
				// 		Type:    types.QUICTransportType,
				// 		Enabled: true,
				// 		IPv4:    "127.0.0.1",
				// 		Port:    4433,
				// 		TLS: config.TLS{
				// 			Insecure: true,
				// 			Key:      "../data/certs/key.pem",
				// 			Cert:     "../data/certs/cert.pem",
				// 		},
				// 	},
				// },
				// {
				// 	Type:    types.UDSTransportType,
				// 	Enabled: true,
				// 	Config: &config.UdsTransport{
				// 		Type:    types.UDSTransportType,
				// 		Enabled: true,
				// 		Socket:  "/tmp/fdb.sock",
				// 	},
				// },
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
				// {
				// 	Type:    types.UDPTransportType,
				// 	Enabled: true,
				// 	Config: &config.UdpTransport{
				// 		Type:    types.UDPTransportType,
				// 		Enabled: true,
				// 		IPv4:    "127.0.0.1",
				// 		Port:    5022,
				// 		DTLS: &config.DTLS{
				// 			Insecure: true,
				// 			Key:      "../data/certs/key.pem",
				// 			Cert:     "../data/certs/cert.pem",
				// 		},
				// 	},
				// },
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

		// Initialize the logger
		testLogger, err := logger.InitializeGlobalLogger(nodeConfig.Logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logger for node %d: %w", i, err)
		}

		// Initialize metrics and tracing system (opentelemetry & prometheus)
		obs, err := observability.Initialize(ctx, nodeConfig, testLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize observability for node %d: %w", i, err)
		}

		// Initialize the RBAC manager
		rbacMgr, rbmErr := rbac.NewManager(ctx, rbac.WithDefaultRoles())
		require.NoError(t, rbmErr)

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
		require.NoError(t, err, "Failed to initialize RPC")

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
			require.NoError(t, startErr)
		}()

		// Wait for the node to start
		stateErr := tNode.state.WaitForState(node.NodeStateType, state.Started, 15*time.Second)
		require.NoError(t, stateErr)

		// // Wait for the RPC to start
		stateErr = tNode.state.WaitForState(rpc.RpcStateType, state.Started, 5*time.Second)
		require.NoError(t, stateErr)

		// Initialize client to raw tcp socket, not actual RPC client.
		client, err := CreateClient(t, ctx, tNode.logger, tNode.config.GetTransportByType(types.TCPTransportType).Config.(*config.TcpTransport).Port)
		require.NoError(t, err)

		// Connect to the node
		connectErr := client.Start(ctx)
		require.NoError(t, connectErr)

		tNode.client = client
	}

	for _, tNode := range nodes {
		// Minus one because own peer needs to be excluded
		wpcErr := tNode.WaitForPeersConnected(len(nodeRoles)-1, 10*time.Second)
		require.NoError(t, wpcErr, "failure to establish mutual node connectivity")
	}

	return nodes, nil
}

// ShutdownTestNodes gracefully shuts down all test nodes and cleans up temporary directories.
func ShutdownTestNodes(t *testing.T, nodes []*TestNode) error {
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
			require.NoError(t, rpcStateErr)

			if err := n.node.Shutdown(); err != nil {
				errChan <- fmt.Errorf("failed to shutdown node %s: %w", n.peerID, err)
				return
			}

			nodeStateErr := n.State().WaitForState(node.NodeStateType, state.Stopped, 15*time.Second)
			require.NoError(t, nodeStateErr)

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
