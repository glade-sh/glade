package apextest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The native table distinguishes compiler rejection from executed exceptions.
// Assertion failures carry dynamic lookup values without relying on debug logs.
func TestReferenceOnlyNamesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_names"
	readTable := func(name string, columns int) [][]string {
		t.Helper()
		file, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.Comma, reader.FieldsPerRecord = '\t', columns
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	cases := readTable("cases.tsv", 2)
	rows := readTable("org.tsv", 5)
	bodies := make(map[string]string)
	for _, row := range cases[1:] {
		if _, exists := bodies[row[0]]; exists {
			t.Fatalf("duplicate native case: %s", row[0])
		}
		bodies[row[0]] = row[1]
	}
	// Each name affected by the shared registration rule needs its own oracle.
	// Catalog membership alone is not evidence of Apex type availability.
	var missing []string
	for _, name := range storage.ReferenceOnlyStandardObjectNames() {
		for _, probe := range []struct{ op, body string }{
			{"declaration", name + " value;"},
			{"new", "Object value = new " + name + "();"},
			{"schema_new", "Object value = new Schema." + name + "();"},
			{"describe", name + ".SObjectType.getDescribe();"},
			{"global_get", "System.assert(false, 'OBS|' + String.valueOf(Schema.getGlobalDescribe().get('" + name + "') == null));"},
			{"for_name", "System.assert(false, 'OBS|' + String.valueOf(Type.forName('" + name + "') == null));"},
		} {
			id := name + "_" + probe.op
			if body, ok := bodies[id]; !ok {
				missing = append(missing, id)
			} else if body != probe.body {
				t.Fatalf("native case %s has unexpected source: %s", id, body)
			}
		}
	}
	if len(missing) != 0 {
		t.Fatalf("native fixture missing %d required cases; first cases: %s", len(missing), strings.Join(missing[:min(10, len(missing))], ", "))
	}
	oracle := make(map[string]bool)
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		switch row[3] {
		case "SUCCESS":
			if row[4] != "" {
				t.Fatalf("successful native observation has an error message: %s", key)
			}
		case "COMPILE_ERROR", "RUNTIME_ERROR":
			if row[4] == "" {
				t.Fatalf("native error observation has no message: %s", key)
			}
		default:
			t.Fatalf("unknown native outcome %q: %s", row[3], key)
		}
		if oracle[key] {
			t.Fatalf("duplicate native observation: %s", key)
		}
		oracle[key] = true
	}
	for id := range bodies {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "isTest"} {
				if key := api + "/" + route + "/" + id; !oracle[key] {
					t.Fatalf("missing native observation: %s", key)
				}
			}
		}
	}
	seen := make(map[string]bool)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api}, schema.Schema{})
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			var methods strings.Builder
			named := make(map[string][]string)
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				key := strings.Join(row[:3], "/")
				if seen[key] || bodies[row[2]] == "" || row[1] != "anonymous" && row[1] != "isTest" {
					t.Fatalf("invalid native row: %v", row)
				}
				seen[key] = true
				body := bodies[row[2]]
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					var analysis sema.Result
					if row[1] == "anonymous" {
						analysis = sema.AnalyzeAnonymous(index, body, api)
					} else {
						path := filepath.Join(t.TempDir(), "ReferenceOnlyProbe.cls")
						writeFile(t, path, "@IsTest private class ReferenceOnlyProbe { @IsTest static void observe() { "+body+" } }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						local := typesys.Build(project.Project{Root: filepath.Dir(path), SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
						analysis = sema.Analyze(local)
					}
					if row[4] != "" && row[3] == "COMPILE_ERROR" {
						for _, item := range analysis.Diagnostics {
							message := item.NativeMessage
							if message == "" {
								message = item.Message
							}
							if item.Severity == diagnostic.Error && message == row[4] {
								return
							}
						}
						t.Fatalf("native compile error %q; Glade diagnostics=%v", row[4], analysis.Diagnostics)
					}
					if analysis.HasErrors() {
						t.Fatalf("native compiled; Glade diagnostics=%v", analysis.Diagnostics)
					}
					if row[1] == "isTest" {
						method := fmt.Sprintf("observe%d", len(named))
						named[method] = row
						fmt.Fprintf(&methods, "@IsTest static void %s() { %s }\n", method, body)
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
			// Replay accepted named bodies through the complete project runner,
			// then repeat with the same index to exercise warm runtime caches.
			path := filepath.Join(root, "ReferenceOnlyRuntimeTest.cls")
			writeFile(t, path, "@IsTest private class ReferenceOnlyRuntimeTest { "+methods.String()+" }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			for repeat := 0; repeat < 2; repeat++ {
				t.Run(fmt.Sprintf("project_runner_%d", repeat), func(t *testing.T) {
					report := Run(namedIndex, Options{SelectedClasses: []string{"ReferenceOnlyRuntimeTest"}, NoDiskCache: true, Parallelism: 1})
					count := 0
					observed := make(map[string]bool)
					for _, suite := range report.Suites {
						for _, result := range suite.Cases {
							row, ok := named[result.MethodName]
							if !ok {
								t.Fatalf("unexpected test result: %+v", result)
							}
							if observed[result.MethodName] {
								t.Fatalf("duplicate test result: %s", result.MethodName)
							}
							observed[result.MethodName] = true
							if row[3] == "SUCCESS" && result.Status != testreport.StatusPass {
								t.Errorf("%s native succeeded; Glade status=%s", row[2], result.Status)
							}
							count++
							got := ""
							if result.Problem != nil {
								kind := result.Problem.Type
								if !strings.Contains(kind, ".") {
									kind = "System." + kind
								}
								// The project report appends VM diagnostic context to this
								// exception; compare the Apex message captured by the org.
								message := strings.TrimSuffix(result.Problem.Message, " (context: while invoking member getDescribe on null receiver)")
								got = kind + ": " + message
							}
							if got != row[4] {
								t.Errorf("%s native=%q; Glade=%q", row[2], row[4], got)
							}
						}
					}
					if count != len(named) {
						t.Fatalf("named replay count=%d want=%d; %v", count, len(named), report.Summary())
					}
				})
			}
		})
	}
	if len(seen) != len(rows)-1 {
		t.Fatal("unreplayed native rows")
	}
}

