package main

import (
	"fmt"
	"strings"

	"github.com/observiq/blitz/generator/filegen"
	"github.com/spf13/cobra"
)

// newLibraryCommand builds the `blitz library` group for inspecting and
// extracting the filegen data library.
func newLibraryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Inspect and extract the filegen data library",
	}
	cmd.AddCommand(
		newLibraryLsCommand(),
		newLibrarySearchCommand(),
		newLibraryShowCommand(),
		newLibraryPathCommand(),
		newLibraryDiffCommand(),
		newLibraryExtractCommand(),
	)
	return cmd
}

func errNoLibrary() error {
	return fmt.Errorf("no data library found; install the blitz package, run from a repo checkout, or set BLITZ_DATA_LIBRARY_DIR")
}

func markOverride(name string, override bool) string {
	if override {
		return name + " (override)"
	}
	return name
}

func newLibraryLsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [package]",
		Short: "List packages, or the files in a package",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lib := filegen.ResolveLibrary()
			w := cmd.OutOrStdout()
			if len(args) == 1 {
				files, err := lib.Files(args[0])
				if err != nil {
					return err
				}
				for _, f := range files {
					fmt.Fprintln(w, markOverride(f.Name, f.Overrides()))
				}
				return nil
			}
			pkgs, err := lib.Packages()
			if err != nil {
				return err
			}
			if len(pkgs) == 0 {
				return errNoLibrary()
			}
			for _, p := range pkgs {
				fmt.Fprintln(w, markOverride(p.Name, p.Overrides()))
			}
			return nil
		},
	}
}

func newLibrarySearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "search <term>",
		Short: "List packages whose name contains the term",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgs, err := filegen.ResolveLibrary().Search(args[0])
			if err != nil {
				return err
			}
			for _, p := range pkgs {
				fmt.Fprintln(cmd.OutOrStdout(), markOverride(p.Name, p.Overrides()))
			}
			return nil
		},
	}
}

func newLibraryShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <package>",
		Short: "Print a package's sample lines",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := filegen.ResolveLibrary().Show(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newLibraryPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the active data library source",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			src := filegen.ResolveLibrary().ActiveSource()
			if src == "" {
				return errNoLibrary()
			}
			fmt.Fprintln(cmd.OutOrStdout(), src)
			return nil
		},
	}
}

func newLibraryDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff [package]",
		Short: "Show how the on-disk library differs from the embedded baseline",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := filegen.ResolveLibrary().Diff()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			shown := 0
			for _, e := range entries {
				if len(args) == 1 && !strings.HasPrefix(e.Path, args[0]+"/") {
					continue
				}
				fmt.Fprintf(w, "%-9s %s\n", e.Status, e.Path)
				shown++
			}
			if shown == 0 {
				fmt.Fprintln(w, "no differences (no on-disk overrides, or no embedded baseline to compare)")
			}
			return nil
		},
	}
}

func newLibraryExtractCommand() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "extract [package] [dest]",
		Short: "Copy a package (or the whole library with --all) to dest/data_library",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			lib := filegen.ResolveLibrary()
			if all {
				dest := "."
				if len(args) >= 1 {
					dest = args[0]
				}
				return lib.ExtractAll(dest)
			}
			if len(args) == 0 {
				return fmt.Errorf("extract requires a package name, or --all")
			}
			dest := "."
			if len(args) >= 2 {
				dest = args[1]
			}
			return lib.Extract(args[0], dest)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "extract the entire library")
	return cmd
}
