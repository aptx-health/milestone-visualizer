package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTmp(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Explicit(t *testing.T) {
	p := writeTmp(t, ".msv.yaml", `
owner: aptx-health
repo: ripit-fitness
milestone: "Suggest Workout (LLM-powered)"
graph_file: docs/milestones/15.md
`)
	c, path, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if path != p {
		t.Errorf("path: got %q want %q", path, p)
	}
	if c.Owner != "aptx-health" || c.Repo != "ripit-fitness" || c.Milestone == "" || c.GraphFile == "" {
		t.Errorf("bad load: %+v", c)
	}
	if c.OwnerRepo() != "aptx-health/ripit-fitness" {
		t.Errorf("OwnerRepo() = %q", c.OwnerRepo())
	}
}

func TestLoad_Empty(t *testing.T) {
	// No env, no explicit — .msv.yaml won't be found from a tmp CWD.
	t.Setenv("MSV_CONFIG", "")
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(t.TempDir())
	c, path, err := Load("")
	if err != nil {
		t.Fatalf("expected no error when no config found, got %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
	if c.Owner != "" {
		t.Errorf("expected empty config, got %+v", c)
	}
}

func TestMerge_FlagsOverride(t *testing.T) {
	c := Config{Owner: "a", Repo: "b", Milestone: "old", GraphFile: "old.md"}
	merged := c.Merge(Overrides{OwnerRepo: "x/y", Milestone: "new"})
	if merged.Owner != "x" || merged.Repo != "y" {
		t.Errorf("owner/repo not overridden: %+v", merged)
	}
	if merged.Milestone != "new" {
		t.Errorf("milestone: got %q", merged.Milestone)
	}
	if merged.GraphFile != "old.md" {
		t.Errorf("graph_file should have been preserved: got %q", merged.GraphFile)
	}
}

func TestMerge_EmptyPreserves(t *testing.T) {
	c := Config{Owner: "a", Repo: "b", Milestone: "keep"}
	merged := c.Merge(Overrides{})
	if merged.Milestone != "keep" {
		t.Errorf("empty overrides should preserve config: %+v", merged)
	}
}

func TestFindUp_WalksToParent(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	_ = os.MkdirAll(sub, 0o755)
	target := filepath.Join(root, "a", ".msv.yaml")
	_ = os.WriteFile(target, []byte("owner: x\nrepo: y\n"), 0o644)

	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(sub)

	c, path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	// resolve symlinks (macOS /var → /private/var) before comparing
	gotAbs, _ := filepath.EvalSymlinks(path)
	wantAbs, _ := filepath.EvalSymlinks(target)
	if gotAbs != wantAbs {
		t.Errorf("expected walk-up to find %q, got %q", wantAbs, gotAbs)
	}
	if c.Owner != "x" || c.Repo != "y" {
		t.Errorf("bad parse: %+v", c)
	}
}
