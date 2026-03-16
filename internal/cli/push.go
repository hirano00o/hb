package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var yes bool
	var draft bool
	var draftSet bool
	var all bool

	cmd := &cobra.Command{
		Use:   "push [<file> ...]",
		Short: "Push local file(s) to Hatena Blog (POST if new, PUT if updated)",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if all && len(args) > 0 {
				return fmt.Errorf("--all and file arguments are mutually exclusive")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("at least one file argument is required, or use --all")
			}

			paths := args
			if all {
				var err error
				paths, err = globMD(".")
				if err != nil {
					return fmt.Errorf("glob: %w", err)
				}
			}

			client, err := newClientFromConfig()
			if err != nil {
				return err
			}

			var errs []error
			for _, path := range paths {
				if err := pushOne(ctx, cmd, client, path, yes, draft, draftSet); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", path, err))
				}
			}
			if len(errs) > 0 {
				return errors.Join(errs...)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&draft, "draft", false, "Override entry draft status")
	cmd.Flags().BoolVar(&all, "all", false, "Push all .md files under the current directory")
	// Track whether --draft was explicitly specified on the command line.
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		draftSet = cmd.Flags().Changed("draft")
		return nil
	}
	return cmd
}

// pushOne pushes a single file to Hatena Blog.
func pushOne(ctx context.Context, cmd *cobra.Command, client *hatena.Client, path string, yes, draft, draftSet bool) error {
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	// Apply --draft override when the flag was explicitly set.
	if draftSet && draft != local.Frontmatter.Draft {
		ok, err := confirmAction(cmd, fmt.Sprintf(
			"Frontmatter draft=%v but --draft=%v. Push as draft=%v? [y/N]: ",
			local.Frontmatter.Draft, draft, draft,
		))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
		local.Frontmatter.Draft = draft
	} else if draftSet {
		local.Frontmatter.Draft = draft
	}

	// Upload local images in the body, replacing them with hatena:syntax.
	// The original local.Body is preserved; pushBody is used only for the API call.
	pushBody, err := article.ReplaceLocalImages(ctx, local.Body, filepath.Dir(path), client.UploadImage)
	if err != nil {
		return fmt.Errorf("replace images: %w", err)
	}
	pushEntry := local.ToEntry()
	pushEntry.Content = pushBody
	// scheduledAt requires draft=yes on the API side regardless of the local draft field.
	if local.Frontmatter.ScheduledAt != nil {
		pushEntry.Draft = true
	}

	// No editUrl → new entry, POST
	if local.Frontmatter.EditURL == "" {
		created, err := client.CreateEntry(ctx, pushEntry)
		if err != nil {
			return err
		}
		// Update local file with the assigned editUrl, url, and date
		local.Frontmatter.EditURL = created.EditURL
		local.Frontmatter.URL = created.URL
		local.Frontmatter.Date = created.Date
		if err := article.Write(path, local); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", created.URL)
		return nil
	}

	// Has editUrl → fetch remote and compare
	remote, err := client.GetEntry(ctx, local.Frontmatter.EditURL)
	if err != nil {
		return err
	}
	remoteArticle := article.FromEntry(remote)

	// Compare the image-replaced body against remote to avoid false positives
	// on re-push: local.Body has local paths, remoteArticle.Body has hatena:syntax.
	localForCompare := *local
	localForCompare.Body = pushBody
	if !hasChanges(&localForCompare, remoteArticle) {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes to push.")
		return nil
	}

	diff, err := unifiedDiff(path, local, remoteArticle)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), diff)

	if !yes {
		ok, err := confirmAction(cmd, "Push these changes? [y/N]: ")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
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

// hasChanges returns true if the local article differs from the remote in any field.
func hasChanges(local, remote *article.Article) bool {
	lf, rf := local.Frontmatter, remote.Frontmatter
	return local.Body != remote.Body ||
		!lf.Date.Equal(rf.Date) ||
		lf.Title != rf.Title ||
		lf.Draft != rf.Draft ||
		!slices.Equal(lf.Category, rf.Category) ||
		lf.CustomURLPath != rf.CustomURLPath ||
		!scheduledAtEqual(lf.ScheduledAt, rf.ScheduledAt)
}

// scheduledAtEqual compares two *time.Time values for equality, treating nil as zero time.
func scheduledAtEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}
