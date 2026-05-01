package main

import (
	"context"
	"fmt"
	"os"

	"github.com/opencsgs/csghub-lite/internal/cli"
	"github.com/opencsgs/csghub-lite/internal/upgrade"
)

var version = "dev"

func main() {
	// Set version for upgrade module
	upgrade.SetVersion(version)

	ctx := context.Background()
	if err := cli.NewRootCmd(version).ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
