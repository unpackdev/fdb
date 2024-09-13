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

	// Table of test cases
	tests := []struct {
		name       string
		message    Message
		shouldFail bool
	}{
		{
			name: "Valid Write and Read",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{},           // 32-byte key (could be generated or hardcoded)
				Data:    []byte("test value"), // Example value to write
			},
			shouldFail: false,
		},
		{
			name: "Invalid Key Length (Too Short)",
			message: Message{
				Handler: WriteHandlerType,
				Key:     [32]byte{}, // 32-byte key but truncated in encoding
				Data:    []byte("test value"),
			},
			shouldFail: true, // Expect failure due to improper key handling
		},
		{
			name: "Invalid Handler Type",
			message: Message{
				Handler: 99,         // Invalid handler type
				Key:     [32]byte{}, // 32-byte key
				Data:    []byte("test value"),
			},
			shouldFail: true, // Expect failure due to unknown handler type
		},
		/*		{
				name: "Empty Data",
				message: Message{
					Handler: WriteHandlerType,
					Key:     [32]byte{}, // 32-byte key
					Data:    []byte(""), // No data
				},
				shouldFail: true, // Expect failure due to no data
			},*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the message for the Write operation
			encodedMessage, err := tt.message.Encode()
			if err != nil {
				t.Fatalf("Failed to encode message: %v", err)
			}

			// Write the message to the server
			startTime := time.Now()
			_, err = client.Write(encodedMessage)
			if err != nil {
				t.Errorf("Failed to write to UDP server: %v", err)
			}
			writeDuration := time.Since(startTime)
			t.Logf("Time taken for write operation: %v", writeDuration)

			// Wait for a short time to allow the server to process the request
			time.Sleep(100 * time.Millisecond)

			// Prepare buffer for reading response
			buffer := make([]byte, 1024)

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

			// Prepare a Read message using the same key
			readMessage := Message{
				Handler: ReadHandlerType,
				Key:     tt.message.Key,
			}
			encodedReadMessage, err := readMessage.Encode()
			if err != nil {
				t.Fatalf("Failed to encode read message: %v", err)
			}

			// Write the read request to the server
			startTime = time.Now()
			_, err = client.Write(encodedReadMessage)
			if err != nil {
				t.Errorf("Failed to write read request to UDP server: %v", err)
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
			assert.Equal(t, tt.message.Data, readValue, "The read value should match the written value")
		})
	}

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
