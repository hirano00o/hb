package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show sync status of local articles against remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClientFromConfig()
			if err != nil {
				return err
			}
			cfg, _ := config.LoadMerged() // errors already caught by newClientFromConfig
			maxPages := 0
			if cfg != nil && cfg.MaxPages != nil {
				maxPages = *cfg.MaxPages
			}
			v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
			return runStatus(cmd, client, dir, maxPages, v)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to scan for articles")
	return cmd
}

func runStatus(cmd *cobra.Command, client *hatena.Client, dir string, maxPages int, showWarnings bool) error {
	files, err := globMD(dir)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Fprint(cmd.OutOrStdout(), "No articles found.\n")
		return nil
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "Fetching remote entries...")

	// Collect local articles, skipping unreadable or frontmatter-less files.
	type localEntry struct {
		path string
		art  *article.Article
	}
	var locals []localEntry
	var readErrCount int
	for _, f := range files {
		a, err := article.Read(f)
		if err != nil {
			readErrCount++
			if showWarnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to read %s: %v (skipping)\n", f, err)
			}
			continue
		}
		if a.Frontmatter.Title == "" && a.Frontmatter.Date.IsZero() {
			if showWarnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping %s: no frontmatter\n", f)
			}
			continue
		}
		locals = append(locals, localEntry{path: f, art: a})
	}

	if readErrCount > 0 && !showWarnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d file(s) skipped due to read errors (use --verbose for details)\n", readErrCount)
	}

	if len(locals) == 0 {
		fmt.Fprint(cmd.OutOrStdout(), "No articles found.\n")
		return nil
	}

	// Fetch remote entries and build editURL → *Article map.
	remoteEntries, err := client.ListEntries(cmd.Context(), maxPages)
	if err != nil {
		return err
	}
	remoteByEditURL := make(map[string]*article.Article, len(remoteEntries))
	for _, e := range remoteEntries {
		if e.EditURL != "" {
			remoteByEditURL[e.EditURL] = article.FromEntry(e)
		}
	}

	// Classify each local file.
	var modified, untracked, upToDate []string
	for _, l := range locals {
		editURL := l.art.Frontmatter.EditURL
		if editURL == "" {
			untracked = append(untracked, l.path)
			continue
		}
		remote, ok := remoteByEditURL[editURL]
		if !ok {
			// editURL present but not found in remote — treat as untracked.
			untracked = append(untracked, l.path)
			continue
		}
		if hasChanges(l.art, remote) {
			if article.HasLocalImages(l.art.Body) {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s contains local images; diff may not be accurate until pushed\n", filepath.Base(l.path))
			}
			modified = append(modified, l.path)
		} else {
			upToDate = append(upToDate, l.path)
		}
	}

	// Sort each group for deterministic output.
	sort.Strings(modified)
	sort.Strings(untracked)
	sort.Strings(upToDate)

	printGroup(cmd, "Modified", modified)
	printGroup(cmd, "Untracked", untracked)
	printGroup(cmd, "Up to date", upToDate)
	return nil
}

func printGroup(cmd *cobra.Command, label string, paths []string) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", label, len(paths))
	for _, p := range paths {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", filepath.Base(p))
	}
	fmt.Fprintln(cmd.OutOrStdout())
}
