package benchmark

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	transport_dummy "github.com/unpackdev/fdb/transports/dummy"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"net"
)

type DummySuite struct {
	fdbInstance *fdb.FDB
	server      *transport_dummy.Server
	client      *net.UDPConn
}

func NewDummySuite(fdbInstance *fdb.FDB) *DummySuite {
	return &DummySuite{
		fdbInstance: fdbInstance,
	}
}

// Start starts the QUIC server for benchmarking.
func (qs *DummySuite) Start(ctx context.Context) error {
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

	if sErr := dummyServer.Start(); sErr != nil {
		zap.L().Error(
			"failed to start dummy transport",
			zap.Error(sErr),
		)
	}

	qs.server = dummyServer
	zap.L().Info("Dummy transport is ready to accept the traffic", zap.String("addr", dummyServer.Addr()))
	return nil
}

// Stop stops the QUIC server and closes the client connection and stream.
func (qs *DummySuite) Stop(ctx context.Context) error {
	if qs.client != nil {
		if err := qs.client.Close(); err != nil {
			return err
		}
	}
	if qs.server != nil {
		if err := qs.server.Stop(); err != nil {
			return err
		}
	}
	fmt.Println("Dummy server stopped successfully")
	return nil
}

func (qs *DummySuite) SetupClient(ctx context.Context) error {
	if qs.client != nil {
		return nil // Already setup, reuse client and stream
	}

	// Resolve the server address
	serverAddr, err := net.ResolveUDPAddr("udp", qs.server.Addr())
	if err != nil {
		return errors.Wrap(err, "failed to resolve server address")
	}

	// Create the UDP client
	client, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to server")
	}

	qs.client = client
	return nil
}

// Run sends a single message through a QUIC stream sequentially.
func (qs *DummySuite) Run(ctx context.Context) error {
	// Check if stream is initialized
	if qs.client == nil {
		return fmt.Errorf("client is not initialized")
	}

	message := createWriteMessage()
	encodedMessage, err := message.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	_, wErr := qs.client.Write(encodedMessage)
	if wErr != nil {
		return errors.Wrap(wErr, "failed to write message dummy message")
	}

	return nil
}
