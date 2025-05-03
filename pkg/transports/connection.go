// pkg/transports/connection.go
package transports

import "github.com/panjf2000/gnet/v2"

// Connection represents a generic connection interface that transports use to send and receive data.
type Connection interface {
	// Send sends data to the connection.
	Send(data []byte) error

	SendWithCallback(data []byte, callback gnet.AsyncCallback) error

	// Close closes the connection.
	Close() error

	// RemoteAddr returns the remote address of the connection.
	RemoteAddr() string
}
