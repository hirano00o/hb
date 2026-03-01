// Command hb is the Hatena Blog management CLI.
package main

import (
	"os"

	"github.com/hirano00o/hb/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
