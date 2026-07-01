package cli

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "msv",
		Short: "Milestone visualizer — track issues, PRs, and dependency graphs across a GitHub milestone",
	}
	root.PersistentFlags().String("config", "", "path to .msv.yaml (overrides search)")
	root.PersistentFlags().Bool("quiet", false, "suppress non-essential text (JSON/exit code only)")
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.AddCommand(newStatusCmd())
	root.AddCommand(newGraphCmd())
	root.AddCommand(newReadyCmd())
	root.AddCommand(newBlockedCmd())
	root.AddCommand(newOrphansCmd())
	root.AddCommand(newDoctorCmd())
	mutate := newGraphMutateGroup()
	mutate.PersistentFlags().StringP("file", "f", "", "graph markdown file (or set graph_file in .msv.yaml)")
	root.AddCommand(mutate)
	return root
}
