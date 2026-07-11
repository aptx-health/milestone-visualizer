package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "msv",
		Short: "Milestone visualizer — track issues, PRs, and dependency graphs across a GitHub milestone",
	}
	root.PersistentFlags().String("config", "", "path to .msv.yaml (overrides search)")
	root.PersistentFlags().Bool("quiet", false, "suppress non-essential text (JSON/exit code only)")
	root.PersistentFlags().Bool("refresh", false, "force a GitHub fetch and update the snapshot")
	root.PersistentFlags().Bool("cached", false, "render from the snapshot and never fetch")
	root.PersistentFlags().Duration("snapshot-ttl", 90*time.Second, "snapshot freshness window")
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.AddCommand(newStatusCmd())
	root.AddCommand(newGraphCmd())
	root.AddCommand(newReadyCmd())
	root.AddCommand(newBlockedCmd())
	root.AddCommand(newOrphansCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newMilestonesCmd())
	mutate := newGraphMutateGroup()
	mutate.PersistentFlags().StringP("file", "f", "", "graph markdown file (or set graph_file in .msv.yaml)")
	root.AddCommand(mutate)
	return root
}
