package cli

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
)

const (
	defaultConcurrency = 5
	defaultTimeoutSec  = 30
)

// newClientFromConfig loads and validates configuration, then returns a new API
// client along with the loaded config so callers can read further settings
// (e.g. Concurrency, MaxPages) without a second load.
// It is a package-level variable so tests can inject a stub client.
var newClientFromConfig = defaultNewClientFromConfig

func defaultNewClientFromConfig() (*hatena.Client, *config.Config, error) {
	cfg, err := config.LoadMerged()
	if err != nil {
		return nil, nil, err
	}
	if err := config.Validate(cfg); err != nil {
		return nil, nil, fmt.Errorf("config: %w", err)
	}
	timeoutSec := defaultTimeoutSec
	if cfg.TimeoutSec != nil {
		timeoutSec = *cfg.TimeoutSec
	}
	return hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey, timeoutSec), cfg, nil
}

// confirmAction prints prompt and reads a y/Y response from stdin.
// Returns true when the user confirms, false otherwise (including EOF).
func confirmAction(cmd *cobra.Command, prompt string) (bool, error) {
	return confirmActionWithScanner(cmd, bufio.NewScanner(cmd.InOrStdin()), prompt)
}

// confirmActionWithScanner is like confirmAction but uses a caller-provided scanner,
// allowing multiple stdin reads to share the same buffered scanner.
func confirmActionWithScanner(cmd *cobra.Command, scanner *bufio.Scanner, prompt string) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y"), nil
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

// resolveTargetPaths returns the target files for commands accepting file args or --all.
func resolveTargetPaths(args []string, all bool) ([]string, error) {
	if all && len(args) > 0 {
		return nil, fmt.Errorf("--all and file arguments are mutually exclusive")
	}
	if !all && len(args) == 0 {
		return nil, fmt.Errorf("at least one file argument is required, or use --all")
	}
	if all {
		paths, err := globMD(".")
		if err != nil {
			return nil, fmt.Errorf("glob: %w", err)
		}
		return paths, nil
	}
	return args, nil
}

// localArticle pairs a local Markdown file path with its parsed article.
type localArticle struct {
	path string
	art  *article.Article
}

// loadArticles reads every .md file under dir and returns the articles that have
// frontmatter. Unreadable files are skipped with a per-file warning on errOut when
// verbose, or a single summary warning otherwise; frontmatter-less files are skipped
// with a warning when verbose.
func loadArticles(dir string, verbose bool, errOut io.Writer) ([]localArticle, error) {
	return scanArticles(dir, verbose, errOut, true)
}

// scanArticles is the loadArticles core. skipNoFrontmatter is false only for pull,
// which collects editUrls from every readable file and neither warns about nor
// drops frontmatter-less files.
func scanArticles(dir string, verbose bool, errOut io.Writer, skipNoFrontmatter bool) ([]localArticle, error) {
	files, err := globMD(dir)
	if err != nil {
		return nil, err
	}
	var arts []localArticle
	var readErrCount int
	for _, f := range files {
		a, err := article.Read(f)
		if err != nil {
			readErrCount++
			if verbose {
				fmt.Fprintf(errOut, "warning: failed to read %s: %v (skipping)\n", f, err)
			}
			continue
		}
		if skipNoFrontmatter && a.Frontmatter.Title == "" && a.Frontmatter.Date.IsZero() {
			if verbose {
				fmt.Fprintf(errOut, "warning: skipping %s: no frontmatter\n", f)
			}
			continue
		}
		arts = append(arts, localArticle{path: f, art: a})
	}
	if readErrCount > 0 && !verbose {
		fmt.Fprintf(errOut, "warning: %d file(s) skipped due to read errors (use --verbose for details)\n", readErrCount)
	}
	return arts, nil
}

// hasChanges returns true if the local article differs from the remote in any field.
func hasChanges(local, remote *article.Article) bool {
	lf, rf := local.Frontmatter, remote.Frontmatter
	// A scheduled entry is stored as draft=yes on the API side regardless of
	// the local draft field (see pushOne), so compare draft under the same
	// normalization; otherwise a scheduled post reports a permanent diff.
	lDraft, rDraft := lf.Draft, rf.Draft
	if lf.ScheduledAt != nil {
		lDraft = true
	}
	if rf.ScheduledAt != nil {
		rDraft = true
	}
	return local.Body != remote.Body ||
		!lf.Date.Equal(rf.Date) ||
		lf.Title != rf.Title ||
		lDraft != rDraft ||
		!slices.Equal(lf.Category, rf.Category) ||
		lf.CustomURLPath != rf.CustomURLPath ||
		!scheduledAtEqual(lf.ScheduledAt, rf.ScheduledAt)
}

// scheduledAtEqual compares two *time.Time values for equality, treating nil as zero time.
func scheduledAtEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}
