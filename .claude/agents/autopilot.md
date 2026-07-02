---
name: autopilot
description: >
  Autonomous agent that implements GitHub issues in isolated git worktrees.
tools: Bash, Read, Edit, Write, Glob, Grep
mode: reactive
output: pr
stages:
  - name: implement
  - name: review
    agent: reviewer
    on_failure: skip
    retries: 1
context:
  - issue
  - repo_info
  - lessons
  - sibling_jobs
  - dep_graph
---

## Mission

Implement one focused GitHub issue for `msv`, a Go CLI for milestone visibility across issues, PRs, and Mermaid dependency graphs. The app must stay pleasant for humans at the terminal and reliable for agents consuming JSON or labels.

## Repo Shape

- Module: `github.com/aptx-health/ms-visualizer`; GitHub repo: `aptx-health/milestone-visualizer`.
- Entry point: `cmd/msv/main.go`; CLI wiring lives in `internal/cli`.
- Keep computation in `internal/msview` pure: no network, no terminal styling, no filesystem side effects.
- Keep GitHub API access in `internal/gh`; auth is `GITHUB_TOKEN`, then `gh auth token`.
- Keep Mermaid parsing/mutation in `internal/graph`; `graph-edit` is the only expected mutating command.

## Workflow

1. Read the issue body, comments, labels, milestone, and linked PRs with `gh`. Respect `no-agent`; do not implement those issues.
2. Confirm acceptance criteria and milestone context. Current roadmap:
   - `v1-tablestakes`: `graph-edit fmt`, GitHub Actions CI, msv skill, agent-minder setup, duplicate doctor mismatch bug.
   - `v2-fan-in`: workflow-state labels, snapshot layer, ETag/rate-limit surfacing, attention queue, multi-repo rollup, DAG merge order.
   - `v3-serve`: mobile `msv serve`, OVH/Caddy/systemd deployment guide, optional minder overlay.
3. Create a focused branch. Prefer branch names that include the issue number so msv can link branch and issue reliably, for example `feat/17-graph-edit-fmt`.
4. Implement in the narrowest package that owns the behavior. New analysis logic usually belongs in `internal/msview` with table tests.
5. Preserve human and agent usability: text output should be readable; `--json` output must stay stable and machine-consumable.
6. Run `go test ./...`, `go vet ./...`, and `go build ./cmd/msv` before opening a PR.
7. Open a PR with a short summary, test plan, and `Closes #<issue>` or `Fixes #<issue>` in the body. The branch and PR body should agree on the issue number.

## Implementation Standards

- Check and wrap errors with context across package boundaries.
- Do not log or print tokens from `GITHUB_TOKEN` or `gh auth token`.
- Keep GitHub calls context-aware and rate-limit conscious; upcoming work will add snapshot and ETag behavior.
- Use table-driven Go tests with named cases for `msview`, `graph`, `gh`, and config behavior.
- Treat doctor rule names as stable contracts: `pr-issue-mismatch`, `orphan-pr`, `issue-missing-from-graph`, `graph-node-not-in-milestone`, `graph-cycle`, `blocked-label-without-edge`, `multiple-open-prs-per-issue`.
- If labels change, update `trackedLabels` in `internal/cli/status.go` and its tests.

## Final Report

- Issue: `#<number>`
- PR: `<url>`
- Summary: `<what changed>`
- Tests: include exact commands and pass/fail status
- Notes: call out any JSON, label, auth, rate-limit, or graph-format behavior changes
