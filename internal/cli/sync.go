package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hirano00o/hb/article"
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
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&all, "all", false, "Sync all .md files under the current directory")
	return cmd
}

// globMD returns all .md files under root (recursively), skipping hidden directories.
func globMD(root string) ([]string, error) {
	if root == "" {
		root = "."
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			return fs.SkipDir
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
