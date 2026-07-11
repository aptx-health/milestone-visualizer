package gh

import "testing"

func TestBranchIssueNumber(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"agent/issue-874", "agent/issue-874", 874},
		{"agent/issue-1", "agent/issue-1", 1},
		{"fix/42", "fix/42", 42},
		{"feat/123-thing", "feat/123-thing", 123},
		{"no-issue", "some-branch-name", 0},
		{"no-dash", "feature/my-feature", 0},
		{"just-digits", "874", 0},
		{"empty", "", 0},
		{"issue-prefix", "issue-42", 42},
		{"agent/issue-0", "agent/issue-0", 0},
		{"chore/99-docs", "chore/99-docs", 99},
		{"docs/88", "docs/88", 88},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BranchIssueNumber(c.in)
			if got != c.want {
				t.Errorf("BranchIssueNumber(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}
