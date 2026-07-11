package cli

import (
	"os"
	"path/filepath"
	"testing"
	"github.com/spf13/cobra"
)

func newTestRootCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("config", "", "path to .msv.yaml")
	return cmd
}

func TestResolveRepo_FromArgs(t *testing.T) {
	cmd := newTestRootCmd()
	r, err := resolveRepo(cmd, []string{"owner/repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.OwnerRepo != "owner/repo" {
		t.Errorf("OwnerRepo = %q", r.OwnerRepo)
	}
}

func TestResolveRepo_FromConfig(t *testing.T) {
	dir := t.TempDir()
	content := []byte("owner: alice\nrepo: bob\n")
	cfgPath := filepath.Join(dir, ".msv.yaml")
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newTestRootCmd()
	if err := cmd.Root().PersistentFlags().Set("config", cfgPath); err != nil {
		t.Fatal(err)
	}
	r, err := resolveRepo(cmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.OwnerRepo != "alice/bob" {
		t.Errorf("owner/repo = %q", r.OwnerRepo)
	}
}

func TestResolveRepo_PrefersArgOverConfig(t *testing.T) {
	dir := t.TempDir()
	content := []byte("owner: alice\nrepo: bob\n")
	cfgPath := filepath.Join(dir, ".msv.yaml")
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newTestRootCmd()
	if err := cmd.Root().PersistentFlags().Set("config", cfgPath); err != nil {
		t.Fatal(err)
	}
	r, err := resolveRepo(cmd, []string{"charlie/delta"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.OwnerRepo != "charlie/delta" {
		t.Errorf("expected charlie/delta, got %q", r.OwnerRepo)
	}
}

func TestResolveRepo_ErrorsWhenEmpty(t *testing.T) {
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	_ = os.Chdir(t.TempDir())
	cmd := newTestRootCmd()
	_, err := resolveRepo(cmd, []string{})
	if err == nil {
		t.Error("expected error when no config and no arg")
	}
}
