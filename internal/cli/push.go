package cli

import (
	"fmt"
	"slices"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "push <file>",
		Short: "Push a local file to Hatena Blog (POST if new, PUT if updated)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path := args[0]
			local, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}

			client, err := newClientFromConfig()
			if err != nil {
				return err
			}

			// No editUrl → new entry, POST
			if local.Frontmatter.EditURL == "" {
				created, err := client.CreateEntry(ctx, local.ToEntry())
				if err != nil {
					return err
				}
				// Update local file with the assigned editUrl and url
				local.Frontmatter.EditURL = created.EditURL
				local.Frontmatter.URL = created.URL
				if err := article.Write(path, local); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", created.URL)
				return nil
			}

			// Has editUrl → fetch remote and compare
			remote, err := client.GetEntry(ctx, local.Frontmatter.EditURL)
			if err != nil {
				return err
			}
			remoteArticle := article.FromEntry(remote)

			if !hasChanges(local, remoteArticle) {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes to push.")
				return nil
			}

			diff, err := unifiedDiff(path, remoteArticle, local)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), diff)

			if !yes {
				ok, err := confirmAction(cmd, "Push these changes? [y/N]: ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			updated, err := client.UpdateEntry(ctx, local.Frontmatter.EditURL, local.ToEntry())
			if err != nil {
				return err
			}
			local.Frontmatter.URL = updated.URL
			if err := article.Write(path, local); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", updated.URL)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

// hasChanges returns true if the local article differs from the remote in any field.
func hasChanges(local, remote *article.Article) bool {
	lf, rf := local.Frontmatter, remote.Frontmatter
	return local.Body != remote.Body ||
		!lf.Date.Equal(rf.Date) ||
		lf.Title != rf.Title ||
		lf.Draft != rf.Draft ||
		!slices.Equal(lf.Category, rf.Category) ||
		lf.CustomURLPath != rf.CustomURLPath
}
