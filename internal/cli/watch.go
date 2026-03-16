package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

const defaultDebounce = 500 * time.Millisecond

// pushFileFunc is a package-level variable so tests can replace the push action.
var pushFileFunc = pushFile

func newWatchCmd() *cobra.Command {
	var dir string
	var debounce time.Duration

	cmd := &cobra.Command{
		Use:   "watch [<file>]",
		Short: "Watch a file or directory for changes and auto-push to Hatena Blog",
		Long: `Watch a local Markdown file or directory for changes.
When a watched file is modified, it is automatically pushed to Hatena Blog.

A brief delay (debounce) is applied so that rapid successive saves only
trigger one push.  Press Ctrl+C to stop watching.

Examples:
  hb watch article.md
  hb watch --dir .`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the set of files to watch.
			var paths []string
			switch {
			case len(args) == 1 && cmd.Flags().Changed("dir"):
				return fmt.Errorf("--dir and a file argument are mutually exclusive")
			case len(args) == 1:
				paths = args
			default:
				var err error
				paths, err = globMD(dir)
				if err != nil {
					return fmt.Errorf("scan directory: %w", err)
				}
				if len(paths) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No .md files found.")
					return nil
				}
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return runWatch(ctx, cmd, paths, debounce)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to watch (used when no file argument is given)")
	cmd.Flags().DurationVar(&debounce, "debounce", defaultDebounce, "Delay between a file change and the push")
	return cmd
}

// runWatch sets up a fsnotify watcher and pushes each changed file after debounce.
func runWatch(ctx context.Context, cmd *cobra.Command, paths []string, debounce time.Duration) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Add each file's directory to the watcher (fsnotify watches directories).
	watched := make(map[string]bool) // directories already added
	fileSet := make(map[string]bool) // canonical set of target .md files
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("resolve path %s: %w", p, err)
		}
		fileSet[abs] = true
		dir := filepath.Dir(abs)
		if !watched[dir] {
			if err := watcher.Add(dir); err != nil {
				return fmt.Errorf("watch %s: %w", dir, err)
			}
			watched[dir] = true
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Watching %d file(s). Press Ctrl+C to stop.\n", len(fileSet))

	// Propagate the context so pushFile can pass it to subcommands.
	cmd.SetContext(ctx)

	// Debounce timers keyed by file path.
	timers := make(map[string]*time.Timer)
	stopAllTimers := func() {
		for _, t := range timers {
			t.Stop()
		}
	}

	for {
		select {
		case <-ctx.Done():
			stopAllTimers()
			fmt.Fprintln(cmd.OutOrStdout(), "\nStopped.")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isWriteEvent(event) {
				continue
			}
			// Only react to files in the target set.
			abs, err := filepath.Abs(event.Name)
			if err != nil {
				continue
			}
			// If watching a directory, include any new .md file.
			if !fileSet[abs] && strings.HasSuffix(abs, ".md") && isUnderWatchedDir(abs, watched) {
				fileSet[abs] = true
			}
			if !fileSet[abs] {
				continue
			}

			// Debounce: reset the timer for this file.
			if t, exists := timers[abs]; exists {
				t.Stop()
			}
			timers[abs] = time.AfterFunc(debounce, func() {
				if ctx.Err() != nil {
					return
				}
				if err := pushFileFunc(cmd, abs); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "push %s: %v\n", abs, err)
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "watcher error: %v\n", err)
		}
	}
}

// isWriteEvent returns true for Write and Create events (both indicate new content).
func isWriteEvent(e fsnotify.Event) bool {
	return e.Has(fsnotify.Write) || e.Has(fsnotify.Create)
}

// isUnderWatchedDir returns true when path is inside one of the watched directories.
func isUnderWatchedDir(path string, watched map[string]bool) bool {
	dir := filepath.Dir(path)
	return watched[dir]
}

// pushFile pushes a single .md file to Hatena Blog without a confirmation prompt.
func pushFile(cmd *cobra.Command, path string) error {
	// Skip files without frontmatter or editUrl gracefully.
	if _, err := os.Stat(path); err != nil {
		return nil // file deleted; ignore
	}

	fmt.Fprintf(cmd.OutOrStdout(), "[watch] pushing %s ...\n", path)

	pushCmd := newPushCmd()
	pushCmd.SetOut(cmd.OutOrStdout())
	pushCmd.SetErr(cmd.ErrOrStderr())
	pushCmd.SetIn(cmd.InOrStdin())
	if cmd.Context() != nil {
		pushCmd.SetContext(cmd.Context())
	}
	if err := pushCmd.Flags().Set("yes", "true"); err != nil {
		return fmt.Errorf("set push flag: %w", err)
	}
	return pushCmd.RunE(pushCmd, []string{path})
}

