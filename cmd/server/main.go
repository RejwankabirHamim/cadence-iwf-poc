package main

import (
	"github.com/RejwankabirHamim/cadence-iwf-poc/cmd/server/iwf"
	"os"
)

// main entry point for the iwf server
func main() {
	app := iwf.BuildCLI()
	app.Run(os.Args)
}
