package msview

import (
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
)

func TestDoctor_MismatchAndOrphan(t *testing.T) {
	items := []gh.Item{
		mkIssue(869, "s", "open"),
		mkIssue(871, "b", "open"),
		mkPR(888, "cross-wired", "closed", false, false, "agent/issue-869", 871),
		mkPR(999, "orphan", "open", false, false, "feature/foo"),
	}
	status := BuildStatusReport("o", "r", "M", items)
	rep := Doctor(status, &graph.Graph{Nodes: map[int]graph.Node{}})

	rules := map[string]int{}
	for _, f := range rep.Findings {
		rules[f.Rule]++
	}
	if rules[RuleMismatch] < 1 {
		t.Errorf("expected mismatch finding, got %+v", rep.Findings)
	}
	if rules[RuleOrphan] < 1 {
		t.Errorf("expected orphan finding, got %+v", rep.Findings)
	}
	if rep.Counts.Error < 1 {
		t.Errorf("expected error count > 0, got %+v", rep.Counts)
	}
}

func TestDoctor_GraphCoverage(t *testing.T) {
	items := []gh.Item{
		mkIssue(1, "in-graph", "open"),
		mkIssue(2, "not-in-graph", "open"),
	}
	g := &graph.Graph{Nodes: map[int]graph.Node{1: {Number: 1}, 99: {Number: 99}}}
	status := BuildStatusReport("o", "r", "M", items)
	rep := Doctor(status, g)

	rules := map[string]int{}
	for _, f := range rep.Findings {
		rules[f.Rule]++
	}
	if rules[RuleGraphMissing] < 1 {
		t.Errorf("expected missing-from-graph finding")
	}
	if rules[RuleGraphExtra] < 1 {
		t.Errorf("expected graph-node-not-in-milestone finding")
	}
}

func TestDoctor_Cycle(t *testing.T) {
	g := &graph.Graph{Nodes: map[int]graph.Node{1: {}, 2: {}, 3: {}}}
	g.AddEdge(1, 2)
	g.AddEdge(2, 3)
	g.AddEdge(3, 1)
	status := BuildStatusReport("o", "r", "M", nil)
	rep := Doctor(status, g)

	found := false
	for _, f := range rep.Findings {
		if f.Rule == RuleCycle {
			found = true
			if len(f.Refs) < 3 {
				t.Errorf("cycle refs should list nodes, got %v", f.Refs)
			}
		}
	}
	if !found {
		t.Errorf("expected cycle finding, got %+v", rep.Findings)
	}
}

func TestDoctor_BlockedLabelWithoutEdge(t *testing.T) {
	items := []gh.Item{
		mkIssue(5, "blocked with no edge", "open", "blocked"),
	}
	status := BuildStatusReport("o", "r", "M", items)
	rep := Doctor(status, &graph.Graph{Nodes: map[int]graph.Node{5: {Number: 5}}})

	found := false
	for _, f := range rep.Findings {
		if f.Rule == RuleBlockedLabel {
			found = true
		}
	}
	if !found {
		t.Errorf("expected blocked-label finding, got %+v", rep.Findings)
	}
}

func TestDoctor_DuplicatePRs(t *testing.T) {
	items := []gh.Item{
		mkIssue(10, "target", "open"),
		mkPR(100, "a", "open", false, false, "agent/issue-10"),
		mkPR(101, "b", "open", false, true, "agent/issue-10"),
	}
	status := BuildStatusReport("o", "r", "M", items)
	rep := Doctor(status, &graph.Graph{Nodes: map[int]graph.Node{}})
	found := false
	for _, f := range rep.Findings {
		if f.Rule == RuleDuplicatePRs {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate-prs finding, got %+v", rep.Findings)
	}
}

func TestDoctor_CleanReport(t *testing.T) {
	items := []gh.Item{
		mkIssue(1, "a", "closed"),
		mkPR(100, "pr", "closed", true, false, "agent/issue-1", 1),
	}
	g := &graph.Graph{Nodes: map[int]graph.Node{1: {Number: 1}}}
	status := BuildStatusReport("o", "r", "M", items)
	rep := Doctor(status, g)
	if len(rep.Findings) != 0 {
		t.Errorf("expected clean report, got %+v", rep.Findings)
	}
}
