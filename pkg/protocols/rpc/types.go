// pkg/protocols/rpc/types.go

package rpc

import (
	"encoding/json"

	"github.com/unpackdev/fdb/pkg/state"
)

// Error codes as defined by the JSON-RPC 2.0 specification.
const (
	RpcStateType state.StateType = "rpc"

	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603

	// Custom error codes
	Unauthorized = -32604
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"` // Must be "2.0"
	ID      any             `json:"id"`      // Can be string, number, or null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`          // Must be "2.0"
	ID      any    `json:"id"`               // Should match the request ID
	Result  any    `json:"result,omitempty"` // Result is mutually exclusive with Error
	Error   *Error `json:"error,omitempty"`  // Error is mutually exclusive with Result
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"` // Optional additional information about the error
}

// NewError creates a new Error instance with the given code and message.
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

type Notification struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

type NotificationParams struct {
	Subscription string `json:"subscription"`
	Result       any    `json:"result"`
}
