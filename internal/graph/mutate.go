package graph

import (
	"fmt"
	"sort"
	"strings"
)

// Render produces a canonical Mermaid flowchart body from a Graph.
//
// Canonical form (so agent edits produce clean diffs):
//   - fixed "flowchart LR" header
//   - node declarations first, sorted by number, one per line
//   - edges next, sorted by (from, to), one per line
//   - two-space indent, single "-->" arrow style, LF line endings
func Render(g *Graph) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	nums := make([]int, 0, len(g.Nodes))
	for n := range g.Nodes {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		node := g.Nodes[n]
		label := strings.TrimSpace(node.Label)
		if label == "" {
			label = fmt.Sprintf("#%d", n)
		}
		fmt.Fprintf(&b, "  %d[%s]\n", n, label)
	}

	edges := make([]Edge, len(g.Edges))
	copy(edges, g.Edges)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	for _, e := range edges {
		fmt.Fprintf(&b, "  %d --> %d\n", e.From, e.To)
	}
	return b.String()
}

// AddNode inserts or updates a node's label.
func (g *Graph) AddNode(n int, label string) {
	if g.Nodes == nil {
		g.Nodes = map[int]Node{}
	}
	g.Nodes[n] = Node{Number: n, Label: strings.TrimSpace(label)}
}

// RemoveNode deletes a node and all edges touching it.
// Returns true if the node existed.
func (g *Graph) RemoveNode(n int) bool {
	if _, ok := g.Nodes[n]; !ok {
		return false
	}
	delete(g.Nodes, n)
	kept := g.Edges[:0]
	for _, e := range g.Edges {
		if e.From == n || e.To == n {
			continue
		}
		kept = append(kept, e)
	}
	g.Edges = kept
	return true
}

// AddEdge inserts a from→to edge (idempotent). Nodes are auto-created
// with a placeholder label if they don't exist.
func (g *Graph) AddEdge(from, to int) {
	if from == to {
		return
	}
	if g.Nodes == nil {
		g.Nodes = map[int]Node{}
	}
	if _, ok := g.Nodes[from]; !ok {
		g.Nodes[from] = Node{Number: from, Label: fmt.Sprintf("#%d", from)}
	}
	if _, ok := g.Nodes[to]; !ok {
		g.Nodes[to] = Node{Number: to, Label: fmt.Sprintf("#%d", to)}
	}
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return
		}
	}
	g.Edges = append(g.Edges, Edge{From: from, To: to})
}

// RemoveEdge deletes a from→to edge. Returns true if the edge existed.
func (g *Graph) RemoveEdge(from, to int) bool {
	kept := g.Edges[:0]
	removed := false
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			removed = true
			continue
		}
		kept = append(kept, e)
	}
	g.Edges = kept
	return removed
}

// ReplaceInDoc rewrites the deps block inside a full markdown document,
// replacing whatever was between <!-- deps:start --> and <!-- deps:end -->
// (or creating those sentinels at the end of the doc if absent) with the
// given mermaid body wrapped in ```mermaid fences.
func ReplaceInDoc(doc, mermaidBody string) (string, error) {
	block := "<!-- deps:start -->\n```mermaid\n" + strings.TrimRight(mermaidBody, "\n") + "\n```\n<!-- deps:end -->"

	if startMark.MatchString(doc) {
		startLoc := startMark.FindStringIndex(doc)
		rest := doc[startLoc[1]:]
		endLoc := endMark.FindStringIndex(rest)
		if endLoc == nil {
			return "", fmt.Errorf("found deps:start but no deps:end in document")
		}
		return doc[:startLoc[0]] + block + rest[endLoc[1]:], nil
	}
	// Append at end.
	sep := "\n\n"
	if strings.HasSuffix(doc, "\n") {
		sep = "\n"
	}
	return doc + sep + block + "\n", nil
}
