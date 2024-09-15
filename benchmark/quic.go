package benchmark

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/types"
	"io"
	"runtime"
	"time"
)

// QuicSuite represents the QUIC-specific benchmark suite.
type QuicSuite struct {
	fdbInstance *fdb.FDB
	quicServer  *fdb.QuicServer
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

	quicServer, ok := quicTransport.(*fdb.QuicServer)
	if !ok {
		return fmt.Errorf("failed to cast transport to QuicServer")
	}

	db, err := qs.fdbInstance.GetDbManager().GetDb("benchmark")
	if err != nil {
		return fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	wHandler := fdb.NewQuicWriteHandler(db)
	quicServer.RegisterHandler(fdb.WriteHandlerType, wHandler.HandleMessage)

	rHandler := fdb.NewQuicReadHandler(db)
	quicServer.RegisterHandler(fdb.ReadHandlerType, rHandler.HandleMessage)

	if err := quicServer.Start(); err != nil {
		return fmt.Errorf("failed to start QUIC server: %w", err)
	}

	qs.quicServer = quicServer
	fmt.Println("QUIC server started successfully")
	return nil
}

// Stop stops the QUIC server.
func (qs *QuicSuite) Stop() {
	if qs.quicServer != nil {
		qs.quicServer.Stop()
		fmt.Println("QUIC server stopped successfully")
	}
}

// Run runs the client-side benchmark for QUIC.
func (qs *QuicSuite) Run(ctx context.Context) error {
	serverAddr := qs.quicServer.Addr()

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	start := time.Now()
	var memStart runtime.MemStats
	runtime.ReadMemStats(&memStart)

	client, err := quic.DialAddr(ctx, serverAddr, clientTLSConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC server: %w", err)
	}
	defer client.CloseWithError(0, "closing connection")

	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	message := createWriteMessage()
	encodedMessage, err := message.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = stream.Write(encodedMessage)
	if err != nil {
		return fmt.Errorf("failed to write message to server: %w", err)
	}

	buffer := make([]byte, 1024)
	_, err = stream.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	fmt.Printf("Response from server: %s\n", string(buffer))

	readMessage := createReadMessage(message.Key)
	encodedReadMessage, err := readMessage.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode read message: %w", err)
	}

	_, err = stream.Write(encodedReadMessage)
	if err != nil {
		return fmt.Errorf("failed to write read message to server: %w", err)
	}

	_, err = io.ReadFull(stream, buffer[:4])
	if err != nil {
		return fmt.Errorf("failed to read data length: %w", err)
	}
	valueLength := binary.BigEndian.Uint32(buffer[:4])

	readBuffer := make([]byte, valueLength)
	_, err = io.ReadFull(stream, readBuffer)
	if err != nil {
		return fmt.Errorf("failed to read value: %w", err)
	}
	fmt.Printf("Data read from server: %s\n", string(readBuffer))

	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)

	elapsed := time.Since(start)
	fmt.Printf("Benchmark completed in %s\n", elapsed)
	fmt.Printf("Memory used: %d bytes\n", memEnd.Alloc-memStart.Alloc)

	return nil
}
