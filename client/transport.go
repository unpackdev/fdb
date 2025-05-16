package client

import (
	"context"
)

// HandlerFunc defines the function signature for handlers
type HandlerFunc func(data []byte) error

// Transport interface defines the methods that all transports must implement
type Transport interface {
	Connect(ctx context.Context) error
	Send(data []byte) error
	Close() error
	RegisterHandler(messageType MessageType, handler HandlerFunc)
}
