package fdb

import (
	"log"
	"net"
)

// ReadHandler struct with MDBX database passed in
type ReadHandler struct {
	db *Db // MDBX database instance
}

// NewReadHandler creates a new ReadHandler with an MDBX database
func NewReadHandler(db *Db) *ReadHandler {
	return &ReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the ReadHandler
func (rh *ReadHandler) HandleMessage(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Check if the buffer length is at least 33 bytes (1 byte for action + 32 bytes for key)
	if len(buffer) < 33 {
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(buffer))
		_, _ = conn.WriteToUDP([]byte("Invalid message format"), addr)
		return
	}

	key := buffer[1:33] // First 32 bytes after the action as key

	// Lookup the value in the MDBX database
	value, err := rh.db.Get(key)
	if err != nil {
		log.Printf("Error reading from MDBX database: %v", err)
		_, _ = conn.WriteToUDP([]byte("Error reading from database"), addr)
		return
	}

	// Send the value back to the client
	_, err = conn.WriteToUDP(value, addr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}
