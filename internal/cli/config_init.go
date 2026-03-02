package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hirano00o/hb/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newConfigInitCmd() *cobra.Command {
	var hatenaID, blogID string
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize hb configuration (project-local by default, global with -g)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if global {
				return runGlobalInit(cmd, hatenaID, blogID)
			}
			return runProjectInit(cmd, hatenaID, blogID)
		},
	}

	cmd.Flags().StringVar(&hatenaID, "hatena-id", "", "Hatena ID")
	cmd.Flags().StringVar(&blogID, "blog-id", "", "Blog ID, e.g. example.hateblo.jp")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Initialize the global config (~/.config/hb/config.yaml)")

	return cmd
}

func runGlobalInit(cmd *cobra.Command, hatenaID, blogID string) error {
	path, err := config.GlobalConfigPath()
	if err != nil {
		return err
	}
	// Use a single scanner for all stdin reads to avoid buffering issues.
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if _, err := os.Stat(path); err == nil {
		ok, err := confirmActionWithScanner(cmd, scanner, fmt.Sprintf("%s already exists. Overwrite? [y/N]: ", path))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
	}
	cfg, err := promptConfigWithScanner(cmd, scanner, hatenaID, blogID)
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Global config saved to %s\n", path)
	return nil
}

func runProjectInit(cmd *cobra.Command, hatenaID, blogID string) error {
	const projectConfigFile = ".hb/config.yaml"
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if _, err := os.Stat(projectConfigFile); err == nil {
		ok, err := confirmActionWithScanner(cmd, scanner, fmt.Sprintf("%s already exists. Overwrite? [y/N]: ", projectConfigFile))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
	}
	cfg, err := promptConfigWithScanner(cmd, scanner, hatenaID, blogID)
	if err != nil {
		return err
	}
	if err := config.Save(projectConfigFile, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Project config created at %s\n", projectConfigFile)
	fmt.Fprintln(cmd.OutOrStdout(), "Edit it to override global settings for this project.")
	return nil
}

// promptConfigWithScanner reads config fields interactively.
// For project configs, any field may be left empty (empty Enter skips the field, which is
// then omitted from the YAML file via omitempty so the global config takes precedence).
// For global configs, the caller is responsible for calling config.Validate after this returns.
func promptConfigWithScanner(cmd *cobra.Command, scanner *bufio.Scanner, hatenaID, blogID string) (*config.Config, error) {
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
	if apiKey != "" {
		cfg.APIKey = apiKey
	}

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
		if err := scanner.Err(); err != nil {
			return "", err
		}
		// scanner.Err() == nil means EOF; treat as missing input rather than success.
		return "", io.ErrUnexpectedEOF
	}
	return strings.TrimSpace(scanner.Text()), nil
}
