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

	cmd := &cobra.Command{
		Use:   "publish <file>",
		Short: "Publish a draft article (set draft=false and remove draft_ prefix)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublish(cmd, args[0], push)
		},
	}

	cmd.Flags().BoolVarP(&push, "push", "p", false, "Push to Hatena Blog after publishing")
	return cmd
}

func newUnpublishCmd() *cobra.Command {
	var push bool

	cmd := &cobra.Command{
		Use:   "unpublish <file>",
		Short: "Unpublish an article (set draft=true and add draft_ prefix)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnpublish(cmd, args[0], push)
		},
	}

	cmd.Flags().BoolVarP(&push, "push", "p", false, "Push to Hatena Blog after unpublishing")
	return cmd
}

func runPublish(cmd *cobra.Command, path string, push bool) error {
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	local.Frontmatter.Draft = false

	// Rename: remove draft_ prefix if present.
	newPath := path
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	if strings.HasPrefix(base, "draft_") {
		newBase := strings.TrimPrefix(base, "draft_")
		newPath = filepath.Join(dir, newBase)
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

	fmt.Fprintf(cmd.OutOrStdout(), "Published: %s\n", newPath)

	if push {
		return pushAfterStateChange(cmd, newPath)
	}
	return nil
}

func runUnpublish(cmd *cobra.Command, path string, push bool) error {
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	local.Frontmatter.Draft = true

	// Rename: add draft_ prefix if not already present.
	newPath := path
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	if !strings.HasPrefix(base, "draft_") {
		newBase := "draft_" + base
		newPath = filepath.Join(dir, newBase)
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

	fmt.Fprintf(cmd.OutOrStdout(), "Unpublished: %s\n", newPath)

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

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	pushBody, err := article.ReplaceLocalImages(ctx, local.Body, filepath.Dir(path), client.UploadImage)
	if err != nil {
		return fmt.Errorf("replace images: %w", err)
	}
	pushEntry := local.ToEntry()
	pushEntry.Content = pushBody
	if local.Frontmatter.ScheduledAt != nil {
		pushEntry.Draft = true
	}

	if local.Frontmatter.EditURL == "" {
		created, err := client.CreateEntry(ctx, pushEntry)
		if err != nil {
			return err
		}
		local.Frontmatter.EditURL = created.EditURL
		local.Frontmatter.URL = created.URL
		local.Frontmatter.Date = created.Date
		if err := article.Write(path, local); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", created.URL)
		return nil
	}

	updated, err := client.UpdateEntry(ctx, local.Frontmatter.EditURL, pushEntry)
	if err != nil {
		return err
	}
	local.Frontmatter.EditURL = updated.EditURL
	local.Frontmatter.URL = updated.URL
	local.Frontmatter.Date = updated.Date
	if err := article.Write(path, local); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", updated.URL)
	return nil
}
