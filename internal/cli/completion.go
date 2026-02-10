// Package cli implements the command-line interface for Reglet.
package cli

import (
	"github.com/spf13/cobra"
)

// newCompletionCommand creates the "completion" command that generates
// shell completion scripts for bash, zsh, fish, and powershell.
//
// Usage:
//
//	cli completion bash > /etc/bash_completion.d/cli
//	cli completion zsh > ~/.zsh/completions/_cli
//	cli completion fish > ~/.config/fish/completions/cli.fish
//	cli completion powershell > cli.ps1
func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for cli.

To load completions:

Bash:
  $ source <(cli completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ cli completion bash > /etc/bash_completion.d/cli
  # macOS:
  $ cli completion bash > $(brew --prefix)/etc/bash_completion.d/cli

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ cli completion zsh > "${fpath[1]}/_cli"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ cli completion fish | source

  # To load completions for each session, execute once:
  $ cli completion fish > ~/.config/fish/completions/cli.fish

PowerShell:
  PS> cli completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> cli completion powershell > cli.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			default:
				// This shouldn't happen due to ValidArgs, but handle it anyway
				return cmd.Help()
			}
		},
	}

	return cmd
}

// registerOutputFormatCompletion registers tab completion for the --output flag
// on the root command. This provides completions for "table", "json", "yaml".
func registerOutputFormatCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"table\tHuman-readable table (default)",
			"json\tJSON output for scripting",
			"yaml\tYAML output",
		}, cobra.ShellCompDirectiveNoFileComp
	})
}
