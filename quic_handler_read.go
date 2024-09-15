package fdb

import (
	"encoding/binary"
	"github.com/quic-go/quic-go"
	"io"
	"log"
)

// QuicReadHandler struct with MDBX database passed in
type QuicReadHandler struct {
	db *Db // MDBX database instance
}

// NewQuicReadHandler creates a new QuicReadHandler with an MDBX database
func NewQuicReadHandler(db *Db) *QuicReadHandler {
	return &QuicReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the QuicReadHandler
func (rh *QuicReadHandler) HandleMessage(conn quic.Connection, stream quic.Stream) {
	// Create a buffer to read the incoming message (1 byte for action + 32 bytes for key)
	buffer := make([]byte, 33)
	_, err := io.ReadFull(stream, buffer)
	if err != nil {
		log.Printf("Error reading from QUIC stream: %v", err)
		_, _ = stream.Write([]byte("Error reading from stream"))
		return
	}

	// Check if the buffer has at least 33 bytes (1 byte for action, 32 bytes for the key)
	if len(buffer) < 33 {
		log.Printf("Invalid message length: %d, expected at least 33 bytes", len(buffer))
		_, _ = stream.Write([]byte("Invalid message format"))
		return
	}

	// Extract the key (1st byte is action, next 32 bytes are the key)
	key := buffer[1:33]
	log.Printf("Received read request for key: %x", key)

	// Query the database using the extracted key
	value, err := rh.db.Get(key)
	if err != nil {
		log.Printf("Error reading from database for key %x: %v", key, err)
		_, _ = stream.Write([]byte("Error reading from database"))
		return
	}

	// Handle empty values from the database (key not found case)
	if len(value) == 0 {
		log.Printf("No value found for key: %x", key)
		_, _ = stream.Write([]byte("No value found for key"))
		return
	}

	// Send the length of the value first (to help the client read it in full)
	valueLength := uint32(len(value))
	lengthBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuffer, valueLength)

	_, err = stream.Write(lengthBuffer)
	if err != nil {
		log.Printf("Error writing value length to QUIC stream: %v", err)
		return
	}

	// Send the actual value back to the client through the QUIC stream
	_, err = stream.Write(value)
	if err != nil {
		log.Printf("Error writing response to QUIC stream: %v", err)
		return
	}

	// Ensure the stream closes after sending the response
	log.Printf("Successfully sent response for key %x", key)
	stream.Close()
}
