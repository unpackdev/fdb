package fdb

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/node"
	"github.com/unpackdev/fdb/transports"
	transport_tcp "github.com/unpackdev/fdb/transports/tcp"
	"github.com/unpackdev/fdb/types"
)

// tRegistry is a transport registry mapping transport types (e.g., QUIC, TCP, UDP, UDS) to their initialization functions.
// Each function initializes the transport, registers appropriate handlers (write, read),
// and returns the instantiated transport or an error if initialization fails.
var tRegistry = map[types.TransportType]func(fdb *FDB, config config.TransportConfig, dbP db.Provider) (transports.Transport, error){
	// types.QUICTransportType: func(fdb *FDB, config config.TransportConfig, dbP db.Provider) (transports.Transport, error) {
	// 	quicTransport, err := fdb.GetTransportByType(types.QUICTransportType)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to retrieve QUIC transport: %w", err)
	// 	}

	// 	quicServer, ok := quicTransport.(*transport_quic.Server)
	// 	if !ok {
	// 		return nil, fmt.Errorf("failed to cast transport to QuicServer")
	// 	}

	// 	// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
	// 	batchWriter := db.NewBatchWriter(dbP.(*db.Db), 512, 500*time.Millisecond, 15)

	// 	wHandler := transport_quic.NewQuicWriteHandler(dbP, batchWriter)
	// 	quicServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	// 	rHandler := transport_quic.NewQuicReadHandler(dbP)
	// 	quicServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	// 	return quicTransport, nil
	// },
	types.TCPTransportType: func(fdb *FDB, cfg config.TransportConfig, dbP db.Provider) (transports.Transport, error) {
		tCfg, ok := cfg.(*config.TcpTransport)
		if !ok {
			return nil, fmt.Errorf("failed to cast transport config to TcpTransport")
		}
		tcpServer, err := transport_tcp.NewServer(fdb.ctx, *tCfg, fdb.logger, fdb.obs)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create TCP server")
		}

		wHandler := node.NewDbWriteHandler(dbP, fdb.batchWriter, fdb.logger, fdb.obs, fdb.dNode.Distributor())
		tcpServer.RegisterHandler(types.WriteHandlerType, wHandler)

		rHandler := node.NewDbReadHandler(dbP, fdb.logger, fdb.obs, fdb.dNode.Distributor())
		tcpServer.RegisterHandler(types.ReadHandlerType, rHandler)

		return tcpServer, nil
	},
	// types.UDSTransportType: func(fdb *FDB, cfg config.TransportConfig, dbP db.Provider) (transports.Transport, error) {
	// 	tCfg, ok := cfg.(*config.UdsTransport)
	// 	if !ok {
	// 		return nil, fmt.Errorf("failed to cast transport config to UdsTransport")
	// 	}
	// 	udsServer, err := transport_uds.NewServer(fdb.ctx, *tCfg, fdb.logger, fdb.obs)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "failed to create UDS server")
	// 	}

	// 	wHandler := node.NewDbWriteHandler(dbP, fdb.batchWriter, fdb.logger, fdb.obs, fdb.dNode.Distributor())
	// 	udsServer.RegisterHandler(types.WriteHandlerType, wHandler)

	// 	rHandler := node.NewDbReadHandler(dbP, fdb.logger, fdb.obs, fdb.dNode.Distributor())
	// 	udsServer.RegisterHandler(types.ReadHandlerType, rHandler)

	// 	return udsServer, nil
	// },
	// types.UDPTransportType: func(fdb *FDB, config config.TransportConfig, dbP db.Provider) (transports.Transport, error) {
	// 	udpTransport, err := fdb.GetTransportByType(types.UDPTransportType)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to retrieve UDP transport: %w", err)
	// 	}

	// 	udpServer, ok := udpTransport.(*transport_udp.Server)
	// 	if !ok {
	// 		return nil, fmt.Errorf("failed to cast transport to UdpServer")
	// 	}

	// 	// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
	// 	batchWriter := db.NewBatchWriter(dbP.(*db.Db), 512, 500*time.Millisecond, 15)

	// 	wHandler := transport_udp.NewUDPWriteHandler(dbP, batchWriter)
	// 	udpServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	// 	rHandler := transport_udp.NewUDPReadHandler(dbP)
	// 	udpServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	// 	return udpTransport, nil
	// },
}
