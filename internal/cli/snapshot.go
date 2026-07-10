package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/graph"
	"github.com/aptx-health/ms-visualizer/internal/msview"
	"github.com/aptx-health/ms-visualizer/internal/snapshot"
)

func snapshotOptions(cmd *cobra.Command) (snapshot.LoadOptions, error) {
	refresh, _ := cmd.Root().PersistentFlags().GetBool("refresh")
	cached, _ := cmd.Root().PersistentFlags().GetBool("cached")
	ttl, _ := cmd.Root().PersistentFlags().GetDuration("snapshot-ttl")
	if ttl <= 0 {
		return snapshot.LoadOptions{}, fmt.Errorf("--snapshot-ttl must be positive")
	}
	return snapshot.LoadOptions{TTL: ttl, Refresh: refresh, Cached: cached}, nil
}

func loadSnapshot(ctx context.Context, cmd *cobra.Command, r Resolved) (snapshot.Snapshot, error) {
	opts, err := snapshotOptions(cmd)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	owner, repo, err := gh.ParseOwnerRepo(r.OwnerRepo)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	path, err := snapshot.Path(owner, repo, r.Milestone)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	snap, err := snapshot.Load(ctx, path, opts, func(ctx context.Context, previous snapshot.Snapshot) (snapshot.Snapshot, error) {
		return fetchSnapshot(ctx, owner, repo, r, previous)
	})
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	snap, metadataChanged := hydrateSnapshotReports(snap)
	snap, changed, err := ensureGraphSource(snap, r)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	if metadataChanged || changed {
		if err := snapshot.Save(path, snap); err != nil {
			return snapshot.Snapshot{}, err
		}
	}
	return snap, nil
}

func hydrateSnapshotReports(s snapshot.Snapshot) (snapshot.Snapshot, bool) {
	rateLimit := rateLimitFromSnapshot(s)
	if rateLimit.Remaining == 0 && s.RateLimitRemaining != 0 {
		rateLimit.Remaining = s.RateLimitRemaining
	}
	if s.Reports.Status.RateLimit == rateLimit {
		return s, false
	}
	s.Reports.Status.RateLimit = rateLimit
	s.Reports.Doctor = msview.Doctor(s.Reports.Status, graphFromReport(s.Reports.Graph))
	s.Reports.Doctor.FetchedAt = s.FetchedAt
	return s, true
}

func fetchSnapshot(ctx context.Context, owner, repo string, r Resolved, previous snapshot.Snapshot) (snapshot.Snapshot, error) {
	client, cache, err := gh.NewClientWithETags(ctx, previous.Metadata.ETags)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	msNum, msTitle := previous.MilestoneNumber, previous.Milestone
	if msNum == 0 {
		var err error
		msNum, msTitle, err = gh.FindMilestone(ctx, client, owner, repo, r.Milestone)
		if err != nil {
			return snapshot.Snapshot{}, err
		}
	}
	items, meta, err := gh.FetchMilestoneWithMetaFrom(ctx, client, owner, repo, msNum, previous.Items)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	if remaining, reset, limit, used := cache.RateLimit(); remaining >= 0 {
		meta.RateLimitRemaining = remaining
		meta.RateLimitReset = reset
		meta.RateLimitLimit = limit
		meta.RateLimitUsed = used
	}
	meta.ETags = cache.ETags()

	fetchedAt := time.Now().UTC()
	status := msview.BuildStatusReport(owner, repo, msTitle, items)
	status.FetchedAt = fetchedAt
	status.RateLimit = msview.RateLimit{
		Remaining: meta.RateLimitRemaining,
		Reset:     meta.RateLimitReset,
		Limit:     meta.RateLimitLimit,
		Used:      meta.RateLimitUsed,
	}
	g, err := graphFromResolved(r)
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	graphReport := msview.BuildGraphReport(owner, repo, msTitle, g, items)
	graphReport.FetchedAt = fetchedAt
	doctor := msview.Doctor(status, g)
	doctor.FetchedAt = fetchedAt
	ready := msview.FindReady(status, g, nil, nil)
	orphans := msview.FindOrphans(status)
	graphSource, err := graphSource(r)
	if err != nil {
		return snapshot.Snapshot{}, err
	}

	return snapshot.Snapshot{
		FetchedAt:          fetchedAt,
		Owner:              owner,
		Repo:               repo,
		Milestone:          msTitle,
		MilestoneNumber:    msNum,
		GraphSource:        graphSource,
		RateLimitRemaining: meta.RateLimitRemaining,
		Metadata: snapshot.Metadata{
			ETags: meta.ETags,
			RateLimit: snapshot.RateLimitMeta{
				Remaining: meta.RateLimitRemaining,
				Reset:     meta.RateLimitReset,
				Limit:     meta.RateLimitLimit,
				Used:      meta.RateLimitUsed,
			},
		},
		Items: items,
		Reports: snapshot.ComputedReports{
			Status:  status,
			Graph:   graphReport,
			Ready:   ready,
			Orphans: orphans,
			Doctor:  doctor,
		},
	}, nil
}

func ensureGraphSource(s snapshot.Snapshot, r Resolved) (snapshot.Snapshot, bool, error) {
	source, err := graphSource(r)
	if err != nil {
		return s, false, err
	}
	if source == "" || source == s.GraphSource {
		return s, false, nil
	}
	g, err := graphFromResolved(r)
	if err != nil {
		return s, false, err
	}
	status := msview.BuildStatusReport(s.Owner, s.Repo, s.Milestone, s.Items)
	status.FetchedAt = s.FetchedAt
	status.RateLimit = rateLimitFromSnapshot(s)
	graphReport := msview.BuildGraphReport(s.Owner, s.Repo, s.Milestone, g, s.Items)
	graphReport.FetchedAt = s.FetchedAt
	doctor := msview.Doctor(status, g)
	doctor.FetchedAt = s.FetchedAt
	s.GraphSource = source
	s.Reports = snapshot.ComputedReports{
		Status:  status,
		Graph:   graphReport,
		Ready:   msview.FindReady(status, g, nil, nil),
		Orphans: msview.FindOrphans(status),
		Doctor:  doctor,
	}
	return s, true, nil
}

func rateLimitFromSnapshot(s snapshot.Snapshot) msview.RateLimit {
	return msview.RateLimit{
		Remaining: s.Metadata.RateLimit.Remaining,
		Reset:     s.Metadata.RateLimit.Reset,
		Limit:     s.Metadata.RateLimit.Limit,
		Used:      s.Metadata.RateLimit.Used,
	}
}

func graphFromResolved(r Resolved) (*graph.Graph, error) {
	if r.GraphFile == "" {
		return &graph.Graph{Nodes: map[int]graph.Node{}}, nil
	}
	doc, err := os.ReadFile(r.GraphFile)
	if err != nil {
		return nil, fmt.Errorf("read graph file: %w", err)
	}
	block, err := graph.ExtractBlock(string(doc))
	if err != nil {
		return nil, fmt.Errorf("parse graph: %w", err)
	}
	return graph.Parse(block)
}

func graphSource(r Resolved) (string, error) {
	if r.GraphFile == "" {
		return "", nil
	}
	return filepath.Abs(r.GraphFile)
}

func graphFromReport(r msview.GraphReport) *graph.Graph {
	g := &graph.Graph{Nodes: map[int]graph.Node{}}
	for _, n := range r.Nodes {
		g.Nodes[n.Number] = graph.Node{Number: n.Number, Label: n.Label}
	}
	for _, e := range r.Edges {
		g.Edges = append(g.Edges, graph.Edge{From: e.From, To: e.To})
	}
	return g
}
