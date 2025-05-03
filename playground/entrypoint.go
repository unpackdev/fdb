package playground

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/types"
	"github.com/unpackdev/fdb/playground/suite"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// Config holds the configuration for the playground environment
type Config struct {
	BasePort  int
	NodeCount int
	LogLevel  zap.AtomicLevel
}

// Run initializes and runs the playground environment based on the provided CLI context and config
func Run(cliCtx *cli.Context, cfg Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use configuration from config parameter, overridden by CLI flags if provided
	basePort := cfg.BasePort
	if basePort == 0 {
		basePort = 9000
	}
	if cliCtx.IsSet("base-port") {
		basePort = cliCtx.Int("base-port")
	}

	nodeCount := cfg.NodeCount
	if nodeCount == 0 {
		nodeCount = 5
	}
	if cliCtx.IsSet("node-count") {
		nodeCount = cliCtx.Int("node-count")
	}

	logLevel := cfg.LogLevel.Level()
	if cliCtx.IsSet("debug") && cliCtx.Bool("debug") {
		logLevel = zap.DebugLevel
	}

	// Define the node roles
	roles := make([]types.Role, nodeCount)
	for i := range roles {
		roles[i] = rbac.RoleNode
	}

	lCfg := config.Logger{
		Enabled:     true,
		Environment: "development",
		Level:       logLevel.String(),
	}

	// Initialize the logger
	testLogger, err := logger.InitializeGlobalLogger(lCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Create the strategy registry
	registry := NewRegistry(testLogger)

	// Register all available strategies
	registry.RegisterWriteStrategy()

	// Initialize the nodes
	fmt.Printf("Initializing %d nodes with base port %d...\n", nodeCount, basePort)
	nodes, err := suite.InitializeNodes(
		ctx,
		testLogger,
		types.Ed25519SignerType,
		roles,
		basePort,
		types.TCPTransportType,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize nodes: %w", err)
	}

	// Ensure we clean up nodes when exiting
	defer func() {
		fmt.Println("Shutting down nodes...")
		if err := suite.ShutdownTestNodes(nodes); err != nil {
			fmt.Printf("Error shutting down nodes: %v\n", err)
		}
	}()

	// Print node information
	fmt.Println("Playground nodes initialized:")
	for i, node := range nodes {
		fmt.Printf("Node %d: PeerID=%s, Role=%s\n", i, node.PeerID().String(), node.Role())
	}

	// Wait for nodes to connect to each other
	fmt.Println("Waiting for nodes to connect to each other...")
	for i, node := range nodes {
		// Minus one because own peer needs to be excluded
		if err := node.WaitForPeersConnected(len(roles)-1, 10*time.Second); err != nil {
			fmt.Printf("Warning: Node %d didn't connect to all peers in time: %v\n", i, err)
		}
	}

	// Setup a signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Playground environment is ready!")

	// Check if a specific strategy is requested
	strategyName := cliCtx.String("strategy")
	if strategyName != "" {
		fmt.Printf("Running strategy: %s\n", strategyName)

		// Create strategy args from CLI flags
		args := make(map[string]any)

		// Common parameters
		if cliCtx.IsSet("workers") {
			args["workers"] = cliCtx.Int("workers")
		}

		// Strategy-specific parameters
		if strategyName == "write" {
			if cliCtx.IsSet("data-size") {
				args["data_size_kb"] = cliCtx.Int("data-size")
			}
			if cliCtx.IsSet("ops-per-sec") {
				args["ops_per_sec"] = cliCtx.Int("ops-per-sec")
			}
			if cliCtx.IsSet("total-ops") {
				args["total_ops"] = cliCtx.Int("total-ops")
			}
			if cliCtx.IsSet("target-node") {
				args["target_node"] = cliCtx.Int("target-node")
			}
		}

		// Use a separate goroutine to run the strategy
		go func() {
			strategyCtx, strategyCancel := context.WithCancel(ctx)
			defer strategyCancel()

			err := registry.RunStrategy(strategyCtx, strategyName, testLogger, nodes, args)
			if err != nil {
				fmt.Printf("Strategy execution failed: %v\n", err)
			}
		}()
	} else {
		fmt.Println("Press Ctrl+C to exit...")
	}

	// Wait for interrupt signal
	<-sigCh
	fmt.Println("Received interrupt signal, shutting down playground...")
	return nil
}
