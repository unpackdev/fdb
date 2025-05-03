package transports

import (
	"log"

	"github.com/unpackdev/fdb/pkg/db"
)

// TCPWriteHandler struct with MDBX database passed in
type TCPWriteHandler struct {
	db     db.Provider     // MDBX database instance
	writer *db.BatchWriter // Batch writer instance
}

// NewTCPWriteHandler creates a new TCPWriteHandler with an MDBX database
func NewTCPWriteHandler(db db.Provider, batchWriter *db.BatchWriter) *TCPWriteHandler {
	return &TCPWriteHandler{
		db:     db,
		writer: batchWriter,
	}
}

// Handle processes the incoming message using the TCPWriteHandler
func (wh *TCPWriteHandler) Handle(conn Connection, frame []byte) {
	// Check if the message is at least 34 bytes (1 byte for action, 32 bytes for key, and at least 1 byte for value)
	if len(frame) < 34 {
		log.Printf("Invalid message length: %d, expected at least 34 bytes", len(frame))
		conn.Send([]byte{0x01}) // Error code
		return
	}

	// Create a [32]byte key from the frame without using the pool
	var key [32]byte
	copy(key[:], frame[1:33]) // Copy directly from frame

	// The remaining part is the value (from byte 33 onwards)
	value := frame[33:]

	// Buffer the write request with the key as [32]byte
	wh.writer.BufferWrite(key, value)

	// Send success response
	conn.Send([]byte{0x00}) // Success code
}
