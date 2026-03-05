// Package cli wires up all hb subcommands using cobra.
package cli

import "github.com/spf13/cobra"

// NewRootCmd builds and returns the root cobra command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hb",
		Short: "Hatena Blog management CLI",
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage hb configuration",
	}
	configCmd.AddCommand(newConfigInitCmd())
	root.AddCommand(configCmd)

	root.AddCommand(newPullCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newPushCmd())
	root.AddCommand(newDiffCmd())

	return root
}
