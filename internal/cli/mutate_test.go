package cli

import (
	"strings"
	"testing"
)

func TestCanonicalizeDoc(t *testing.T) {
	// A non-canonical hand-written block: edges declared inline with node
	// labels, unsorted, using the deps sentinels.
	noncanonical := "# Milestone\n\n" +
		"<!-- deps:start -->\n" +
		"```mermaid\n" +
		"flowchart LR\n" +
		"  2[#2 b] --> 3[#3 c]\n" +
		"  1[#1 a] --> 2\n" +
		"```\n" +
		"<!-- deps:end -->\n"

	cases := []struct {
		name        string
		doc         string
		wantChanged bool
		wantErr     bool
		wantSubstr  []string
		wantAbsent  []string
	}{
		{
			name:        "noncanonical is rewritten",
			doc:         noncanonical,
			wantChanged: true,
			wantSubstr: []string{
				"  1[#1 a]\n",
				"  2[#2 b]\n",
				"  3[#3 c]\n",
				"  1 --> 2\n",
				"  2 --> 3\n",
				"# Milestone",
			},
			// inline edge+label form should be gone
			wantAbsent: []string{"[#2 b] -->", "[#1 a] -->"},
		},
		{
			name:        "no deps block is an error",
			doc:         "# just prose, no graph\n",
			wantErr:     true,
			wantChanged: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, err := canonicalizeDoc(tc.doc)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
			for _, s := range tc.wantSubstr {
				if !strings.Contains(out, s) {
					t.Errorf("output missing %q:\n%s", s, out)
				}
			}
			for _, s := range tc.wantAbsent {
				if strings.Contains(out, s) {
					t.Errorf("output should not contain %q:\n%s", s, out)
				}
			}
		})
	}
}

func TestCanonicalizeDoc_Idempotent(t *testing.T) {
	doc := "<!-- deps:start -->\n```mermaid\nflowchart LR\n  1[#1 a] --> 2[#2 b]\n```\n<!-- deps:end -->\n"

	first, changed, err := canonicalizeDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("expected first pass to canonicalize the block")
	}
	// Running fmt on already-canonical output must report no change.
	second, changed2, err := canonicalizeDoc(first)
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Errorf("second pass should be a no-op, but reported changed:\n%s", second)
	}
	if second != first {
		t.Errorf("canonical form not stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
