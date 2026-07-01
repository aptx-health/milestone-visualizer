package msview

import (
	"sort"

	"github.com/aptx-health/ms-visualizer/internal/gh"
)

// BuildStatusReport is the pure data-assembly step for `msv status`.
// Given the flat set of gh.Items fetched for a milestone, it partitions
// them into issue-centric views, computes PR link classification, and
// surfaces orphan PRs (no issue reference in branch or body).
func BuildStatusReport(owner, repo, milestone string, items []gh.Item) StatusReport {
	issues := []gh.Item{}
	prs := []gh.Item{}
	for _, it := range items {
		if it.IsPR {
			prs = append(prs, it)
		} else {
			issues = append(issues, it)
		}
	}
	issueSet := map[int]bool{}
	for _, is := range issues {
		issueSet[is.Number] = true
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })

	// index PRs by every issue they claim (branch OR fixes)
	prByIssue := map[int][]PRLink{}
	prAttached := map[int]bool{}
	for _, pr := range prs {
		branchN := gh.BranchIssueNumber(pr.BranchName)
		fixesSet := map[int]bool{}
		for _, n := range pr.FixesRefs {
			fixesSet[n] = true
		}
		claimed := map[int]bool{}
		if branchN != 0 {
			claimed[branchN] = true
		}
		for n := range fixesSet {
			claimed[n] = true
		}
		for issueN := range claimed {
			link := classifyLink(issueN, branchN, fixesSet)
			prByIssue[issueN] = append(prByIssue[issueN], PRLink{
				Number:      pr.Number,
				Title:       pr.Title,
				State:       classifyPRState(pr),
				BranchName:  pr.BranchName,
				FixesRefs:   pr.FixesRefs,
				URL:         pr.URL,
				Link:        link,
				BranchIssue: branchN,
			})
			prAttached[pr.Number] = true
		}
	}

	// Build issue views
	views := make([]IssueView, 0, len(issues))
	sum := Summary{}
	for _, is := range issues {
		if is.State == "closed" {
			sum.IssuesClosed++
		} else {
			sum.IssuesOpen++
		}
		links := prByIssue[is.Number]
		sort.Slice(links, func(i, j int) bool { return links[i].Number < links[j].Number })
		views = append(views, IssueView{
			Number:    is.Number,
			Title:     is.Title,
			State:     is.State,
			Labels:    is.Labels,
			Assignees: is.Assignees,
			URL:       is.URL,
			PRs:       links,
		})
	}

	// Orphans: PRs that were fetched (relevant to the milestone) but not
	// attached to any known issue — i.e. the branch/fixes referenced only
	// out-of-milestone issues (rare) or referenced nothing at all.
	orphans := []PRLink{}
	for _, pr := range prs {
		if prAttached[pr.Number] {
			continue
		}
		branchN := gh.BranchIssueNumber(pr.BranchName)
		fixesSet := map[int]bool{}
		for _, n := range pr.FixesRefs {
			fixesSet[n] = true
		}
		orphans = append(orphans, PRLink{
			Number:      pr.Number,
			Title:       pr.Title,
			State:       classifyPRState(pr),
			BranchName:  pr.BranchName,
			FixesRefs:   pr.FixesRefs,
			URL:         pr.URL,
			Link:        classifyLink(0, branchN, fixesSet),
			BranchIssue: branchN,
		})
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Number < orphans[j].Number })

	// PR summary counts across all PRs (deduped)
	seen := map[int]bool{}
	for _, pr := range prs {
		if seen[pr.Number] {
			continue
		}
		seen[pr.Number] = true
		switch classifyPRState(pr) {
		case PRMerged:
			sum.PRsMerged++
		case PROpen:
			sum.PRsOpen++
		case PRDraft:
			sum.PRsDraft++
		case PRClosed:
			sum.PRsClosed++
		}
	}

	return StatusReport{
		Owner:     owner,
		Repo:      repo,
		Milestone: milestone,
		Summary:   sum,
		Issues:    views,
		Orphans:   orphans,
	}
}

func classifyPRState(pr gh.Item) PRState {
	switch {
	case pr.Merged:
		return PRMerged
	case pr.State == "closed":
		return PRClosed
	case pr.Draft:
		return PRDraft
	default:
		return PROpen
	}
}

// classifyLink decides how a PR is linked to a specific issue.
// issueN=0 means "classify the PR overall" (used for orphans).
func classifyLink(issueN, branchN int, fixesSet map[int]bool) LinkStatus {
	branchMatches := branchN != 0 && branchN == issueN
	fixesMatches := fixesSet[issueN]

	switch {
	case branchMatches && fixesMatches:
		return LinkBranchAndFixes
	case branchMatches && !fixesMatches && len(fixesSet) > 0:
		return LinkMismatch // branch says this issue, Fixes says a different one
	case branchMatches:
		return LinkBranchOnly
	case fixesMatches && branchN != 0 && branchN != issueN:
		return LinkMismatch // Fixes says this issue, branch says a different one
	case fixesMatches:
		return LinkFixesOnly
	}
	return LinkFixesOnly // fallback for orphan-classification callers
}
