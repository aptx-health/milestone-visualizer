package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
	"github.com/aptx-health/ms-visualizer/internal/msview"
)

func newReadyCmd() *cobra.Command {
	var milestone, file string
	var asJSON bool
	var labels []string
	cmd := &cobra.Command{
		Use:   "ready [<owner>/<repo>]",
		Short: "List issues that are unblocked and ready to pick up",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := resolve(cmd, args, milestone, file)
			if err != nil {
				return err
			}
			status, g, err := loadStatusAndGraph(ctx, r)
			if err != nil {
				return err
			}
			ready := msview.FindReady(status, g, labels)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"ready": ready, "count": len(ready)})
			}
			fmt.Println(hdr.Render(fmt.Sprintf("Ready: %d", len(ready))))
			for _, i := range ready {
				fmt.Printf("  %s  %s  %s\n",
					ok.Render(fmt.Sprintf("#%d", i.Number)),
					truncate(i.Title, 60),
					dim.Render(i.Reason))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone")
	cmd.Flags().StringVarP(&file, "file", "f", "", "graph markdown file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "restrict to issues carrying at least one of these labels")
	return cmd
}

func newBlockedCmd() *cobra.Command {
	var milestone, file string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "blocked <issue-number> [<owner>/<repo>]",
		Short: "Show which predecessors are keeping an issue blocked",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueN, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("issue number: %w", err)
			}
			positional := []string{}
			if len(args) > 1 {
				positional = append(positional, args[1])
			}
			ctx := context.Background()
			r, err := resolve(cmd, positional, milestone, file)
			if err != nil {
				return err
			}
			status, g, err := loadStatusAndGraph(ctx, r)
			if err != nil {
				return err
			}
			info := msview.BlockedBy(status, g, issueN)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			verb := "READY"
			style := ok
			if info.Blocked {
				verb = "BLOCKED"
				style = warn
			}
			fmt.Println(style.Render(fmt.Sprintf("#%d %s  %s", info.Number, verb, info.Title)))
			for _, d := range info.By {
				marker := ok.Render("✓")
				if !d.Done {
					marker = warn.Render("✗")
				}
				fmt.Printf("  %s #%d  %s  (%s)\n", marker, d.Number, truncate(d.Title, 50), d.State)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone")
	cmd.Flags().StringVarP(&file, "file", "f", "", "graph markdown file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

func newOrphansCmd() *cobra.Command {
	var milestone string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "orphans [<owner>/<repo>]",
		Short: "List PRs relevant to the milestone that don't link to any milestone issue",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			r, err := resolve(cmd, args, milestone, "")
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
			msNum, msTitle, err := gh.FindMilestone(ctx, client, owner, repo, r.Milestone)
			if err != nil {
				return err
			}
			items, err := gh.FetchMilestone(ctx, client, owner, repo, msNum)
			if err != nil {
				return err
			}
			status := msview.BuildStatusReport(owner, repo, msTitle, items)
			orphans := msview.FindOrphans(status)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"orphans": orphans, "count": len(orphans)})
			}
			fmt.Println(hdr.Render(fmt.Sprintf("Orphans: %d", len(orphans))))
			for _, o := range orphans {
				fmt.Printf("  #%d  %s  (%s)\n", o.Number, truncate(o.Title, 60), o.BranchName)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// loadStatusAndGraph fetches the milestone and parses the graph file.
// Shared by ready/blocked/graph. Requires a graph file to be resolvable.
func loadStatusAndGraph(ctx context.Context, r Resolved) (msview.StatusReport, *graph.Graph, error) {
	owner, repo, err := gh.ParseOwnerRepo(r.OwnerRepo)
	if err != nil {
		return msview.StatusReport{}, nil, err
	}
	client, err := gh.NewClient(ctx)
	if err != nil {
		return msview.StatusReport{}, nil, err
	}
	msNum, msTitle, err := gh.FindMilestone(ctx, client, owner, repo, r.Milestone)
	if err != nil {
		return msview.StatusReport{}, nil, err
	}
	items, err := gh.FetchMilestone(ctx, client, owner, repo, msNum)
	if err != nil {
		return msview.StatusReport{}, nil, err
	}
	status := msview.BuildStatusReport(owner, repo, msTitle, items)

	if r.GraphFile == "" {
		return status, &graph.Graph{Nodes: map[int]graph.Node{}}, nil
	}
	doc, err := os.ReadFile(r.GraphFile)
	if err != nil {
		return status, nil, fmt.Errorf("read graph file: %w", err)
	}
	block, err := graph.ExtractBlock(string(doc))
	if err != nil {
		return status, nil, fmt.Errorf("parse graph: %w", err)
	}
	g, err := graph.Parse(block)
	if err != nil {
		return status, nil, err
	}
	return status, g, nil
}
