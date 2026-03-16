package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const defaultConcurrency = 5

func newPullCmd() *cobra.Command {
	var force bool
	var dir string
	var fromStr, toStr string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull all remote entries to local Markdown files",
		RunE: func(cmd *cobra.Command, args []string) error {
			var from, to time.Time
			if fromStr != "" {
				t, err := parseFilterDate(fromStr)
				if err != nil {
					return fmt.Errorf("--from: %w", err)
				}
				from = t
			}
			if toStr != "" {
				t, err := parseFilterDate(toStr)
				if err != nil {
					return fmt.Errorf("--to: %w", err)
				}
				to = t
			}

			cfg, err := config.LoadMerged()
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				return fmt.Errorf("config: %w", err)
			}
			concurrency := defaultConcurrency
			if cfg.Concurrency != nil && *cfg.Concurrency > 0 {
				concurrency = *cfg.Concurrency
			}
			maxPages := 0
			if cfg.MaxPages != nil {
				maxPages = *cfg.MaxPages
			}

			client := hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey)
			v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
			return runPull(cmd, client, dir, force, from, to, concurrency, maxPages, v)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "On filename conflict, auto-rename with millisecond suffix instead of prompting")
	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to save files")
	cmd.Flags().StringVar(&fromStr, "from", "", "Filter entries published on or after this date (YYYY-mm-dd, YYYY/mm/dd, or YYYYmmdd)")
	cmd.Flags().StringVar(&toStr, "to", "", "Filter entries published on or before this date (YYYY-mm-dd, YYYY/mm/dd, or YYYYmmdd)")
	return cmd
}

func runPull(cmd *cobra.Command, client *hatena.Client, dir string, force bool, from, to time.Time, concurrency, maxPages int, showWarnings bool) error {
	ctx := cmd.Context()

	entries, err := client.ListEntries(ctx, maxPages)
	if err != nil {
		return err
	}

	entries = filterEntriesByDate(entries, from, to)

	// Build a set of known editUrls from local files to skip already-fetched entries.
	knownEditURLs, err := collectLocalEditURLs(dir, cmd.ErrOrStderr(), showWarnings)
	if err != nil {
		return err
	}

	// Filter out already-known entries before parallel processing.
	toProcess := make([]*hatena.Entry, 0, len(entries))
	for _, e := range entries {
		if _, exists := knownEditURLs[e.EditURL]; exists {
			continue
		}
		toProcess = append(toProcess, e)
	}

	var (
		saved      atomic.Int64
		interactMu sync.Mutex
	)

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrency)

	for _, e := range toProcess {
		e := e
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			a := article.FromEntry(e)
			filename := article.GenerateFilename(e.Title, e.Date, e.Draft)
			path := filepath.Join(dir, filename)

			// Hold the lock across resolveConflict AND article.Write so that
			// two goroutines resolving to the same destination path cannot
			// both pass the conflict check before either has written the file
			// (TOCTOU race).
			interactMu.Lock()
			destPath, skip, err := resolveConflict(cmd, path, force)
			if err != nil {
				interactMu.Unlock()
				return err
			}
			if skip {
				fmt.Fprintf(cmd.OutOrStdout(), "skipped: %s\n", path)
				interactMu.Unlock()
				return nil
			}
			if err := article.Write(destPath, a); err != nil {
				interactMu.Unlock()
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved: %s\n", destPath)
			interactMu.Unlock()
			saved.Add(1)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%d entries saved.\n", saved.Load())
	return nil
}

// parseFilterDate parses a date string in YYYY-mm-dd, YYYY/mm/dd, or YYYYmmdd format.
// Comparison uses year/month/day only; time and timezone are ignored.
func parseFilterDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "20060102"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, errors.New("invalid date format: use YYYY-mm-dd, YYYY/mm/dd, or YYYYmmdd")
}

// filterEntriesByDate returns entries whose Date falls within [from, to] (inclusive).
// Zero-value from or to means no lower/upper bound respectively.
func filterEntriesByDate(entries []*hatena.Entry, from, to time.Time) []*hatena.Entry {
	if from.IsZero() && to.IsZero() {
		return entries
	}
	result := make([]*hatena.Entry, 0, len(entries))
	for _, e := range entries {
		d := time.Date(e.Date.Year(), e.Date.Month(), e.Date.Day(), 0, 0, 0, 0, time.UTC)
		if !from.IsZero() && d.Before(from) {
			continue
		}
		if !to.IsZero() && d.After(to) {
			continue
		}
		result = append(result, e)
	}
	return result
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
	// Use only the base name to prevent path traversal (e.g. "../../etc/passwd").
	return filepath.Join(filepath.Dir(path), filepath.Base(input)), false, nil
}

// autoRename generates a path that does not yet exist by appending an incrementing
// counter suffix to the base name.
// e.g. "20260301_Title.md" → "20260301_Title_1.md", "_2.md", …
func autoRename(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// collectLocalEditURLs walks the given directory recursively and collects all editUrl values
// found in frontmatter of .md files. Unreadable files are skipped; when showWarnings is true
// a per-file warning is written to w, otherwise a summary count is emitted instead.
func collectLocalEditURLs(dir string, w io.Writer, showWarnings bool) (map[string]struct{}, error) {
	known := map[string]struct{}{}
	files, err := globMD(dir)
	if err != nil {
		return known, err
	}
	var readErrCount int
	for _, f := range files {
		a, err := article.Read(f)
		if err != nil {
			readErrCount++
			if showWarnings {
				fmt.Fprintf(w, "warning: failed to read %s: %v (skipping)\n", f, err)
			}
			continue
		}
		if a.Frontmatter.EditURL != "" {
			known[a.Frontmatter.EditURL] = struct{}{}
		}
	}
	if readErrCount > 0 && !showWarnings {
		fmt.Fprintf(w, "warning: %d file(s) skipped due to read errors (use --verbose for details)\n", readErrCount)
	}
	return known, nil
}
