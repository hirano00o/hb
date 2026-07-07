package cli

import (
	"fmt"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

// datetimeLayouts lists the accepted datetime formats for hb schedule.
var datetimeLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
}

func newScheduleCmd() *cobra.Command {
	var clear bool

	cmd := &cobra.Command{
		Use:   "schedule <file> [<datetime>]",
		Short: "Set or clear scheduledAt in a local article's frontmatter",
		Long: `Set the scheduledAt field in the article's frontmatter.

Accepted datetime formats:
  RFC3339:          2026-04-01T12:00:00Z
  Space-separated:  2026-04-01 12:00:00  (interpreted as UTC)

With --clear, remove scheduledAt instead; no <datetime> is accepted.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear {
				if len(args) != 1 {
					return fmt.Errorf("--clear cannot be used with a <datetime> argument")
				}
				return runSchedule(cmd, args[0], "", true)
			}
			if len(args) != 2 {
				return fmt.Errorf("<datetime> is required unless --clear is given")
			}
			return runSchedule(cmd, args[0], args[1], false)
		},
	}

	cmd.Flags().BoolVar(&clear, "clear", false, "Clear scheduledAt instead of setting it")
	return cmd
}

// parseScheduleDatetime tries each supported layout and returns the parsed time in UTC.
func parseScheduleDatetime(raw string) (time.Time, error) {
	for _, layout := range datetimeLayouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime %q: accepted formats are RFC3339 (2006-01-02T15:04:05Z) or \"2006-01-02 15:04:05\"", raw)
}

func runSchedule(cmd *cobra.Command, path, rawDatetime string, clear bool) error {
	var scheduledAt *time.Time
	if !clear {
		t, err := parseScheduleDatetime(rawDatetime)
		if err != nil {
			return err
		}
		scheduledAt = &t
	}

	a, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	a.Frontmatter.ScheduledAt = scheduledAt
	if err := article.Write(path, a); err != nil {
		return err
	}

	if clear {
		fmt.Fprintf(cmd.OutOrStdout(), "Unscheduled: %s\n", path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Scheduled: %s (%s)\n", path, scheduledAt.Format(time.RFC3339))
	}
	return nil
}
