package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jethros-projects/keep-clam-cli/internal/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Args, cli.NewEnv(os.Stdout, os.Stderr)); err != nil {
		fmt.Fprintf(os.Stderr, "clam: %v\n", err)
		os.Exit(1)
	}
}
