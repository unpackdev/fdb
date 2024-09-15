package fdb

import (
	"encoding/binary"
	"github.com/quic-go/quic-go"
	"log"
)

// QuicReadHandler struct with MDBX database passed in
type QuicReadHandler struct {
	db Provider // MDBX database instance
}

// NewQuicReadHandler creates a new QuicReadHandler with an MDBX database
func NewQuicReadHandler(db Provider) *QuicReadHandler {
	return &QuicReadHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the QuicReadHandler
func (rh *QuicReadHandler) HandleMessage(conn quic.Connection, stream quic.Stream, message *Message) {
	//log.Printf("Processing read request: Handler=%d, Key=%x", message.Handler, message.Key)

	// Query the database using the key from the Message struct
	value, err := rh.db.Get(message.Key[:])
	if err != nil {
		log.Printf("Error reading from database: %v", err)
		_, _ = stream.Write([]byte("Error reading from database"))
		return
	}

	if len(value) == 0 {
		log.Printf("No value found for key: %x", message.Key)
		_, _ = stream.Write([]byte("No value found for key"))
		return
	}

	// Send response length first
	valueLength := uint32(len(value))
	lengthBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuffer, valueLength)
	_, err = stream.Write(lengthBuffer)
	if err != nil {
		log.Printf("Error writing value length: %v", err)
		return
	}

	// Send the value itself
	_, err = stream.Write(value)
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}

	//log.Printf("Successfully sent response for key: %x", message.Key)
}
