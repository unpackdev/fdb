package fdb

import (
	"github.com/panjf2000/gnet"
	"log"
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
func (rh *ReadHandler) HandleMessage(c gnet.Conn, frame []byte) {
	// Check frame length
	if len(frame) < 33 { // 1 byte action + 32-byte key
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(frame))
		c.SendTo([]byte("Invalid message format"))
		return
	}

	key := frame[1:33] // 32-byte key

	// Read from the database
	value, err := rh.db.Get(key)
	if err != nil {
		log.Printf("Error reading from database: %v", err)
		c.SendTo([]byte("Error reading from database"))
		return
	}

	// Send the value back to the client
	c.SendTo(value)
}

// GnetReadHandler struct with MDBX database passed in
type GnetReadHandler struct {
	db *Db // MDBX database instance
}

// NewGnetReadHandler creates a new GnetReadHandler with an MDBX database
func NewGnetReadHandler(db *Db) *GnetReadHandler {
	return &GnetReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the GnetReadHandler
func (rh *GnetReadHandler) HandleMessage(c gnet.Conn, frame []byte) {
	// Check if the frame length is at least 33 bytes (1 byte for action + 32 bytes for key)
	if len(frame) < 33 {
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(frame))
		response := []byte("Invalid message format")
		_ = c.AsyncWrite(response)
		return
	}

	key := frame[1:33] // First 32 bytes after the action as key

	// Lookup the value in the MDBX database
	value, err := rh.db.Get(key)
	if err != nil {
		log.Printf("Error reading from MDBX database: %v", err)
		response := []byte("Error reading from database")
		_ = c.AsyncWrite(response)
		return
	}

	// Send the value back to the client
	_ = c.AsyncWrite(value)
}
