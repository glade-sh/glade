package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/pluginhost"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func replaceDefaultHTTPClient(t *testing.T, fn roundTripFunc) func() {
	t.Helper()
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: fn}
	return func() {
		http.DefaultClient = oldClient
	}
}

type cliJSONEnvelopeForTest struct {
	SchemaVersion string          `json:"schemaVersion"`
	Command       string          `json:"command"`
	Status        string          `json:"status"`
	ExitCode      int             `json:"exitCode"`
	Data          json.RawMessage `json:"data"`
}

func decodeCLIEnvelopeData(t *testing.T, raw []byte, command string, out any) cliJSONEnvelopeForTest {
	t.Helper()
	var env cliJSONEnvelopeForTest
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, string(raw))
	}
	if env.SchemaVersion != "1.0" || env.Command != command {
		t.Fatalf("unexpected envelope: %#v\n%s", env, string(raw))
	}
	if len(env.Data) == 0 {
		t.Fatalf("envelope missing data: %#v\n%s", env, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatalf("envelope data is not expected JSON: %v\n%s", err, string(raw))
		}
	}
	return env
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "glade "+Version+"\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunVersionJSONUsesCLIEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		SchemaVersion string `json:"schemaVersion"`
		Command       string `json:"command"`
		Status        string `json:"status"`
		ExitCode      int    `json:"exitCode"`
		Data          struct {
			Version string `json:"version"`
			Go      string `json:"go"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.SchemaVersion != "1.0" || got.Command != "version" || got.Status != "passed" || got.ExitCode != 0 || got.Data.Version == "" || got.Data.Go == "" {
		t.Fatalf("unexpected envelope: %#v\n%s", got, stdout.String())
	}
}

func TestRunTestWritesTraceFile(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "basic")
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "PassingTest", "--trace", tracePath, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	if !bytes.Contains(data, []byte(`"traceEvents"`)) {
		t.Fatalf("trace document missing traceEvents: %s", string(data))
	}
}

func TestRunTestPerfJSONRecordsEffectiveExecutionProvenance(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/PerfExecutionTest.cls"), `
@isTest
private class PerfExecutionTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	perfPath := filepath.Join(t.TempDir(), "test-perf.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"test", "--project", root, "--json", "--no-progress", "--no-serve", "--no-cache",
		"--no-parallel-methods", "--parallelism", "3", "--perf-json", perfPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	perf := readPerfJSONObject(t, perfPath)
	if perf["schemaVersion"] != "1.1" {
		t.Fatalf("schemaVersion = %#v, want 1.1", perf["schemaVersion"])
	}
	execution := requireJSONObject(t, perf, "execution")
	if execution["executionMode"] != "local" {
		t.Fatalf("execution.executionMode = %#v, want local", execution["executionMode"])
	}
	if execution["parallelMethods"] != false || execution["requestedParallelism"] != float64(3) || execution["effectiveParallelism"] != float64(3) {
		t.Fatalf("execution parallelism = %#v", execution)
	}
	if execution["gomaxprocs"] != float64(runtime.GOMAXPROCS(0)) {
		t.Fatalf("execution.gomaxprocs = %#v, want %d", execution["gomaxprocs"], runtime.GOMAXPROCS(0))
	}
	if execution["diskCachePolicy"] != string(apextest.DiskRuntimeCacheNoDiskCache) {
		t.Fatalf("execution.diskCachePolicy = %#v", execution["diskCachePolicy"])
	}
	args := execution["arguments"].([]any)
	if len(args) == 0 || args[0] != "test" {
		t.Fatalf("execution.arguments = %#v", args)
	}
	phases := requireJSONObject(t, requireJSONObject(t, perf, "apexPerf"), "phases")
	compileWant := int64(requireJSONNumber(t, phases, "semanticGateNs")+requireJSONNumber(t, phases, "projectCompileNs")+requireJSONNumber(t, phases, "testCompileNs")) / int64(time.Millisecond)
	if got := int64(requireJSONNumber(t, perf, "compileMs")); got != compileWant {
		t.Fatalf("compileMs = %d, want %d", got, compileWant)
	}
	startupWant := int64(requireJSONNumber(t, phases, "semanticKeyNs")+requireJSONNumber(t, phases, "semanticGateNs")+requireJSONNumber(t, phases, "projectLoadNs")+requireJSONNumber(t, phases, "schemaLoadNs")+requireJSONNumber(t, phases, "indexBuildNs")+requireJSONNumber(t, phases, "discoverNs")+requireJSONNumber(t, phases, "runtimeKeyNs")+optionalJSONNumber(t, phases, "cacheValidateNs")+optionalJSONNumber(t, phases, "cacheDecodeNs")+optionalJSONNumber(t, phases, "cacheEncodeNs")+optionalJSONNumber(t, phases, "orgBuildNs")+optionalJSONNumber(t, phases, "projectCompileNs")+optionalJSONNumber(t, phases, "testCompileNs")) / int64(time.Millisecond)
	if got := int64(requireJSONNumber(t, perf, "startupMs")); got != startupWant {
		t.Fatalf("startupMs = %d, want %d", got, startupWant)
	}
}

func TestLoadTestIndexWithPerfPhasesRetainsBuildArtifactsAcrossSchemaBranches(t *testing.T) {
	for _, tt := range []struct {
		name          string
		perfEnabled   bool
		malformedMeta bool
	}{
		{name: "schema loaded", perfEnabled: false},
		{name: "schema fallback", perfEnabled: true, malformedMeta: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			classPath := filepath.Join(root, "force-app/main/default/classes/SnapshotGenerationTest.cls")
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeTestFile(t, classPath, `@isTest private class SnapshotGenerationTest { @isTest static void passes() { System.assertEquals(1, 1); } }`)
			if tt.malformedMeta {
				writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Snapshot__c/Snapshot__c.object-meta.xml"), `<CustomObject>`)
			}

			generation, err := loadTestIndexWithPerfPhases(root, tt.perfEnabled)
			if err != nil {
				t.Fatal(err)
			}
			if generation.Artifacts.Sources == nil || generation.Artifacts.SourceDigests == nil {
				t.Fatalf("build artifacts = %#v, want source arena and digests", generation.Artifacts)
			}
			var found typesys.TypeSymbol
			for _, typ := range generation.Index.Types {
				if typ.Name == "SnapshotGenerationTest" {
					found = typ
					break
				}
			}
			if found.File == "" {
				t.Fatalf("index types = %#v, want SnapshotGenerationTest", generation.Index.Types)
			}
			if _, ok := generation.Artifacts.SourceForType(found); !ok {
				t.Fatalf("artifact source missing for %#v", found)
			}
			if _, ok := generation.Artifacts.SourceDigests.Digest(found.File); !ok {
				t.Fatalf("artifact digest missing for %q", found.File)
			}
			if tt.malformedMeta && !generation.Index.HasErrors() {
				t.Fatalf("fallback index diagnostics = %#v, want GLADESCHEMA001", generation.Index.Diagnostics)
			}
		})
	}
}

func TestRunTestRejectsTraceWithWatch(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "basic")
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "PassingTest", "--trace", tracePath, "--watch-once", "--json", "--no-progress"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit = 0 stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--trace cannot be combined with --watch or --watch-once") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Fatalf("trace path exists or stat failed unexpectedly: %v", err)
	}
}

func TestRunTestValidatesServicesConfig(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "basic")
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(fixture, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	servicesPath := filepath.Join(dir, "services.yml")
	if err := os.WriteFile(servicesPath, []byte(`version: 0
mode: strict
calloutFixtures: [fixture.json]
asyncDrain: true
asyncMaxDepth: 5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--class", "PassingTest", "--services", servicesPath, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty with --no-progress", stderr.String())
	}
}

func TestProgressModeForJSONDefaultsOff(t *testing.T) {
	if got := progressModeForFlags(true, false, false, false); string(got) != "off" {
		t.Fatalf("json progress mode = %q, want off", got)
	}
	if got := progressModeForFlags(true, true, false, false); string(got) != "visible" {
		t.Fatalf("explicit visible progress mode = %q, want visible", got)
	}
	if got := progressModeForFlags(true, false, true, false); string(got) != "json" {
		t.Fatalf("explicit json progress mode = %q, want json", got)
	}
}

func TestRunVersionJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		SchemaVersion string      `json:"schemaVersion"`
		Command       string      `json:"command"`
		Status        string      `json:"status"`
		ExitCode      int         `json:"exitCode"`
		Data          versionInfo `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got.SchemaVersion != "1.0" || got.Command != "version" || got.Status != "passed" || got.ExitCode != 0 {
		t.Fatalf("unexpected envelope: %#v\n%s", got, stdout.String())
	}
	if got.Data.Version != Version {
		t.Fatalf("version = %q, want %q", got.Data.Version, Version)
	}
	if got.Data.Go == "" || got.Data.OS == "" || got.Data.Arch == "" {
		t.Fatalf("missing version fields in %#v", got)
	}
}

func TestRunUpdateHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"update", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "glade update") {
		t.Fatalf("help omitted update usage:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "GLADE_UPDATE_ALLOW_SHELL=1") {
		t.Fatalf("help omitted update shell guard:\n%s", stdout.String())
	}
}

func TestRunUpdateDryRunPrintsInstallCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"update", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=") {
		t.Fatalf("dry run omitted install command:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "curl -fsSL https://glade.sh/install.sh | sh") {
		t.Fatalf("dry run should pin the install directory instead of using the default installer path:\n%s", stdout.String())
	}
}

func TestUpdateInstallCommandUsesExecutableDirectory(t *testing.T) {
	got := updateInstallCommandForExecutable("/tmp/Glade Bin/glade")
	want := "curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR='/tmp/Glade Bin' sh"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestUpdateInstallCommandQuotesSingleQuotes(t *testing.T) {
	got := updateInstallCommandForExecutable("/tmp/matt's bin/glade")
	want := "curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR='/tmp/matt'\\''s bin' sh"
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestToolchainStatusJSONReportsMissingToolchain(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"toolchain", "status", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	var got struct {
		OK     bool   `json:"ok"`
		Path   string `json:"path"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got.OK || got.Path == "" || !strings.Contains(got.Detail, "missing") {
		t.Fatalf("unexpected status JSON: %#v", got)
	}
}

func TestRunDoctorReportsParser(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Glade doctor", "Project", "Toolchain", "Parser", "Next:", "glade check", "glade test changed --since origin/main"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "+") || strings.Contains(out, "╭") {
		t.Fatalf("doctor output used decorative box:\n%s", out)
	}
}

func TestRunDoctorJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"doctor", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		Version      string `json:"version"`
		GoVersion    string `json:"goVersion"`
		OSArch       string `json:"osArch"`
		CWD          string `json:"cwd"`
		ExitCode     int    `json:"exitCode"`
		ParserStatus string `json:"parserStatus"`
		ParserOK     bool   `json:"parserOK"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got.Version != Version {
		t.Fatalf("version = %q, want %q", got.Version, Version)
	}
	if got.GoVersion == "" || got.OSArch == "" || got.CWD == "" || got.ParserStatus == "" {
		t.Fatalf("doctor JSON missing runtime fields: %#v", got)
	}
	if !got.ParserOK {
		t.Fatalf("parserOK = false, want true: %#v", got)
	}
	if got.ExitCode != code {
		t.Fatalf("doctor JSON exitCode = %d, process code = %d", got.ExitCode, code)
	}
}

func TestRunDoctorJSONExitCodeMatchesSetupStatus(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"doctor", "--project", root, "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got struct {
		Status        string `json:"status"`
		ExitCode      int    `json:"exitCode"`
		ConfigMissing bool   `json:"configMissing"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got.Status != "failed" || got.ExitCode != code || !got.ConfigMissing {
		t.Fatalf("doctor JSON did not match setup failure: code=%d got=%#v", code, got)
	}
}

func TestRunDoctorShortFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"doctor", "-p", ".", "-j"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got["version"] != Version {
		t.Fatalf("version = %v, want %q", got["version"], Version)
	}
}

func TestRunDoctorProjectPathMustExist(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "missing")
	code := Run(context.Background(), []string{"doctor", "--project", missing}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), missing) || !strings.Contains(stderr.String(), "no such file or directory") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunDoctorReportsProjectLocalDataEnvironment(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Label__c")
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  packageDirs: [force-app]
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"doctor", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"Local data", filepath.ToSlash(filepath.Join(root, ".glade", "envs", "dev.sqlite")), "not created"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "envs", "dev.sqlite")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("doctor should not create the default db, stat err=%v", err)
	}
}

func TestRunDoctorReportsProjectLocalDataSchemaRefresh(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "First_Field__c")
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  packageDirs: [force-app]
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	firstFieldPath := filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/First_Field__c.field-meta.xml")
	if err := os.Remove(firstFieldPath); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/Second_Field__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Second_Field__c</fullName><label>Second_Field__c</label><type>Text</type></CustomField>`)

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"doctor", "--project", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("doctor unexpectedly succeeded stdout=%q", stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"Local data", "schema changed", "glade db inspect --project"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, got)
		}
	}
}

func TestRunConfigValidateReportsOK(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  packageDirs: [force-app]
  defaultNamespace: ns
org:
  features: [PersonAccounts]
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "validate", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "status: ok") || !strings.Contains(got, filepath.Join(root, "glade.yml")) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunConfigValidateReportsParseError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  potato: true
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "validate", "--project", root}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unsupported config key "potato"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunConfigValidateMissingProjectUsesConfigExitCode(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "validate", "--project", root}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "glade.yml not found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunConfigShowJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"sfdxns","sourceApiVersion":"65.0"}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  packageDirs: [force-app, packages/core]
  defaultNamespace: glns
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["pkg:artifact:.glade/packages/pkg.glade-package.json:1.0"]
  packageShims: ["pkg:test-support/package-shims/pkg"]
org:
  features: [PersonAccounts]
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "show", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		SchemaVersion string `json:"schemaVersion"`
		Command       string `json:"command"`
		Status        string `json:"status"`
		ExitCode      int    `json:"exitCode"`
		Data          struct {
			ConfigPath                 string   `json:"configPath"`
			ConfigFound                bool     `json:"configFound"`
			ProjectRoot                string   `json:"projectRoot"`
			Namespace                  string   `json:"namespace"`
			SourceAPIVersion           string   `json:"sourceApiVersion"`
			PackageDirs                []string `json:"packageDirs"`
			OrgFeatures                []string `json:"orgFeatures"`
			ManagedPackageDependencies []struct {
				Namespace string `json:"namespace"`
			} `json:"managedPackageDependencies"`
			NamespaceRemaps []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"namespaceRemaps"`
			PackageShims []struct {
				Namespace string `json:"namespace"`
			} `json:"packageShims"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got.SchemaVersion != "1.0" || got.Command != "config show" || got.Status != "passed" || got.ExitCode != 0 {
		t.Fatalf("unexpected envelope: %#v\n%s", got, stdout.String())
	}
	if !got.Data.ConfigFound || got.Data.ConfigPath != filepath.Join(root, "glade.yml") {
		t.Fatalf("config fields = %#v", got)
	}
	if got.Data.ProjectRoot != root || got.Data.Namespace != "glns" || got.Data.SourceAPIVersion != "65.0" {
		t.Fatalf("project fields = %#v", got)
	}
	if strings.Join(got.Data.PackageDirs, ",") != "force-app,packages/core" {
		t.Fatalf("packageDirs = %#v", got.Data.PackageDirs)
	}
	if strings.Join(got.Data.OrgFeatures, ",") != "PersonAccounts" {
		t.Fatalf("orgFeatures = %#v", got.Data.OrgFeatures)
	}
	if len(got.Data.ManagedPackageDependencies) != 1 || got.Data.ManagedPackageDependencies[0].Namespace != "pkg" {
		t.Fatalf("managed package dependencies = %#v", got.Data.ManagedPackageDependencies)
	}
	if len(got.Data.NamespaceRemaps) != 1 || got.Data.NamespaceRemaps[0].From != "BasePkg" || got.Data.NamespaceRemaps[0].To != "stagepkg" {
		t.Fatalf("namespace remaps = %#v", got.Data.NamespaceRemaps)
	}
	if len(got.Data.PackageShims) != 1 || got.Data.PackageShims[0].Namespace != "pkg" {
		t.Fatalf("package shims = %#v", got.Data.PackageShims)
	}
}

func TestRunConfigShowTextReportsNamespaceRemaps(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  packageDirs: [force-app]
  namespaceRemaps: ["BasePkg:stagepkg"]
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "show", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "namespaceRemaps: 1") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunConfigInitWritesGladeYML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"sfdxns","sourceApiVersion":"65.0"}`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "init", "--project", root, "--yes", "--namespace", "glns", "--feature", "PersonAccounts", "--package-dir", "force-app", "--package-dir", "packages/core"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "glade.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"project:",
		"  packageDirs: [force-app, packages/core]",
		"  defaultNamespace: glns",
		"org:",
		"  features: [PersonAccounts]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("glade.yml missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(stdout.String(), "created: "+filepath.Join(root, "glade.yml")) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestConfigInitPromptAcceptsOverrides(t *testing.T) {
	opts := configInitOptions{
		namespace:   "pkg",
		packageDirs: []string{"force-app"},
	}
	var stdout bytes.Buffer
	if err := promptConfigInit(strings.NewReader("packages/core\nnewns\nPersonAccounts, Communities\n"), &stdout, &opts); err != nil {
		t.Fatal(err)
	}
	if strings.Join(opts.packageDirs, ",") != "packages/core" {
		t.Fatalf("packageDirs = %#v", opts.packageDirs)
	}
	if opts.namespace != "newns" {
		t.Fatalf("namespace = %q", opts.namespace)
	}
	if strings.Join(opts.features, ",") != "PersonAccounts,Communities" {
		t.Fatalf("features = %#v", opts.features)
	}
	if !strings.Contains(stdout.String(), "Package dirs [force-app]:") {
		t.Fatalf("prompt output = %q", stdout.String())
	}
}

func TestRunConfigInitRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "glade.yml"), `project:
  packageDirs: [force-app]
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"config", "init", "--project", root, "--yes"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "already exists") || !strings.Contains(stderr.String(), "--force") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunInitAliasesConfigInit(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"aliasns"}`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"init", "--project", root, "--yes"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "glade.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "  packageDirs: [force-app]") || !strings.Contains(got, "  defaultNamespace: aliasns") {
		t.Fatalf("glade.yml =\n%s", got)
	}
	if !strings.Contains(stdout.String(), "next: glade config validate --project") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"wat"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "wat"`) {
		t.Fatalf("stderr did not include diagnostic: %q", stderr.String())
	}
}

func TestRunUnknownCommandSuggestsClosestMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"chek"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "chek"`) {
		t.Fatalf("stderr did not include diagnostic: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `did you mean "check"?`) {
		t.Fatalf("stderr missing suggestion: %q", stderr.String())
	}
}

func TestRunUnknownFlagSuggestsClosestMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--projec", "."}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown flag "--projec"`) {
		t.Fatalf("stderr did not include unknown flag: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `did you mean "--project"?`) {
		t.Fatalf("stderr missing suggestion: %q", stderr.String())
	}
}

func TestRunTestUnknownFlagSuggestsClosestMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--filteer", "AccountTest"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown flag "--filteer"`) {
		t.Fatalf("stderr did not include unknown flag: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `did you mean "--filter"?`) {
		t.Fatalf("stderr missing suggestion: %q", stderr.String())
	}
}

func TestRunLeafCommandUnknownFlagSuggestsClosestMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--opne", "--once"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown flag "--opne"`) {
		t.Fatalf("stderr did not include unknown flag: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `did you mean "--open"?`) {
		t.Fatalf("stderr missing suggestion: %q", stderr.String())
	}
}

func TestCompatIsNotPublicCommand(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "compat"`) {
		t.Fatalf("stderr did not include unknown command diagnostic: %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "  compat ") || strings.Contains(stdout.String(), "glade compat") {
		t.Fatalf("public help still mentions compat:\n%s", stdout.String())
	}
}

func TestRenderIsNotPublicCommand(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"render"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "render"`) {
		t.Fatalf("stderr did not include unknown command diagnostic: %q", stderr.String())
	}
}

func testPluginsCommandConfig(t *testing.T) pluginsCommandConfig {
	t.Helper()
	return pluginsCommandConfig{
		storeRoot:   t.TempDir(),
		registryURL: pluginhost.DefaultRegistryURL,
		ci:          false,
	}
}

func runPluginsForTest(ctx context.Context, args []string, stdout, stderr io.Writer, config pluginsCommandConfig) int {
	if err := runPluginsWithConfig(ctx, args, stdout, stderr, config); err != nil {
		writeCommandError(stderr, "plugins", err)
		return 1
	}
	return 0
}

func TestPluginsListEmpty(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"list"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No plugins installed.") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestPluginsListJSONIncludesEditorMetadata(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	config := testPluginsCommandConfig(t)
	config.storeRoot = home
	manifestPath := filepath.Join(home, "plugins", "compat", "0.1.0", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["surface"],"summary":"Surface commands."}],"editor":{"actions":[{"id":"surface.refresh","title":"Refresh Surface","description":"Refresh support metadata.","view":"plugins","contexts":["project","lastLocalRun"],"command":["surface","refresh"],"args":["--json"],"inputs":[{"name":"packet","label":"Packet","type":"string","required":true,"default":"Data.Runtime"}],"output":"glade.markdownReport.v1","icon":"refresh-cw"}]}}`
	if err := os.WriteFile(manifestPath, []byte(manifest+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(home, "plugins", "installed.json")
	state := fmt.Sprintf(`{"version":1,"plugins":[{"name":"compat","canonicalName":"@glade/compat","storageName":"glade__compat","version":"0.1.0","linked":true,"commands":["surface"],"executable":"/tmp/glade-plugin-compat","manifest":%q,"source":"link:/tmp/glade-plugin-compat"}]}`+"\n", manifestPath)
	if err := os.WriteFile(statePath, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"list", "--json"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%s", code, stderr.String())
	}
	var got struct {
		SchemaVersion string `json:"schemaVersion"`
		Command       string `json:"command"`
		Status        string `json:"status"`
		ExitCode      int    `json:"exitCode"`
		Data          struct {
			Plugins []struct {
				Identity     string   `json:"identity"`
				Version      string   `json:"version"`
				Linked       bool     `json:"linked"`
				CommandRoots []string `json:"commandRoots"`
				Executable   string   `json:"executable"`
				ManifestPath string   `json:"manifestPath"`
				Source       string   `json:"source"`
				Editor       *struct {
					Actions []struct {
						ID      string   `json:"id"`
						View    string   `json:"view"`
						Command []string `json:"command"`
						Output  string   `json:"output"`
						Inputs  []struct {
							Name     string `json:"name"`
							Required bool   `json:"required"`
						} `json:"inputs"`
					} `json:"actions"`
				} `json:"editor"`
			} `json:"plugins"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got.SchemaVersion != "1.0" || got.Command != "plugins list" || got.Status != "passed" || got.ExitCode != 0 {
		t.Fatalf("unexpected envelope: %#v\n%s", got, stdout.String())
	}
	if len(got.Data.Plugins) != 1 {
		t.Fatalf("plugins length = %d, want 1: %#v", len(got.Data.Plugins), got)
	}
	plugin := got.Data.Plugins[0]
	if plugin.Identity != "@glade/compat" || plugin.Version != "0.1.0" || !plugin.Linked || plugin.CommandRoots[0] != "surface" ||
		plugin.ManifestPath != manifestPath || plugin.Source == "" || plugin.Editor == nil {
		t.Fatalf("plugin JSON missing identity fields or editor metadata: %#v", plugin)
	}
	action := plugin.Editor.Actions[0]
	if action.ID != "surface.refresh" || action.View != "plugins" || action.Command[0] != "surface" ||
		action.Output != "glade.markdownReport.v1" || action.Inputs[0].Name != "packet" || !action.Inputs[0].Required {
		t.Fatalf("unexpected editor action JSON: %#v", action)
	}
}

func TestPluginsListJSONIncludesLinkedExecutableEditorMetadata(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	home := t.TempDir()
	config := testPluginsCommandConfig(t)
	config.storeRoot = home
	exe := filepath.Join(home, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}],"editor":{"actions":[{"id":"compat.postParity","title":"Scan Unsupported Local Surfaces","view":"runs","contexts":["project"],"command":["compat","post-parity"],"args":["--project","${projectRoot}","--json","--editor-findings"],"output":"glade.findings.v1","icon":"search"}]}}'
  exit 0
fi
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"link", "--exec", exe}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("link exit=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runPluginsForTest(context.Background(), []string{"list", "--json"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%s", code, stderr.String())
	}
	var got struct {
		Data struct {
			Plugins []struct {
				Editor *struct {
					Actions []struct {
						ID     string `json:"id"`
						Output string `json:"output"`
					} `json:"actions"`
				} `json:"editor"`
			} `json:"plugins"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if len(got.Data.Plugins) != 1 || got.Data.Plugins[0].Editor == nil || len(got.Data.Plugins[0].Editor.Actions) != 1 {
		t.Fatalf("linked plugin editor metadata missing:\n%#v\n%s", got, stdout.String())
	}
	action := got.Data.Plugins[0].Editor.Actions[0]
	if action.ID != "compat.postParity" || action.Output != "glade.findings.v1" {
		t.Fatalf("unexpected linked plugin editor action: %#v", action)
	}
}

func TestPluginsListJSONCancellationCleansUpLinkedExecutableDescendant(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	exe := filepath.Join(t.TempDir(), "glade-plugin-compat")
	pidPath := exe + ".pid"
	cleanupNeeded := true
	t.Cleanup(func() {
		if cleanupNeeded {
			bestEffortKillCLIRecordedPID(pidPath)
		}
	})
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
	  sleep 30 &
	  child=$!
	  printf '%s\n' "$child" > "$0.pid"
	  wait "$child"
fi
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(exe, "prewarm").Run(); err != nil {
		t.Fatalf("prewarm helper: %v", err)
	}

	var stdout bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- writePluginsListJSONWithManifestTimeout(ctx, &stdout, []pluginhost.InstalledPlugin{{
			Name:       "compat",
			Version:    "0.1.0",
			Executable: exe,
			Source:     "link:test",
			Linked:     true,
			Commands:   []string{"compat"},
		}}, 30*time.Second)
	}()

	pid, err := waitForCLIRecordedPID(pidPath, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("plugin list cancellation took %s, want <1s", elapsed)
	}
	waitForCLIProcessAbsent(t, pid, time.Second)
	cleanupNeeded = false
}

func TestPluginsListJSONTimeoutBeforePIDWriteDoesNotRequirePIDFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	exe := filepath.Join(t.TempDir(), "glade-plugin-compat")
	pidPath := exe + ".pid"
	cleanupNeeded := true
	t.Cleanup(func() {
		if cleanupNeeded {
			bestEffortKillCLIRecordedPID(pidPath)
		}
	})
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
	  sleep 30
	  printf '%s\n' "$$" > "$0.pid"
fi
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := writePluginsListJSONWithManifestTimeout(context.Background(), &stdout, []pluginhost.InstalledPlugin{{
		Name:       "compat",
		Version:    "0.1.0",
		Executable: exe,
		Source:     "link:test",
		Linked:     true,
		Commands:   []string{"compat"},
	}}, 25*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	pid, present, readErr := readCLIRecordedPIDIfPresent(pidPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if present {
		waitForCLIProcessAbsent(t, pid, time.Second)
	}
	cleanupNeeded = false
}

func TestReadCLIRecordedPIDIfPresent(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		pid, present, err := readCLIRecordedPIDIfPresent(filepath.Join(t.TempDir(), "missing.pid"))
		if err != nil || present || pid != 0 {
			t.Fatalf("pid=%d present=%t err=%v, want absent without error", pid, present, err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "valid.pid")
		if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		pid, present, err := readCLIRecordedPIDIfPresent(path)
		if err != nil || !present || pid != os.Getpid() {
			t.Fatalf("pid=%d present=%t err=%v, want pid %d", pid, present, err, os.Getpid())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.pid")
		if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readCLIRecordedPIDIfPresent(path); err == nil {
			t.Fatal("invalid PID unexpectedly succeeded")
		}
	})
	t.Run("read error", func(t *testing.T) {
		if _, _, err := readCLIRecordedPIDIfPresent(t.TempDir()); err == nil {
			t.Fatal("directory read unexpectedly succeeded")
		}
	})
}

func readCLIRecordedPIDIfPresent(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read recorded PID: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false, fmt.Errorf("invalid recorded PID %q: %w", data, err)
	}
	if pid <= 0 {
		return 0, false, fmt.Errorf("invalid recorded PID %q: must be positive", data)
	}
	return pid, true, nil
}

func waitForCLIRecordedPID(path string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		pid, present, err := readCLIRecordedPIDIfPresent(path)
		if err != nil {
			return 0, err
		}
		if present {
			return pid, nil
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("timed out waiting for PID file %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForCLIProcessAbsent(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := exec.Command("kill", "-0", strconv.Itoa(pid)).Run(); err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant PID %d remained after %s", pid, timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func bestEffortKillCLIRecordedPID(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	_ = exec.Command("kill", "-KILL", strconv.Itoa(pid)).Run()
}

func TestPluginsListAndWhichUseCanonicalIdentity(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	config := testPluginsCommandConfig(t)
	config.storeRoot = home
	statePath := filepath.Join(home, "plugins", "installed.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"version":1,"plugins":[{"name":"compat","canonicalName":"@glade/compat","storageName":"glade__compat","version":"0.1.0","commands":["compat"]}]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"list"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "@glade/compat 0.1.0") {
		t.Fatalf("list did not use canonical identity:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runPluginsForTest(context.Background(), []string{"which", "compat"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("which exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "compat is provided by @glade/compat 0.1.0") {
		t.Fatalf("which did not use canonical identity:\n%s", stdout.String())
	}
}

func TestPluginsDoctorJSON(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	home := t.TempDir()
	config := testPluginsCommandConfig(t)
	config.storeRoot = home
	exe := filepath.Join(home, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["surface"],"summary":"Surface commands."}]}'
  exit 0
fi
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(home, "plugins", "installed.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	state := fmt.Sprintf(`{"version":1,"plugins":[{"name":"compat","version":"0.1.0","commands":["surface"],"executable":%q,"manifest":"/tmp/plugin.json","source":"link:test","linked":true}]}`+"\n", exe)
	if err := os.WriteFile(statePath, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"doctor", "--json"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("doctor exit=%d stderr=%s", code, stderr.String())
	}
	var got struct {
		OK      bool `json:"ok"`
		Plugins []struct {
			Identity string `json:"identity"`
			Version  string `json:"version"`
			OK       bool   `json:"ok"`
			Message  string `json:"message"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if !got.OK || len(got.Plugins) != 1 || !got.Plugins[0].OK || got.Plugins[0].Identity != "compat" || got.Plugins[0].Message != "ok" {
		t.Fatalf("unexpected doctor JSON: %#v", got)
	}
}

func TestPluginsLinkListsPlugin(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	home := t.TempDir()
	config := testPluginsCommandConfig(t)
	config.storeRoot = home
	exe := filepath.Join(home, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}'
  exit 0
fi
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"link", "--exec", exe}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("link exit=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = runPluginsForTest(context.Background(), []string{"list"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "compat 0.1.0 linked") {
		t.Fatalf("unexpected list output:\n%s", stdout.String())
	}
}

func TestPluginsInstallMissingArchiveDoesNotFetchRegistry(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	config.registryURL = "http://127.0.0.1:1/index.json"
	var stdout, stderr bytes.Buffer

	code := runPluginsForTest(context.Background(), []string{"install", "./missing-plugin.tar.gz"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing-plugin.tar.gz") {
		t.Fatalf("stderr missing archive path: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "connect: connection refused") {
		t.Fatalf("missing archive attempted registry fetch: %q", stderr.String())
	}
}

func TestPluginsSearchAndInfoUseRegistry(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@glade/compat","aliases":["compat"],"version":"0.1.0","publisher":"glade","trust":"first-party","summary":"Compatibility fixtures.","commands":["compat","surface"],"docsURL":"https://glade.sh/guide/plugins/compat","assets":[{"os":%q,"arch":%q,"url":"https://example.test/compat.tar.gz","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}]}`, runtime.GOOS, runtime.GOARCH)
	}))
	defer server.Close()
	config.registryURL = server.URL

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"search", "surface"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "@glade/compat 0.1.0 first-party") {
		t.Fatalf("unexpected search output:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runPluginsForTest(context.Background(), []string{"info", "@glade/compat"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("info exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "commands: compat, surface") || !strings.Contains(stdout.String(), "docs: https://glade.sh/guide/plugins/compat") {
		t.Fatalf("unexpected info output:\n%s", stdout.String())
	}
}

func TestPluginsAvailableListsRegistry(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@glade/compat","aliases":["compat"],"version":"0.1.0","publisher":"glade","trust":"first-party","summary":"Compatibility fixtures.","commands":["compat","surface"],"assets":[{"os":%q,"arch":%q,"url":"https://example.test/compat.tar.gz","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]},{"name":"@glade/performance","aliases":["performance"],"version":"0.1.0","publisher":"glade","trust":"first-party","summary":"Performance scans.","commands":["performance"],"assets":[{"os":%q,"arch":%q,"url":"https://example.test/performance.tar.gz","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}]}`, runtime.GOOS, runtime.GOARCH, runtime.GOOS, runtime.GOARCH)
	}))
	defer server.Close()
	config.registryURL = server.URL

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"available"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("available exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "@glade/compat 0.1.0 first-party Compatibility fixtures.") ||
		!strings.Contains(stdout.String(), "@glade/performance 0.1.0 first-party Performance scans.") {
		t.Fatalf("unexpected available output:\n%s", stdout.String())
	}
}

func TestPluginsAvailableDefaultRegistryPreviewError(t *testing.T) {
	config := testPluginsCommandConfig(t)
	restoreHTTPClient := replaceDefaultHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.URL.String(), "https://plugins.glade.sh/index.json"; got != want {
			t.Fatalf("registry URL = %q, want %q", got, want)
		}
		return nil, fmt.Errorf("dial tcp: lookup plugins.glade.sh: no such host")
	})
	defer restoreHTTPClient()

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"available"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("available exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	got := stderr.String()
	if strings.HasPrefix(got, `glade: Get "https://plugins.glade.sh/index.json"`) {
		t.Fatalf("default registry error leads with raw transport detail: %q", got)
	}
	for _, want := range []string{
		"default plugin registry is in preview",
		"GLADE_PLUGIN_REGISTRY_URL",
		"direct archive",
		"glade plugins link",
		`detail: Get "https://plugins.glade.sh/index.json": dial tcp: lookup plugins.glade.sh: no such host`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr missing %q:\n%s", want, got)
		}
	}
}

func TestPluginsInstallDefaultRegistryPreviewError(t *testing.T) {
	config := testPluginsCommandConfig(t)
	restoreHTTPClient := replaceDefaultHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.URL.String(), "https://plugins.glade.sh/index.json"; got != want {
			t.Fatalf("registry URL = %q, want %q", got, want)
		}
		return nil, fmt.Errorf("dial tcp: lookup plugins.glade.sh: no such host")
	})
	defer restoreHTTPClient()

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"install", "@glade/compat"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("install exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	got := stderr.String()
	for _, want := range []string{
		"default plugin registry is in preview",
		"GLADE_PLUGIN_REGISTRY_URL",
		"direct archive",
		"glade plugins link",
		`detail: Get "https://plugins.glade.sh/index.json": dial tcp: lookup plugins.glade.sh: no such host`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr missing %q:\n%s", want, got)
		}
	}
}

func TestPluginsAvailableCustomRegistryKeepsEndpointError(t *testing.T) {
	config := testPluginsCommandConfig(t)
	config.registryURL = "https://registry.example.test/index.json"
	restoreHTTPClient := replaceDefaultHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial tcp: lookup registry.example.test: no such host")
	})
	defer restoreHTTPClient()

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"available"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("available exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	got := stderr.String()
	if strings.Contains(got, "default plugin registry is in preview") {
		t.Fatalf("custom registry error was rewritten as preview message: %q", got)
	}
	for _, want := range []string{
		`Get "https://registry.example.test/index.json"`,
		"lookup registry.example.test: no such host",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr missing %q:\n%s", want, got)
		}
	}
}

func TestPluginsAvailableRejectsArguments(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"available", "quality"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("available exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: glade plugins available") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}

