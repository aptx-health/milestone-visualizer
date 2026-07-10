package msview

import (
	"fmt"
	"sort"
	"time"

	"github.com/aptx-health/ms-visualizer/internal/graph"
)

// Finding is a single lint issue.
type Finding struct {
	Rule     string `json:"rule"`     // stable machine key
	Severity string `json:"severity"` // info | warn | error
	Message  string `json:"message"`
	// Refs the finding relates to — issue numbers, PR numbers, etc.
	Refs []int `json:"refs,omitempty"`
}

// DoctorReport is the shape of `msv doctor --json`.
type DoctorReport struct {
	Owner     string    `json:"owner"`
	Repo      string    `json:"repo"`
	Milestone string    `json:"milestone"`
	FetchedAt time.Time `json:"fetched_at"`
	Findings  []Finding `json:"findings"`
	Counts    struct {
		Error int `json:"error"`
		Warn  int `json:"warn"`
		Info  int `json:"info"`
	} `json:"counts"`
}

// Rules the doctor enforces (used for exit-code gating too).
const (
	RuleMismatch     = "pr-issue-mismatch"
	RuleOrphan       = "orphan-pr"
	RuleGraphMissing = "issue-missing-from-graph"
	RuleGraphExtra   = "graph-node-not-in-milestone"
	RuleCycle        = "graph-cycle"
	RuleBlockedLabel = "blocked-label-without-edge"
	RuleDuplicatePRs = "multiple-open-prs-per-issue"
	RuleRateLow      = "github-rate-limit-low"
	RuleRateBurnHigh = "github-rate-limit-burn-high"
)

// Doctor runs every lint against the current status + graph and returns
// a report. Callers use it to render text and to pick an exit code.
func Doctor(status StatusReport, g *graph.Graph) DoctorReport {
	report := DoctorReport{
		Owner:     status.Owner,
		Repo:      status.Repo,
		Milestone: status.Milestone,
	}

	// 1. PR/issue link mismatches. A cross-wired PR is attached to every
	// issue it claims (branch AND Fixes), so the same underlying mismatch
	// surfaces once per claimed issue. Dedupe by PR number so a single
	// mismatched PR yields a single finding.
	mismatchSeen := map[int]bool{}
	for _, iv := range status.Issues {
		for _, p := range iv.PRs {
			if p.Link == LinkMismatch {
				if mismatchSeen[p.Number] {
					continue
				}
				mismatchSeen[p.Number] = true
				report.Findings = append(report.Findings, Finding{
					Rule:     RuleMismatch,
					Severity: "error",
					Message: fmt.Sprintf(
						"PR #%d: branch says issue #%d but Fixes says %v",
						p.Number, p.BranchIssue, p.FixesRefs),
					Refs: []int{p.Number, iv.Number},
				})
			}
		}
	}

	// 2. Orphan PRs — relevant but couldn't be linked.
	for _, o := range status.Orphans {
		report.Findings = append(report.Findings, Finding{
			Rule:     RuleOrphan,
			Severity: "warn",
			Message:  fmt.Sprintf("PR #%d has no Fixes ref and no issue in branch %q", o.Number, o.BranchName),
			Refs:     []int{o.Number},
		})
	}

	// 3. Multiple open/draft PRs targeting the same issue.
	for _, iv := range status.Issues {
		open := []int{}
		for _, p := range iv.PRs {
			if p.State == PROpen || p.State == PRDraft {
				open = append(open, p.Number)
			}
		}
		if len(open) > 1 {
			sort.Ints(open)
			report.Findings = append(report.Findings, Finding{
				Rule:     RuleDuplicatePRs,
				Severity: "warn",
				Message:  fmt.Sprintf("issue #%d has %d open/draft PRs: %v", iv.Number, len(open), open),
				Refs:     append([]int{iv.Number}, open...),
			})
		}
	}

	// 4/5. Graph vs milestone coverage.
	if len(g.Nodes) > 0 {
		msIssues := map[int]bool{}
		for _, iv := range status.Issues {
			msIssues[iv.Number] = true
		}
		graphNodes := map[int]bool{}
		for n := range g.Nodes {
			graphNodes[n] = true
		}
		// Missing from graph: open issues (still actionable) not in graph.
		missing := []int{}
		for _, iv := range status.Issues {
			if iv.State == "closed" {
				continue
			}
			if !graphNodes[iv.Number] {
				missing = append(missing, iv.Number)
			}
		}
		sort.Ints(missing)
		for _, n := range missing {
			report.Findings = append(report.Findings, Finding{
				Rule:     RuleGraphMissing,
				Severity: "info",
				Message:  fmt.Sprintf("open issue #%d is not in the dependency graph", n),
				Refs:     []int{n},
			})
		}
		// Extra in graph: graph node not in milestone.
		extra := []int{}
		for n := range graphNodes {
			if !msIssues[n] {
				extra = append(extra, n)
			}
		}
		sort.Ints(extra)
		for _, n := range extra {
			report.Findings = append(report.Findings, Finding{
				Rule:     RuleGraphExtra,
				Severity: "warn",
				Message:  fmt.Sprintf("graph node #%d isn't in the milestone", n),
				Refs:     []int{n},
			})
		}

		// 6. Cycles.
		if cycle := findCycle(g); len(cycle) > 0 {
			report.Findings = append(report.Findings, Finding{
				Rule:     RuleCycle,
				Severity: "error",
				Message:  fmt.Sprintf("cycle in dependency graph: %v", cycle),
				Refs:     cycle,
			})
		}
	}

	// 7. Issues carrying `blocked` label but with no incoming edge — the
	// author probably forgot to record the dependency.
	for _, iv := range status.Issues {
		hasBlockedLabel := false
		for _, l := range iv.Labels {
			if stringsLower(l) == "blocked" {
				hasBlockedLabel = true
				break
			}
		}
		if !hasBlockedLabel {
			continue
		}
		hasInbound := false
		for _, e := range g.Edges {
			if e.To == iv.Number {
				hasInbound = true
				break
			}
		}
		if !hasInbound {
			report.Findings = append(report.Findings, Finding{
				Rule:     RuleBlockedLabel,
				Severity: "info",
				Message:  fmt.Sprintf("issue #%d has `blocked` label but no incoming graph edge — record the dependency", iv.Number),
				Refs:     []int{iv.Number},
			})
		}
	}

	addRateLimitFindings(&report, status)

	// Tally counts and stable-sort findings.
	sort.SliceStable(report.Findings, func(i, j int) bool {
		return sevRank(report.Findings[i].Severity) < sevRank(report.Findings[j].Severity)
	})
	for _, f := range report.Findings {
		switch f.Severity {
		case "error":
			report.Counts.Error++
		case "warn":
			report.Counts.Warn++
		case "info":
			report.Counts.Info++
		}
	}
	return report
}

