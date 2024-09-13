package fdb

import (
	"github.com/pkg/errors"
	"log"
	"net"
)

// ReadHandler struct with MDBX database passed in
type ReadHandler struct {
	db *Db // Pass the MDBX database instance here
}

// NewReadHandler creates a new ReadHandler with an MDBX database
func NewReadHandler(db *Db) *ReadHandler {
	return &ReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the ReadHandler
func (rh *ReadHandler) HandleMessage(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Example: Use the first 8 bytes of the buffer as the key
	key := buffer[:8]

	// Read the value from the MDBX database
	value, err := rh.db.Get(key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.Printf("Key not found in MDBX database: %v", err)
			_, _ = conn.WriteToUDP([]byte("Key not found"), addr)
		} else {
			log.Printf("Error reading from MDBX database: %v", err)
			_, _ = conn.WriteToUDP([]byte("Error reading from database"), addr)
		}
		return
	}

	// Send the retrieved value back to the client
	_, err = conn.WriteToUDP(value, addr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}
