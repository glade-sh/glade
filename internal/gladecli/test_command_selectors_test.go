package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestSchemaImportDescribeWritesSchema(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "describe.json")
	output := filepath.Join(root, "schema.json")
	data := `{"objects":[{"name":"Account","label":"Account","labelPlural":"Accounts","fields":[{"name":"Id","type":"id","label":"Account ID","nillable":false},{"name":"Name","type":"string","label":"Account Name","nillable":false,"createable":true,"updateable":true}]}]}`
	if err := os.WriteFile(input, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "import", "describe", "--input", input, "--output", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `"name": "Account"`) {
		t.Fatalf("schema output missing Account:\n%s", string(written))
	}
}

func TestSchemaImportDescribeWritesProjectCache(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reportsDir := filepath.Join(root, "reports")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(reportsDir, "org-describe.json")
	output := filepath.Join(root, "schema", "local.schema.json")
	data := `{"objects":[{"name":"Account","label":"Account","labelPlural":"Accounts","fields":[{"name":"Id","type":"id","label":"Account ID","nillable":false},{"name":"Name","type":"string","label":"Account Name","nillable":false,"createable":true,"updateable":true}]},{"name":"Widget__c","label":"Widget","labelPlural":"Widgets","fields":[{"name":"Id","type":"id","label":"Widget ID","nillable":false},{"name":"Title__c","type":"string","label":"Title","nillable":true,"createable":true,"updateable":true}]}]}`
	if err := os.WriteFile(input, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "import", "describe", "--input", input, "--output", output, "--project-cache", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `"name": "Account"`) || !strings.Contains(string(written), `"name": "Widget__c"`) {
		t.Fatalf("schema output missing objects:\n%s", string(written))
	}
	if _, err := os.Stat(filepath.Join(codeintel.CacheDir(root), "index.json")); err != nil {
		t.Fatalf("cache file stat: %v", err)
	}
	graph, _, err := codeintel.ReadCache(root)
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}
	if got := graph.Symbols[codeintel.SObjectID("Account")]; got.Kind != codeintel.SymbolSObject || got.Name != "Account" {
		t.Fatalf("Account symbol = %#v", got)
	}
	if got := graph.Symbols[codeintel.SObjectFieldID("Widget__c", "Title__c")]; got.Kind != codeintel.SymbolSObjectField || got.Type != "Text" {
		t.Fatalf("Widget Title symbol = %#v", got)
	}
}

func TestSchemaImportDescribeProjectCacheRequiresProjectRoot(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "describe.json")
	if err := os.WriteFile(input, []byte(`{"objects":[{"name":"Account","fields":[{"name":"Id","type":"id","nillable":false}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "import", "describe", "--input", input, "--project-cache", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "not a Glade project root") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunTestSelectsExactClass(t *testing.T) {
	run := runSelectionTest(t, "--class", "AccountServiceTest")
	if got, want := classNames(run), []string{"AccountServiceTest"}; !equalStrings(got, want) {
		t.Fatalf("classes = %#v, want %#v", got, want)
	}
}

func TestRunTestSelectsExactMethod(t *testing.T) {
	run := runSelectionTest(t, "--class", "AccountServiceTest", "--method", "testCreatesAccount")
	if got, want := caseNames(run), []string{"AccountServiceTest.testCreatesAccount"}; !equalStrings(got, want) {
		t.Fatalf("cases = %#v, want %#v", got, want)
	}
}

func TestRunTestRejectsMissingExactClassSelectorJSON(t *testing.T) {
	root := selectionFixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "NoSuchTest", "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	envelope := decodeSelectorFailureEnvelope(t, stdout.Bytes())
	if envelope.Status != "failed" || envelope.ExitCode == 0 {
		t.Fatalf("envelope status=%q exitCode=%d stdout=%s", envelope.Status, envelope.ExitCode, stdout.String())
	}
	if envelope.Summary.Total != 1 || envelope.Summary.Errors != 1 {
		t.Fatalf("summary = %#v stdout=%s", envelope.Summary, stdout.String())
	}
	if got := firstSelectorFailureMessage(envelope.Data); !strings.Contains(got, `no test class matched --class "NoSuchTest"`) {
		t.Fatalf("selector message = %q stdout=%s", got, stdout.String())
	}
}

func TestRunTestRejectsMissingExactMethodSelectorJSON(t *testing.T) {
	root := selectionFixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "AccountServiceTest", "--method", "noSuch", "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	envelope := decodeSelectorFailureEnvelope(t, stdout.Bytes())
	if envelope.Summary.Total != 1 || envelope.Summary.Errors != 1 {
		t.Fatalf("summary = %#v stdout=%s", envelope.Summary, stdout.String())
	}
	if got := firstSelectorFailureMessage(envelope.Data); !strings.Contains(got, `no test method matched --class "AccountServiceTest" --method "noSuch"`) {
		t.Fatalf("selector message = %q stdout=%s", got, stdout.String())
	}
}

func TestRunTestRejectsMissingExactMethodSelectorConsole(t *testing.T) {
	root := selectionFixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "AccountServiceTest", "--method", "noSuch", "--no-cache", "--no-progress"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"Glade test",
		"no test method matched",
		`--class "AccountServiceTest" --method "noSuch"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s\nstderr=%s", want, stdout.String(), stderr.String())
		}
	}
}

