package apextest

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// reference_only_surfaces holds the captures for FieldServiceMobileExtension:
// describe, global describe, construction, and static and dynamic SOQL at API
// 62 through 67. cases.tsv: case_id, body. org.tsv: source_api, route, case_id,
// outcome, message. Routes are anonymous (Tooling executeAnonymous at that API)
// and isTest (one @IsTest method per case in a class at that API).
func TestReferenceOnlySurfacesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_surfaces"
	cases := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "cases.tsv"), 2)
	rows := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "org.tsv"), 5)
	// The 2026-11-05 Winter '27 org agrees with org.tsv on every isTest row at
	// API 62 through 67 and adds API 68, which equals API 67.
	rows = append(rows, referenceOnlySurfacesAPI68Rows(t, fixture, rows)...)
	bodies := make(map[string]string)
	for _, row := range cases[1:] {
		if _, dup := bodies[row[0]]; dup {
			t.Fatalf("duplicate native case %s", row[0])
		}
		bodies[row[0]] = row[1]
	}
	byAPI := make(map[string][][]string)
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		if seen[key] || bodies[row[2]] == "" || row[1] != "anonymous" && row[1] != "isTest" {
			t.Fatalf("invalid native row: %v", row)
		}
		switch row[3] {
		case "SUCCESS":
		case "COMPILE_ERROR", "RUNTIME_ERROR":
			if row[4] == "" {
				t.Fatalf("native error without message: %v", row)
			}
		default:
			t.Fatalf("unknown native outcome: %v", row)
		}
		seen[key] = true
		byAPI[row[0]] = append(byAPI[row[0]], row)
	}
	apis := make([]string, 0, len(byAPI))
	for api := range byAPI {
		apis = append(apis, api)
	}
	sort.Strings(apis)
	if len(apis) != 7 {
		t.Fatalf("native APIs=%v", apis)
	}
	compileMatches := func(t *testing.T, analysis sema.Result, row []string) bool {
		t.Helper()
		if row[3] != "COMPILE_ERROR" {
			if analysis.HasErrors() {
				t.Fatalf("%s native compiled; Glade diagnostics=%v", strings.Join(row[:3], "/"), analysis.Diagnostics)
			}
			return true
		}
		for _, item := range analysis.Diagnostics {
			message := item.NativeMessage
			if message == "" {
				message = item.Message
			}
			if item.Severity == diagnostic.Error && message == row[4] {
				return false
			}
		}
		t.Fatalf("%s native compile error %q; Glade diagnostics=%v", strings.Join(row[:3], "/"), row[4], analysis.Diagnostics)
		return false
	}
	for _, api := range apis {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var methods strings.Builder
			expected := make(map[string]string)
			var anonymous [][]string
			for _, row := range byAPI[api] {
				body := bodies[row[2]]
				if row[1] == "anonymous" {
					anonymous = append(anonymous, row)
					continue
				}
				path := filepath.Join(t.TempDir(), "SurfaceProbe.cls")
				writeFile(t, path, "@IsTest private class SurfaceProbe { @IsTest static void observe() { "+body+" } }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				local := typesys.Build(project.Project{Root: filepath.Dir(path), SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				if !compileMatches(t, sema.Analyze(local), row) {
					continue
				}
				if row[3] == "SUCCESS" {
					t.Fatalf("%s: success rows are not modelled", row[2])
				}
				expected[row[2]] = row[4]
				fmt.Fprintf(&methods, "@IsTest static void %s() { %s }\n", row[2], body)
			}
			if len(expected) > 0 {
				path := filepath.Join(root, "SurfaceRuntimeTest.cls")
				writeFile(t, path, "@IsTest private class SurfaceRuntimeTest { "+methods.String()+" }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				if analysis := sema.Analyze(index); analysis.HasErrors() {
					t.Fatalf("accepted named bodies rejected: %v", analysis.Diagnostics)
				}
				for repeat := 0; repeat < 2; repeat++ {
					assertReferenceOnlyNamedRuntime(t, index, "SurfaceRuntimeTest", expected)
				}
			}
			index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			for _, row := range anonymous {
				body := bodies[row[2]]
				t.Run("anonymous/"+row[2], func(t *testing.T) {
					if !compileMatches(t, sema.AnalyzeAnonymous(index, body, api), row) {
						return
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = runner.execute(program, func(machine *vm.VM) { machine.SetToolingExecuteAnonymous(true) })
					got := ""
					if err != nil {
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) {
							t.Fatal(err)
						}
						kind := fault.Type
						if !strings.Contains(kind, ".") {
							kind = "System." + kind
						}
						got = kind + ": " + fault.ExceptionMessage()
					}
					if got != row[4] {
						t.Fatalf("native=%q; Glade=%q", row[4], got)
					}
				})
			}
		})
	}
}

