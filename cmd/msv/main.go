package main

import (
	"fmt"
	"os"

	"github.com/aptx-health/ms-visualizer/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitRuntimeError)
	}
}
