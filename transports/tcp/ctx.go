// pkg/transports/tcp/connection_context.go
package tcp

import (
	"bufio"
	"bytes"
	"context"
	"github.com/unpackdev/fdb/transports"
)

// ConnectionContext holds the per-connection buffer and metadata.
type ConnectionContext struct {
	Buffer        []byte
	BytesReader   *bytes.Reader
	BufioReader   *bufio.Reader
	Conn          transports.Connection
	WSUpgraded    bool               // Indicates if the connection has been upgraded to WebSocket.
	ProtocolState any                // Holds protocol-specific state (e.g., WebSocket codec).
	Protocol      string             // Tracks the protocol ("http", "websocket").
	Ctx           context.Context    // Context for this connection.
	Cancel        context.CancelFunc // Cancel function to cancel the context.
}
