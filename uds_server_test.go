package fdb

import (
	"context"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

// Start UDS server function with handlers for write and read
func startUDSServer(ctx context.Context, db *Db) (*UDSServer, error) {
	socketPath := "/tmp/fdb_test.sock"
	log.Printf("Starting UDS server on socket: %s", socketPath)

	server, err := NewUDSServer(socketPath)
	if err != nil {
		return nil, err
	}

	wHandler := NewUDSWriteHandler(db)
	server.RegisterHandler(WriteHandlerType, wHandler.HandleMessage)

	rHandler := NewUDSReadHandler(db)
	server.RegisterHandler(ReadHandlerType, rHandler.HandleMessage)

	go func() {
		log.Println("Starting gnet server...")
		err := server.Start()
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
		log.Println("gnet server started.")
	}()

	log.Println("Awaiting for started closure...")
	<-server.WaitStarted() // Wait for the server to signal that it has started
	log.Println("UDS server started.")

	return server, nil
}

// TestUDSServer tests the gnet-based UDS server with various scenarios
func TestUDSServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Remove the test UDS socket file after the test completes
	socketPath := "/tmp/fdb_test.sock"
	defer os.Remove(socketPath)

	// Setup the MDBX manager and database
	manager := setupTestManager(t)

	// Get the database from the manager
	db, err := manager.GetDb("test")
	assert.NoError(t, err)
	defer db.Destroy()

	server, sErr := startUDSServer(ctx, db)
	assert.NoError(t, sErr)

	// Connect to the UDS server as a client
	conn, err := net.Dial("unix", server.Addr())
	if err != nil {
		t.Fatalf("Failed to create UDS client: %v", err)
	}
	defer conn.Close()

	// Table of test cases
	tests := []struct {
		name          string
		message       Message
		shouldFail    bool
		modifyMessage func([]byte) []byte // Function to modify the encoded message for testing
	}{
		{
			name: "Valid Write and Read",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{}, // Will be set to random key
				Data:    []byte("test value"),
			},
			shouldFail:    false,
			modifyMessage: nil,
		},
		{
			name: "Invalid Key Length (Too Short)",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{}, // Will be set to random key
				Data:    []byte("test value"),
			},
			shouldFail: true,
			modifyMessage: func(encodedMsg []byte) []byte {
				// Truncate the key to 31 bytes to simulate an invalid key length
				return append(encodedMsg[:1], encodedMsg[1:32]...) // Handler + 31-byte key
			},
		},
		{
			name: "Invalid Handler Type",
			message: Message{
				Handler: 99,         // Invalid handler type
				Key:     [32]byte{}, // Will be set to random key
				Data:    []byte("test value"),
			},
			shouldFail:    true,
			modifyMessage: nil,
		},
		{
			name: "Empty Data",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{}, // Will be set to random key
				Data:    []byte(""), // No data
			},
			shouldFail:    true, // Adjusted to reflect the protocol requirement
			modifyMessage: nil,
		},
	}

	for _, tt := range tests {
		tt := tt // capture tt for the closure
		t.Run(tt.name, func(t *testing.T) {
			// Generate a random 32-byte key
			var keyBytes [32]byte
			_, err := rand.Read(keyBytes[:])
			if err != nil {
				t.Fatalf("Failed to generate random key: %v", err)
			}
			tt.message.Key = keyBytes

			// Encode the message
			encodedMessage, err := tt.message.Encode()
			if err != nil {
				t.Fatalf("Failed to encode message: %v", err)
			}

			// Modify the encoded message if needed
			if tt.modifyMessage != nil {
				encodedMessage = tt.modifyMessage(encodedMessage)
			}

			// Send the message to the server
			_, err = conn.Write(encodedMessage)
			if err != nil {
				t.Errorf("Failed to write to UDS server: %v", err)
				return
			}

			// Prepare buffer for reading response
			buffer := make([]byte, 1024)

			// Set read deadline to prevent blocking indefinitely
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			// Read response after write
			n, err := conn.Read(buffer)
			if err != nil {
				if tt.shouldFail {
					// Expected failure due to read timeout or error
					t.Logf("Expected failure occurred: %v", err)
					return
				}
				t.Errorf("Failed to read from UDS server: %v", err)
				return
			}

			// Log the response received from the server after write
			response := string(buffer[:n])
			t.Logf("Response from server after write: %s", response)

			if tt.shouldFail {
				// If we expected a failure but received a response, check if it's an error message
				if response == "Message written to database" {
					t.Errorf("Expected failure, but write succeeded")
				} else {
					t.Logf("Received expected error response: %s", response)
				}
				return
			}

			// For valid write operations, proceed to read the value back
			// Prepare a Read message using the same key
			readMessage := Message{
				Handler: ReadHandlerType,
				Key:     tt.message.Key,
			}
			encodedReadMessage, err := readMessage.Encode()
			if err != nil {
				t.Fatalf("Failed to encode read message: %v", err)
			}

			// Send the read request to the server
			_, err = conn.Write(encodedReadMessage)
			if err != nil {
				t.Errorf("Failed to write read request to UDS server: %v", err)
				return
			}

			// Read response after read
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, err = conn.Read(buffer)
			if err != nil {
				t.Errorf("Failed to read from UDS server: %v", err)
				return
			}

			// Log the response received from the server after read
			readValue := buffer[:n]
			t.Logf("Response from server after read: %s", string(readValue))

			// Compare the written value with the read value
			assert.Equal(t, tt.message.Data, readValue, "The read value should match the written value")
		})
	}
}
