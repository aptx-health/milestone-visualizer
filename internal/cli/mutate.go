package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/config"
	"github.com/aptx-health/ms-visualizer/internal/graph"
)

func newGraphMutateGroup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph-edit",
		Short: "Mutate the milestone dependency graph (canonical Mermaid rewrite)",
	}
	cmd.AddCommand(newAddEdgeCmd())
	cmd.AddCommand(newRmEdgeCmd())
	cmd.AddCommand(newAddNodeCmd())
	cmd.AddCommand(newRmNodeCmd())
	cmd.AddCommand(newFmtCmd())
	return cmd
}

// newFmtCmd canonicalizes the graph doc's Mermaid block without changing its
// meaning: parse → render → replace, with no mutation in between. This lets
// agents normalize a hand-written block so their first real mutation produces
// a clean, minimal diff instead of a whole-block rewrite.
func newFmtCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "fmt",
		Short: "Rewrite the graph doc's Mermaid block in canonical form (no semantic change)",
		Long: "Parse the milestone graph's Mermaid block and write it back in canonical\n" +
			"form (nodes declared first, then bare edges), changing nothing semantically.\n\n" +
			"Exits 0 whether or not the file changed. With --check, the file is left\n" +
			"untouched and the command exits non-zero when the doc is not already\n" +
			"canonical — suitable for a CI gate.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := resolveGraphFile(cmd)
			if err != nil {
				return err
			}
			orig, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read graph file: %w", err)
			}
			formatted, changed, err := canonicalizeDoc(string(orig))
			if err != nil {
				return fmt.Errorf("canonicalize %s: %w", file, err)
			}
			quiet, _ := cmd.Root().PersistentFlags().GetBool("quiet")

			if check {
				if changed {
					if !quiet {
						fmt.Printf("%s is not in canonical form (run: msv graph-edit fmt --file %s)\n", file, file)
					}
					os.Exit(ExitLintFindings)
				}
				if !quiet {
					fmt.Printf("%s is already canonical\n", file)
				}
				return nil
			}

			if !changed {
				if !quiet {
					fmt.Printf("%s already canonical\n", file)
				}
				return nil
			}
			if err := os.WriteFile(file, []byte(formatted), 0o644); err != nil {
				return fmt.Errorf("write graph file: %w", err)
			}
			if !quiet {
				fmt.Printf("formatted %s\n", file)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false,
		"do not write; exit non-zero if the doc is not already canonical (CI gate)")
	return cmd
}

// canonicalizeDoc parses the Mermaid graph block out of a full markdown doc
// and rewrites it in canonical form without changing its meaning. It returns
// the canonicalized document, whether it differs from the original, and any
// parse error. It performs no I/O and never mutates the graph.
func canonicalizeDoc(orig string) (formatted string, changed bool, err error) {
	block, err := graph.ExtractBlock(orig)
	if err != nil {
		return "", false, fmt.Errorf("extract graph block: %w", err)
	}
	g, err := graph.Parse(block)
	if err != nil {
		return "", false, fmt.Errorf("parse graph block: %w", err)
	}
	formatted, err = graph.ReplaceInDoc(orig, graph.Render(g))
	if err != nil {
		return "", false, fmt.Errorf("render canonical graph: %w", err)
	}
	return formatted, formatted != orig, nil
}

// resolveGraphFile picks the graph markdown file from the --file flag,
// falling back to graph_file in the loaded config.
func resolveGraphFile(cmd *cobra.Command) (string, error) {
	cfgPath, _ := cmd.Root().PersistentFlags().GetString("config")
	c, _, err := config.Load(cfgPath)
	if err != nil {
		return "", err
	}
	file, _ := cmd.Flags().GetString("file")
	if file == "" {
		file = c.GraphFile
	}
	if file == "" {
		return "", fmt.Errorf("graph file not resolved: pass --file or set graph_file in .msv.yaml")
	}
	return file, nil
}

func newAddEdgeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-edge <from> <to>",
		Short: "Add a dependency edge (from must land before to)",
		Args:  cobra.ExactArgs(2),
		RunE:  runMutate(func(g *graph.Graph, args []string) error {
			from, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			to, err := strconv.Atoi(args[1])
			if err != nil {
				return err
			}
			g.AddEdge(from, to)
			return nil
		}),
	}
}

func newRmEdgeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm-edge <from> <to>",
		Short: "Remove a dependency edge",
		Args:  cobra.ExactArgs(2),
		RunE: runMutate(func(g *graph.Graph, args []string) error {
			from, _ := strconv.Atoi(args[0])
			to, _ := strconv.Atoi(args[1])
			if !g.RemoveEdge(from, to) {
				return fmt.Errorf("edge %d -> %d not found", from, to)
			}
			return nil
		}),
	}
}

func newAddNodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-node <number> <label...>",
		Short: "Add or update a node label",
		Args:  cobra.MinimumNArgs(2),
		RunE: runMutate(func(g *graph.Graph, args []string) error {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			label := strings.Join(args[1:], " ")
			g.AddNode(n, label)
			return nil
		}),
	}
}

func newRmNodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm-node <number>",
		Short: "Remove a node and any edges touching it",
		Args:  cobra.ExactArgs(1),
		RunE: runMutate(func(g *graph.Graph, args []string) error {
			n, _ := strconv.Atoi(args[0])
			if !g.RemoveNode(n) {
				return fmt.Errorf("node %d not found", n)
			}
			return nil
		}),
	}
}

// runMutate loads the graph file resolved from config/flags, runs the
// mutation, then writes the file back with a canonical Mermaid block.
func runMutate(op func(g *graph.Graph, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		file, err := resolveGraphFile(cmd)
		if err != nil {
			return err
		}
		doc, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read graph file: %w", err)
		}
		block, err := graph.ExtractBlock(string(doc))
		if err != nil {
			// Empty graph is fine — mutation will create one.
			block = "flowchart LR\n"
		}
		g, err := graph.Parse(block)
		if err != nil {
			return err
		}
		if err := op(g, args); err != nil {
			return err
		}
		newDoc, err := graph.ReplaceInDoc(string(doc), graph.Render(g))
		if err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(newDoc), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", file)
		return nil
	}
}
