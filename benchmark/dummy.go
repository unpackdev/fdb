package benchmark

import (
	"context"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb"
	transport_dummy "github.com/unpackdev/fdb/transports/dummy"
	"github.com/unpackdev/fdb/types"
)

type DummySuite struct {
	fdbInstance *fdb.FDB
	server      *transport_dummy.Server
	client      quic.Connection
	stream      quic.Stream
}

func NewDummySuite(fdbInstance *fdb.FDB) *DummySuite {
	return &DummySuite{
		fdbInstance: fdbInstance,
	}
}

// Start starts the QUIC server for benchmarking.
func (qs *DummySuite) Start() error {
	dummyTransport, err := qs.fdbInstance.GetTransportByType(types.DummyTransportType)
	if err != nil {
		return fmt.Errorf("failed to retrieve QUIC transport: %w", err)
	}

	dummyServer, ok := dummyTransport.(*transport_dummy.Server)
	if !ok {
		return fmt.Errorf("failed to cast transport to DummyServer")
	}

	db, err := qs.fdbInstance.GetDbManager().GetDb("benchmark")
	if err != nil {
		return fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	wHandler := transport_dummy.NewDummyWriteHandler(db)
	dummyServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	rHandler := transport_dummy.NewDummyReadHandler(db)
	dummyServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	if err := dummyServer.Start(); err != nil {
		return fmt.Errorf("failed to start Dummy server: %w", err)
	}

	qs.server = dummyServer
	fmt.Println("QUIC server started successfully")
	return nil
}

// Stop stops the QUIC server and closes the client connection and stream.
func (qs *DummySuite) Stop() {
	if qs.stream != nil {
		qs.stream.Close()
	}
	if qs.client != nil {
		qs.client.CloseWithError(0, "closing connection")
	}
	if qs.server != nil {
		qs.server.Stop()
		fmt.Println("QUIC server stopped successfully")
	}
}

// SetupClient sets up a QUIC client and stream only once.
func (qs *DummySuite) SetupClient(ctx context.Context) error {
	if qs.client != nil && qs.stream != nil {
		return nil // Already setup, reuse client and stream
	}

	/*	serverAddr := qs.server.Addr()

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
		qs.stream = stream*/

	return nil
}

// Run sends a single message through a QUIC stream sequentially.
func (qs *DummySuite) Run(ctx context.Context) error {
	// Check if stream is initialized
	if qs.stream == nil {
		return fmt.Errorf("stream is not initialized")
	}

	/*	// Send the write message
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
	*/
	return nil
}
