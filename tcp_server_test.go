package fdb

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTCPServer(t *testing.T) {
	// Create the server
	server := NewTCPServer("tcp://127.0.0.1:8781")

	// Register handlers
	server.RegisterHandler(WriteHandlerType, WriteHandler)
	server.RegisterHandler(ReadHandlerType, ReadHandler)

	// Start the server in a separate goroutine
	go func() {
		err := server.Start()
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait a bit for the server to start
	time.Sleep(200 * time.Millisecond)

	// Connect to the server
	conn, err := net.Dial("tcp", "127.0.0.1:8781")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Test cases
	tests := []struct {
		name             string
		actionType       HandlerType
		payload          []byte
		expectedResponse []byte
	}{
		{
			name:             "Test WriteHandler",
			actionType:       WriteHandlerType,
			payload:          []byte("Test data for write"),
			expectedResponse: []byte("WRITE HANDLED"),
		},
		{
			name:             "Test ReadHandler",
			actionType:       ReadHandlerType,
			payload:          []byte("Test data for read"),
			expectedResponse: []byte("READ HANDLED"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare the message
			message := append([]byte{byte(tt.actionType)}, tt.payload...)

			// Encode the length prefix
			length := uint32(len(message))
			lengthBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(lengthBuf, length)

			// Send the length-prefixed message
			_, err = conn.Write(lengthBuf)
			if err != nil {
				t.Fatalf("Failed to send length prefix: %v", err)
			}
			_, err = conn.Write(message)
			if err != nil {
				t.Fatalf("Failed to send message: %v", err)
			}

			// Read the response
			response := make([]byte, 1024)
			n, err := conn.Read(response)
			if err != nil {
				t.Fatalf("Failed to read response: %v", err)
			}

			// Verify the response
			assert.Equal(t, tt.expectedResponse, response[:n], "Unexpected response from server")
		})
	}

	// Optionally, wait a bit before ending the test
	time.Sleep(100 * time.Millisecond)
}