// mixed-org.tsv: a helper class at API h (reflection, construction, static and
// dynamic SOQL, describe over FieldServiceMobileExtension) is called from a
// caller at API t, from an @IsTest method and from anonymous Apex. On the
// Winter '27 org the result does not depend on h or t.
func TestReferenceOnlySurfacesMixedVersionNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_surfaces"
	methods := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "mixed-cases.tsv"), 3)[1:]
	rows := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "mixed-org.tsv"), 5)[1:]
	if len(methods) != 7 || len(rows) != 126 {
		t.Fatalf("mixed methods=%d rows=%d", len(methods), len(rows))
	}
	oracle := make(map[string][]string)
	for _, row := range rows {
		key := row[0] + "/" + row[1] + "/" + row[2]
		if oracle[key] != nil || row[3] != "RUNTIME_ERROR" {
			t.Fatalf("invalid mixed row: %v", row)
		}
		oracle[key] = row
	}
	for _, callerAPI := range []string{"62.0", "66.0", "67.0"} {
		t.Run(callerAPI, func(t *testing.T) {
			root := t.TempDir()
			var files []string
			var testBody strings.Builder
			expected := make(map[string]string)
			type anon struct{ id, body string }
			var anonymous []anon
			for _, helperAPI := range []string{"62.0", "66.0", "67.0"} {
				for _, m := range methods {
					className := "MixedHelper" + strings.TrimSuffix(helperAPI, ".0") + m[0]
					path := filepath.Join(root, className+".cls")
					writeFile(t, path, "public class "+className+" { "+m[1]+" }")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+helperAPI+"</apiVersion></ApexClass>")
					files = append(files, path)
					id := "mixed_h" + strings.TrimSuffix(helperAPI, ".0") + "_" + m[0]
					call := strings.Replace(m[2], "H.", className+".", 1)
					body := "System.assert(false, 'OBS|' + String.valueOf(" + call + "));"
					expected[id] = oracle[callerAPI+"/isTest/"+id][4]
					fmt.Fprintf(&testBody, "@IsTest static void %s() { %s }\n", id, body)
					anonymous = append(anonymous, anon{id, body})
				}
			}
			path := filepath.Join(root, "MixedCallerTest.cls")
			writeFile(t, path, "@IsTest private class MixedCallerTest { "+testBody.String()+" }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+callerAPI+"</apiVersion></ApexClass>")
			files = append(files, path)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: callerAPI, ApexFiles: files}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("mixed sources rejected: %v", analysis.Diagnostics)
			}
			assertReferenceOnlyNamedRuntime(t, index, "MixedCallerTest", expected)
			org := orgFromIndex(index)
			org.APIVersion = callerAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, item := range anonymous {
				t.Run("anonymous/"+item.id, func(t *testing.T) {
					want := oracle[callerAPI+"/anonymous/"+item.id][4]
					if analysis := sema.AnalyzeAnonymous(index, item.body, callerAPI); analysis.HasErrors() {
						t.Fatalf("anonymous rejected: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(item.body, vm.CompileOptions{APIVersion: callerAPI})
					if err != nil {
						t.Fatal(err)
					}
					_, err = runner.execute(program, func(machine *vm.VM) { machine.SetToolingExecuteAnonymous(true) })
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) {
						t.Fatalf("native=%q; Glade err=%v", want, err)
					}
					kind := fault.Type
					if !strings.Contains(kind, ".") {
						kind = "System." + kind
					}
					if got := kind + ": " + fault.ExceptionMessage(); got != want {
						t.Fatalf("native=%q; Glade=%q", want, got)
					}
				})
			}
		})
	}
}

