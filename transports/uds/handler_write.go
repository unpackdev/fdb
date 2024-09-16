package transport_uds

import (
	"fmt"
	"github.com/panjf2000/gnet"
	"github.com/unpackdev/fdb/db"
	"log"
)

// UDSWriteHandler struct with MDBX database and BatchWriter passed in
type UDSWriteHandler struct {
	db     db.Provider     // MDBX database instance
	writer *db.BatchWriter // Batch writer instance
}

// NewUDSWriteHandler creates a new UDSWriteHandler with an MDBX database and BatchWriter
func NewUDSWriteHandler(db db.Provider, batchWriter *db.BatchWriter) *UDSWriteHandler {
	return &UDSWriteHandler{
		db:     db,
		writer: batchWriter,
	}
}

// HandleMessage processes the incoming message using the UDSWriteHandler
func (wh *UDSWriteHandler) HandleMessage(c gnet.Conn, frame []byte) {
	// Check if the message is at least 34 bytes (1 byte for action, 32 bytes for key, and at least 1 byte for value)
	if len(frame) < 34 {
		log.Printf("Invalid message length: %d, expected at least 34 bytes", len(frame))
		c.SendTo([]byte{0x01})
		return
	}

	// Create a [32]byte key from the frame
	var key [32]byte
	copy(key[:], frame[1:33]) // Copy directly from frame

	// The remaining part is the value (from byte 33 onwards)
	value := frame[33:]

	// Buffer the write request with the key as [32]byte
	wh.writer.BufferWrite(key, value)

	fmt.Println("WRITTEN TO BUFFER")
	// Send success response
	c.SendTo([]byte{0x00})

	fmt.Println("SENT BACK 0x00 STATUS CODE")
}
