package msview

import (
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
)

func TestBuildGraphReport_EdgeStates(t *testing.T) {
	g := &graph.Graph{
		Nodes: map[int]graph.Node{
			1: {Number: 1, Label: "root"},
			2: {Number: 2, Label: "mid"},
			3: {Number: 3, Label: "leaf"},
		},
		Edges: []graph.Edge{
			{From: 1, To: 2},
			{From: 2, To: 3},
		},
	}
	items := []gh.Item{
		mkIssue(1, "root", "closed"),
		mkIssue(2, "mid", "open"),
		mkIssue(3, "leaf", "open"),
	}
	r := BuildGraphReport("o", "r", "M", g, items)
	edgeState := map[[2]int]string{}
	for _, e := range r.Edges {
		edgeState[[2]int{e.From, e.To}] = e.State
	}
	if edgeState[[2]int{1, 2}] != "ready" {
		t.Errorf("1→2 should be ready, got %s", edgeState[[2]int{1, 2}])
	}
	if edgeState[[2]int{2, 3}] != "blocked" {
		t.Errorf("2→3 should be blocked, got %s", edgeState[[2]int{2, 3}])
	}
}

func TestBuildGraphReport_MergedPRMarksDone(t *testing.T) {
	g := &graph.Graph{
		Nodes: map[int]graph.Node{
			1: {Number: 1, Label: "a"},
			2: {Number: 2, Label: "b"},
		},
		Edges: []graph.Edge{{From: 1, To: 2}},
	}
	items := []gh.Item{
		mkIssue(1, "a", "open"),
		mkIssue(2, "b", "open"),
		mkPR(50, "pr", "closed", true, false, "agent/issue-1", 1),
	}
	r := BuildGraphReport("o", "r", "M", g, items)
	var node1 GraphNodeView
	for _, n := range r.Nodes {
		if n.Number == 1 {
			node1 = n
		}
	}
	if !node1.Done {
		t.Errorf("node 1 should be Done via merged PR, got %+v", node1)
	}
	if node1.PR == nil || node1.PR.State != PRMerged {
		t.Errorf("node 1 should carry the merged PR link, got %+v", node1.PR)
	}
	if r.Edges[0].State != "ready" {
		t.Errorf("edge 1→2 should be ready when 1 is done, got %s", r.Edges[0].State)
	}
}
