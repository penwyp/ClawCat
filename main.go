package main

import (
	"github.com/penwyp/ClawCat/cmd"
	"log"
	"os"
)

// Build information set by linker
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}