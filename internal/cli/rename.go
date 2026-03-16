package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hirano00o/hb/article"
	"github.com/spf13/cobra"
)

func newRenameCmd() *cobra.Command {
	var title string
	var force bool

	cmd := &cobra.Command{
		Use:   "rename <file>",
		Short: "Rename a local article and update its frontmatter title",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRename(cmd, args[0], title, force)
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "New article title (required)")
	if err := cmd.MarkFlagRequired("title"); err != nil {
		panic(err)
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite destination file if it already exists")
	return cmd
}

func runRename(cmd *cobra.Command, path, newTitle string, force bool) error {
	local, err := article.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	local.Frontmatter.Title = newTitle
	newBase := article.GenerateFilename(newTitle, local.Frontmatter.Date, local.Frontmatter.Draft)
	newPath := filepath.Join(filepath.Dir(path), newBase)

	if newPath != path {
		if !force {
			if err := checkNoConflict(newPath); err != nil {
				return err
			}
		}
	}

	if err := article.Write(newPath, local); err != nil {
		return err
	}
	if newPath != path {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old file: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Renamed: %s → %s\n", path, newPath)
	return nil
}

// checkNoConflict returns an error if the given path already exists.
func checkNoConflict(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("destination %q already exists; use --force to overwrite", path)
	}
	return nil
}

// renameFile renames src to dst using os.Rename.
func renameFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}
	return nil
}
