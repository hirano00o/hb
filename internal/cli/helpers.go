package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
	"github.com/spf13/cobra"
)

// newClientFromConfig loads and validates configuration, then returns a new API client.
func newClientFromConfig() (*hatena.Client, error) {
	cfg, err := config.LoadMerged()
	if err != nil {
		return nil, err
	}
	if err := config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return hatena.NewClient(cfg.HatenaID, cfg.BlogID, cfg.APIKey), nil
}

// confirmAction prints prompt and reads a y/Y response from stdin.
// Returns true when the user confirms, false otherwise (including EOF).
func confirmAction(cmd *cobra.Command, prompt string) (bool, error) {
	return confirmActionWithScanner(cmd, bufio.NewScanner(cmd.InOrStdin()), prompt)
}

// confirmActionWithScanner is like confirmAction but uses a caller-provided scanner,
// allowing multiple stdin reads to share the same buffered scanner.
func confirmActionWithScanner(cmd *cobra.Command, scanner *bufio.Scanner, prompt string) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "y"), nil
}
