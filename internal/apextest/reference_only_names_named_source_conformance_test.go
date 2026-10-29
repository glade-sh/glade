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
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// named-cases.tsv holds class signatures and static SOQL in named source.
// Class cases compile alone (route named); accepted classes are then called
// from anonymous Apex and from an @IsTest method. Direct cases use the
// anonymous and @IsTest routes of the main fixture.
func TestReferenceOnlyNameNamedSourceNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_names"
	const placeholder = "RefSigCase"
	cases := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "named-cases.tsv"), 3)
	rows := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "named-org.tsv"), 5)
	type probe struct {
		index       int
		class, body string
	}
	probes := make(map[string]probe)
	var ids []string
	for i, row := range cases[1:] {
		if _, exists := probes[row[0]]; exists || row[2] == "" {
			t.Fatalf("invalid named-source case: %v", row)
		}
		probes[row[0]] = probe{index: i, class: row[1], body: row[2]}
		ids = append(ids, row[0])
	}
	sort.Strings(ids)
	// Signature and SOQL rows need FieldServiceMobileExtension and two other
	// reference-only names, so the shared rule is not inferred from one name.
	for _, name := range []string{"FieldServiceMobileExtension", "ActivationTarget", "DataAction"} {
		for _, op := range []string{"sig_field", "sig_static_field", "sig_property", "sig_param", "sig_return", "sig_return_new",
			"sig_list_field", "sig_list_param", "sig_list_return", "sig_set_field", "sig_map_value_field", "soql", "soql_count", "soql_named"} {
			if _, ok := probes[name+"_"+op]; !ok {
				t.Fatalf("missing named-source case %s_%s", name, op)
			}
		}
	}
	oracle := make(map[string][]string)
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		switch {
		case row[3] == "SUCCESS" && row[4] == "", (row[3] == "COMPILE_ERROR" || row[3] == "RUNTIME_ERROR") && row[4] != "":
		default:
			t.Fatalf("invalid native observation: %v", row)
		}
		if _, ok := probes[row[2]]; !ok || oracle[key] != nil {
			t.Fatalf("unknown or duplicate native observation: %v", row)
		}
		oracle[key] = row
	}
	required := 0
	for _, id := range ids {
		for _, api := range []string{"62.0", "67.0"} {
			routes := []string{"anonymous", "isTest"}
			if probes[id].class != "" {
				named := oracle[api+"/named/"+id]
				if named == nil {
					t.Fatalf("missing native observation %s/named/%s", api, id)
				}
				required++
				routes = nil
				if named[3] == "SUCCESS" {
					routes = []string{"named_anonymous", "named_isTest"}
				}
			}
			for _, route := range routes {
				if oracle[api+"/"+route+"/"+id] == nil {
					t.Fatalf("missing native observation %s/%s/%s", api, route, id)
				}
				required++
			}
		}
	}
	if required != len(oracle) {
		t.Fatalf("native observations=%d required=%d", len(oracle), required)
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
	writeClass := func(t *testing.T, dir, name, source, api string) string {
		t.Helper()
		path := filepath.Join(dir, name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		return path
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var files []string
			callers := make(map[string]string)
			var anonymous []string
			var methods strings.Builder
			expected := make(map[string]string)
			addTestMethod := func(id, route, body string) {
				method := fmt.Sprintf("observe%d", len(expected))
				expected[method] = strings.Join([]string{api, route, id}, "/")
				fmt.Fprintf(&methods, "@IsTest static void %s() { %s }\n", method, body)
			}
			for _, id := range ids {
				probe := probes[id]
				if probe.class == "" {
					// Direct bodies compile inside their own @IsTest wrapper.
					row := oracle[api+"/isTest/"+id]
					dir := t.TempDir()
					path := writeClass(t, dir, "ReferenceOnlyNamedProbe", "@IsTest private class ReferenceOnlyNamedProbe { @IsTest static void observe() { "+probe.body+" } }", api)
					local := typesys.Build(project.Project{Root: dir, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					if compileMatches(t, sema.Analyze(local), row) {
						addTestMethod(id, "isTest", probe.body)
					}
					anonymous = append(anonymous, id)
					continue
				}
				className := fmt.Sprintf("RefSigProbe%d", probe.index)
				source := strings.ReplaceAll(probe.class, placeholder, className)
				dir := t.TempDir()
				path := writeClass(t, dir, className, source, api)
				local := typesys.Build(project.Project{Root: dir, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				if !compileMatches(t, sema.Analyze(local), oracle[api+"/named/"+id]) {
					continue
				}
				files = append(files, writeClass(t, root, className, source, api))
				caller := strings.ReplaceAll(probe.body, placeholder, className)
				callers[id] = caller
				addTestMethod(id, "named_isTest", caller)
				anonymous = append(anonymous, id)
			}
			files = append(files, writeClass(t, root, "ReferenceOnlyNamedRuntimeTest", "@IsTest private class ReferenceOnlyNamedRuntimeTest { "+methods.String()+" }", api))
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("accepted named sources rejected: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, id := range anonymous {
				route, body := "anonymous", probes[id].body
				if caller, ok := callers[id]; ok {
					route, body = "named_anonymous", caller
				}
				row := oracle[api+"/"+route+"/"+id]
				t.Run(route+"/"+id, func(t *testing.T) {
					if !compileMatches(t, sema.AnalyzeAnonymous(index, body, api), row) {
						return
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for repeat := 0; repeat < 2; repeat++ {
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
							t.Fatalf("repeat %d native=%q; Glade=%q", repeat, row[4], got)
						}
					}
				})
			}
			for repeat := 0; repeat < 2; repeat++ {
				t.Run(fmt.Sprintf("project_runner_%d", repeat), func(t *testing.T) {
					report := Run(index, Options{SelectedClasses: []string{"ReferenceOnlyNamedRuntimeTest"}, NoDiskCache: true, Parallelism: 1})
					seen := make(map[string]bool)
					for _, suite := range report.Suites {
						for _, result := range suite.Cases {
							key, ok := expected[result.MethodName]
							if !ok || seen[result.MethodName] {
								t.Fatalf("unexpected or duplicate result: %+v", result)
							}
							seen[result.MethodName] = true
							row := oracle[key]
							got := ""
							if result.Problem != nil {
								kind := result.Problem.Type
								if !strings.Contains(kind, ".") {
									kind = "System." + kind
								}
								got = kind + ": " + result.Problem.Message
							} else if result.Status != testreport.StatusPass {
								got = "status " + string(result.Status)
							}
							if got != row[4] {
								t.Errorf("%s native=%q; Glade=%q", key, row[4], got)
							}
						}
					}
					if len(seen) != len(expected) {
						t.Fatalf("named results=%d want=%d; %v", len(seen), len(expected), report.Summary())
					}
				})
			}
		})
	}
}

// Static SOQL in a named @IsTest method follows the reflection boundary:
// no rows at API 62-66 and an unsupported sObject type at API 67.
func TestReferenceOnlyNameFieldServiceSOQLVersionsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_names"
	rows := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "soql-version-org.tsv"), 5)
	bodies := make(map[string]string)
	for _, row := range readReferenceOnlyNativeTable(t, filepath.Join(fixture, "named-cases.tsv"), 3)[1:] {
		bodies[row[0]] = row[2]
	}
	if len(rows) != 13 {
		t.Fatalf("SOQL version rows=%d want=12", len(rows)-1)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var source strings.Builder
			expected := make(map[string]string)
			source.WriteString("@IsTest private class ReferenceOnlySOQLVersionTest { ")
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				method := strings.TrimPrefix(row[2], "FieldServiceMobileExtension_")
				if row[1] != "isTest" || row[3] != "RUNTIME_ERROR" || bodies[row[2]] == "" || expected[method] != "" {
					t.Fatalf("invalid SOQL version observation: %v", row)
				}
				expected[method] = row[4]
				fmt.Fprintf(&source, "@IsTest static void %s() { %s } ", method, bodies[row[2]])
			}
			source.WriteString("}")
			if len(expected) != 2 {
				t.Fatal("incomplete SOQL version observations")
			}
			root := t.TempDir()
			path := filepath.Join(root, "ReferenceOnlySOQLVersionTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			assertReferenceOnlyNamedRuntime(t, index, "ReferenceOnlySOQLVersionTest", expected)
		})
	}
}
