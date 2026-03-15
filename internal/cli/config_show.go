package cli

import (
	"fmt"
	"strings"

	"github.com/hirano00o/hb/config"
	"github.com/spf13/cobra"
)

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show merged configuration (global → project → env vars)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadMerged()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "hatena_id: %s\n", cfg.HatenaID)
			fmt.Fprintf(out, "blog_id:   %s\n", cfg.BlogID)
			fmt.Fprintf(out, "api_key:   %s\n", maskAPIKey(cfg.APIKey))
			if cfg.Concurrency != nil {
				fmt.Fprintf(out, "concurrency: %d\n", *cfg.Concurrency)
			} else {
				fmt.Fprintf(out, "concurrency: %d (default)\n", defaultConcurrency)
			}
			if cfg.MaxPages != nil && *cfg.MaxPages > 0 {
				fmt.Fprintf(out, "max_pages: %d\n", *cfg.MaxPages)
			} else {
				fmt.Fprintf(out, "max_pages: unlimited\n")
			}
			return nil
		},
	}
}

// maskAPIKey masks all but the last 4 characters of the API key.
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}
