package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/shutdown"
	"github.com/unpackdev/fdb/pkg/types"
	"github.com/unpackdev/fdb/playground/registry"
	"github.com/unpackdev/fdb/playground/strategies"
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

	// Initialize the shutdown manager
	shutdownMgr := shutdown.NewManager(ctx, testLogger)
	shutdownMgr.Start()

	// Create the strategy registry
	registry := registry.New(testLogger)

	// Register all available strategies
	registry.RegisterAll()

	// Initialize the nodes
	fmt.Printf("Initializing %d nodes with base port %d...\n", nodeCount, basePort)
	nodes, err := suite.InitializeNodes(
		shutdownMgr.Context(), // Use the shutdown manager's context
		testLogger,
		types.Ed25519SignerType,
		roles,
		basePort,
		types.TCPTransportType,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize nodes: %w", err)
	}

	// Register node shutdown with the shutdown manager
	shutdownMgr.AddShutdownCallback(func() error {
		testLogger.Info("Shutting down nodes...")
		return suite.ShutdownTestNodes(nodes)
	})

	testLogger.Info("Playground nodes initialized:")
	for i, node := range nodes {
		testLogger.Info("Node initialized", "index", i, "peer_id", node.PeerID().String(), "role", node.Role())
	}

	// Wait for nodes to connect to each other
	testLogger.Info("Waiting for nodes to connect to each other...")
	for i, node := range nodes {
		// Minus one because own peer needs to be excluded
		if err := node.WaitForPeersConnected(len(roles)-1, 10*time.Second); err != nil {
			testLogger.Warn("Node didn't connect to all peers in time", "index", i, "error", err)
		}
	}

	testLogger.Info("Playground environment is ready!")

	// Check if a specific strategy is requested either via flag or command name
	strategyName := cliCtx.String("strategy")

	// Also check the command name if strategy is not specified directly
	if strategyName == "" && cliCtx.Command != nil {
		// If the command is a subcommand like "write", use that as the strategy name
		if cliCtx.Command.Name != "playground" {
			strategyName = cliCtx.Command.Name
		}
	}

	if strategyName != "" {
		testLogger.Info("Running strategy", "strategy", strategyName)

		// Get info about the strategy
		strategyInfo, found := registry.GetStrategy(strategyName)
		if !found {
			return fmt.Errorf("strategy '%s' not found in registry", strategyName)
		}

		// Parse CLI arguments using the strategy's argument mappings
		args := strategies.ParseCliArgs(cliCtx, strategyInfo)

		// Create a channel to signal when the strategy is done
		strategyCh := make(chan error, 1)

		// Use a separate goroutine to run the strategy
		go func() {
			strategyCtx, strategyCancel := context.WithCancel(shutdownMgr.Context())
			defer strategyCancel()

			// Run the strategy and send the result to the channel
			err := registry.RunStrategy(strategyCtx, strategyName, testLogger, nodes, args)
			strategyCh <- err
		}()

		// Wait for strategy completion in another goroutine
		go func() {
			// When strategy completes, trigger shutdown
			err := <-strategyCh
			if err != nil {
				testLogger.Error("Strategy execution failed", "error", err.Error())
				shutdownMgr.Trigger(fmt.Sprintf("strategy failed: %v", err))
			} else {
				testLogger.Info("Strategy completed successfully")
				shutdownMgr.Trigger("strategy completed successfully")
			}
		}()
	} else {
		testLogger.Info("Press Ctrl+C to exit...")
	}

	// Wait for the shutdown process to complete
	if err := shutdownMgr.Wait(); err != nil {
		testLogger.Error("Shutdown completed with errors", "error", err.Error())
	}

	testLogger.Info("Playground shutdown complete.", "cause", shutdownMgr.ShutdownCause())
	return nil
}
