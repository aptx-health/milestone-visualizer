package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aptx-health/ms-visualizer/internal/gh"
	"github.com/aptx-health/ms-visualizer/internal/msview"
)

const (
	defaultLockPoll = 100 * time.Millisecond
	lockTimeout     = 30 * time.Second
)

// Snapshot is the persisted, derived artifact every read-only command renders
// from. It is safe to delete; the next uncached load recreates it from GitHub.
type Snapshot struct {
	FetchedAt          time.Time       `json:"fetched_at"`
	Owner              string          `json:"owner"`
	Repo               string          `json:"repo"`
	Milestone          string          `json:"milestone"`
	MilestoneNumber    int             `json:"milestone_number"`
	GraphSource        string          `json:"graph_source,omitempty"`
	RateLimitRemaining int             `json:"rate_limit_remaining"`
	Items              []gh.Item       `json:"items"`
	Reports            ComputedReports `json:"reports"`
}

type ComputedReports struct {
	Status  msview.StatusReport `json:"status"`
	Graph   msview.GraphReport  `json:"graph"`
	Ready   []msview.ReadyIssue `json:"ready"`
	Orphans []msview.PRLink     `json:"orphans"`
	Doctor  msview.DoctorReport `json:"doctor"`
}

type LoadOptions struct {
	TTL     time.Duration
	Refresh bool
	Cached  bool
}

type FetchFunc func(context.Context) (Snapshot, error)

func DefaultTTL() time.Duration {
	return 90 * time.Second
}

func Path(owner, repo, milestone string) (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, slug(owner)+"-"+slug(repo), slug(milestone)+".json"), nil
}

func Load(ctx context.Context, path string, opts LoadOptions, fetch FetchFunc) (Snapshot, error) {
	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL()
	}
	if opts.Cached && opts.Refresh {
		return Snapshot{}, fmt.Errorf("--cached and --refresh cannot be used together")
	}
	if opts.Cached {
		s, err := read(path)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read cached snapshot: %w", err)
		}
		return s, nil
	}
	if !opts.Refresh {
		if s, err := read(path); err == nil && time.Since(s.FetchedAt) < opts.TTL {
			return s, nil
		}
	}

	lockCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()
	unlock, err := lock(lockCtx, path+".lock")
	if err != nil {
		return Snapshot{}, fmt.Errorf("acquire snapshot lock: %w", err)
	}
	defer unlock()

	if !opts.Refresh {
		if s, err := read(path); err == nil && time.Since(s.FetchedAt) < opts.TTL {
			return s, nil
		}
	}
	s, err := fetch(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if s.FetchedAt.IsZero() {
		s.FetchedAt = time.Now().UTC()
	}
	if err := write(path, s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func Save(path string, s Snapshot) error {
	return write(path, s)
}

func read(path string) (Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, err
	}
	if s.FetchedAt.IsZero() {
		return Snapshot{}, fmt.Errorf("snapshot missing fetched_at")
	}
	return s, nil
}

func write(path string, s Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func lock(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if isLockStale(path) {
			_ = os.Remove(path)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(defaultLockPoll):
		}
	}
}

// isLockStale reports whether the process that wrote path is no longer running.
func isLockStale(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	return proc.Signal(syscall.Signal(0)) != nil
}

func stateDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdg != "" {
		return filepath.Join(xdg, "msv"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "msv"), nil
}

func slug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
