// Package cli wires up all hb subcommands using cobra.
package cli

import (
	"github.com/spf13/cobra"
)

// newCompletionCmd returns a cobra command that generates shell completion scripts.
// Delegating to cobra's built-in generators avoids duplicating completion logic
// and ensures completions stay in sync with the command tree automatically.
func newCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate shell completion script for hb.

To enable completions:

  bash:
    $ source <(hb completion bash)

  zsh:
    # If shell completion is not already enabled in your environment you will need
    # to enable it.  You can execute the following once:
    $ echo "autoload -U compinit; compinit" >> ~/.zshrc
    $ source <(hb completion zsh)

  fish:
    $ hb completion fish | source

  powershell:
    PS> hb completion powershell | Out-String | Invoke-Expression
`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
}
