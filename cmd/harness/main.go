package main

import (
	"fmt"
	"os"

	"github.com/ittakestwo123/Harnesslab/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
