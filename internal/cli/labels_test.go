package cli

import "testing"

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
