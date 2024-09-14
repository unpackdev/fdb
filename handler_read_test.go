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

// Benchmark for read operations using gnet-based UDP server and Message struct
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
	key := [32]byte{}                  // 32-byte key
	value := []byte("benchmark value") // Example value to write

	// Perform an initial write to store the value in the database
	writeMessage := Message{
		Handler: WriteHandlerType,
		Key:     key,
		Data:    value,
	}

	encodedWriteMessage, err := writeMessage.Encode()
	if err != nil {
		b.Fatalf("Failed to encode write message: %v", err)
	}

	_, err = client.Write(encodedWriteMessage)
	if err != nil {
		b.Fatalf("Failed to write initial data to UDP server: %v", err)
	}

	// Read the response to the write operation (if any)
	buffer := make([]byte, 1024)
	n, err := client.Read(buffer)
	if err != nil {
		b.Fatalf("Failed to read response from UDP server: %v", err)
	}
	// Optionally check the response
	log.Printf("Write response: %s", string(buffer[:n]))

	time.Sleep(100 * time.Millisecond) // Allow the server to process the write request

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Prepare the read message
		readMessage := Message{
			Handler: ReadHandlerType,
			Key:     key,
			Data:    nil, // No data for read
		}

		encodedReadMessage, err := readMessage.Encode()
		if err != nil {
			b.Fatalf("Failed to encode read message: %v", err)
		}

		_, err = client.Write(encodedReadMessage)
		if err != nil {
			b.Errorf("Failed to write read request to UDP server: %v", err)
			continue
		}

		buffer := make([]byte, 1024)
		n, err = client.Read(buffer)
		if err != nil {
			b.Errorf("Failed to read from UDP server: %v", err)
			continue
		}

		// Optionally check the response
		// log.Printf("Read response: %s", string(buffer[:n]))
	}

	b.StopTimer()

	// Stop the server
	server.Stop()
	time.Sleep(100 * time.Millisecond) // Allow some time for the server to stop
}
