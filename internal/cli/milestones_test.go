package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/gh"
)

var testMilestones = []gh.MilestoneInfo{
	{Number: 1, Title: "v1-tablestakes", State: "closed", OpenIssues: 0, ClosedIssues: 8},
	{Number: 2, Title: "The Long Title That Exceeds Width By A Lot and Should Be Truncated", State: "open", OpenIssues: 3},
	{Number: 3, Title: "Due Soon", State: "open", OpenIssues: 1, Description: "Urgent milestone"},
}

func TestRenderMilestonesOutputContainsRequiredFields(t *testing.T) {
	var buf strings.Builder
	renderMilestonesText(testMilestones, &buf)
	got := buf.String()

	// The JSON contract requires the title column to be present
	if !containsAny(got, "title") {
		t.Error("header should contain column name 'title'")
	}
	// Open milestone should be green
	if !strings.Contains(got, "open") {
		t.Error("output should contain the word 'open' for open milestones")
	}
	// Closed milestone should have a different style
	if !strings.Contains(got, "closed") {
		t.Error("output should contain the word 'closed' for closed milestones")
	}
}

func TestMilestoneInfoJSONContract(t *testing.T) {
	m := gh.MilestoneInfo{
		Number:       42,
		Title:        "Test Milestone",
		State:        "open",
		OpenIssues:   7,
		ClosedIssues: 2,
		Description:  "test desc",
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal MilestoneInfo: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Check all required fields are present
	for _, field := range []string{"number", "title", "state", "open_issues", "closed_issues", "description"} {
		if _, ok := result[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
}

func TestMilestoneInfoDueOnOmittedWhenNil(t *testing.T) {
	m := gh.MilestoneInfo{Number: 1, Title: "No Due"}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, has := result["due_on"]; has {
		t.Error("due_on should be omitted when nil")
	}
}

func TestMilestoneInfoDueOnPresentWhenSet(t *testing.T) {
	dueOn := "2026-01-01T00:00:00Z"
	m := gh.MilestoneInfo{Number: 1, Title: "Has Due", DueOn: &dueOn}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, has := result["due_on"]; !has {
		t.Error("due_on should be present when set")
	}
}

func containsAny(s, substr string) bool {
	return strings.Contains(s, substr)
}
