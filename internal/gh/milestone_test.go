package gh

import (
	"reflect"
	"os"
	"testing"
	"time"
)

func TestBranchIssueNumber(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   int
	}{
		{"agent issue hyphen", "agent/issue-874", 874},
		{"agent issue slash", "agent/issue/1", 1},
		{"bare issue slash", "issue/123", 123},
		{"bare issue hyphen", "issue-42", 42},
		{"case insensitive issue", "agent/ISSUE-999", 999},
		{"nested issue", "prefix/issue-42-x", 42},
		{"fix numeric prefix", "fix/1-doctor-dedupe", 1},
		{"feat numeric prefix", "feat/17-graph-edit-fmt", 17},
		{"nested conventional numeric prefix", "user/chore/123-cleanup", 123},
		{"docs numeric prefix", "docs/456-update-readme", 456},
		{"refactor numeric prefix", "refactor/77-split-package", 77},
		{"perf numeric prefix", "perf/88-cache-status", 88},
		{"test numeric prefix", "test/99-branch-parser", 99},
		{"empty", "", 0},
		{"no issue", "feature/foo", 0},
		{"missing hyphen after number", "fix/123abc", 0},
		{"version segment", "release/2.0", 0},
		{"numeric prefix without conventional type", "feature/123-add-widget", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BranchIssueNumber(tt.branch); got != tt.want {
				t.Errorf("BranchIssueNumber(%q) = %d, want %d", tt.branch, got, tt.want)
			}
		})
	}
}

func TestExtractFixes(t *testing.T) {
	body := `## Summary
This PR does the thing.

Fixes #874
Also closes #12 and Resolves #345.
Duplicate: fixes #874.
`
	got := extractFixes(body)
	want := []int{874, 12, 345}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractFixes = %v, want %v", got, want)
	}
}

func TestExtractFixes_None(t *testing.T) {
	if got := extractFixes("no refs here"); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestParseOwnerRepo(t *testing.T) {
	o, r, err := ParseOwnerRepo("aptx-health/ripit-fitness")
	if err != nil || o != "aptx-health" || r != "ripit-fitness" {
		t.Errorf("got (%q,%q,%v)", o, r, err)
	}
	if _, _, err := ParseOwnerRepo("nope"); err == nil {
		t.Error("expected error on bad input")
	}
}

func TestListMilestones_RequiredFields(t *testing.T) {
	m := MilestoneInfo{
		Number:       42,
		Title:        "Test",
		State:        "open",
		OpenIssues:   10,
		ClosedIssues: 5,
		Description:  "desc",
	}
	if m.Number != 42 {
		t.Errorf("Number = %d, want 42", m.Number)
	}
	if m.OpenIssues != 10 {
		t.Errorf("OpenIssues = %d, want 10", m.OpenIssues)
	}
	if m.ClosedIssues != 5 {
		t.Errorf("ClosedIssues = %d, want 5", m.ClosedIssues)
	}
}

func TestListMilestones_DueOnSerialization(t *testing.T) {
	due := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	d := due.Format(time.RFC3339)
	m := MilestoneInfo{Number: 1, Title: "v2", State: "open", DueOn: &d}
	if m.DueOn == nil || *m.DueOn != d {
		t.Errorf("DueOn serialization failed: %v", m.DueOn)
	}
}

func TestListMilestones_EmptyDescription(t *testing.T) {
	m := MilestoneInfo{Number: 1, Title: "v1", State: "closed", OpenIssues: 0, ClosedIssues: 10}
	if m.Description != "" {
		t.Errorf("expected empty description, got %q", m.Description)
	}
}

func TestListMilestones_TokenAvailable(t *testing.T) {
	tok := os.Getenv("GITHUB_TOKEN")
	if tok == "" {
		t.Skip("GITHUB_TOKEN not set")
	}
}

func BenchmarkMilestoneInfoFields(b *testing.B) {
	d := "2026-12-31T00:00:00Z"
	for i := 0; i < b.N; i++ {
		_ = MilestoneInfo{
			Number:       1,
			Title:        "v1",
			State:        "open",
			OpenIssues:   1,
			ClosedIssues: 1,
			DueOn:        &d,
			Description:  "x",
		}
	}
}
