package transports

import (
	"log"

	"github.com/unpackdev/fdb/pkg/db"
)

// TCPReadHandler struct with MDBX database passed in
type TCPReadHandler struct {
	db db.Provider // MDBX database instance
}

// NewTCPReadHandler creates a new TCPReadHandler with an MDBX database
func NewTCPReadHandler(db db.Provider) *TCPReadHandler {
	return &TCPReadHandler{
		db: db,
	}
}

// Handle processes the incoming message using the TCPReadHandler
func (rh *TCPReadHandler) Handle(conn Connection, frame []byte) {
	if len(frame) < 33 { // 1 byte action + 32-byte key
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(frame))
		conn.Send([]byte("Invalid message format"))
		return
	}

	// Extract the key (32 bytes starting from the second byte)
	key := frame[1:33]

	// Read from the database using the key
	value, err := rh.db.Get(key)
	if err != nil {
		log.Printf("Error reading from database: %v", err)
		conn.Send([]byte("Error reading from database"))
		return
	}

	if len(value) == 0 {
		log.Printf("No value found for key: %x", key)
		conn.Send([]byte("No value found for key"))
		return
	}

	// Send the value back to the client
	conn.Send(value)
}
