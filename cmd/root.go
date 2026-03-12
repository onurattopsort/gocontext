package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/onurattopsort/gocontext/internal/inspect"
)

var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "gocontext",
	Short: "AST extraction tool for progressive disclosure of Go codebases",
	Long: `gocontext provides structured access to Go source code for LLM agents.
It supports progressive disclosure from high-level package trees down to
exact function bodies, using go/parser and go/ast under the hood.`,
}

var treeCmd = &cobra.Command{
	Use:   "tree [dir]",
	Short: "List packages with their doc synopses",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		summaries, err := inspect.Tree(dir)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(summaries)
		}

		if len(summaries) == 0 {
			fmt.Println("No Go packages found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, s := range summaries {
			synopsis := s.Synopsis
			if synopsis == "" {
				synopsis = "(no doc)"
			}
			fmt.Fprintf(w, "%s\t%s\n", s.ImportPath, synopsis)
		}
		w.Flush()
		return nil
	},
}

var packageCmd = &cobra.Command{
	Use:   "package <import_path>",
	Short: "Show exported symbols and docs for a package",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := inspect.ResolveImportPath(args[0])
		if err != nil {
			return err
		}

		detail, err := inspect.Package(dir)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(detail)
		}

		// Package header.
		fmt.Printf("package %s\n", detail.Name)
		fmt.Printf("import %q\n", detail.ImportPath)
		if detail.Doc != "" {
			fmt.Printf("\n%s\n", detail.Doc)
		}

		// Types.
		for _, t := range detail.Types {
			fmt.Printf("\n── %s %s ──\n", t.Kind, t.Name)
			if t.Doc != "" {
				fmt.Printf("%s\n", t.Doc)
			}
			for _, f := range t.Funcs {
				fmt.Printf("\n  %s\n", f.Signature)
				if f.Doc != "" {
					fmt.Printf("    %s\n", indent(f.Doc, "    "))
				}
			}
			for _, m := range t.Methods {
				fmt.Printf("\n  %s\n", m.Signature)
				if m.Doc != "" {
					fmt.Printf("    %s\n", indent(m.Doc, "    "))
				}
			}
		}

		// Top-level functions.
		if len(detail.Funcs) > 0 {
			fmt.Printf("\n── functions ──\n")
			for _, f := range detail.Funcs {
				fmt.Printf("\n  %s\n", f.Signature)
				if f.Doc != "" {
					fmt.Printf("    %s\n", indent(f.Doc, "    "))
				}
			}
		}

		return nil
	},
}

var symbolCmd = &cobra.Command{
	Use:   "symbol <import_path> <symbol_name>",
	Short: "Show the definition of a specific symbol",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := inspect.ResolveImportPath(args[0])
		if err != nil {
			return err
		}

		src, err := inspect.Symbol(dir, args[1])
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(map[string]string{
				"symbol": args[1],
				"source": src,
			})
		}

		fmt.Println(src)
		return nil
	},
}

var bodyCmd = &cobra.Command{
	Use:   "body <import_path> <function_or_method>",
	Short: "Show the full source of a function or method (use Type.Method for methods)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := inspect.ResolveImportPath(args[0])
		if err != nil {
			return err
		}

		src, err := inspect.Body(dir, args[1])
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(map[string]string{
				"name":   args[1],
				"source": src,
			})
		}

		fmt.Println(src)
		return nil
	},
}

var refsCmd = &cobra.Command{
	Use:   "refs <dir> <symbol_name>",
	Short: "Find all references to a symbol across the codebase",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		refs, err := inspect.Refs(args[0], args[1])
		if err != nil {
			return err
		}

		if len(refs) == 0 {
			if jsonOutput {
				return printJSON([]struct{}{})
			}
			fmt.Printf("No references to %q found.\n", args[1])
			return nil
		}

		if jsonOutput {
			return printJSON(refs)
		}

		fmt.Printf("References to %q (%d found):\n\n", args[1], len(refs))
		for _, r := range refs {
			fmt.Printf("  %s:%d:%d [%s]\n", r.File, r.Line, r.Column, r.Kind)
			fmt.Printf("    %s\n\n", r.Context)
		}
		return nil
	},
}

var overviewCmd = &cobra.Command{
	Use:   "overview [dir]",
	Short: "Single-call summary of all packages, types, and function signatures",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		overview, err := inspect.Overview(dir)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(overview)
		}

		if len(overview) == 0 {
			fmt.Println("No Go packages found.")
			return nil
		}

		for _, pkg := range overview {
			fmt.Printf("── %s ──\n", pkg.ImportPath)
			if pkg.Synopsis != "" {
				fmt.Printf("  %s\n", pkg.Synopsis)
			}
			if len(pkg.Types) > 0 {
				fmt.Printf("  types:  %s\n", strings.Join(pkg.Types, ", "))
			}
			if len(pkg.Funcs) > 0 {
				for _, sig := range pkg.Funcs {
					fmt.Printf("  %s\n", sig)
				}
			}
			fmt.Println()
		}

		return nil
	},
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= 1 {
		return s
	}
	for i := 1; i < len(lines); i++ {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(packageCmd)
	rootCmd.AddCommand(symbolCmd)
	rootCmd.AddCommand(bodyCmd)
	rootCmd.AddCommand(refsCmd)
	rootCmd.AddCommand(overviewCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
