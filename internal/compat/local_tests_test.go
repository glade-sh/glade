package compat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/testreport"
)

func TestRunLocalTestsClassifiesBasicFixture(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "basic")})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Total != 3 || report.Summary.Pass != 1 || report.Summary.AssertFailures != 1 || report.Summary.Unsupported != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if report.Ready {
		t.Fatalf("ready = true, want false")
	}
	var failing LocalTestOutcome
	for _, outcome := range report.Outcomes {
		if outcome.Class == "FailingTest" {
			failing = outcome
			break
		}
	}
	if failing.TraceEvents == 0 || failing.ProfileEvents == 0 || len(failing.ProfileCategories) == 0 {
		t.Fatalf("failing outcome missing trace/profile summary: %#v", failing)
	}
}

func TestRunLocalTestsReportsTopFailures(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{
		Project:     filepath.Join("..", "..", "testdata", "local-tests", "basic"),
		TopFailures: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.TopFailures) != 2 {
		t.Fatalf("topFailures = %#v", report.TopFailures)
	}
	if report.TopFailures[0].Count == 0 || report.TopFailures[0].Outcome == "pass" {
		t.Fatalf("topFailures[0] = %#v", report.TopFailures[0])
	}
}

func TestLocalTestRunOutcomeSplitsRuntimeAndTimeout(t *testing.T) {
	runtimeGap := localTestRunOutcome("fixture", testreport.Case{
		ClassName:  "RuntimeGapTest",
		MethodName: "fails",
		Status:     testreport.StatusFail,
		Problem:    &testreport.Problem{Type: "RuntimeError", Message: "method dispatch failed"},
	})
	if runtimeGap.Outcome != "runtime_gap" {
		t.Fatalf("runtime outcome = %#v", runtimeGap)
	}

	assertFail := localTestRunOutcome("fixture", testreport.Case{
		ClassName:  "AssertTest",
		MethodName: "fails",
		Status:     testreport.StatusFail,
		Problem:    &testreport.Problem{Type: "AssertException", Message: "Assertion Failed"},
	})
	if assertFail.Outcome != "assert_fail" || assertFail.Phase != "assert" {
		t.Fatalf("assert outcome = %#v", assertFail)
	}

	timeout := localTestRunOutcome("fixture", testreport.Case{
		ClassName:  "TimeoutTest",
		MethodName: "hangs",
		Status:     testreport.StatusUnsupported,
		Problem:    &testreport.Problem{Type: "Canceled", Message: "context deadline exceeded"},
	})
	if timeout.Outcome != "timeout" || timeout.CapabilityID != "apex.test.timeout" {
		t.Fatalf("timeout outcome = %#v", timeout)
	}
}

func TestRunLocalTestsPlatformAPIsFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "platform-apis")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 4 || report.Summary.Pass != 4 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsNamedCredentialCalloutsFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "named-credential-callouts")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 2 || report.Summary.Pass != 2 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsFilesEmailFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "files-email")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 2 || report.Summary.Pass != 2 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsWorkflowFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "workflow")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 1 || report.Summary.Pass != 1 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsFlowFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "flow")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 1 || report.Summary.Pass != 1 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsResourcesLabelsFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "resources-labels")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 2 || report.Summary.Pass != 2 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsUIControllerContractsFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "ui-controller-contracts")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 2 || report.Summary.Pass != 2 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsVisualforcePagesFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "visualforce-pages")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 3 || report.Summary.Pass != 3 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsOrgLikeRunnerFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "org-like-runner")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 2 || report.Summary.Pass != 2 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsEnterpriseComposedFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 2 || report.Summary.Pass != 2 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestRunLocalTestsMetadataDeployFixtureReady(t *testing.T) {
	report, err := RunLocalTests(LocalTestOptions{Project: filepath.Join("..", "..", "testdata", "local-tests", "metadata-deploy")})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Ready {
		t.Fatalf("ready = false, summary = %#v outcomes = %#v", report.Summary, report.Outcomes)
	}
	if report.Summary.Total != 1 || report.Summary.Pass != 1 || report.Summary.CompileErrors != 0 || report.Summary.Unsupported != 0 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestCheckLocalTestCorpusFixture(t *testing.T) {
	report, err := CheckLocalTestCorpus(filepath.Join("..", "..", "docs", "fixtures", "local-tests-corpus.json"))
	if err != nil {
		t.Fatalf("CheckLocalTestCorpus error = %v, report = %#v", err, report)
	}
	if !report.Ready || len(report.Projects) != 13 {
		t.Fatalf("report = %#v", report)
	}
}

func TestCheckPostParityTraceFixture(t *testing.T) {
	report, err := CheckPostParityTraceFixture(filepath.Join("..", "..", "docs", "fixtures", "post-parity-trace-events.json"))
	if err != nil {
		t.Fatalf("CheckPostParityTraceFixture error = %v, report = %#v", err, report)
	}
	if !report.Ready || len(report.Surfaces) != 3 {
		t.Fatalf("report = %#v", report)
	}
}

func TestCheckUIControllerDiscoveryFixture(t *testing.T) {
	report, err := CheckUIControllerDiscovery(filepath.Join("..", "..", "docs", "fixtures", "ui-controller-discovery.json"))
	if err != nil {
		t.Fatalf("CheckUIControllerDiscovery error = %v, report = %#v", err, report)
	}
	if !report.Ready || report.Summary.AuraBundles != 1 || report.Summary.LWCBundles != 1 || report.Summary.UnresolvedApex != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunLocalTestsReportsLoadError(t *testing.T) {
	root := t.TempDir()
	writeLocalTestFile(t, filepath.Join(root, "sfdx-project.json"), `{`)
	report, err := RunLocalTests(LocalTestOptions{Project: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.LoadErrors != 1 || report.Outcomes[0].Outcome != "load_error" {
		t.Fatalf("report = %#v", report)
	}
}

func writeLocalTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
