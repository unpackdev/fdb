// pkg/transports/tcp/server_benchmark_test.go
package tcp

/*
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/peerdns/peerd/pkg/config"
	"github.com/peerdns/peerd/pkg/protocols/rpc"
	"github.com/peerdns/peerd/pkg/types"
	"github.com/stretchr/testify/assert"
)

func BenchmarkTCPServer(b *testing.B) {
	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gLog, obs := setupServerTest(b, ctx)

	// Choose a unique port to avoid conflicts
	port := 8782

	config := config.Config{
		Rpc: config.Rpc{
			PoolMaxSize: 1,
			Transport: config.TcpTransport{
				Type:    types.TCPTransport,
				Enabled: true,
				IPv4:    net.ParseIP("127.0.0.1"),
				Port:    port, // Unique port for benchmarking
				TLS:     nil,  // Disable TLS for simplicity
			},
		},
	}

	// Create the server
	server, err := NewServer(ctx, tcpConfig, gLog, obs)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	// Initialize RPC Server and Register RPC Handlers
	rpcServer := rpc.NewRPC(ctx, gLog, obs) // **Fixed:** Passed gLog and obs
	rpc.RegisterDefaultHandlers(rpcServer)

	// Register RPC Handler
	server.RegisterHandler(types.RPCProtocol, rpc.JSONRPCHandler(rpcServer))

	// Error channel to capture server.Start() errors
	startErrChan := make(chan error, 1)

	// Start the server
	go func() {
		err := server.Start(ctx)
		if err != nil {
			startErrChan <- err
		}
	}()

	// Wait until the server has started or an error occurs
	select {
	case <-server.WaitStarted():
		b.Log("TCP Server successfully started")
	case err := <-startErrChan:
		b.Fatalf("Failed to start server: %v", err)
	case <-time.After(5 * time.Second):
		b.Fatalf("TCP server did not start in time")
	}

	// Ensure the server is stopped after benchmark
	defer func() {
		if err := server.Stop(); err != nil {
			b.Fatalf("Failed to stop server: %v", err)
		}
	}()

	// Pre-define RPC Echo Request
	echoRequest := rpc.RPCRequest{
		ID:     1,
		Method: "echo",
		Params: json.RawMessage(`{"message":"Benchmark RPC Echo"}`),
	}
	echoRequestBytes, err := rpc.MarshalRPCRequest(echoRequest)
	if err != nil {
		b.Fatalf("Failed to marshal RPC request: %v", err)
	}
	echoMessage := createRequestMessage(types.RPCProtocol, echoRequestBytes)

	// Pre-define RPC Add Request
	addRequest := rpc.RPCRequest{
		ID:     2,
		Method: "add",
		Params: json.RawMessage(`{"a":100,"b":200}`),
	}
	addRequestBytes, err := rpc.MarshalRPCRequest(addRequest)
	if err != nil {
		b.Fatalf("Failed to marshal RPC request: %v", err)
	}
	addMessage := createRequestMessage(types.RPCProtocol, addRequestBytes)

	// Pre-define expected responses for comparison
	echoResponse := rpc.RPCResponse{
		ID:     1,
		Result: "Benchmark RPC Echo",
	}
	echoResponseBytes, err := json.Marshal(echoResponse)
	if err != nil {
		b.Fatalf("Failed to marshal RPC response: %v", err)
	}
	expectedEchoResponse := createResponseMessage(echoResponseBytes)

	addResponse := rpc.RPCResponse{
		ID:     2,
		Result: 300,
	}
	addResponseBytes, err := json.Marshal(addResponse)
	if err != nil {
		b.Fatalf("Failed to marshal RPC response: %v", err)
	}
	expectedAddResponse := createResponseMessage(addResponseBytes)

	// Separate benchmarks for clarity and isolation
	b.Run("SequentialRequests", func(b *testing.B) {
		// Establish a single connection for all sequential requests
		conn, err := net.Dial("tcp", tcpConfig.Addr())
		if err != nil {
			b.Fatalf("Failed to connect to server for SequentialRequests: %v", err)
		}
		defer conn.Close()

		for i := 0; i < b.N; i++ {
			// Send Echo request
			_, err := conn.Write(echoMessage)
			if err != nil {
				b.Fatalf("Failed to send Echo request: %v", err)
			}

			// Read Echo response length
			respLengthBuf := make([]byte, 4)
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, err = io.ReadFull(conn, respLengthBuf)
			if err != nil {
				b.Fatalf("Failed to read Echo response length: %v", err)
			}
			respLength := binary.BigEndian.Uint32(respLengthBuf)

			// Read Echo response based on the length
			response := make([]byte, respLength)
			_, err = io.ReadFull(conn, response)
			if err != nil {
				b.Fatalf("Failed to read Echo response: %v", err)
			}

			// Compare the entire response
			assert.Equal(b, expectedEchoResponse, response, "Unexpected Echo response from server")

			// Send Add request
			_, err = conn.Write(addMessage)
			if err != nil {
				b.Fatalf("Failed to send Add request: %v", err)
			}

			// Read Add response length
			respLengthBuf = make([]byte, 4)
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, err = io.ReadFull(conn, respLengthBuf)
			if err != nil {
				b.Fatalf("Failed to read Add response length: %v", err)
			}
			respLength = binary.BigEndian.Uint32(respLengthBuf)

			// Read Add response based on the length
			addResp := make([]byte, respLength)
			_, err = io.ReadFull(conn, addResp)
			if err != nil {
				b.Fatalf("Failed to read Add response: %v", err)
			}

			// Compare the entire response
			assert.Equal(b, expectedAddResponse, addResp, "Unexpected Add response from server")
		}
	})

	b.Run("ParallelRequests", func(b *testing.B) {
		// Use b.RunParallel for managing goroutines efficiently
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Establish a new connection for each iteration
				conn, err := net.Dial("tcp", tcpConfig.Addr())
				if err != nil {
					b.Error("Failed to connect to server in ParallelRequests:", err)
					continue
				}

				// Ensure the connection is closed after handling the requests
				func() {
					defer conn.Close()

					// Send Echo request
					_, err = conn.Write(echoMessage)
					if err != nil {
						b.Error("Failed to send Echo request in ParallelRequests:", err)
						return
					}

					// Read Echo response length
					respLengthBuf := make([]byte, 4)
					conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					_, err = io.ReadFull(conn, respLengthBuf)
					if err != nil {
						b.Error("Failed to read Echo response length in ParallelRequests:", err)
						return
					}
					respLength := binary.BigEndian.Uint32(respLengthBuf)

					// Read Echo response based on the length
					response := make([]byte, respLength)
					_, err = io.ReadFull(conn, response)
					if err != nil {
						b.Error("Failed to read Echo response in ParallelRequests:", err)
						return
					}

					// Compare the entire response
					if !assert.Equal(b, expectedEchoResponse, response, "Unexpected Echo response from server in ParallelRequests") {
						return
					}

					// Send Add request
					_, err = conn.Write(addMessage)
					if err != nil {
						b.Error("Failed to send Add request in ParallelRequests:", err)
						return
					}

					// Read Add response length
					respLengthBuf = make([]byte, 4)
					conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					_, err = io.ReadFull(conn, respLengthBuf)
					if err != nil {
						b.Error("Failed to read Add response length in ParallelRequests:", err)
						return
					}
					respLength = binary.BigEndian.Uint32(respLengthBuf)

					// Read Add response based on the length
					addResp := make([]byte, respLength)
					_, err = io.ReadFull(conn, addResp)
					if err != nil {
						b.Error("Failed to read Add response in ParallelRequests:", err)
						return
					}

					// Compare the entire response
					if !assert.Equal(b, expectedAddResponse, addResp, "Unexpected Add response from server in ParallelRequests") {
						return
					}
				}()
			}
		})
	})
}
*/
