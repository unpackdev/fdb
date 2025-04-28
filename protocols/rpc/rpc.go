// pkg/protocols/rpc/rpc.go
package rpc

import (
	"context"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/state"
	"github.com/unpackdev/fdb/transports/tcp"
	"go.uber.org/zap"
	"sync"
	"time"
)

// RPC encapsulates the RPC server, transport, and connection pool.
type RPC struct {
	ctx            context.Context
	server         *Server
	transport      *tcp.Server
	connectionPool *ConnectionPool
	cfg            *config.Rpc
	wg             sync.WaitGroup
	startOnce      sync.Once // Ensures Start is called only once
	startErr       error     // Stores any error from Start
	logger         logger.Logger
	stateMgr       *state.StateManager
}

// NewRPC initializes and returns a new RPC instance.
func NewRPC(ctx context.Context, cfg config.Rpc, gLog logger.Logger, obs *observability.Observability, stateMgr *state.StateManager) (*RPC, error) {
	stateMgr.SetState(RpcStateType, state.Initializing)

	// Create RPC server
	rpcServer := NewServer(gLog, obs)

	// Create the TCP server
	tcpServer, err := tcp.NewServer(ctx, cfg.Transport, gLog, obs)
	if err != nil {
		stateMgr.SetState(RpcStateType, state.Failed)
		return nil, err
	}

	defer stateMgr.SetState(RpcStateType, state.Initialized)

	// Initialize HTTP handler
	httpHandler := NewHTTPHandler(rpcServer, gLog)

	// Initialize WebSocket handler (assuming it's updated similarly)
	wsHandler := NewWebSocketHandler(rpcServer, gLog)

	// Initialize Multiplexing handler
	multiplexingHandler := tcp.NewMultiplexingTrafficHandler(httpHandler, wsHandler, gLog)

	tcpServer.SetOnTrafficHandler(multiplexingHandler.Handle)
	tcpServer.SetOnCloseHandler(multiplexingHandler.OnClose)

	// Initialize connection pool
	connectionPool := NewConnectionPool(tcpServer.Addr(), cfg.PoolMaxSize)

	// Set the WebSocket handler in the server
	rpcServer.SetWebSocketHandler(wsHandler)

	return &RPC{
		ctx:            ctx,
		server:         rpcServer,
		transport:      tcpServer,
		connectionPool: connectionPool,
		cfg:            &cfg,
		logger:         gLog,
		stateMgr:       stateMgr,
	}, nil
}

// Start launches the RPC server and transport.
func (rpc *RPC) Start(ctx context.Context) error {
	rpc.startOnce.Do(func() {
		rpc.stateMgr.SetState(RpcStateType, state.Starting)
		// Start the transport server
		rpc.wg.Add(1)
		go func() {
			defer rpc.wg.Done()
			if err := rpc.transport.Start(ctx); err != nil {
				rpc.server.logger.Error("Failed to start transport", zap.Error(err))
				rpc.startErr = err
				rpc.stateMgr.SetState(RpcStateType, state.Failed)
			}
		}()

		// Wait for the transport to start
		select {
		case <-rpc.transport.WaitStarted():
			rpc.server.logger.Info("RPC transport started", zap.String("addr", rpc.transport.Addr()))
			rpc.stateMgr.SetState(RpcStateType, state.Started)
		case <-time.After(10 * time.Second):
			rpc.stateMgr.SetState(RpcStateType, state.Failed)
			rpc.startErr = errors.New("RPC transport did not start in time")
		}
	})

	return rpc.startErr
}

// Stop gracefully stops the RPC server and transport.
func (rpc *RPC) Stop() error {
	rpc.stateMgr.SetState(RpcStateType, state.Stopping)
	// Stop the transport server
	if err := rpc.transport.Stop(); err != nil {
		rpc.server.logger.Error("Failed to stop transport", zap.Error(err))
		rpc.stateMgr.SetState(RpcStateType, state.Failed)
		return err
	}

	// Close the connection pool
	rpc.connectionPool.Close()

	// Wait for all goroutines to finish
	rpc.wg.Wait()

	rpc.server.logger.Info("RPC server stopped gracefully")
	rpc.stateMgr.SetState(RpcStateType, state.Stopped)
	return nil
}

// GetConnectionPool returns the connection pool for external usage if needed.
func (rpc *RPC) GetConnectionPool() *ConnectionPool {
	return rpc.connectionPool
}

// Server returns the underlying RPC server.
func (rpc *RPC) Server() *Server {
	return rpc.server
}

// Transport returns the underlying transport server.
func (rpc *RPC) Transport() *tcp.Server {
	return rpc.transport
}

// Call sends an RPC request using the connection pool.
func (rpc *RPC) Call(ctx context.Context, req Request) (Response, error) {
	client := NewRPCClient(rpc.connectionPool)
	return client.Call(ctx, req)
}
