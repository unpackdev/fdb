package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/packets"
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

	// Parse the response into a structured DBResponse object
	dbResp, err := packets.DecodeDBResponse(resp)
	require.NoError(t, err, "Failed to decode response")

	// Check that the status is success
	require.Equal(t, types.HandlerStatusSuccess, dbResp.Status, "Response status should be HandlerStatusSuccess")

	// Additional checks on the response data if needed
	require.Greater(t, len(dbResp.Data), 0, "Response should have data")

	// Get all nodes we'll use for testing
	bootstrapNode := nodes.GetBootstrapNode()
	regularNode := nodes.GetNodeByIndex(1)
	require.NotNil(t, regularNode, "Failed to get second node")

	// Define table-driven test cases
	testCases := []struct {
		name     string
		sizeKB   int           // Size of test data in KB
		random   bool          // Whether to use random data
		waitTime time.Duration // Time to wait for P2P distribution
	}{
		{
			name:     "Basic string value",
			sizeKB:   1,
			random:   false,
			waitTime: 100 * time.Millisecond,
		},
		{
			name:     "JSON data",
			sizeKB:   2,
			random:   false,
			waitTime: 100 * time.Millisecond,
		},
		{
			name:     "Large binary data (100KB)",
			sizeKB:   65,
			random:   false,
			waitTime: 500 * time.Millisecond, // Extra time for larger payload
		},
		{
			name:     "Large binary data (500KB)",
			sizeKB:   500,
			random:   false,
			waitTime: 500 * time.Millisecond, // Extra time for larger payload
		},
		{
			name:     "Larger binary data (1MB)",
			sizeKB:   1024,
			random:   false,
			waitTime: 500 * time.Millisecond, // Extra time for larger payload
		},
		{
			name:     "Larger binary data (10MB)",
			sizeKB:   10 * 1024,
			random:   false,
			waitTime: 500 * time.Millisecond, // Extra time for larger payload
		},
		{
			name:     "Larger binary data (500MB)",
			sizeKB:   500 * 1024,
			random:   false,
			waitTime: 3 * time.Second, // Extra time for larger payload
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate test data with the specified size
			testData, err := GenerateTestDataKB(tc.sizeKB, tc.random)
			require.NoError(t, err, "Failed to generate test data")
			t.Logf("Generated test data of size %d KB (%d bytes)", tc.sizeKB, len(testData))

			// Generate a unique key for this test case
			key, err := messages.GenerateRandomKey()
			require.NoError(t, err, "Failed to generate random key")
			t.Logf("Using test key (hex): %s", hex.EncodeToString(key[:]))

			// 1. WRITE PHASE: Write record to bootstrap node
			t.Log("Writing record to bootstrap node database")
			writeMsg := &messages.Message{
				Handler: types.WriteHandlerType,
				Key:     key,
				Data:    testData,
			}

			encodedWriteMsg, err := writeMsg.Encode()
			require.NoError(t, err, "Failed to encode write message")

			// Send write message to bootstrap node
			writeResp, err := bootstrapNode.SendAndReceiveMessage(
				bootstrapNode,
				types.TCPTransportType,
				types.WriteHandlerType,
				encodedWriteMsg,
				5*time.Second,
			)
			require.NoError(t, err, "Failed to write record to bootstrap node")
			require.NotNil(t, writeResp, "Write response should not be nil")

			// Parse the write response into a structured DBResponse object
			writeDbResp, err := packets.DecodeDBResponse(writeResp)
			require.NoError(t, err, "Failed to decode write response")
			require.Equal(t, types.HandlerStatusSuccess, writeDbResp.Status, "Write operation should succeed with HandlerStatusSuccess")

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

			// Parse the response into a structured DBResponse object
			bootstrapDbResp, err := packets.DecodeDBResponse(bootstrapResp)
			require.NoError(t, err, "Failed to decode bootstrap node response")
			require.Equal(t, types.HandlerStatusSuccess, bootstrapDbResp.Status, "Bootstrap node response status should be HandlerStatusSuccess")

			// Extract the actual payload from the structured response
			bootstrapValue := bootstrapDbResp.Data
			require.Equal(t, testData, bootstrapValue, "Record value mismatch on bootstrap node")

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

			// Parse the response into a structured DBResponse object
			regularDbResp, err := packets.DecodeDBResponse(regularResp)
			require.NoError(t, err, "Failed to decode regular node response")
			require.Equal(t, types.HandlerStatusSuccess, regularDbResp.Status, "Regular node response status should be HandlerStatusSuccess")

			// Extract the actual payload from the structured response
			regularValue := regularDbResp.Data
			require.Equal(t, testData, regularValue, "Record value mismatch on regular node")

			t.Logf("%s: P2P distribution successful!", tc.name)
		})
	}
}

