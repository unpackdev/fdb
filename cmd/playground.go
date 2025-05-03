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
func PlaygroundCommand() *cli.Command {
	return &cli.Command{
		Name:  "playground",
		Usage: "Play with (f)db clients over simulated network",
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
			// Create a config with defaults that can be overridden by CLI flags
			config := playground.Config{
				BasePort:  DefaultBasePort,
				NodeCount: DefaultNodeCount,
				LogLevel:  zap.NewAtomicLevelAt(zap.DebugLevel),
			}

			// Parse log level if provided
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
	}
}
