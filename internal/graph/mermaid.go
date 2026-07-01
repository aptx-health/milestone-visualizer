// Package graph parses a Mermaid flowchart out of a milestone doc and
// produces a dependency DAG keyed by issue number.
package graph

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Node is a single issue node in the dependency graph.
type Node struct {
	Number int
	Label  string // e.g. "#869 Suggestion table"
}

// Edge represents "From must complete before To can start."
type Edge struct {
	From int
	To   int
}

// Graph is the parsed dependency graph.
type Graph struct {
	Nodes map[int]Node
	Edges []Edge
}

var (
	// Sentinels wrap the mermaid block we own.
	startMark = regexp.MustCompile(`<!--\s*deps:start\s*-->`)
	endMark   = regexp.MustCompile(`<!--\s*deps:end\s*-->`)

	// Match nodes like: 869[#869 Suggestion table]  or  869["something"]
	// (can appear anywhere on a line — either side of an arrow).
	nodeRE = regexp.MustCompile(`(?:^|[\s>])(\d+)\s*[\[({]([^\])}]+)[\])}]`)

	// Match edges like:  869 --> 876   or  869[#869 X] --> 876[#876 Y]
	// The left/right nodes may have optional [..], (..), or {..} label blocks
	// between the number and the arrow, and we skip labels on the target too.
	edgeRE = regexp.MustCompile(`(\d+)(?:\s*[\[({][^\])}]*[\])}])?\s*(?:--o|--x|-\.->|==>|-->)\s*(\d+)`)
)

// ExtractBlock returns the Mermaid text between the deps:start/end sentinels.
// Falls back to the first ```mermaid fenced block if sentinels are absent.
func ExtractBlock(doc string) (string, error) {
	if m := startMark.FindStringIndex(doc); m != nil {
		if e := endMark.FindStringIndex(doc[m[1]:]); e != nil {
			return doc[m[1] : m[1]+e[0]], nil
		}
		return "", fmt.Errorf("found deps:start but no deps:end")
	}
	// Fallback: first mermaid fence
	fenceRE := regexp.MustCompile("(?s)```mermaid\\s*(.+?)```")
	if m := fenceRE.FindStringSubmatch(doc); len(m) == 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("no <!-- deps:start --> block or ```mermaid fence found")
}

// Parse extracts a Graph from a Mermaid flowchart body.
func Parse(mermaid string) (*Graph, error) {
	g := &Graph{Nodes: map[int]Node{}}

	for _, m := range nodeRE.FindAllStringSubmatch(mermaid, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		g.Nodes[n] = Node{Number: n, Label: strings.TrimSpace(m[2])}
	}

	for _, m := range edgeRE.FindAllStringSubmatch(mermaid, -1) {
		from, err1 := strconv.Atoi(m[1])
		to, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			continue
		}
		g.Edges = append(g.Edges, Edge{From: from, To: to})
		if _, ok := g.Nodes[from]; !ok {
			g.Nodes[from] = Node{Number: from, Label: fmt.Sprintf("#%d", from)}
		}
		if _, ok := g.Nodes[to]; !ok {
			g.Nodes[to] = Node{Number: to, Label: fmt.Sprintf("#%d", to)}
		}
	}
	return g, nil
}

// TopoLayers groups nodes into dependency layers (longest-path from any root).
// Returns [][]int of issue numbers, ordered from roots outward.
func (g *Graph) TopoLayers() [][]int {
	depth := map[int]int{}
	inbound := map[int][]int{}
	for _, e := range g.Edges {
		inbound[e.To] = append(inbound[e.To], e.From)
	}
	var visit func(n int) int
	visit = func(n int) int {
		if d, ok := depth[n]; ok {
			return d
		}
		max := 0
		for _, p := range inbound[n] {
			if d := visit(p) + 1; d > max {
				max = d
			}
		}
		depth[n] = max
		return max
	}
	for n := range g.Nodes {
		visit(n)
	}

	maxDepth := 0
	for _, d := range depth {
		if d > maxDepth {
			maxDepth = d
		}
	}
	layers := make([][]int, maxDepth+1)
	for n, d := range depth {
		layers[d] = append(layers[d], n)
	}
	return layers
}