func addRateLimitFindings(report *DoctorReport, status StatusReport) {
	budget := status.RateLimit
	if budget.Limit == 0 && budget.Reset.IsZero() && budget.Remaining == 0 {
		return
	}
	if budget.Remaining >= 0 && budget.Remaining <= 500 {
		report.Findings = append(report.Findings, Finding{
			Rule:     RuleRateLow,
			Severity: "info",
			Message:  fmt.Sprintf("GitHub API budget is low: %d requests remain before reset", budget.Remaining),
		})
	}
	if budget.Limit <= 0 || budget.Reset.IsZero() || budget.Remaining < 0 {
		return
	}
	now := status.FetchedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !budget.Reset.After(now) {
		return
	}
	used := budget.Used
	if used == 0 {
		used = budget.Limit - budget.Remaining
	}
	if used <= 0 {
		return
	}
	windowStart := budget.Reset.Add(-time.Hour)
	elapsed := now.Sub(windowStart).Minutes()
	remainingWindow := budget.Reset.Sub(now).Minutes()
	if elapsed < 5 || remainingWindow <= 0 {
		return
	}
	burnPerMinute := float64(used) / elapsed
	sustainablePerMinute := float64(budget.Remaining) / remainingWindow
	if burnPerMinute > sustainablePerMinute && budget.Remaining < budget.Limit/2 {
		report.Findings = append(report.Findings, Finding{
			Rule:     RuleRateBurnHigh,
			Severity: "info",
			Message: fmt.Sprintf(
				"GitHub API budget burn is high: %.1f requests/min used with %d remaining until reset",
				burnPerMinute, budget.Remaining),
		})
	}
}

func sevRank(s string) int {
	switch s {
	case "error":
		return 0
	case "warn":
		return 1
	default:
		return 2
	}
}

// findCycle returns a cycle (as an ordered list of nodes) if one exists.
// Standard DFS with recursion-stack tracking.
func findCycle(g *graph.Graph) []int {
	adj := map[int][]int{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	state := map[int]int{} // 0=unseen, 1=on stack, 2=done
	stack := []int{}
	var dfs func(n int) []int
	dfs = func(n int) []int {
		state[n] = 1
		stack = append(stack, n)
		for _, m := range adj[n] {
			if state[m] == 1 {
				// Found cycle: slice stack from m onward.
				for i, x := range stack {
					if x == m {
						cyc := append([]int{}, stack[i:]...)
						return append(cyc, m)
					}
				}
			}
			if state[m] == 0 {
				if c := dfs(m); len(c) > 0 {
					return c
				}
			}
		}
		state[n] = 2
		stack = stack[:len(stack)-1]
		return nil
	}
	nums := make([]int, 0, len(g.Nodes))
	for n := range g.Nodes {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		if state[n] == 0 {
			if c := dfs(n); len(c) > 0 {
				return c
			}
		}
	}
	return nil
}
