package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/hirano00o/hb/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a project-local hb config (.hb/config.yaml)",
		RunE: func(cmd *cobra.Command, args []string) error {
			const projectConfigFile = ".hb/config.yaml"
			if _, err := os.Stat(projectConfigFile); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s already exists. Overwrite? [y/N]: ", projectConfigFile)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() {
					return nil
				}
				if !strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			cfg := &config.Config{}
			if err := config.Save(projectConfigFile, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project config created at %s\n", projectConfigFile)
			fmt.Fprintln(cmd.OutOrStdout(), "Edit it to override global settings for this project.")
			return nil
		},
	}
}
