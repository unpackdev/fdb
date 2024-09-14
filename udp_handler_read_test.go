package fdb

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net"
	"sync"
	"testing"
	"time"
)

// Benchmark for read operations
func BenchmarkUDPServerRead(b *testing.B) {
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

	client, err := net.DialUDP("udp", nil, server.addr)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer client.Close()

	// Prepare test data
	actionWrite := byte('W')
	actionRead := byte('R')
	key := make([]byte, 32)            // 32-byte key
	value := []byte("benchmark value") // Example value to write

	// Perform an initial write to store the value in the database
	writeMessage := append([]byte{actionWrite}, append(key, value...)...)
	_, err = client.Write(writeMessage)
	if err != nil {
		b.Fatalf("Failed to write initial data to UDP server: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Allow the server to process the write request

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Read message: 1-byte action + 32-byte key
		readMessage := append([]byte{actionRead}, key...)
		_, err := client.Write(readMessage)
		if err != nil {
			b.Errorf("Failed to write read request to UDP server: %v", err)
		}

		buffer := make([]byte, 1024)
		_, _, err = client.ReadFromUDP(buffer)
		if err != nil {
			b.Errorf("Failed to read from UDP server: %v", err)
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
