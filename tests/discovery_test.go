package tests

import (
	"context"
	"testing"
	"time"

	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"

	"github.com/stretchr/testify/require"
)

// TestNodeDiscovery tests the discovery of nodes in the network. If successfully mutually connected to each other, the
// test is considered as successful.
func TestNodeDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Base port for this test from which all nodes will be spawned.
	// This is to ensure other tests can start in parallel without conflicts
	basePort := 50750

	// Define the node roles.
	// This practice is used to define sequential nodes that are going to be initialized for
	// this test to successfully complete its mission.
	// There is 1 type of node for now:
	// 	RoleNode: Starts node only (a full node)
	roles := []types.Role{
		rbac.RoleNode,
		rbac.RoleNode,
	}

	// Initialize test nodes based on the defined roles.
	nodes, err := InitializeTestNodes(t, ctx, zap.DebugLevel, types.Ed25519SignerType, roles, basePort, types.TCPTransportType)
	require.NoError(t, err, "Failed to initialize test nodes")

	// Ensure all nodes are shut down and directories are cleaned up after the test
	defer func() {
		err := ShutdownTestNodes(t, nodes)
		require.NoError(t, err, "Failed to shutdown test nodes")
	}()

	for _, tNode := range nodes {
		// Minus one because own peer needs to be excluded
		wpcErr := tNode.WaitForPeersConnected(len(roles)-1, 10*time.Second)
		require.NoError(t, wpcErr, "failure to establish mutual node connectivity")
	}
}
