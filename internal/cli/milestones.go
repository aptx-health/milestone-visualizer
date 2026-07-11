package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
			r, err := resolveRepo(cmd, args)
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
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"owner":      owner,
					"repo":       repo,
					"state":      state,
					"milestones": milestones,
					"count":      len(milestones),
				})
			}
			renderMilestonesText(milestones)
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "filter by state: open, closed, or all (default: all)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

var (
	milestoneHdr  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	milestoneOk   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	milestoneWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	milestoneDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func renderMilestonesText(milestones []gh.MilestoneInfo) {
	fmt.Println(milestoneHdr.Render("Milestones:"))
	fmt.Println()

	// Table header
	fmt.Printf("%-6s %-20s %-8s %8s %8s  %s\n",
		milestoneHdr.Render("#"),
		milestoneHdr.Render("title"),
		milestoneHdr.Render("state"),
		milestoneHdr.Render("open"),
		milestoneHdr.Render("closed"),
		milestoneHdr.Render("due"))
	fmt.Println(milestoneDim.Render(strings.Repeat("─", 6+22+10+10+12+40)))

	for _, m := range milestones {
		stateStr := "open"
		stateStyle := milestoneOk
		if m.State == "closed" {
			stateStr = "closed"
			stateStyle = milestoneDim
		}
		dueStr := ""
		if m.DueOn != nil {
			t, err := time.Parse(time.RFC3339, *m.DueOn)
			if err == nil {
				dueStr = t.Format("2006-01-02")
			}
		}
		fmt.Printf("  %-5d %-20s %-8s %8d %8d  %s\n",
			m.Number,
			truncate(m.Title, 20),
			stateStyle.Render(stateStr),
			m.OpenIssues,
			m.ClosedIssues,
			milestoneDim.Render(dueStr))
	}
}
