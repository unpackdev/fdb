package fdb

import (
	"github.com/panjf2000/gnet"
	"github.com/unpackdev/fdb/db"
	"log"
)

// UDSReadHandler struct with MDBX database passed in
type UDSReadHandler struct {
	db db.Provider // MDBX database instance
}

// NewUDSReadHandler creates a new UDSReadHandler with an MDBX database
func NewUDSReadHandler(db db.Provider) *UDSReadHandler {
	return &UDSReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the UDSReadHandler
func (rh *UDSReadHandler) HandleMessage(c gnet.Conn, frame []byte) {
	if len(frame) < 33 { // 1 byte action + 32-byte key
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(frame))
		c.SendTo([]byte("Invalid message format"))
		return
	}

	// Extract the key (32 bytes starting from the second byte)
	key := frame[1:33]

	// Read from the database using the key
	value, err := rh.db.Get(key)
	if err != nil {
		log.Printf("Error reading from database: %v", err)
		c.SendTo([]byte("Error reading from database"))
		return
	}

	if len(value) == 0 {
		log.Printf("No value found for key: %x", key)
		c.SendTo([]byte("No value found for key"))
		return
	}

	// Send the value back to the client
	c.SendTo(value)

	log.Printf("Successfully sent response for key: %x", key)
}
