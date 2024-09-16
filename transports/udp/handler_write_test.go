package transport_udp

import (
	"context"
	"crypto/rand"
	"github.com/stretchr/testify/assert"
	"log"
	"net"
	"testing"
	"time"
)

// Benchmark for write operations using gnet-based UDP server and Message struct
func BenchmarkUDPServerWrite(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the MDBX manager and database
	manager := setupBenchmarkTestManager(b, "/tmp/fdb", "benchmark")

	// Get the database from the manager
	db, err := manager.GetDb("benchmark")
	assert.NoError(b, err)
	defer db.Destroy()

	server, sErr := startUDPServer(ctx, db)
	assert.NoError(b, sErr)

	log.Println("STARTED UDP ABOUT TO RESOLVE ADDR", server.Addr().String())

	// Add a small delay before resolving the address to ensure server is ready
	time.Sleep(100 * time.Millisecond)

	// Resolve the server address
	serverAddr, err := net.ResolveUDPAddr("udp", server.Addr().String())
	if err != nil {
		b.Fatalf("Failed to resolve server address: %v", err)
	}

	// Create the UDP client
	client, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer client.Close()

	log.Println("STARTING...")

	// Generate a random 32-byte key
	var keyBytes [32]byte
	_, err = rand.Read(keyBytes[:])
	if err != nil {
		b.Fatalf("Failed to generate random key: %v", err)
	}
	key := keyBytes

	// Ensure the data is non-empty
	value := []byte("benchmark value") // Example value to write

	// Create the message
	message := Message{
		Handler: WriteHandlerType,
		Key:     key,
		Data:    value,
	}

	// Encode the message once before the loop
	encodedMessage, err := message.Encode()
	if err != nil {
		b.Fatalf("Failed to encode message: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Write the encoded message to the server
		_, err := client.Write(encodedMessage)
		if err != nil {
			b.Errorf("Failed to write to UDP server: %v", err)
			continue
		}

		/*// Read the response from the server
		buffer := make([]byte, 1024)
		rdErr := client.SetReadDeadline(time.Now().Add(1 * time.Second))
		assert.NoError(b, rdErr)

		_, _, rErr := client.ReadFromUDP(buffer)
		if rErr != nil {
			b.Errorf("Failed to read from UDP server: %v", rErr)
			continue
		}*/
		// Optionally process the response
		// response := string(buffer[:n])
		// b.Logf("Received response: %s", response)
	}

	b.StopTimer()

	// Stop the server
	server.Stop()

	// Wait for a bit to ensure the server has properly stopped
	log.Println("Waiting for the server to stop...")
	time.Sleep(200 * time.Millisecond) // Adjust timing if needed

	log.Println("Server has been stopped.")
}
