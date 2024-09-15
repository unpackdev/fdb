package benchmark

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb"
	transport_quic "github.com/unpackdev/fdb/transports/quic"
	"github.com/unpackdev/fdb/types"
	"io"
	"sync"
)

// Buffer pool for reusing buffers
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 1024)
	},
}

// QuicSuite represents the QUIC-specific benchmark suite.
type QuicSuite struct {
	fdbInstance *fdb.FDB
	quicServer  *transport_quic.Server
	client      quic.Connection
	stream      quic.Stream
}

// NewQuicSuite creates a new QuicSuite for benchmarking.
func NewQuicSuite(fdbInstance *fdb.FDB) *QuicSuite {
	return &QuicSuite{
		fdbInstance: fdbInstance,
	}
}

// Start starts the QUIC server for benchmarking.
func (qs *QuicSuite) Start() error {
	quicTransport, err := qs.fdbInstance.GetTransportByType(types.QUICTransportType)
	if err != nil {
		return fmt.Errorf("failed to retrieve QUIC transport: %w", err)
	}

	quicServer, ok := quicTransport.(*transport_quic.Server)
	if !ok {
		return fmt.Errorf("failed to cast transport to QuicServer")
	}

	db, err := qs.fdbInstance.GetDbManager().GetDb("benchmark")
	if err != nil {
		return fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	wHandler := transport_quic.NewQuicWriteHandler(db)
	quicServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	rHandler := transport_quic.NewQuicReadHandler(db)
	quicServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	if err := quicServer.Start(); err != nil {
		return fmt.Errorf("failed to start QUIC server: %w", err)
	}

	qs.quicServer = quicServer
	fmt.Println("QUIC server started successfully")
	return nil
}

// Stop stops the QUIC server and closes the client connection and stream.
func (qs *QuicSuite) Stop() {
	if qs.stream != nil {
		qs.stream.Close()
	}
	if qs.client != nil {
		qs.client.CloseWithError(0, "closing connection")
	}
	if qs.quicServer != nil {
		qs.quicServer.Stop()
		fmt.Println("QUIC server stopped successfully")
	}
}

// SetupClient sets up a QUIC client and stream only once.
func (qs *QuicSuite) SetupClient(ctx context.Context) error {
	if qs.client != nil && qs.stream != nil {
		return nil // Already setup, reuse client and stream
	}

	serverAddr := qs.quicServer.Addr()

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	// Connect to the server
	client, err := quic.DialAddr(ctx, serverAddr, clientTLSConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC server: %w", err)
	}
	qs.client = client

	// Open a stream to send messages
	stream, err := client.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	qs.stream = stream

	return nil
}

// Run sends a single message through a QUIC stream sequentially.
func (qs *QuicSuite) Run(ctx context.Context) error {
	// Check if stream is initialized
	if qs.stream == nil {
		return fmt.Errorf("stream is not initialized")
	}

	// Send the write message
	message := createWriteMessage()
	encodedMessage, err := message.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = qs.stream.Write(encodedMessage)
	if err != nil {
		return fmt.Errorf("failed to write message to server: %w", err)
	}

	// Reuse buffer from pool
	buffer := bufferPool.Get().([]byte)
	defer bufferPool.Put(buffer) // Return buffer to pool after use

	// Read the response
	_, err = qs.stream.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Perform read operation
	readMessage := createReadMessage(message.Key)
	encodedReadMessage, err := readMessage.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode read message: %w", err)
	}

	_, err = qs.stream.Write(encodedReadMessage)
	if err != nil {
		return fmt.Errorf("failed to write read message: %w", err)
	}

	// Read the data length
	_, err = io.ReadFull(qs.stream, buffer[:4])
	if err != nil {
		return fmt.Errorf("failed to read data length: %w", err)
	}
	valueLength := binary.BigEndian.Uint32(buffer[:4])

	// Read the actual data
	readBuffer := make([]byte, valueLength)
	_, err = io.ReadFull(qs.stream, readBuffer)
	if err != nil {
		return fmt.Errorf("failed to read value: %w", err)
	}

	return nil
}
