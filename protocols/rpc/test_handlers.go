// pkg/protocols/rpc/handlers_test.go
package rpc

import (
	"context"
	"encoding/json"
)

// EchoHandler returns the received message.
func EchoHandler(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	var echoParams struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &echoParams); err != nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Invalid params for echo method",
		}
	}
	return echoParams.Message, nil
}

// AddHandler returns the sum of two integers as float64.
func AddHandler(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
	var addParams struct {
		A int  `json:"a"`
		B *int `json:"b"` // Pointer to detect missing 'b'
	}
	if err := json.Unmarshal(params, &addParams); err != nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Invalid params for add method",
		}
	}

	// Validate that 'b' is provided
	if addParams.B == nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Missing 'b' parameter for add method",
		}
	}

	sum := addParams.A + *addParams.B
	return float64(sum), nil // Return as float64
}

// RegisterTestHandlers registers the default RPC test methods.
func RegisterTestHandlers(server *Server) {
	server.RegisterMethod("echo", EchoHandler)
	server.RegisterMethod("add", AddHandler)
}
