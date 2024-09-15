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
		Usage: "Lorem ipsum dolor sit amet...",
		Commands: []*cli.Command{
			// Load commands from the cmd package
			cmd.ServerCommand(),
			cmd.BenchmarkCommand(), // Load the 'test' command
		},
	}

	// Run the app and handle any errors
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("Error running CLI: %v", err)
	}
}
