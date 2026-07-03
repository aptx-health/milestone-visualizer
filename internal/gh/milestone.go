package gh

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v72/github"
)

// Item is an issue or PR under a milestone with cross-linkage data.
type Item struct {
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	State      string   `json:"state"` // open, closed
	IsPR       bool     `json:"is_pr"`
	Merged     bool     `json:"merged"`
	Draft      bool     `json:"draft"`
	Labels     []string `json:"labels,omitempty"`
	BranchName string   `json:"branch_name,omitempty"` // PRs only
	FixesRefs  []int    `json:"fixes_refs,omitempty"`  // issue numbers referenced by "Fixes #N" / "Closes #N" in PR body
	Assignees  []string `json:"assignees,omitempty"`
	URL        string   `json:"url"`
}

// FetchMeta carries metadata from the API responses used to build a fetch.
type FetchMeta struct {
	RateLimitRemaining int `json:"rate_limit_remaining"`
}

var fixesRE = regexp.MustCompile(`(?i)\b(?:fix(?:es|ed)?|close[sd]?|resolve[sd]?)\s+#(\d+)`)
var branchIssueRE = regexp.MustCompile(`(?i)(?:^|/)(?:agent[/-])?issue[/-]?(\d+)`)

// FindMilestone resolves a milestone title (or numeric string) to its number.
func FindMilestone(ctx context.Context, c *github.Client, owner, repo, titleOrNum string) (int, string, error) {
	if n, err := strconv.Atoi(titleOrNum); err == nil {
		ms, _, err := c.Issues.GetMilestone(ctx, owner, repo, n)
		if err != nil {
			return 0, "", err
		}
		return ms.GetNumber(), ms.GetTitle(), nil
	}
	opt := &github.MilestoneListOptions{State: "all", ListOptions: github.ListOptions{PerPage: 100}}
	for {
		list, resp, err := c.Issues.ListMilestones(ctx, owner, repo, opt)
		if err != nil {
			return 0, "", err
		}
		for _, m := range list {
			if strings.EqualFold(m.GetTitle(), titleOrNum) {
				return m.GetNumber(), m.GetTitle(), nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return 0, "", fmt.Errorf("milestone %q not found", titleOrNum)
}

// FetchMilestone returns all issues attached to a milestone plus any PRs
// that link to those issues via Fixes/Closes refs OR via `agent/issue-N`
// branch names. PRs generally don't inherit the milestone from their linked
// issue, so we scan the repo's recent PRs and cross-reference.
func FetchMilestone(ctx context.Context, c *github.Client, owner, repo string, msNum int) ([]Item, error) {
	items, _, err := FetchMilestoneWithMeta(ctx, c, owner, repo, msNum)
	return items, err
}

// FetchMilestoneWithMeta is FetchMilestone plus API metadata useful for
// freshness and cost-aware consumers.
func FetchMilestoneWithMeta(ctx context.Context, c *github.Client, owner, repo string, msNum int) ([]Item, FetchMeta, error) {
	issueSet := map[int]bool{}
	items := []Item{}
	meta := FetchMeta{RateLimitRemaining: -1}

	iopt := &github.IssueListByRepoOptions{
		Milestone:   strconv.Itoa(msNum),
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		issues, resp, err := c.Issues.ListByRepo(ctx, owner, repo, iopt)
		if err != nil {
			return nil, meta, fmt.Errorf("list issues: %w", err)
		}
		meta.RateLimitRemaining = resp.Rate.Remaining
		for _, is := range issues {
			if is.IsPullRequest() {
				continue
			}
			issueSet[is.GetNumber()] = true
			items = append(items, Item{
				Number:    is.GetNumber(),
				Title:     is.GetTitle(),
				State:     is.GetState(),
				Labels:    labelNames(is.Labels),
				Assignees: assigneeLogins(is.Assignees),
				URL:       is.GetHTMLURL(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		iopt.ListOptions.Page = resp.NextPage
	}

	// Scan repo PRs and keep any that reference a milestone issue.
	popt := &github.PullRequestListOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	seenPRs := 0
	const maxPRScan = 400
	for seenPRs < maxPRScan {
		prs, resp, err := c.PullRequests.List(ctx, owner, repo, popt)
		if err != nil {
			return nil, meta, fmt.Errorf("list prs: %w", err)
		}
		meta.RateLimitRemaining = resp.Rate.Remaining
		for _, pr := range prs {
			seenPRs++
			branch := pr.GetHead().GetRef()
			body := pr.GetBody()
			fixes := extractFixes(body)
			branchN := BranchIssueNumber(branch)

			relevant := false
			if branchN != 0 && issueSet[branchN] {
				relevant = true
			}
			for _, n := range fixes {
				if issueSet[n] {
					relevant = true
					break
				}
			}
			if !relevant {
				continue
			}

			items = append(items, Item{
				Number:     pr.GetNumber(),
				Title:      pr.GetTitle(),
				State:      pr.GetState(),
				IsPR:       true,
				Merged:     pr.GetMerged() || pr.GetMergedAt().Time.Year() > 1,
				Draft:      pr.GetDraft(),
				Labels:     labelNames(pr.Labels),
				BranchName: branch,
				FixesRefs:  fixes,
				Assignees:  assigneeLogins(pr.Assignees),
				URL:        pr.GetHTMLURL(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		popt.ListOptions.Page = resp.NextPage
	}
	return items, meta, nil
}

// BranchIssueNumber extracts the issue number encoded in a PR branch name
// (e.g. "agent/issue-874" → 874). Returns 0 if none.
func BranchIssueNumber(branch string) int {
	m := branchIssueRE.FindStringSubmatch(branch)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func extractFixes(body string) []int {
	matches := fixesRE.FindAllStringSubmatch(body, -1)
	out := make([]int, 0, len(matches))
	seen := map[int]bool{}
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func labelNames(labels []*github.Label) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.GetName())
	}
	return out
}

func assigneeLogins(users []*github.User) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.GetLogin())
	}
	return out
}