func TestPluginsSearchWithoutQueryListsRegistry(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@glade/compat","version":"0.1.0","trust":"first-party","summary":"Compatibility fixtures.","assets":[{"os":%q,"arch":%q,"url":"https://example.test/compat.tar.gz","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}]}`, runtime.GOOS, runtime.GOARCH)
	}))
	defer server.Close()
	config.registryURL = server.URL

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"search"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "@glade/compat 0.1.0 first-party Compatibility fixtures.") {
		t.Fatalf("unexpected search output:\n%s", stdout.String())
	}
}

func TestPluginsHelpShowsAvailable(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"--help"}, &stdout, &stderr, config)
	if code != 0 {
		t.Fatalf("help exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"available",
		"List plugins available to install.",
		"glade plugins install <plugin-name-or-archive> [--registry <url>] [--sha256 <hash>] [--yes]",
		"glade plugins list [--json]",
		"glade plugins link --exec <plugin-executable>",
		"glade plugins lock [--include-linked]",
		"--progress-json",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("plugins help missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"help", "plugins"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help plugins exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "glade plugins install <plugin-name-or-archive> [--registry <url>] [--sha256 <hash>] [--yes]") {
		t.Fatalf("help plugins does not match detailed plugin help:\n%s", stdout.String())
	}
}

func TestRunTUIValidateNoUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"tui", "--project", ".", "--view", "tests", "--no-ui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("tui exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Glade TUI",
		"project: .",
		"view: tests",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("tui dry run missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunTestUIAliasValidateNoUI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--ui", "--project", ".", "--no-ui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("test ui exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Glade TUI",
		"project: .",
		"view: tests",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("test ui dry run missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunDBUIAliasValidateNoUI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "glade.db")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "--ui", "--db", dbPath, "--project", ".", "--no-ui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("db ui exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Glade TUI",
		"project: .",
		"db: " + dbPath,
		"view: data",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db ui dry run missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunDBUIAliasCarriesImportDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "glade.db")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "--ui", "--db", dbPath, "--project", ".", "--target-org", "devhub", "--object", "Account", "--no-ui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("db ui exit=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"target-org: devhub",
		"object: Account",
		"view: data",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db ui dry run missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunDBUsageMentionsImport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: glade db ui|seed|reset|export|inspect|query|describe|import") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCodeIntelligenceHelpListsProductCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "inspect",
			args: []string{"inspect", "--help"},
			want: []string{"definition", "references", "--kind <kind>", "--full-paths"},
		},
		{
			name: "schema",
			args: []string{"schema", "import", "describe", "--help"},
			want: []string{"--project-cache"},
		},
		{
			name: "refactor",
			args: []string{"refactor", "--help"},
			want: []string{"rename"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tt.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("help exit=%d stderr=%s", code, stderr.String())
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("help missing %q:\n%s", want, stdout.String())
				}
			}
		})
	}
}

func TestPluginsInstallRemoteURLRequiresSHA256(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"install", "https://example.test/plugin.tar.gz"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("exit=%d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "remote plugin archive installs require --sha256") {
		t.Fatalf("stderr missing sha256 message: %q", stderr.String())
	}
}

func TestPluginsInstallRemoteURLProgressFinishesOnTrustError(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	config.ci = true
	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"install", "https://example.test/plugin.tar.gz", "--progress"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"plugins install · Resolving plugin target", "done · plugins install failed"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestPluginsInstallCommunityRegistryInCIStopsBeforeDownload(t *testing.T) {
	t.Parallel()
	config := testPluginsCommandConfig(t)
	config.ci = true
	downloaded := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@acme/quality","version":"1.2.0","trust":"community","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/quality.tar.gz")
		case "/quality.tar.gz":
			downloaded = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	config.registryURL = server.URL + "/index.json"

	var stdout, stderr bytes.Buffer
	code := runPluginsForTest(context.Background(), []string{"install", "@acme/quality"}, &stdout, &stderr, config)
	if code != 1 {
		t.Fatalf("exit=%d, want 1", code)
	}
	if downloaded {
		t.Fatal("community plugin was downloaded before CI trust gate")
	}
	if !strings.Contains(stderr.String(), "rerun with --yes") {
		t.Fatalf("stderr missing CI trust message: %q", stderr.String())
	}
}

func TestScopedPluginCoordinateIsNotArchiveInstallArg(t *testing.T) {
	t.Parallel()
	if isArchiveInstallArg("@glade/compat") {
		t.Fatal("@glade/compat was classified as an archive path")
	}
	if !isArchiveInstallArg("./glade-plugin-compat_0.1.0_darwin_arm64.tar.gz") {
		t.Fatal("local archive path was not classified as an archive")
	}
}

func TestCompatDispatchesToLinkedPlugin(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	home := t.TempDir()
	config := testPluginsCommandConfig(t)
	config.storeRoot = home
	exe := filepath.Join(home, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}'
  exit 0
fi
echo "called plugin with $*"
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runPluginsForTest(context.Background(), []string{"link", "--exec", exe}, &stdout, &stderr, config); code != 0 {
		t.Fatalf("link failed: %s", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code, ok := runInstalledPluginCommandWithStore(context.Background(), []string{"compat", "local-tests", "--help"}, &stdout, &stderr, pluginhost.NewStore(config.storeRoot))
	if !ok {
		t.Fatal("compat command was not dispatched to installed plugin")
	}
	if code != 0 {
		t.Fatalf("dispatch failed code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "called plugin with compat local-tests --help") {
		t.Fatalf("unexpected stdout:\n%s", stdout.String())
	}
}

func TestRunExecDoubleDashStopsFlagParsing(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--", "String value = '--json'; System.assertEquals('--json', value);"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunCommandHelp(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string
		notWant []string
	}{
		{
			name: "test flag help",
			args: []string{"test", "--help"},
			want: []string{"Usage:", "glade test", "glade test serve", "clear-cache", "startup and semantic caches", "--no-cache", "Bypass startup and semantic caches", "--connect", "--daemon", "--debounce", "--watch-backend", "--gc-aggressive", "--cpu-profile", "--mem-profile", "--perf-json"},
		},
		{
			name: "test serve help",
			args: []string{"test", "serve", "--help"},
			want: []string{"Serve flags:", "--no-warm"},
		},
		{
			name: "parse flag help",
			args: []string{"parse", "--help"},
			want: []string{"Usage:", "glade parse <paths...>", "--json", "Examples:"},
		},
		{
			name: "help check",
			args: []string{"help", "check"},
			want: []string{"Usage:", "glade check", "--project <root>", "--no-cache", "Bypass semantic disk and memory caching", "--progress-json", "--cpu-profile", "--mem-profile", "--perf-json", "Examples:"},
		},
		{
			name: "schema subcommand help",
			args: []string{"schema", "load", "--help"},
			want: []string{"Usage:", "glade schema load", "--project <root>", "--progress"},
		},
		{
			name: "schema parent help",
			args: []string{"help", "schema"},
			want: []string{"Usage:", "glade schema load", "glade schema import describe", "import describe"},
		},
		{
			name: "completion help",
			args: []string{"help", "completion"},
			want: []string{"Usage:", "glade completion bash|zsh|fish", "Install:", "source <(glade completion bash)", "Examples:"},
		},
		{
			name: "config help",
			args: []string{"help", "config"},
			want: []string{"Usage:", "glade config show", "validate", "init", "--package-dir <path>", "--feature <name>"},
		},
		{
			name: "toolchain help",
			args: []string{"help", "toolchain"},
			want: []string{"Usage:", "glade toolchain status [--json]", "--json", "Notes:", "GLADE_HOME", "XDG_DATA_HOME", "glade toolchain install --from ."},
		},
		{
			name: "init help",
			args: []string{"init", "--help"},
			want: []string{"Usage:", "glade init", "--force", "--namespace <name>", "--package-dir <path>"},
		},
		{
			name: "playground help",
			args: []string{"playground", "--help"},
			want: []string{"Usage:", "glade playground", "--list-examples", "--example <id>", "--no-db", "--reset-on-start", "GLADE_SERVER_PUBLIC=1"},
		},
		{
			name: "server help",
			args: []string{"help", "server"},
			want: []string{"Usage:", "glade server", "--addr <host:port>", "GLADE_SERVER_PUBLIC=1"},
		},
		{
			name:    "org help",
			args:    []string{"help", "org"},
			want:    []string{"Usage:", "glade org create", "glade org list", "glade org status", "glade org auth", "glade org create refinement-local", "--db .glade/orgs/refinement-local.sqlite", "--addr 127.0.0.1:17911", "GLADE_SERVER_PUBLIC=1"},
			notWant: []string{"glade org create refinement-local --project . --db .glade/orgs/refinement-local.sqlite --addr 127.0.0.1:17911"},
		},
		{
			name: "help test",
			args: []string{"help", "test"},
			want: []string{"Usage:", "glade test", "glade test serve", "clear-cache", "--no-cache", "--connect", "--daemon", "--ui"},
		},
		{
			name: "help tui",
			args: []string{"help", "tui"},
			want: []string{"Usage:", "glade tui [--project <root>] [--env <name>|--db <path>] [--view <project|tests|data|plugins>] [--target-org <alias>] [--object <Object>]", "--env <name>", "--target-org <alias>", "--object <Object>", "--no-ui", "glade tui --project . --view tests", "glade tui --project . --view data"},
		},
		{
			name: "help dev",
			args: []string{"help", "dev"},
			want: []string{"Usage:", "glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]", "glade dev lwc [--project <root>] [--db <path>] [--port <port>|--addr <host:port>] [--ready-file <path>]", "Preview features:", "Visualforce local rendering", "LWC local shell", "Subcommands:", "vf", "lwc"},
		},
		{
			name: "help db",
			args: []string{"help", "db"},
			want: []string{"Usage:", "glade db --ui [--project <root>] [--env <name>|--db <path>]", "glade db import sf [--target-org <alias>] [--project <root>] [--env <name>|--db <path>]", "glade db import sf [--target-org <alias>] --list-objects", "glade db query [--project <root>] [--env <name>|--db <path>] --json [--limit <n>] [--query-all] <soql>", "glade db describe [--project <root>] [--env <name>|--db <path>] --json [ObjectName]", "--env <name>", "--db <path>", "--target-org <alias>", "--object <Object>", "--fields <list>", "--limit <n>", "--query-all", "--ui", "GLADE_SERVER_PUBLIC=1", "glade db inspect --project .", "glade db import sf --target-org devhub --project . --object Account", "glade db query --project . --json \"SELECT Id, Name FROM FileRow__c\""},
		},
		{
			name: "help profile",
			args: []string{"help", "profile"},
			want: []string{"Usage:", "glade profile analyze <trace.json> [--json] [--format text|markdown|pprof]", "--format <mode>", "pprof"},
		},
		{
			name: "help package",
			args: []string{"help", "package"},
			want: []string{"Usage:", "glade package capture --target-org <alias>", "capture", "@glade/orgpackage", "glade package capture --target-org packaging"},
		},
		{
			name: "help exit codes",
			args: []string{"help", "exit-codes"},
			want: []string{"Exit codes", "0  Command completed", "1  Command failed", "2  Command was not understood"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tt.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			got := stdout.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("stdout missing %q:\n%s", want, got)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(got, notWant) {
					t.Fatalf("stdout unexpectedly contains %q:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestUsageErrorsMatchHelpSurface(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "parse",
			args: []string{"parse"},
			want: []string{"usage: glade parse <paths...> [--json] [--progress|--progress-json|--no-progress]"},
		},
		{
			name: "inspect",
			args: []string{"inspect"},
			want: []string{"usage: glade inspect symbols [--project <root>] [--kind <kind>] [--full-paths] [--json]"},
		},
		{
			name: "schema",
			args: []string{"schema"},
			want: []string{"glade schema load [--project <root>] [--json] [--progress|--progress-json|--no-progress]", "--project-cache <root>"},
		},
		{
			name: "exec",
			args: []string{"exec"},
			want: []string{"usage: glade exec [--project <root>] [--db <path>] [--dry-run] [--json] [--trace <path>] [--debug-log <path>] [--limit-mode <mode>]"},
		},
		{
			name: "profile",
			args: []string{"profile", "analyze"},
			want: []string{"usage: glade profile analyze <trace.json> [--json] [--format text|markdown|pprof]"},
		},
		{
			name: "package",
			args: []string{"package"},
			want: []string{"usage: glade package build|info|validate|diff|capture ..."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tt.args, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("exit code = 0, want usage failure; stdout=%q", stdout.String())
			}
			got := stderr.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("stderr missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestRunTopLevelHelpIsWorkflowDoorway(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Glade — local Apex runtime",
		"Start here:",
		"glade doctor",
		"glade check",
		"glade test changed --since origin/main",
		"glade tui --project .",
		"Workflows:",
		"More:",
		"glade help workflows",
		"glade examples",
		"glade support",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("top-level help missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Commands\n  version") {
		t.Fatalf("top-level help still leads with full command inventory:\n%s", got)
	}
}

func TestRunDiscoveryCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "examples",
			args: []string{"examples"},
			want: []string{"Built-in examples", "refinement-service", "Try:", "glade playground --example refinement-service --open"},
		},
		{
			name: "example show",
			args: []string{"examples", "show", "refinement-service"},
			want: []string{"refinement-service", "Refinement Service", "Try:", "glade playground --example refinement-service --open"},
		},
		{
			name: "explain",
			args: []string{"explain", "GLADESEMA002"},
			want: []string{"GLADESEMA002", "unknown type", "Try:", "glade schema load --project ."},
		},
		{
			name: "support",
			args: []string{"support"},
			want: []string{"Glade support", "Diagnostics", "glade doctor", "glade check --json"},
		},
		{
			name: "workflows help",
			args: []string{"help", "workflows"},
			want: []string{"Workflows", "Local check loop", "glade test changed --since origin/main", ".glade/logs/latest.apexlog"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tt.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
				}
			}
		})
	}
}

func TestRunCompletionBash(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"completion", "bash"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"_glade_completion",
		"complete -F _glade_completion glade",
		"version update doctor toolchain config init parse inspect schema refactor check exec",
		"--project",
		"--class",
		"--method",
		"--class-file",
		"--package-dir",
		"--progress-json",
		"config show validate init",
		"test changed failed serve daemon clear-cache",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("bash completion missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "compat") {
		t.Fatalf("completion mentions maintenance command compat:\n%s", got)
	}
}

func TestRunCompletionFish(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"completion", "fish"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"complete -c glade",
		"-a 'test'",
		"-l project",
		"-l class",
		"-l method",
		"-l class-file",
		"-l progress-json",
		"-a 'changed failed serve daemon clear-cache'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fish completion missing %q:\n%s", want, got)
		}
	}
}

func TestRunCompletionRejectsUnknownShell(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"completion", "powershell"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unsupported completion shell "powershell"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunTopLevelHelpAlignment(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"help", "commands"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"package",
		"Build, inspect, validate, diff, and capture managed package artifacts.",
		"dap",
		"Run the Debug Adapter Protocol server over stdio.",
		"server",
		"Start the local Salesforce-compatible API baseline.",
		"playground",
		"Start the local Apex playground web UI.",
		"db",
		"Seed, import, reset, export, inspect, query, and describe a persistent local database.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing aligned line %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{
		"\t  package",
		"\n            Start the local Apex playground web UI.",
		"\n                Report generated stub behavioral contract policy.",
	} {
		if strings.Contains(got, bad) {
			t.Fatalf("stdout contains bad indentation %q:\n%s", bad, got)
		}
	}
}

type editorCommandTestEnv map[string]string

func (env editorCommandTestEnv) getenv(name string) string {
	return env[name]
}

func testEditorCommandDeps(t *testing.T, deps editorCommandDeps) (editorCommandDeps, editorCommandTestEnv) {
	t.Helper()
	env := make(editorCommandTestEnv)
	userShareDir := filepath.Join(t.TempDir(), "user-share")
	deps.getenv = env.getenv
	deps.userShareDir = func() string { return userShareDir }
	deps.executable = func() (string, error) { return "", os.ErrNotExist }
	deps.getwd = func() (string, error) { return "", os.ErrNotExist }
	return deps, env
}

func TestRunEditorInstallVSCodeUsesVSIX(t *testing.T) {
	t.Parallel()
	vsix := filepath.Join(t.TempDir(), "vscode-glade.vsix")
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ranName string
	var ranArgs []string
	deps, _ := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranName = name
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	})

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"install", "vscode", "--vsix", vsix, "--force"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	if ranName != "/usr/local/bin/code" {
		t.Fatalf("ran name = %q", ranName)
	}
	wantArgs := []string{"--install-extension", vsix, "--force"}
	if strings.Join(ranArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("ran args = %#v, want %#v", ranArgs, wantArgs)
	}
	if !strings.Contains(stdout.String(), "installed vscode extension") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunEditorInstallVSCodeSuppressesSuccessfulEditorOutput(t *testing.T) {
	t.Parallel()
	vsix := filepath.Join(t.TempDir(), "vscode-glade.vsix")
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps, _ := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Installing extensions...\nExtension 'vscode-glade-0.0.1.vsix' was successfully installed.\n(node:5882) [DEP0169] DeprecationWarning: `url.parse()` behavior is not standardized.\n"), nil
		},
	})

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"install", "vscode", "--vsix", vsix, "--force"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("installed vscode extension: %s\n", vsix)
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunEditorDoctorVSCodeReportsPaths(t *testing.T) {
	t.Parallel()
	gladePath := filepath.Join(t.TempDir(), "bin", "glade")
	deps, _ := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			switch name {
			case "code":
				return "/usr/local/bin/code", nil
			case "glade":
				return gladePath, nil
			default:
				return "", os.ErrNotExist
			}
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	})

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"doctor", "vscode"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{
		"editor: code (/usr/local/bin/code)",
		"glade: " + gladePath,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunEditorDoctorVSCodeJSONReportsBundledVSIX(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	vsix := filepath.Join(home, "editor", "vscode-glade.vsix")
	if err := os.MkdirAll(filepath.Dir(vsix), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps, env := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			switch name {
			case "code":
				return "/usr/local/bin/code", nil
			case "glade":
				return filepath.Join(home, ".local", "bin", "glade"), nil
			default:
				return "", os.ErrNotExist
			}
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	})
	env["GLADE_HOME"] = home

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"doctor", "vscode", "--json"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Target string `json:"target"`
		Editor struct {
			Command string `json:"command"`
			Path    string `json:"path"`
			OK      bool   `json:"ok"`
		} `json:"editor"`
		Glade struct {
			Path string `json:"path"`
			OK   bool   `json:"ok"`
		} `json:"glade"`
		BundledVSIX struct {
			Path   string `json:"path"`
			Exists bool   `json:"exists"`
		} `json:"bundledVsix"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if got.Target != "vscode" || got.Editor.Command != "code" || !got.Editor.OK || !got.Glade.OK {
		t.Fatalf("doctor json = %#v", got)
	}
	if got.BundledVSIX.Path != vsix || !got.BundledVSIX.Exists {
		t.Fatalf("bundled vsix = %#v, want %q", got.BundledVSIX, vsix)
	}
}

func TestRunEditorInstallVSCodeUsesBundledVSIXWhenPathOmitted(t *testing.T) {
	t.Parallel()
	vsix := filepath.Join(t.TempDir(), "vscode-glade.vsix")
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ranArgs []string
	deps, env := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	})
	env["GLADE_VSCODE_VSIX"] = vsix

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"install", "vscode", "--force"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--install-extension", vsix, "--force"}
	if strings.Join(ranArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("ran args = %#v, want %#v", ranArgs, wantArgs)
	}
}

func TestRunEditorInstallVSCodeUsesGladeHomeBundledVSIX(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	vsix := filepath.Join(home, "editor", "vscode-glade.vsix")
	if err := os.MkdirAll(filepath.Dir(vsix), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ranArgs []string
	deps, env := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	})
	env["GLADE_HOME"] = home
	env["GLADE_VSCODE_VSIX"] = ""

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"install", "vscode", "--force"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--install-extension", vsix, "--force"}
	if strings.Join(ranArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("ran args = %#v, want %#v", ranArgs, wantArgs)
	}
}

func TestRunEditorInstallVSCodePrefersSourceCheckoutOverUserShareVSIX(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/glade-sh/glade\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	extensionRoot := filepath.Join(root, "contrib", "vscode-glade")
	if err := os.MkdirAll(filepath.Join(extensionRoot, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extensionRoot, "package.json"), []byte(`{"name":"vscode-glade"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceVSIX := filepath.Join(extensionRoot, "dist", "vscode-glade-0.0.2.vsix")
	if err := os.WriteFile(sourceVSIX, []byte("source-vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(sourceVSIX); err == nil {
		sourceVSIX = resolved
	}
	xdg := filepath.Join(root, "xdg")
	userShareVSIX := filepath.Join(xdg, "glade", "editor", "vscode-glade.vsix")
	if err := os.MkdirAll(filepath.Dir(userShareVSIX), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userShareVSIX, []byte("stale-user-share-vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ranArgs []string
	deps, env := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	})
	env["GLADE_VSCODE_VSIX"] = ""
	env["GLADE_HOME"] = ""
	deps.userShareDir = func() string { return filepath.Join(xdg, "glade") }
	deps.getwd = func() (string, error) { return filepath.EvalSymlinks(extensionRoot) }

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"install", "vscode"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--install-extension", sourceVSIX}
	if strings.Join(ranArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("ran args = %#v, want %#v", ranArgs, wantArgs)
	}
}

func TestRunEditorInstallVSCodeFindsSourceCheckoutVSIX(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/glade-sh/glade\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	extensionRoot := filepath.Join(root, "contrib", "vscode-glade")
	if err := os.MkdirAll(filepath.Join(extensionRoot, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extensionRoot, "package.json"), []byte(`{"name":"vscode-glade"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	vsix := filepath.Join(extensionRoot, "dist", "vscode-glade-0.0.1.vsix")
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(vsix); err == nil {
		vsix = resolved
	}
	var ranArgs []string
	deps, env := testEditorCommandDeps(t, editorCommandDeps{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	})
	env["GLADE_VSCODE_VSIX"] = ""
	env["GLADE_HOME"] = filepath.Join(root, "missing-glade-home")
	deps.userShareDir = func() string { return filepath.Join(root, "xdg", "glade") }
	deps.getwd = func() (string, error) {
		return filepath.EvalSymlinks(filepath.Join(root, "contrib", "vscode-glade"))
	}

	var stdout bytes.Buffer
	if err := runEditorWithCommandDeps(context.Background(), []string{"install", "vscode"}, &stdout, deps); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--install-extension", vsix}
	if strings.Join(ranArgs, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("ran args = %#v, want %#v", ranArgs, wantArgs)
	}
}

func TestRunDevStatus(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"Glade dev",
		"Local feedback loop",
		"Project       " + filepath.Base(root),
		"Package dirs  1",
		"Apex classes  2",
		"Apex tests    1",
		"Metadata      loaded",
		"On change:",
		"Next:",
		"glade dev test --project " + root,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func TestRunDevTestWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	runsDir := filepath.Join(root, ".glade", "runs")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "test", "--project", root, "--out", runsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"1 selected", "MathUtilTest.adds", "1 passed", "Report: "} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
	latestPath := filepath.Join(runsDir, "latest.json")
	if _, err := os.Stat(latestPath); err != nil {
		t.Fatalf("latest pointer missing: %v", err)
	}
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"summaryPath"`) || !strings.Contains(string(data), `"resultsPath"`) {
		t.Fatalf("latest pointer missing paths: %s", data)
	}
}

func TestRunDevWatchOnceUsesHumanOutput(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	runsDir := filepath.Join(root, ".glade", "runs")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "watch", "--project", root, "--watch-once", "--out", runsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"Glade dev", "Watching project for Apex changes.", "On change:", "Press Ctrl-C to stop.", "1 passed", "Report: "} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"event":"watch.started"`) {
		t.Fatalf("dev watch leaked raw NDJSON:\n%s", got)
	}
}

func TestRunReportCommands(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	runsDir := filepath.Join(root, ".glade", "runs")

	var devOut, devErr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "test", "--project", root, "--out", runsDir}, &devOut, &devErr)
	if code != 0 {
		t.Fatalf("dev test exit code = %d, stderr=%q stdout=%q", code, devErr.String(), devOut.String())
	}

	var listOut, listErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "list", "--runs-dir", runsDir}, &listOut, &listErr)
	if code != 0 {
		t.Fatalf("report list exit code = %d, stderr=%q", code, listErr.String())
	}
	if got := listOut.String(); !strings.Contains(got, "202") || !strings.Contains(got, "latest") {
		t.Fatalf("report list output = %q", got)
	}

	var showOut, showErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "show", "latest", "--runs-dir", runsDir}, &showOut, &showErr)
	if code != 0 {
		t.Fatalf("report show exit code = %d, stderr=%q", code, showErr.String())
	}
	if got := showOut.String(); !strings.Contains(got, "1 passed") {
		t.Fatalf("report show output = %q", got)
	}
	if got := showOut.String(); !strings.Contains(got, "Glade report") || !strings.Contains(got, "Artifacts:") || !strings.Contains(got, "glade report export latest") {
		t.Fatalf("report show output missing report shell:\n%s", got)
	}

	zipPath := filepath.Join(root, "report.zip")
	var exportOut, exportErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "export", "latest", "--runs-dir", runsDir, "--output", zipPath}, &exportOut, &exportErr)
	if code != 0 {
		t.Fatalf("report export exit code = %d, stderr=%q", code, exportErr.String())
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("zip missing: %v", err)
	}

	var cleanOut, cleanErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "clean", "--runs-dir", runsDir, "--keep", "0"}, &cleanOut, &cleanErr)
	if code != 0 {
		t.Fatalf("report clean exit code = %d, stderr=%q", code, cleanErr.String())
	}
	if got := cleanOut.String(); !strings.Contains(got, "Removed 1 run.") {
		t.Fatalf("clean output = %q", got)
	}
}

func TestRunReportJSONGitHubAndHTML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/FailingTest.cls"), `
@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(2, 1);
  }
}
`)
	runsDir := filepath.Join(root, ".glade", "runs")
	var devOut, devErr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "test", "--project", root, "--out", runsDir}, &devOut, &devErr)
	if code != 1 {
		t.Fatalf("dev test exit code = %d, want 1; stderr=%q stdout=%q", code, devErr.String(), devOut.String())
	}

	var jsonOut, jsonErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "show", "latest", "--runs-dir", runsDir, "--json"}, &jsonOut, &jsonErr)
	if code != 0 {
		t.Fatalf("report json exit code = %d, stderr=%q", code, jsonErr.String())
	}
	if !json.Valid(jsonOut.Bytes()) || !strings.Contains(jsonOut.String(), `"latest"`) || !strings.Contains(jsonOut.String(), `"result"`) {
		t.Fatalf("report json output = %q", jsonOut.String())
	}

	var ghOut, ghErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "github", "latest", "--runs-dir", runsDir}, &ghOut, &ghErr)
	if code != 0 {
		t.Fatalf("report github exit code = %d, stderr=%q", code, ghErr.String())
	}
	if !strings.Contains(ghOut.String(), "::error") || !strings.Contains(ghOut.String(), "FailingTest") {
		t.Fatalf("github annotations = %q", ghOut.String())
	}

	htmlPath := filepath.Join(root, "report.html")
	var htmlOut, htmlErr bytes.Buffer
	code = Run(context.Background(), []string{"report", "export", "latest", "--runs-dir", runsDir, "--output", htmlPath, "--format", "html"}, &htmlOut, &htmlErr)
	if code != 0 {
		t.Fatalf("report html exit code = %d, stderr=%q", code, htmlErr.String())
	}
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlData), "<!doctype html>") || !strings.Contains(string(htmlData), "FailingTest.fails") {
		t.Fatalf("html report = %s", htmlData)
	}
}

func TestRunDevTestFailedRerunsLatestFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/FailingTest.cls"), `
@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(2, 1);
  }
}
`)
	runsDir := filepath.Join(root, ".glade", "runs")
	var firstOut, firstErr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "test", "--project", root, "--out", runsDir}, &firstOut, &firstErr)
	if code != 1 {
		t.Fatalf("first run exit code = %d, want 1; stderr=%q stdout=%q", code, firstErr.String(), firstOut.String())
	}

	var rerunOut, rerunErr bytes.Buffer
	code = Run(context.Background(), []string{"dev", "test", "--project", root, "--out", runsDir, "--failed"}, &rerunOut, &rerunErr)
	if code != 1 {
		t.Fatalf("rerun exit code = %d, want 1; stderr=%q stdout=%q", code, rerunErr.String(), rerunOut.String())
	}
	got := rerunOut.String()
	if !strings.Contains(got, "1 selected") || !strings.Contains(got, "FailingTest.fails") {
		t.Fatalf("rerun output = %q", got)
	}
}

func TestLatestFailedFilterIncludesAllFailures(t *testing.T) {
	runsDir := t.TempDir()
	runDir := filepath.Join(runsDir, "20260611-120000")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resultsPath := filepath.Join(runDir, "results.json")
	writeTestFile(t, resultsPath, `{
  "suites": [{
    "name": "Failures",
    "cases": [
      {"className": "FirstTest", "methodName": "fails", "status": "fail"},
      {"className": "SecondTest", "methodName": "breaks", "status": "runtime_error"},
      {"className": "PassingTest", "methodName": "passes", "status": "pass"}
    ]
  }]
}`)
	writeTestFile(t, filepath.Join(runsDir, "latest.json"), `{
  "runId": "20260611-120000",
  "runDir": "`+filepath.ToSlash(runDir)+`",
  "resultsPath": "`+filepath.ToSlash(resultsPath)+`"
}`)

	got, err := latestFailedFilter(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "FirstTest.fails,SecondTest.breaks" {
		t.Fatalf("filter = %q", got)
	}
}

func TestOrgForProjectAccountAddressFieldsExposeFirstComponent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	org, err := orgForProject(root)
	if err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
for (String fieldName : fields.keySet()) {
    Schema.DescribeFieldResult describe = fields.get(fieldName).getDescribe();
    if (describe.getType() != Schema.DisplayType.Address) {
        continue;
    }
    String prefix = fieldName.removeEnd('address');
    String firstComponent = prefix + 'street';
    System.assertNotEquals(null, fields.get(firstComponent), fieldName + ' missing ' + firstComponent);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRunPackageBuildUsesRequestedNamespace(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"PKG","sourceApiVersion":"61.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Address.cls"), `
