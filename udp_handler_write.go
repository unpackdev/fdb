package fdb

import (
	"github.com/panjf2000/gnet"
	"log"
	"net"
)

// WriteHandler struct with MDBX database passed in
type WriteHandler struct {
	db *Db // Pass the MDBX database instance here
}

// NewWriteHandler creates a new WriteHandler with an MDBX database
func NewWriteHandler(db *Db) *WriteHandler {
	return &WriteHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the WriteHandler
func (wh *WriteHandler) HandleMessage(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	// Check if the buffer length is at least 33 bytes (1 byte for action + 32 bytes for key)
	if len(buffer) < 34 {
		log.Printf("Invalid message length: %d, expected at least 34 bytes", len(buffer))
		_, _ = conn.WriteToUDP([]byte("Invalid message format"), addr)
		return
	}

	key := buffer[1:33]  // First 32 bytes after the action as key (Ethereum hash)
	value := buffer[33:] // Remaining bytes as value

	// Write to the MDBX database
	err := wh.db.Set(key, value)
	if err != nil {
		log.Printf("Error writing to MDBX database: %v", err)
		_, _ = conn.WriteToUDP([]byte("Error writing to database"), addr)
		return
	}

	// Send success response
	response := []byte("Message written to MDBX")
	_, err = conn.WriteToUDP(response, addr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}
}

// GnetWriteHandler struct with MDBX database passed in
type GnetWriteHandler struct {
	db *Db // Pass the MDBX database instance here
}

// NewGnetWriteHandler creates a new GnetWriteHandler with an MDBX database
func NewGnetWriteHandler(db *Db) *GnetWriteHandler {
	return &GnetWriteHandler{
		db: db,
	}
}

// HandleMessage processes the incoming message using the GnetWriteHandler
func (wh *GnetWriteHandler) HandleMessage(c gnet.Conn, frame []byte) {
	// Check if the frame length is at least 34 bytes (1 byte for action + 32 bytes for key + at least 1 byte of value)
	if len(frame) < 34 {
		log.Printf("Invalid message length: %d, expected at least 34 bytes", len(frame))
		response := []byte("Invalid message format")
		_ = c.AsyncWrite(response)
		return
	}

	key := frame[1:33]  // First 32 bytes after the action as key (Ethereum hash)
	value := frame[33:] // Remaining bytes as value

	// Write to the MDBX database
	err := wh.db.Set(key, value)
	if err != nil {
		log.Printf("Error writing to MDBX database: %v", err)
		response := []byte("Error writing to database")
		_ = c.AsyncWrite(response)
		return
	}

	// Send success response
	response := []byte("Message written to MDBX")
	_ = c.AsyncWrite(response)
}
