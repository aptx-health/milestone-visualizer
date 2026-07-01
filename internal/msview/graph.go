package msview

import (
	"sort"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
)

// BuildGraphReport merges a parsed dependency graph with the live milestone
// state, producing a per-node overlay and per-edge blocked/ready status.
func BuildGraphReport(owner, repo, milestone string, g *graph.Graph, items []gh.Item) GraphReport {
	status := BuildStatusReport(owner, repo, milestone, items)
	viewByIssue := map[int]IssueView{}
	inMilestone := map[int]bool{}
	for _, iv := range status.Issues {
		viewByIssue[iv.Number] = iv
		inMilestone[iv.Number] = true
	}

	layers := g.TopoLayers()
	layerOf := map[int]int{}
	for i, layer := range layers {
		for _, n := range layer {
			layerOf[n] = i
		}
	}

	// nodes
	nodes := make([]GraphNodeView, 0, len(g.Nodes))
	doneCache := map[int]bool{}
	for n, node := range g.Nodes {
		iv, has := viewByIssue[n]
		var chosen *PRLink
		if len(iv.PRs) > 0 {
			// Prefer highest-ranked linked PR.
			best := 0
			for i, p := range iv.PRs {
				if prRank(p) > prRank(iv.PRs[best]) {
					best = i
				}
				_ = i
			}
			pr := iv.PRs[best]
			chosen = &pr
		}
		done := false
		if has {
			done = iv.State == "closed" || (chosen != nil && chosen.State == PRMerged)
		}
		doneCache[n] = done
		nodes = append(nodes, GraphNodeView{
			Number:      n,
			Label:       node.Label,
			Layer:       layerOf[n],
			InGraph:     true,
			InMilestone: inMilestone[n],
			IssueClosed: iv.State == "closed",
			Labels:      iv.Labels,
			PR:          chosen,
			Done:        done,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Layer != nodes[j].Layer {
			return nodes[i].Layer < nodes[j].Layer
		}
		return nodes[i].Number < nodes[j].Number
	})

	// edges
	edges := make([]GraphEdgeView, 0, len(g.Edges))
	for _, e := range g.Edges {
		state := "blocked"
		if doneCache[e.From] {
			state = "ready"
			if doneCache[e.To] {
				state = "done"
			}
		}
		edges = append(edges, GraphEdgeView{From: e.From, To: e.To, State: state})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return GraphReport{
		Owner:     owner,
		Repo:      repo,
		Milestone: milestone,
		Layers:    layers,
		Nodes:     nodes,
		Edges:     edges,
	}
}

func prRank(p PRLink) int {
	switch p.State {
	case PRMerged:
		return 3
	case PROpen:
		return 2
	case PRDraft:
		return 1
	default:
		return 0
	}
}