// referenceOnlySurfacesAPI68Rows returns the API 68 rows of the 2026-11-05
// capture. It also requires every row of that capture at API 62 through 67 to
// equal the matching isTest row of org.tsv, so a later org that disagrees
// fails here and not silently.
func referenceOnlySurfacesAPI68Rows(t *testing.T, fixture string, base [][]string) [][]string {
	t.Helper()
	known := make(map[string]string)
	for _, row := range base[1:] {
		if row[1] == "isTest" {
			known[row[0]+"/"+row[2]] = row[3] + "|" + row[4]
		}
	}
	var api68 [][]string
	compared := 0
	for _, row := range readReferenceOnlyNativeTable(t, filepath.Join(fixture, "org-winter27-20261105.tsv"), 5)[1:] {
		if row[1] != "isTest" {
			t.Fatalf("unexpected route: %v", row)
		}
		if row[0] == "68.0" {
			api68 = append(api68, row)
			continue
		}
		want, ok := known[row[0]+"/"+row[2]]
		if !ok {
			continue
		}
		compared++
		if got := row[3] + "|" + row[4]; got != want {
			t.Fatalf("2026-11-05 %s/%s=%q; org.tsv=%q", row[0], row[2], got, want)
		}
	}
	if len(api68) != 51 || compared < 130 {
		t.Fatalf("2026-11-05 capture: api68=%d compared=%d", len(api68), compared)
	}
	return api68
}

// referenceOnlyTriggerBodies are the trigger-body probes behind trigger-org.tsv
// and trigger-org-winter27.tsv (#3). Each body runs in a
// before-insert trigger on Account fired from an @IsTest insert. R292TrgHelper
// holds the same declaration and query in a class, which stays class-visible.
var referenceOnlyTriggerBodies = map[string]string{
	"trg_decl":            "FieldServiceMobileExtension f; System.assert(false, 'OBS|' + String.valueOf(f == null));",
	"trg_new":             "System.assert(false, 'OBS|' + String.valueOf(new FieldServiceMobileExtension() != null));",
	"trg_soql":            "System.assert(false, 'OBS|' + String.valueOf([select Id from FieldServiceMobileExtension].size()));",
	"trg_soql_count":      "System.assert(false, 'OBS|' + String.valueOf([select count() from FieldServiceMobileExtension]));",
	"trg_soql_typed":      "List<FieldServiceMobileExtension> l = [select Id from FieldServiceMobileExtension]; System.assert(false, 'OBS|' + String.valueOf(l.size()));",
	"trg_describe":        "System.assert(false, 'OBS|' + String.valueOf(FieldServiceMobileExtension.SObjectType.getDescribe().getName()));",
	"trg_gd":              "System.assert(false, 'OBS|' + String.valueOf(Schema.getGlobalDescribe().get('FieldServiceMobileExtension') == null));",
	"trg_dynq":            "System.assert(false, 'OBS|' + String.valueOf(Database.query('select Id from FieldServiceMobileExtension').size()));",
	"trg_tf":              "System.assert(false, 'OBS|' + String.valueOf(Type.forName('FieldServiceMobileExtension') == null));",
	"trg_helper_sig":      "System.assert(false, 'OBS|' + String.valueOf(R292TrgHelper.sig()));",
	"trg_helper_sq":       "System.assert(false, 'OBS|' + String.valueOf(R292TrgHelper.sq()));",
	"trg_control_account": "System.assert(false, 'OBS|' + String.valueOf([select count() from Account]));",
}

