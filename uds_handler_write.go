package fdb

import (
	"github.com/panjf2000/gnet"
	"log"
)

// UDSWriteHandler struct with MDBX database passed in
type UDSWriteHandler struct {
	db Provider // MDBX database instance
}

// NewUDSWriteHandler creates a new UDSWriteHandler with an MDBX database
func NewUDSWriteHandler(db Provider) *UDSWriteHandler {
	return &UDSWriteHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the UDSWriteHandler
func (wh *UDSWriteHandler) HandleMessage(c gnet.Conn, frame []byte) {

	// Check if the message is at least 34 bytes (1 byte for action, 32 bytes for key, and at least 1 byte for value)
	if len(frame) < 34 {
		log.Printf("Invalid message length: %d, expected at least 34 bytes", len(frame))
		c.SendTo([]byte("Invalid message format"))
		return
	}

	// Extract the 32-byte key from the frame (bytes 1 to 33)
	key := frame[1:33]
	// The remaining part is the value (from byte 33 onwards)
	value := frame[33:]

	// Write to the database
	err := wh.db.Set(key, value)
	if err != nil {
		log.Printf("Error writing to database: %v", err)
		c.SendTo([]byte("Error writing to database"))
		return
	}

	// Send success response
	c.SendTo([]byte("Message written to database"))

	log.Printf("Successfully wrote key: %x", key)
}
