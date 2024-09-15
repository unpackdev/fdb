package fdb

import (
	"github.com/panjf2000/gnet"
	"github.com/unpackdev/fdb/db"
	"log"
)

// ReadHandler struct with MDBX database passed in
type ReadHandler struct {
	db db.Provider // MDBX database instance
}

// NewReadHandler creates a new ReadHandler with an MDBX database
func NewReadHandler(db db.Provider) *ReadHandler {
	return &ReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the ReadHandler
func (rh *ReadHandler) HandleMessage(c gnet.Conn, frame []byte) {
	if len(frame) < 33 { // 1 byte action + 32-byte key
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(frame))
		c.SendTo([]byte("Invalid message format"))
		return
	}

	// Read from the database
	value, err := rh.db.Get(frame[1:33])
	if err != nil {
		log.Printf("Error reading from database: %v", err)
		c.SendTo([]byte("Error reading from database"))
		return
	}

	// Send the value back to the client
	c.SendTo(value)
}
