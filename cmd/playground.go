package cmd

import (
	"fmt"

	"github.com/unpackdev/fdb/playground"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	DefaultBasePort  = 50000
	DefaultNodeCount = 5
)

// PlaygroundCommand returns a cli.Command that plays with the real clients over simulated network
// commonFlags returns the common flags used by all playground commands
func commonFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:    "base-port",
			Aliases: []string{"p"},
			Value:   DefaultBasePort,
			Usage:   "Base port for the nodes (each node will use this port + index)",
		},
		&cli.IntFlag{
			Name:    "node-count",
			Aliases: []string{"n"},
			Value:   DefaultNodeCount,
			Usage:   "Number of nodes to start in the playground",
		},
		&cli.StringFlag{
			Name:    "log-level",
			Aliases: []string{"l"},
			Value:   "debug",
			Usage:   "Log level (debug, info, warn, error)",
		},
	}
}

// writeStrategyFlags returns the flags specific to the write strategy
func writeStrategyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:  "workers",
			Value: 10,
			Usage: "Number of concurrent workers for write operations (optimized with XOR key distribution)",
		},
		&cli.IntFlag{
			Name:  "data-size",
			Value: 10,
			Usage: "Size of data to write in KB (uses optimized TCP streaming with 128KB buffer)",
		},
		&cli.IntFlag{
			Name:  "ops-per-sec",
			Value: 100,
			Usage: "Target operations per second across all workers",
		},
		&cli.IntFlag{
			Name:  "total-ops",
			Value: 1000,
			Usage: "Total number of operations to perform",
		},
		&cli.IntFlag{
			Name:  "target-node",
			Value: 0,
			Usage: "Index of the node to send writes to (default 0 = first node)",
		},
	}
}

func PlaygroundCommand() *cli.Command {
	return &cli.Command{
		Name:  "playground",
		Usage: "Play with (f)db clients over simulated network",
		Subcommands: []*cli.Command{
			{
				Name:  "network",
				Usage: "Run just the network without any specific test strategy",
				Flags: commonFlags(),
				Action: func(c *cli.Context) error {
					config := playground.Config{
						BasePort:  DefaultBasePort,
						NodeCount: DefaultNodeCount,
						LogLevel:  zap.NewAtomicLevelAt(zap.DebugLevel),
					}

					if c.IsSet("log-level") {
						logLevelStr := c.String("log-level")
						level, err := zap.ParseAtomicLevel(logLevelStr)
						if err != nil {
							return fmt.Errorf("invalid log level: %w", err)
						}

						config.LogLevel = level
					}

					// Run the playground with both the context and config
					return playground.Run(c, config)
				},
			},
			{
				Name:        "write",
				Usage:       "Run a write performance test",
				Description: "Tests the P2P network database write and data transfer performance",
				Flags:       append(commonFlags(), writeStrategyFlags()...),
				Action: func(c *cli.Context) error {
					config := playground.Config{
						BasePort:  DefaultBasePort,
						NodeCount: DefaultNodeCount,
						LogLevel:  zap.NewAtomicLevelAt(zap.DebugLevel),
					}

					if c.IsSet("log-level") {
						logLevelStr := c.String("log-level")
						level, err := zap.ParseAtomicLevel(logLevelStr)
						if err != nil {
							return fmt.Errorf("invalid log level: %w", err)
						}

						config.LogLevel = level
					}

					// Pass the strategy name via a flag
					c.Set("strategy", "write")

					// Run the playground with the write strategy
					return playground.Run(c, config)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "base-port",
				Aliases: []string{"p"},
				Value:   DefaultBasePort,
				Usage:   "Base port for the nodes (each node will use this port + index)",
			},
			&cli.IntFlag{
				Name:    "node-count",
				Aliases: []string{"n"},
				Value:   DefaultNodeCount,
				Usage:   "Number of nodes to start in the playground",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "debug",
				Usage:   "Log level (debug, info, warn, error)",
			},
		},
		Action: func(c *cli.Context) error {
			// By default, show help when no subcommand is specified
			cli.ShowAppHelp(c)
			return nil
		},
	}
}