func TestRunTestSelectorFailureDoesNotPopulateLastFailed(t *testing.T) {
	root := selectionFixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "AccountServiceTest", "--method", "noSuch", "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	failures, err := readLastFailedTests(root)
	if err != nil {
		t.Fatalf("read last-failed: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("selector failure populated last-failed filters: %#v", failures)
	}
}

func TestRunTestAllowsBroadFilterToSelectZeroWithExactClass(t *testing.T) {
	root := selectionFixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "AccountServiceTest", "--filter", "noSuch", "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	run, err := decodeTestRunJSON(stdout.Bytes())
	if err != nil {
		t.Fatalf("decode run: %v\n%s", err, stdout.String())
	}
	if got := run.Summary(); got.Total != 0 || got.Errors != 0 {
		t.Fatalf("summary = %#v stdout=%s", got, stdout.String())
	}
}

func TestRunTestSelectsClassFile(t *testing.T) {
	root := selectionFixtureRoot(t)
	classFile := filepath.Join(t.TempDir(), "tests.txt")
	if err := os.WriteFile(classFile, []byte("# comment\nBillingServiceTest\n\nContactServiceTest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := runSelectionTestInRoot(t, root, "--class-file", classFile)
	if got, want := classNames(run), []string{"BillingServiceTest", "ContactServiceTest"}; !equalStrings(got, want) {
		t.Fatalf("classes = %#v, want %#v", got, want)
	}
}

func TestRunTestSelectsDeterministicClassShard(t *testing.T) {
	run := runSelectionTest(t, "--shard-count", "2", "--shard-index", "1")
	if got, want := classNames(run), []string{"AccountServiceTestExtra", "ContactServiceTest"}; !equalStrings(got, want) {
		t.Fatalf("classes = %#v, want %#v", got, want)
	}
}

func TestLoadCLIDurationHistoryReadsClassAndMethodMaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perf.json")
	data := `{
	  "classDurations": {"SlowClass": 9000, "FastClass": 10},
	  "methodDurations": {"SlowClass.slow": 8000, "SlowClass.fast": 20}
	}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	history, err := loadCLIDurationHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if history.Classes["SlowClass"] != 9000 {
		t.Fatalf("class duration missing: %#v", history.Classes)
	}
	if history.Methods["SlowClass.slow"] != 8000 {
		t.Fatalf("method duration missing: %#v", history.Methods)
	}
}

func TestDefaultCLIDurationHistoryPathUsesGladeCache(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, ".glade", "test-durations.json")
	if got := defaultCLIDurationHistoryPath(root); got != want {
		t.Fatalf("default duration history path = %q, want %q", got, want)
	}
}

func TestWriteCLIDurationHistoryMergesObservedDurations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-durations.json")
	existing := cliDurationHistory{
		Classes: map[string]int64{"BillingTest": 1000},
		Methods: map[string]int64{"BillingTest.slow": 900},
	}
	run := testreport.Run{Suites: []testreport.Suite{{
		Name: "BillingTest",
		Cases: []testreport.Case{{
			ClassName:  "BillingTest",
			MethodName: "slow",
			Status:     testreport.StatusPass,
			DurationMS: 2100,
		}, {
			ClassName:  "BillingTest",
			MethodName: "fast",
			Status:     testreport.StatusPass,
			DurationMS: 300,
		}},
	}}}

	if err := writeCLIDurationHistory(path, run, existing); err != nil {
		t.Fatal(err)
	}
	history, err := loadCLIDurationHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := history.Classes["BillingTest"], int64(1350); got != want {
		t.Fatalf("merged class duration = %d, want %d", got, want)
	}
	if got, want := history.Methods["BillingTest.slow"], int64(1200); got != want {
		t.Fatalf("merged method duration = %d, want %d", got, want)
	}
	if got := history.Methods["BillingTest.fast"]; got != 300 {
		t.Fatalf("new method duration = %d, want 300", got)
	}
}

func runSelectionTest(t *testing.T, args ...string) testreport.Run {
	t.Helper()
	return runSelectionTestInRoot(t, selectionFixtureRoot(t), args...)
}

func runSelectionTestInRoot(t *testing.T, root string, args ...string) testreport.Run {
	t.Helper()
	cliArgs := []string{"test", "--project", root, "--json", "--no-cache", "--no-progress"}
	cliArgs = append(cliArgs, args...)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), cliArgs, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	run, err := decodeTestRunJSON(stdout.Bytes())
	if err != nil {
		t.Fatalf("decode run: %v\n%s", err, stdout.String())
	}
	return run
}

func decodeTestRunJSON(data []byte) (testreport.Run, error) {
	var envelope struct {
		Data testreport.Run `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Data.Suites) > 0 {
		return envelope.Data, nil
	}
	var run testreport.Run
	err := json.Unmarshal(data, &run)
	return run, err
}

type selectorFailureEnvelope struct {
	Status   string             `json:"status"`
	ExitCode int                `json:"exitCode"`
	Summary  testreport.Summary `json:"summary"`
	Data     testreport.Run     `json:"data"`
}

func decodeSelectorFailureEnvelope(t *testing.T, data []byte) selectorFailureEnvelope {
	t.Helper()
	var envelope selectorFailureEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode selector failure envelope: %v\n%s", err, string(data))
	}
	return envelope
}

func firstSelectorFailureMessage(run testreport.Run) string {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem != nil {
				return testCase.Problem.Message
			}
		}
	}
	return ""
}

func selectionFixtureRoot(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "test-selection"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "test-selection")
	if err := copySelectionFixture(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copySelectionFixture(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func classNames(run testreport.Run) []string {
	seen := map[string]bool{}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.ClassName != "" {
				seen[testCase.ClassName] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func caseNames(run testreport.Run) []string {
	var out []string
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			out = append(out, testCase.ClassName+"."+testCase.MethodName)
		}
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
