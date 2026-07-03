package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/msview"
)

// Exit codes are stable and documented so CI/agents can branch on them.
const (
	ExitOK               = 0
	ExitRuntimeError     = 1
	ExitLintFindings     = 2 // any findings at all when using default fail-on
	ExitMismatchDetected = 3 // reserved: doctor detected an error-severity mismatch
)

func newDoctorCmd() *cobra.Command {
	var milestone, file string
	var asJSON bool
	var failOn []string
	cmd := &cobra.Command{
		Use:   "doctor [<owner>/<repo>]",
		Short: "Lint the milestone (mismatch, orphans, cycles, coverage) — exit non-zero on findings",
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
			report := snap.Reports.Doctor

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				renderDoctorText(report)
			}

			// Merge config-level fail_on with any --fail-on flags.
			effective := failOn
			if len(effective) == 0 {
				effective = r.Config.FailOn
			}
			if shouldFail(report, effective) {
				os.Exit(ExitLintFindings)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&milestone, "milestone", "m", "", "milestone")
	cmd.Flags().StringVarP(&file, "file", "f", "", "graph markdown file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().StringSliceVar(&failOn, "fail-on", nil,
		"severities/rules to exit non-zero on (any|error|warn|info|<rule-name>)")
	return cmd
}

func renderDoctorText(r msview.DoctorReport) {
	fmt.Println(hdr.Render(fmt.Sprintf("Doctor: %s (%s/%s)", r.Milestone, r.Owner, r.Repo)))
	fmt.Println(dim.Render(fmt.Sprintf("errors: %d · warnings: %d · info: %d",
		r.Counts.Error, r.Counts.Warn, r.Counts.Info)))
	if len(r.Findings) == 0 {
		fmt.Println(ok.Render("✓ clean"))
		return
	}
	fmt.Println()
	for _, f := range r.Findings {
		style := info
		switch f.Severity {
		case "error":
			style = danger
		case "warn":
			style = warn
		}
		fmt.Printf("  %s  %s\n    %s\n",
			style.Render(f.Severity),
			dim.Render(f.Rule),
			f.Message)
	}
}

// shouldFail returns true when any finding matches the effective fail-on list.
// Empty list defaults to "error".
func shouldFail(r msview.DoctorReport, failOn []string) bool {
	if len(failOn) == 0 {
		return r.Counts.Error > 0
	}
	want := map[string]bool{}
	for _, s := range failOn {
		want[s] = true
	}
	if want["any"] && len(r.Findings) > 0 {
		return true
	}
	for _, f := range r.Findings {
		if want[f.Severity] || want[f.Rule] {
			return true
		}
	}
	return false
}
