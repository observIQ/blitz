package main

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCommand creates the completion command for shell autocompletion
func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
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
			// Register completion functions for flags with enum values
			registerFlagCompletions(cmd.Root())

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
	return cmd
}

// registerFlagCompletions registers completion functions for flags with specific valid values
func registerFlagCompletions(rootCmd *cobra.Command) {
	// Generator type completions
	_ = rootCmd.RegisterFlagCompletionFunc("generator-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"nop", "json", "winevt", "palo-alto", "apache-common", "apache-combined", "apache-error", "nginx", "postgres", "kubernetes"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Output type completions
	_ = rootCmd.RegisterFlagCompletionFunc("output-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"nop", "stdout", "tcp", "udp", "syslog", "otlp-grpc", "file"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Logging type completions
	_ = rootCmd.RegisterFlagCompletionFunc("logging-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"stdout", "file"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Log level completions
	_ = rootCmd.RegisterFlagCompletionFunc("logging-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Generator JSON type completions
	_ = rootCmd.RegisterFlagCompletionFunc("generator-json-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"default", "pii"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Generator Kubernetes format completions
	_ = rootCmd.RegisterFlagCompletionFunc("generator-kubernetes-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"cri-o"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Syslog transport completions
	_ = rootCmd.RegisterFlagCompletionFunc("output-syslog-transport", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"tcp", "udp"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Syslog RFC completions
	_ = rootCmd.RegisterFlagCompletionFunc("output-syslog-rfc", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"3164", "5424"}, cobra.ShellCompDirectiveNoFileComp
	})

	// TLS min version completions
	_ = rootCmd.RegisterFlagCompletionFunc("output-tcp-tls-min-version", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"1.2", "1.3"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("output-syslog-tls-min-version", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"1.2", "1.3"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("otlp-grpc-tls-min-version", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"1.2", "1.3"}, cobra.ShellCompDirectiveNoFileComp
	})
}
