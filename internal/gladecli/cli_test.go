package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

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

func TestRunVersionJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if got["version"] != Version {
		t.Fatalf("version = %q, want %q", got["version"], Version)
	}
	for _, key := range []string{"go", "os", "arch"} {
		if got[key] == "" {
			t.Fatalf("missing %s in %#v", key, got)
		}
	}
}

func TestRunDoctorReportsParser(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "parser") {
		t.Fatalf("doctor output missing parser line:\n%s", out)
	}
	// This test binary is built with CGO, so the parser must be available.
	if !strings.Contains(out, "ok (tree-sitter)") {
		t.Fatalf("expected parser ok in CGO build, got:\n%s", out)
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

func TestCompatIsNotPublicCommand(t *testing.T) {
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

func TestRunCommandHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "test flag help",
			args: []string{"test", "--help"},
			want: []string{"Usage:", "glade test", "glade test serve", "clear-cache", "startup.gob", "--no-cache", "--connect", "--daemon"},
		},
		{
			name: "test serve help",
			args: []string{"test", "serve", "--help"},
			want: []string{"Serve flags:", "startup.gob", "--no-warm"},
		},
		{
			name: "parse flag help",
			args: []string{"parse", "--help"},
			want: []string{"Usage:", "glade parse <paths...>", "--json", "Examples:"},
		},
		{
			name: "help check",
			args: []string{"help", "check"},
			want: []string{"Usage:", "glade check", "--project <root>", "--progress-json", "Examples:"},
		},
		{
			name: "schema subcommand help",
			args: []string{"schema", "load", "--help"},
			want: []string{"Usage:", "glade schema load", "--project <root>", "--progress"},
		},
		{
			name: "completion help",
			args: []string{"help", "completion"},
			want: []string{"Usage:", "glade completion bash|zsh|fish", "Examples:"},
		},
		{
			name: "help test",
			args: []string{"help", "test"},
			want: []string{"Usage:", "glade test", "glade test serve", "clear-cache", "startup.gob", "--no-cache", "--connect", "--daemon"},
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
		"version doctor parse inspect schema check exec",
		"--project",
		"--progress-json",
		"test serve clear-cache",
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
		"-l progress-json",
		"-a 'serve clear-cache'",
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
	code := Run(context.Background(), nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"package",
		"Build managed package artifacts.",
		"dap",
		"Run the Debug Adapter Protocol server over stdio.",
		"server",
		"Start the local Salesforce-compatible API baseline.",
		"playground",
		"Start the local Apex playground web UI.",
		"db",
		"Seed, reset, export, and inspect a persistent local database.",
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

func TestRunEditorInstallVSCodeUsesVSIX(t *testing.T) {
	vsix := filepath.Join(t.TempDir(), "vscode-glade.vsix")
	if err := os.WriteFile(vsix, []byte("vsix"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ranName string
	var ranArgs []string
	restore := stubEditorCommandDeps(t,
		func(name string) (string, error) {
			if name != "code" {
				t.Fatalf("looked up %q, want code", name)
			}
			return "/usr/local/bin/code", nil
		},
		func(ctx context.Context, name string, args ...string) ([]byte, error) {
			ranName = name
			ranArgs = append([]string(nil), args...)
			return []byte("installed\n"), nil
		},
	)
	defer restore()

	var stdout bytes.Buffer
	if err := runEditor(context.Background(), []string{"install", "vscode", "--vsix", vsix, "--force"}, &stdout); err != nil {
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

func TestRunEditorDoctorVSCodeReportsPaths(t *testing.T) {
	restore := stubEditorCommandDeps(t,
		func(name string) (string, error) {
			switch name {
			case "code":
				return "/usr/local/bin/code", nil
			case "glade":
				return "/Users/matt/.local/bin/glade", nil
			default:
				return "", os.ErrNotExist
			}
		},
		func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, nil
		},
	)
	defer restore()

	var stdout bytes.Buffer
	if err := runEditor(context.Background(), []string{"doctor", "vscode"}, &stdout); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{
		"editor: code (/usr/local/bin/code)",
		"glade: /Users/matt/.local/bin/glade",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func stubEditorCommandDeps(t *testing.T, lookPath func(string) (string, error), run func(context.Context, string, ...string) ([]byte, error)) func() {
	t.Helper()
	origLookPath := editorCommandLookPath
	origRun := editorCommandRun
	editorCommandLookPath = lookPath
	editorCommandRun = run
	return func() {
		editorCommandLookPath = origLookPath
		editorCommandRun = origRun
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
		"Project: " + root,
		"Package dirs: 1",
		"Apex classes: 2",
		"Apex tests: 1",
		"Metadata: loaded",
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
	for _, want := range []string{"Watching ", "Strategy: affected tests", "1 passed", "Report: "} {
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
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"NU","sourceApiVersion":"61.0","packageDirectories":[{"path":"force-app","default":true}]}`)
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
	if !strings.Contains(stdout.String(), `"namespace": "pkg"`) {
		t.Fatalf("stdout = %q", stdout.String())
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
	if !strings.Contains(stdout.String(), `"name": "Hello"`) {
		t.Fatalf("stdout did not include parsed declaration: %q", stdout.String())
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

func TestRunInspectPerformanceJSON(t *testing.T) {
	root := writePerformanceScanProject(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "performance", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"\"schemaVersion\": 1", "\"findings\"", "perf.soql.loop"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunInspectPerformanceMarkdown(t *testing.T) {
	root := writePerformanceScanProject(t)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "performance", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"# Performance Scan", "`perf.soql.loop`"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
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
	if !strings.Contains(stdout.String(), "Thing__c") {
		t.Fatalf("stdout did not include schema object: %q", stdout.String())
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
	if !strings.Contains(stdout.String(), `"diagnostics": 0`) {
		t.Fatalf("stdout did not include zero diagnostics: %q", stdout.String())
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
	if !strings.Contains(stdout.String(), "GLADESEMA002") {
		t.Fatalf("stdout did not include semantic diagnostic: %q", stdout.String())
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
	if !strings.Contains(stdout.String(), "no diagnostics") {
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

func TestRunDebugProfileMarkdown(t *testing.T) {
	logPath := filepath.Join("..", "apexlog", "testdata", "core.log")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"debug", "profile", "--log", logPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "SOQL: 1 queries / 1 rows") {
		t.Fatalf("profile stdout missing SOQL summary: %s", stdout.String())
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

func TestRunDAPLaunchEmitsStopped(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
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
	case <-time.After(time.Second):
		t.Fatal("DAP server did not stop")
	}
}

func TestRunExec(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "Integer x = 1 + 1; System.debug('x=' + x); System.assertEquals(2, x);"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "x=2" {
		t.Fatalf("stdout = %q", stdout.String())
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
	deadline := time.After(5 * time.Second)
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

func TestRunExecTraceFile(t *testing.T) {
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
}

func TestRunExecFailure(t *testing.T) {
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
	if !strings.Contains(stdout.String(), "glade playground:") || !strings.Contains(stdout.String(), "/playground/") {
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
	if !strings.Contains(stdout.String(), `"passed": 1`) {
		t.Fatalf("stdout did not include passed result: %q", stdout.String())
	}
	junit, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(junit), `<testsuites name="glade test" tests="1" failures="0" errors="0" skipped="0"`) {
		t.Fatalf("junit output = %q", string(junit))
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
