package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <file>",
		Short: "Fetch the remote version of an entry and overwrite the local file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			local, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if local.Frontmatter.EditURL == "" {
				return fmt.Errorf("%s has no editUrl in frontmatter; use 'hb pull' first", path)
			}

			cfg, err := config.LoadMerged()
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("config: %w", err)
			}

			client := hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey)
			remote, err := client.GetEntry(local.Frontmatter.EditURL)
			if err != nil {
				return err
			}

			remoteArticle := article.FromEntry(remote)
			diff := unifiedDiff(path, local, remoteArticle)
			if diff == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), diff)
			fmt.Fprint(cmd.OutOrStdout(), "Overwrite local file? [y/N]: ")

			scanner := bufio.NewScanner(cmd.InOrStdin())
			if !scanner.Scan() {
				return nil
			}
			if !strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
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
func articleToString(a *article.Article) string {
	header, err := article.RenderFrontmatter(&a.Frontmatter)
	if err != nil {
		return a.Body
	}
	return header + a.Body
}
