package oaercli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/watch"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "oaer "+Version+"\n" {
		t.Fatalf("stdout = %q", got)
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
	output := filepath.Join(root, "out", "znu.oaer-package.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "build", "--project", root, "--namespace", "znu", "--version", "test-version", "--output", output, "--json"}, &stdout, &stderr)
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
	if artifact.Namespace != "znu" || artifact.Version != "test-version" || artifact.SourceAPIVersion != "61.0" || artifact.SourceHash == "" {
		t.Fatalf("artifact metadata = %#v", artifact)
	}
	if len(artifact.ApexTypes) != 1 || artifact.ApexTypes[0].Name != "Address" || artifact.ApexTypes[0].Namespace != "znu" {
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
	if !strings.Contains(stdout.String(), `"namespace": "znu"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatValidate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "validate", "../../docs/fixtures/parser-smoke.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "parser-smoke.json: ok") {
		t.Fatalf("stdout did not include fixture status: %q", stdout.String())
	}
}

func TestRunCompatRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "run", "../../docs/fixtures/parser-smoke.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "parser-smoke.json: parse ok=true") {
		t.Fatalf("stdout did not include fixture run status: %q", stdout.String())
	}
}

func TestRunCompatMVP(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "mvp"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "MVP readiness: not ready") || !strings.Contains(stdout.String(), "full-featured aer-parity MVP") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatMVPRequireReadyFailsWhilePreview(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "mvp", "--require-ready"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "MVP readiness: not ready") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "MVP readiness gate failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCompatMatrixJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "matrix", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ready": false`) || !strings.Contains(stdout.String(), `"requiredForMVP": true`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatPostParity(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController">{!$Label.EditTitle}</apex:page>`)
	writeTestFile(t, filepath.Join(root, "src/reports/Sales/Pipeline.report-meta.xml"), `<Report><name>Pipeline</name></Report>`)
	writeTestFile(t, filepath.Join(root, "src/dashboards/Sales/Pipeline.dashboard-meta.xml"), `<Dashboard><title>Pipeline</title></Dashboard>`)
	writeTestFile(t, filepath.Join(root, "src/classes/UsesPlatform.cls"), `public class UsesPlatform {
  void run() {
    PageReference p = Page.Edit;
    System.debug(Label.Save);
  }
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "post-parity", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Post-parity readiness: not ready") || !strings.Contains(stdout.String(), "Reports: 1") || !strings.Contains(stdout.String(), "Dashboards: 1") || !strings.Contains(stdout.String(), "Status counts:") || !strings.Contains(stdout.String(), "Surfaces by area:") || !strings.Contains(stdout.String(), "visualforce.controller-test") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatPostParityJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src/lwc/menu/menu.js"), `import run from '@salesforce/apex/MenuController.run';`)
	writeTestFile(t, filepath.Join(root, "src/reports/Sales/Pipeline.report-meta.xml"), `<Report><name>Pipeline</name></Report>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "post-parity", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"target": "legacy-project local test readiness"`,
		`"ready": false`,
		`"capability": "lwc.controller-test"`,
		`"reports": 1`,
		`"dashboards": 0`,
		`"statusCounts"`,
		`"areas"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
}

func TestRunCompatLocalTestsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "local-tests", "--project", "../../testdata/local-tests/basic", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"target": "local Apex test execution readiness"`,
		`"ready": false`,
		`"pass": 1`,
		`"assertFail": 1`,
		`"unsupported": 1`,
		`"phases"`,
		`"name": "load_start"`,
		`"class": "PassingTest"`,
		`"class": "FailingTest"`,
		`"class": "UnsupportedTest"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
}

func TestRunCompatLocalTestsBlockersOnlyAndFilters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"compat", "local-tests",
		"--project", "../../testdata/local-tests/basic",
		"--class", "FailingTest",
		"--method", "fails",
		"--blockers-only",
		"--top-failures", "1",
		"--max-failure-groups", "1",
		"--timeout", "5000",
		"--profile-on-timeout",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"total": 1`) || !strings.Contains(stdout.String(), `"assertFail": 1`) || !strings.Contains(stdout.String(), `"topFailures"`) || !strings.Contains(stdout.String(), `"triageStopped": true`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "PassingTest") || strings.Contains(stdout.String(), "UnsupportedTest") {
		t.Fatalf("stdout included unfiltered tests: %q", stdout.String())
	}
}

func TestRunCompatPostParityRequireReadyFails(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController"/>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "post-parity", "--project", root, "--require-ready"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "Post-parity readiness: not ready") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "post-parity readiness gate failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCompatServerExamplesAcceptsFilterFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "example-projects", "alpha-pkg-develop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "example-projects", "beta-pkg-develop"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"compat", "server-examples",
		"--project", root,
		"--project-filter", "alpha",
		"--route", "event",
		"--probe", "apexrest",
		"--outcome", "unsupported",
		"--blockers-only",
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{`"projects"`, "alpha-pkg-develop"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
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
	if !strings.Contains(stdout.String(), `"code": "OAERSCHEMA001"`) || !strings.Contains(stdout.String(), `"types": 1`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatExamples(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/classes/MyClass.cls"), `public class MyClass {
		@AuraEnabled public static String doWork() { return 'ok'; }
	}`)
	writeTestFile(t, filepath.Join(root, "force-app/classes/MyTest.cls"), `@isTest private class MyTest {
		@isTest static void testIt() { System.assert(true); }
	}`)
	writeTestFile(t, filepath.Join(root, "force-app/triggers/MyTrigger.trigger"), `trigger MyTrigger on Account (before insert) { if (Trigger.isBefore) {} }`)
	writeTestFile(t, filepath.Join(root, "force-app/pages/MyPage.page"), `<apex:page/>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "examples", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"project:",
		"classes: 2",
		"triggers: 1",
		"test classes: 1",
		"vf pages: 1",
		"annotations:",
		"AuraEnabled",
		"isTest",
		"soql features:",
		"dml features:",
		"namespace refs:",
		"System",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q: %q", want, out)
		}
	}
}

func TestRunCompatExamplesJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/classes/A.cls"), `public class A {}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "examples", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"projects"`,
		`"name"`,
		`"counts"`,
		`"apexClasses"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
}

func TestRunCompatExamplesOutputAndCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/classes/A.cls"), `public class A {}`)
	path := filepath.Join(t.TempDir(), "examples.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "examples", "--project", root, "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"projects"`) {
		t.Fatalf("output file missing projects key")
	}

	// Check mode should succeed against identical file.
	var stdout2, stderr2 bytes.Buffer
	code = Run(context.Background(), []string{"compat", "examples", "--project", root, "--check", path}, &stdout2, &stderr2)
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0; stderr=%q", code, stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "ok") {
		t.Fatalf("check stdout missing ok: %q", stdout2.String())
	}
}

func TestRunCompatPostParityMarkdownOutputAndCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController">{!$Label.EditTitle}</apex:page>`)
	path := filepath.Join(t.TempDir(), "post-parity.md")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "post-parity", "--project", root, "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Post-Parity Readiness",
		"## Status Counts",
		"## Top Blockers",
		"## Surfaces By Area",
		"`visualforce.controller-test`",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("post-parity markdown missing %q: %q", want, string(content))
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "post-parity", "--project", root, "--check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatDashboard(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "dashboard"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Compatibility Dashboard") || !strings.Contains(stdout.String(), "`triggers.runtime`") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatDashboardOutputAndCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.md")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "dashboard", "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Compatibility Dashboard") {
		t.Fatalf("dashboard file = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "dashboard", "--check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatGaps(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "gaps"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Known Gaps") || !strings.Contains(stdout.String(), "`apex.sema.body`") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatGapsOutputAndCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known-gaps.md")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "gaps", "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Known Gaps") {
		t.Fatalf("known gaps file = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "gaps", "--check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatStdlib(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "stdlib"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Standard Library Coverage") || !strings.Contains(stdout.String(), "`String.trim`") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatStdlibJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "stdlib", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"api": "String.trim"`) || !strings.Contains(stdout.String(), `"status": "supported"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatStdlibOutputAndCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdlib.md")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "stdlib", "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Standard Library Coverage") {
		t.Fatalf("stdlib file = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "stdlib", "--check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatDocsInventoryJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_methods_system_string.md"), `# String Class

## Namespace
[System](./apex_namespace_System.md)

## String Methods
### trim()
Removes leading and trailing white space.
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"schemaVersion": 1`,
		`"sourcePath": "apex_methods_system_string.md"`,
		`"namespace": "System"`,
		`"signature": "trim()"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
}

func TestRunCompatDocsInventoryOutputCheckAndDiff(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_methods_system_list.md"), `# List Class

## List Methods
### add(listElement)
Adds an element.
`)
	path := filepath.Join(t.TempDir(), "apex-docs-inventory.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"name": "List"`) {
		t.Fatalf("inventory file = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	writeTestFile(t, filepath.Join(root, "apex_methods_system_map.md"), `# Map Class

## Map Methods
### clear()
Clears the map.
`)
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--diff", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("diff exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"addedDocuments"`) || !strings.Contains(stdout.String(), "apex_methods_system_map.md") {
		t.Fatalf("diff stdout = %q", stdout.String())
	}
}

func TestRunCompatCatalogJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_methods_system_string.md"), `# String Class

## String Methods
### trim()
Removes leading and trailing white space.
`)
	inventoryPath := filepath.Join(t.TempDir(), "inventory.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--output", inventoryPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "catalog", "--inventory", inventoryPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("catalog exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"schemaVersion": 1`,
		`"area": "Core stdlib"`,
		`"symbol": "String.trim"`,
		`"target": "executable-parity"`,
		`"status": "supported"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("catalog stdout missing %q: %q", want, stdout.String())
		}
	}
}

func TestRunCompatCatalogOutputAndCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_connectapi_output_FeedElement.md"), `# FeedElement

## Properties
### body
The feed body.
`)
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "inventory.json")
	catalogPath := filepath.Join(dir, "catalog.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--output", inventoryPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "catalog", "--inventory", inventoryPath, "--output", catalogPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("catalog output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"target": "typed-stub"`) {
		t.Fatalf("catalog file = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "catalog", "--inventory", inventoryPath, "--check", catalogPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("catalog check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatProductNamespacesOutputAndCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_connectapi_output_FeedElement.md"), `# FeedElement

## Properties
### body
The feed body.
`)
	writeTestFile(t, filepath.Join(root, "apex_class_Metadata_DeployContainer.md"), `# DeployContainer Class

## Methods
### addMetadata(metadata)
Adds metadata.
`)
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "inventory.json")
	catalogPath := filepath.Join(dir, "catalog.json")
	reportPath := filepath.Join(dir, "product-namespaces.json")
	markdownPath := filepath.Join(dir, "product-namespaces.md")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--output", inventoryPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "catalog", "--inventory", inventoryPath, "--output", catalogPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("catalog exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "product-namespaces", "--catalog", catalogPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespaces exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"namespace": "ConnectApi"`,
		`"namespace": "Metadata"`,
		`"target": "typed-stub"`,
		`"executionPolicy": "return deterministic unsupported diagnostics until a local model is chosen"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("product namespaces stdout missing %q: %q", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "product-namespaces", "--catalog", catalogPath, "--output", reportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespaces output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "product-namespaces", "--catalog", catalogPath, "--check", reportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespaces check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "product-namespaces", "--source", root, "--output", markdownPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespaces markdown output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Product Namespace Coverage") || !strings.Contains(string(content), "| ConnectApi | `typed-stub` | `unknown` |") {
		t.Fatalf("product namespaces markdown = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "product-namespaces", "--inventory", inventoryPath, "--check", markdownPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespaces markdown check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
}

func TestRunCompatProductNamespacesSymbolsGoOutputAndCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_connectapi_output_FeedElement.md"), `# FeedElement

## Properties
### body
The feed body.
`)
	writeTestFile(t, filepath.Join(root, "apex_class_cache_OrgPartition.md"), `# OrgPartition Class

## Namespace
[cache](./apex_namespace_cache.md)

## Methods
### docOnly(value)
Docs-only method.
`)
	dir := t.TempDir()
	symbolsPath := filepath.Join(dir, "product_namespace_symbols_generated.go")
	toolingPath := filepath.Join(dir, "tooling.json")
	writeTestFile(t, toolingPath, `{
  "publicDeclarations": {
    "cache": {
      "OrgPartition": {
        "methods": [{
          "name": "get",
          "returnType": "System.Object",
          "isStatic": true,
          "parameters": [{"name": "key", "type": "System.String"}]
        }]
      }
    }
  }
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "product-namespaces", "--source", root, "--tooling-completions", toolingPath, "--symbols-go", "--output", symbolsPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespace symbols output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(symbolsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`var productNamespaceSymbolSpecs = []StandardSymbolSpec`,
		`Name: "Cache.OrgPartition"`,
		`{Name: "docOnly", ReturnType: "Object", Parameters: []string{"Object"}}`,
		`{Name: "get", ReturnType: "Object", Parameters: []string{"String"}, Static: true}`,
		`Name: "ConnectApi.FeedElement"`,
		`{Name: "body", Type: "Object"}`,
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("symbols output missing %q:\n%s", want, string(content))
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "product-namespaces", "--source", root, "--tooling-completions", toolingPath, "--symbols-go", "--check", symbolsPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product namespace symbols check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatToolingFixtures(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "tooling-report.json")
	writeTestFile(t, reportPath, `{
  "schemaVersion": 1,
  "orgAlias": "fixture",
  "snippets": [{
    "id": "system-test",
    "source": "System.debug('ok');",
    "cli": "sf",
    "status": 0,
    "compiled": true,
    "executed": true,
    "success": true,
    "rawShape": {"payloadKey": "result"},
    "fixture": {
      "commandKind": "tooling-execute-anonymous",
      "compiled": true,
      "executed": true,
      "success": true
    }
  }]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "tooling-fixtures", reportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("tooling fixtures exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok (1 snippets)") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "tooling-fixtures", reportPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("tooling fixtures json exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"snippets": 1`) {
		t.Fatalf("json stdout = %q", stdout.String())
	}
}

func TestRunCompatSalesforceCoverageOutputAndCheck(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_methods_system_string.md"), `# String Class

## String Methods
### trim()
Removes leading and trailing white space.
`)
	writeTestFile(t, filepath.Join(root, "apex_connectapi_output_FeedElement.md"), `# FeedElement

## Properties
### body
The feed body.
`)
	reportPath := filepath.Join(t.TempDir(), "salesforce-coverage.md")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "salesforce-coverage", "--source", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("salesforce coverage json exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"schemaVersion": 1`,
		`"area": "Core stdlib"`,
		`"area": "Product namespaces"`,
		`"supported": 1`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("salesforce coverage stdout missing %q: %q", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "salesforce-coverage", "--source", root, "--output", reportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("salesforce coverage output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Salesforce Coverage Manifest") {
		t.Fatalf("report file = %q", string(content))
	}
	jsonReportPath := filepath.Join(t.TempDir(), "salesforce-coverage.json")
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "salesforce-coverage", "--source", root, "--output", jsonReportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("salesforce coverage json output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	jsonContent, err := os.ReadFile(jsonReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonContent), `"schemaVersion": 1`) || !strings.Contains(string(jsonContent), `"areas"`) {
		t.Fatalf("json report file = %q", string(jsonContent))
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "salesforce-coverage", "--source", root, "--check", jsonReportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("salesforce coverage json check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "salesforce-coverage", "--source", root, "--check", reportPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("salesforce coverage check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatSalesforceCoverageWithToolingSymbols(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_class_System_Managed.md"), `# Managed Class

## Methods
### doIt()
Does it.
`)
	dir := t.TempDir()
	symbolsPath := filepath.Join(dir, "symbols.json")
	writeTestFile(t, symbolsPath, `{
  "records": [{
    "Name": "Managed",
    "NamespacePrefix": "",
    "SymbolTable": {
      "methods": [{"name": "doIt"}]
    }
  }]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "salesforce-coverage", "--source", root, "--tooling-symbols", symbolsPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("salesforce coverage tooling symbols exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"symbolTableClasses": 1`,
		`"symbolTableMethods": 1`,
		`"catalogSystemEntriesInTooling": 2`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("salesforce coverage stdout missing %q: %q", want, stdout.String())
		}
	}
}

func TestRunCompatStandardObjectsOutputAndCheck(t *testing.T) {
	dir := t.TempDir()
	markdownPath := filepath.Join(dir, "standard-objects.md")
	jsonPath := filepath.Join(dir, "standard-objects.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "standard-objects", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("standard objects json exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"schemaVersion": 1`,
		`"object": "Account"`,
		`"keyPrefix": "001"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("standard objects stdout missing %q: %q", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "standard-objects", "--output", markdownPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("standard objects markdown output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Standard Object Coverage") || !strings.Contains(string(content), "`Account`") {
		t.Fatalf("markdown = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "standard-objects", "--output", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("standard objects json output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "standard-objects", "--check", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("standard objects json check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatStubBehaviorJSONAndCheck(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "stub-behavior", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub behavior json exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"schemaVersion": 1`,
		`"target": "standard platform stub behavior"`,
		`"implemented":`,
		`"passive-default":`,
		`"unsupported":`,
		`"unknown":`,
		`"id": "String.trim(`,
		`"status": "implemented"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stub behavior stdout missing %q: %q", want, stdout.String())
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "stub-behavior.json")
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "stub-behavior", "--output", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub behavior output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "stub-behavior", "--check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub behavior check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatStubInventoryOutputAndCheck(t *testing.T) {
	sourceRoot := t.TempDir()
	writeTestFile(t, filepath.Join(sourceRoot, "apex-system-stubs", "System", "String.cls"), `global class String {
    global static String valueOf(Object value) { return null; }
    global String trim() { return null; }
}`)
	writeTestFile(t, filepath.Join(sourceRoot, "apex-sobject-stubs", "Account.cls"), `global class Account extends SObject {
    public static SObjectFields Fields { get; private set; }
    global class SObjectFields {
        public SObjectField Name;
    }
    /** Parent relationship for OwnerId **/
    global User Owner { get; private set; }
    /** Child relationship Contacts **/
    global List<Contact> Contacts { get; private set; }
    global String Name { get; set; }
}`)
	dir := t.TempDir()
	markdownPath := filepath.Join(dir, "stub-inventory.md")
	jsonPath := filepath.Join(dir, "stub-inventory.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "stub-inventory", "--source", sourceRoot, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub inventory json exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"schemaVersion": 1`,
		`"systemStubClasses": 1`,
		`"sobjectStubClasses": 1`,
		`"systemSourceMissingGeneratedTypeCount": 0`,
		`"sobjectSourceMissingActiveCount": 0`,
		`"sobjectFieldMissingActiveCount": 0`,
		`"sobjectFieldMissingFeatureGatedCount": 0`,
		`"sobjectFieldMissingSupportedFeatureCount": 0`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stub inventory stdout missing %q: %q", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "stub-inventory", "--source", sourceRoot, "--output", markdownPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub inventory markdown output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# Stub Inventory") || !strings.Contains(string(content), "System stub classes: 1") {
		t.Fatalf("markdown = %q", string(content))
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "stub-inventory", "--source", sourceRoot, "--output", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub inventory json output exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "stub-inventory", "--source", sourceRoot, "--check", jsonPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stub inventory json check exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCompatEvidenceJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apex_methods_system_string.md"), `# String Class

## String Methods
### trim()
Removes leading and trailing white space.
`)
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "inventory.json")
	catalogPath := filepath.Join(dir, "catalog.json")
	fixturePath := filepath.Join(dir, "fixture.json")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "docs-inventory", "--source", root, "--output", inventoryPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "catalog", "--inventory", inventoryPath, "--output", catalogPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("catalog exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	writeTestFile(t, fixturePath, `{
  "name": "string-evidence",
  "evidence": [{"symbol": "String.trim", "kind": "exec"}],
  "source": [{"path": "anonymous.apex", "content": "System.debug('x');"}],
  "command": {"kind": "exec", "args": ["System.debug('x');"]},
  "expected": {"stdout": "x\n", "result": {"debug": ["x"], "ok": true}}
}`)

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "evidence", "--catalog", catalogPath, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evidence exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		`"fixtures": 1`,
		`"evidence": 1`,
		`"symbol": "String.trim"`,
		`"covered"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("evidence stdout missing %q: %q", want, stdout.String())
		}
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
	if !strings.Contains(stdout.String(), `"name": "Hello"`) || !strings.Contains(stdout.String(), `"name": "Thing__c"`) {
		t.Fatalf("stdout did not include symbols and schema: %q", stdout.String())
	}
}

func TestRunInspectGapsJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/pages/Edit.page"), `<apex:page controller="EditController"><apex:stylesheet value="{!URLFOR($Resource.Resources, 'site.css')}"/></apex:page>`)
	writeTestFile(t, filepath.Join(root, "force-app/main/lwc/widget/widget.js"), `import save from '@salesforce/apex/WidgetController.save';`)
	writeTestFile(t, filepath.Join(root, "force-app/main/workflows/Account.workflow-meta.xml"), `<Workflow><rules>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "gaps", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"capability": "visualforce.controller-test"`) ||
		!strings.Contains(out, `"capability": "lwc.controller-test"`) ||
		!strings.Contains(out, `"capability": "workflow.save-order"`) ||
		!strings.Contains(out, `"topBlockers"`) {
		t.Fatalf("stdout did not include project gap findings: %q", out)
	}
}

func TestRunInspectGapsText(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController"/>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "gaps", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "testBlockingFindings:") || !strings.Contains(out, "visualforce.controller-test") {
		t.Fatalf("stdout did not include text report: %q", out)
	}
}

func TestRunInspectGapsLegacyAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src/pages/Edit.page"), `<apex:page controller="EditController"/>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "post-parity", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "visualforce.controller-test") {
		t.Fatalf("stdout did not include alias report: %q", stdout.String())
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
	if !strings.Contains(stdout.String(), "OAERSEMA002") {
		t.Fatalf("stdout did not include semantic diagnostic: %q", stdout.String())
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
	if !strings.Contains(string(junit), `<testsuites name="oaer test" tests="1" failures="0" errors="0" skipped="0"`) {
		t.Fatalf("junit output = %q", string(junit))
	}
}

func TestRunTestCompatJSON(t *testing.T) {
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
	code := Run(context.Background(), []string{"test", "--project", root, "--compat-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"target": "local Apex test execution readiness"`) ||
		!strings.Contains(stdout.String(), `"ready": true`) ||
		!strings.Contains(stdout.String(), `"outcome": "pass"`) {
		t.Fatalf("stdout = %q", stdout.String())
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
	dbPath := filepath.Join(dir, "oaer.db")
	fixturePath := filepath.Join(dir, "fixture.json")
	writeTestFile(t, fixturePath, `{
  "version":"oaer.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"db", "seed", "--db", dbPath, fixturePath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("seed exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"schemaVersion": 1`) || !strings.Contains(stdout.String(), `"Account": 1`) || !strings.Contains(stdout.String(), `"users": 1`) {
		t.Fatalf("seed stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "inspect", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inspect exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "schemaVersion: 1") || !strings.Contains(stdout.String(), "Account: 1") || !strings.Contains(stdout.String(), "User: 1") {
		t.Fatalf("inspect stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "export", "--db", dbPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("export exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version": "oaer.storage.v1"`) || !strings.Contains(stdout.String(), `"Acme"`) {
		t.Fatalf("export stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"db", "reset", "--db", dbPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reset exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"Account": 0`) || !strings.Contains(stdout.String(), `"users": 1`) {
		t.Fatalf("reset stdout = %q", stdout.String())
	}
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
