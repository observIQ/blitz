package main

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCommand creates the completion command for shell autocompletion
func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "completion [bash|zsh|fish|powershell]",
		Short:  "Generate shell completion script",
		Hidden: true,
		Long: `Generate the autocompletion script for blitz for the specified shell.

To load completions:

Bash:
  $ source <(blitz completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ blitz completion bash > /etc/bash_completion.d/blitz
  # macOS:
  $ blitz completion bash > $(brew --prefix)/etc/bash_completion.d/blitz

Zsh:
  $ source <(blitz completion zsh)

  # To load completions for each session, execute once:
  $ blitz completion zsh > "${fpath[1]}/_blitz"

Fish:
  $ blitz completion fish | source

  # To load completions for each session, execute once:
  $ blitz completion fish > ~/.config/fish/completions/blitz.fish

PowerShell:
  PS> blitz completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> blitz completion powershell > blitz.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				_ = cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}
}
