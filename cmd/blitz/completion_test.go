package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/observiq/blitz/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFilegenModeCompletion pins that shell completion offers the filegen
// mode values the config accepts.
func TestFilegenModeCompletion(t *testing.T) {
	root := &cobra.Command{Use: "blitz"}
	root.Flags().String("generator-filegen-mode", "", "")
	registerFlagCompletions(root)

	complete, ok := root.GetFlagCompletionFunc("generator-filegen-mode")
	require.True(t, ok)
	got, _ := complete(root, nil, "")
	assert.Equal(t, []string{"random", "replay"}, got)
}

// TestTypeCompletionCoversEveryType pins that generator and output type
// completion offers every type the config accepts.
func TestTypeCompletionCoversEveryType(t *testing.T) {
	root := &cobra.Command{Use: "blitz"}
	root.Flags().String("generator-type", "", "")
	root.Flags().String("output-type", "", "")
	registerFlagCompletions(root)

	for flag, want := range map[string][]string{
		"generator-type": {"nop", "json", "winevt", "wel", "palo-alto", "apache-common", "apache-combined",
			"apache-error", "nginx", "postgres", "kubernetes", "filegen", "okta", "hostmetrics", "traces", "fix", "flow", "f5"},
		"output-type": {"nop", "stdout", "tcp", "udp", "syslog", "otlp-grpc", "file", "hec", "flow",
			"prometheus-remote-write", "prometheus-scrape"},
	} {
		complete, ok := root.GetFlagCompletionFunc(flag)
		require.True(t, ok, flag)
		got, _ := complete(root, nil, "")
		assert.ElementsMatch(t, want, got, flag)
	}
}

// TestShellCompletionOffersValues pins completion as a shell runs it: TAB
// calls the hidden __complete command, not the completion command, so the
// value lists must be registered on the root itself.
func TestShellCompletionOffersValues(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{cobra.ShellCompNoDescRequestCmd, "--generator-filegen-mode", ""})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})
	require.NoError(t, rootCmd.Execute())
	got := strings.Fields(out.String())
	assert.Contains(t, got, "random")
	assert.Contains(t, got, "replay")
}

// TestCompletionCoversEveryChoice pins that every flag whose usage lists its
// values ("One of: a|b" or "One or more of: a|b") completes to that list, so
// completion and --help cannot drift apart.
func TestCompletionCoversEveryChoice(t *testing.T) {
	for _, o := range config.DefaultOverrides() {
		want := o.Choices()
		if want == nil {
			continue
		}
		complete, ok := rootCmd.GetFlagCompletionFunc(o.Flag)
		require.True(t, ok, o.Flag)
		got, _ := complete(rootCmd, nil, "")
		assert.Equal(t, want, got, o.Flag)
	}
}
