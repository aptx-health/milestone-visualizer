// Package msview holds the pure data model and computations shared by
// every command. Nothing in here touches the network or the terminal —
// it consumes gh.Item slices and produces typed results the CLI can
// render as text or JSON.
package msview

// LinkStatus classifies why an issue and a PR ended up linked.
type LinkStatus string

const (
	LinkBranchAndFixes LinkStatus = "branch+fixes"
	LinkBranchOnly     LinkStatus = "branch-only"
	LinkFixesOnly      LinkStatus = "fixes-only"
	LinkMismatch       LinkStatus = "mismatch" // Fixes says A, branch says B (both non-zero, different)
)

// PRState is a compact classification we render.
type PRState string

const (
	PRMerged PRState = "merged"
	PROpen   PRState = "open"
	PRDraft  PRState = "draft"
	PRClosed PRState = "closed"
)

// IssueView is the milestone-level view of an issue plus its linked PRs.
type IssueView struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"` // open / closed
	Labels    []string  `json:"labels"`
	Assignees []string  `json:"assignees,omitempty"`
	URL       string    `json:"url"`
	PRs       []PRLink  `json:"prs,omitempty"`
}

// PRLink is a PR referenced by an issue via branch name or Fixes ref.
type PRLink struct {
	Number     int        `json:"number"`
	Title      string     `json:"title"`
	State      PRState    `json:"state"`
	BranchName string     `json:"branch"`
	FixesRefs  []int      `json:"fixes,omitempty"`
	URL        string     `json:"url"`
	Link       LinkStatus `json:"link"`
	// BranchIssue is the issue number encoded in the branch, if any.
	BranchIssue int `json:"branch_issue,omitempty"`
}

// StatusReport is the top-level `msv status` payload.
type StatusReport struct {
	Owner     string      `json:"owner"`
	Repo      string      `json:"repo"`
	Milestone string      `json:"milestone"`
	Summary   Summary     `json:"summary"`
	Issues    []IssueView `json:"issues"`
	Orphans   []PRLink    `json:"orphans"`
}

// Summary is the top-line count block.
type Summary struct {
	IssuesOpen   int `json:"issues_open"`
	IssuesClosed int `json:"issues_closed"`
	PRsMerged    int `json:"prs_merged"`
	PRsOpen      int `json:"prs_open"`
	PRsDraft     int `json:"prs_draft"`
	PRsClosed    int `json:"prs_closed"`
}

// GraphNodeView augments a graph node with live issue status.
type GraphNodeView struct {
	Number   int       `json:"number"`
	Label    string    `json:"label"`
	Layer    int       `json:"layer"`
	InGraph  bool      `json:"in_graph"`
	InMilestone bool   `json:"in_milestone"`
	IssueClosed bool   `json:"issue_closed"`
	Labels   []string  `json:"labels,omitempty"`
	PR       *PRLink   `json:"pr,omitempty"`
	Done     bool      `json:"done"`
}

// GraphEdgeView is an edge with live blocked/ready state.
type GraphEdgeView struct {
	From    int    `json:"from"`
	To      int    `json:"to"`
	State   string `json:"state"` // done | ready | blocked
}

// GraphReport is `msv graph --json`.
type GraphReport struct {
	Owner     string          `json:"owner"`
	Repo      string          `json:"repo"`
	Milestone string          `json:"milestone"`
	Layers    [][]int         `json:"layers"`
	Nodes     []GraphNodeView `json:"nodes"`
	Edges     []GraphEdgeView `json:"edges"`
}
