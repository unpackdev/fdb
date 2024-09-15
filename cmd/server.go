package cmd

import (
	"github.com/urfave/cli/v2"
)

// ServerCommand returns a cli.Command that benchmarks the real client
func ServerCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "Manage (f)db server",
		Action: func(c *cli.Context) error {
			return nil
		},
	}
}
