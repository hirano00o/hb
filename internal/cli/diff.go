package cli

import (
	"errors"
	"fmt"

	"github.com/hirano00o/hb/article"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "diff [<file> ...]",
		Short: "Show unified diff between local file(s) and remote entry",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if all && len(args) > 0 {
				return fmt.Errorf("--all and file arguments are mutually exclusive")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("at least one file argument is required, or use --all")
			}

			paths := args
			if all {
				var err error
				paths, err = globMD(".")
				if err != nil {
					return fmt.Errorf("glob: %w", err)
				}
			}

			client, err := newClientFromConfig()
			if err != nil {
				return err
			}

			var errs []error
			for _, path := range paths {
				local, err := article.Read(path)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: read: %w", path, err))
					continue
				}
				if local.Frontmatter.EditURL == "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: No editUrl in frontmatter; this is a new (unpublished) entry.\n", path)
					continue
				}

				remote, err := client.GetEntry(ctx, local.Frontmatter.EditURL)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", path, err))
					continue
				}
				remoteArticle := article.FromEntry(remote)
				if article.HasLocalImages(local.Body) {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: %s contains local images; image lines may appear as differences until pushed\n", path)
				}
				diff, err := unifiedDiff(path, local, remoteArticle)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", path, err))
					continue
				}
				if diff == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "No differences.")
					continue
				}
				fmt.Fprint(cmd.OutOrStdout(), diff)
			}
			if len(errs) > 0 {
				return errors.Join(errs...)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Diff all .md files under the current directory")
	return cmd
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