global class Address {
  global String street;
  public String internalCode;
  global String format() { return street; }
  public String helper() { return internalCode; }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Hidden.cls"), `
public class Hidden {
  global String shouldNotExport;
}
`)
	output := filepath.Join(root, "out", "pkg.glade-package.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "build", "--project", root, "--namespace", "pkg", "--version", "test-version", "--output", output, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		Namespace        string `json:"namespace"`
		Version          string `json:"version"`
		SourceAPIVersion string `json:"sourceApiVersion"`
		SourceHash       string `json:"sourceHash"`
		ApexTypes        []struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Members   []struct {
				Name string `json:"name"`
			} `json:"members"`
		} `json:"apexTypes"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Namespace != "pkg" || artifact.Version != "test-version" || artifact.SourceAPIVersion != "61.0" || artifact.SourceHash == "" {
		t.Fatalf("artifact metadata = %#v", artifact)
	}
	if len(artifact.ApexTypes) != 1 || artifact.ApexTypes[0].Name != "Address" || artifact.ApexTypes[0].Namespace != "pkg" {
		t.Fatalf("apex types = %#v", artifact.ApexTypes)
	}
	memberNames := make([]string, 0, len(artifact.ApexTypes[0].Members))
	for _, member := range artifact.ApexTypes[0].Members {
		memberNames = append(memberNames, member.Name)
	}
	if strings.Contains(strings.Join(memberNames, ","), "internalCode") || strings.Contains(strings.Join(memberNames, ","), "helper") {
		t.Fatalf("exported non-global members: %#v", artifact.ApexTypes[0].Members)
	}
	if !strings.Contains(strings.Join(memberNames, ","), "street") || !strings.Contains(strings.Join(memberNames, ","), "format") {
		t.Fatalf("missing global members: %#v", artifact.ApexTypes[0].Members)
	}
	var buildOut struct {
		Namespace string `json:"namespace"`
		Version   string `json:"version"`
	}
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "package build", &buildOut)
	if env.Status != "passed" || env.ExitCode != 0 || buildOut.Namespace != "pkg" || buildOut.Version != "test-version" {
		t.Fatalf("unexpected package build envelope: env=%#v data=%#v\n%s", env, buildOut, stdout.String())
	}
}

func TestRunPackageRichArtifactWorkflow(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"PKG","sourceApiVersion":"61.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Address.cls"), `
global class Address {
  global String street;
  global String format() { return street; }
}
`)
	first := filepath.Join(root, "out", "first.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "build", "--project", root, "--namespace", "pkg", "--version", "1.0", "--output", first, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("build exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "package") || !strings.Contains(stderr.String(), "done") {
		t.Fatalf("progress stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"package", "info", first, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"namespace": "pkg"`) || !strings.Contains(stdout.String(), `"apexTypes": 1`) {
		t.Fatalf("info stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"package", "validate", first}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "package artifact ok") {
		t.Fatalf("validate stdout = %q", stdout.String())
	}

	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Zed.cls"), `
global class Zed {
  global void touch() {}
}
`)
	second := filepath.Join(root, "out", "second.json")
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"package", "build", "--project", root, "--namespace", "pkg", "--version", "2.0", "--output", second, "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second build exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"package", "diff", first, second, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("diff exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"addedTypes": 1`) || !strings.Contains(stdout.String(), `"changed": true`) {
		t.Fatalf("diff stdout = %q", stdout.String())
	}
}

