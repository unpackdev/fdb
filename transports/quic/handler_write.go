package transport_quic

import (
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/messages"
	"log"
)

// QuicWriteHandler struct with MDBX database passed in
type QuicWriteHandler struct {
	db db.Provider // Pass the MDBX database instance here
}

// NewQuicWriteHandler creates a new QuicWriteHandler with an MDBX database
func NewQuicWriteHandler(db db.Provider) *QuicWriteHandler {
	return &QuicWriteHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the QuicWriteHandler
func (wh *QuicWriteHandler) HandleMessage(conn quic.Connection, stream quic.Stream, message *messages.Message) {
	// Log the message for debugging purposes
	//log.Printf("Processing write request: Handler=%d, Key=%x, Data=%s", message.Handler, message.Key, string(message.Data))

	// Write the key and data to the database
	err := wh.db.Set(message.Key[:], message.Data)
	if err != nil {
		log.Printf("Error writing to database: %v", err)
		_, _ = stream.Write([]byte("Error writing to database"))
		return
	}

	// Send success response
	_, err = stream.Write([]byte("Message written to database"))
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}

	//log.Printf("Successfully wrote key: %x", message.Key)
}
