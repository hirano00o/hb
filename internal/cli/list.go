package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var dir string
	var draftOnly, publishedOnly bool
	var filterCategory string
	var listCategories bool
	var scheduledOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local articles",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
			return runList(cmd, dir, draftOnly, publishedOnly, v, filterCategory, listCategories, scheduledOnly)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to scan for articles")
	cmd.Flags().BoolVar(&draftOnly, "draft", false, "Show only draft articles")
	cmd.Flags().BoolVar(&publishedOnly, "published", false, "Show only published articles")
	cmd.Flags().StringVar(&filterCategory, "category", "", "Show only articles containing this category")
	cmd.Flags().BoolVar(&listCategories, "categories", false, "List all categories with article counts")
	cmd.Flags().BoolVar(&scheduledOnly, "scheduled", false, "Show only scheduled articles")
	return cmd
}

func runList(cmd *cobra.Command, dir string, draftOnly, publishedOnly bool, showWarnings bool, filterCategory string, listCategories bool, scheduledOnly bool) error {
	// --categories is a summary mode: it aggregates article counts per category across all
	// articles and returns early before any article-level filter is applied (see below).
	// Allowing --draft/--published/--category/--scheduled together would silently ignore those
	// filters, producing counts that do not match what the user asked for.
	if listCategories && (draftOnly || publishedOnly || filterCategory != "" || scheduledOnly) {
		return fmt.Errorf("--categories cannot be used with --draft, --published, --category, or --scheduled")
	}
	// The draft field is a boolean, so --draft (draft=true) and --published (draft=false)
	// are mutually exclusive states. No article can satisfy both at once.
	if draftOnly && publishedOnly {
		return fmt.Errorf("--draft and --published cannot be used together")
	}
	if scheduledOnly && (draftOnly || publishedOnly) {
		return fmt.Errorf("--scheduled cannot be used with --draft or --published")
	}

	files, err := globMD(dir)
	if err != nil {
		return err
	}

	var articles []*article.Article
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
		articles = append(articles, a)
	}

	if readErrCount > 0 && !showWarnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d file(s) skipped due to read errors (use --verbose for details)\n", readErrCount)
	}

	if listCategories {
		counts := make(map[string]int)
		for _, a := range articles {
			for _, cat := range a.Frontmatter.Category {
				counts[cat]++
			}
		}
		type catCount struct {
			name  string
			count int
		}
		var sorted []catCount
		for name, count := range counts {
			sorted = append(sorted, catCount{name, count})
		}
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].count != sorted[j].count {
				return sorted[i].count > sorted[j].count
			}
			return sorted[i].name < sorted[j].name
		})
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CATEGORY\tCOUNT")
		fmt.Fprintln(w, "--------\t-----")
		for _, cc := range sorted {
			fmt.Fprintf(w, "%s\t%d\n", cc.name, cc.count)
		}
		return w.Flush()
	}

	// Apply draft/published/category/scheduled filters after collecting all articles for --categories.
	var filtered []*article.Article
	for _, a := range articles {
		if draftOnly && !a.Frontmatter.Draft {
			continue
		}
		if publishedOnly && a.Frontmatter.Draft {
			continue
		}
		if filterCategory != "" {
			found := false
			for _, cat := range a.Frontmatter.Category {
				if strings.EqualFold(cat, filterCategory) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if scheduledOnly && a.Frontmatter.ScheduledAt == nil {
			continue
		}
		filtered = append(filtered, a)
	}

	if len(filtered) == 0 {
		fmt.Fprint(cmd.OutOrStdout(), "No articles found.\n")
		return nil
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Frontmatter.Date.After(filtered[j].Frontmatter.Date)
	})

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 1, ' ', 0)
	fmt.Fprintln(w, "TITLE\tDATE\tSTATUS\tCATEGORIES\tSCHEDULED_AT")
	fmt.Fprintln(w, "-----\t----\t------\t----------\t------------")
	for _, a := range filtered {
		fm := a.Frontmatter
		status := "published"
		if fm.Draft {
			status = "draft"
		}
		scheduledAt := ""
		if fm.ScheduledAt != nil {
			scheduledAt = fm.ScheduledAt.UTC().Format("2006-01-02T15:04Z")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			fm.Title,
			fm.Date.Format("2006-01-02"),
			status,
			strings.Join(fm.Category, " "),
			scheduledAt,
		)
	}
	return w.Flush()
}
