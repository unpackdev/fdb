package fdb

import (
	"context"
	"github.com/stretchr/testify/assert"
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
func startUDPServer(ctx context.Context, serverStarted *sync.WaitGroup, db *Db) *UdpServer {
	server, err := New(8781, "127.0.0.1")
	if err != nil {
		panic(err)
	}

	// Register handlers with actual WriteHandler and ReadHandler
	wHandler := NewWriteHandler(db)
	server.RegisterHandler(WriteHandlerType, wHandler.HandleMessage)

	rHandler := NewReadHandler(db)
	server.RegisterHandler(ReadHandlerType, rHandler.HandleMessage)

	go func() {
		server.Start()
	}()

	serverStarted.Done()
	return server
}

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
	server := startUDPServer(ctx, serverStarted, db)
	serverStarted.Wait()

	// Create UDP client
	client, err := net.DialUDP("udp", nil, server.Addr()) // Use server.Addr() to get the server address
	if err != nil {
		t.Fatalf("Failed to create UDP client: %v", err)
	}
	defer client.Close()

	// Prepare buffer for reading response
	buffer := make([]byte, 1024)

	// Prepare test data
	actionWrite := byte('W')      // Action byte for write
	actionRead := byte('R')       // Action byte for read
	key := make([]byte, 32)       // 32-byte key (could be generated or hardcoded)
	value := []byte("test value") // Example value to write

	// Test Write operation
	writeMessage := append([]byte{actionWrite}, append(key, value...)...)
	startTime := time.Now()
	_, err = client.Write(writeMessage)
	if err != nil {
		t.Errorf("Failed to write to UDP server: %v", err)
	}
	writeDuration := time.Since(startTime)
	t.Logf("Time taken for write operation: %v", writeDuration)

	// Wait for a short time to allow the server to process the request
	time.Sleep(100 * time.Millisecond)

	// Read response after write
	startTime = time.Now()
	n, _, err := client.ReadFromUDP(buffer)
	if err != nil {
		t.Errorf("Failed to read from UDP server: %v", err)
	}
	readDuration := time.Since(startTime)
	t.Logf("Time taken for read operation after write: %v", readDuration)

	// Log the response received from the server after write
	t.Logf("Response from server after write: %s", string(buffer[:n]))

	// Test Read operation (using the same key)
	readMessage := append([]byte{actionRead}, key...)
	startTime = time.Now()
	_, err = client.Write(readMessage)
	if err != nil {
		t.Errorf("Failed to write to UDP server (read request): %v", err)
	}
	writeDuration = time.Since(startTime)
	t.Logf("Time taken for write operation (read request): %v", writeDuration)

	// Wait for the server to process the request
	time.Sleep(100 * time.Millisecond)

	// Read response after read
	startTime = time.Now()
	n, _, err = client.ReadFromUDP(buffer)
	if err != nil {
		t.Errorf("Failed to read from UDP server: %v", err)
	}
	readDuration = time.Since(startTime)
	t.Logf("Time taken for read operation: %v", readDuration)

	// Log the response received from the server after read
	readValue := buffer[:n]
	t.Logf("Response from server after read: %s", string(readValue))

	// Compare the written value with the read value
	assert.Equal(t, value, readValue, "The read value should match the written value")

	// Stop the server
	server.Stop()

	// Ensure server has stopped
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop in time")
	default:
	}
}

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

	client, err := net.DialUDP("udp", nil, server.addr)
	if err != nil {
		b.Fatalf("Failed to create UDP client: %v", err)
	}
	defer client.Close()

	// Prepare test data
	actionWrite := byte('W')
	key := make([]byte, 32)            // 32-byte key (can be generated or hardcoded)
	value := []byte("benchmark value") // Example value to write

	time.Sleep(100 * time.Millisecond) // Allow server to fully start

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Write message: 1-byte action + 32-byte key + value
		writeMessage := append([]byte{actionWrite}, append(key, value...)...)
		_, err := client.Write(writeMessage)
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
