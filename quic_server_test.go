package fdb

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unpackdev/fdb/db"
	"io"
	"log"
	"testing"
	"time"
)

func startQuicServer(ctx context.Context, db db.Provider) (*QuicServer, error) {
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
	require.NoError(t, err) // Use require instead of t.Fatal
	defer db.Destroy()

	server, sErr := startQuicServer(ctx, db)
	require.NoError(t, sErr) // Use require instead of t.Fatal

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	// Create the QUIC client
	client, err := quic.DialAddr(context.Background(), server.Addr(), clientTLSConfig, nil)
	require.NoError(t, err) // Use require instead of t.Fatal
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
			require.NoError(t, err) // Use require instead of t.Fatal
			tt.message.Key = keyBytes

			// Encode the message
			encodedMessage, err := tt.message.Encode()
			require.NoError(t, err) // Use require instead of t.Fatal

			// Modify the encoded message if needed
			if tt.modifyMessage != nil {
				encodedMessage = tt.modifyMessage(encodedMessage)
			}

			// Open a single stream for both write and read operations
			stream, err := client.OpenStreamSync(context.Background())
			require.NoError(t, err) // Use require instead of t.Fatal
			defer stream.Close()

			// Send the write message to the server
			_, err = stream.Write(encodedMessage)
			require.NoError(t, err) // Use require instead of t.Error

			// Prepare buffer for reading response
			buffer := make([]byte, 1024)

			// Set a read deadline to prevent hanging
			stream.SetReadDeadline(time.Now().Add(5 * time.Second))

			// Read response after write
			n, err := stream.Read(buffer)
			require.NoError(t, err) // Use require instead of t.Error

			// Log the response received from the server after write
			response := string(buffer[:n])
			t.Logf("Response from server after write: %s", response)

			// Ensure the response is correct
			assert.Equal(t, "Message written to database", response)

			// Prepare to read the value back
			readMessage := Message{
				Handler: ReadHandlerType,
				Key:     tt.message.Key,
			}
			encodedReadMessage, err := readMessage.Encode()
			require.NoError(t, err) // Use require instead of t.Fatal

			// Send the read message to the server on the same stream
			_, err = stream.Write(encodedReadMessage)
			require.NoError(t, err) // Use require instead of t.Error

			// Set a read deadline to prevent hanging
			stream.SetReadDeadline(time.Now().Add(10 * time.Second))

			// Read response length (first 4 bytes)
			_, err = io.ReadFull(stream, buffer[:4])
			require.NoError(t, err)

			// Extract length and read value
			valueLength := binary.BigEndian.Uint32(buffer[:4])
			readBuffer := make([]byte, valueLength)
			_, err = io.ReadFull(stream, readBuffer)
			require.NoError(t, err)

			// Log the response received from the server after read
			t.Logf("Response from server after read: %s", string(readBuffer))

			// Compare the written value with the read value
			assert.Equal(t, tt.message.Data, readBuffer, "The read value should match the written value")
		})
	}
}
