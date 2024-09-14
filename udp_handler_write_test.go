package fdb

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
	"time"
)

// Benchmark for write operations
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
	server := startUDPServer(ctx, serverStarted, db)
	serverStarted.Wait()

	client, err := net.DialUDP("udp", nil, server.Addr()) // Use server.Addr() to get the server address
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
		}
	}

	b.StopTimer()

	// Ensure server stops
	server.Stop()
	time.Sleep(1 * time.Second) // Allow more time for server to stop

	// Verify server has stopped
	select {
	case <-time.After(2 * time.Second):
		b.Fatal("Server did not stop in time")
	default:
	}
}
