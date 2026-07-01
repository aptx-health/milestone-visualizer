package gh

import (
	"reflect"
	"testing"
)

func TestBranchIssueNumber(t *testing.T) {
	tests := map[string]int{
		"agent/issue-874":   874,
		"agent/issue-1":     1,
		"issue/123":         123,
		"issue-42":          42,
		"feature/foo":       0,
		"":                  0,
		"agent/ISSUE-999":   999,
		"prefix/issue-42-x": 42,
	}
	for in, want := range tests {
		if got := BranchIssueNumber(in); got != want {
			t.Errorf("BranchIssueNumber(%q) = %d, want %d", in, got, want)
		}
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
