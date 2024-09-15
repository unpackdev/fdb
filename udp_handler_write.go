package fdb

import (
	"github.com/panjf2000/gnet"
	"log"
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
func (wh *WriteHandler) HandleMessage(c gnet.Conn, frame []byte) {

	// Check frame length
	if len(frame) < 34 { // 1 byte action + 32-byte key + at least 1 byte value
		log.Printf("Invalid message length: %d, expected at least 34 bytes", len(frame))
		c.SendTo([]byte("Invalid message format"))
		return
	}

	// Write to the database
	err := wh.db.Set(frame[1:33], frame[33:])
	if err != nil {
		log.Printf("Error writing to database: %v", err)
		c.SendTo([]byte("Error writing to database"))
		return
	}

	// Send success response
	c.SendTo([]byte("Message written to database"))
}
