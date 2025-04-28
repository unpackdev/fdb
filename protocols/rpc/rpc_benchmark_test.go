// pkg/protocols/rpc/rpc_benchmark_test.go
package rpc_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/unpackdev/fdb/protocols/rpc"
	"io"
	"net/http"
	neturl "net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func BenchmarkRPCOverHTTP(b *testing.B) {
	// Setup the RPC server
	_, addr, cleanup := rpc.SetupRPCServerForTest(b)
	defer cleanup()

	// Prepare the request data
	request := rpc.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "echo",
		Params:  json.RawMessage(`{"message":"Benchmarking RPC over HTTP"}`),
	}
	reqBytes, err := json.Marshal(request)
	require.NoError(b, err, "Failed to marshal request")

	// Prepare the URL
	url := fmt.Sprintf("http://%s/rpc", addr)

	// Create a persistent HTTP client with keep-alive
	transport := &http.Transport{
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	// Reset the timer to exclude setup time
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create a new HTTP request for each iteration
		httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBytes))
		if err != nil {
			b.Fatalf("Failed to create HTTP request: %v", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		// Keep-Alive header
		httpReq.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(httpReq)
		if err != nil {
			b.Fatalf("Failed to send HTTP request: %v", err)
		}
		io.Copy(io.Discard, resp.Body) // Discard the response body
		resp.Body.Close()
	}
}

func BenchmarkRPCOverWebSocket(b *testing.B) {
	// Setup the RPC server
	_, addr, cleanup := rpc.SetupRPCServerForTest(b)
	defer cleanup()

	url := neturl.URL{Scheme: "ws", Host: addr, Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	require.NoError(b, err, "Failed to connect to WebSocket server")
	defer c.Close()

	// Prepare the request
	request := rpc.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "echo",
		Params:  json.RawMessage(`{"message":"Benchmarking RPC over WebSocket"}`),
	}
	reqBytes, err := json.Marshal(request)
	require.NoError(b, err, "Failed to marshal request")

	// Reset the timer to exclude setup time
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Send the request
		err = c.WriteMessage(websocket.TextMessage, reqBytes)
		require.NoError(b, err, "Failed to send WebSocket message")

		// Read the response
		_, _, err := c.ReadMessage()
		require.NoError(b, err, "Failed to read WebSocket message")
	}
}

func BenchmarkRPCOverHTTPParallel(b *testing.B) {
	// Setup the RPC server
	_, addr, cleanup := rpc.SetupRPCServerForTest(b)
	defer cleanup()

	// Prepare the request data
	request := rpc.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "echo",
		Params:  json.RawMessage(`{"message":"Benchmarking RPC over HTTP with concurrency"}`),
	}
	reqBytes, err := json.Marshal(request)
	if err != nil {
		b.Fatalf("Failed to marshal request: %v", err)
	}

	// Prepare the URL
	url := fmt.Sprintf("http://%s/rpc", addr)

	// Create an HTTP client with connection pooling
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
		},
	}

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Use b.RunParallel to simulate concurrent clients
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Create a new HTTP request for each iteration
			httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBytes))
			if err != nil {
				b.Fatalf("Failed to create HTTP request: %v", err)
			}
			httpReq.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(httpReq)
			if err != nil {
				b.Fatalf("Failed to send HTTP request: %v", err)
			}
			io.Copy(io.Discard, resp.Body) // Discard the response body
			resp.Body.Close()
		}
	})
}

func BenchmarkRPCOverWebSocketConcurrent(b *testing.B) {
	// Setup the RPC server
	_, addr, cleanup := rpc.SetupRPCServerForTest(b)
	defer cleanup()

	// Prepare the request data
	request := rpc.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "echo",
		Params:  json.RawMessage(`{"message":"Benchmarking RPC over WebSocket with concurrency"}`),
	}
	reqBytes, err := json.Marshal(request)
	if err != nil {
		b.Fatalf("Failed to marshal request: %v", err)
	}

	// Prepare the URL
	url := neturl.URL{Scheme: "ws", Host: addr, Path: "/"}

	// Determine the number of goroutines (concurrent clients)
	numClients := 100

	// Channels for synchronization
	done := make(chan struct{})
	errCh := make(chan error, numClients)

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Launch goroutines
	for i := 0; i < numClients; i++ {
		go func() {
			// Establish a WebSocket connection
			c, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
			if err != nil {
				errCh <- fmt.Errorf("Failed to connect to WebSocket server: %v", err)
				return
			}
			defer c.Close()

			// Send requests
			for j := 0; j < b.N/numClients; j++ {
				// Send the request
				err = c.WriteMessage(websocket.TextMessage, reqBytes)
				if err != nil {
					errCh <- fmt.Errorf("Failed to send WebSocket message: %v", err)
					return
				}

				// Read the response
				_, _, err := c.ReadMessage()
				if err != nil {
					errCh <- fmt.Errorf("Failed to read WebSocket message: %v", err)
					return
				}
			}

			// Signal completion
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines to finish or an error to occur
	for i := 0; i < numClients; i++ {
		select {
		case <-done:
			// One client finished
		case err := <-errCh:
			b.Fatalf("Error during benchmark: %v", err)
		}
	}
}
