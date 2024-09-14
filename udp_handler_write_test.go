package fdb

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
	"time"
)

// Benchmark for write operations using gnet-based UDP server and Message struct
func BenchmarkUDPServerWrite(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the MDBX manager and database
	manager := setupBenchmarkTestManager(b)

	// Get the database from the manager
	db, err := manager.GetDb("test")
	assert.NoError(b, err)
	defer db.Destroy()

	serverStarted := &sync.WaitGroup{}
	serverStarted.Add(1)
	server, sErr := startUDPServer(ctx, serverStarted, db)
	assert.NoError(b, sErr)
	serverStarted.Wait()

	// Wait for the server to start
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

	// Prepare test data
	key := [32]byte{}                  // 32-byte key (can be generated or hardcoded)
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

	time.Sleep(100 * time.Millisecond) // Allow server to fully start

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Write the encoded message to the server
		_, err := client.Write(encodedMessage)
		if err != nil {
			b.Errorf("Failed to write to UDP server: %v", err)
			continue
		}

		// Optionally read the response from the server (if any)
		// buffer := make([]byte, 1024)
		// n, err := client.Read(buffer)
		// if err != nil {
		//     b.Errorf("Failed to read from UDP server: %v", err)
		//     continue
		// }
		// Use response if needed
	}

	b.StopTimer()

	// Stop the server
	server.Stop()
	time.Sleep(100 * time.Millisecond) // Allow some time for the server to stop
}
