// pkg/transports/tcp/connection_context.go
package tcp

import (
	"bufio"
	"bytes"
	"context"

	"github.com/unpackdev/fdb/pkg/transports"
)

// ChunkedMessageInfo stores state for reassembling large messages that exceed TCP packet size
type ChunkedMessageInfo struct {
	TotalSize   uint32        // Total expected message size
	CurrentSize uint32        // Current bytes received
	Buffer      *bytes.Buffer // Buffer for building the complete message
}

// ConnectionContext holds the per-connection buffer and metadata.
type ConnectionContext struct {
	Buffer             []byte
	BytesReader        *bytes.Reader
	BufioReader        *bufio.Reader
	Conn               transports.Connection
	WSUpgraded         bool                // Indicates if the connection has been upgraded to WebSocket.
	ProtocolState      any                 // Holds protocol-specific state (e.g., WebSocket codec).
	Protocol           string              // Tracks the protocol ("http", "websocket").
	Ctx                context.Context     // Context for this connection.
	Cancel             context.CancelFunc  // Cancel function to cancel the context.
	ChunkedMessageInfo *ChunkedMessageInfo // For handling large chunked messages
}