func TestRunPackageInfoPrintsCaptureProvenance(t *testing.T) {
	root := t.TempDir()
	artifact := filepath.Join(root, "pkg.glade-package.json")
	writeTestFile(t, artifact, `{
  "schemaVersion": 2,
  "namespace": "pkg",
  "version": "1.2.3.4",
  "sourceHash": "abc",
  "builtAt": "2026-06-19T12:00:00Z",
  "capture": {
    "source": "org",
    "orgId": "00Dxx0000000001",
    "targetOrg": "packaging",
    "packageId": "033xx0000000001"
  }
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "info", artifact}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info exit code = %d, stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"captureSource: org", "captureTargetOrg: packaging", "captureOrgId: 00Dxx0000000001", "capturePackageId: 033xx0000000001"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("info missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunPackageCaptureBridgeGuidesWhenPluginMissing(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "capture", "--target-org", "packaging", "--namespace", "pkg", "--output", "pkg.glade-package.json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected missing plugin failure stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "glade plugins install @glade/orgpackage") || !strings.Contains(stderr.String(), "glade orgpackage capture") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCheckReportsMalformedMetadataAsDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/classes/A.cls"), `public class A {}`)
	writeTestFile(t, filepath.Join(root, "force-app/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"code": "GLADESCHEMA001"`) || !strings.Contains(stdout.String(), `"types": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCheckRejectsGenerationChangedAfterIndexBeforeSemanticAnalysis(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(root, source string)
	}{
		{
			name: "Apex source",
			mutate: func(_ string, source string) {
				writeTestFile(t, source, "public class Generation { public static Integer value() { return 2; } }\n")
			},
		},
		{
			name: "Apex metadata sidecar",
			mutate: func(_ string, source string) {
				writeTestFile(t, source+"-meta.xml", "<ApexClass><status>Active</status></ApexClass>\n")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			source := filepath.Join(root, "force-app/main/default/classes/Generation.cls")
			writeTestFile(t, source, "public class Generation { public static Integer value() { return 1; } }\n")
			restore := setCheckGenerationValidationHookForTesting(func() { test.mutate(root, source) })
			t.Cleanup(restore)

			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
			}
			if !strings.Contains(stdout.String(), `"code": "GLADEGEN001"`) || !strings.Contains(stdout.String(), "source snapshot mismatch") {
				t.Fatalf("stdout = %q", stdout.String())
			}
		})
	}
}

func TestRunCheckSemanticDiskCachePreservesExactResult(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restoreDisk := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	checkSemanticResults.Reset()
	t.Cleanup(checkSemanticResults.Reset)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CachedCheck.cls"), `public class CachedCheck { public static MissingCachedCheck value; }`)

	run := func(perfPath string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--perf-json", perfPath}, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	firstPerf := filepath.Join(t.TempDir(), "first.json")
	firstCode, firstOutput, firstErr := run(firstPerf)
	if firstCode != 1 {
		t.Fatalf("first code = %d stderr=%q stdout=%q", firstCode, firstErr, firstOutput)
	}
	first := readPerfJSONObject(t, firstPerf)
	firstCache := requireJSONObject(t, first, "semanticCache")
	if firstCache["source"] != "build" {
		t.Fatalf("first semantic cache = %#v", firstCache)
	}
	optionsFingerprint := requireJSONObject(t, firstCache, "identity")["optionsFingerprint"].(string)
	cachePath := filepath.Join(root, ".glade", "semantic", "result-"+optionsFingerprint+".json")
	if info, err := os.Stat(cachePath); err != nil {
		t.Fatalf("semantic cache file stat = %v", err)
	} else if info.Size() == 0 {
		t.Fatal("semantic cache file is empty")
	}

	checkSemanticResults.Reset()
	secondPerf := filepath.Join(t.TempDir(), "second.json")
	secondCode, secondOutput, secondErr := run(secondPerf)
	if secondCode != firstCode || secondOutput != firstOutput {
		t.Fatalf("disk-hit result changed\nfirst code/output: %d %q\nsecond code/output: %d %q\nstderr=%q", firstCode, firstOutput, secondCode, secondOutput, secondErr)
	}
	second := readPerfJSONObject(t, secondPerf)
	if cache := requireJSONObject(t, second, "semanticCache"); cache["source"] != "disk" {
		t.Fatalf("second semantic cache = %#v; first=%#v", cache, firstCache)
	}
	if semaPerf := requireJSONObject(t, second, "semaPerf"); requireJSONNumber(t, semaPerf, "totalNs") != 0 {
		t.Fatalf("disk hit performed semantic analysis: %#v", semaPerf)
	}
}

func TestRunCheckNoCacheDoesNotReadWriteOrRetainSemanticResult(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restoreDisk := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	checkSemanticResults.Reset()
	t.Cleanup(checkSemanticResults.Reset)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/NoCachedCheck.cls"), `public class NoCachedCheck {}`)

	for i := 0; i < 2; i++ {
		perfPath := filepath.Join(t.TempDir(), fmt.Sprintf("no-cache-%d.json", i))
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--no-cache", "--perf-json", perfPath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("run %d code=%d stderr=%q stdout=%q", i, code, stderr.String(), stdout.String())
		}
		perf := readPerfJSONObject(t, perfPath)
		if cache := requireJSONObject(t, perf, "semanticCache"); cache["source"] != "build" {
			t.Fatalf("run %d semantic cache = %#v", i, cache)
		}
	}
	if stats := checkSemanticResults.Stats(); stats.Entries != 0 || stats.RetainedBytes != 0 {
		t.Fatalf("no-cache retained state = %#v", stats)
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "semantic")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("semantic cache directory stat = %v, want not exist", err)
	}
}

func TestRunCheckRejectsMutationAfterSemanticIdentityBeforePublication(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restoreDisk := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	checkSemanticResults.Reset()
	t.Cleanup(checkSemanticResults.Reset)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	source := filepath.Join(root, "force-app/main/default/classes/IdentityRace.cls")
	writeTestFile(t, source, `public class IdentityRace { public static Integer value() { return 1; } }`)
	restore := setCheckSemanticIdentityHookForTesting(func() {
		writeTestFile(t, source, `public class IdentityRace { public static Integer value() { return 2; } }`)
	})
	t.Cleanup(restore)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stdout.String(), `"code": "GLADEGEN001"`) || !strings.Contains(stdout.String(), "source snapshot mismatch") {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stats := checkSemanticResults.Stats(); stats.Entries != 0 {
		t.Fatalf("mutated generation was published: %#v", stats)
	}
}

func TestRunCheckDoesNotReportPerformanceDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/QueryInLoop.cls"), `
public class QueryInLoop {
  public static void run(List<Account> accounts) {
    for (Account account : accounts) {
      List<Contact> contacts = [SELECT Id FROM Contact WHERE AccountId = :account.Id];
      System.debug(contacts.size());
    }
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "GLADEPERF") {
		t.Fatalf("check JSON reported performance diagnostics:\n%s", stdout.String())
	}
}

func TestNewCheckSemaPerfIncludesSourceArenaCounters(t *testing.T) {
	got := newCheckSemaPerf(sema.PerfCounters{
		Enabled:                  true,
		WorkspacePhysicalReads:   1,
		WorkspacePhysicalSources: 1,
		WorkspaceLogicalViews:    2,
		WorkspaceOccurrences:     3,
		SourceArenaHits:          17,
		SourceArenaMisses:        0,
		SourceArenaFallbackReads: 0,
	})
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"workspacePhysicalReads":1`, `"workspacePhysicalSources":1`,
		`"workspaceLogicalViews":2`, `"workspaceOccurrences":3`,
		`"sourceArenaHits":17`, `"sourceArenaMisses":0`, `"sourceArenaFallbackReads":0`,
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("perf JSON missing %s: %s", want, data)
		}
	}
}

func TestRunCheckPerfJSONReportsSinglePhysicalSourceRead(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Arena.cls"), `public class Arena { public void run() { System.debug('arena'); } }`)
	perfPath := filepath.Join(t.TempDir(), "check-perf.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--perf-json", perfPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(perfPath)
	if err != nil {
		t.Fatal(err)
	}
	var perf struct {
		SemaPerf struct {
			WorkspacePhysicalReads   uint64 `json:"workspacePhysicalReads"`
			WorkspacePhysicalSources uint64 `json:"workspacePhysicalSources"`
			WorkspaceOccurrences     uint64 `json:"workspaceOccurrences"`
			SourceArenaHits          uint64 `json:"sourceArenaHits"`
			SourceArenaMisses        uint64 `json:"sourceArenaMisses"`
			SourceArenaFallbackReads uint64 `json:"sourceArenaFallbackReads"`
		} `json:"semaPerf"`
	}
	if err := json.Unmarshal(data, &perf); err != nil {
		t.Fatal(err)
	}
	got := perf.SemaPerf
	if got.WorkspacePhysicalReads != 1 || got.WorkspacePhysicalSources != 1 || got.WorkspaceOccurrences != 1 {
		t.Fatalf("workspace source counts = %#v", got)
	}
	if got.SourceArenaHits == 0 || got.SourceArenaMisses != 0 || got.SourceArenaFallbackReads != 0 {
		t.Fatalf("source arena counts = %#v", got)
	}
}

func TestRunCheckFormatsForCI(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Broken.cls"), "public class Broken { public MissingType run() { return null; } }")

	sarifPath := filepath.Join(root, "check.sarif")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--format", "sarif", "--output", sarifPath, "--no-progress"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("sarif exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("sarif stdout = %q, want empty when --output is used", stdout.String())
	}
	data, err := os.ReadFile(sarifPath)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) || !strings.Contains(string(data), `"version": "2.1.0"`) || !strings.Contains(string(data), "GLADESEMA002") {
		t.Fatalf("sarif = %s", data)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"check", "--project", root, "--format", "github", "--no-progress"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("github exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "::error") || !strings.Contains(stdout.String(), "GLADESEMA002") {
		t.Fatalf("github stdout = %q", stdout.String())
	}
}

func TestRunParseJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Hello.cls")
	if err := os.WriteFile(path, []byte("public class Hello {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"parse", path, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got apexast.Result
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "parse", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected parse envelope: %#v\n%s", env, stdout.String())
	}
	if len(got.Files) != 1 || len(got.Files[0].Declarations) != 1 || got.Files[0].Declarations[0].Name != "Hello" {
		t.Fatalf("parse data did not include parsed declaration: %#v\n%s", got, stdout.String())
	}
}

func TestRunParseProgressWritesToStderr(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "Hello.cls")
	second := filepath.Join(dir, "World.cls")
	writeTestFile(t, first, "public class Hello {}")
	writeTestFile(t, second, "public class World {}")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"parse", dir, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "parse") || !strings.Contains(stderr.String(), "done") {
		t.Fatalf("progress stderr = %q", stderr.String())
	}
}

func TestRunInspectSymbolsJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg","sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")
	writeTestFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "symbols", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name": "Hello"`) || !strings.Contains(stdout.String(), `"name": "pkg__Thing__c"`) {
		t.Fatalf("stdout did not include symbols and schema: %q", stdout.String())
	}
}

func TestRunInspectSymbolsTextIsScannableAndRelative(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg","sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")
	writeTestFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "symbols", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"Project symbols", "Summary:", "Apex types", "Objects", "Symbols:", "class", "Hello", "force-app/main/classes/Hello.cls", "object", "pkg__Thing__c"} {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect symbols missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, root) {
		t.Fatalf("inspect symbols leaked absolute project root:\n%s", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"inspect", "symbols", "--project", root, "--kind", "object"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("kind-filtered inspect exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got = stdout.String()
	if strings.Contains(got, "Hello") || !strings.Contains(got, "pkg__Thing__c") {
		t.Fatalf("inspect --kind object did not filter symbols:\n%s", got)
	}
}

func TestInspectPerformanceMovedToPlugin(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "performance", "--project", "."}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("inspect performance unexpectedly succeeded without plugin")
	}
	if !strings.Contains(stderr.String(), "performance scans are provided by the performance plugin") ||
		!strings.Contains(stderr.String(), "glade plugins install @glade/performance") {
		t.Fatalf("stderr missing plugin guidance:\n%s", stderr.String())
	}
}

func TestRunInspectSymbolsProgressWritesToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "symbols", "--project", root, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Project symbols") {
		t.Fatalf("stdout missing inspect result:\n%s", stdout.String())
	}
	for _, want := range []string{"inspect symbols · Loading project", "inspect symbols · 2/3 Indexing Apex symbols", "done · inspect complete"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunSchemaLoad(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "load", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"Glade schema load", "Loaded local metadata", "Objects   1", "Fields", "Next:", "glade check"} {
		if !strings.Contains(got, want) {
			t.Fatalf("schema load output missing %q:\n%s", want, got)
		}
	}
}

func TestRunSchemaLoadCapsObjectList(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for i := 0; i < 82; i++ {
		name := fmt.Sprintf("Thing%02d__c", i)
		writeTestFile(t, filepath.Join(root, "force-app/main/objects", name, name+".object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "load", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if strings.Contains(got, "Thing81__c") {
		t.Fatalf("schema output included object past budget:\n%s", got)
	}
	if !strings.Contains(got, "... 2 more objects omitted. Use `glade schema load --project . --json` for complete output.") {
		t.Fatalf("schema output missing omitted-count line:\n%s", got)
	}
}

func TestRunSchemaLoadProgressWritesToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "load", "--project", root, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Thing__c") {
		t.Fatalf("stdout did not include schema result: %q", stdout.String())
	}
	for _, want := range []string{"schema · Loading project", "schema · 1/2 Loading metadata", "done · schema loaded"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunCheckJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public List<Thing__c> run() { return null; } }")
	writeTestFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	for _, key := range []string{"schemaVersion", "command", "status", "exitCode", "project", "summary", "diagnostics", "suggestions"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("check JSON missing %q: %#v", key, got)
		}
	}
	if got["command"] != "check" || got["status"] != "passed" {
		t.Fatalf("check JSON command/status = %#v", got)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("check JSON missing data object: %#v", got["data"])
	}
	if _, ok := data["diagnostics"]; ok {
		t.Fatalf("check JSON data duplicated diagnostics: %#v", data["diagnostics"])
	}
}

func TestRunCheckJSONOmitsDuplicateDataDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Broken.cls"), "public class Broken { public MissingType run() { return null; } }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got struct {
		Diagnostics []map[string]any `json:"diagnostics"`
		Summary     struct {
			Diagnostics int `json:"diagnostics"`
		} `json:"summary"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if len(got.Diagnostics) == 0 {
		t.Fatalf("envelope diagnostics missing:\n%s", stdout.String())
	}
	if got.Summary.Diagnostics == 0 {
		t.Fatalf("summary diagnostics missing:\n%s", stdout.String())
	}
	if _, ok := got.Data["diagnostics"]; ok {
		t.Fatalf("data duplicated diagnostics:\n%s", stdout.String())
	}
}

func TestRunTriggerBodyCheckReportsBodyAndCapabilityDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/triggers/BadBody.trigger"), `
trigger BadBody on Account (before insert) {
  Integer value = 'wrong';
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/triggers/DuplicateEvent.trigger"), `
trigger DuplicateEvent on Account (before insert, before insert) {}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/triggers/NotTriggerable.trigger"), `
trigger NotTriggerable on ApexClass (before insert) {}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"GLADESEMA018", "GLADESEMA029", "GLADESEMA030"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("check JSON missing %s:\n%s", want, stdout.String())
		}
	}
}

