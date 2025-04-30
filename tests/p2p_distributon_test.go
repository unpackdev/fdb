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

	// Send a test message and wait for response
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

	// The write handler responds with a 1-byte response: 0x00 for success or 0x01 for error
	// See transports/db_write_handler.go for details
	require.Equal(t, 1, len(resp), "Response should be exactly 1 byte")
	require.Equal(t, byte(0x00), resp[0], "Response should be 0x00 (success)")

	// Test P2P distributor
	t.Run("P2P Distribution", func(t *testing.T) {
		// Create a test record with a fixed key for consistency
		// Generate a random 32-byte key
		key, err := messages.GenerateRandomKey()
		require.NoError(t, err, "Failed to generate random key")

		value := []byte("test record value payload")

		t.Logf("Using test key (hex): %s", hex.EncodeToString(key[:]))

		// Get the bootstrap node and a regular node
		bootstrapNode := nodes.GetBootstrapNode()
		regularNode := nodes.GetNodeByIndex(1) // Get the second node
		require.NotNil(t, regularNode, "Failed to get the second node")

		// Create a message with the fixed key
		t.Log("Writing record to bootstrap node database")

		// Create a message with our specific key (not random)
		writeMsg := &messages.Message{
			Handler: types.WriteHandlerType,
			Key:     key,
			Data:    value,
		}

		// Encode the message
		encodedWriteMsg, err := writeMsg.Encode()
		require.NoError(t, err, "Failed to encode write message")

		// Write the record to the bootstrap node
		writeResp, err := bootstrapNode.SendAndReceiveMessage(
			bootstrapNode,
			types.TCPTransportType,
			types.WriteHandlerType,
			encodedWriteMsg,
			5*time.Second,
		)
		require.NoError(t, err, "Failed to write record to bootstrap node")
		require.Equal(t, byte(0x00), writeResp[0], "Write operation should succeed with 0x00")

		// The DbWriteHandler automatically distributes records after writing
		// We're explicitly calling a BatchWriter.Flush() in the handler

		// Give some time for distribution to occur
		t.Log("Waiting for record distribution...")
		time.Sleep(1 * time.Second)

		// Verify on bootstrap node first (should be immediate)
		t.Log("Verifying record exists on bootstrap node")

		// Create a read message with the SAME key
		t.Logf("Reading with the exact same key (hex): %s", hex.EncodeToString(key[:]))

		// Create a read message with our specific key (not random)
		readMsg := &messages.Message{
			Handler: types.ReadHandlerType,
			Key:     key, // Same key we used for writing
			Data:    nil, // No data needed for read requests
		}

		// Encode the message
		encodedReadMsg, err := readMsg.Encode()
		require.NoError(t, err, "Failed to encode read message")

		// Read from the bootstrap node
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

		// Response format is now: [1 byte status][value bytes]
		// For successful reads, status byte should be 0x00
		require.Equal(t, byte(0x00), bootstrapResp[0], "Read response status byte should be 0x00")

		// Extract the actual payload (skip the status byte)
		bootstrapValue := bootstrapResp[1:]
		require.Equal(t, value, bootstrapValue, "Record value mismatch on bootstrap node")

		// Now verify on the regular node (should get there via P2P distribution)
		t.Log("Verifying record exists on regular node")

		// Read from the regular node using the same message
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

		// Response format is now: [1 byte status][value bytes]
		// For successful reads, status byte should be 0x00
		require.Equal(t, byte(0x00), regularResp[0], "Read response status byte should be 0x00")

		// Extract the actual payload (skip the status byte)
		regularValue := regularResp[1:]
		require.Equal(t, value, regularValue, "Record value mismatch on regular node")

		t.Log("P2P distribution test successful!")
	})
}
