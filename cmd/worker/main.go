package main

import (
	"os"
)

// main entry point for the iwf worker
func main() {
	app := BuildCLI()
	app.Run(os.Args)
}
