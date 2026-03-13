// Package cli wires up all hb subcommands using cobra.
package cli

import "github.com/spf13/cobra"

const version = "v0.1.0"

// NewRootCmd builds and returns the root cobra command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "hb",
		Short:   "Hatena Blog management CLI",
		Version: version,
	}

	root.PersistentFlags().BoolP("verbose", "v", false, "Show verbose output including skipped-file warnings")

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
	root.AddCommand(newNewCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newOpenCmd())
	root.AddCommand(newStatusCmd())

	return root
}
