package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newPublishCmd() *cobra.Command {
	var push bool
	var undo bool

	cmd := &cobra.Command{
		Use:   "publish <file>",
		Short: "Publish a draft article (set draft=false and remove draft_ prefix); --undo reverts to draft",
		Long: `Publish a draft article: set draft=false and remove the draft_ filename prefix.

With --undo, revert to draft instead: set draft=true and add the draft_ prefix.
Either direction clears scheduledAt.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublish(cmd, args[0], push, undo)
		},
	}

	cmd.Flags().BoolVarP(&push, "push", "p", false, "Push to Hatena Blog after publishing")
	cmd.Flags().BoolVar(&undo, "undo", false, "Revert to draft (set draft=true and add draft_ prefix)")
	return cmd
}

func runPublish(cmd *cobra.Command, path string, push, undo bool) error {
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	local.Frontmatter.Draft = undo
	// Publish: a lingering scheduledAt would turn the next push back into a
	// scheduled draft (see pushOne), contradicting "publish now".
	// Undo: without this, a scheduled entry would still auto-publish at the
	// scheduled time on the next push despite being reverted to draft.
	hadSchedule := local.Frontmatter.ScheduledAt != nil
	local.Frontmatter.ScheduledAt = nil

	// Rename: remove draft_ prefix when publishing, add it when undoing.
	newPath := path
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	if undo {
		if !strings.HasPrefix(base, "draft_") {
			newPath = filepath.Join(dir, "draft_"+base)
		}
	} else if strings.HasPrefix(base, "draft_") {
		newPath = filepath.Join(dir, strings.TrimPrefix(base, "draft_"))
	}
	if newPath != path {
		if err := checkNoConflict(newPath); err != nil {
			return err
		}
	}

	if err := article.Write(path, local); err != nil {
		return err
	}
	if newPath != path {
		if err := renameFile(path, newPath); err != nil {
			return err
		}
	}

	if undo {
		fmt.Fprintf(cmd.OutOrStdout(), "Unpublished: %s\n", newPath)
		if hadSchedule {
			fmt.Fprintln(cmd.OutOrStdout(), "Cleared scheduledAt: the article will no longer publish at the scheduled time.")
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Published: %s\n", newPath)
		if hadSchedule {
			fmt.Fprintln(cmd.OutOrStdout(), "Cleared scheduledAt: the article is published now instead of at the scheduled time.")
		}
	}

	if push {
		return pushAfterStateChange(cmd, newPath)
	}
	return nil
}

// pushAfterStateChange pushes the file at path to Hatena Blog without confirmation.
// It is a simplified push: no diff display, no prompt.
func pushAfterStateChange(cmd *cobra.Command, path string) error {
	ctx := cmd.Context()
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	client, _, err := newClientFromConfig()
	if err != nil {
		return err
	}

	pushEntry, _, err := preparePushEntry(ctx, client, local, path)
	if err != nil {
		return err
	}

	if local.Frontmatter.EditURL == "" {
		created, err := client.CreateEntry(ctx, pushEntry)
		if err != nil {
			return err
		}
		return writeBackAndReport(cmd, path, local, created, "Created")
	}

	updated, err := client.UpdateEntry(ctx, local.Frontmatter.EditURL, pushEntry)
	if err != nil {
		return err
	}
	return writeBackAndReport(cmd, path, local, updated, "Updated")
}
