package fdb

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/pprof"
	"github.com/unpackdev/fdb/transports"
	transport_dummy "github.com/unpackdev/fdb/transports/dummy"
	transport_quic "github.com/unpackdev/fdb/transports/quic"
	transport_tcp "github.com/unpackdev/fdb/transports/tcp"
	transport_udp "github.com/unpackdev/fdb/transports/udp"
	transport_uds "github.com/unpackdev/fdb/transports/uds"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type FDB struct {
	ctx       context.Context
	config    config.Config
	tm        *transports.Manager
	dbManager *db.Manager
}

func New(ctx context.Context, cnf config.Config) (*FDB, error) {
	if err := cnf.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	// Sets the global logger.
	// I hate to pass by reference logger everywhere...
	// In case you wish to use your own zap logger you can disable logger here,
	// implement your own and set the globals on your end.
	if cnf.Logger.Enabled {
		zLog, zlErr := logger.GetLogger(cnf.Logger.Environment, cnf.Logger.Level)
		if zlErr != nil {
			return nil, errors.Wrap(zlErr, "failure to construct new logger")
		}
		zap.ReplaceGlobals(zLog)
	}

	// Create a new transport manager
	transportManager := transports.NewManager()

	dbM, dbmErr := db.NewManager(ctx, cnf.Mdbx)
	if dbmErr != nil {
		return nil, errors.Wrap(dbmErr, "failure to create database manager")
	}

	fdbInstance := &FDB{
		ctx:       ctx,
		config:    cnf,
		tm:        transportManager,
		dbManager: dbM,
	}

	for _, transport := range cnf.Transports {
		switch t := transport.Config.(type) {
		case *config.DummyTransport:
			udsServer, err := transport_dummy.NewDummyServer(ctx, *t)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create dummy server")
			}
			if err := transportManager.RegisterTransport(types.DummyTransportType, udsServer); err != nil {
				return nil, errors.Wrap(err, "failed to register UDS transport")
			}
		case *config.QuicTransport:
			quicServer, err := transport_quic.NewServer(ctx, *t)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create QUIC server")
			}
			if err := transportManager.RegisterTransport(types.QUICTransportType, quicServer); err != nil {
				return nil, errors.Wrap(err, "failed to register QUIC transport")
			}

		case *config.UdsTransport:
			udsServer, err := transport_uds.NewServer(ctx, *t)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create UDS server")
			}
			if err := transportManager.RegisterTransport(types.UDSTransportType, udsServer); err != nil {
				return nil, errors.Wrap(err, "failed to register UDS transport")
			}
		case *config.TcpTransport:
			tcpServer, err := transport_tcp.NewServer(ctx, *t)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create TCP server")
			}
			if err := transportManager.RegisterTransport(types.TCPTransportType, tcpServer); err != nil {
				return nil, errors.Wrap(err, "failed to register TCP transport")
			}
		case *config.UdpTransport:
			udpServer, err := transport_udp.NewServer(ctx, *t)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create UDP server")
			}
			if err := transportManager.RegisterTransport(types.UDPTransportType, udpServer); err != nil {
				return nil, errors.Wrap(err, "failed to register UDP transport")
			}
		default:
			return nil, fmt.Errorf("unknown transport type provided: %v", t.GetTransportType())
		}
	}

	return fdbInstance, nil
}

func (fdb *FDB) Start(ctx context.Context, transports ...types.TransportType) error {
	g, gCtx := errgroup.WithContext(ctx)

	bDb, err := fdb.GetDbManager().GetDb("fdb")
	if err != nil {
		return fmt.Errorf("failed to retrieve fdb database: %w", err)
	}

	pCfg, pcErr := fdb.config.GetPprofByServiceTag("fdb")
	if pcErr != nil {
		return errors.Wrapf(pcErr, "failed to retrieve fdb pprof config for service tag: %s", "fdb")
	}

	if pCfg.Enabled {
		g.Go(func() error {
			return pprof.New(ctx, *pCfg).Start()
		})
	}

	for _, transport := range transports {
		transportFn, tnOk := tRegistry[transport]
		if !tnOk {
			return fmt.Errorf("unknown transport type provided: %v - rejecting serving transports", transport)
		}

		iTransport, itErr := transportFn(fdb, bDb)
		if itErr != nil {
			return errors.Wrapf(itErr, "failure to create transport: %s", transport)
		}

		g.Go(func() error {
			return iTransport.Start(gCtx)
		})
	}

	if gErr := g.Wait(); gErr != nil {
		return errors.Wrap(gErr, "failure to start fdb database")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (fdb *FDB) Stop(transports ...types.TransportType) error {
	for _, transport := range transports {
		t, tErr := fdb.tm.GetTransport(transport)
		if tErr != nil {
			return tErr
		}

		if err := t.Stop(); err != nil {
			return err
		}
	}

	zap.L().Info("All transports successfully stopped")
	return nil
}

func (fdb *FDB) GetConfig() config.Config {
	return fdb.config
}

func (fdb *FDB) GetDbManager() *db.Manager {
	return fdb.dbManager
}

func (fdb *FDB) GetTransportManager() *transports.Manager {
	return fdb.tm
}

// GetTransportByType allows retrieval of specific transport from the manager
func (fdb *FDB) GetTransportByType(tType types.TransportType) (transports.Transport, error) {
	return fdb.tm.GetTransport(tType)
}
