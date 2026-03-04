package cli

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <file>",
		Short: "Sync the remote version of an entry to the local file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path := args[0]
			local, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if local.Frontmatter.EditURL == "" {
				return fmt.Errorf("%s has no editUrl in frontmatter; use 'hb pull' first", path)
			}

			client, err := newClientFromConfig()
			if err != nil {
				return err
			}
			remote, err := client.GetEntry(ctx, local.Frontmatter.EditURL)
			if err != nil {
				return err
			}

			remoteArticle := article.FromEntry(remote)
			diff, err := unifiedDiff(path, local, remoteArticle)
			if err != nil {
				return err
			}
			if diff == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), diff)

			ok, err := confirmAction(cmd, "Overwrite local file? [y/N]: ")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
			if err := article.Write(path, remoteArticle); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", path)
			return nil
		},
	}
}

// globMD returns all .md files under root (recursively).
func globMD(root string) ([]string, error) {
	if root == "" {
		root = "."
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// articleToString renders an Article to its file content string for diffing.
func articleToString(a *article.Article) (string, error) {
	header, err := article.RenderFrontmatter(&a.Frontmatter)
	if err != nil {
		return "", fmt.Errorf("render frontmatter: %w", err)
	}
	return header + a.Body, nil
}
