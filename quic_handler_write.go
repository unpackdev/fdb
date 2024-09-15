package fdb

import (
	"github.com/quic-go/quic-go"
	"io"
	"log"
)

// QuicWriteHandler struct with MDBX database passed in
type QuicWriteHandler struct {
	db *Db // Pass the MDBX database instance here
}

// NewQuicWriteHandler creates a new QuicWriteHandler with an MDBX database
func NewQuicWriteHandler(db *Db) *QuicWriteHandler {
	return &QuicWriteHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the QuicWriteHandler
func (wh *QuicWriteHandler) HandleMessage(conn quic.Connection, stream quic.Stream) {
	// Create a buffer to read the incoming message
	buffer := make([]byte, 34) // 1 byte for action + 32 bytes for key + at least 1 byte for value
	n, err := io.ReadFull(stream, buffer)
	if err != nil {
		log.Printf("Error reading from QUIC stream: %v", err)
		_, _ = stream.Write([]byte("Error reading from stream"))
		return
	}

	if n < 34 {
		log.Printf("Invalid message length: %d, expected at least 34 bytes", n)
		_, _ = stream.Write([]byte("Invalid message format"))
		return
	}

	// Extract key and value
	key := buffer[1:33]
	value := buffer[33:]

	// Write the key-value pair to the database
	err = wh.db.Set(key, value)
	if err != nil {
		log.Printf("Error writing to database: %v", err)
		_, _ = stream.Write([]byte("Error writing to database"))
		return
	}

	// Send success response
	_, err = stream.Write([]byte("Message written to database"))
	if err != nil {
		log.Printf("Error sending response to QUIC stream: %v", err)
	}
}
