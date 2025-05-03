package rpc

import (
	"context"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/state"
	"github.com/unpackdev/fdb/pkg/transports/tcp"
	"github.com/unpackdev/fdb/pkg/types"

	"github.com/unpackdev/fdb/pkg/config"

	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func SetupRPCServerForTest(t testing.TB) (rpcInstance *RPC, addr string, cleanup func()) {
	ctx := context.Background()

	// Use a free port.
	port := tcp.GetFreePortForTest(t)

	// Define node configuration.
	nodeConfig := config.Config{
		Id: "test_node_1",
		Logger: config.Logger{
			Enabled:     true,
			Environment: "development",
			Level:       "error",
		},
		Observability: config.Observability{},
		Rpc: config.Rpc{
			PoolMaxSize: 5,
			Transport: config.TcpTransport{
				Enabled: true,
				IPv4:    net.ParseIP("127.0.0.1").String(),
				Port:    port,
				Type:    types.TCPTransportType,
				TLS:     nil,
			},
		},
	}

	// Initialize global logger.
	gLog, err := logger.InitializeGlobalLogger(nodeConfig.Id, nodeConfig.Logger)
	require.NoError(t, err, "Failed to initialize global logger")

	// Initialize Observability.
	obs, err := observability.New(ctx, nodeConfig, gLog)
	require.NoError(t, err, "Failed to initialize Observability")

	stateMgr, smErr := state.NewManager(gLog, obs)
	require.NoError(t, smErr, "Failed to initialize state manager")

	// Create the RPC instance.
	rpcInstance, err = NewRPC(ctx, nodeConfig.Rpc, gLog, obs, stateMgr)
	require.NoError(t, err, "Failed to initialize RPC")

	// Register test handlers.
	RegisterTestHandlers(rpcInstance.Server())

	// Start the RPC server.
	err = rpcInstance.Start(ctx)
	require.NoError(t, err, "Failed to start RPC server")

	addr = rpcInstance.Transport().Addr()

	cleanup = func() {
		err := rpcInstance.Stop()
		if err != nil {
			t.Fatalf("Failed to stop RPC server: %v", err)
		}
	}

	return rpcInstance, addr, cleanup
}
