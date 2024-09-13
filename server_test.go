package main

import (
	"net"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	go func() {
		err := startServer()
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(500 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send test data
	message := []byte("Hello, Server")
	_, err = conn.Write(message)
	if err != nil {
		t.Fatalf("Failed to write to server: %v", err)
	}

	// Read the echoed data
	buffer := make([]byte, len(message))
	_, err = conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read from server: %v", err)
	}

	// Validate the response
	if string(buffer) != string(message) {
		t.Fatalf("Expected %s but got %s", string(message), string(buffer))
	}
}

func BenchmarkServer(b *testing.B) {
	go func() {
		err := startServer()
		if err != nil {
			b.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(500 * time.Millisecond)

	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			b.Fatalf("Failed to connect to server: %v", err)
		}

		message := []byte("Benchmark Test")
		_, err = conn.Write(message)
		if err != nil {
			b.Fatalf("Failed to write to server: %v", err)
		}

		_, err = conn.Read(make([]byte, len(message)))
		if err != nil {
			b.Fatalf("Failed to read from server: %v", err)
		}

		conn.Close()
	}
}
