package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// Native-consistency controls for the corpus expected-failure environment kinds (pattern).
// The corpus tooling classifies a failing test as an environment failure only when its signature is
// pinned here and Salesforce answers the same way for the same environment shape.
//
//   dependency_missing (per-reference form): unloaded_dependency conformance matrix and
//     TestDependencyGateMissingDependencyPolicy; the signature below pins the message layout.
//   repo_metadata_missing / undefined_metadata: TestCase2701MetadataFixtureTransitions plus the absent/present
//     field control here.
//   repo_defect: compile_defects.tsv rows (native rejects the same source with the same text).
//   locale: date_parse_locale.tsv rows (the user's locale decides Date.parse).
//   feature-gated objects: org_account_contact_availability, org_object_availability and
//     network_configuration conformance fixtures (plain org rejects, feature org accepts).

func readControlTSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "conformance", "expected_env_controls", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("%s: rows=%d err=%v", name, len(rows), err)
	}
	return rows[1:]
}

func runProblems(run testreport.Run) []string {
	var out []string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				out = append(out, string(c.Problem.Type)+"|"+c.Problem.Message)
			}
		}
	}
	return out
}

func envControlProject(t *testing.T, withField bool, body string) salesforceSurfaceProject {
	t.Helper()
	project := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	meta := filepath.Join(project.root, "force-app/main/default")
	writeFile(t, filepath.Join(meta, "objects/Cust__c/Cust__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Cust</label><pluralLabel>Custs</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	if withField {
		project.writeField("Cust__c", "Note__c", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Note__c</fullName><label>Note</label><type>Text</type><length>40</length></CustomField>`)
	}
	project.writeClass("EnvCtl", body)
	project.writeClass("EnvCtlTest", `@isTest private class EnvCtlTest { @isTest static void runs() { EnvCtl.go(); } }`)
	return project
}

// repo_metadata_missing: a field the project never defines is a compile rejection naming the field and its
// object; defining it removes the failure. The corpus rule md_ref_unknown_field matches this shape.
func TestExpectedEnvControlUndefinedFieldAbsentPresent(t *testing.T) {
	body := `public class EnvCtl { public static void go() { Cust__c c = new Cust__c(); c.Note__c = 'x'; System.assertEquals('x', c.Note__c); } }`
	absent := envControlProject(t, false, body).run()
	if got := absent.Summary(); got.Total != 1 || got.Passed != 0 {
		t.Fatalf("absent field: summary=%#v problems=%v", got, runProblems(absent))
	}
	pattern := regexp.MustCompile(`(?m)^ Class EnvCtl : method "go" references unknown field "Note__c" on Cust__c$`)
	if p := strings.Join(runProblems(absent), "\n"); !pattern.MatchString(p) {
		t.Fatalf("absent field: signature changed: %q", p)
	}
	present := envControlProject(t, true, body).run()
	if got := present.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("present field: summary=%#v problems=%v", got, runProblems(present))
	}
}

// repo_defect: the compile text and the unmanaged @Deprecated text equal the org rows (compile_defects.tsv).
func TestExpectedEnvControlRepoDefectsMatchOrgRows(t *testing.T) {
	rows := readControlTSV(t, "compile_defects.tsv")
	want := map[string]string{}
	for _, r := range rows {
		if len(r) != 3 {
			t.Fatalf("row %q", r)
		}
		if prev, ok := want[r[1]]; ok && prev != r[2] {
			t.Fatalf("org rows differ between APIs for %s", r[1])
		}
		want[r[1]] = r[2]
	}
	void := envControlProject(t, false, `public class EnvCtl { public static void helper() {} public static void go() { String s = EnvCtl.helper(); } }`).run()
	if p := strings.Join(runProblems(void), "\n"); !strings.Contains(p, want["void_assigned_to_string"]) {
		t.Fatalf("void assignment: want %q in %q", want["void_assigned_to_string"], p)
	}
	fixed := envControlProject(t, false, `public class EnvCtl { public static String helper() { return 'x'; } public static void go() { String s = EnvCtl.helper(); System.assertEquals('x', s); } }`).run()
	if got := fixed.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("fixed void assignment: summary=%#v problems=%v", got, runProblems(fixed))
	}
	dep := envControlProject(t, false, "@Deprecated\npublic class EnvCtl { public static void go() {} }").run()
	if p := strings.Join(runProblems(dep), "\n"); !strings.Contains(p, want["deprecated_class_unmanaged"]+"") && !strings.Contains(p, "Only managed identifiers can be marked deprecated: EnvCtl") {
		t.Fatalf("deprecated class: want the native text in %q", p)
	}
	if !strings.HasPrefix(want["deprecated_class_unmanaged"], "Only managed identifiers can be marked deprecated: ") {
		t.Fatalf("org row changed: %q", want["deprecated_class_unmanaged"])
	}
	plain := envControlProject(t, false, `public class EnvCtl { public static void go() {} }`).run()
	if got := plain.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("no @Deprecated: summary=%#v problems=%v", got, runProblems(plain))
	}
}

// dependency_missing, per-reference form: a declared package with no source leaves a statically referenced
// class unresolved; the test fails with the unresolved reference, the unloaded package list and the
// original consequence appended. The corpus rule dm_unresolved_ref matches this layout. Supplying the
// class removes the failure. Native rows: testdata/conformance/unloaded_dependency.
func TestExpectedEnvControlDependencyPerReferenceSignature(t *testing.T) {
	build := func(withClass bool) salesforceSurfaceProject {
		project := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"AbsentPkg"}]}]}`)
		project.writeClass("Svc", `public class Svc { public static void go() { ApiMock.setTestMock(); } }`)
		project.writeClass("SvcTest", `@isTest private class SvcTest { @isTest static void runs() { Svc.go(); } }`)
		if withClass {
			project.writeClass("ApiMock", `public class ApiMock { public static void setTestMock() {} }`)
		}
		return project
	}
	absent := build(false).run()
	if got := absent.Summary(); got.Total != 1 || got.Passed != 0 {
		t.Fatalf("absent class: summary=%#v problems=%v", got, runProblems(absent))
	}
	layout := regexp.MustCompile(`unresolved Apex reference "ApiMock"; unloaded declared dependencies: AbsentPkg; CompileError: Dependent class is invalid and needs recompilation:\n Class Svc : Variable does not exist: ApiMock`)
	if p := strings.Join(runProblems(absent), "\\n"); !layout.MatchString(p) {
		t.Fatalf("absent class: signature changed: %q", p)
	}
	present := build(true).run()
	if got := present.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("present class: summary=%#v problems=%v", got, runProblems(present))
	}
}
