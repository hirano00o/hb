package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/hirano00o/hb/config"
	"github.com/spf13/cobra"
)

func newConfigInitCmd() *cobra.Command {
	var hatenaID, blogID, apiKey string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the global hb configuration (~/.config/hb/config.yaml)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := &config.Config{}
			if hatenaID != "" && blogID != "" && apiKey != "" {
				cfg.HatenaID = hatenaID
				cfg.BlogID = blogID
				cfg.APIKey = apiKey
			} else {
				var err error
				cfg, err = promptConfig(cmd, hatenaID, blogID, apiKey)
				if err != nil {
					return err
				}
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
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Hatena Blog API key")

	return cmd
}

func promptConfig(cmd *cobra.Command, hatenaID, blogID, apiKey string) (*config.Config, error) {
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

	if apiKey != "" {
		cfg.APIKey = apiKey
	} else {
		fmt.Fprint(cmd.OutOrStdout(), "API Key: ")
		if !scanner.Scan() {
			return nil, fmt.Errorf("read API key: %w", scanner.Err())
		}
		cfg.APIKey = strings.TrimSpace(scanner.Text())
	}

	return cfg, nil
}
