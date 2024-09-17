package client

import (
	"context"

	"github.com/panjf2000/gnet/v2"
)

// HandlerFunc defines the function signature for handlers
type HandlerFunc func(c gnet.Conn, data []byte) error

// Transport interface defines the methods that all transports must implement
type Transport interface {
	Connect(ctx context.Context) error
	Send(data []byte) error
	Close() error
	RegisterHandler(messageType MessageType, handler HandlerFunc)
}
