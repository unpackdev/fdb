// pkg/protocols/rpc/subscription_benchmark_test.go
package rpc_test

import (
	"encoding/json"
	"fmt"
	"github.com/unpackdev/fdb/protocols/rpc"
	"net/url"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func BenchmarkSubscriptionBroadcast(b *testing.B) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Setup the RPC server
	rpcInstance, addr, cleanup := rpc.SetupRPCServerForTest(b)
	defer cleanup()

	// Register subscription handlers
	rpcInstance.Server().RegisterMethod("subscribe", rpc.SubscribeHandler)
	rpcInstance.Server().RegisterMethod("unsubscribe", rpc.UnsubscribeHandler)

	// Number of subscribers
	numSubscribers := 1000

	// Prepare slices to hold connections
	subscribers := make([]*websocket.Conn, numSubscribers)

	// Subscribe clients to the event
	for i := 0; i < numSubscribers; i++ {
		// Connect to the server over WebSocket
		url := url.URL{Scheme: "ws", Host: addr, Path: "/"}
		c, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
		require.NoError(b, err, "Failed to connect to WebSocket server")

		// Subscribe to an event
		subscribeRequest := rpc.Request{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "subscribe",
			Params: json.RawMessage(`{
                "method": "benchmark_event",
                "params": {}
            }`),
		}

		// Marshal the request
		reqBytes, err := json.Marshal(subscribeRequest)
		require.NoError(b, err, "Failed to marshal subscribe request")

		// Send the subscription request
		err = c.WriteMessage(websocket.TextMessage, reqBytes)
		require.NoError(b, err, "Failed to send subscription request")

		// Read the subscription response
		_, respBytes, err := c.ReadMessage()
		require.NoError(b, err, "Failed to read subscription response")

		// Unmarshal the response
		var subscribeResponse rpc.Response
		err = json.Unmarshal(respBytes, &subscribeResponse)
		require.NoError(b, err, "Failed to unmarshal subscription response")

		// Check for errors
		require.Nil(b, subscribeResponse.Error, "Subscription request returned an error")

		// Save the connection
		subscribers[i] = c
	}

	// Allow subscribers to be ready
	time.Sleep(100 * time.Millisecond)

	// Reset the timer
	b.ResetTimer()

	// Start the benchmark
	for i := 0; i < b.N; i++ {
		// Broadcast the event
		eventData := fmt.Sprintf("Benchmark event data %d", i)
		rpcInstance.Server().BroadcastEvent("benchmark_event", eventData)
	}

	// Stop the timer
	b.StopTimer()

	// After broadcasting events
	var wg sync.WaitGroup
	for _, c := range subscribers {
		wg.Add(1)
		go func(conn *websocket.Conn) {
			defer wg.Done()
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
						b.Logf("Error reading message: %v", err)
					}
					break
				}
			}
		}(c)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
