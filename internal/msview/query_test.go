package msview

import (
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
)

func newGraph(edges ...[2]int) *graph.Graph {
	g := &graph.Graph{Nodes: map[int]graph.Node{}}
	for _, e := range edges {
		g.Nodes[e[0]] = graph.Node{Number: e[0], Label: ""}
		g.Nodes[e[1]] = graph.Node{Number: e[1], Label: ""}
		g.Edges = append(g.Edges, graph.Edge{From: e[0], To: e[1]})
	}
	return g
}

func TestFindReady_NoDepsAndUnblocked(t *testing.T) {
	items := []gh.Item{
		mkIssue(1, "root a", "closed", "agent-ready"),
		mkIssue(2, "root b", "open", "agent-ready"),      // no deps → ready
		mkIssue(3, "child of 1", "open", "agent-ready"),  // 1 is closed → unblocked
		mkIssue(4, "child of 2", "open", "agent-ready"),  // 2 open → blocked
		mkIssue(5, "already-worked", "open", "agent-ready"),
		mkPR(50, "pr for 5", "open", false, false, "agent/issue-5"),
	}
	g := newGraph([2]int{1, 3}, [2]int{2, 4})
	status := BuildStatusReport("o", "r", "M", items)
	ready := FindReady(status, g, nil)

	got := map[int]string{}
	for _, r := range ready {
		got[r.Number] = r.Reason
	}
	if got[2] != "no-deps" {
		t.Errorf("issue 2 should be no-deps ready, got %+v", ready)
	}
	if got[3] != "unblocked" {
		t.Errorf("issue 3 should be unblocked (dep 1 closed), got %+v", ready)
	}
	if _, in := got[4]; in {
		t.Errorf("issue 4 should be blocked (dep 2 open), got %+v", ready)
	}
	if _, in := got[5]; in {
		t.Errorf("issue 5 has an open PR, should not be ready")
	}
}

func TestFindReady_LabelFilter(t *testing.T) {
	items := []gh.Item{
		mkIssue(1, "a", "open", "bug"),
		mkIssue(2, "b", "open", "agent-ready"),
	}
	status := BuildStatusReport("o", "r", "M", items)
	ready := FindReady(status, newGraph(), []string{"agent-ready"})
	if len(ready) != 1 || ready[0].Number != 2 {
		t.Errorf("label filter failed: %+v", ready)
	}
}

func TestBlockedBy(t *testing.T) {
	items := []gh.Item{
		mkIssue(1, "one", "closed"),
		mkIssue(2, "two", "open"),
		mkIssue(3, "three", "open"),
	}
	g := newGraph([2]int{1, 3}, [2]int{2, 3})
	status := BuildStatusReport("o", "r", "M", items)
	info := BlockedBy(status, g, 3)
	if !info.Blocked {
		t.Errorf("issue 3 should be blocked")
	}
	if len(info.By) != 2 {
		t.Fatalf("expected 2 deps, got %+v", info.By)
	}
	done := map[int]bool{}
	for _, d := range info.By {
		done[d.Number] = d.Done
	}
	if !done[1] || done[2] {
		t.Errorf("dep-done wrong: %+v", info.By)
	}
}

func TestBlockedBy_UnknownDepStaysBlocking(t *testing.T) {
	items := []gh.Item{mkIssue(3, "three", "open")}
	g := newGraph([2]int{99, 3})
	status := BuildStatusReport("o", "r", "M", items)
	info := BlockedBy(status, g, 3)
	if !info.Blocked {
		t.Errorf("unknown dep should still block")
	}
	if info.By[0].State != "unknown" {
		t.Errorf("expected unknown state, got %q", info.By[0].State)
	}
}
