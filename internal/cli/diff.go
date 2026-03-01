package cli

import (
	"fmt"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <file>",
		Short: "Show unified diff between local file and remote entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path := args[0]
			local, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if local.Frontmatter.EditURL == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No editUrl in frontmatter; this is a new (unpublished) entry.")
				return nil
			}

			cfg, err := config.LoadMerged()
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("config: %w", err)
			}

			client := hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey)
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
				fmt.Fprintln(cmd.OutOrStdout(), "No differences.")
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), diff)
			return nil
		},
	}
}

// unifiedDiff returns a unified diff string comparing local to remote article content.
func unifiedDiff(path string, local, remote *article.Article) (string, error) {
	localStr, err := articleToString(local)
	if err != nil {
		return "", err
	}
	remoteStr, err := articleToString(remote)
	if err != nil {
		return "", err
	}
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(localStr),
		B:        difflib.SplitLines(remoteStr),
		FromFile: path + " (local)",
		ToFile:   path + " (remote)",
		Context:  3,
	})
	if err != nil {
		return "", fmt.Errorf("diff generation: %w", err)
	}
	return diff, nil
}
