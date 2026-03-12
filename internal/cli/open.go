package cli

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

// openBrowser is a package-level variable so tests can replace it with a stub.
var openBrowser = defaultOpenBrowser

func defaultOpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Run()
	case "linux":
		return exec.Command("xdg-open", url).Run()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func newOpenCmd() *cobra.Command {
	var edit bool
	cmd := &cobra.Command{
		Use:   "open <file>",
		Short: "Open the article in the default browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			a, err := article.Read(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}

			u := a.Frontmatter.URL
			if edit {
				u = a.Frontmatter.EditURL
			}
			if u == "" {
				if edit {
					return fmt.Errorf("no edit URL found in %s: article may not be published yet", path)
				}
				return fmt.Errorf("no URL found in %s: article may not be published yet", path)
			}
			parsed, err := url.Parse(u)
			if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
				return fmt.Errorf("invalid URL %q: must be a valid http or https URL", u)
			}

			if err := openBrowser(u); err != nil {
				return fmt.Errorf("open browser: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Opened: %s\n", u)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&edit, "edit", "e", false, "Open the edit page instead of the public URL")
	return cmd
}
