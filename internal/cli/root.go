// Package cli wires up all hb subcommands using cobra.
package cli

import "github.com/spf13/cobra"

// version is set at build time via -ldflags "-X github.com/hirano00o/hb/internal/cli.version=<ver>".
// It defaults to "dev" for local builds.
var version = "dev"

// NewRootCmd builds and returns the root cobra command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "hb",
		Short:   "Hatena Blog management CLI",
		Version: version,
	}

	root.PersistentFlags().Bool("verbose", false, "Show verbose output including skipped-file warnings")

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage hb configuration",
	}
	configCmd.AddCommand(newConfigInitCmd())
	configCmd.AddCommand(newConfigShowCmd())
	root.AddCommand(configCmd)

	root.AddCommand(newPullCmd())
	root.AddCommand(newSyncCmd())
	root.AddCommand(newPushCmd())
	root.AddCommand(newDiffCmd())
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newNewCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newOpenCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newPublishCmd())
	root.AddCommand(newUnpublishCmd())
	root.AddCommand(newRenameCmd())

	return root
}
