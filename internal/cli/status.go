package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/hirano00o/hb/article"
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
			return runStatus(cmd, client, dir)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to scan for articles")
	return cmd
}

func runStatus(cmd *cobra.Command, client *hatena.Client, dir string) error {
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
	for _, f := range files {
		a, err := article.Read(f)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to read %s: %v\n", f, err)
			continue
		}
		if a.Frontmatter.Title == "" && a.Frontmatter.Date.IsZero() {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping %s: no frontmatter\n", f)
			continue
		}
		locals = append(locals, localEntry{path: f, art: a})
	}

	if len(locals) == 0 {
		fmt.Fprint(cmd.OutOrStdout(), "No articles found.\n")
		return nil
	}

	// Fetch remote entries and build editURL → *Article map.
	remoteEntries, err := client.ListEntries(cmd.Context(), 0)
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
		localStr, err := articleToString(l.art)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to render %s: %v\n", l.path, err)
			continue
		}
		remoteStr, err := articleToString(remote)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to render remote entry for %s: %v\n", l.path, err)
			continue
		}
		if localStr != remoteStr {
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
