package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

// timeNow is a package-level variable so tests can inject a fixed time.
var timeNow = time.Now

// isStdinPipe returns true when stdin is a pipe (not a terminal).
// It is a package-level variable so tests can stub it.
var isStdinPipe = func() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// bodySentinel is the NoOptDefVal for -b: indicates -b was given with no value.
const bodySentinel = "\x00"

func newNewCmd() *cobra.Command {
	return newNewCmdIn(".")
}

func newNewCmdIn(dir string) *cobra.Command {
	var draft bool
	var push bool
	var body string
	var title string

	cmd := &cobra.Command{
		Use:   "new --title <title>",
		Short: "Create a new local article file with frontmatter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := timeNow()

			// Resolve body content.
			resolvedBody, err := resolveBody(cmd, body)
			if err != nil {
				return err
			}

			filename := article.GenerateFilename(title, now, draft)
			path := filepath.Join(dir, filename)

			// Abort if the file already exists to avoid silent overwrites.
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("file %q already exists: rename it first", path)
			}

			a := &article.Article{
				Frontmatter: article.Frontmatter{
					Title: title,
					Date:  now,
					Draft: draft,
				},
				Body: resolvedBody,
			}

			if err := article.Write(path, a); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", path)

			if !push {
				return nil
			}

			// --push: POST to the API (same flow as push.go CreateEntry path).
			ctx := cmd.Context()
			client, err := newClientFromConfig()
			if err != nil {
				return err
			}

			pushBody, err := article.ReplaceLocalImages(ctx, a.Body, filepath.Dir(path), client.UploadImage)
			if err != nil {
				return fmt.Errorf("replace images: %w", err)
			}
			pushEntry := a.ToEntry()
			pushEntry.Content = pushBody

			created, err := client.CreateEntry(ctx, pushEntry)
			if err != nil {
				return err
			}
			a.Frontmatter.EditURL = created.EditURL
			a.Frontmatter.URL = created.URL
			a.Frontmatter.Date = created.Date
			if err := article.Write(path, a); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", created.URL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Article title (required)")
	_ = cmd.MarkFlagRequired("title")
	cmd.Flags().BoolVar(&draft, "draft", false, "Create as draft (adds draft_ prefix to filename and sets draft: true)")
	cmd.Flags().BoolVarP(&push, "push", "p", false, "Push to Hatena Blog after creating the local file")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Article body (-b alone reads from stdin pipe)")
	// NoOptDefVal makes -b without a value set body to sentinel instead of the empty default.
	cmd.Flags().Lookup("body").NoOptDefVal = bodySentinel

	return cmd
}

// resolveBody determines the article body from the -b flag value.
//
//   - flag not given            → empty string
//   - -b "text"                 → text, with literal \n replaced by real newlines
//   - -b (no value) + stdin pipe → read from stdin as-is
//   - -b (no value) + no pipe   → error
func resolveBody(cmd *cobra.Command, body string) (string, error) {
	if !cmd.Flags().Changed("body") {
		return "", nil
	}
	if body != bodySentinel {
		// Argument provided: convert literal \n to real newlines.
		return strings.ReplaceAll(body, `\n`, "\n"), nil
	}
	// Sentinel: -b was given without a value; read from stdin pipe.
	if !isStdinPipe() {
		return "", fmt.Errorf("-b requires a value or piped stdin")
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(data), nil
}
