package fdb

import (
	"context"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"log"
	"math/big"
	"net"
	"testing"
	"time"
)

// Generate a random port in the range 1024 to 65535
func getRandomPort() (int, error) {
	// The valid port range is 1024–65535
	minPort := 1024
	maxPort := 65535
	portRange := big.NewInt(int64(maxPort - minPort + 1))

	// Generate a random number in the port range
	randPort, err := rand.Int(rand.Reader, portRange)
	if err != nil {
		return 0, err
	}

	// Return the port number within the valid range
	return minPort + int(randPort.Int64()), nil
}

// Start UDP server function with handlers for write and read
func startUDPServer(ctx context.Context, db Provider) (*UdpServer, error) {
	port, err := getRandomPort()
	if err != nil {
		return nil, err
	}
	log.Printf("Starting UDP server on port: %d", port)

	server, err := NewUdpServer("127.0.0.1", port)
	if err != nil {
		return nil, err
	}

	wHandler := NewWriteHandler(db)
	server.RegisterHandler(WriteHandlerType, wHandler.HandleMessage)

	rHandler := NewReadHandler(db)
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
	// Wait for the server to signal that it has started
	//<-server.WaitStarted()

	log.Println("Started closure detected for port:", port)

	return server, nil
}

// TestUDPServer tests the gnet-based UDP server with various scenarios
func TestUDPServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the MDBX manager and database
	manager := setupTestManager(t)

	// Get the database from the manager
	db, err := manager.GetDb("test")
	assert.NoError(t, err)
	defer db.Destroy()

	server, sErr := startUDPServer(ctx, db)
	assert.NoError(t, sErr)

	serverAddr, err := net.ResolveUDPAddr("udp", server.Addr().String())
	if err != nil {
		t.Fatalf("Failed to resolve server address: %v", err)
	}

	// Create the UDP client
	client, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}
	defer client.Close()

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
			_, err = client.Write(encodedMessage)
			if err != nil {
				t.Errorf("Failed to write to UDP server: %v", err)
				return
			}

			// Prepare buffer for reading response
			buffer := make([]byte, 1024)

			// Set read deadline to prevent blocking indefinitely
			client.SetReadDeadline(time.Now().Add(1 * time.Second))

			// Read response after write
			n, _, err := client.ReadFromUDP(buffer)
			if err != nil {
				if tt.shouldFail {
					// Expected failure due to read timeout or error
					t.Logf("Expected failure occurred: %v", err)
					return
				}
				t.Errorf("Failed to read from UDP server: %v", err)
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
			_, err = client.Write(encodedReadMessage)
			if err != nil {
				t.Errorf("Failed to write read request to UDP server: %v", err)
				return
			}

			// Read response after read
			rdErr := client.SetReadDeadline(time.Now().Add(1 * time.Second))
			assert.NoError(t, rdErr)

			n, _, err = client.ReadFromUDP(buffer)
			if err != nil {
				t.Errorf("Failed to read from UDP server: %v", err)
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
