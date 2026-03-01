package cli

import (
	"fmt"

	"github.com/hirano00o/hb/config"
	"github.com/hirano00o/hb/hatena"
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
