package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUsesFreshSnapshotWithoutFetching(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.json")
	want := Snapshot{FetchedAt: time.Now().UTC(), Owner: "aptx", Repo: "repo", Milestone: "M"}
	if err := write(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := Load(context.Background(), path, LoadOptions{TTL: time.Minute}, func(context.Context) (Snapshot, error) {
		t.Fatal("fetch should not be called")
		return Snapshot{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Owner != want.Owner || got.Repo != want.Repo || got.Milestone != want.Milestone {
		t.Fatalf("got snapshot %+v, want %+v", got, want)
	}
}

func TestLoadRefreshesStaleSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.json")
	old := Snapshot{FetchedAt: time.Now().Add(-time.Hour).UTC(), Owner: "old"}
	if err := write(path, old); err != nil {
		t.Fatal(err)
	}

	calls := 0
	got, err := Load(context.Background(), path, LoadOptions{TTL: time.Second}, func(context.Context) (Snapshot, error) {
		calls++
		return Snapshot{FetchedAt: time.Now().UTC(), Owner: "new"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1", calls)
	}
	if got.Owner != "new" {
		t.Fatalf("owner = %q, want new", got.Owner)
	}
}

func TestLoadCachedModeNeverFetches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	_, err := Load(context.Background(), path, LoadOptions{Cached: true}, func(context.Context) (Snapshot, error) {
		t.Fatal("fetch should not be called")
		return Snapshot{}, nil
	})
	if err == nil {
		t.Fatal("expected missing cached snapshot error")
	}
}

func TestPathUsesCentralStateDir(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	got, err := Path("AptX-Health", "Milestone Visualizer", "Release 1.0")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(state, "msv", "aptx-health-milestone-visualizer", "release-1-0.json")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestLoadRejectsConflictingModes(t *testing.T) {
	_, err := Load(context.Background(), filepath.Join(t.TempDir(), "snap.json"), LoadOptions{
		Refresh: true,
		Cached:  true,
	}, func(context.Context) (Snapshot, error) {
		return Snapshot{}, nil
	})
	if err == nil {
		t.Fatal("expected conflicting mode error")
	}
}

func TestLockHonorsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "snap.lock")
	if err := os.WriteFile(lockPath, []byte("held"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := lock(ctx, lockPath)
	if err == nil {
		t.Fatal("expected lock cancellation error")
	}
}
