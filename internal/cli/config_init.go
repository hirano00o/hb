package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/hirano00o/hb/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newConfigInitCmd() *cobra.Command {
	var hatenaID, blogID string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the global hb configuration (~/.config/hb/config.yaml)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := promptConfig(cmd, hatenaID, blogID)
			if err != nil {
				return err
			}
			if err := config.Validate(cfg); err != nil {
				return err
			}
			path, err := config.GlobalConfigPath()
			if err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Global config saved to %s\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&hatenaID, "hatena-id", "", "Hatena ID")
	cmd.Flags().StringVar(&blogID, "blog-id", "", "Blog ID (e.g. example.hateblo.jp)")

	return cmd
}

func promptConfig(cmd *cobra.Command, hatenaID, blogID string) (*config.Config, error) {
	scanner := bufio.NewScanner(cmd.InOrStdin())
	cfg := &config.Config{}

	if hatenaID != "" {
		cfg.HatenaID = hatenaID
	} else {
		fmt.Fprint(cmd.OutOrStdout(), "Hatena ID: ")
		if !scanner.Scan() {
			return nil, fmt.Errorf("read hatena ID: %w", scanner.Err())
		}
		cfg.HatenaID = strings.TrimSpace(scanner.Text())
	}

	if blogID != "" {
		cfg.BlogID = blogID
	} else {
		fmt.Fprint(cmd.OutOrStdout(), "Blog ID (e.g. example.hateblo.jp): ")
		if !scanner.Scan() {
			return nil, fmt.Errorf("read blog ID: %w", scanner.Err())
		}
		cfg.BlogID = strings.TrimSpace(scanner.Text())
	}

	fmt.Fprint(cmd.OutOrStdout(), "API Key: ")
	apiKey, err := readPassword(cmd, scanner)
	if err != nil {
		return nil, fmt.Errorf("read API key: %w", err)
	}
	cfg.APIKey = apiKey

	return cfg, nil
}

// readPassword reads a password from stdin.
// When stdin is a terminal, it switches to raw mode and echoes '*' for each character typed.
// Backspace removes the last character, Ctrl+C returns an error.
// Falls back to plain text reading via scanner when stdin is not a terminal (e.g. pipe).
func readPassword(cmd *cobra.Command, scanner *bufio.Scanner) (string, error) {
	// Try to get the file descriptor of stdin for terminal detection.
	type fder interface{ Fd() uintptr }
	if f, ok := cmd.InOrStdin().(fder); ok {
		fd := int(f.Fd())
		if term.IsTerminal(fd) {
			oldState, err := term.MakeRaw(fd)
			if err != nil {
				return "", err
			}
			defer term.Restore(fd, oldState) //nolint:errcheck

			out := cmd.OutOrStdout()
			var buf []byte
			b := make([]byte, 1)
			for {
				if _, err := cmd.InOrStdin().Read(b); err != nil {
					return "", err
				}
				switch b[0] {
				case '\r', '\n':
					fmt.Fprintln(out)
					return string(buf), nil
				case '\x7f', '\b': // Backspace
					if len(buf) > 0 {
						buf = buf[:len(buf)-1]
						fmt.Fprint(out, "\b \b")
					}
				case '\x03': // Ctrl+C
					fmt.Fprintln(out)
					return "", fmt.Errorf("interrupted")
				default:
					buf = append(buf, b[0])
					fmt.Fprint(out, "*")
				}
			}
		}
	}
	// Non-terminal fallback: read via scanner (no masking).
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return strings.TrimSpace(scanner.Text()), nil
}
