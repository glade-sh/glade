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
	// drift.tsv lists rows captured on the Winter '27 org that contradict the
	// Summer '26 rows in reference_only_names (API 62-66 reflection and static
	// SOQL). Advisor ruling 35: each is asserted twice below, as the recorded
	// Winter '27 native answer and as Glade's current Summer '26 answer, so the
	// test fails if either changes.
	drift := make(map[string]bool)
	for _, row := range readReferenceOnlyNativeTable(t, filepath.Join(fixture, "drift.tsv"), 3)[1:] {
		drift[row[0]+"/"+row[1]] = true
	}
	bodies := make(map[string]string)
	for _, row := range cases[1:] {
		if _, dup := bodies[row[0]]; dup {
			t.Fatalf("duplicate native case %s", row[0])
		}
		bodies[row[0]] = row[1]
	}
	byAPI := make(map[string][][]string)
	driftRows := make(map[string][][]string)
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
		if drift[row[0]+"/"+row[2]] && row[1] == "isTest" {
			delete(drift, row[0]+"/"+row[2])
			driftRows[row[0]] = append(driftRows[row[0]], row)
			continue
		}
		byAPI[row[0]] = append(byAPI[row[0]], row)
	}
	if len(drift) != 0 {
		t.Fatalf("drift rows without an isTest observation: %v", drift)
	}
	apis := make([]string, 0, len(byAPI))
	for api := range byAPI {
		apis = append(apis, api)
	}
	sort.Strings(apis)
	if len(apis) != 6 {
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
			assertSurfaceDrift(t, api, bodies, byAPI, driftRows[api])
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
	// Glade still gates reflection and static SOQL on the helper's API version
	// (reference_only_names rows); the Winter '27 org no longer does. See drift.tsv.
	drift := func(helperAPI, method string) bool {
		return helperAPI != "67.0" && (method == "tf" || method == "sq")
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
					isDrift := drift(helperAPI, m[0])
					className := "MixedHelper" + strings.TrimSuffix(helperAPI, ".0") + m[0]
					path := filepath.Join(root, className+".cls")
					writeFile(t, path, "public class "+className+" { "+m[1]+" }")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+helperAPI+"</apiVersion></ApexClass>")
					files = append(files, path)
					id := "mixed_h" + strings.TrimSuffix(helperAPI, ".0") + "_" + m[0]
					call := strings.Replace(m[2], "H.", className+".", 1)
					body := "System.assert(false, 'OBS|' + String.valueOf(" + call + "));"
					if isDrift {
						// Winter '27 native rows do not depend on the helper API (compare the
						// h67 row); Glade still gates on it. Assert both, advisor ruling 35.
						winter := oracle[callerAPI+"/isTest/mixed_h67_"+m[0]]
						if oracle[callerAPI+"/isTest/"+id][4] != winter[4] || oracle[callerAPI+"/anonymous/"+id][4] != oracle[callerAPI+"/anonymous/mixed_h67_"+m[0]][4] {
							t.Fatalf("%s: Winter '27 native row depends on the helper API", id)
						}
						want := gladeMixedDrift[m[0]]
						if want == "" || want == winter[4] {
							t.Fatalf("%s: Glade drift answer %q must be set and differ from native %q", id, want, winter[4])
						}
						expected[id] = want
						fmt.Fprintf(&testBody, "@IsTest static void %s() { %s }\n", id, body)
						continue
					}
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

// trigger-org.tsv records native trigger-body behaviour that Glade does not model
// yet (#3). The rows are evidence, not a replay.
func TestReferenceOnlySurfacesTriggerRowsRecorded(t *testing.T) {
	rows := readReferenceOnlyNativeTable(t, "testdata/conformance/reference_only_surfaces/trigger-org.tsv", 5)[1:]
	if len(rows) != 36 {
		t.Fatalf("trigger rows=%d", len(rows))
	}
	for _, row := range rows {
		if row[1] != "trigger" || row[3] != "RUNTIME_ERROR" || !strings.HasPrefix(row[2], "trg_") {
			t.Fatalf("invalid trigger row: %v", row)
		}
	}
}

// gladeSurfaceDrift is Glade's current (Summer '26) isTest answer for the rows in
// drift.tsv. The recorded native rows are the Winter '27 answers.
var gladeSurfaceDrift = map[string]string{
	"construct_type_forName":        "System.AssertException: Assertion Failed: OBS|false",
	"construct_type_forName_schema": "System.AssertException: Assertion Failed: OBS|false",
	"construct_typeof_newInstance":  "System.AssertException: Assertion Failed: OBS|true",
	"query_static_list":             "System.AssertException: Assertion Failed: OBS|0",
	"query_static_count":            "System.AssertException: Assertion Failed: OBS|0",
	"query_static_list_typed":       "System.AssertException: Assertion Failed: OBS|0",
}

// assertSurfaceDrift pins both sides of a release-drift row at one API: the
// recorded Winter '27 native row (equal to the API 67 row) and Glade's current
// behaviour, run in one @IsTest class at that API.
func assertSurfaceDrift(t *testing.T, api string, bodies map[string]string, byAPI map[string][][]string, drifted [][]string) {
	t.Helper()
	if len(drifted) == 0 {
		return
	}
	winter := make(map[string][]string)
	for _, row := range byAPI["67.0"] {
		if row[1] == "isTest" {
			winter[row[2]] = row
		}
	}
	var methods strings.Builder
	for _, row := range drifted {
		id := row[2]
		if row[3] != "RUNTIME_ERROR" || winter[id] == nil || winter[id][3] != row[3] || winter[id][4] != row[4] {
			t.Fatalf("%s/%s: Winter '27 native row %v does not match the API 67 row %v", api, id, row, winter[id])
		}
		fmt.Fprintf(&methods, "@IsTest static void %s() { %s }\n", id, bodies[id])
	}
	root := t.TempDir()
	path := filepath.Join(root, "SurfaceDriftTest.cls")
	writeFile(t, path, "@IsTest private class SurfaceDriftTest { "+methods.String()+" }")
	writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
	if analysis := sema.Analyze(index); analysis.HasErrors() {
		t.Fatalf("drift bodies rejected: %v", analysis.Diagnostics)
	}
	expected := make(map[string]string)
	for _, row := range drifted {
		want := gladeSurfaceDrift[row[2]]
		if want == "" || want == row[4] {
			t.Fatalf("%s: Glade drift answer %q must be set and differ from the native row %q", row[2], want, row[4])
		}
		expected[row[2]] = want
	}
	assertReferenceOnlyNamedRuntime(t, index, "SurfaceDriftTest", expected)
}

// gladeMixedDrift is Glade's current isTest answer for a helper at API 62-66
// called through reflection (tf) or static SOQL (sq); native Winter '27 differs.
var gladeMixedDrift = map[string]string{
	"tf": "System.AssertException: Assertion Failed: OBS|false",
	"sq": "System.AssertException: Assertion Failed: OBS|0",
}
