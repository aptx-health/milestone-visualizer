package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/gh"
)

var testMilestones = []gh.MilestoneInfo{
	{Number: 1, Title: "v1-tablestakes", State: "closed", OpenIssues: 0, ClosedIssues: 8},
	{Number: 2, Title: "v2-api-refactor", State: "open", OpenIssues: 5, ClosedIssues: 3},
	{Number: 3, Title: "v3-performance", State: "open", OpenIssues: 12, ClosedIssues: 4},
	{Number: 4, Title: "v4-batches", State: "all", OpenIssues: 2, ClosedIssues: 1},
}

func TestRenderMilestonesText_NoPanic(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	renderMilestonesText(testMilestones)

	w.Close()
	os.Stdout = old
	_, err := io.Copy(io.Discard, r)
	if err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
}

func TestRenderMilestonesText_OutputHasContent(t *testing.T) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	renderMilestonesText(testMilestones)

	w.Close()
	os.Stdout = old
	_, err := io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestMilestonesJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	data := map[string]any{
		"owner":      "test",
		"repo":       "test-repo",
		"milestones": testMilestones,
		"count":      len(testMilestones),
	}
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty JSON output")
	}
}
