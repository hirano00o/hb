package cli

import (
	"errors"
	"fmt"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "diff [<file> ...]",
		Short: "Show unified diff between local file(s) and remote entry",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := resolveTargetPaths(args, all)
			if err != nil {
				return err
			}
			client, _, err := newClientFromConfig()
			if err != nil {
				return err
			}
			return runDiff(cmd, client, paths)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Diff all .md files under the current directory")
	return cmd
}

func runDiff(cmd *cobra.Command, client *hatena.Client, paths []string) error {
	ctx := cmd.Context()

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
			fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
			continue
		}
		fmt.Fprint(cmd.OutOrStdout(), diff)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
