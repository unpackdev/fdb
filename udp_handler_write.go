package fdb

import (
	"log"
	"net"
)

// WriteHandler struct with MDBX database passed in
type WriteHandler struct {
	db *Db // Pass the MDBX database instance here
}

// NewWriteHandler creates a new WriteHandler with an MDBX database
func NewWriteHandler(db *Db) *WriteHandler {
	return &WriteHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the WriteHandler
func (wh *WriteHandler) HandleMessage(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Example: Use the first 8 bytes as the key and the rest as the value
	if len(buffer) < 8 {
		log.Printf("Invalid message length: %d, expected at least 8 bytes", len(buffer))
		_, _ = conn.WriteToUDP([]byte("Invalid message format"), addr)
		return
	}
	
	key := buffer[:8]   // Example: First 8 bytes as key
	value := buffer[8:] // Example: Remaining bytes as value

	err := wh.db.Set(key, value)
	if err != nil {
		log.Printf("Error writing to MDBX database: %v", err)
	}

	// After writing to the database, send a response
	response := []byte("Message received and written to MDBX")
	_, err = conn.WriteToUDP(response, addr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}
