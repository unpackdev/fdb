// pkg/transports/tcp/connection.go
package tcp

import (
	"encoding/binary"

	"github.com/panjf2000/gnet/v2"
)

// Connection implements transports.Connection for TCP.
type Connection struct {
	conn gnet.Conn
}

// NewTCPConnection creates a new TCPConnection.
func NewTCPConnection(conn gnet.Conn) *Connection {
	return &Connection{conn: conn}
}

func (c *Connection) Conn() gnet.Conn {
	return c.conn
}

// Send sends data to the connection without additional framing.
// Since the custom OnTraffic handler handles raw HTTP, no length prefix or protocol type byte is needed.
func (c *Connection) Send(data []byte) error {
	return c.conn.AsyncWrite(data, nil)
}

func (c *Connection) SendWithCallback(data []byte, callback gnet.AsyncCallback) error {
	return c.conn.AsyncWrite(data, callback)
}

// SendCustom sends data to the connection with a length prefix.
func (c *Connection) SendCustom(data []byte) error {
	length := uint32(len(data))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)
	// Prepend the length prefix to the actual data
	fullMessage := append(lengthBuf, data...)
	// Use AsyncWrite with a nil callback as we're not handling callbacks here.
	return c.conn.AsyncWrite(fullMessage, nil)
}

// Close closes the connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote address of the connection.
func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}
