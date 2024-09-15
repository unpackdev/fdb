package fdb

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

// Benchmark for read operations using QUIC server and Message struct
func BenchmarkQUICServerRead(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the MDBX manager and database
	manager := setupBenchmarkTestManager(b, "/tmp/fdb", "benchmark_quic")

	// Get the database from the manager
	db, err := manager.GetDb("benchmark_quic")
	assert.NoError(b, err)
	defer db.Destroy()

	server, sErr := startQuicServer(ctx, db)
	assert.NoError(b, sErr)

	log.Println("STARTED QUIC SERVER FOR READ BENCHMARK")

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	// Create the QUIC client
	client, err := quic.DialAddr(context.Background(), server.Addr(), clientTLSConfig, nil)
	assert.NoError(b, err)
	defer client.CloseWithError(0, "closing connection")

	// Generate a random 32-byte key and write the value to the database first
	var keyBytes [32]byte
	_, err = rand.Read(keyBytes[:])
	assert.NoError(b, err)

	// Value that will be written first so it can be read
	value := []byte("benchmark read value")

	// Create the write message to store the value in the database first
	writeMessage := Message{
		Handler: WriteHandlerType,
		Key:     keyBytes,
		Data:    value,
	}
	encodedWriteMessage, err := writeMessage.Encode()
	assert.NoError(b, err)

	// Write the value to the server first
	stream, err := client.OpenStreamSync(context.Background())
	assert.NoError(b, err)
	defer stream.Close()

	_, err = stream.Write(encodedWriteMessage)
	assert.NoError(b, err)

	// Read and ensure the server wrote the data correctly
	buffer := make([]byte, 1024)
	_, err = stream.Read(buffer)
	assert.NoError(b, err)

	// Prepare the read message for reading the value back
	readMessage := Message{
		Handler: ReadHandlerType,
		Key:     keyBytes,
	}
	encodedReadMessage, err := readMessage.Encode()
	assert.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Send the read message to the server
		_, err := stream.Write(encodedReadMessage)
		assert.NoError(b, err)

		// Read response length (first 4 bytes)
		_, err = stream.Read(buffer[:4])
		assert.NoError(b, err)

		// Extract length and read the actual value
		valueLength := binary.BigEndian.Uint32(buffer[:4])
		readBuffer := make([]byte, valueLength)
		_, err = stream.Read(readBuffer)
		assert.NoError(b, err)

		// Optionally, verify the received value
		assert.Equal(b, value, readBuffer)
	}

	b.StopTimer()

	// Close the stream and server after the benchmark
	err = stream.Close()
	assert.NoError(b, err)

	server.Stop()

	log.Println("Server has been stopped.")
}