func TestP2PLoadDistribution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Base port for this test from which all nodes will be spawned.
	basePort := 51000

	// Define the node roles: 1 bootstrap node and 1 regular node
	roles := []types.Role{
		rbac.RoleNode,
		rbac.RoleNode,
	}

	// Initialize test nodes based on the defined roles
	nodes, err := InitializeTestNodes(t, ctx, zap.WarnLevel, types.Ed25519SignerType, roles, basePort, types.TCPTransportType)
	require.NoError(t, err, "Failed to initialize test nodes")

	// Ensure nodes are shut down and directories cleaned up after the test
	defer func() {
		err := ShutdownTestNodes(t, nodes)
		require.NoError(t, err, "Failed to shutdown test nodes")
	}()

	// Verify the cluster is up and running with a simple write
	testMessage := []byte("test message payload")
	resp, err := nodes.GetBootstrapNode().SendAndReceiveMessage(
		nodes.GetBootstrapNode(),
		types.TCPTransportType,
		types.WriteHandlerType,
		testMessage,
		5*time.Second,
	)
	require.NoError(t, err, "Failed to send message or receive response")

	// Parse the response into a structured DBResponse object
	dbResp, err := packets.DecodeDBResponse(resp)
	require.NoError(t, err, "Failed to decode response")

	// Check that the status is success
	require.Equal(t, types.HandlerStatusSuccess, dbResp.Status, "Response status should be HandlerStatusSuccess")

	// Get nodes we'll use for testing
	bootstrapNode := nodes.GetBootstrapNode()
	regularNode := nodes.GetNodeByIndex(1)
	require.NotNil(t, regularNode, "Failed to get second node")

	// Define benchmark test cases with different record counts and payload sizes
	testCases := []struct {
		name           string
		numRecords     int           // Number of records to write
		payloadSize    int           // Size of each record in bytes
		payloadType    string        // Type of payload (string, json, binary)
		sampleInterval int           // Check every Nth record during verification
		waitTime       time.Duration // Time to wait after writing all records
	}{
		// {
		// 	name:           "Small records (300 × 50B)",
		// 	numRecords:     300,
		// 	payloadSize:    50,
		// 	payloadType:    "string",
		// 	sampleInterval: 10, // Verify every 10th record
		// 	waitTime:       300 * time.Millisecond,
		// },
		// {
		// 	name:           "Medium batch (1000 × 200B)",
		// 	numRecords:     1000,
		// 	payloadSize:    200,
		// 	payloadType:    "string",
		// 	sampleInterval: 100, // Verify every 100th record
		// 	waitTime:       1 * time.Second,
		// },
		{
			name:           "Large batch (5000 × 100B)",
			numRecords:     100000,
			payloadSize:    10,
			payloadType:    "string",
			sampleInterval: 500, // Verify every 500th record
			waitTime:       5 * time.Second,
		},
	}

	// Run all benchmark test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate test data
			t.Logf("Preparing %d records of %d bytes each", tc.numRecords, tc.payloadSize)

			// Pre-generate all keys and payloads
			keys := make([][32]byte, tc.numRecords)
			payloads := make([][]byte, tc.numRecords)

			for i := 0; i < tc.numRecords; i++ {
				// Generate unique key
				key, err := messages.GenerateRandomKey()
				require.NoError(t, err, "Failed to generate random key")
				keys[i] = key

				// Generate payload based on type
				var payload []byte
				switch tc.payloadType {
				case "string":
					// Random string data
					payload = make([]byte, tc.payloadSize)
					_, err := rand.Read(payload)
					require.NoError(t, err, "Failed to generate random payload")
				case "json":
					// JSON with random ID
					id := make([]byte, 8)
					_, err := rand.Read(id)
					require.NoError(t, err)
					json := fmt.Sprintf(`{"id":"%x","index":%d,"data":"padding_%s"}`,
						id, i, strings.Repeat("X", tc.payloadSize-50))
					payload = []byte(json)
				default:
					// Binary data
					payload = make([]byte, tc.payloadSize)
					_, err := rand.Read(payload)
					require.NoError(t, err, "Failed to generate random payload")
				}
				payloads[i] = payload
			}

			// 1. WRITE PHASE: Time how long it takes to write all records
			writeStart := time.Now()
			t.Log("Starting benchmark writes...")

			// Write all records
			for i := 0; i < tc.numRecords; i++ {
				writeMsg := &messages.Message{
					Handler: types.WriteHandlerType,
					Key:     keys[i],
					Data:    payloads[i],
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

				// Parse the write response
				writeDbResp, err := packets.DecodeDBResponse(writeResp)
				require.NoError(t, err, "Failed to decode write response")
				require.Equal(t, types.HandlerStatusSuccess, writeDbResp.Status, "Write operation should succeed with HandlerStatusSuccess")
			}

			// Calculate write performance
			writeDuration := time.Since(writeStart)
			writeOpsPerSec := float64(tc.numRecords) / writeDuration.Seconds()
			totalDataMB := float64(tc.numRecords*tc.payloadSize) / (1024 * 1024)
			mbPerSec := totalDataMB / writeDuration.Seconds()

			t.Logf("Write phase completed in %v", writeDuration)
			t.Logf("Write performance: %.2f ops/sec, %.2f MB/sec", writeOpsPerSec, mbPerSec)

			// Wait for P2P distribution to occur
			t.Logf("Waiting %v for P2P distribution to complete...", tc.waitTime)
			time.Sleep(tc.waitTime)

			// 2. READ PHASE: Sample records to verify distribution
			t.Logf("Verifying record distribution by sampling every %dth record", tc.sampleInterval)

			verifyStart := time.Now()
			verifiedCount := 0

			// Sample verification - check every Nth record
			for i := 0; i < tc.numRecords; i += tc.sampleInterval {
				readMsg := &messages.Message{
					Handler: types.ReadHandlerType,
					Key:     keys[i],
					Data:    nil,
				}
				encodedReadMsg, err := readMsg.Encode()
				require.NoError(t, err, "Failed to encode read message")

				// Try to read from regular node (should have received via P2P)
				regularResp, err := regularNode.SendAndReceiveMessage(
					regularNode,
					types.TCPTransportType,
					types.ReadHandlerType,
					encodedReadMsg,
					2*time.Second,
				)

				if err == nil {
					// Try to parse the response
					regularDbResp, decodeErr := packets.DecodeDBResponse(regularResp)
					if decodeErr == nil && regularDbResp.Status == types.HandlerStatusSuccess {
						// Success - check value
						regularValue := regularDbResp.Data
						if !bytes.Equal(payloads[i], regularValue) {
							t.Errorf("Record value mismatch on regular node for record %d", i)
							continue
						}
						verifiedCount++
					} else {
						t.Logf("Record #%d not found or error decoding response: %v", i, decodeErr)
					}
				} else {
					t.Logf("Record #%d not found on regular node yet: %v", i, err)
				}
			}

			verifyDuration := time.Since(verifyStart)
			samplesChecked := (tc.numRecords + tc.sampleInterval - 1) / tc.sampleInterval // ceiling division

			t.Logf("Verification completed in %v", verifyDuration)
			t.Logf("Distribution success rate: %d/%d samples verified (%.1f%%)",
				verifiedCount, samplesChecked, float64(verifiedCount)/float64(samplesChecked)*100)
			t.Logf("P2P distribution benchmark complete: %d records × %d bytes", tc.numRecords, tc.payloadSize)

			// Require a reasonable success rate (80%+ of the samples should be successful)
			successRate := float64(verifiedCount) / float64(samplesChecked)
			require.GreaterOrEqual(t, successRate, 0.8, "P2P distribution success rate should be at least 80%%")
		})
	}
}
