package fdb

import (
	"context"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/transports"
	transport_quic "github.com/unpackdev/fdb/transports/quic"
	"github.com/unpackdev/fdb/types"
)

type FDB struct {
	ctx              context.Context
	config           config.Config
	transportManager *transports.Manager
	dbManager        *db.Manager
}

func New(ctx context.Context, cnf config.Config) (*FDB, error) {
	if err := cnf.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	// Create a new transport manager
	transportManager := transports.NewManager()

	dbM, dbmErr := db.NewManager(ctx, cnf.MdbxNodes)
	if dbmErr != nil {
		return nil, errors.Wrap(dbmErr, "failure to create database manager")
	}

	fdbInstance := &FDB{
		ctx:              ctx,
		config:           cnf,
		transportManager: transportManager,
		dbManager:        dbM,
	}

	for _, transport := range cnf.Transports {
		switch t := transport.Config.(type) {
		case config.QuicTransport:
			quicServer, err := transport_quic.NewServer(ctx, t)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create QUIC server")
			}
			if err := transportManager.RegisterTransport(types.QUICTransportType, quicServer); err != nil {
				return nil, errors.Wrap(err, "failed to register QUIC transport")
			}

			/*		case config.UdsTransport:
					udsServer, err := NewUDSServer(t.IPv4)
					if err != nil {
						return nil, errors.Wrap(err, "failed to create UDS server")
					}
					if err := transportManager.RegisterTransport(types.UDSTransportType, udsServer); err != nil {
						return nil, errors.Wrap(err, "failed to register UDS transport")
					}*/
		default:
			return nil, errors.New("unknown transport type")
		}
	}

	return fdbInstance, nil
}

func (fdb *FDB) GetConfig() config.Config {
	return fdb.config
}

func (fdb *FDB) GetDbManager() *db.Manager {
	return fdb.dbManager
}

func (fdb *FDB) GetTransportManager() *transports.Manager {
	return fdb.transportManager
}

// GetTransportByType allows retrieval of specific transport from the manager
func (fdb *FDB) GetTransportByType(tType types.TransportType) (transports.Transport, error) {
	return fdb.transportManager.GetTransport(tType)
}
