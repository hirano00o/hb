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
			// Interrupted by SIGINT or SIGTERM; exit 130 (128+SIGINT) as a
			// conventional non-zero exit code for signal-driven termination.
			// signal.NotifyContext does not expose the specific signal received,
			// so we use 130 for both. This is acceptable for an interactive CLI.
			os.Exit(130)
		}
		os.Exit(1)
	}
}
