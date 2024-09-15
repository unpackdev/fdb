package fdb

import (
	"context"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/types"
)

type FDB struct {
	ctx              context.Context
	config           config.Config
	transportManager *TransportManager
}

func New(ctx context.Context, cnf config.Config) (*FDB, error) {
	if err := cnf.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	// Create a new transport manager
	transportManager := NewTransportManager()

	fdbInstance := &FDB{
		ctx:              ctx,
		config:           cnf,
		transportManager: transportManager,
	}

	for _, transport := range cnf.Transports {
		switch t := transport.Config.(type) {
		case config.QuicTransport:
			quicServer, err := NewQuicServer(t.IPv4, t.Port, nil)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create QUIC server")
			}
			if err := transportManager.RegisterTransport(types.QUICTransportType, quicServer); err != nil {
				return nil, errors.Wrap(err, "failed to register QUIC transport")
			}

		case config.UdsTransport:
			udsServer, err := NewUDSServer(t.IPv4)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create UDS server")
			}
			if err := transportManager.RegisterTransport(types.UDSTransportType, udsServer); err != nil {
				return nil, errors.Wrap(err, "failed to register UDS transport")
			}
		default:
			return nil, errors.New("unknown transport type")
		}
	}

	return fdbInstance, nil
}

// GetTransportByType allows retrieval of specific transport from the manager
func (fdb *FDB) GetTransportByType(tType types.TransportType) (Transport, error) {
	return fdb.transportManager.GetTransport(tType)
}
