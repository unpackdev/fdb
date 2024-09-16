package fdb

import (
	"fmt"
	"github.com/unpackdev/fdb/db"
	transport_tcp "github.com/unpackdev/fdb/transports/tcp"
	"github.com/unpackdev/fdb/types"
	"time"
)

var registry = map[types.TransportType]func(fdb *FDB) error{
	types.TCPTransportType: func(fdb *FDB) error {
		tcpTransport, err := fdb.GetTransportByType(types.TCPTransportType)
		if err != nil {
			return fmt.Errorf("failed to retrieve TCP transport: %w", err)
		}

		tcpServer, ok := tcpTransport.(*transport_tcp.Server)
		if !ok {
			return fmt.Errorf("failed to cast transport to TcpServer")
		}

		bDb, err := fdb.GetDbManager().GetDb("fdb")
		if err != nil {
			return fmt.Errorf("failed to retrieve benchmark database: %w", err)
		}

		// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
		batchWriter := db.NewBatchWriter(bDb.(*db.Db), 512, 500*time.Millisecond, 15)

		wHandler := transport_tcp.NewTCPWriteHandler(bDb, batchWriter)
		tcpServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

		rHandler := transport_tcp.NewTCPReadHandler(bDb)
		tcpServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

		return tcpServer.Start()
	},
}
