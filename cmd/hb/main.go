// Command hb is the Hatena Blog management CLI.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hirano00o/hb/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root := cli.NewRootCmd()
	root.SetContext(ctx)
	if err := root.Execute(); err != nil {
		if ctx.Err() != nil {
			// SIGINT/SIGTERM: exit 130 without printing error (context cancellation is expected)
			os.Exit(130)
		}
		os.Exit(1)
	}
}
