package msview

import (
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/gh"
)

func mkIssue(n int, title, state string, labels ...string) gh.Item {
	return gh.Item{Number: n, Title: title, State: state, Labels: labels}
}

func mkPR(n int, title, state string, merged, draft bool, branch string, fixes ...int) gh.Item {
	return gh.Item{
		Number: n, Title: title, State: state, IsPR: true,
		Merged: merged, Draft: draft, BranchName: branch, FixesRefs: fixes,
	}
}

func TestBuildStatusReport_Counts(t *testing.T) {
	items := []gh.Item{
		mkIssue(100, "A", "open"),
		mkIssue(101, "B", "closed"),
		mkIssue(102, "C", "open"),
		mkPR(200, "pr-a", "closed", true, false, "agent/issue-100", 100),
		mkPR(201, "pr-c", "open", false, true, "agent/issue-102", 102),
	}
	r := BuildStatusReport("o", "r", "M", items)

	if r.Summary.IssuesOpen != 2 || r.Summary.IssuesClosed != 1 {
		t.Errorf("issue counts wrong: %+v", r.Summary)
	}
	if r.Summary.PRsMerged != 1 || r.Summary.PRsDraft != 1 {
		t.Errorf("pr counts wrong: %+v", r.Summary)
	}
	if len(r.Issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(r.Issues))
	}
	if r.Issues[0].Number != 100 {
		t.Errorf("issues not sorted ascending: %+v", r.Issues)
	}
}

func TestBuildStatusReport_LinkClassification(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		fixes  []int
		issue  int
		want   LinkStatus
	}{
		{"branch+fixes", "agent/issue-100", []int{100}, 100, LinkBranchAndFixes},
		{"branch-only", "agent/issue-100", nil, 100, LinkBranchOnly},
		{"fixes-only", "feature/foo", []int{100}, 100, LinkFixesOnly},
		{"mismatch-fixes-side", "agent/issue-101", []int{100}, 100, LinkMismatch},
		{"mismatch-branch-side", "agent/issue-100", []int{101}, 100, LinkMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := []gh.Item{
				mkIssue(tt.issue, "issue", "open"),
				mkPR(500, "pr", "open", false, false, tt.branch, tt.fixes...),
			}
			r := BuildStatusReport("o", "r", "M", items)
			if len(r.Issues) != 1 || len(r.Issues[0].PRs) < 1 {
				t.Fatalf("expected linked PR, got %+v", r.Issues)
			}
			// Find the link corresponding to this issue.
			found := false
			for _, l := range r.Issues[0].PRs {
				if l.Number == 500 {
					if l.Link != tt.want {
						t.Errorf("link classification: got %v want %v", l.Link, tt.want)
					}
					found = true
				}
			}
			if !found {
				t.Errorf("no PR link found on issue")
			}
		})
	}
}

func TestBuildStatusReport_MismatchAlsoAttachesToBranchIssue(t *testing.T) {
	// Simulates the real Ripit bug: PR body says Fixes #871 but branch is
	// agent/issue-869. Both should surface with mismatch classification.
	items := []gh.Item{
		mkIssue(869, "Suggestion table", "open"),
		mkIssue(871, "Beta storage", "open"),
		mkPR(888, "pr", "closed", false, false, "agent/issue-869", 871),
	}
	r := BuildStatusReport("o", "r", "M", items)
	byIssue := map[int]IssueView{}
	for _, iv := range r.Issues {
		byIssue[iv.Number] = iv
	}
	if len(byIssue[869].PRs) != 1 || byIssue[869].PRs[0].Link != LinkMismatch {
		t.Errorf("issue 869 should have mismatch link, got %+v", byIssue[869].PRs)
	}
	if len(byIssue[871].PRs) != 1 || byIssue[871].PRs[0].Link != LinkMismatch {
		t.Errorf("issue 871 should have mismatch link, got %+v", byIssue[871].PRs)
	}
}

func TestBuildStatusReport_Orphans(t *testing.T) {
	// PR references only an out-of-milestone issue → orphan.
	items := []gh.Item{
		mkIssue(100, "A", "open"),
		mkPR(600, "pr", "open", false, false, "feature/foo"),
	}
	r := BuildStatusReport("o", "r", "M", items)
	if len(r.Orphans) != 1 || r.Orphans[0].Number != 600 {
		t.Errorf("expected 1 orphan #600, got %+v", r.Orphans)
	}
	if len(r.Issues[0].PRs) != 0 {
		t.Errorf("orphan should not attach to issue: %+v", r.Issues[0].PRs)
	}
}
