package main

import (
	"github.com/unpackdev/fdb/cmd"
	"github.com/urfave/cli/v2"
	"log"
	"os"
)

func main() {
	app := &cli.App{
		Name:  "(f)db",
		Usage: "Fast Database Transports",
		Commands: []*cli.Command{
			cmd.CertsCommand(),     // Command for handling certificates
			cmd.BenchmarkCommand(), // Command for running benchmarks
			cmd.ServeCommand(),     // Command to start the server
		},
	}

	// Run the app and handle any errors
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Error running CLI: %v", err)
	}
}
