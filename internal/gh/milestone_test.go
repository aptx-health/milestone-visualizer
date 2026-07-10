package gh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-github/v72/github"
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

func TestFetchMilestoneWithMetaFromReusesPreviousSegmentsOnNotModified(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	client := github.NewClient(server.Client())
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	client.BaseURL = baseURL

	previous := []Item{
		{Number: 1, Title: "issue", State: "open"},
		{Number: 10, Title: "pr", State: "open", IsPR: true, BranchName: "agent/issue-1"},
	}
	items, meta, err := FetchMilestoneWithMetaFrom(context.Background(), client, "o", "r", 1, previous)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, previous) {
		t.Fatalf("items = %+v, want previous %+v", items, previous)
	}
	if !meta.IssuesNotModified || !meta.PRsNotModified {
		t.Fatalf("not-modified flags = issues %v prs %v", meta.IssuesNotModified, meta.PRsNotModified)
	}
	if len(requests) != 2 || requests[0] != "/repos/o/r/issues" || requests[1] != "/repos/o/r/pulls" {
		t.Fatalf("requests = %v, want issues then pulls", requests)
	}
}
