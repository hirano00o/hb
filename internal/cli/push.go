package cli

import (
	"fmt"
	"slices"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <file>",
		Short: "Push a local file to Hatena Blog (POST if new, PUT if updated)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			local, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}

			cfg, err := config.LoadMerged()
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("config: %w", err)
			}

			client := hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey)

			// No editUrl → new entry, POST
			if local.Frontmatter.EditURL == "" {
				created, err := client.CreateEntry(local.ToEntry())
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
			remote, err := client.GetEntry(local.Frontmatter.EditURL)
			if err != nil {
				return err
			}
			remoteArticle := article.FromEntry(remote)

			if !hasChanges(local, remoteArticle) {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes to push.")
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'hb diff %s' to review differences.\n", path)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tip: run 'hb diff %s' to review changes before pushing.\n", path)

			updated, err := client.UpdateEntry(local.Frontmatter.EditURL, local.ToEntry())
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
