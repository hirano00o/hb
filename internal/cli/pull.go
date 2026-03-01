package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var force bool
	var dir string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull all remote entries to local Markdown files",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := config.LoadMerged()
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("config: %w", err)
			}

			client := hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey)
			entries, err := client.ListEntries(ctx)
			if err != nil {
				return err
			}

			// Build a set of known editUrls from local files to skip already-fetched entries.
			knownEditURLs := map[string]struct{}{}
			if !force {
				knownEditURLs, err = collectLocalEditURLs(dir, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}

			saved := 0
			for _, e := range entries {
				if !force {
					if _, exists := knownEditURLs[e.EditURL]; exists {
						continue
					}
				}
				a := article.FromEntry(e)
				filename := article.GenerateFilename(e.Title, e.Date, e.Draft)
				path := filepath.Join(dir, filename)
				if err := article.Write(path, a); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "saved: %s\n", path)
				saved++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d entries saved.\n", saved)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files")
	cmd.Flags().StringVar(&dir, "dir", "", "Directory to save files (default: current directory)")
	return cmd
}

// collectLocalEditURLs walks the given directory recursively and collects all editUrl values
// found in frontmatter of .md files. Unreadable files are skipped with a warning to w.
func collectLocalEditURLs(dir string, w io.Writer) (map[string]struct{}, error) {
	known := map[string]struct{}{}
	files, err := globMD(dir)
	if err != nil {
		return known, err
	}
	for _, f := range files {
		a, err := article.Read(f)
		if err != nil {
			fmt.Fprintf(w, "warning: skipping %s: %v\n", f, err)
			continue
		}
		if a.Frontmatter.EditURL != "" {
			known[a.Frontmatter.EditURL] = struct{}{}
		}
	}
	return known, nil
}
