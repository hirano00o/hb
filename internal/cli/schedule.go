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
	cmd := &cobra.Command{
		Use:   "schedule <file> <datetime>",
		Short: "Set scheduledAt in a local article's frontmatter",
		Long: `Set the scheduledAt field in the article's frontmatter.

Accepted datetime formats:
  RFC3339:          2026-04-01T12:00:00Z
  Space-separated:  2026-04-01 12:00:00  (interpreted as UTC)`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			rawDatetime := args[1]
			return runSchedule(path, rawDatetime)
		},
	}
	return cmd
}

func newUnscheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unschedule <file>",
		Short: "Clear scheduledAt from a local article's frontmatter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			return runUnschedule(path)
		},
	}
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

func runSchedule(path, rawDatetime string) error {
	t, err := parseScheduleDatetime(rawDatetime)
	if err != nil {
		return err
	}

	a, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	a.Frontmatter.ScheduledAt = &t
	if err := article.Write(path, a); err != nil {
		return err
	}
	return nil
}

func runUnschedule(path string) error {
	a, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	a.Frontmatter.ScheduledAt = nil
	if err := article.Write(path, a); err != nil {
		return err
	}
	return nil
}
