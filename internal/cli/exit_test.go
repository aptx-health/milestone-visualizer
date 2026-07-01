package cli

import (
	"testing"

	"github.com/aptx-health/ms-visualizer/internal/msview"
)

func mkFinding(rule, sev string) msview.Finding {
	return msview.Finding{Rule: rule, Severity: sev}
}

func makeReport(fs ...msview.Finding) msview.DoctorReport {
	r := msview.DoctorReport{Findings: fs}
	for _, f := range fs {
		switch f.Severity {
		case "error":
			r.Counts.Error++
		case "warn":
			r.Counts.Warn++
		case "info":
			r.Counts.Info++
		}
	}
	return r
}

func TestShouldFail_DefaultOnlyOnError(t *testing.T) {
	// Empty fail-on = default = "error"
	if shouldFail(makeReport(), nil) {
		t.Error("clean report should not fail")
	}
	if shouldFail(makeReport(mkFinding("x", "warn")), nil) {
		t.Error("warn should not fail by default")
	}
	if !shouldFail(makeReport(mkFinding("x", "error")), nil) {
		t.Error("error should fail by default")
	}
}

func TestShouldFail_ExplicitSeverities(t *testing.T) {
	rep := makeReport(mkFinding("x", "warn"))
	if !shouldFail(rep, []string{"warn"}) {
		t.Error("fail-on=warn should trigger on warn")
	}
	if shouldFail(rep, []string{"error"}) {
		t.Error("fail-on=error should not trigger on warn")
	}
}

func TestShouldFail_RuleName(t *testing.T) {
	rep := makeReport(mkFinding(msview.RuleMismatch, "error"))
	if !shouldFail(rep, []string{msview.RuleMismatch}) {
		t.Error("fail-on with rule name should trigger")
	}
	if shouldFail(rep, []string{msview.RuleOrphan}) {
		t.Error("fail-on with different rule should not trigger")
	}
}

func TestShouldFail_Any(t *testing.T) {
	rep := makeReport(mkFinding("x", "info"))
	if !shouldFail(rep, []string{"any"}) {
		t.Error("fail-on=any should trigger on info")
	}
	if shouldFail(makeReport(), []string{"any"}) {
		t.Error("fail-on=any should not trigger on clean")
	}
}

func TestExitCodesAreStable(t *testing.T) {
	// Guard against accidental renumbering.
	if ExitOK != 0 || ExitRuntimeError != 1 || ExitLintFindings != 2 {
		t.Errorf("exit codes changed: OK=%d Runtime=%d Lint=%d", ExitOK, ExitRuntimeError, ExitLintFindings)
	}
}
