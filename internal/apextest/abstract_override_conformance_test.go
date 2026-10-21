package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type abstractOverrideError struct {
	Class   string
	Line    int
	Message string
}

type abstractOverrideRow struct {
	Set, ID, APIVersion, Route string
	Source                     string
	Sources, ClassApis         map[string]string
	TestClass                  string
	ExpectedErrors             []abstractOverrideError
	ExpectedRuntime            string
	Terminal, Observed         string
}

// Native abstract, virtual and Object-method override rows. The anonymous and
// isTest transports return only the first compile problem; check-only deploys
// return every problem. Accepted rows replace only the tagged terminal
// assertion with an equality check on the native value.
func TestAbstractOverrideOrgConformance(t *testing.T) {
	var data struct {
		APIVersions []string
		Routes      []string
		Rows        []abstractOverrideRow
	}
	raw, err := os.ReadFile("testdata/conformance/abstract_override.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,63.0,64.0,65.0,66.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest,deploy" || len(data.Rows) != 447 {
		t.Fatalf("native matrix shape: APIs=%v routes=%v rows=%d", data.APIVersions, data.Routes, len(data.Rows))
	}
	type located struct {
		class, message string
		line           int
	}
	errorsOf := func(diagnostics []diagnostic.Diagnostic) []located {
		var out []located
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			message := d.NativeMessage
			if message == "" {
				message = d.Message
			}
			line := 0
			if d.NativeLine != nil {
				line = *d.NativeLine
			} else if d.Range != nil {
				line = d.Range.Start.Line
			}
			out = append(out, located{class: strings.TrimSuffix(filepath.Base(d.File), ".cls"), message: message, line: line})
		}
		return out
	}
	build := func(t *testing.T, api string, sources, classAPIs map[string]string) typesys.Index {
		t.Helper()
		root := t.TempDir()
		names := make([]string, 0, len(sources))
		for name := range sources {
			names = append(names, name)
		}
		sort.Strings(names)
		paths := make([]string, 0, len(names))
		for _, name := range names {
			path := filepath.Join(root, name+".cls")
			writeFile(t, path, sources[name])
			classAPI := api
			if override, ok := classAPIs[name]; ok {
				classAPI = override
			}
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+classAPI+"</apiVersion></ApexClass>")
			paths = append(paths, path)
		}
		return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
	}
	// Execute-anonymous declarations move to transient files at their original
	// line and column, as the product exec path prepares them.
	anonymous := func(t *testing.T, source string) (map[string]string, string) {
		t.Helper()
		parsed := apexast.ParseSource("__glade_anonymous.cls", source)
		files := map[string]string{}
		body := []byte(source)
		for _, decl := range parsed.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			default:
				continue
			}
			start, end := decl.Range.Start.Offset, decl.Range.End.Offset
			if start < 0 || end <= start || end > len(source) {
				t.Fatalf("invalid declaration range: %s", decl.Name)
			}
			prefix := []byte(source[:start])
			for i := range prefix {
				if prefix[i] != '\n' && prefix[i] != '\r' {
					prefix[i] = ' '
				}
			}
			files[fmt.Sprintf("GladeAnonymous%d", len(files))] = string(prefix) + source[start:end]
			for i := start; i < end; i++ {
				if body[i] != '\n' && body[i] != '\r' {
					body[i] = ' '
				}
			}
		}
		return files, string(body)
	}
	observe := func(t *testing.T, row abstractOverrideRow, source string) string {
		t.Helper()
		if row.Terminal == "" || row.Observed == "" || strings.Count(source, row.Terminal) != 1 {
			t.Fatal("missing captured observation assertion")
		}
		return strings.Replace(source, row.Terminal, "System.assertEquals("+conformanceApexString(row.ExpectedRuntime)+","+row.Observed+");", 1)
	}
	compare := func(t *testing.T, row abstractOverrideRow, local []located) {
		t.Helper()
		if row.Route != "deploy" && len(local) > 1 {
			// One problem per transport response; later local errors are not
			// observable through that route.
			local = local[:1]
		}
		// Anonymous blocks report a line; class inserts report the rejected
		// class; deploys report both for every problem.
		useClass := row.Route != "anonymous" || row.Source == ""
		useLine := row.Route == "deploy" || row.Source != ""
		format := func(class string, line int, message string) string {
			if !useClass {
				class = ""
			}
			if !useLine {
				line = 0
			}
			return fmt.Sprintf("%s:%d: %s", class, line, message)
		}
		var want, got []string
		for _, e := range row.ExpectedErrors {
			// The deploy also rejects the @IsTest class that depends on the
			// rejected subclass. Semantic analysis reports only the subclass's
			// own problems, without a separate dependent-class error.
			if strings.HasPrefix(e.Message, "Dependent class is invalid and needs recompilation:") {
				continue
			}
			want = append(want, format(e.Class, e.Line, e.Message))
		}
		for _, e := range local {
			got = append(got, format(e.class, e.line, e.message))
		}
		if strings.Join(want, "\n") != strings.Join(got, "\n") {
			t.Fatalf("native compilation\n%s\nlocal\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
		}
	}
	total := 0
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "abstract override", api)
			baseIndex := build(t, api, nil, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			seen := map[string]bool{}
			for _, row := range data.Rows {
				if row.APIVersion != api {
					continue
				}
				key := row.Route + "/" + row.Set + "/" + row.ID
				if seen[key] {
					t.Fatalf("duplicate native row %s", key)
				}
				seen[key] = true
				total++
				rejected := len(row.ExpectedErrors) != 0
				executes := row.TestClass != "" || row.Source != ""
				if rejected && row.ExpectedRuntime != "" || executes && !rejected && row.ExpectedRuntime == "" || !executes && row.ExpectedRuntime != "" {
					t.Fatalf("native row outcome: %#v", row)
				}
				counts.Run(t, row.Route, row.Set+"/"+row.ID, key, func(t *testing.T) {
					switch row.Route {
					case "anonymous":
						if row.Source == "" {
							// Companion rows compile one top-level class that the
							// anonymous block references; only its compilation is native.
							if len(row.Sources) != 1 || row.Set != "M2" {
								t.Fatalf("invalid anonymous row: %#v", row)
							}
							compare(t, row, errorsOf(sema.Analyze(build(t, api, row.Sources, nil)).Diagnostics))
							return
						}
						// AOJ rows extend a class saved in the org; a project
						// companion stands in for it, as the exec path merges a
						// loaded project with the transient declarations.
						files, body := anonymous(t, row.Source)
						if want := 2 - min(len(row.Sources), 1); len(files) != want {
							t.Fatalf("captured anonymous row declares %d types, want %d", len(files), want)
						}
						index := sema.WithAnonymousDeclarationContext(build(t, api, files, nil))
						analysisIndex, requestBase := index, base
						if len(row.Sources) != 0 {
							projectIndex := build(t, api, row.Sources, row.ClassApis)
							analysisIndex = projectIndex
							analysisIndex.Types = append(append([]typesys.TypeSymbol(nil), projectIndex.Types...), index.Types...)
							requestBase = newConformanceRunner(t, projectIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
						}
						local := errorsOf(sema.NativeLifecycleDiagnostics(index, index.Diagnostics))
						if len(local) == 0 {
							local = errorsOf(sema.AnalyzeAnonymousDeclarationsInContext(analysisIndex, index).Diagnostics)
						}
						if len(local) == 0 {
							local = errorsOf(sema.AnalyzeAnonymous(analysisIndex, body, api).Diagnostics)
						}
						compare(t, row, local)
						if rejected {
							return
						}
						body = observe(t, row, body)
						if analysis := sema.AnalyzeAnonymous(analysisIndex, body, api); analysis.HasErrors() {
							t.Fatalf("local assertion semantics: %#v", analysis.Diagnostics)
						}
						runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
						if err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(analysisIndex, body, api)})
						if err != nil {
							t.Fatal(err)
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: requestBase, RequestRuntime: &runtime, Org: &org})
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					case "isTest", "deploy":
						if row.Source != "" || len(row.Sources) == 0 {
							t.Fatalf("invalid named row: %#v", row)
						}
						index := build(t, api, row.Sources, row.ClassApis)
						compare(t, row, errorsOf(sema.Analyze(index).Diagnostics))
						if rejected || row.TestClass == "" {
							return
						}
						sources := make(map[string]string, len(row.Sources))
						for name, source := range row.Sources {
							sources[name] = source
						}
						sources[row.TestClass] = observe(t, row, sources[row.TestClass])
						executionIndex := build(t, api, sources, row.ClassApis)
						if analysis := sema.Analyze(executionIndex); analysis.HasErrors() {
							t.Fatalf("local assertion semantics: %#v", analysis.Diagnostics)
						}
						cases := Discover(executionIndex, Options{SelectedClasses: []string{row.TestClass}})
						if len(cases) != 1 {
							t.Fatalf("captured @IsTest method missing: %#v", cases)
						}
						methods, methodErrors := compileTestMethods(cases)
						programs, programErrors := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("test lowering: %v %v", methodErrors[key], programErrors[key])
						}
						runner := newConformanceRunner(t, executionIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatalf("unknown route %q", row.Route)
					}
				})
			}
		})
	}
	if total != len(data.Rows) {
		t.Fatalf("rows outside the API matrix: %d of %d", len(data.Rows)-total, len(data.Rows))
	}
}
