package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

// execCommand is a package-level variable so tests can replace it.
var execCommand = exec.Command

func newEditCmd() *cobra.Command {
	var pushFlag bool

	cmd := &cobra.Command{
		Use:   "edit <file>",
		Short: "Open a local article in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			return runEdit(cmd, path, pushFlag)
		},
	}

	cmd.Flags().BoolVar(&pushFlag, "push", false, "Push changes without confirmation prompt")
	return cmd
}

// fileHash returns the SHA-256 hash of the file at path.
func fileHash(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}

func runEdit(cmd *cobra.Command, path string, autoPush bool) error {
	before, err := fileHash(path)
	if err != nil {
		return fmt.Errorf("hash before edit: %w", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := execCommand(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	after, err := fileHash(path)
	if err != nil {
		return fmt.Errorf("hash after edit: %w", err)
	}

	if bytes.Equal(before, after) {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
		return nil
	}

	// Show diff: local (after edit) vs remote if editUrl is available.
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if local.Frontmatter.EditURL != "" {
		ctx := cmd.Context()
		client, err := newClientFromConfig()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch remote entry: %v — verify that editUrl is correct and accessible\n", err)
		} else {
			remote, err := client.GetEntry(ctx, local.Frontmatter.EditURL)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch remote entry: %v — verify that editUrl is correct and accessible\n", err)
			} else {
				remoteArticle := article.FromEntry(remote)
				diff, err := unifiedDiff(path, local, remoteArticle)
				if err == nil && diff != "" {
					fmt.Fprint(cmd.OutOrStdout(), diff)
				}
			}
		}
	}

	if autoPush {
		return doPush(cmd, path)
	}

	ok, err := confirmAction(cmd, "Push changes? [y/N]: ")
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}
	return doPush(cmd, path)
}

// doPush pushes the file at path by re-using the push command's RunE with --yes.
func doPush(cmd *cobra.Command, path string) error {
	pushCmd := newPushCmd()
	pushCmd.SetOut(cmd.OutOrStdout())
	pushCmd.SetErr(cmd.ErrOrStderr())
	pushCmd.SetIn(cmd.InOrStdin())
	pushCmd.SetContext(cmd.Context())
	// Mark --yes so the push command skips its own confirmation prompt.
	if err := pushCmd.Flags().Set("yes", "true"); err != nil {
		return fmt.Errorf("set push flag: %w", err)
	}
	return pushCmd.RunE(pushCmd, []string{path})
}
