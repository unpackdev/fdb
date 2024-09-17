package fdb

import (
	"fmt"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/transports"
	transport_quic "github.com/unpackdev/fdb/transports/quic"
	transport_tcp "github.com/unpackdev/fdb/transports/tcp"
	transport_udp "github.com/unpackdev/fdb/transports/udp"
	transport_uds "github.com/unpackdev/fdb/transports/uds"
	"github.com/unpackdev/fdb/types"
	"time"
)

// tRegistry is a transport registry mapping transport types (e.g., QUIC, TCP, UDP, UDS) to their initialization functions.
// Each function initializes the transport, registers appropriate handlers (write, read),
// and returns the instantiated transport or an error if initialization fails.
var tRegistry = map[types.TransportType]func(fdb *FDB, dbP db.Provider) (transports.Transport, error){
	types.QUICTransportType: func(fdb *FDB, dbP db.Provider) (transports.Transport, error) {
		quicTransport, err := fdb.GetTransportByType(types.QUICTransportType)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve QUIC transport: %w", err)
		}

		quicServer, ok := quicTransport.(*transport_quic.Server)
		if !ok {
			return nil, fmt.Errorf("failed to cast transport to QuicServer")
		}

		// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
		batchWriter := db.NewBatchWriter(dbP.(*db.Db), 512, 500*time.Millisecond, 15)

		wHandler := transport_quic.NewQuicWriteHandler(dbP, batchWriter)
		quicServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

		rHandler := transport_quic.NewQuicReadHandler(dbP)
		quicServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

		return quicTransport, nil
	},
	types.TCPTransportType: func(fdb *FDB, dbP db.Provider) (transports.Transport, error) {
		tcpTransport, err := fdb.GetTransportByType(types.TCPTransportType)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve TCP transport: %w", err)
		}

		tcpServer, ok := tcpTransport.(*transport_tcp.Server)
		if !ok {
			return nil, fmt.Errorf("failed to cast transport to TcpServer")
		}

		// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
		batchWriter := db.NewBatchWriter(dbP.(*db.Db), 512, 500*time.Millisecond, 15)

		wHandler := transport_tcp.NewTCPWriteHandler(dbP, batchWriter)
		tcpServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

		rHandler := transport_tcp.NewTCPReadHandler(dbP)
		tcpServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

		return tcpTransport, nil
	},
	types.UDSTransportType: func(fdb *FDB, dbP db.Provider) (transports.Transport, error) {
		udsTransport, err := fdb.GetTransportByType(types.UDSTransportType)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve UDS transport: %w", err)
		}

		udsServer, ok := udsTransport.(*transport_uds.Server)
		if !ok {
			return nil, fmt.Errorf("failed to cast transport to UdsServer")
		}

		// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
		batchWriter := db.NewBatchWriter(dbP.(*db.Db), 512, 500*time.Millisecond, 15)

		// Register write and read handlers
		wHandler := transport_uds.NewUDSWriteHandler(dbP, batchWriter)
		udsServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

		rHandler := transport_uds.NewUDSReadHandler(dbP)
		udsServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

		return udsTransport, nil
	},
	types.UDPTransportType: func(fdb *FDB, dbP db.Provider) (transports.Transport, error) {
		udpTransport, err := fdb.GetTransportByType(types.UDPTransportType)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve UDP transport: %w", err)
		}

		udpServer, ok := udpTransport.(*transport_udp.Server)
		if !ok {
			return nil, fmt.Errorf("failed to cast transport to UdpServer")
		}

		// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
		batchWriter := db.NewBatchWriter(dbP.(*db.Db), 512, 500*time.Millisecond, 15)

		wHandler := transport_udp.NewUDPWriteHandler(dbP, batchWriter)
		udpServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

		rHandler := transport_udp.NewUDPReadHandler(dbP)
		udpServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

		return udpTransport, nil
	},
}