func TestRunTriggerBodyCheckAcceptsSalesforceLegalTriggers(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/AccountHandler.cls"), `
public class AccountHandler {
  public static void handle(List<Account> accounts) {
    System.debug(accounts.size());
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/triggers/AccountTrigger.trigger"), `
trigger AccountTrigger on Account (before insert, after update) {
  static Integer processed = 0;
  processed++;
  AccountHandler.handle(Trigger.new);
  if (Trigger.isUpdate) {
    Account previous = Trigger.oldMap.get(Trigger.new[0].Id);
    System.debug(previous);
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/triggers/ContentVersionTrigger.trigger"), `
trigger ContentVersionTrigger on ContentVersion (before delete, after delete) {
  System.debug(Trigger.old.size());
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunCheckAndTestRejectReservedApexIdentifier(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	reservedPath := filepath.Join(root, "force-app/main/classes/ReservedCurrencyTest.cls")
	writeTestFile(t, reservedPath, `
@isTest
private class ReservedCurrencyTest {
  @isTest
  static void rejectsReservedIdentifier() {
    String currency = 'USD';
    System.assertEquals('USD', currency);
  }
}
`)

	var parseStdout, parseStderr bytes.Buffer
	parseCode := Run(context.Background(), []string{"parse", reservedPath, "--json", "--no-progress"}, &parseStdout, &parseStderr)
	if parseCode != 1 || !strings.Contains(parseStdout.String(), "APEXPARSE002") || !strings.Contains(parseStdout.String(), "Identifier name is reserved: currency") {
		t.Fatalf("parse did not reject reserved identifier: code=%d stdout=%q stderr=%q", parseCode, parseStdout.String(), parseStderr.String())
	}

	var checkStdout, checkStderr bytes.Buffer
	checkCode := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress"}, &checkStdout, &checkStderr)
	if checkCode != 1 {
		t.Fatalf("check exit code = %d, want 1; stderr=%q stdout=%q", checkCode, checkStderr.String(), checkStdout.String())
	}
	var checkEnvelope struct {
		Status      string                  `json:"status"`
		ExitCode    int                     `json:"exitCode"`
		Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(checkStdout.Bytes(), &checkEnvelope); err != nil {
		t.Fatalf("check stdout is not JSON: %v\n%s", err, checkStdout.String())
	}
	if checkEnvelope.Status != "failed" || checkEnvelope.ExitCode != 1 {
		t.Fatalf("check envelope = %#v", checkEnvelope)
	}
	var foundReserved bool
	for _, diag := range checkEnvelope.Diagnostics {
		if diag.Code == "APEXPARSE002" && diag.Message == "Identifier name is reserved: currency" {
			foundReserved = true
			break
		}
	}
	if !foundReserved {
		t.Fatalf("check diagnostics = %#v", checkEnvelope.Diagnostics)
	}

	var testStdout, testStderr bytes.Buffer
	testCode := Run(context.Background(), []string{"test", "--project", root, "--json", "--no-progress", "--no-cache"}, &testStdout, &testStderr)
	if testCode != 1 {
		t.Fatalf("test exit code = %d, want 1; stderr=%q stdout=%q", testCode, testStderr.String(), testStdout.String())
	}
	var testEnvelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []struct {
			Problem *testreport.Problem `json:"problem"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(testStdout.Bytes(), &testEnvelope); err != nil {
		t.Fatalf("test stdout is not JSON: %v\n%s", err, testStdout.String())
	}
	if testEnvelope.Status != "failed" || testEnvelope.ExitCode != 1 {
		t.Fatalf("test envelope = %#v", testEnvelope)
	}
	if testEnvelope.Summary.Total != 1 || testEnvelope.Summary.CompileErrors != 1 || testEnvelope.Summary.Passed != 0 {
		t.Fatalf("test summary = %#v", testEnvelope.Summary)
	}
	if len(testEnvelope.Tests) != 1 || testEnvelope.Tests[0].Problem == nil ||
		!strings.Contains(testEnvelope.Tests[0].Problem.Message, "Identifier name is reserved: currency") {
		t.Fatalf("test cases = %#v", testEnvelope.Tests)
	}
}

func TestRunTestRejectsInvalidMethodBodyBeforeExecution(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/BadBodyTest.cls"), `
@isTest
private class BadBodyTest {
  @isTest
  static void constructsUnknownType() {
    Object thing = new MissingHelper();
    System.assert(false);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--json", "--no-progress", "--no-cache"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []struct {
			Problem *testreport.Problem `json:"problem"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if envelope.Status != "failed" || envelope.ExitCode != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if envelope.Summary.Total != 1 || envelope.Summary.CompileErrors != 1 ||
		envelope.Summary.Passed != 0 || envelope.Summary.Failed != 0 {
		t.Fatalf("summary = %#v", envelope.Summary)
	}
	if len(envelope.Tests) != 1 || envelope.Tests[0].Problem == nil ||
		!strings.Contains(envelope.Tests[0].Problem.Message, "constructs unknown type") ||
		!strings.Contains(envelope.Tests[0].Problem.Message, "MissingHelper") {
		t.Fatalf("test cases = %#v", envelope.Tests)
	}
}

func TestRunCheckUnknownType(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public MissingType run() { return null; } }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"Glade check", "diagnostic found", "GLADESEMA002", "Try:", "glade schema load --project ."} {
		if !strings.Contains(got, want) {
			t.Fatalf("check output missing %q:\n%s", want, got)
		}
	}
}

func TestRunCheckProgressWritesPhasesToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "No diagnostics") {
		t.Fatalf("stdout missing check result: %q", stdout.String())
	}
	for _, want := range []string{
		"check · Loading project",
		"check · 1/4 Loading metadata",
		"check · 2/4 Indexing Apex symbols",
		"check · 3/4 Running semantic checks",
		"done · check complete",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunCheckProgressJSONKeepsStdoutJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--progress-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not json: %q", stdout.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("stderr line is not json: %q\nall stderr:\n%s", line, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), `"phase":"check"`) {
		t.Fatalf("stderr missing check phase:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), `"kind":"done"`) || !strings.Contains(stderr.String(), `"exitCode":0`) {
		t.Fatalf("stderr missing done exit code:\n%s", stderr.String())
	}
}

func TestRunCheckNoProgressSuppressesProgress(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello {}")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDebugParseJSON(t *testing.T) {
	logPath := filepath.Join("..", "apexlog", "testdata", "core.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "parse", "--log", logPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"entries"`) {
		t.Fatalf("parse stdout missing entries: %s", stdout.String())
	}
}

func TestRunDebugProfileText(t *testing.T) {
	logPath := filepath.Join("..", "apexlog", "testdata", "core.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "profile", "--log", logPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"Glade debug profile", "Events:", "Runtime:", "SOQL queries", "DML statements", "Hot events:", "Next:", "glade debug explain --log"} {
		if !strings.Contains(got, want) {
			t.Fatalf("profile stdout missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "## ") || strings.Contains(got, "| Rank |") {
		t.Fatalf("default profile output looks like Markdown:\n%s", got)
	}
}

func TestRunDebugProfileMarkdownFormat(t *testing.T) {
	logPath := filepath.Join("..", "apexlog", "testdata", "core.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "profile", "--log", logPath, "--format", "markdown"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "## Runtime summary") {
		t.Fatalf("markdown profile stdout missing markdown heading:\n%s", stdout.String())
	}
}

func TestRunDebugProfileJSON(t *testing.T) {
	logPath := filepath.Join("..", "apexlog", "testdata", "core.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "profile", "--log", logPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"limits"`) {
		t.Fatalf("profile json missing limits: %s", stdout.String())
	}
}

func TestRunDebugExplainText(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "debuglog", "testdata", "subscriber.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "explain", "--log", logPath, "--project", projectRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "TestProcessor.cls") {
		t.Fatalf("explain stdout missing source file: %s", stdout.String())
	}
}

func TestRunDebugExplainJSON(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "debuglog", "testdata", "subscriber.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "explain", "--log", logPath, "--project", projectRoot, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"entries"`) || !strings.Contains(stdout.String(), `"candidates"`) {
		t.Fatalf("explain json missing entries or candidates: %s", stdout.String())
	}
}

func TestRunDebugEditorJSON(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "debuglog", "testdata", "subscriber.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "editor", "--log", logPath, "--project", projectRoot, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{`"language": "apexlog"`, `"folds"`, `"links"`, `"hovers"`, `"semanticTokens"`, `"diagnostics"`, `"replayFrames"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("editor json missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"command": "debug editor"`) {
		t.Fatalf("editor json should be raw analysis, not CLI envelope:\n%s", got)
	}
}

func TestRunDebugEditorJSONWithoutProject(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "apex.log")
	if err := os.WriteFile(logPath, []byte(strings.Join([]string{
		"00:00:00.001 (1000000)|METHOD_ENTRY|[2]|01p000000000001|ns.MissingClass.run()",
		"00:00:00.002 (2000000)|USER_DEBUG|[3]|DEBUG|hello",
		"00:00:00.003 (3000000)|METHOD_EXIT|[2]|ns.MissingClass.run()",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "editor", "--log", logPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"language": "apexlog"`) || !strings.Contains(stdout.String(), `"folds"`) {
		t.Fatalf("editor json missing parser-only analysis: %s", stdout.String())
	}
}

func TestRunDebugEditorTextSummary(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "debuglog", "testdata", "subscriber.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "editor", "--log", logPath, "--project", projectRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"Glade debug editor", "Entries:", "Folds:", "Links:", "Variables:", "Diagnostics:", "Next:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("editor text missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunDebugRepro(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "debuglog", "testdata", "subscriber.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "repro", "--log", logPath, "--project", projectRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{"TestProcessorRunReproTest", "new Account(Name = 'Acme')", "ns.TestProcessor.run();"} {
		if !strings.Contains(got, want) {
			t.Fatalf("repro stdout missing %q:\n%s", want, got)
		}
	}
}

func TestRunDebugRequiresLog(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "parse"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("parse code = 0, want failure")
	}
	if !strings.Contains(stderr.String(), "--log is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunDAP(t *testing.T) {
	initMessage, err := encodeDAPRequest(dap.CommandInitialize, 1, map[string]any{"clientID": "test"})
	if err != nil {
		t.Fatal(err)
	}
	disconnectMessage, err := encodeDAPRequest(dap.CommandDisconnect, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err = runDAP(context.Background(), nil, bytes.NewReader(append(initMessage, disconnectMessage...)), &stdout)
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	if !strings.Contains(got, `"command":"initialize"`) || !strings.Contains(got, `"event":"initialized"`) || !strings.Contains(got, `"event":"terminated"`) {
		t.Fatalf("dap stdout missing handshake messages:\n%s", got)
	}
}

func TestRunDAPAcceptsDBFlag(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")
	initMessage, err := encodeDAPRequest(dap.CommandInitialize, 1, map[string]any{"clientID": "test"})
	if err != nil {
		t.Fatal(err)
	}
	disconnectMessage, err := encodeDAPRequest(dap.CommandDisconnect, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err = runDAP(context.Background(), []string{"--project", root, "--db", dbPath}, bytes.NewReader(append(initMessage, disconnectMessage...)), &stdout)
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	if !strings.Contains(got, `"command":"initialize"`) || !strings.Contains(got, `"event":"terminated"`) {
		t.Fatalf("dap stdout missing handshake messages:\n%s", got)
	}
}

const dapTestTimeout = 45 * time.Second

func TestRunDAPLaunchEmitsStopped(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() {
		_ = inW.Close()
		_ = outR.Close()
	})
	done := make(chan error, 1)
	go func() {
		defer outW.Close()
		done <- runDAP(context.Background(), []string{"--project", filepath.Join("..", "debuglog", "testdata", "project")}, inR, outW)
	}()

	messages := make(chan map[string]any, 16)
	go readDAPMessages(t, outR, messages)

	writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
	writeDAPRequest(t, inW, dap.CommandSetBreakpoints, 2, map[string]any{
		"source":      map[string]any{"name": "anonymous.apex"},
		"breakpoints": []map[string]any{{"line": 2}},
	})
	writeDAPRequest(t, inW, dap.CommandLaunch, 3, map[string]any{"program": "Integer x = 1;\nx = x + 1;"})
	waitForDAPEvent(t, messages, "stopped")
	writeDAPRequest(t, inW, dap.CommandDisconnect, 4, nil)

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(dapTestTimeout):
		t.Fatal("DAP server did not stop")
	}
}

func TestRunDAPLaunchAcceptsIDEProjectRootAndAnonymousBody(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- runDAP(context.Background(), nil, inR, outW)
	}()
	messages := make(chan map[string]any, 16)
	go readDAPMessages(t, outR, messages)

	writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
	writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{
		"projectRoot":   filepath.Join("..", "debuglog", "testdata", "project"),
		"anonymousBody": "System.debug('alias body');",
	})
	writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
	stderrOutput := waitForDAPTerminatedAndStderr(t, messages)
	writeDAPRequest(t, inW, dap.CommandDisconnect, 4, nil)
	_ = inW.Close()
	_ = outW.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(dapTestTimeout):
		t.Fatal("DAP server did not stop")
	}
	if strings.Contains(stderrOutput, "launch requires program or source") {
		t.Fatalf("DAP launch did not accept IDE aliases: %q", stderrOutput)
	}
}

func TestRunDAPWithDBPersistsOnCleanTermination(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer outW.Close()
		done <- runDAP(context.Background(), []string{"--project", root, "--db", dbPath}, inR, outW)
	}()

	messages := make(chan map[string]any, 16)
	go readDAPMessages(t, outR, messages)

	writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
	writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{
		"source": "insert new Account(Name = 'DAP Supply');",
	})
	writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
	waitForDAPEvent(t, messages, "terminated")
	_ = inW.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(dapTestTimeout):
		t.Fatal("DAP server did not stop")
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db inspect", &inspect)
	if got := inspect.ByObject["Account"]; got != 1 {
		t.Fatalf("Account rows = %d, want 1; inspect=%s", got, stdout.String())
	}
}

func TestRunDAPLaunchCanExecutePrivateTestMethodInClass(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/SampleTest.cls"), `
@IsTest
private class SampleTest {
  @IsTest
  static void checks() {
    System.assertEquals(1, 1);
  }
}
`)

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer outW.Close()
		done <- runDAP(context.Background(), []string{"--project", root}, inR, outW)
	}()

	messages := make(chan map[string]any, 16)
	go readDAPMessages(t, outR, messages)

	writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
	writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{
		"source":     "checks();",
		"className":  "SampleTest",
		"methodName": "checks",
	})
	writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
	stderrOutput := waitForDAPTerminatedAndStderr(t, messages)
	_ = inW.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(dapTestTimeout):
		t.Fatal("DAP server did not stop")
	}
	if stderrOutput != "" {
		t.Fatalf("DAP stderr output = %q", stderrOutput)
	}
}

func TestRunDAPLaunchTestMethodDoesNotPersistDBWrites(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/SampleTest.cls"), `
@IsTest
private class SampleTest {
  @IsTest
  static void insertsAccount() {
    insert new Account(Name = 'Test Debug Row');
  }
}
`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer outW.Close()
		done <- runDAP(context.Background(), []string{"--project", root, "--db", dbPath}, inR, outW)
	}()

	messages := make(chan map[string]any, 16)
	go readDAPMessages(t, outR, messages)

	writeDAPRequest(t, inW, dap.CommandInitialize, 1, nil)
	writeDAPRequest(t, inW, dap.CommandLaunch, 2, map[string]any{
		"source":     "insertsAccount();",
		"className":  "SampleTest",
		"methodName": "insertsAccount",
	})
	writeDAPRequest(t, inW, dap.CommandConfigurationDone, 3, nil)
	stderrOutput := waitForDAPTerminatedAndStderr(t, messages)
	_ = inW.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(dapTestTimeout):
		t.Fatal("DAP server did not stop")
	}
	if stderrOutput != "" {
		t.Fatalf("DAP stderr output = %q", stderrOutput)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db inspect", &inspect)
	if got := inspect.ByObject["Account"]; got != 0 {
		t.Fatalf("Account rows = %d, want 0; inspect=%s", got, stdout.String())
	}
}

func TestRunExec(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "Integer x = 1 + 1; System.debug('x=' + x); System.assertEquals(2, x);"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{"Glade exec", "Anonymous Apex executed", "USER_DEBUG x=2", "Limits:", "SOQL queries", "Log:", ".glade/logs/exec-", ".apexlog", "Next:", "glade debug profile --log"} {
		if !strings.Contains(got, want) {
			t.Fatalf("exec output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "|EXECUTION_STARTED") {
		t.Fatalf("exec default dumped raw debug log:\n%s", got)
	}
}

func TestRunExecRejectsReservedLocalIdentifiersWithAndWithoutProject(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "projectless", args: []string{"exec", "String CuRrEnCy = 'USD';"}},
		{name: "project", args: []string{"exec", "--project", projectRoot, "String CuRrEnCy = 'USD';"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), test.args, &stdout, &stderr)
			if code == 0 || !strings.Contains(strings.ToLower(stderr.String()), "identifier name is reserved: currency") {
				t.Fatalf("reserved anonymous local must fail: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunExecRejectsSemanticErrorsWithoutProject(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"switch on true { when true { System.debug(true); } }",
	}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "switch does not support Boolean selectors") {
		t.Fatalf("standalone exec must run semantic analysis: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExecSummaryCapsDebugLines(t *testing.T) {
	var result vm.Result
	for i := 0; i < 82; i++ {
		result.Debug = append(result.Debug, fmt.Sprintf("line %d", i))
	}
	var out bytes.Buffer
	if err := writeExecSummary(&out, result, filepath.Join(".glade", "logs", "exec-test.log")); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "USER_DEBUG line 81") {
		t.Fatalf("exec summary included debug line past budget:\n%s", got)
	}
	if !strings.Contains(got, "... 2 more debug lines omitted. See the debug log path below for complete output.") {
		t.Fatalf("exec summary missing omitted-count line:\n%s", got)
	}
}

func TestRunExecWithDBPersistsAnonymousDML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--db", dbPath,
		"insert new Account(Name = 'Pond Supply');",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
		Records  int            `json:"records"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db inspect", &inspect)
	if got := inspect.ByObject["Account"]; got != 1 {
		t.Fatalf("Account rows = %d, want 1; inspect=%s", got, stdout.String())
	}
}

func TestRunExecWithProjectRegistersLocalClasses(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/LocalProbe.cls"), `
public class LocalProbe {
  public static String value() {
    return 'local';
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"System.assertEquals('local', LocalProbe.value());",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExecWithProjectRejectsAnonymousSemanticDiagnosticsBeforeExecution(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--project", root, "String value = 'x'; insert value;"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "GLADESEMA034") {
		t.Fatalf("anonymous semantic error must fail before execution: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExecLoadsCurrentProjectOrgFeaturesByDefault(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [MultiCurrency]\n")
	t.Chdir(root)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"System.assert(UserInfo.isMultiCurrencyOrganization());",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExecWithDBDryRunDoesNotPersist(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--db", dbPath,
		"--dry-run",
		"insert new Account(Name = 'Dry Run Supply');",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db inspect", &inspect)
	if got := inspect.ByObject["Account"]; got != 0 {
		t.Fatalf("Account rows = %d, want 0; inspect=%s", got, stdout.String())
	}
}

func TestRunExecWithDBDoesNotPersistOnExecutionError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--project", root,
		"--db", dbPath,
		"insert new Account(Name = 'Bad Supply'); System.assertEquals(1, 2);",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exec succeeded, want failure stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath, "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect failed code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db inspect", &inspect)
	if got := inspect.ByObject["Account"]; got != 0 {
		t.Fatalf("Account rows = %d, want 0; inspect=%s", got, stdout.String())
	}
}

func TestRunDebugReplayJSON(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "debuglog", "testdata", "subscriber.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "replay", "--log", logPath, "--project", projectRoot, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{`"command": "debug replay"`, `"source"`, "insert setup_accountRows;", "ns.TestProcessor.run();"} {
		if !strings.Contains(got, want) {
			t.Fatalf("replay stdout missing %q:\n%s", want, got)
		}
	}
}

func TestRunDebugReplayEntryIndexJSON(t *testing.T) {
	projectRoot := filepath.Join("..", "debuglog", "testdata", "project")
	logPath := filepath.Join("..", "apexlog", "testdata", "exception.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "replay", "--log", logPath, "--project", projectRoot, "--entry-index", "2", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "ns.TestProcessor.fail();") {
		t.Fatalf("replay entry-index json missing selected method: %s", stdout.String())
	}
}

func writeDAPRequest(t *testing.T, w io.Writer, command string, seq int, args any) {
	t.Helper()
	message, err := encodeDAPRequest(command, seq, args)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(message); err != nil {
		t.Fatal(err)
	}
}

func readDAPMessages(t *testing.T, r io.Reader, out chan<- map[string]any) {
	t.Helper()
	reader := dap.NewReader(r)
	for {
		raw, err := reader.Read()
		if err != nil {
			close(out)
			return
		}
		var message map[string]any
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Errorf("decode DAP message: %v", err)
			close(out)
			return
		}
		out <- message
	}
}

func waitForDAPEvent(t *testing.T, messages <-chan map[string]any, event string) {
	t.Helper()
	deadline := time.After(dapTestTimeout)
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				t.Fatalf("DAP stream closed before %q", event)
			}
			if message["type"] == dap.MessageTypeEvent && message["event"] == event {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for DAP event %q", event)
		}
	}
}

func waitForDAPTerminatedAndStderr(t *testing.T, messages <-chan map[string]any) string {
	t.Helper()
	deadline := time.After(dapTestTimeout)
	var stderr strings.Builder
	for {
		select {
		case message, ok := <-messages:
			if !ok {
				t.Fatal("DAP stream closed before terminated")
			}
			if message["type"] != dap.MessageTypeEvent {
				continue
			}
			if message["event"] == "output" {
				body, _ := message["body"].(map[string]any)
				if body["category"] == "stderr" {
					stderr.WriteString(fmt.Sprint(body["output"]))
				}
			}
			if message["event"] == "terminated" {
				return stderr.String()
			}
		case <-deadline:
			t.Fatal("timeout waiting for DAP terminated event")
		}
	}
}

func TestRunExecJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--json", "List<Integer> xs = new List<Integer>{1, 2}; System.assertEquals(2, xs.size()); System.debug('hello');"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"hello"`) || !strings.Contains(stdout.String(), `"trace"`) {
		t.Fatalf("stdout did not include JSON debug output: %q", stdout.String())
	}
}

func TestRunExecLimitProfileJSONAndExplicitCaps(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec",
		"--json",
		"--limit-profile", "strict-async",
		"--limit-queries", "7",
		"System.assertEquals(7, Limits.getLimitQueries()); System.debug('profile');",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got["command"] != "exec" || got["status"] != "passed" {
		t.Fatalf("exec JSON command/status = %#v", got)
	}
	if !strings.Contains(stdout.String(), `"profile"`) {
		t.Fatalf("exec JSON missing debug output: %s", stdout.String())
	}
}

func TestRunExecTraceFile(t *testing.T) {
	t.Chdir(t.TempDir())
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--trace", tracePath, "Integer x = 1; System.assertEquals(1, x);"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `"format": "chrome-trace-event"`) || !strings.Contains(text, `"traceEvents"`) {
		t.Fatalf("trace file did not include chrome trace document: %q", text)
	}
}

func TestRunExecDebugLogFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "apex.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--debug-log", logPath, "System.debug('hello world'); Integer x = 1;"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"APEX_CODE,DEBUG;",
		"|EXECUTION_STARTED",
		"|USER_DEBUG|",
		"hello world",
		"|CUMULATIVE_LIMIT_USAGE",
		"|EXECUTION_FINISHED",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("debug log missing %q; got:\n%s", want, text)
		}
	}
	if !strings.Contains(stdout.String(), "Glade exec") || !strings.Contains(stdout.String(), "Log:") {
		t.Fatalf("debug log file stdout should summarize artifact:\n%s", stdout.String())
	}
}

func TestRunExecDebugLogCreatesParentDirs(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "reports", "exec.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--debug-log", logPath, "System.debug('nested log');"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "nested log") {
		t.Fatalf("debug log missing user message:\n%s", content)
	}
}

func TestRunExecDebugLogStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--debug-log", "-", "System.debug('to stdout');"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "|EXECUTION_STARTED") || !strings.Contains(stdout.String(), "to stdout") {
		t.Fatalf("expected debug log on stdout, got:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "Glade exec") {
		t.Fatalf("--debug-log - should not append human summary:\n%s", stdout.String())
	}
}

func TestRunExecDebugLogRawMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--debug-log", "raw", "System.debug('raw mode');"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "|EXECUTION_STARTED") || !strings.Contains(got, "raw mode") {
		t.Fatalf("expected raw debug log on stdout, got:\n%s", got)
	}
	if strings.Contains(got, "Glade exec") {
		t.Fatalf("raw mode should not append human summary:\n%s", got)
	}
}

func TestRunExecFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "System.assertEquals(3, 1 + 1);"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "System.AssertException") {
		t.Fatalf("stderr did not include assertion failure: %q", stderr.String())
	}
}

func TestRunPlaygroundOnce(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "playground.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--workspace", "default", "--data-root", root, "--db", dbPath, "--once"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "URL") || !strings.Contains(stdout.String(), "/playground/") || strings.Contains(stdout.String(), "glade playground:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunPlaygroundProjectRefFlag(t *testing.T) {
	root := t.TempDir()
	projectRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "playground.sqlite")
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"playground",
		"--workspace", "default",
		"--data-root", root,
		"--db", dbPath,
		"--project-ref", "Local Probe=" + projectRoot,
		"--once",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunPlaygroundListExamples(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/LocalProbe.cls"), "public class LocalProbe {}")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"playground",
		"--list-examples",
		"--project-ref", "Local Probe=" + projectRoot,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"refinement-service", "Refinement Service", "files", "DML", "local-probe", "Local Probe"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "glade playground:") {
		t.Fatalf("list-examples should not start the server:\n%s", out)
	}
}

func TestRunPlaygroundExampleFlagPrintsDeepLocalURL(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "playground.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"playground",
		"--workspace", "default",
		"--data-root", root,
		"--db", dbPath,
		"--example", "refinement-service",
		"--once",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "http://127.0.0.1:1789/playground/?example=refinement-service") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunPlaygroundExampleFlagImpliesExamples(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"playground",
		"--wizard",
		"--data-root", root,
		"--example", "refinement-service",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"--examples", "--example", "refinement-service", "?example=refinement-service"} {
		if !strings.Contains(out, want) {
			t.Fatalf("wizard output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPlaygroundExampleFlagRejectsUnknownID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--example", "missing-example", "--once"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown playground example "missing-example"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPlaygroundExampleFlagRejectsProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--project", projectRoot, "--example", "refinement-service", "--once"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--example requires the managed scratch workspace") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPlaygroundExampleFlagRejectsProjectRefs(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(projectRoot, "force-app/main/default/classes/LocalProbe.cls"), "public class LocalProbe {}")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--project-ref", "Local Probe=" + projectRoot, "--example", "refinement-service", "--once"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--example cannot be combined with --project-ref") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPlaygroundExamplesFlag(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "playground.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"playground",
		"--workspace", "default",
		"--data-root", root,
		"--db", dbPath,
		"--examples",
		"--once",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunPlaygroundNoDB(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--no-db", "--once"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "playground", "org.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("org sqlite should not be written with --no-db; stat err=%v", err)
	}
}

func TestRunPlaygroundResetOnStartRefusesProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--project", projectRoot, "--reset-on-start", "--once"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--reset-on-start refuses --project") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestResetPlaygroundStateRejectsEscapingWorkspaceID(t *testing.T) {
	parent := t.TempDir()
	dataRoot := filepath.Join(parent, "playground")
	outside := filepath.Join(parent, "outside")
	sentinel := filepath.Join(outside, "sentinel.txt")
	writeTestFile(t, sentinel, "keep")

	err := resetPlaygroundState(dataRoot, "../../outside", "")
	if err == nil {
		t.Fatalf("resetPlaygroundState() error = nil, want path rejection")
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		t.Fatalf("sentinel was removed or changed: %v", statErr)
	}
}

func TestResetPlaygroundStateRejectsSymlinkedWorkspaces(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "playground")
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "default", "sentinel.txt")
	writeTestFile(t, sentinel, "keep")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dataRoot, "workspaces")); err != nil {
		t.Fatal(err)
	}
	if err := resetPlaygroundState(dataRoot, "default", ""); err == nil {
		t.Fatal("resetPlaygroundState() succeeded through symlink")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatalf("sentinel = %q, %v", data, err)
	}
}

func TestResetPlaygroundStateRejectsInternalSymlinkedWorkspace(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "playground")
	other := filepath.Join(dataRoot, "other")
	sentinel := filepath.Join(other, "sentinel.txt")
	writeTestFile(t, sentinel, "keep")
	if err := os.MkdirAll(filepath.Join(dataRoot, "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(dataRoot, "workspaces", "default")); err != nil {
		t.Fatal(err)
	}
	if err := resetPlaygroundState(dataRoot, "default", ""); err == nil {
		t.Fatal("resetPlaygroundState() succeeded through internal symlink")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatalf("sentinel = %q, %v", data, err)
	}
}

func TestRunPlaygroundRejectsEscapingWorkspaceID(t *testing.T) {
	parent := t.TempDir()
	dataRoot := filepath.Join(parent, "playground")
	outside := filepath.Join(parent, "outside")
	writeTestFile(t, filepath.Join(outside, "sentinel.txt"), "keep")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--data-root", dataRoot, "--workspace", "../../outside", "--once"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid playground workspace id") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "sfdx-project.json")); !os.IsNotExist(err) {
		t.Fatalf("playground wrote outside data root; stat err=%v", err)
	}
}

func TestResetPlaygroundStateClearsResultCache(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "playground")
	workspaceFile := filepath.Join(dataRoot, "workspaces", "default", "anonymous.apex")
	cacheFile := filepath.Join(dataRoot, "cache", "latest.json")
	dbPath := filepath.Join(dataRoot, "org.sqlite")
	writeTestFile(t, workspaceFile, "System.debug('old');")
	writeTestFile(t, cacheFile, `{"status":"pass"}`)
	writeTestFile(t, dbPath, "sqlite")

	if err := resetPlaygroundState(dataRoot, "default", dbPath); err != nil {
		t.Fatalf("resetPlaygroundState() error = %v", err)
	}
	for _, path := range []string{workspaceFile, cacheFile, dbPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed; stat err=%v", path, err)
		}
	}
}

func TestRunPlaygroundWizardPrintsCommand(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--wizard", "--data-root", root, "--examples", "--example", "refinement-service", "--no-db", "--reset-on-start", "--public"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"GLADE_SERVER_PUBLIC=1", "glade playground", "--data-root", root, "--examples", "--example", "refinement-service", "--no-db", "--reset-on-start", "--public", "--open", "?example=refinement-service"} {
		if !strings.Contains(out, want) {
			t.Fatalf("wizard output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPlaygroundWizardHonorsNoOpenAndPublicAddress(t *testing.T) {
	root := t.TempDir()
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--wizard", "--project", projectRoot, "--data-root", root, "--public", "--no-open"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"GLADE_SERVER_PUBLIC=1", "--public", "--no-open", "http://0.0.0.0:8080/playground/"} {
		if !strings.Contains(out, want) {
			t.Fatalf("wizard output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--open") {
		t.Fatalf("wizard output should not include --open when --no-open is explicit:\n%s", out)
	}
}

func TestRunPlaygroundWizardPublicAddressPrintsOptInEnv(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--wizard", "--addr", "0.0.0.0:8080", "--data-root", root, "--no-open"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"GLADE_SERVER_PUBLIC=1", "--addr 0.0.0.0:8080", "--no-open", "http://0.0.0.0:8080/playground/"} {
		if !strings.Contains(out, want) {
			t.Fatalf("wizard output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--public") {
		t.Fatalf("explicit public --addr wizard should not rewrite to --public:\n%s", out)
	}
}

func TestRunPlaygroundRejectsFlagTokenAsProjectValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--project", "--once", "--wizard"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--project requires a value") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPlaygroundUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown flag "--bogus"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunTestJSONAndJUnit(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assertEquals(2, 1 + 1);
  }
}
`)
	junitPath := filepath.Join(t.TempDir(), "junit.xml")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--json", "--junit", junitPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	for _, key := range []string{"schemaVersion", "command", "status", "exitCode", "summary", "tests", "artifacts", "suggestions"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("test JSON missing %q: %#v", key, got)
		}
	}
	if got["command"] != "test" || got["status"] != "passed" {
		t.Fatalf("test JSON command/status = %#v", got)
	}
	suggestions, ok := got["suggestions"].([]any)
	if !ok {
		t.Fatalf("test JSON suggestions = %#v", got["suggestions"])
	}
	for _, suggestion := range suggestions {
		if suggestion == "glade test failed" {
			t.Fatalf("test JSON suggested last-failed rerun after passing run: %#v", suggestions)
		}
	}
	junit, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(junit), `<testsuites name="glade test" tests="1" failures="0" errors="0" skipped="0"`) {
		t.Fatalf("junit output = %q", string(junit))
	}
}

func TestRunTestLimitProfileAndExplicitCaps(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/LimitProfileTest.cls"), `
@isTest
private class LimitProfileTest {
  @isTest static void usesExplicitCapOverProfile() {
    System.assertEquals(7, Limits.getLimitQueries());
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"test",
		"--project", root,
		"--limit-profile", "strict-async",
		"--limit-queries", "7",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "1 passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTestProgressWritesToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assertEquals(2, 1 + 1);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "1 passed") {
		t.Fatalf("stdout did not include console result: %q", stdout.String())
	}
	got := stderr.String()
	for _, want := range []string{"test ·", "1/1", "SampleTest.passes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr missing %q:\n%s", want, got)
		}
	}
}

func TestRunDevTestProgressWritesToStderr(t *testing.T) {
	_ = os.RemoveAll(".glade")
	t.Cleanup(func() { _ = os.RemoveAll(".glade") })
	root := filepath.Join("..", "..", "testdata", "local-tests", "basic")
	outRoot := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"dev", "test", "--project", root, "--class", "PassingTest", "--out", outRoot, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Glade test") {
		t.Fatalf("stdout missing test summary:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "test ·") {
		t.Fatalf("stderr missing test progress:\n%s", stderr.String())
	}
	if _, err := os.Stat(".glade"); !os.IsNotExist(err) {
		t.Fatalf("dev test wrote package-local .glade artifact: %v", err)
	}
}

func TestRunTestProgressJSONWritesNDJSONToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assertEquals(2, 1 + 1);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--progress-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "1 passed") {
		t.Fatalf("stdout did not include console result: %q", stdout.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("stderr line is not json: %q\nall stderr:\n%s", line, stderr.String())
		}
	}
	for _, want := range []string{`"kind":"phase_start"`, `"kind":"phase_tick"`, `"kind":"done"`} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), `"exitCode":0`) {
		t.Fatalf("stderr missing done exit code:\n%s", stderr.String())
	}
}

