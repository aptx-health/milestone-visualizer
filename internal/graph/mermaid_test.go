package graph

import (
	"strings"
	"testing"
)

func TestExtractBlock_Sentinels(t *testing.T) {
	doc := "prose\n<!-- deps:start -->\n```mermaid\nflowchart\n```\n<!-- deps:end -->\ntail"
	got, err := ExtractBlock(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "flowchart") {
		t.Errorf("expected block content, got %q", got)
	}
}

func TestExtractBlock_MermaidFenceFallback(t *testing.T) {
	doc := "```mermaid\nflowchart LR\n1 --> 2\n```"
	got, err := ExtractBlock(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "1 --> 2") {
		t.Errorf("expected fence content, got %q", got)
	}
}

func TestExtractBlock_MissingEndSentinel(t *testing.T) {
	doc := "<!-- deps:start -->\n1 --> 2"
	if _, err := ExtractBlock(doc); err == nil {
		t.Error("expected error on missing end sentinel")
	}
}

func TestParse_EdgesAndNodes(t *testing.T) {
	src := `flowchart LR
  1[#1 a] --> 2[#2 b]
  2 --> 3[#3 c]
  3 --> 4
`
	g, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(g.Edges))
	}
	if g.Nodes[1].Label != "#1 a" || g.Nodes[3].Label != "#3 c" {
		t.Errorf("node labels wrong: %+v", g.Nodes)
	}
	// Node 4 was only referenced, no bracket label — auto-labeled.
	if g.Nodes[4].Label != "#4" {
		t.Errorf("auto-labeled node wrong: %q", g.Nodes[4].Label)
	}
}

func TestParse_MultipleArrowStyles(t *testing.T) {
	src := "1 --> 2\n2 -.-> 3\n3 ==> 4\n"
	g, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 3 {
		t.Errorf("expected 3 edges across arrow styles, got %d", len(g.Edges))
	}
}

func TestTopoLayers_LongestPath(t *testing.T) {
	// 1 → 2 → 4
	// 1 → 3 → 4
	// 4 → 5
	g := &Graph{
		Nodes: map[int]Node{1: {}, 2: {}, 3: {}, 4: {}, 5: {}},
		Edges: []Edge{
			{1, 2}, {1, 3}, {2, 4}, {3, 4}, {4, 5},
		},
	}
	layers := g.TopoLayers()
	if len(layers) != 4 {
		t.Fatalf("expected 4 layers, got %d: %+v", len(layers), layers)
	}
	if !contains(layers[0], 1) || !contains(layers[2], 4) || !contains(layers[3], 5) {
		t.Errorf("layer assignments wrong: %+v", layers)
	}
}

func contains(xs []int, want int) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
