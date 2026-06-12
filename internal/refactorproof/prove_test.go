package refactorproof

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
)

func TestResultStatusRanksFailuresFirst(t *testing.T) {
	result := Result{Stages: []StageResult{
		{Name: StageAffectedTests, Status: StageStatusNotRun},
		{Name: StageSemanticCheck, Status: StageStatusPass},
		{Name: StageAPISurfaceDelta, Status: StageStatusFail},
	}}

	if got := result.Status(); got != StageStatusFail {
		t.Fatalf("status = %q, want %q", got, StageStatusFail)
	}
}

func TestResultStatusWarnsWhenTestsAreNotRun(t *testing.T) {
	result := Result{Stages: []StageResult{
		{Name: StageGitDiff, Status: StageStatusPass},
		{Name: StageAffectedTests, Status: StageStatusNotRun},
	}}

	if got := result.Status(); got != StageStatusWarn {
		t.Fatalf("status = %q, want %q", got, StageStatusWarn)
	}
}

func TestProofFindingsWarnWhenStageNotRun(t *testing.T) {
	findings := proofFindings([]StageResult{{
		Name:    StageAffectedTests,
		Status:  StageStatusNotRun,
		Message: "affected tests not run",
	}})
	if len(findings) != 1 {
		t.Fatalf("findings = %#v", findings)
	}
	if findings[0].Severity != enterprise.SeverityMedium {
		t.Fatalf("severity = %q, want %q", findings[0].Severity, enterprise.SeverityMedium)
	}
}

func TestProveReturnsEnterpriseReportWithSchemaVersion(t *testing.T) {
	root := filepath.Clean("../..")

	result, err := Prove(context.Background(), Options{
		Root:  root,
		Since: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Report.SchemaVersion != enterprise.SchemaVersion {
		t.Fatalf("schema version = %q, want %q", result.Report.SchemaVersion, enterprise.SchemaVersion)
	}
	if len(result.Stages) == 0 {
		t.Fatal("expected proof stages")
	}
}
