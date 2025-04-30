package tests

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

// TestP2PDistribution tests the discovery of nodes in the network and attempts to distribute database entries across the
// network.
func TestP2PDistribution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Base port for this test from which all nodes will be spawned.
	// This is to ensure other tests can start in parallel without conflicts
	basePort := 50850

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

	// Verify the cluster is up and running
	testMessage := []byte("test message payload")
	resp, err := nodes.GetBootstrapNode().SendAndReceiveMessage(
		nodes.GetBootstrapNode(),
		types.TCPTransportType,
		types.WriteHandlerType,
		testMessage,
		5*time.Second,
	)
	require.NoError(t, err, "Failed to send message or receive response")
	require.NotNil(t, resp, "Response should not be nil")
	require.Equal(t, 1, len(resp), "Response should be exactly 1 byte")
	require.Equal(t, byte(0x00), resp[0], "Response should be 0x00 (success)")

	// Get all nodes we'll use for testing
	bootstrapNode := nodes.GetBootstrapNode()
	regularNode := nodes.GetNodeByIndex(1)
	require.NotNil(t, regularNode, "Failed to get second node")

	// Define table-driven test cases
	testCases := []struct {
		name     string
		value    []byte
		waitTime time.Duration // Time to wait for P2P distribution
	}{
		{
			name:     "Basic string value",
			value:    []byte("test record value payload"),
			waitTime: 100 * time.Millisecond,
		},
		{
			name:     "JSON data",
			value:    []byte(`{"id":"12345","name":"test","data":[1,2,3,4,5]}`),
			waitTime: 100 * time.Millisecond,
		},
		// This is broken, 4096 bytes is currently processing fine, need to get it working by buffering...
		// {
		// 	name:     "Large binary data (100KB)",
		// 	value:    bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 25 * 1024), // ~100KB of data
		// 	waitTime: 3 * time.Second, // Extra time for larger payload
		// },
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate a unique key for this test case
			key, err := messages.GenerateRandomKey()
			require.NoError(t, err, "Failed to generate random key")
			t.Logf("Using test key (hex): %s", hex.EncodeToString(key[:]))

			// 1. WRITE PHASE: Write record to bootstrap node
			t.Log("Writing record to bootstrap node database")
			writeMsg := &messages.Message{
				Handler: types.WriteHandlerType,
				Key:     key,
				Data:    tc.value,
			}

			encodedWriteMsg, err := writeMsg.Encode()
			require.NoError(t, err, "Failed to encode write message")

			writeResp, err := bootstrapNode.SendAndReceiveMessage(
				bootstrapNode,
				types.TCPTransportType,
				types.WriteHandlerType,
				encodedWriteMsg,
				5*time.Second,
			)
			require.NoError(t, err, "Failed to write record to bootstrap node")
			require.Equal(t, byte(0x00), writeResp[0], "Write operation should succeed with 0x00")

			// Wait for P2P distribution to occur
			// Batch writter flushes immediately if there's > bufferSize messages or every 100ms it flushes
			// so let's await for a little bit of the time (100ms is enough)
			t.Logf("Waiting %v for record distribution...", tc.waitTime)
			time.Sleep(tc.waitTime)

			// 2. READ PHASE: Create read message for verification
			readMsg := &messages.Message{
				Handler: types.ReadHandlerType,
				Key:     key,
				Data:    nil,
			}
			encodedReadMsg, err := readMsg.Encode()
			require.NoError(t, err, "Failed to encode read message")

			// 2a. Verify on bootstrap node (should be immediate)
			t.Log("Verifying record exists on bootstrap node")
			bootstrapResp, err := bootstrapNode.SendAndReceiveMessage(
				bootstrapNode,
				types.TCPTransportType,
				types.ReadHandlerType,
				encodedReadMsg,
				5*time.Second,
			)
			require.NoError(t, err, "Failed to read record from bootstrap node")
			require.NotNil(t, bootstrapResp, "Bootstrap node response should not be nil")
			require.True(t, len(bootstrapResp) > 1, "Bootstrap node response should have at least a status byte and value")
			require.Equal(t, byte(0x00), bootstrapResp[0], "Read response status byte should be 0x00")

			// Extract the actual payload (skip the status byte)
			bootstrapValue := bootstrapResp[1:]
			require.Equal(t, tc.value, bootstrapValue, "Record value mismatch on bootstrap node")

			// 2b. Verify on regular node (should get there via P2P distribution)
			t.Log("Verifying record exists on regular node")
			regularResp, err := regularNode.SendAndReceiveMessage(
				regularNode,
				types.TCPTransportType,
				types.ReadHandlerType,
				encodedReadMsg,
				5*time.Second,
			)
			require.NoError(t, err, "Failed to read record from regular node")
			require.NotNil(t, regularResp, "Regular node response should not be nil")
			require.True(t, len(regularResp) > 1, "Regular node response should have at least a status byte and value")
			require.Equal(t, byte(0x00), regularResp[0], "Read response status byte should be 0x00")

			// Extract the actual payload (skip the status byte)
			regularValue := regularResp[1:]
			require.Equal(t, tc.value, regularValue, "Record value mismatch on regular node")

			t.Logf("%s: P2P distribution successful!", tc.name)
		})
	}

}
