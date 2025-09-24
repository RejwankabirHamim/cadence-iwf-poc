package main

import (
	"log"
	"os"
)

func main() {
	app := BuildCApiCLI()
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Failed to run CLI: %v", err)
	}
}
