package main

import (
	"fmt"
	"os"

	"github.com/penwyp/ClawCat/cmd"
)

// Build information set by linker
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Print to stderr directly for fatal errors at startup
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
