package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/config"
)

// Resolved is the effective (owner, repo, milestone, graph file) after
// merging config + flag + positional overrides.
type Resolved struct {
	Config    config.Config
	OwnerRepo string
	Milestone string
	GraphFile string
}

// resolve loads .msv.yaml (via --config or search) and overlays flag/positional
// values. Positional argv is only used when non-empty.
func resolve(cmd *cobra.Command, argv []string, flagMilestone, flagGraphFile string) (Resolved, error) {
	cfgPath, _ := cmd.Root().PersistentFlags().GetString("config")
	c, _, err := config.Load(cfgPath)
	if err != nil {
		return Resolved{}, err
	}

	var argOwnerRepo string
	if len(argv) > 0 {
		argOwnerRepo = argv[0]
	}
	merged := c.Merge(config.Overrides{
		OwnerRepo: argOwnerRepo,
		Milestone: flagMilestone,
		GraphFile: flagGraphFile,
	})

	if merged.OwnerRepo() == "" {
		return Resolved{}, fmt.Errorf("owner/repo not provided: pass as positional arg or set owner+repo in .msv.yaml")
	}
	if merged.Milestone == "" {
		return Resolved{}, fmt.Errorf("milestone not provided: use --milestone/-m or set milestone in .msv.yaml")
	}
	return Resolved{
		Config:    merged,
		OwnerRepo: merged.OwnerRepo(),
		Milestone: merged.Milestone,
		GraphFile: merged.GraphFile,
	}, nil
}

// resolveRepo is like resolve() but without requiring a milestone.
func resolveRepo(cmd *cobra.Command, argv []string) (Resolved, error) {
	cfgPath, _ := cmd.Root().PersistentFlags().GetString("config")
	c, _, err := config.Load(cfgPath)
	if err != nil {
		return Resolved{}, err
	}

	var argOwnerRepo string
	if len(argv) > 0 {
		argOwnerRepo = argv[0]
	}
	merged := c.Merge(config.Overrides{
		OwnerRepo: argOwnerRepo,
	})

	if merged.OwnerRepo() == "" {
		return Resolved{}, fmt.Errorf("owner/repo not provided: pass as positional arg or set owner+repo in .msv.yaml")
	}
	return Resolved{
		Config:    merged,
		OwnerRepo: merged.OwnerRepo(),
		Milestone: merged.Milestone,
		GraphFile: merged.GraphFile,
	}, nil
}
