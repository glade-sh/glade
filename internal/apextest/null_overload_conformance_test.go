package apextest

import (
	"encoding/json"
	"os"
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

// Each captured route has its own native oracle. Named tests execute through Run;
// only the terminal transport assertion is replaced by an exact payload check.
// Native sources and complete assertion messages remain in the exported data.
// Diagnostic differences have no category-only or rewritten-text assertions.
func TestNullOverloadOrgConformance(t *testing.T) {
	type compilation struct{ Name, Source, Expected string }
	type namedRow struct {
		ID, Group, Owner, Reason, Route string
		Compilations                    map[string]compilation
	}
	type runtimeRow struct {
		ID, Code, Route string
		Expected        map[string]string
	}
	type testObservation struct {
		Name, Source, Compilation, Outcome, Message, Value string
	}
	type testRow struct {
		ID, Route, Owner, Reason string
		Observations             map[string]testObservation
	}
	var data struct {
		APIVersions             []string `json:"apiVersions"`
		ObservedRoutes          []string `json:"observedRoutes"`
		Declarations            map[string]string
		NamedCompilations       []namedRow `json:"named"`
		Remaining               []namedRow
		AnonymousRuntime        []runtimeRow `json:"runtime"`
		IsTest, IsTestRemaining []testRow
	}
	raw, err := os.ReadFile("testdata/conformance/null_overload.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.NamedCompilations) != 20 || len(data.Remaining) != 7 || len(data.AnonymousRuntime) != 10 || len(data.IsTest) != 17 || len(data.IsTestRemaining) != 5 || strings.Join(data.APIVersions, ",") != "61.0,62.0,67.0" {
		t.Fatalf("oracle shape: %d named, %d diagnostic differences, %d runtime, %v", len(data.NamedCompilations), len(data.Remaining), len(data.AnonymousRuntime), data.APIVersions)
	}
	if strings.Join(data.ObservedRoutes, ",") != "named-compiler,execute-anonymous,is-test" {
		t.Fatalf("uncaptured execution route: %v", data.ObservedRoutes)
	}
	seen := map[string]bool{}
	for _, row := range append(append([]namedRow{}, data.NamedCompilations...), data.Remaining...) {
		if row.Route != "named-compiler" || row.ID == "" || seen[row.ID] || len(row.Compilations) != 3 {
			t.Fatalf("incomplete/duplicate compiler row: %#v", row)
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			native := row.Compilations[api]
			if native.Name == "" || native.Source == "" || native.Expected == "" {
				t.Fatalf("missing native row %s %s", row.ID, api)
			}
		}
	}
	for _, row := range data.Remaining {
		if row.Owner != "source syntax" || row.Reason == "" {
			t.Fatalf("undisposed diagnostic difference: %#v", row)
		}
		t.Logf("%s diagnostic difference owned by %s: %s; native rows retained without assertions", row.ID, row.Owner, row.Reason)
	}

	observedTests := map[string]testRow{}
	for _, row := range append(append([]testRow{}, data.IsTest...), data.IsTestRemaining...) {
		if row.Route != "is-test" || !seen[row.ID] || observedTests[row.ID].ID != "" || len(row.Observations) != 3 {
			t.Fatalf("incomplete/repeated native test row: %#v", row)
		}
		observedTests[row.ID] = row
		for _, api := range data.APIVersions {
			native := row.Observations[api]
			if native.Name == "" || native.Source == "" || native.Compilation == "" {
				t.Fatalf("missing native test declaration: %s %s", row.ID, api)
			}
			if native.Compilation == "compiled" && (native.Outcome != "Fail" || native.Message != "System.AssertException: Assertion Failed: P|"+row.ID+"|"+native.Value) {
				t.Fatalf("missing exact native test execution: %s %s", row.ID, api)
			}
		}
	}
	for _, row := range data.AnonymousRuntime {
		for _, api := range data.APIVersions {
			native := observedTests[row.ID].Observations[api]
			if native.Message == "" || native.Value != row.Expected[api] {
				t.Fatalf("incomplete captured anonymous/@IsTest matrix: %s %s", row.ID, api)
			}
		}
	}
	for _, api := range data.APIVersions {
		if observedTests["R014"].Observations[api].Message == "" {
			t.Fatalf("R014 missing native @IsTest execution: %s", api)
		}
	}
	for _, row := range data.IsTestRemaining {
		if row.Owner != "source syntax" || row.Reason == "" {
			t.Fatalf("undisposed @IsTest diagnostic difference: %#v", row)
		}
		t.Logf("%s @IsTest diagnostic difference owned by %s: %s; native rows retained without assertions", row.ID, row.Owner, row.Reason)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			build := func(extra map[string]string) typesys.Index {
				t.Helper()
				root := t.TempDir()
				names := make([]string, 0, len(data.Declarations)+len(extra))
				sources := map[string]string{}
				for name, source := range data.Declarations {
					sources[name] = source
					names = append(names, name)
				}
				for name, source := range extra {
					sources[name] = source
					names = append(names, name)
				}
				sort.Strings(names)
				paths := make([]string, 0, len(names))
				for _, name := range names {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, sources[name])
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "A02W2Widget__c"}}})
			}
			compilerObservation := func(index typesys.Index) string {
				diagnostics := sema.Analyze(index).Diagnostics
				errors := []diagnostic.Diagnostic{}
				for _, d := range diagnostics {
					if d.Severity == diagnostic.Error {
						d.File, d.Code, d.Range = "", "", nil
						errors = append(errors, d)
					}
				}
				observed := "compiled"
				if len(errors) > 0 {
					observed = "COMPILE_ERROR\t" + indexCompileError(errors).Error()
				}
				return observed
			}
			for _, row := range data.NamedCompilations {
				t.Run(row.ID+"/namedCompiler", func(t *testing.T) {
					native := row.Compilations[api]
					index := build(map[string]string{native.Name: native.Source})
					observed := compilerObservation(index)
					if observed != native.Expected {
						t.Fatalf("native <%s> actual <%s>", native.Expected, observed)
					}
				})
			}
			index := build(nil)
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("runtime fixture semantics: %v", analysis.Diagnostics)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			runtimeSeen := map[string]bool{}
			for _, row := range data.AnonymousRuntime {
				t.Run(row.ID+"/anonymous", func(t *testing.T) {
					expected, ok := row.Expected[api]
					if row.Route != "execute-anonymous" || !seen[row.ID] || runtimeSeen[row.ID] || !ok || row.Code == "" {
						t.Fatalf("incomplete/repeated runtime row: %#v", row)
					}
					runtimeSeen[row.ID] = true
					source := "Object r; " + row.Code + " String observedText=String.valueOf(r); System.assert(" + conformanceApexString(expected) + ".equals(observedText),'native <'+" + conformanceApexString(expected) + "+'> actual <'+observedText+'>');"
					if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
						t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
			namedSources := map[string]string{}
			for _, row := range data.IsTest {
				native := row.Observations[api]
				t.Run(row.ID+"/isTestCompiler", func(t *testing.T) {
					observed := compilerObservation(build(map[string]string{native.Name: native.Source}))
					if observed != native.Compilation {
						t.Fatalf("native <%s> actual <%s>", native.Compilation, observed)
					}
				})
				if native.Compilation == "compiled" {
					marker := "System.assert(false,'P|" + row.ID + "|'+captured);"
					if strings.Count(native.Source, marker) != 1 {
						t.Fatalf("missing/repeated terminal transport assertion: %s", row.ID)
					}
					assertion := "System.assert(" + conformanceApexString(native.Value) + ".equals(captured),'native <'+" + conformanceApexString(native.Value) + "+'> actual <'+captured+'>');"
					namedSources[native.Name] = strings.Replace(native.Source, marker, assertion, 1)
				}
			}
			namedIndex := build(namedSources)
			if namedIndex.HasErrors() {
				t.Fatalf("native test parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("native test semantics: %v", analysis.Diagnostics)
			}
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			results := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					if _, ok := namedSources[result.ClassName]; !ok || result.MethodName != "observed" || results[result.ClassName].MethodName != "" {
						t.Fatalf("unexpected/repeated native test result: %#v", result)
					}
					results[result.ClassName] = result
				}
			}
			if len(namedSources) != 15 || len(results) != 15 || run.Summary().Total != 15 {
				t.Fatalf("native test count: sources=%d results=%d summary=%#v", len(namedSources), len(results), run.Summary())
			}
			for _, row := range data.IsTest {
				native := row.Observations[api]
				if native.Compilation != "compiled" {
					continue
				}
				t.Run(row.ID+"/isTest", func(t *testing.T) {
					result := results[native.Name]
					if result.Status != testreport.StatusPass || result.Problem != nil {
						t.Fatalf("native payload <%s>: status=%s problem=%#v", native.Value, result.Status, result.Problem)
					}
				})
			}

		})
	}
}
