package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	var dir string
	var months int

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show statistics about local articles",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
			return runStats(cmd, dir, months, v)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to scan for articles")
	cmd.Flags().IntVar(&months, "months", 6, "Number of recent months to show (0 = all)")
	return cmd
}

func runStats(cmd *cobra.Command, dir string, months int, showWarnings bool) error {
	files, err := globMD(dir)
	if err != nil {
		return err
	}

	var arts []*article.Article
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
		arts = append(arts, a)
	}

	if readErrCount > 0 && !showWarnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d file(s) skipped due to read errors (use --verbose for details)\n", readErrCount)
	}

	total := len(arts)
	var published, drafts, scheduled int
	categoryCount := make(map[string]int)
	monthCount := make(map[string]int)

	for _, a := range arts {
		fm := a.Frontmatter
		if fm.ScheduledAt != nil {
			scheduled++
		} else if fm.Draft {
			drafts++
		} else {
			published++
		}

		if len(fm.Category) == 0 {
			categoryCount["(none)"]++
		} else {
			for _, cat := range fm.Category {
				categoryCount[cat]++
			}
		}

		if !fm.Date.IsZero() {
			key := fm.Date.UTC().Format("2006-01")
			monthCount[key]++
		}
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Total:     %d articles\n", total)
	fmt.Fprintf(out, "Published: %d\n", published)
	fmt.Fprintf(out, "Drafts:    %d\n", drafts)
	fmt.Fprintf(out, "Scheduled: %d\n", scheduled)

	fmt.Fprintln(out)
	fmt.Fprintln(out, "By category:")
	type catEntry struct {
		name  string
		count int
	}
	cats := make([]catEntry, 0, len(categoryCount))
	for name, count := range categoryCount {
		cats = append(cats, catEntry{name, count})
	}
	sort.Slice(cats, func(i, j int) bool {
		if cats[i].count != cats[j].count {
			return cats[i].count > cats[j].count
		}
		return cats[i].name < cats[j].name
	})
	for _, c := range cats {
		fmt.Fprintf(out, "  %-10s %d\n", c.name+":", c.count)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "By month (recent "+monthsLabel(months)+"):")

	// Collect all month keys and sort descending.
	allMonths := make([]string, 0, len(monthCount))
	for m := range monthCount {
		allMonths = append(allMonths, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(allMonths)))

	// When months > 0, compute a cutoff so only recent N months are shown.
	// cutoff is the first day of the month that is (months-1) months before the current month.
	var cutoff time.Time
	if months > 0 {
		now := time.Now().UTC()
		// AddDate handles month arithmetic correctly (e.g., wrapping year).
		cutoff = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(months - 1), 0)
	}

	printed := 0
	for _, m := range allMonths {
		if months > 0 {
			t, parseErr := time.Parse("2006-01", m)
			if parseErr == nil && t.Before(cutoff) {
				continue
			}
		}
		fmt.Fprintf(out, "  %s  %d\n", m+":", monthCount[m])
		printed++
		if months > 0 && printed >= months {
			break
		}
	}

	return nil
}

func monthsLabel(months int) string {
	if months == 0 {
		return "all"
	}
	return fmt.Sprintf("%d", months)
}
