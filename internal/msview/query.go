package msview

import (
	"sort"
	"time"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
)

// ReadyIssue describes an issue whose graph dependencies are all done
// (or which has no graph deps) and which isn't already closed / worked on.
type ReadyIssue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	URL    string   `json:"url"`
	// Reason is the primary trigger — either "no-deps" or "unblocked".
	Reason string `json:"reason"`
}

// BlockedInfo lists the still-open predecessors of a given issue.
type BlockedInfo struct {
	Number    int          `json:"number"`
	Title     string       `json:"title"`
	FetchedAt time.Time    `json:"fetched_at"`
	Blocked   bool         `json:"blocked"`
	By        []BlockedDep `json:"by"`
}

type BlockedDep struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"` // open | closed
	Done   bool   `json:"done"`
}

// FindReady returns issues that:
//   - are open
//   - have no open predecessors in the graph
//   - are not already claimed (no linked open/draft PR)
//
// If restrictToLabels is non-empty, only issues carrying at least one of
// those labels are returned (case-insensitive). If excludeLabels is non-empty,
// issues carrying any of those labels are omitted.
func FindReady(status StatusReport, g *graph.Graph, restrictToLabels, excludeLabels []string) []ReadyIssue {
	byIssue := indexIssues(status)
	graphNodes := map[int]bool{}
	for n := range g.Nodes {
		graphNodes[n] = true
	}
	predecessors := map[int][]int{}
	for _, e := range g.Edges {
		predecessors[e.To] = append(predecessors[e.To], e.From)
	}

	wantLabel := lowerSet(restrictToLabels)
	denyLabel := lowerSet(excludeLabels)

	out := []ReadyIssue{}
	for _, iv := range status.Issues {
		if iv.State == "closed" {
			continue
		}
		if hasClaimedPR(iv) {
			continue
		}
		if len(wantLabel) > 0 && !hasAnyLabel(iv.Labels, wantLabel) {
			continue
		}
		if len(denyLabel) > 0 && hasAnyLabel(iv.Labels, denyLabel) {
			continue
		}
		preds := predecessors[iv.Number]
		reason := "no-deps"
		if len(preds) > 0 {
			blocked := false
			for _, p := range preds {
				if !isDoneByNumber(byIssue, p) {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			reason = "unblocked"
		} else if !graphNodes[iv.Number] {
			// If the graph exists at all and this issue isn't in it,
			// still report as no-deps but flag it.
			reason = "no-deps"
		}
		out = append(out, ReadyIssue{
			Number: iv.Number, Title: iv.Title, Labels: iv.Labels, URL: iv.URL, Reason: reason,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// BlockedBy returns the predecessor-status breakdown for a single issue.
func BlockedBy(status StatusReport, g *graph.Graph, issueNumber int) BlockedInfo {
	byIssue := indexIssues(status)
	iv, ok := byIssue[issueNumber]
	info := BlockedInfo{Number: issueNumber}
	if ok {
		info.Title = iv.Title
	}
	preds := []int{}
	for _, e := range g.Edges {
		if e.To == issueNumber {
			preds = append(preds, e.From)
		}
	}
	sort.Ints(preds)
	for _, p := range preds {
		pv, has := byIssue[p]
		dep := BlockedDep{Number: p}
		if has {
			dep.Title = pv.Title
			dep.State = pv.State
			dep.Done = isDone(pv)
		} else {
			dep.State = "unknown"
		}
		if !dep.Done {
			info.Blocked = true
		}
		info.By = append(info.By, dep)
	}
	return info
}

// FindOrphans returns PRs that couldn't be linked to any milestone issue
// through branch name or Fixes ref. (Just a re-shape of StatusReport.Orphans.)
func FindOrphans(status StatusReport) []PRLink {
	out := make([]PRLink, len(status.Orphans))
	copy(out, status.Orphans)
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// Fetch helpers below — these are the tiny shims the CLI can call so it
// doesn't have to know about gh.Item.

func BuildStatusFromItems(owner, repo, milestone string, items []gh.Item) StatusReport {
	return BuildStatusReport(owner, repo, milestone, items)
}

// --- internal helpers ---

func indexIssues(s StatusReport) map[int]IssueView {
	m := map[int]IssueView{}
	for _, iv := range s.Issues {
		m[iv.Number] = iv
	}
	return m
}

func isDone(iv IssueView) bool {
	if iv.State == "closed" {
		return true
	}
	for _, p := range iv.PRs {
		if p.State == PRMerged {
			return true
		}
	}
	return false
}

func isDoneByNumber(m map[int]IssueView, n int) bool {
	iv, ok := m[n]
	if !ok {
		// dep not in milestone — treat as not-done so we don't
		// silently unblock things pointing at missing deps.
		return false
	}
	return isDone(iv)
}

func hasClaimedPR(iv IssueView) bool {
	for _, p := range iv.PRs {
		if p.State == PROpen || p.State == PRDraft {
			return true
		}
	}
	return false
}

func hasAnyLabel(labels []string, want map[string]bool) bool {
	for _, l := range labels {
		if want[stringsLower(l)] {
			return true
		}
	}
	return false
}

func lowerSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		if x == "" {
			continue
		}
		m[stringsLower(x)] = true
	}
	return m
}

// stringsLower avoids importing strings in this file's header list.
func stringsLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
