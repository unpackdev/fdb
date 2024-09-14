package fdb

import (
	"context"
	"github.com/stretchr/testify/assert"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

// Define handler functions
func writeHandler(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	_, err := conn.WriteToUDP([]byte("WRITE HANDLED"), addr)
	if err != nil {
		// Handle error appropriately
	}
}

func readHandler(conn *net.UDPConn, buffer []byte, addr *net.UDPAddr) {
	_, err := conn.WriteToUDP([]byte("READ HANDLED"), addr)
	if err != nil {
		// Handle error appropriately
	}
}

// Start UDP server function with handlers for write and read
func startUDPServer(ctx context.Context, serverStarted *sync.WaitGroup, db *Db) (*UdpServer, error) {
	server, err := NewUdpServer("127.0.0.1", 8781)
	if err != nil {
		return nil, err
	}

	// Register handlers with actual WriteHandler and ReadHandler
	wHandler := NewWriteHandler(db)
	server.RegisterHandler(WriteHandlerType, wHandler.HandleMessage)

	rHandler := NewReadHandler(db)
	server.RegisterHandler(ReadHandlerType, rHandler.HandleMessage)

	go func() {
		err := server.Start()
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	serverStarted.Done()
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

	serverStarted := &sync.WaitGroup{}
	serverStarted.Add(1)
	server, sErr := startUDPServer(ctx, serverStarted, db)
	assert.NoError(t, sErr)
	serverStarted.Wait()

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
				Key:     [32]byte{},
				Data:    []byte("test value"),
			},
			shouldFail:    false,
			modifyMessage: nil,
		},
		{
			name: "Invalid Key Length (Too Short)",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{},
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
				Handler: 99, // Invalid handler type
				Key:     [32]byte{},
				Data:    []byte("test value"),
			},
			shouldFail:    true,
			modifyMessage: nil,
		},
		{
			name: "Empty Data",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{},
				Data:    []byte(""), // No data
			},
			shouldFail:    false, // Depending on your application's logic, adjust this
			modifyMessage: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				if response == "Message written to MDBX" {
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
			client.SetReadDeadline(time.Now().Add(1 * time.Second))
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
