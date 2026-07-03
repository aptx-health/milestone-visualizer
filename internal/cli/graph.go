package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/graph"
	"github.com/aptx-health/ms-visualizer/internal/msview"
)

func newGraphCmd() *cobra.Command {
	var milestone, file string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "graph [<owner>/<repo>]",
		Short: "Parse a milestone's Mermaid dep graph and overlay live issue status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := resolve(cmd, args, milestone, file)
			if err != nil {
				return err
			}
			snap, err := loadSnapshot(ctx, cmd, r)
			if err != nil {
				return err
			}
			report := snap.Reports.Graph
			if r.GraphFile == "" {
				doc, err := loadDoc(r.GraphFile)
				if err != nil {
					return err
				}
				block, err := graph.ExtractBlock(doc)
				if err != nil {
					return fmt.Errorf("read graph: %w", err)
				}
				g, err := graph.Parse(block)
				if err != nil {
					return err
				}
				report = msview.BuildGraphReport(snap.Owner, snap.Repo, snap.Milestone, g, snap.Items)
				report.FetchedAt = snap.FetchedAt
			}

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderGraphText(report)
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone title or number (or set in .msv.yaml)")
	cmd.Flags().StringVarP(&file, "file", "f", "", "markdown file with the mermaid block (or set graph_file in .msv.yaml)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

func loadDoc(file string) (string, error) {
	if file == "" {
		fi, err := os.Stdin.Stat()
		if err == nil && fi.Mode()&os.ModeCharDevice == 0 {
			b, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
		return "", fmt.Errorf("no --file provided and nothing on stdin")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func renderGraphText(r msview.GraphReport) {
	fmt.Println(hdr.Render("Milestone: " + r.Milestone))
	fmt.Println(dim.Render(fmt.Sprintf("%d nodes · %d edges", len(r.Nodes), len(r.Edges))))
	fmt.Println()

	byLayer := map[int][]msview.GraphNodeView{}
	for _, n := range r.Nodes {
		byLayer[n.Layer] = append(byLayer[n.Layer], n)
	}
	for i := 0; i < len(r.Layers); i++ {
		nodes := byLayer[i]
		sort.Slice(nodes, func(a, b int) bool { return nodes[a].Number < nodes[b].Number })
		fmt.Println(hdr.Render(fmt.Sprintf("Layer %d", i)))
		for _, n := range nodes {
			fmt.Println("  " + renderNodeText(n))
		}
		if i < len(r.Layers)-1 {
			fmt.Println(dim.Render("   │"))
		}
	}

	fmt.Println()
	fmt.Println(hdr.Render("Edges (blocked → unlocks)"))
	for _, e := range r.Edges {
		arrow := "→"
		switch e.State {
		case "ready":
			arrow = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("→ ready")
		case "blocked":
			arrow = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("→ blocked")
		}
		fmt.Printf("  #%d %s #%d\n", e.From, arrow, e.To)
	}
}

func renderNodeText(n msview.GraphNodeView) string {
	glyph := "○"
	style := info
	suffix := ""

	if !n.InMilestone {
		style = dim
		return fmt.Sprintf("%s %s  %s", style.Render(glyph),
			style.Render(fmt.Sprintf("#%d", n.Number)),
			style.Render(strings.TrimPrefix(n.Label, fmt.Sprintf("#%d ", n.Number))+"  (not in milestone)"))
	}
	if n.PR != nil {
		switch n.PR.State {
		case msview.PRMerged:
			glyph, style = "●", ok
			suffix = ok.Render(fmt.Sprintf("  PR #%d merged", n.PR.Number))
		case msview.PRClosed:
			glyph, style = "○", dim
			suffix = dim.Render(fmt.Sprintf("  PR #%d closed", n.PR.Number))
		case msview.PRDraft:
			glyph, style = "◐", warn
			suffix = warn.Render(fmt.Sprintf("  PR #%d draft", n.PR.Number))
		default:
			glyph, style = "◑", info
			suffix = info.Render(fmt.Sprintf("  PR #%d open", n.PR.Number))
		}
	} else if n.IssueClosed {
		glyph, style = "●", ok
	}

	label := truncate(strings.TrimPrefix(n.Label, fmt.Sprintf("#%d ", n.Number)), 40)
	return fmt.Sprintf("%s %s  %-40s  %s%s",
		style.Render(glyph),
		style.Render(fmt.Sprintf("#%d", n.Number)),
		label,
		highlightLabels(n.Labels),
		suffix,
	)
}
