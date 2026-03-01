package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hirano00o/hb/article"
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
			client, err := newClientFromConfig()
			if err != nil {
				return err
			}
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

				destPath, skip, err := resolveConflict(cmd, path, force)
				if err != nil {
					return err
				}
				if skip {
					fmt.Fprintf(cmd.OutOrStdout(), "skipped: %s\n", path)
					continue
				}
				if err := article.Write(destPath, a); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "saved: %s\n", destPath)
				saved++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d entries saved.\n", saved)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "On filename conflict, auto-rename with millisecond suffix instead of prompting")
	cmd.Flags().StringVar(&dir, "dir", "", "Directory to save files (default: current directory)")
	return cmd
}

// resolveConflict checks if path already exists and, if so, determines the destination path.
// When force is true, an automatic millisecond suffix is applied without prompting.
// Returns the resolved destination path, a skip flag, and any error.
func resolveConflict(cmd *cobra.Command, path string, force bool) (dest string, skip bool, err error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, false, nil
	}

	if force {
		return autoRename(path), false, nil
	}

	// Interactive: ask to rename or skip.
	fmt.Fprintf(cmd.OutOrStdout(), "File already exists: %s\n", path)
	fmt.Fprint(cmd.OutOrStdout(), "Enter new filename to rename (leave empty to auto-rename), or 's' to skip: ")

	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", false, err
		}
		// EOF → auto-rename
		return autoRename(path), false, nil
	}
	input := strings.TrimSpace(scanner.Text())

	if strings.EqualFold(input, "s") {
		return "", true, nil
	}
	if input == "" {
		return autoRename(path), false, nil
	}
	// User provided a custom name; place it in the same directory.
	return filepath.Join(filepath.Dir(path), input), false, nil
}

// autoRename appends the current milliseconds (up to 4 digits) as a suffix to the base name.
// e.g. "20260301_Title.md" → "20260301_Title_123.md"
func autoRename(path string) string {
	ms := time.Now().UnixMilli() % 10000
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)
	return filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, ms, ext))
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
