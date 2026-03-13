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

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local articles",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
			return runList(cmd, dir, draftOnly, publishedOnly, v)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "Directory to scan for articles")
	cmd.Flags().BoolVar(&draftOnly, "draft", false, "Show only draft articles")
	cmd.Flags().BoolVar(&publishedOnly, "published", false, "Show only published articles")
	return cmd
}

func runList(cmd *cobra.Command, dir string, draftOnly, publishedOnly bool, showWarnings bool) error {
	if draftOnly && publishedOnly {
		return fmt.Errorf("--draft and --published cannot be used together")
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
		if draftOnly && !a.Frontmatter.Draft {
			continue
		}
		if publishedOnly && a.Frontmatter.Draft {
			continue
		}
		articles = append(articles, a)
	}

	if readErrCount > 0 && !showWarnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d file(s) skipped due to read errors (use --verbose for details)\n", readErrCount)
	}

	if len(articles) == 0 {
		fmt.Fprint(cmd.OutOrStdout(), "No articles found.\n")
		return nil
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Frontmatter.Date.After(articles[j].Frontmatter.Date)
	})

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 1, ' ', 0)
	fmt.Fprintln(w, "TITLE\tDATE\tSTATUS\tCATEGORIES")
	fmt.Fprintln(w, "-----\t----\t------\t----------")
	for _, a := range articles {
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
