package fdb

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/accounts"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/node"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/pprof"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/state"
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
	ctx      context.Context
	config   config.Config
	obs      *observability.Observability
	tm       *transports.Manager
	dbMgr    *db.Manager
	rbacMgr  *rbac.Manager
	store    *accounts.Store
	account  *accounts.Account
	stateMgr *state.StateManager
	dNode    *node.Node
}

func New(ctx context.Context, cfg config.Config) (*FDB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	// Sets the global logger.
	// I hate to pass by reference logger everywhere...
	// In case you wish to use your own zap logger you can disable logger here,
	// implement your own and set the globals on your end.
	zLog, zlErr := logger.InitializeGlobalLogger(cfg.Logger)
	if zlErr != nil {
		return nil, errors.Wrap(zlErr, "failure to initialize logger")
	}

	// Initialize metrics and tracing system (opentelemetry && prometheus)
	obs, err := observability.Initialize(ctx, cfg, zLog)
	if err != nil {
		return nil, errors.Wrap(zlErr, "failure to initialize observability")
	}

	// Create a new transport manager
	transportManager := transports.NewManager()

	dbM, dbmErr := db.NewManager(ctx, cfg.Mdbx)
	if dbmErr != nil {
		return nil, errors.Wrap(dbmErr, "failure to create database manager")
	}

	// Initialize global rbac (resource based access control)
	rbacMgr, rbmErr := rbac.NewManager(
		ctx,
		rbac.WithDefaultRoles(),
	)
	if rbmErr != nil {
		return nil, errors.Wrap(rbmErr, "failed to create rbac manager")
	}

	// Initialize global identity store.
	// Identities will be necessary throughout different applications.
	// This is where all peer ids can be found
	store, err := accounts.NewStore(cfg.Identity, zLog, rbacMgr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new account store")
	}

	// Attempt to load the identity from the manager using the PeerID
	account, err := store.GetByPeerID(cfg.Networking.PeerID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account from identity manager with PeerID: %s", cfg.Networking.PeerID)
	}

	stateMgr, smErr := state.NewManager(zLog, obs)
	if smErr != nil {
		return nil, errors.Wrap(smErr, "failure to create new global state manager")
	}

	// Node is basically a wrapper around consensus, chain, identity management, peer system and peer discovery system.
	// Not to forget metrics and ping-pong game between peers to establish metrics baseline.
	// Construction is done here because other services might need it.
	// In this state, block producer callback is not set.
	// Sequencer will be setting up block producer callback and utilize it.
	dNode, dnErr := node.NewNode(ctx, cfg, rbacMgr, zLog, store, obs, stateMgr)
	if dnErr != nil {
		return nil, errors.Wrap(dnErr, "failed to initialize node")
	}

	fdbInstance := &FDB{
		ctx:      ctx,
		config:   cfg,
		obs:      obs,
		tm:       transportManager,
		dbMgr:    dbM,
		rbacMgr:  rbacMgr,
		store:    store,
		account:  account,
		stateMgr: stateMgr,
		dNode:    dNode,
	}

	for _, transport := range cfg.Transports {
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
			tcpServer, err := transport_tcp.NewServer(ctx, *t, zLog, obs)
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

	g.Go(func() error {
		return fdb.dNode.Start()
	})

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

	if err := fdb.dNode.Shutdown(); err != nil {
		return err
	}

	zap.L().Info("All transports successfully stopped")
	return nil
}

func (fdb *FDB) GetConfig() config.Config {
	return fdb.config
}

func (fdb *FDB) GetDbManager() *db.Manager {
	return fdb.dbMgr
}

func (fdb *FDB) GetRbacManager() *rbac.Manager {
	return fdb.rbacMgr
}

func (fdb *FDB) GetTransportManager() *transports.Manager {
	return fdb.tm
}

func (fdb *FDB) GetStateManager() *state.StateManager {
	return fdb.stateMgr
}

func (fdb *FDB) GetObservability() *observability.Observability {
	return fdb.obs
}

func (fdb *FDB) GetAccount() *accounts.Account {
	return fdb.account
}

func (fdb *FDB) GetNode() *node.Node {
	return fdb.dNode
}

// GetTransportByType allows retrieval of specific transport from the manager
func (fdb *FDB) GetTransportByType(tType types.TransportType) (transports.Transport, error) {
	return fdb.tm.GetTransport(tType)
}