func TestReferenceOnlyNameAllowsOwnedClassNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_names/"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(fixture + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	helper, body, oracle := read("shadow.cls"), read("shadow.apex"), read("shadow-org.tsv")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, route := range []string{"anonymous", "isTest"} {
				if !strings.Contains(oracle, api+"\t"+route+"\tSUCCESS\t\n") {
					t.Fatal("missing native shadow-class boundary")
				}
			}
			root := t.TempDir()
			helperPath, testPath := filepath.Join(root, "ActivationTarget.cls"), filepath.Join(root, "ReferenceOnlyShadowTest.cls")
			writeFile(t, helperPath, helper)
			writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>62.0</apiVersion></ApexClass>")
			writeFile(t, testPath, "@IsTest private class ReferenceOnlyShadowTest { @IsTest static void observe() { "+body+" } }")
			writeFile(t, testPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{helperPath, testPath}}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
			report := Run(index, Options{SelectedClasses: []string{"ReferenceOnlyShadowTest"}, NoDiskCache: true, Parallelism: 1})
			if report.Summary().Passed != 1 || report.Summary().Failed != 0 {
				t.Fatalf("owned class named after reference-only target: %+v", report)
			}
		})
	}
}

// The same transaction crosses anonymous and named frames twice, so a cached
// named object token must not make the following anonymous lookup succeed.
func TestReferenceOnlyNameFieldServiceContextNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_names"
	rows := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "context-org.tsv"), 5)
	if len(rows) != 5 {
		t.Fatalf("context rows=%d want=4", len(rows)-1)
	}
	observations := make(map[string]bool)
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		if row[0] != "62.0" && row[0] != "67.0" || row[1] != "anonymous" && row[1] != "isTest" ||
			row[2] != "warm_named_availability" || row[3] != "RUNTIME_ERROR" || row[4] == "" || observations[key] {
			t.Fatalf("invalid context observation: %v", row)
		}
		observations[key] = true
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			api := row[0]
			root := t.TempDir()
			helperName := "RefOnlyAvailabilityWarm" + strings.TrimSuffix(api, ".0")
			helper, err := os.ReadFile(filepath.Join(fixture, helperName+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(fixture, helperName+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			helperPath, testPath := filepath.Join(root, helperName+".cls"), filepath.Join(root, "ReferenceOnlyContextTest.cls")
			writeFile(t, helperPath, string(helper))
			writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			writeFile(t, testPath, "@IsTest private class ReferenceOnlyContextTest { @IsTest static void observe() { "+string(body)+" } }")
			writeFile(t, testPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{helperPath, testPath}}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			switch row[1] {
			case "anonymous":
				// Named analysis above must not admit these source forms into the
				// shared index used by a later anonymous compilation.
				bodies := make(map[string]string)
				for _, probe := range readReferenceOnlyNativeTable(t, filepath.Join(fixture, "cases.tsv"), 2)[1:] {
					bodies[probe[0]] = probe[1]
				}
				for _, native := range readReferenceOnlyNativeTable(t, filepath.Join(fixture, "org.tsv"), 5)[1:] {
					if native[0] != api || native[1] != "anonymous" || native[3] != "COMPILE_ERROR" || !strings.HasPrefix(native[2], "FieldServiceMobileExtension_") {
						continue
					}
					analysis := sema.AnalyzeAnonymous(index, bodies[native[2]], api)
					matched := false
					for _, item := range analysis.Diagnostics {
						message := item.NativeMessage
						if message == "" {
							message = item.Message
						}
						matched = matched || item.Severity == diagnostic.Error && message == native[4]
					}
					if !matched {
						t.Fatalf("warm %s native=%q; Glade=%v", native[2], native[4], analysis.Diagnostics)
					}
				}
				if analysis := sema.AnalyzeAnonymous(index, string(body), api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for repeat := 0; repeat < 2; repeat++ {
					_, err = runner.execute(program, func(machine *vm.VM) { machine.SetToolingExecuteAnonymous(true) })
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) {
						t.Fatalf("native exception %q; Glade=%v", row[4], err)
					}
					kind := fault.Type
					if !strings.Contains(kind, ".") {
						kind = "System." + kind
					}
					if got := kind + ": " + fault.ExceptionMessage(); got != row[4] {
						t.Fatalf("native=%q; Glade=%q", row[4], got)
					}
				}
			case "isTest":
				for repeat := 0; repeat < 2; repeat++ {
					assertReferenceOnlyNamedRuntime(t, index, "ReferenceOnlyContextTest", map[string]string{"observe": row[4]})
				}
			default:
				t.Fatalf("unknown native route: %s", row[1])
			}
		})
	}
}

