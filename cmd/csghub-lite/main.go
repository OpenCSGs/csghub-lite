package main

import (
	"context"
	"fmt"
	"os"

	"github.com/opencsgs/csghub-lite/internal/cli"
)

var version = "dev"

func main() {
	ctx := context.Background()
	if err := cli.NewRootCmd(version).ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
