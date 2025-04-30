package fdb

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/accounts"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/node"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/pprof"
	"github.com/unpackdev/fdb/protocols/rpc"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/state"
	"github.com/unpackdev/fdb/transports"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type FDB struct {
	ctx         context.Context
	config      config.Config
	logger      logger.Logger
	obs         *observability.Observability
	tm          *transports.Manager
	dbMgr       *db.Manager
	rbacMgr     *rbac.Manager
	store       *accounts.Store
	account     *accounts.Account
	stateMgr    *state.StateManager
	dNode       *node.Node
	rpcSvc      *rpc.RPC
	batchWriter *db.BatchWriter
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
	tManager := transports.NewManager()

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

	// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
	dbI, dbiErr := dbM.GetDb("fdb")
	if dbiErr != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", dbiErr)
	}

	batchWriter := db.NewBatchWriter(dbI.(*db.Db), 2048, 100*time.Millisecond, 15)

	// Node is basically a wrapper around consensus, chain, identity management, peer system and peer discovery system.
	// Not to forget metrics and ping-pong game between peers to establish metrics baseline.
	// Construction is done here because other services might need it.
	// In this state, block producer callback is not set.
	// Sequencer will be setting up block producer callback and utilize it.
	dNode, dnErr := node.NewNode(ctx, cfg, rbacMgr, zLog, store, obs, stateMgr, dbM, batchWriter)
	if dnErr != nil {
		return nil, errors.Wrap(dnErr, "failed to initialize node")
	}

	rpcSvc, riErr := rpc.NewRPC(ctx, cfg.Rpc, zLog, obs, stateMgr)
	if riErr != nil {
		return nil, fmt.Errorf("failed to initialize RPC: %w", riErr)
	}

	fdbInstance := &FDB{
		ctx:         ctx,
		config:      cfg,
		logger:      zLog,
		obs:         obs,
		tm:          tManager,
		dbMgr:       dbM,
		rbacMgr:     rbacMgr,
		store:       store,
		account:     account,
		stateMgr:    stateMgr,
		dNode:       dNode,
		rpcSvc:      rpcSvc,
		batchWriter: batchWriter,
	}

	for _, transport := range cfg.Transports {
		transportFn, tnOk := tRegistry[transport.Config.GetTransportType()]
		if !tnOk {
			return nil, fmt.Errorf("unknown transport type provided: %v - rejecting serving transports", transport)
		}

		iTransport, itErr := transportFn(fdbInstance, transport.Config, dbI)
		if itErr != nil {
			return nil, errors.Wrapf(itErr, "failure to create transport: %s", transport.Config.GetTransportType())
		}

		if err := tManager.RegisterTransport(transport.Config.GetTransportType(), iTransport); err != nil {
			return nil, errors.Wrapf(err, "failed to register transport: %s", transport.Config.GetTransportType())
		}
	}

	return fdbInstance, nil
}

func NewWithArgs(
	ctx context.Context, cfg config.Config, logger logger.Logger,
	obs *observability.Observability, tManager *transports.Manager, dbM *db.Manager,
	rbacMgr *rbac.Manager, store *accounts.Store, stateMgr *state.StateManager, rpcSvc *rpc.RPC) (*FDB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	// Attempt to load the identity from the manager using the PeerID
	// TODO: Perhaps even this should be outside as an argument. For now keeping it as is to keep things sane.
	account, err := store.GetByPeerID(cfg.Networking.PeerID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account from identity manager with PeerID: %s", cfg.Networking.PeerID)
	}

	// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
	dbI, dbiErr := dbM.GetDb("fdb")
	if dbiErr != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", dbiErr)
	}

	batchWriter := db.NewBatchWriter(dbI.(*db.Db), 2048, 100*time.Millisecond, 15)

	// Create a new node
	dNode, err := node.NewNode(ctx, cfg, rbacMgr, logger, store, obs, stateMgr, dbM, batchWriter)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize node: %w", err)
	}

	fdbInstance := &FDB{
		ctx:         ctx,
		config:      cfg,
		logger:      logger,
		obs:         obs,
		tm:          tManager,
		dbMgr:       dbM,
		rbacMgr:     rbacMgr,
		store:       store,
		account:     account,
		stateMgr:    stateMgr,
		rpcSvc:      rpcSvc,
		dNode:       dNode,
		batchWriter: batchWriter,
	}

	for _, transport := range cfg.Transports {
		transportFn, tnOk := tRegistry[transport.Config.GetTransportType()]
		if !tnOk {
			return nil, fmt.Errorf("unknown transport type provided: %v - rejecting serving transports", transport)
		}

		iTransport, itErr := transportFn(fdbInstance, transport.Config, dbI)
		if itErr != nil {
			return nil, errors.Wrapf(itErr, "failure to create transport: %s", transport.Config.GetTransportType())
		}

		if err := tManager.RegisterTransport(transport.Config.GetTransportType(), iTransport); err != nil {
			return nil, errors.Wrapf(err, "failed to register transport: %s", transport.Config.GetTransportType())
		}
	}

	return fdbInstance, nil
}

func (fdb *FDB) Start(ctx context.Context, transports ...types.TransportType) error {
	g, gCtx := errgroup.WithContext(ctx)

	pCfg, pcErr := fdb.config.GetPprofByServiceTag("fdb")
	if pcErr != nil {
		return errors.Wrapf(pcErr, "failed to retrieve fdb pprof config for service tag: %s", "fdb")
	}

	if pCfg.Enabled {
		g.Go(func() error {
			return pprof.New(ctx, *pCfg).Start()
		})
	}

	for _, transport := range fdb.tm.GetTransports() {
		g.Go(func() error {
			return transport.Start(ctx)
		})
	}

	g.Go(func() error {
		return fdb.dNode.Start()
	})

	if fdb.rpcSvc != nil {
		g.Go(func() error {
			return fdb.rpcSvc.Start(gCtx)
		})
	}

	if gErr := g.Wait(); gErr != nil {
		return errors.Wrap(gErr, "failure to start fdb database")
	}

	<-ctx.Done()
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
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

	if fdb.rpcSvc != nil {
		if err := fdb.rpcSvc.Stop(); err != nil {
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
