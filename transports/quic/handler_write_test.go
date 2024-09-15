package transport_quic

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

// Benchmark for write operations using QUIC server and Message struct
func BenchmarkQUICServerWrite(b *testing.B) {
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

	log.Println("STARTED QUIC SERVER FOR WRITE BENCHMARK")

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	// Create the QUIC client
	client, err := quic.DialAddr(context.Background(), server.Addr(), clientTLSConfig, nil)
	assert.NoError(b, err)
	defer client.CloseWithError(0, "closing connection")

	// Generate a random 32-byte key
	var keyBytes [32]byte
	_, err = rand.Read(keyBytes[:])
	assert.NoError(b, err)

	value := []byte("benchmark value")

	// Create the message
	message := Message{
		Handler: WriteHandlerType,
		Key:     keyBytes,
		Data:    value,
	}

	encodedMessage, err := message.Encode()
	assert.NoError(b, err)

	// Open a single stream for write operations
	stream, err := client.OpenStreamSync(context.Background())
	assert.NoError(b, err)
	defer stream.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Write the encoded message to the server
		_, err := stream.Write(encodedMessage)
		assert.NoError(b, err)

		// Optionally, read back the response if needed
		buffer := make([]byte, 1024)
		n, err := stream.Read(buffer)
		assert.NoError(b, err)
		assert.GreaterOrEqual(b, n, 1)
	}

	b.StopTimer()

	// Close the stream and server after the benchmark
	err = stream.Close()
	assert.NoError(b, err)

	server.Stop()

	log.Println("Server has been stopped.")
}
