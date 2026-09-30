package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func TestSchemaImportDescribeFeedsPinnedCheckAndTest(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "reports", "org-describe.json")
	output := filepath.Join(root, ".glade", "schema", "org.json")
	if err := os.MkdirAll(filepath.Dir(input), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	describe := `{"objects":[{"name":"Account","label":"Account","labelPlural":"Accounts","fields":[{"name":"Id","type":"id","label":"Account ID","nillable":false},{"name":"Org_Only__c","type":"string","label":"Org Only","nillable":true,"createable":true,"updateable":true}]}]}`
	if err := os.WriteFile(input, []byte(describe), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"schema", "import", "describe", "--input", input, "--output", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("schema import exit=%d stderr=%s", code, stderr.String())
	}
	snapshot, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	digestBytes := sha256.Sum256(snapshot)
	digest := hex.EncodeToString(digestBytes[:])
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("project:\n  schemaSnapshot: .glade/schema/org.json\n  schemaSnapshotSHA256: "+digest+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SnapshotProbe.cls"), `
public class SnapshotProbe {
  public static Account build() {
    return new Account(Org_Only__c = 'local');
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SnapshotProbeTest.cls"), `
@isTest
private class SnapshotProbeTest {
  @isTest static void usesImportedField() {
    System.assertEquals('local', SnapshotProbe.build().Org_Only__c);
  }
}
`)

	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr); code != 0 {
		t.Fatalf("check exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"test", "--project", root, "--json", "--no-progress", "--no-cache"}, &stdout, &stderr); code != 0 {
		t.Fatalf("test exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var run struct {
		Summary testreport.Summary `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.Summary.Total != 1 || run.Summary.Passed != 1 {
		t.Fatalf("test summary = %#v", run.Summary)
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

func TestRunTestRejectsMixedValidMissingClassFileJSON(t *testing.T) {
	root := selectionFixtureRoot(t)
	classFile := filepath.Join(t.TempDir(), "classes.txt")
	if err := os.WriteFile(classFile, []byte("AccountServiceTest\nMissingTest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class-file", classFile, "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	envelope := decodeSelectorFailureEnvelope(t, stdout.Bytes())
	if code != 1 || envelope.ExitCode != code || envelope.Status != "failed" ||
		envelope.Summary.Total != 1 || envelope.Summary.Errors != 1 || envelope.Summary.Passed != 0 {
		t.Fatalf("mixed class-file selection exit=%d envelope=%#v stdout=%q stderr=%q", code, envelope, stdout.String(), stderr.String())
	}
	if got := firstSelectorFailureMessage(envelope.Data); !strings.Contains(got, `no test class matched --class-file entry "MissingTest"`) {
		t.Fatalf("selector message = %q stdout=%s", got, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON selector failure leaked stderr: %q", stderr.String())
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

func TestWriteTestJSONEnvelopeMarksEmptySelection(t *testing.T) {
	var stdout bytes.Buffer
	if err := writeTestJSONEnvelope(&stdout, testreport.Run{}, ""); err != nil {
		t.Fatalf("write empty test envelope: %v", err)
	}
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode empty test envelope: %v\n%s", err, stdout.String())
	}
	if envelope.Status != "empty" || envelope.ExitCode != 0 || envelope.Summary.Total != 0 {
		t.Fatalf("empty selection envelope = %#v; want status=empty, exitCode=0, total=0", envelope)
	}
}

func TestWriteTestJSONEnvelopeMarksSkippedCasesPartial(t *testing.T) {
	result := testreport.Run{Suites: []testreport.Suite{{
		Name: "skipped suite",
		Cases: []testreport.Case{{
			ClassName:  "ExampleTest",
			MethodName: "notRun",
			Status:     testreport.StatusSkipped,
		}},
	}}}
	var stdout bytes.Buffer
	if err := writeTestJSONEnvelope(&stdout, result, ""); err != nil {
		t.Fatalf("write skipped test envelope: %v", err)
	}
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []struct {
			Status testreport.Status `json:"status"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode skipped test envelope: %v\n%s", err, stdout.String())
	}
	if envelope.Status != "partial" || envelope.ExitCode != 0 || envelope.Summary.Total != 1 || envelope.Summary.Skipped != 1 || len(envelope.Tests) != 1 || envelope.Tests[0].Status != testreport.StatusSkipped {
		t.Fatalf("skipped test envelope = %#v; want partial/0 with one explicit skipped result", envelope)
	}
}

func TestRunTestValidKnownFailureJSON(t *testing.T) {
	root := selectionFixtureRoot(t)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/KnownFailureTest.cls"), `
@isTest private class KnownFailureTest {
  @isTest static void failsAsExpected() {
    System.assert(false, 'expected control failure');
  }
}`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "KnownFailureTest", "--method", "failsAsExpected", "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []struct {
			Status testreport.Status `json:"status"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode known-failure envelope: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if code != 1 || envelope.ExitCode != code || envelope.Status != "failed" || envelope.Summary.Total != 1 || envelope.Summary.Failed != 1 || len(envelope.Tests) != 1 || envelope.Tests[0].Status != testreport.StatusFail {
		t.Fatalf("known-failure result exit=%d envelope=%#v\nstdout=%s\nstderr=%s", code, envelope, stdout.String(), stderr.String())
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
	perfPath := filepath.Join(t.TempDir(), "empty-test-perf.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "AccountServiceTest", "--filter", "noSuch", "--json", "--no-cache", "--no-progress", "--perf-json", perfPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, stdout.String())
	}
	if envelope.Status != "empty" || envelope.ExitCode != code || envelope.Summary.Total != 0 {
		t.Fatalf("empty selection envelope = %#v, process exit=%d", envelope, code)
	}
	run, err := decodeTestRunJSON(stdout.Bytes())
	if err != nil {
		t.Fatalf("decode run: %v\n%s", err, stdout.String())
	}
	if got := run.Summary(); got.Total != 0 || got.Errors != 0 {
		t.Fatalf("summary = %#v stdout=%s", got, stdout.String())
	}
	perfBytes, err := os.ReadFile(perfPath)
	if err != nil {
		t.Fatalf("read perf output: %v", err)
	}
	var perf struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
	}
	if err := json.Unmarshal(perfBytes, &perf); err != nil {
		t.Fatalf("decode perf output: %v\n%s", err, string(perfBytes))
	}
	if perf.Status != "empty" || perf.ExitCode != code || perf.Summary.Total != 0 {
		t.Fatalf("empty selection perf output = %#v, process exit=%d", perf, code)
	}
}

func TestRunTestRejectsEmptyClassFile(t *testing.T) {
	root := selectionFixtureRoot(t)
	classFile := filepath.Join(t.TempDir(), "empty-classes.txt")
	if err := os.WriteFile(classFile, []byte("# no explicit classes\n  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class-file", classFile, "--json", "--no-cache", "--no-progress"}, &stdout, &stderr)
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []struct {
			Status  testreport.Status   `json:"status"`
			Problem *testreport.Problem `json:"problem"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode empty class-file selector result: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if code != 1 || envelope.ExitCode != code || envelope.Status != "failed" || envelope.Summary.Total != 1 || envelope.Summary.Errors != 1 || len(envelope.Tests) != 1 || envelope.Tests[0].Status != testreport.StatusRuntimeError || envelope.Tests[0].Problem == nil || envelope.Tests[0].Problem.Type != "Selector" || !strings.Contains(envelope.Tests[0].Problem.Message, "must contain at least one test class") {
		t.Fatalf("empty class file result exit=%d envelope=%#v stdout=%q stderr=%q", code, envelope, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON selector failure leaked non-JSON stderr: %q", stderr.String())
	}
}

func TestEmptyClassFileSelectionsWriteRequestedArtifacts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		contents   string
		wantExit   int
		wantStatus string
	}{
		{name: "hand-written empty", contents: "# no classes\n", wantExit: 1, wantStatus: "failed"},
		{name: "generated empty shard", contents: generatedEmptyClassShardMarker + "\n", wantExit: 0, wantStatus: "empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := selectionFixtureRoot(t)
			outputDir := t.TempDir()
			classFile := filepath.Join(outputDir, "classes.txt")
			junitPath := filepath.Join(outputDir, "junit.xml")
			tracePath := filepath.Join(outputDir, "trace.json")
			if err := os.WriteFile(classFile, []byte(tc.contents), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"test", "--project", root, "--class-file", classFile, "--json", "--no-cache", "--no-progress", "--junit", junitPath, "--trace", tracePath}, &stdout, &stderr)
			var envelope struct {
				Status    string `json:"status"`
				ExitCode  int    `json:"exitCode"`
				Artifacts []struct {
					Kind string `json:"kind"`
					Path string `json:"path"`
				} `json:"artifacts"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode envelope: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
			}
			if code != tc.wantExit || envelope.ExitCode != code || envelope.Status != tc.wantStatus || stderr.Len() != 0 {
				t.Fatalf("exit=%d envelope=%#v stderr=%s", code, envelope, stderr.String())
			}
			if len(envelope.Artifacts) != 1 || envelope.Artifacts[0].Kind != "junit" || envelope.Artifacts[0].Path != junitPath {
				t.Fatalf("JUnit artifact missing from envelope: %#v", envelope.Artifacts)
			}
			for _, path := range []string{junitPath, tracePath} {
				data, err := os.ReadFile(path)
				if err != nil || len(data) == 0 {
					t.Fatalf("read requested artifact %s: bytes=%d err=%v", path, len(data), err)
				}
			}
		})
	}
}

func TestEmptyClassFileSelectionRejectsProfilingArtifacts(t *testing.T) {
	root := selectionFixtureRoot(t)
	classFile := filepath.Join(t.TempDir(), "empty-shard.txt")
	if err := os.WriteFile(classFile, []byte(generatedEmptyClassShardMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--perf-json", "--cpu-profile", "--mem-profile"} {
		t.Run(flag, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "profile.out")
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"test", "--project", root, "--class-file", classFile, flag, output, "--no-progress"}, &stdout, &stderr)
			if code == 0 || !strings.Contains(stderr.String(), flag+" cannot be combined with an empty --class-file selection") {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("unexpected profiling artifact %s: err=%v", output, err)
			}
		})
	}
}

func TestGeneratedEmptyClassShardIsSafeNoop(t *testing.T) {
	root := selectionFixtureRoot(t)
	shardDir := filepath.Join(t.TempDir(), "shards")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--write-class-shards", shardDir, "--shard-count", "5", "--no-cache", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("write class shards exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(shardDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("generated shard files = %d, want 5", len(entries))
	}
	emptyShardCount := 0
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(shardDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if len(bytes.TrimSpace(data)) == 0 {
			t.Fatalf("generated shard %s is a zero-byte selection", entry.Name())
		}
		if strings.TrimSpace(string(data)) != generatedEmptyClassShardMarker {
			continue
		}
		emptyShardCount++
		var emptyStdout, emptyStderr bytes.Buffer
		emptyExit := Run(context.Background(), []string{"test", "--project", root, "--class-file", filepath.Join(shardDir, entry.Name()), "--json", "--no-cache", "--no-progress"}, &emptyStdout, &emptyStderr)
		var envelope struct {
			Status   string             `json:"status"`
			ExitCode int                `json:"exitCode"`
			Summary  testreport.Summary `json:"summary"`
		}
		if err := json.Unmarshal(emptyStdout.Bytes(), &envelope); err != nil {
			t.Fatalf("decode generated empty shard %s: %v\nstdout=%s\nstderr=%s", entry.Name(), err, emptyStdout.String(), emptyStderr.String())
		}
		if emptyExit != 0 || envelope.ExitCode != 0 || envelope.Status != "empty" || envelope.Summary.Total != 0 || emptyStderr.Len() != 0 {
			t.Fatalf("generated empty shard %s result exit=%d envelope=%#v stderr=%q", entry.Name(), emptyExit, envelope, emptyStderr.String())
		}
	}
	if emptyShardCount == 0 {
		t.Fatal("expected at least one generated empty shard")
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
