package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/gh"
)

func newMilestonesCmd() *cobra.Command {
	var state string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "milestones [<owner>/<repo>]",
		Short: "List a repo's milestones with open/closed issue counts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			r, err := resolve(cmd, args, "", "")
			if err != nil {
				return err
			}
			owner, repo, err := gh.ParseOwnerRepo(r.OwnerRepo)
			if err != nil {
				return err
			}
			client, err := gh.NewClient(ctx)
			if err != nil {
				return err
			}
			milestones, err := gh.ListMilestones(ctx, client, owner, repo, state)
			if err != nil {
				return err
			}
			if asJSON || os.Getenv("TEST_MILESTONES_JSON") == "1" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(milestones)
			}
			renderMilestonesText(milestones, os.Stdout)
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "filter by state: open, closed, or all (default: all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

var (
	mlHdr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	mlOk  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	mlDim = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// renderSeparator builds a separator matching the column widths of renderMilestonesText
func renderSeparator() string {
	return fmt.Sprintf("%-5s %-18s %-8s %6s %7s  %-21s  %s",
		strings.Repeat("─", 5),
		strings.Repeat("─", 18),
		strings.Repeat("─", 8),
		strings.Repeat("─", 6),
		strings.Repeat("─", 7),
		strings.Repeat("─", 21),
		"─")
}

func renderMilestonesText(milestones []gh.MilestoneInfo, w io.Writer) {
	fmt.Fprintln(w, mlHdr.Render("Milestones:"))
	fmt.Fprintln(w)

	// Table header — style each cell individually so ANSI codes don't break width calculation
	fmt.Fprintf(w, "  %-5s %-18s %-8s %6s %7s  %-21s  %s\n",
		mlHdr.Render("#"),
		mlHdr.Render("title"),
		mlHdr.Render("state"),
		mlHdr.Render("open"),
		mlHdr.Render("closed"),
		mlHdr.Render("due"),
		mlHdr.Render("description"))
	fmt.Fprintln(w, mlDim.Render(strings.Repeat("─", 120)))

	for _, m := range milestones {
		stateStr := "open"
		stateStyle := mlOk
		if m.State == "closed" {
			stateStr = "closed"
			stateStyle = mlDim
		}
		dueStr := ""
		if m.DueOn != nil {
			dueStr = *m.DueOn
			if len(dueStr) > 10 {
				dueStr = dueStr[:10] // "2026-01-02"
			}
		}
		fmt.Fprintf(w, "  %-5d %-18s %-8s %6d %7d  %-21s  %s\n",
			m.Number,
			truncate(m.Title, 18),
			stateStyle.Render(stateStr),
			m.OpenIssues,
			m.ClosedIssues,
			mlDim.Render(dueStr),
			mlDim.Render(truncate(m.Description, 31)))
	}
}
