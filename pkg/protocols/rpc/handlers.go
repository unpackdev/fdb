// pkg/protocols/rpc/handlers.go
package rpc

import (
	"context"
	json "github.com/goccy/go-json"
	"time"
)

type HandlerType string

type HandlerMethodName string

func (h HandlerType) String() string {
	return string(h)
}

func (h HandlerMethodName) String() string {
	return string(h)
}

// HandlerFunc defines the function signature for handling RPC methods.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, *Error)

// SubscribeHandler handles subscription requests.
func SubscribeHandler(ctx context.Context, params json.RawMessage) (any, *Error) {
	// Extract the ClientConnection from context
	clientConn, ok := ctx.Value(clientConnKey).(*ClientConnection)
	if !ok {
		return nil, &Error{
			Code:    InternalError,
			Message: "Failed to retrieve client connection",
		}
	}

	// Parse the subscription parameters
	var subParams struct {
		Method string      `json:"method"`
		Params interface{} `json:"params"`
	}
	if err := json.Unmarshal(params, &subParams); err != nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Invalid params for subscribe method",
		}
	}

	// Generate a unique subscription ID
	subscriptionID := generateSubscriptionID()

	// Create a new Subscription
	subscription := &Subscription{
		ID:         subscriptionID,
		Method:     subParams.Method,
		Params:     subParams.Params,
		CreatedAt:  time.Now(),
		Connection: clientConn,
	}

	// Add the subscription to the client's subscription list
	clientConn.Mu.Lock()
	clientConn.Subscriptions[subscriptionID] = subscription
	clientConn.Mu.Unlock()

	// Return the subscription ID to the client
	return subscriptionID, nil
}

// UnsubscribeHandler handles unsubscription requests.
func UnsubscribeHandler(ctx context.Context, params json.RawMessage) (any, *Error) {
	// Extract the ClientConnection from context using the correct key
	clientConn, ok := ctx.Value(clientConnKey).(*ClientConnection)
	if !ok {
		return nil, &Error{
			Code:    InternalError,
			Message: "Failed to retrieve client connection",
		}
	}

	// Parse the subscription ID
	var subID string
	if err := json.Unmarshal(params, &subID); err != nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Invalid params for unsubscribe method",
		}
	}

	// Remove the subscription
	clientConn.Mu.Lock()
	delete(clientConn.Subscriptions, subID)
	clientConn.Mu.Unlock()

	return true, nil
}
