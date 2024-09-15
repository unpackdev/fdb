package fdb

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
	"time"
)

// Start QUIC server with handlers for write and read
func startQuicServer(ctx context.Context, db *Db) (*QuicServer, error) {
	port, err := getRandomPort()
	if err != nil {
		return nil, err
	}
	log.Printf("Starting QUIC server on port: %d", port)

	tlsConfig := GenerateTLSConfig()

	server, err := NewQuicServer("127.0.0.1", port, tlsConfig)
	if err != nil {
		return nil, err
	}

	wHandler := NewQuicWriteHandler(db)
	server.RegisterHandler(WriteHandlerType, wHandler.HandleMessage)

	rHandler := NewQuicReadHandler(db)
	server.RegisterHandler(ReadHandlerType, rHandler.HandleMessage)

	go func() {
		log.Println("Starting QUIC server...")
		err := server.Start()
		if err != nil {
			log.Fatalf("Failed to start QUIC server: %v", err)
		}
		log.Println("QUIC server started.")
	}()

	log.Println("Awaiting for started closure...")
	<-server.WaitStarted()
	log.Println("Started closure detected for port:", port)

	return server, nil
}

// TestQUICServer tests the QUIC server with various scenarios
func TestQUICServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the MDBX manager and database
	manager := setupTestManager(t)

	// Get the database from the manager
	db, err := manager.GetDb("test")
	assert.NoError(t, err)
	defer db.Destroy()

	server, sErr := startQuicServer(ctx, db)
	assert.NoError(t, sErr)

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	// Create the QUIC client
	client, err := quic.DialAddr(context.Background(), server.Addr(), clientTLSConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create QUIC client: %v", err)
	}
	defer client.CloseWithError(0, "closing connection")

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

			// Open a new stream to the QUIC server
			stream, err := client.OpenStreamSync(context.Background())
			if err != nil {
				t.Fatalf("Failed to open stream: %v", err)
			}
			defer stream.Close()

			// Send the message to the server
			_, err = stream.Write(encodedMessage)
			if err != nil {
				t.Errorf("Failed to write to QUIC server: %v", err)
				return
			}

			// Set a read deadline to prevent hanging
			stream.SetReadDeadline(time.Now().Add(5 * time.Second)) // Increased to 5 seconds

			// Prepare buffer for reading response
			buffer := make([]byte, 1024)

			// Read response after write
			n, err := stream.Read(buffer)
			if err != nil {
				if tt.shouldFail {
					t.Logf("Expected failure occurred: %v", err)
					return
				}
				t.Errorf("Failed to read from QUIC server: %v", err)
				return
			}

			// Log the response received from the server after write
			response := string(buffer[:n])
			t.Logf("Response from server after write: %s", response)

			// Ensure stream is closed properly
			defer stream.Close()

			// Validate the response
			if tt.shouldFail {
				if response == "Message written to database" {
					t.Errorf("Expected failure, but write succeeded")
				} else {
					t.Logf("Received expected error response: %s", response)
				}
				return
			}

			// For valid write operations, proceed to read the value back
			readMessage := Message{
				Handler: ReadHandlerType,
				Key:     tt.message.Key,
			}
			encodedReadMessage, err := readMessage.Encode()
			if err != nil {
				t.Fatalf("Failed to encode read message: %v", err)
			}

			// Open a new stream for the read operation
			stream, err = client.OpenStreamSync(context.Background())
			if err != nil {
				t.Fatalf("Failed to open stream: %v", err)
			}
			defer stream.Close()

			_, err = stream.Write(encodedReadMessage)
			if err != nil {
				t.Errorf("Failed to write read request to QUIC server: %v", err)
				return
			}

			// Set a read deadline to prevent hanging
			stream.SetReadDeadline(time.Now().Add(2 * time.Second))

			// Read response after read
			n, err = stream.Read(buffer)
			if err != nil {
				t.Errorf("Failed to read from QUIC server: %v", err)
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
