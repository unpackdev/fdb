// pkg/protocols/rpc/rpc_test.go
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPCOverHTTP(t *testing.T) {
	_, addr, cleanup := rpc.SetupRPCServerForTest(t)
	defer cleanup()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Define test cases.
	tests := []struct {
		name             string
		request          rpc.Request
		expectedResponse rpc.Response
	}{
		{
			name: "Echo Method",
			request: rpc.Request{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "echo",
				Params:  json.RawMessage(`{"message":"Hello, RPC over HTTP!"}`),
			},
			expectedResponse: rpc.Response{
				JSONRPC: "2.0",
				ID:      1,
				Result:  "Hello, RPC over HTTP!",
			},
		},
		{
			name: "Add Method",
			request: rpc.Request{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "add",
				Params:  json.RawMessage(`{"a":10,"b":20}`),
			},
			expectedResponse: rpc.Response{
				JSONRPC: "2.0",
				ID:      2,
				Result:  30.0, // float64
			},
		},
		{
			name: "Unknown Method",
			request: rpc.Request{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "subtract",
				Params:  json.RawMessage(`{"a":10,"b":5}`),
			},
			expectedResponse: rpc.Response{
				JSONRPC: "2.0",
				ID:      3,
				Error: &rpc.Error{
					Code:    rpc.MethodNotFound,
					Message: "Method not found",
				},
			},
		},
		{
			name: "Invalid Params",
			request: rpc.Request{
				JSONRPC: "2.0",
				ID:      4,
				Method:  "add",
				Params:  json.RawMessage(`{"a":10}`), // Missing 'b'
			},
			expectedResponse: rpc.Response{
				JSONRPC: "2.0",
				ID:      4,
				Error: &rpc.Error{
					Code:    rpc.InvalidParams,
					Message: "Missing 'b' parameter for add method",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the request.
			reqBytes, err := json.Marshal(tt.request)
			require.NoError(t, err, "Failed to marshal request")

			// Send HTTP POST request to /rpc endpoint.
			url := fmt.Sprintf("http://%s/rpc", addr)
			httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBytes))
			require.NoError(t, err, "Failed to create HTTP request")
			httpReq.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(httpReq)
			require.NoError(t, err, "Failed to send HTTP request")
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode, "Unexpected HTTP status code")

			// Read the response body.
			respBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "Failed to read response body")

			// Unmarshal the response.
			var rpcResp rpc.Response
			err = json.Unmarshal(respBytes, &rpcResp)
			require.NoError(t, err, "Failed to unmarshal RPC response")

			// Compare the JSONRPC version.
			assert.Equal(t, tt.expectedResponse.JSONRPC, rpcResp.JSONRPC, "JSONRPC version mismatch")

			// Compare the response ID using EqualValues.
			assert.EqualValues(t, tt.expectedResponse.ID, rpcResp.ID, "Response ID mismatch")

			if tt.expectedResponse.Error != nil {
				require.NotNil(t, rpcResp.Error, "Expected an error but got none")
				assert.Equal(t, tt.expectedResponse.Error.Code, rpcResp.Error.Code, "Error code mismatch")
				assert.Equal(t, tt.expectedResponse.Error.Message, rpcResp.Error.Message, "Error message mismatch")
			} else {
				assert.Nil(t, rpcResp.Error, "Did not expect an error but got one")
				// Handle based on expected result type.
				switch expected := tt.expectedResponse.Result.(type) {
				case string:
					result, ok := rpcResp.Result.(string)
					require.True(t, ok, "Expected Result to be string")
					assert.Equal(t, expected, result, "Result mismatch")
				case float64:
					result, ok := rpcResp.Result.(float64)
					require.True(t, ok, "Expected Result to be float64")
					assert.Equal(t, expected, result, "Result mismatch")
				default:
					t.Errorf("Unsupported expected result type: %T", expected)
				}
			}
		})
	}
}

func TestRPCOverWebSocket(t *testing.T) {
	_, addr, cleanup := rpc.SetupRPCServerForTest(t)
	defer cleanup()

	url := neturl.URL{Scheme: "ws", Host: addr, Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	require.NoError(t, err, "Failed to connect to WebSocket server")
	defer c.Close()

	// Define test case.
	request := rpc.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "echo",
		Params:  json.RawMessage(`{"message":"Hello, RPC over WebSocket!"}`),
	}
	expectedResponse := rpc.Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "Hello, RPC over WebSocket!",
	}

	// Marshal the request.
	reqBytes, err := json.Marshal(request)
	require.NoError(t, err, "Failed to marshal request")

	// Send the request over WebSocket.
	err = c.WriteMessage(websocket.TextMessage, reqBytes)
	require.NoError(t, err, "Failed to send WebSocket message")

	// Read the response.
	_, respBytes, err := c.ReadMessage()
	require.NoError(t, err, "Failed to read WebSocket message")

	// Unmarshal the response.
	var rpcResp rpc.Response
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err, "Failed to unmarshal RPC response")

	// Compare the JSONRPC version.
	assert.Equal(t, expectedResponse.JSONRPC, rpcResp.JSONRPC, "JSONRPC version mismatch")

	// Compare the response ID using EqualValues.
	assert.EqualValues(t, expectedResponse.ID, rpcResp.ID, "Response ID mismatch")

	assert.Nil(t, rpcResp.Error, "Did not expect an error but got one")
	result, ok := rpcResp.Result.(string)
	require.True(t, ok, "Expected Result to be string")
	assert.Equal(t, expectedResponse.Result, result, "Result mismatch")
}
