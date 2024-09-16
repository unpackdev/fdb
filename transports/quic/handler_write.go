package transport_quic

import (
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/messages"
	"log"
)

// QuicWriteHandler struct with MDBX database passed in
type QuicWriteHandler struct {
	db     db.Provider     // MDBX database instance
	writer *db.BatchWriter // Batch writer instance
}

// NewQuicWriteHandler creates a new QuicWriteHandler with an MDBX database
func NewQuicWriteHandler(db db.Provider, batchWriter *db.BatchWriter) *QuicWriteHandler {
	return &QuicWriteHandler{
		db:     db,
		writer: batchWriter,
	}
}

// HandleMessage processes the incoming message using the QuicWriteHandler
func (wh *QuicWriteHandler) HandleMessage(conn quic.Connection, stream quic.Stream, message *messages.Message) {
	// Log the message for debugging purposes
	//log.Printf("Processing write request: Handler=%d, Key=%x, Data=%s", message.Handler, message.Key, string(message.Data))

	// Buffer the write request with the key as [32]byte
	wh.writer.BufferWrite(message.Key, message.Data)

	// Send success response

	if _, err := stream.Write([]byte{0x00}); err != nil {
		log.Printf("Error sending response: %v", err)
	}

	//log.Printf("Successfully wrote key: %x", message.Key)
}