// TestReferenceOnlySurfacesTriggerNativeConformance replays every trigger-body
// row. A trigger body sees the reference-only types: declaration, new and
// describe work, Type.forName and getGlobalDescribe are non-null, static
// count() throws SecurityException and the other static or dynamic queries throw
// the long QueryException. The 2026-11-05 capture adds API 68, equal to 67; the
// first capture must equal it at 62, 66 and 67.
//
// An assertion failure inside a trigger is wrapped in the insert's DmlException
// (CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY, "caused by: System.AssertException"),
// as natively, so every row compares the whole message with whitespace
// collapsed (the System. prefix is optional on Glade's side).
func TestReferenceOnlySurfacesTriggerNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_surfaces"
	first := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "trigger-org.tsv"), 5)[1:]
	recheck := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "trigger-org-winter27-20261105.tsv"), 5)[1:]
	if len(first) != 36 || len(recheck) != 48 {
		t.Fatalf("trigger rows first=%d recheck=%d", len(first), len(recheck))
	}
	type key struct{ api, id string }
	native := make(map[key]string)
	for _, group := range [][][]string{first, recheck} {
		for _, row := range group {
			k := key{row[0], row[2]}
			if row[1] != "trigger" || row[3] != "RUNTIME_ERROR" || referenceOnlyTriggerBodies[row[2]] == "" {
				t.Fatalf("invalid trigger row: %v", row)
			}
			if prior, dup := native[k]; dup && prior != row[4] {
				t.Fatalf("captures disagree at %v: %q vs %q", k, prior, row[4])
			}
			native[k] = row[4]
		}
	}
	for _, id := range sortedKeys(referenceOnlyTriggerBodies) {
		if native[key{"67.0", id}] != native[key{"68.0", id}] {
			t.Fatalf("API 68 differs from 67 at %s", id)
		}
	}
	for _, api := range []string{"62.0", "66.0", "67.0", "68.0"} {
		for _, id := range sortedKeys(referenceOnlyTriggerBodies) {
			want, ok := native[key{api, id}]
			if !ok {
				t.Fatalf("missing native row %s/%s", api, id)
			}
			t.Run(api+"/"+id, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				base := filepath.Join(root, "force-app/main/default")
				put := func(rel, src, meta string) {
					path := filepath.Join(base, rel)
					writeFile(t, path, src)
					writeFile(t, path+"-meta.xml", meta)
				}
				classMeta := "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>"
				put("classes/R292TrgHelper.cls", "public class R292TrgHelper { public static Boolean sig() { FieldServiceMobileExtension f = new FieldServiceMobileExtension(); return f != null; } public static Integer sq() { return [select Id from FieldServiceMobileExtension].size(); } }", classMeta)
				put("classes/TrgT.cls", "@IsTest private class TrgT { @IsTest static void m() { insert new Account(Name='r'); } }", classMeta)
				put("triggers/R292Trg.trigger", "trigger R292Trg on Account (before insert) { "+referenceOnlyTriggerBodies[id]+" }", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); analysis.HasErrors() {
					t.Fatalf("native compiled; Glade diagnostics=%v", analysis.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{"TrgT"}, NoDiskCache: true})
				got := ""
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							got = c.Problem.Type + ": " + c.Problem.Message
						}
					}
				}
				if got == "" {
					t.Fatalf("trigger body passed; native=%q", want)
				}
				if !strings.HasPrefix(got, "System.") {
					got = "System." + got
				}
				got = strings.Join(strings.Fields(got), " ")
				if got != strings.Join(strings.Fields(want), " ") {
					t.Fatalf("native=%q; Glade=%q", want, got)
				}
			})
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
