package cli

import (
	"fmt"
	"os"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var yes bool
	var removeLocal bool

	cmd := &cobra.Command{
		Use:   "delete <file>",
		Short: "Delete a remote Hatena Blog entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path := args[0]
			local, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if local.Frontmatter.EditURL == "" {
				return fmt.Errorf("%s has no edit_url; push it first", path)
			}

			client, err := newClientFromConfig()
			if err != nil {
				return err
			}

			if !yes {
				ok, err := confirmAction(cmd, fmt.Sprintf("Delete remote entry %s? [y/N]: ", local.Frontmatter.URL))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			if err := client.DeleteEntry(ctx, local.Frontmatter.EditURL); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted: %s\n", local.Frontmatter.URL)

			if removeLocal {
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("remove local file: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed local: %s\n", path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&removeLocal, "remove-local", false, "Also remove the local file after deletion")
	return cmd
}
