// pkg/protocols/rpc/subscription_test.go
package rpc_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/unpackdev/fdb/pkg/protocols/rpc"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionFunctionality(t *testing.T) {
	// Setup the RPC server with subscription handlers
	rpcInstance, addr, cleanup := rpc.SetupRPCServerForTest(t)
	defer cleanup()

	// Register the subscription handlers
	rpcInstance.Server().RegisterMethod("subscribe", rpc.SubscribeHandler)
	rpcInstance.Server().RegisterMethod("unsubscribe", rpc.UnsubscribeHandler)

	// Connect to the server over WebSocket
	url := url.URL{Scheme: "ws", Host: addr, Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	require.NoError(t, err, "Failed to connect to WebSocket server")
	defer c.Close()

	// Subscribe to an event
	subscribeRequest := rpc.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "subscribe",
		Params: json.RawMessage(`{
            "method": "test_event",
            "params": {}
        }`),
	}

	// Marshal the request
	reqBytes, err := json.Marshal(subscribeRequest)
	require.NoError(t, err, "Failed to marshal subscribe request")

	// Send the subscription request
	err = c.WriteMessage(websocket.TextMessage, reqBytes)
	require.NoError(t, err, "Failed to send subscription request")

	// Read the subscription response
	_, respBytes, err := c.ReadMessage()
	require.NoError(t, err, "Failed to read subscription response")

	// Unmarshal the response
	var subscribeResponse rpc.Response
	err = json.Unmarshal(respBytes, &subscribeResponse)
	require.NoError(t, err, "Failed to unmarshal subscription response")

	// Check for errors
	require.Nil(t, subscribeResponse.Error, "Subscription request returned an error")

	// Extract the subscription ID
	subscriptionID, ok := subscribeResponse.Result.(string)
	require.True(t, ok, "Subscription ID should be a string")
	require.NotEmpty(t, subscriptionID, "Subscription ID should not be empty")

	// Simulate an event broadcast from the server
	eventData := "Test event data"
	rpcInstance.Server().BroadcastEvent("test_event", eventData)

	// Read the notification
	_, notifBytes, err := c.ReadMessage()
	require.NoError(t, err, "Failed to read notification")

	// Unmarshal the notification
	var notification rpc.Notification
	err = json.Unmarshal(notifBytes, &notification)
	require.NoError(t, err, "Failed to unmarshal notification")

	// Verify the notification
	assert.Equal(t, "2.0", notification.JSONRPC, "JSONRPC version mismatch")
	assert.Equal(t, "test_event", notification.Method, "Method mismatch")
	assert.Equal(t, subscriptionID, notification.Params.Subscription, "Subscription ID mismatch")

	// Verify the event data
	result, ok := notification.Params.Result.(string)
	require.True(t, ok, "Expected result to be a string")
	assert.Equal(t, eventData, result, "Event data mismatch")

	// Unsubscribe from the event
	unsubscribeRequest := rpc.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "unsubscribe",
		Params:  json.RawMessage(fmt.Sprintf(`"%s"`, subscriptionID)),
	}

	// Marshal the unsubscribe request
	reqBytes, err = json.Marshal(unsubscribeRequest)
	require.NoError(t, err, "Failed to marshal unsubscribe request")

	// Send the unsubscribe request
	err = c.WriteMessage(websocket.TextMessage, reqBytes)
	require.NoError(t, err, "Failed to send unsubscribe request")

	// Read the unsubscribe response
	_, respBytes, err = c.ReadMessage()
	require.NoError(t, err, "Failed to read unsubscribe response")

	// Unmarshal the response
	var unsubscribeResponse rpc.Response
	err = json.Unmarshal(respBytes, &unsubscribeResponse)
	require.NoError(t, err, "Failed to unmarshal unsubscribe response")

	// Check for errors
	require.Nil(t, unsubscribeResponse.Error, "Unsubscribe request returned an error")

	// Verify the unsubscribe result
	unsubscribed, ok := unsubscribeResponse.Result.(bool)
	require.True(t, ok, "Expected unsubscribe result to be a boolean")
	assert.True(t, unsubscribed, "Unsubscribe result should be true")

	// Attempt to receive another notification after unsubscribing
	// Simulate another event broadcast
	rpcInstance.Server().BroadcastEvent("test_event", "This should not be received")

	// Set a short read deadline
	c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	// Try to read a notification (we expect this to fail)
	_, _, err = c.ReadMessage()
	require.Error(t, err, "Expected read to timeout or fail after unsubscribing")
}
