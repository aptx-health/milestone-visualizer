package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/graph"
	"github.com/aptx-health/ms-visualizer/internal/msview"
)

func newReadyCmd() *cobra.Command {
	var milestone, file string
	var asJSON bool
	var labels []string
	var excludeLabels []string
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
			status, g, fetchedAt, err := loadStatusAndGraph(ctx, cmd, r)
			if err != nil {
				return err
			}
			ready := msview.FindReady(status, g, labels, excludeLabels)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"fetched_at": fetchedAt, "ready": ready, "count": len(ready)})
			}
			fmt.Println(hdr.Render(fmt.Sprintf("Ready: %d", len(ready))))
			for _, i := range ready {
				fmt.Println(formatReadyRow(i))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone")
	cmd.Flags().StringVarP(&file, "file", "f", "", "graph markdown file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "restrict to issues carrying at least one of these labels")
	cmd.Flags().StringArrayVar(&excludeLabels, "exclude-label", nil, "exclude issues carrying this label (repeatable)")
	return cmd
}

func formatReadyRow(i msview.ReadyIssue) string {
	return fmt.Sprintf("  %s  %s  %s  %s",
		ok.Render(fmt.Sprintf("#%d", i.Number)),
		truncate(i.Title, 60),
		highlightLabels(i.Labels),
		dim.Render(i.Reason))
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
			status, g, fetchedAt, err := loadStatusAndGraph(ctx, cmd, r)
			if err != nil {
				return err
			}
			info := msview.BlockedBy(status, g, issueN)
			info.FetchedAt = fetchedAt
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
			snap, err := loadSnapshot(ctx, cmd, r)
			if err != nil {
				return err
			}
			status := snap.Reports.Status
			orphans := msview.FindOrphans(status)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"fetched_at": snap.FetchedAt, "orphans": orphans, "count": len(orphans)})
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

// loadStatusAndGraph reads the current snapshot and reconstructs the graph
// shape from the persisted graph report.
func loadStatusAndGraph(ctx context.Context, cmd *cobra.Command, r Resolved) (msview.StatusReport, *graph.Graph, time.Time, error) {
	snap, err := loadSnapshot(ctx, cmd, r)
	if err != nil {
		return msview.StatusReport{}, nil, time.Time{}, err
	}
	return snap.Reports.Status, graphFromReport(snap.Reports.Graph), snap.FetchedAt, nil
}
