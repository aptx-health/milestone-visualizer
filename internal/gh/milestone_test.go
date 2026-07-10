package gh

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
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

// serveConditional writes body honoring If-None-Match, returning 304 when the
// client's ETag still matches (mirroring GitHub's conditional-request behavior).
func serveConditional(w http.ResponseWriter, r *http.Request, etag, link, body string) {
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Remaining", "4999")
	if link != "" {
		w.Header().Set("Link", link)
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func titleByNumber(items []Item, number int) (string, bool) {
	for _, it := range items {
		if it.Number == number {
			return it.Title, true
		}
	}
	return "", false
}

// TestFetchMilestone_MultiPage304PicksUpLaterPageChanges guards the core
// conditional-request correctness property: a 304 on page 1 must NOT mask
// changes on page 2. ETags are per-page, so page 1 can be unchanged while a
// later page is not — the transport replays page 1 from cache and still fetches
// the changed page 2.
func TestFetchMilestone_MultiPage304PicksUpLaterPageChanges(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	// Mutable page-2 content so the second run can change it.
	page2ETag := `"issues-p2-a"`
	page2Body := `[{"number":2,"title":"two","state":"open"}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/issues" && r.URL.Query().Get("page") == "":
			link := `<http://` + r.Host + `/repos/o/r/issues?milestone=1&state=all&per_page=100&page=2>; rel="next"`
			serveConditional(w, r, `"issues-p1"`, link, `[{"number":1,"title":"one","state":"open"}]`)
		case r.URL.Path == "/repos/o/r/issues" && r.URL.Query().Get("page") == "2":
			serveConditional(w, r, page2ETag, "", page2Body)
		case r.URL.Path == "/repos/o/r/pulls":
			serveConditional(w, r, `"pulls-p1"`, "", `[]`)
		default:
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	newClient := func(cache map[string]ResponseCacheEntry) (*ConditionalCache, func() []Item) {
		client, cc, err := NewClientWithCache(context.Background(), cache)
		if err != nil {
			t.Fatal(err)
		}
		client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
		return cc, func() []Item {
			items, _, err := FetchMilestoneWithMeta(context.Background(), client, "o", "r", 1)
			if err != nil {
				t.Fatal(err)
			}
			return items
		}
	}

	// Run 1 populates the cache with both pages.
	cc1, fetch1 := newClient(nil)
	items1 := fetch1()
	if _, ok := titleByNumber(items1, 2); !ok {
		t.Fatalf("run 1 missing page-2 issue: %+v", items1)
	}

	// Run 2: page 1 is unchanged (304, replayed) but page 2's content changed.
	page2ETag = `"issues-p2-b"`
	page2Body = `[{"number":2,"title":"two-updated","state":"open"}]`

	_, fetch2 := newClient(cc1.Entries())
	items2 := fetch2()
	if _, ok := titleByNumber(items2, 1); !ok {
		t.Fatalf("run 2 dropped page-1 issue (304 replay failed): %+v", items2)
	}
	title, ok := titleByNumber(items2, 2)
	if !ok || title != "two-updated" {
		t.Fatalf("run 2 page-2 title = %q ok=%v, want the updated value", title, ok)
	}
}

// TestFetchMilestone_PRRelevanceRecomputedOn304 guards the dependent-cache
// property: PR relevance depends on the current milestone issue set. When the
// PR list is unchanged (304) but an issue is removed, the replayed PR bodies
// must be re-evaluated so a PR linked only to the removed issue drops out.
func TestFetchMilestone_PRRelevanceRecomputedOn304(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	issuesETag := `"issues-a"`
	issuesBody := `[{"number":1,"title":"one","state":"open"},{"number":2,"title":"two","state":"open"}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/issues":
			serveConditional(w, r, issuesETag, "", issuesBody)
		case "/repos/o/r/pulls":
			serveConditional(w, r, `"pulls-a"`, "",
				`[{"number":10,"title":"pr","state":"open","body":"Fixes #2","head":{"ref":"feature"}}]`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	newClient := func(cache map[string]ResponseCacheEntry) (*ConditionalCache, func() []Item) {
		client, cc, err := NewClientWithCache(context.Background(), cache)
		if err != nil {
			t.Fatal(err)
		}
		client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")
		return cc, func() []Item {
			items, _, err := FetchMilestoneWithMeta(context.Background(), client, "o", "r", 1)
			if err != nil {
				t.Fatal(err)
			}
			return items
		}
	}

	// Run 1: PR #10 (Fixes #2) is relevant because issue #2 is present.
	cc1, fetch1 := newClient(nil)
	items1 := fetch1()
	if _, ok := titleByNumber(items1, 10); !ok {
		t.Fatalf("run 1 should include PR #10: %+v", items1)
	}

	// Run 2: issue #2 is removed (issues change → 200); PR list unchanged (304).
	issuesETag = `"issues-b"`
	issuesBody = `[{"number":1,"title":"one","state":"open"}]`

	_, fetch2 := newClient(cc1.Entries())
	items2 := fetch2()
	if _, ok := titleByNumber(items2, 10); ok {
		t.Fatalf("run 2 should drop PR #10 after issue #2 removed: %+v", items2)
	}
	if !reflect.DeepEqual(itemNumbers(items2), []int{1}) {
		t.Fatalf("run 2 items = %v, want just issue #1", itemNumbers(items2))
	}
}

func itemNumbers(items []Item) []int {
	out := []int{}
	for _, it := range items {
		out = append(out, it.Number)
	}
	return out
}
