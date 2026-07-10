package cli

import (
	"strings"
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/msview"
)

func TestTrackedLabels_Contains(t *testing.T) {
	must := []string{
		"agent-ready", "autopilot", "blocked", "bug",
		"design", "in-progress", "needs-review", "no-agent",
		"spike", "ux",
	}
	got := map[string]bool{}
	for _, l := range trackedLabels {
		got[l] = true
	}
	for _, want := range must {
		if !got[want] {
			t.Errorf("trackedLabels missing %q", want)
		}
	}
}

func TestFormatReadyRow_IncludesLabels(t *testing.T) {
	row := formatReadyRow(msview.ReadyIssue{
		Number: 6,
		Title:  "human only",
		Labels: []string{"agent-ready", "no-agent"},
		Reason: "no-deps",
	})
	for _, want := range []string{"#6", "human only", "agent-ready", "no-agent", "no-deps"} {
		if !strings.Contains(row, want) {
			t.Errorf("ready row missing %q: %q", want, row)
		}
	}
}