func TestRunTestStaticHelperMethod(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void adds() {
    System.assertEquals(3, MathUtil.add(1, 2));
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"className": "MathUtilTest"`) || !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTestStaticHelperMethodWithBranching(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer max(Integer a, Integer b) {
    if (a > b) {
      return a;
    } else {
      return b;
    }
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void maxChoosesLargerValue() {
    System.assertEquals(5, MathUtil.max(5, 2));
    System.assertEquals(7, MathUtil.max(3, 7));
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTestStaticHelperMethodWithWhileLoop(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer sumTo(Integer n) {
    Integer total = 0;
    Integer i = 1;
    while (i <= n) {
      total = total + i;
      i = i + 1;
    }
    return total;
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void sumsRange() {
    System.assertEquals(15, MathUtil.sumTo(5));
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTestInstanceHelperMethod(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Calculator.cls"), `
public class Calculator {
  public Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/CalculatorTest.cls"), `
@isTest
private class CalculatorTest {
  @isTest static void instanceMethodAdds() {
    Calculator calc = new Calculator();
    System.assertEquals(7, calc.add(3, 4));
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTestWatchOnceStreamsEvents(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assert(true);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--watch-once"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"event":"watch.started"`) || !strings.Contains(stdout.String(), `"event":"watch.run_finished"`) {
		t.Fatalf("watch stdout = %q", stdout.String())
	}
}

func TestRunTestWatchOnceEmitsProfileSummaryWhenTracing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/ProfiledWatchTest.cls"), `
@isTest
private class ProfiledWatchTest {
  @isTest static void fails() {
    System.assert(false, 'trace this failure');
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--watch-once", "--trace-blockers", "--no-progress"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"event":"watch.profile_summary"`) || !strings.Contains(got, `"profiles":1`) || !strings.Contains(got, "ProfiledWatchTest.fails") {
		t.Fatalf("watch stdout missing profile summary:\n%s", got)
	}
}

func TestRunTestDaemonFilterUsesWarmService(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() {
    System.assertEquals(2, 1 + 1);
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() {
    System.assertEquals(3, 1 + 2);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--daemon", "--filter", "WarmOneTest", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"total": 1`) || !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "WarmTwoTest") {
		t.Fatalf("daemon filter ran unselected class: %q", stdout.String())
	}
}

func TestRunTestDaemonChangedSinceNarrowsMultipleAffectedClasses(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCLI(t, root, "init")
	runGitCLI(t, root, "config", "user.email", "glade@example.test")
	runGitCLI(t, root, "config", "user.name", "GLADE")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Helper.cls"), `
public class Helper {
  public static void touch() {}
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() {
    Helper.touch();
    System.assert(true);
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() {
    Helper.touch();
    System.assert(true);
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/WarmThreeTest.cls"), `
@isTest
private class WarmThreeTest {
  @isTest static void passes() {
    System.assert(true);
  }
}
`)
	runGitCLI(t, root, "add", ".")
	runGitCLI(t, root, "commit", "-m", "baseline")
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Helper.cls"), `
public class Helper {
  public static void touch() {
    // changed implementation detail
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--daemon", "--changed-since", "HEAD", "--json", "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"total": 2`) || !strings.Contains(stdout.String(), `"passed": 2`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "WarmThreeTest") {
		t.Fatalf("daemon changed-since ran unselected class: %q", stdout.String())
	}
}

func TestRunTestDaemonStatusNoServer(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "daemon", "status", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "test daemon: stopped") || !strings.Contains(got, "socket: ") || !strings.Contains(got, "pid: none") {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunTestDaemonStatusAndStopRunningServer(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	socket := testdaemon.ServeSocketPath(root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := testdaemon.NewServer(testdaemon.ServerConfig{
		Root:  root,
		Warm:  true,
		Watch: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx, io.Discard) }()
	waitForServer(t, ctx, socket)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "daemon", "status", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "test daemon: running") || !strings.Contains(got, "ready: true") || !strings.Contains(got, "pid: ") {
		t.Fatalf("status stdout = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"test", "daemon", "stop", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stop exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "test daemon: stopped") {
		t.Fatalf("stop stdout = %q", stdout.String())
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket still present: %v", err)
	}
}

func TestRunTestLastFailedRerunsRecordedFailures(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/FailingTest.cls"), `
@isTest
private class FailingTest {
  @isTest static void fails() { System.assert(false, 'still broken'); }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/PassingTest.cls"), `
@isTest
private class PassingTest {
  @isTest static void passes() { System.assert(true); }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--filter", "FailingTest", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("first exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"test", "--project", root, "--last-failed", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("last-failed exit code = %d, want 1; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	if !strings.Contains(got, `"total": 1`) || !strings.Contains(got, "FailingTest") {
		t.Fatalf("stdout = %q", got)
	}
	if strings.Contains(got, "PassingTest") {
		t.Fatalf("last-failed ran passing class: %q", got)
	}
}

func TestRunTestChangedAliasUsesHead(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"help", "test"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"glade test changed", "--since <ref>", "--last-failed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRewriteChangedTestArgsDefaultsToHead(t *testing.T) {
	args, err := rewriteChangedTestArgs([]string{"--project", "repo", "--json", "--daemon"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "--project repo --changed-since HEAD --json --daemon" {
		t.Fatalf("args = %q", got)
	}
	args, err = rewriteChangedTestArgs([]string{"--project", "repo", "--since", "origin/main", "--no-progress"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "--project repo --changed-since origin/main --no-progress" {
		t.Fatalf("args = %q", got)
	}
}

func TestRunTestWizardPrintsDailyLoopCommands(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--wizard"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"Test wizard",
		"daemon: stopped",
		"cache:",
		"glade test changed --project",
		"glade test --project",
		"--last-failed",
		"glade test serve --project",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("wizard missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunTestProgressIncludesStartupCacheHint(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CacheHintTest.cls"), `
@isTest
private class CacheHintTest {
  @isTest static void passes() { System.assert(true); }
}
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--filter", "CacheHintTest", "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "startup cache:") {
		t.Fatalf("stderr missing cache hint:\n%s", stderr.String())
	}
}

func TestRunTestProgressReportsFreshCacheAndParallelBypass(t *testing.T) {
	restoreDiskCache := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDiskCache)
	previousGOMAXPROCS := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousGOMAXPROCS) })

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CacheHintTest.cls"), `
@isTest
private class CacheHintTest {
  @isTest static void passes() { System.assert(true); }
}
`)
	writeFreshTestStartupCache(t, root)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--filter", "CacheHintTest", "--progress", "--no-serve"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"startup cache: fresh",
		"one-shot cache: bypassed for parallel methods with more than one worker; the startup cache will not be read or written for this run. Use glade test serve --project " + root,
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func runGitCLI(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestRunTestDaemonWatchOnceStreamsEvents(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assert(true);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--daemon", "--watch-once"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"event":"watch.started"`) || !strings.Contains(stdout.String(), `"event":"watch.run_finished"`) {
		t.Fatalf("watch stdout = %q", stdout.String())
	}
}

func TestWatchIndexUpdateUsesIncrementalForApexOnlyChanges(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Sample.cls")
	triggerPath := filepath.Join(root, "SampleTrigger.trigger")
	writeTestFile(t, classPath, "public class Sample { public void oldName() {} }")
	writeTestFile(t, triggerPath, "trigger SampleTrigger on Account (before insert) {}")
	index, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, classPath, "public class Sample { public void newName() {} }")

	updated, err := updateWatchIndex(root, index, []watch.Change{{
		Path: classPath,
		Op:   watch.ChangeModified,
		Kind: watch.FileKindApexClass,
		Name: "Sample",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Triggers) != 1 {
		t.Fatalf("triggers = %#v", updated.Triggers)
	}
	if len(updated.Types) != 1 || len(updated.Types[0].Members) != 1 || updated.Types[0].Members[0].Name != "newName" {
		t.Fatalf("types = %#v", updated.Types)
	}
}

func TestWatchIndexUpdateReturnsFailedFallbackWithoutCandidate(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Stable { public void beforeEdit() {} }")
	index, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, manifestPath, "{")
	writeTestFile(t, classPath, "public class Stable {")
	updated, err := updateWatchIndex(root, index, []watch.Change{{
		Path: classPath,
		Op:   watch.ChangeModified,
		Kind: watch.FileKindApexClass,
		Name: "Stable",
	}})
	if err == nil {
		t.Fatal("updateWatchIndex succeeded after authoritative fallback failed")
	}
	if updated.Project != (typesys.ProjectInfo{}) || len(updated.Types) != 0 || len(updated.Triggers) != 0 || len(updated.Diagnostics) != 0 {
		t.Errorf("updateWatchIndex returned a candidate after failed fallback: %#v", updated)
	}
}

func TestWatchIndexStatePreservesLiveStateAndRecoversAfterFailedFallback(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	manifest := `{"packageDirectories":[{"path":"force-app","default":true}]}`
	writeTestFile(t, manifestPath, manifest)
	writeTestFile(t, classPath, "public class Stable { public void beforeEdit() {} }")
	index, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := watch.BuildReferenceGraph(index)
	beforeIndex := index
	beforeGraph := watch.BuildReferenceGraph(index)

	writeTestFile(t, manifestPath, "{")
	writeTestFile(t, classPath, "public class Stable {")
	index, graph, err = updateWatchIndexState(root, index, graph, []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Stable"}})
	if err == nil {
		t.Fatal("watch state update succeeded after fallback failure")
	}
	if !reflect.DeepEqual(index, beforeIndex) || !reflect.DeepEqual(graph, beforeGraph) {
		t.Errorf("failed fallback changed live watch state:\nindex: %#v\ngraph: %#v", index, graph)
	}

	writeTestFile(t, manifestPath, manifest)
	writeTestFile(t, classPath, "public class Stable { public void afterEdit() {} }")
	index, graph, err = updateWatchIndexState(root, index, graph, []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Stable"}})
	if err != nil {
		t.Fatalf("subsequent valid change failed: %v", err)
	}
	if len(index.Types) != 1 || len(index.Types[0].Members) != 1 || index.Types[0].Members[0].Name != "afterEdit" || graph == nil {
		t.Errorf("subsequent valid change did not use retained state: index=%#v graph=%#v", index, graph)
	}
}

func TestRunLSPDiagnosticsOnce(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Broken.cls"), "public class Broken { public MissingType run() { return null; } }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"lsp", "--project", root, "--diagnostics-once"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Content-Length:") || !strings.Contains(stdout.String(), "textDocument/publishDiagnostics") {
		t.Fatalf("lsp stdout = %q", stdout.String())
	}
}

func TestRunExecDebugEmitsDAPInitializeResponse(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--debug", "Integer x = 1; System.assertEquals(1, x);"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Content-Length:") || !strings.Contains(stdout.String(), "supportsConfigurationDoneRequest") {
		t.Fatalf("debug stdout = %q", stdout.String())
	}
}

func TestRunTestDebugEmitsDAPInitializeResponse(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assert(true);
  }
}
`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--debug"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Content-Length:") || !strings.Contains(stdout.String(), "supportsConfigurationDoneRequest") {
		t.Fatalf("debug stdout = %q", stdout.String())
	}
}

func TestRunProfileAnalyzeJSON(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	writeTestFile(t, tracePath, `{"format":"chrome-trace-event","version":1,"traceEvents":[{"name":"apex.statement.expr","cat":"apex.statement","ph":"i","ts":1,"pid":1,"tid":1,"args":{"sourceOffset":5}}]}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"profile", "analyze", tracePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"events": 1`) || !strings.Contains(stdout.String(), `"apex.statement.expr"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunProfileAnalyzePprofFormat(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	writeTestFile(t, tracePath, `{"format":"chrome-trace-event","version":1,"traceEvents":[{"name":"apex.method.RefinementService.run","cat":"apex.method","ph":"X","ts":1,"dur":2000,"pid":1,"tid":1,"args":{"file":"RefinementService.cls","line":3}}]}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"profile", "analyze", tracePath, "--format", "pprof"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	data := stdout.Bytes()
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Fatalf("stdout is not gzipped pprof data: %q", stdout.String())
	}
}

func TestReleaseBuildScriptWritesMachineReadableManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "release-build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"release-manifest.json",
		"archive_sha256",
		"version_output",
		"doctor_json",
		"parser_smoke",
		"vscode_extension_package",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release-build.sh missing %q", want)
		}
	}
}

func TestRunDBSeedInspectExportAndReset(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")
	fixturePath := filepath.Join(dir, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"schemaVersion": 1`) || !strings.Contains(stdout.String(), `"Account": 1`) || !strings.Contains(stdout.String(), `"users": 2`) {
		t.Fatalf("seed stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "schemaVersion: 1") || !strings.Contains(stdout.String(), "Account: 1") || !strings.Contains(stdout.String(), "User: 2") {
		t.Fatalf("inspect stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "export", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("export exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version": "glade.storage.v1"`) || !strings.Contains(stdout.String(), `"Acme"`) {
		t.Fatalf("export stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "reset", "--db", dbPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reset exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"Account": 0`) || !strings.Contains(stdout.String(), `"users": 2`) {
		t.Fatalf("reset stdout = %q", stdout.String())
	}
}

func TestRunDBInspectUsesDefaultProjectEnvironment(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Label__c")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), filepath.ToSlash(filepath.Join(root, ".glade", "envs", "dev.sqlite"))) {
		t.Fatalf("inspect did not use default project env db:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "envs", "dev.sqlite")); err != nil {
		t.Fatalf("default db was not created: %v", err)
	}
}

func TestRunDBRejectsDatabaseFromDifferentProjectSchema(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	writeProjectWithWidgetField(t, first, "First_Field__c")
	writeProjectWithWidgetField(t, second, "Second_Field__c")
	dbPath := filepath.Join(first, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--project", first}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first inspect exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--project", second, "--db", dbPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("second inspect unexpectedly succeeded stdout=%q", stdout.String())
	}
	got := stderr.String()
	for _, want := range []string{"database belongs to a different Glade project schema", filepath.ToSlash(dbPath), "glade db inspect --project"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr missing %q:\n%s", want, got)
		}
	}
}

func TestRunDBRefreshesSameProjectSchemaDrift(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "First_Field__c")
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first inspect exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	firstFieldPath := filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/First_Field__c.field-meta.xml")
	if err := os.Remove(firstFieldPath); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/Second_Field__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Second_Field__c</fullName><label>Second_Field__c</label><type>Text</type></CustomField>`)

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second inspect exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "describe", "--project", root, "--json", "Widget__c"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("describe exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"name": "Second_Field__c"`) {
		t.Fatalf("describe did not include refreshed field:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), `"name": "First_Field__c"`) {
		t.Fatalf("describe still included stale field:\n%s", stdout.String())
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("default db missing after refresh: %v", err)
	}
}

func TestRunDBRejectsSchemaRefreshThatWouldDropRecords(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Label__c")
	dbPath := filepath.Join(root, ".glade", "envs", "dev.sqlite")
	fixturePath := filepath.Join(root, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Widget__c","records":[{"alias":"widget","fields":{"Name":{"kind":"string","string":"Kept"}}}]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--project", root, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	if err := os.RemoveAll(filepath.Join(root, "force-app/main/default/objects/Widget__c")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Other__c/Other__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Other</label><pluralLabel>Others</pluralLabel></CustomObject>`)

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--project", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("inspect unexpectedly refreshed destructive schema change; stdout=%q", stdout.String())
	}
	got := stderr.String()
	for _, want := range []string{"schema refresh would drop local records", "Widget__c: 1", filepath.ToSlash(dbPath), "glade db export"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr missing %q:\n%s", want, got)
		}
	}
}

func TestRunDBQueryJSONUsesSOQLRuntimeAndLimit(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")
	fixturePath := filepath.Join(dir, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Account","records":[
    {"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}},
    {"alias":"globex","fields":{"Name":{"kind":"string","string":"Globex"}}}
  ]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "query", "--db", dbPath, "--project", ".", "--json", "--limit", "1", "SELECT Id, Name FROM Account ORDER BY Name"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("query exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	var got struct {
		Query     string           `json:"query"`
		TotalSize int              `json:"totalSize"`
		Done      bool             `json:"done"`
		Columns   []string         `json:"columns"`
		Records   []map[string]any `json:"records"`
	}
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "db query", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected query envelope: %#v\n%s", env, stdout.String())
	}
	if got.Query != "SELECT Id, Name FROM Account ORDER BY Name" {
		t.Fatalf("query = %q", got.Query)
	}
	if got.TotalSize != 1 || !got.Done {
		t.Fatalf("totalSize/done = %d/%v", got.TotalSize, got.Done)
	}
	if fmt.Sprint(got.Columns) != "[Id Name]" {
		t.Fatalf("columns = %#v", got.Columns)
	}
	if len(got.Records) != 1 {
		t.Fatalf("records = %#v", got.Records)
	}
	if got.Records[0]["Name"] != "Acme" {
		t.Fatalf("first record = %#v", got.Records[0])
	}
	if id, ok := got.Records[0]["Id"].(string); !ok || id == "" {
		t.Fatalf("record Id = %#v", got.Records[0]["Id"])
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "query", "--db", dbPath, "--project", ".", "--json", "SELECT Name, Id FROM Account ORDER BY Name LIMIT 1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ordered query exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db query", &got)
	if fmt.Sprint(got.Columns) != "[Name Id]" {
		t.Fatalf("ordered query columns = %#v", got.Columns)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "query", "--db", dbPath, "--project", ".", "--json", "SELECT FIELDS(ALL) FROM Account LIMIT 1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("FIELDS query exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db query", &got)
	if len(got.Columns) == 1 && got.Columns[0] == "FIELDS(ALL)" {
		t.Fatalf("FIELDS query columns were not expanded: %#v\n%s", got.Columns, stdout.String())
	}
	if !containsString(got.Columns, "Id") || !containsString(got.Columns, "Name") {
		t.Fatalf("FIELDS query columns missing record fields: %#v\n%s", got.Columns, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "query", "--db", dbPath, "--project", ".", "--json", "SELECT COUNT() FROM Account"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("COUNT query exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db query", &got)
	if fmt.Sprint(got.Columns) != "[expr0]" {
		t.Fatalf("COUNT query columns should match aggregate record payload: %#v\n%s", got.Columns, stdout.String())
	}
}

func TestRunDBQueryAllJSONIncludesDeletedRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")
	fixturePath := filepath.Join(dir, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Account","records":[
    {"alias":"live","fields":{"Name":{"kind":"string","string":"Live"}}},
    {"alias":"deleted","fields":{"Name":{"kind":"string","string":"Deleted"}}}
  ]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	store, org, err := openDBStore(dbPath, ".")
	if err != nil {
		t.Fatal(err)
	}
	account := org.Objects["Account"]
	for id, record := range account.Records {
		if record.Fields["Name"].String == "Deleted" {
			record.System.IsDeleted = true
			account.Records[id] = record
		}
	}
	org.Objects["Account"] = account
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "query", "--db", dbPath, "--project", ".", "--json", "--query-all", "SELECT Id, Name, IsDeleted FROM Account ORDER BY Name"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("query exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got struct {
		TotalSize int              `json:"totalSize"`
		Records   []map[string]any `json:"records"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db query", &got)
	if got.TotalSize != 2 || len(got.Records) != 2 {
		t.Fatalf("query-all payload = %#v", got)
	}
	if got.Records[0]["Name"] != "Deleted" || got.Records[0]["IsDeleted"] != true {
		t.Fatalf("deleted row = %#v", got.Records[0])
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestRunDBDescribeJSONListsObjectsWithRecordCounts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")
	fixturePath := filepath.Join(dir, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "describe", "--db", dbPath, "--project", ".", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("describe exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	var got struct {
		Objects []struct {
			Name      string `json:"name"`
			Label     string `json:"label"`
			KeyPrefix string `json:"keyPrefix"`
			Records   int    `json:"records"`
		} `json:"objects"`
	}
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "db describe", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected describe envelope: %#v\n%s", env, stdout.String())
	}
	for _, object := range got.Objects {
		if object.Name == "Account" {
			if object.Label != "Account" || object.KeyPrefix != "001" || object.Records != 1 {
				t.Fatalf("Account describe = %#v", object)
			}
			return
		}
	}
	t.Fatalf("Account missing from objects: %#v", got.Objects)
}

func TestRunDBDescribeObjectJSONIncludesFields(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "describe", "--db", dbPath, "--project", ".", "--json", "Account"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("describe object exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	var got struct {
		Name      string `json:"name"`
		Label     string `json:"label"`
		KeyPrefix string `json:"keyPrefix"`
		Fields    []struct {
			Name        string   `json:"name"`
			Label       string   `json:"label"`
			Type        string   `json:"type"`
			DisplayType string   `json:"displayType"`
			ReferenceTo []string `json:"referenceTo"`
		} `json:"fields"`
	}
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "db describe", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected describe object envelope: %#v\n%s", env, stdout.String())
	}
	if got.Name != "Account" || got.Label != "Account" || got.KeyPrefix != "001" {
		t.Fatalf("object describe = %#v", got)
	}
	if !strings.Contains(stdout.String(), `"referenceTo": []`) {
		t.Fatalf("non-reference fields should encode referenceTo as []:\n%s", stdout.String())
	}
	for _, field := range got.Fields {
		if field.Name == "OwnerId" {
			if field.Type != "REFERENCE" || field.DisplayType != "REFERENCE" || fmt.Sprint(field.ReferenceTo) != "[User]" {
				t.Fatalf("OwnerId field = %#v", field)
			}
			return
		}
	}
	t.Fatalf("OwnerId missing from fields: %#v", got.Fields)
}

func TestRunDBImportSFListsObjects(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	dir := t.TempDir()
	fakeSF := filepath.Join(dir, "sf")
	writeTestFile(t, fakeSF, `#!/bin/sh
if [ "$1" = "sobject" ] && [ "$2" = "list" ]; then
  printf '{"status":0,"result":["Account","Invoice__c"]}'
  exit 0
fi
echo "unexpected sf args: $*" >&2
exit 1
`)
	if err := os.Chmod(fakeSF, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "import", "sf", "--target-org", "devhub", "--list-objects", "--category", "all", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("import list exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got struct {
		TargetOrg string   `json:"targetOrg"`
		Category  string   `json:"category"`
		Objects   []string `json:"objects"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("list JSON decode: %v\n%s", err, stdout.String())
	}
	if got.TargetOrg != "devhub" || got.Category != "all" || strings.Join(got.Objects, ",") != "Account,Invoice__c" {
		t.Fatalf("list payload = %#v", got)
	}
}

func TestRunDBImportSFSeedsDatabase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")
	fakeSF := filepath.Join(dir, "sf")
	writeTestFile(t, fakeSF, `#!/bin/sh
case "$*" in
  *"sobject list"*)
    printf '{"status":0,"result":["Account","Invoice__c"]}'
    exit 0
    ;;
  *"data query"*Account*)
    printf '{"status":0,"result":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Account"},"Id":"001000000000123AAA","Name":"Acme","NumberOfEmployees":7,"IsDeleted":false}]}}'
    exit 0
    ;;
esac
echo "unexpected sf args: $*" >&2
exit 1
`)
	if err := os.Chmod(fakeSF, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "import", "sf", "--target-org", "devhub", "--db", dbPath, "--project", ".", "--object", "Account", "--fields", "Id,Name,NumberOfEmployees,IsDeleted", "--limit", "2", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("import exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var inspect struct {
		ByObject map[string]int `json:"byObject"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &inspect); err != nil {
		t.Fatalf("inspect JSON decode: %v\n%s", err, stdout.String())
	}
	if inspect.ByObject["Account"] != 1 {
		t.Fatalf("inspect payload = %#v", inspect)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "query", "--db", dbPath, "--project", ".", "--json", "SELECT Id, Name, NumberOfEmployees, IsDeleted FROM Account"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("query exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var query struct {
		Records []map[string]any `json:"records"`
	}
	decodeCLIEnvelopeData(t, stdout.Bytes(), "db query", &query)
	if len(query.Records) != 1 || query.Records[0]["Name"] != "Acme" || query.Records[0]["NumberOfEmployees"] != float64(7) {
		t.Fatalf("query payload = %#v", query)
	}
}

func TestRunDBSeedWizardAndProgress(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")
	fixturePath := filepath.Join(dir, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"glade.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--wizard", "--db", dbPath, "--project", dir, fixturePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("wizard exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"glade db seed", "--db", dbPath, "--project", dir, fixturePath, "--progress"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("wizard output missing %q:\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "seed", "--db", dbPath, "--project", dir, fixturePath, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "db seed") || !strings.Contains(stderr.String(), "done") {
		t.Fatalf("progress stderr = %q", stderr.String())
	}
}

func TestRunDBRejectsFlagTokenAsDBValue(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "inspect", "--db", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--db requires a path") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "--json")); err == nil {
		t.Fatalf("db file named --json was created")
	}
}

func TestRunDBRejectsUnknownFlagBeforeFixture(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "glade.db")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, "--bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown flag "--bogus"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReportRejectsFlagTokenAsRunsDirValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "show", "latest", "--runs-dir", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--runs-dir requires a path") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReportListRejectsIgnoredOutputFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "list", "--output", "report.zip"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown flag "--output"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func writePerformanceScanProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Risk.cls"), `
public class Risk {
    @AuraEnabled
    public static List<Account> load(List<Id> ids) {
        List<Account> out = new List<Account>();
        for (Id idValue : ids) {
            out.add([SELECT Id, Name FROM Account WHERE Id = :idValue]);
        }
        return out;
    }
}
`)
	return root
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeProjectWithWidgetField(t *testing.T, root, fieldName string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"61.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/WidgetService.cls"), "public class WidgetService { public void run() {} }")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/Widget__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel></CustomObject>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/"+fieldName+".field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+fieldName+`</fullName><label>`+fieldName+`</label><type>Text</type></CustomField>`)
}

func writeTestProject(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/MathUtilTest.cls"), `
@isTest
private class MathUtilTest {
  @isTest static void adds() {
    System.assertEquals(3, MathUtil.add(1, 2));
  }
}
`)
}
