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
	return cmd
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
		cfgPath, _ := cmd.Root().PersistentFlags().GetString("config")
		c, _, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		// Allow --file to override.
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			file = c.GraphFile
		}
		if file == "" {
			return fmt.Errorf("graph file not resolved: pass --file or set graph_file in .msv.yaml")
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

// attach --file to each mutation subcommand
func init() {
	// no-op — we add the flag in the group parent for simplicity
}
