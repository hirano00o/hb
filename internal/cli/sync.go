package cli

import (
	"errors"
	"fmt"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var yes bool
	var all bool

	cmd := &cobra.Command{
		Use:   "sync [<file> ...]",
		Short: "Sync the remote version of an entry to the local file",
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
			return runSync(cmd, client, paths, yes)
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&all, "all", false, "Sync all .md files under the current directory")
	return cmd
}

func runSync(cmd *cobra.Command, client *hatena.Client, paths []string, yes bool) error {
	ctx := cmd.Context()

	var errs []error
	for _, path := range paths {
		local, err := article.Read(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: read: %w", path, err))
			continue
		}
		if local.Frontmatter.EditURL == "" {
			// --all: skip entries without editUrl and collect as error
			errs = append(errs, fmt.Errorf("%s: editUrl is missing from frontmatter", path))
			continue
		}

		remote, err := client.GetEntry(ctx, local.Frontmatter.EditURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}

		remoteArticle := article.FromEntry(remote)
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

		if !yes {
			ok, err := confirmAction(cmd, "Overwrite local file? [y/N]: ")
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
				continue
			}
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				continue
			}
		}
		if err := article.Write(path, remoteArticle); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", path)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
