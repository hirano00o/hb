package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var dir string
	var titleOnly, bodyOnly bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search local articles by keyword",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
			return runSearch(cmd, args[0], dir, titleOnly, bodyOnly, v)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to scan for articles")
	cmd.Flags().BoolVar(&titleOnly, "title", false, "Search in title only")
	cmd.Flags().BoolVar(&bodyOnly, "body", false, "Search in body only")
	return cmd
}

func runSearch(cmd *cobra.Command, query, dir string, titleOnly, bodyOnly bool, showWarnings bool) error {
	locals, err := loadArticles(dir, showWarnings, cmd.ErrOrStderr())
	if err != nil {
		return err
	}

	q := strings.ToLower(query)

	var matched []*article.Article
	for _, l := range locals {
		a := l.art
		inTitle := strings.Contains(strings.ToLower(a.Frontmatter.Title), q)
		inBody := strings.Contains(strings.ToLower(a.Body), q)
		var hit bool
		switch {
		case titleOnly && bodyOnly:
			hit = inTitle && inBody
		case titleOnly:
			hit = inTitle
		case bodyOnly:
			hit = inBody
		default:
			hit = inTitle || inBody
		}
		if hit {
			matched = append(matched, a)
		}
	}

	if len(matched) == 0 {
		fmt.Fprint(cmd.OutOrStdout(), "No articles found.\n")
		return nil
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Frontmatter.Date.After(matched[j].Frontmatter.Date)
	})

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 1, ' ', 0)
	fmt.Fprintln(w, "TITLE\tDATE\tSTATUS\tCATEGORIES")
	fmt.Fprintln(w, "-----\t----\t------\t----------")
	for _, a := range matched {
		fm := a.Frontmatter
		status := "published"
		if fm.Draft {
			status = "draft"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			fm.Title,
			fm.Date.Format("2006-01-02"),
			status,
			strings.Join(fm.Category, " "),
		)
	}
	return w.Flush()
}