func TestReferenceOnlyNameFieldServiceReflectionVersionsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/reference_only_names"
	rows := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "reflection-version-org.tsv"), 5)
	cases := readReferenceOnlyNativeTable(t, filepath.Join(fixture, "reflection-version-cases.tsv"), 2)
	if len(rows) != 13 || len(cases) != 3 {
		t.Fatalf("reflection rows=%d cases=%d", len(rows)-1, len(cases)-1)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var source strings.Builder
			expected := make(map[string]string)
			source.WriteString("@IsTest private class ReferenceOnlyReflectionTest { ")
			for _, probe := range cases[1:] {
				fmt.Fprintf(&source, "@IsTest static void %s() { %s } ", probe[0], probe[1])
			}
			source.WriteString("}")
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				if row[1] != "isTest" || row[3] != "RUNTIME_ERROR" || expected[row[2]] != "" {
					t.Fatalf("invalid reflection observation: %v", row)
				}
				expected[row[2]] = row[4]
			}
			if len(expected) != 2 {
				t.Fatal("incomplete reflection version observations")
			}
			path := filepath.Join(root, "ReferenceOnlyReflectionTest.cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			assertReferenceOnlyNamedRuntime(t, index, "ReferenceOnlyReflectionTest", expected)
		})
	}
}

func readReferenceOnlyNativeTable(t *testing.T, path string, columns int) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.FieldsPerRecord = '\t', columns
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func assertReferenceOnlyNamedRuntime(t *testing.T, index typesys.Index, class string, expected map[string]string) {
	t.Helper()
	report := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
	seen := make(map[string]bool)
	for _, suite := range report.Suites {
		for _, result := range suite.Cases {
			message, ok := expected[result.MethodName]
			if !ok || seen[result.MethodName] {
				t.Fatalf("unexpected or duplicate result: %+v", result)
			}
			seen[result.MethodName] = true
			if result.Problem == nil {
				t.Fatalf("native exception %q; Glade result=%+v", message, result)
			}
			kind := result.Problem.Type
			if !strings.Contains(kind, ".") {
				kind = "System." + kind
			}
			if got := kind + ": " + result.Problem.Message; got != message {
				t.Errorf("%s native=%q; Glade=%q", result.MethodName, message, got)
			}
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("named results=%d want=%d; %v", len(seen), len(expected), report.Summary())
	}
}

// Preserve existing owned Schema/nested-class identity while adding the captured
// platform fallback. Native source-shadow precedent is in describe_source_shadow.
func TestReferenceOnlyNamePreservesOwnedSchemaType(t *testing.T) {
	const helper = `public class Schema { public class FieldServiceMobileExtension { public String marker = 'owned'; } }`
	const otherHelper = `public class OtherOwner { public class FieldServiceMobileExtension { public String marker = 'other'; } }`
	const body = `Schema.FieldServiceMobileExtension value = new Schema.FieldServiceMobileExtension();
System.assertEquals('owned', value.marker);
OtherOwner.FieldServiceMobileExtension other = new OtherOwner.FieldServiceMobileExtension();
System.assertEquals('other', other.marker);
Type reflected = Type.forName('Schema.FieldServiceMobileExtension');
System.assertNotEquals(null, reflected);
System.assertEquals('owned', ((Schema.FieldServiceMobileExtension)reflected.newInstance()).marker);`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			helperPath, otherPath, testPath := filepath.Join(root, "Schema.cls"), filepath.Join(root, "OtherOwner.cls"), filepath.Join(root, "ReferenceOnlyOwnedSchemaTest.cls")
			writeFile(t, helperPath, helper)
			writeFile(t, otherPath, otherHelper)
			writeFile(t, testPath, "@IsTest private class ReferenceOnlyOwnedSchemaTest { @IsTest static void observe() { "+body+" } }")
			for _, path := range []string{helperPath, otherPath, testPath} {
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{helperPath, otherPath, testPath}}, schema.Schema{})
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program, func(machine *vm.VM) { machine.SetToolingExecuteAnonymous(true) }); err != nil {
				t.Fatal(err)
			}
			report := Run(index, Options{SelectedClasses: []string{"ReferenceOnlyOwnedSchemaTest"}, NoDiskCache: true, Parallelism: 1})
			if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 || report.Suites[0].Cases[0].Status != testreport.StatusPass {
				t.Fatalf("owned Schema type rejected: %+v", report)
			}
		})
	}
}
