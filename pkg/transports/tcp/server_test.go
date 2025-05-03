// pkg/transports/tcp/server_test.go
package tcp

/*
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/peerdns/peerd/pkg/logger"
	"github.com/peerdns/peerd/pkg/observability"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"testing"
	"time"

	"github.com/peerdns/peerd/pkg/config"
	"github.com/peerdns/peerd/pkg/protocols/rpc"
	"github.com/peerdns/peerd/pkg/types"
	"github.com/stretchr/testify/assert"
)

func setupServerTest(t testing.TB, ctx context.Context) (logger.Logger, *observability.Observability) {
	nodeConfig := config.Config{
		Logger: config.Logger{
			Enabled:     true,
			Environment: "development",
			Level:       "error", // Set to debug for detailed logs
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
	}

	gLog, err := logger.InitializeGlobalLogger(nodeConfig.Logger)
	require.NoError(t, err, "Failed to initialize global logger")

	// Initialize Observability
	obs, err := observability.New(ctx, nodeConfig, gLog)
	require.NoError(t, err, "Failed to initialize Observability")
	require.NotNil(t, obs)

	return gLog, obs
}

func TestTCPServer(t *testing.T) {
	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gLog, obs := setupServerTest(t, ctx)

	// Create TCP transport configuration without TLS for testing
	tcpConfig := config.TcpTransport{
		Type:    types.TCPTransport,
		Enabled: true,
		IPv4:    "127.0.0.1",
		Port:    8781, // Use a test port
		TLS:     nil,  // Disable TLS for simplicity
	}

	// Create the server
	server, err := NewServer(ctx, tcpConfig, gLog, obs)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initialize RPC Server and Register RPC Handlers
	rpcServer := rpc.NewRPCServer()
	rpc.RegisterRPCMethods(rpcServer)

	// Register RPC Handler
	server.RegisterHandler(types.RPCProtocol, rpc.JSONRPCHandler(rpcServer))

	// Start the server
	go func() {
		if err := server.Start(ctx); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait until the server has started
	select {
	case <-server.WaitStarted():
		// Server has started
	case <-time.After(2 * time.Second):
		t.Fatalf("Server did not start in time")
	}

	// Ensure the server is stopped after test
	defer func() {
		if err := server.Stop(); err != nil {
			t.Fatalf("Failed to stop server: %v", err)
		}
	}()

	// Connect to the server
	conn, err := net.Dial("tcp", tcpConfig.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Define test cases
	tests := []struct {
		name             string
		protocolType     types.ProtocolType
		request          []byte
		expectedResponse []byte
	}{
		{
			name:         "Test RPC Echo",
			protocolType: types.RPCProtocol,
			request: func() []byte {
				req := rpc.RPCRequest{
					ID:     1,
					Method: "echo",
					Params: json.RawMessage(`{"message":"Test RPC Echo"}`),
				}
				reqBytes, _ := rpc.MarshalRPCRequest(req)
				// Create request message with protocol type byte
				return createRequestMessage(types.RPCProtocol, reqBytes)
			}(),
			expectedResponse: func() []byte {
				resp := rpc.RPCResponse{
					ID:     1,
					Result: "Test RPC Echo",
				}
				respBytes, _ := json.Marshal(resp)
				// Create expected response message without protocol type byte
				return createResponseMessage(respBytes)
			}(),
		},
		{
			name:         "Test RPC Add",
			protocolType: types.RPCProtocol,
			request: func() []byte {
				req := rpc.RPCRequest{
					ID:     2,
					Method: "add",
					Params: json.RawMessage(`{"a":15,"b":25}`),
				}
				reqBytes, _ := rpc.MarshalRPCRequest(req)
				// Create request message with protocol type byte
				return createRequestMessage(types.RPCProtocol, reqBytes)
			}(),
			expectedResponse: func() []byte {
				resp := rpc.RPCResponse{
					ID:     2,
					Result: 40,
				}
				respBytes, _ := json.Marshal(resp)
				// Create expected response message without protocol type byte
				return createResponseMessage(respBytes)
			}(),
		},
		{
			name:         "Test RPC Unknown Method",
			protocolType: types.RPCProtocol,
			request: func() []byte {
				req := rpc.RPCRequest{
					ID:     3,
					Method: "unknown",
					Params: json.RawMessage(`{"a":5,"b":5}`),
				}
				reqBytes, _ := rpc.MarshalRPCRequest(req)
				// Create request message with protocol type byte
				return createRequestMessage(types.RPCProtocol, reqBytes)
			}(),
			expectedResponse: func() []byte {
				resp := rpc.RPCResponse{
					ID: 3,
					Error: &rpc.RPCError{
						Code:    -32601,
						Message: "Method not found",
					},
				}
				respBytes, _ := json.Marshal(resp)
				// Create expected response message without protocol type byte
				return createResponseMessage(respBytes)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Send the request
			_, err := conn.Write(tt.request)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}

			// Read the response length
			respLengthBuf := make([]byte, 4)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := io.ReadFull(conn, respLengthBuf)
			if err != nil {
				t.Fatalf("Failed to read response length: %v", err)
			}
			if n != 4 {
				t.Fatalf("Expected 4 bytes for response length, got %d", n)
			}
			respLength := binary.BigEndian.Uint32(respLengthBuf)

			// Read the response based on the length
			response := make([]byte, respLength)
			totalRead := 0
			for totalRead < int(respLength) {
				n, err := io.ReadFull(conn, response[totalRead:])
				if err != nil {
					t.Fatalf("Failed to read response: %v", err)
				}
				totalRead += n
			}

			// Compare the entire response
			assert.Equal(t, tt.expectedResponse, response, "Unexpected response from server")
		})
	}
}
*/
