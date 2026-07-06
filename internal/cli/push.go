package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

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

			paths, err := resolveTargetPaths(args, all)
			if err != nil {
				return err
			}

			client, _, err := newClientFromConfig()
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

	pushEntry, pushBody, err := preparePushEntry(ctx, client, local, path)
	if err != nil {
		return err
	}

	// No editUrl → new entry, POST
	if local.Frontmatter.EditURL == "" {
		created, err := client.CreateEntry(ctx, pushEntry)
		if err != nil {
			return err
		}
		return writeBackAndReport(cmd, path, local, created, "Created")
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
	return writeBackAndReport(cmd, path, local, updated, "Updated")
}

// preparePushEntry uploads local images and builds the API entry
// (scheduledAt forces draft=yes on the API side).
// The original local.Body is preserved; pushBody is used only for the API call
// and returned separately so callers can diff against the remote body.
func preparePushEntry(ctx context.Context, client *hatena.Client, local *article.Article, path string) (entry *hatena.Entry, pushBody string, err error) {
	pushBody, err = article.ReplaceLocalImages(ctx, local.Body, filepath.Dir(path), client.UploadImage)
	if err != nil {
		return nil, "", fmt.Errorf("replace images: %w", err)
	}
	entry = local.ToEntry()
	entry.Content = pushBody
	if local.Frontmatter.ScheduledAt != nil {
		entry.Draft = true
	}
	return entry, pushBody, nil
}

// writeBackAndReport persists the server-assigned editUrl/url/date and reports the result.
func writeBackAndReport(cmd *cobra.Command, path string, local *article.Article, result *hatena.Entry, verb string) error {
	local.Frontmatter.EditURL = result.EditURL
	local.Frontmatter.URL = result.URL
	local.Frontmatter.Date = result.Date
	if err := article.Write(path, local); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", verb, result.URL)
	return nil
}
