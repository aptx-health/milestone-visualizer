package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/msview"
)

func newStatusCmd() *cobra.Command {
	var milestone string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status [<owner>/<repo>]",
		Short: "Render a burn-down table of a milestone's issues + linked PRs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := resolve(cmd, args, milestone, "")
			if err != nil {
				return err
			}
			snap, err := loadSnapshot(ctx, cmd, r)
			if err != nil {
				return err
			}
			report := snap.Reports.Status

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderStatusText(report)
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone title or number (or set in .msv.yaml)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	return cmd
}

var (
	hdr    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ok     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	danger = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	info   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

var trackedLabels = []string{
	"needs-review", "blocked", "agent-ready", "autopilot", "in-progress",
	"bug", "spike", "ux", "design", "no-agent",
}

func renderStatusText(r msview.StatusReport) {
	fmt.Println(hdr.Render(fmt.Sprintf("Milestone: %s  (%s/%s)", r.Milestone, r.Owner, r.Repo)))
	fmt.Println(dim.Render(fmt.Sprintf(
		"issues: %d closed / %d open · PRs: %d merged / %d open / %d draft",
		r.Summary.IssuesClosed, r.Summary.IssuesOpen,
		r.Summary.PRsMerged, r.Summary.PRsOpen, r.Summary.PRsDraft)))
	fmt.Println()

	titleW := 46
	for _, iv := range r.Issues {
		if l := len(iv.Title); l > titleW {
			titleW = l
		}
	}
	if titleW > 60 {
		titleW = 60
	}

	fmt.Printf("%-6s %-6s %-*s  %-24s  %s\n",
		hdr.Render("#"), hdr.Render("state"), titleW, hdr.Render("title"),
		hdr.Render("labels"), hdr.Render("linked PRs"))
	fmt.Println(dim.Render(strings.Repeat("─", 6+7+titleW+26+22)))

	for _, iv := range r.Issues {
		fmt.Println(formatIssueRow(iv, titleW))
	}

	if len(r.Orphans) > 0 {
		fmt.Println()
		fmt.Println(warn.Render("Unlinked open PRs (no Fixes ref, no issue in branch):"))
		for _, pr := range r.Orphans {
			fmt.Printf("  #%d  %s  (%s)\n", pr.Number, truncate(pr.Title, 60), pr.BranchName)
		}
	}
}

func formatIssueRow(iv msview.IssueView, titleW int) string {
	state := "open"
	stateStyle := info
	if iv.State == "closed" {
		state = "done"
		stateStyle = ok
	}
	return fmt.Sprintf("%-6s %-6s %-*s  %-24s  %s",
		fmt.Sprintf("#%d", iv.Number),
		stateStyle.Render(state),
		titleW, truncate(iv.Title, titleW),
		highlightLabels(iv.Labels),
		formatLinkedPRs(iv.PRs),
	)
}

func highlightLabels(labels []string) string {
	if len(labels) == 0 {
		return dim.Render("—")
	}
	present := map[string]bool{}
	for _, l := range labels {
		present[strings.ToLower(l)] = true
	}
	out := []string{}
	for _, want := range trackedLabels {
		if !present[want] {
			continue
		}
		style := info
		switch want {
		case "blocked", "bug":
			style = danger
		case "needs-review":
			style = warn
		case "agent-ready", "autopilot", "in-progress":
			style = ok
		case "no-agent":
			style = dim
		}
		out = append(out, style.Render(want))
	}
	if len(out) == 0 {
		return dim.Render(strings.Join(labels, ","))
	}
	return strings.Join(out, " ")
}

func formatLinkedPRs(prs []msview.PRLink) string {
	if len(prs) == 0 {
		return dim.Render("—")
	}
	parts := []string{}
	for _, p := range prs {
		var state string
		switch p.State {
		case msview.PRMerged:
			state = ok.Render("merged")
		case msview.PRClosed:
			state = dim.Render("closed")
		case msview.PRDraft:
			state = warn.Render("draft")
		default:
			state = info.Render("open")
		}
		suffix := ""
		if p.Link == msview.LinkMismatch {
			suffix = danger.Render(" ⚠mismatch")
		}
		parts = append(parts, fmt.Sprintf("#%d %s%s", p.Number, state, suffix))
	}
	return strings.Join(parts, "  ")
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}
