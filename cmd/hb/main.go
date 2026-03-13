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
		os.Exit(1)
	}
}
