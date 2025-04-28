// pkg/transports/connection.go
package transports

// Connection represents a generic connection interface that transports use to send and receive data.
type Connection interface {
	// Send sends data to the connection.
	Send(data []byte)

	// Close closes the connection.
	Close() error

	// RemoteAddr returns the remote address of the connection.
	RemoteAddr() string
}
