package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type dependencyGateCell struct {
	ID, API, Route, Group, State, Kind, Anonymous, Carry string
	Classes, Triggers, Objects                           map[string]string
	Tests                                                []string
	SourceSHA256                                         map[string]string `json:"sourceSha256"`
	Native                                               struct {
		Outcome     string
		Value       json.RawMessage
		Diagnostics struct {
			CompileProblem string
			Line           int
			Compile        []struct {
				Name, Type, Problem string
				Line                int
			}
		}
		ComponentFailures []struct {
			FullName, ComponentType, Problem string
			LineNumber                       int
		}
	}
}

func TestDependencyGateBaselineConformance(t *testing.T) {
	runDependencyGateConformance(t, "baseline", 96)
}
func TestDependencyGateSignatureConformance(t *testing.T) {
	runDependencyGateConformance(t, "signature", 96)
}
func TestDependencyGatePackagesConformance(t *testing.T) {
	runDependencyGateConformance(t, "packages", 9)
}
func TestDependencyGateTransitionConformance(t *testing.T) {
	runDependencyGateConformance(t, "transition", 3)
}

func runDependencyGateConformance(t *testing.T, cluster string, cells int) {
	t.Helper()
	var data struct {
		APIVersions            []string
		Cluster                string
		NativeObservationCount int
		Cells                  []dependencyGateCell
		ExcludedArtifacts      map[string][]json.RawMessage
	}
	raw, err := os.ReadFile("testdata/conformance/dependency_gate_" + cluster + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || len(data.Cells) != cells || data.NativeObservationCount != cells || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete native matrix")
	}
	for artifact, rows := range data.ExcludedArtifacts {
		if len(rows) != 0 {
			t.Fatalf("unexpected %s artifacts: %d", artifact, len(rows))
		}
	}
	counts := map[string]*conformanceCounts{}
	for _, api := range data.APIVersions {
		counts[api] = newConformanceCountLog(t, "dependency-gate-"+cluster, api)
	}
	bases := newConformanceSchemaCache(t)
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.ID + "/" + cell.Route
		if seen[key] || counts[cell.API] == nil {
			t.Fatalf("repeated or unsupported native cell %s", key)
		}
		seen[key] = true
		runConformanceSubtestParallel(t, cell.API+"/"+cell.ID+"/"+cell.Route, func(t *testing.T) {
			dependencyGateCheckSources(t, cell)
			if cell.Carry != "" && cell.Kind != "package" {
				t.Logf("gate scope an invalid trigger dependency or file less dependency_missing should not carry: %s; native=%s", cell.Carry, cell.Native.Outcome)
				return
			}
			if cell.Carry == "" {
				route := cell.Route
				if route == "istest" {
					route = "isTest"
				}
				kind := "exact"
				if dependencyGateCategoryOnly(cell) {
					kind = "category"
				}
				counts[cell.API].TrackKind(t, route, kind, 1)
			}
			index := dependencyGateIndex(t, cell)
			if cell.Kind == "package" {
				dependencyGatePackage(t, cell, index)
				return
			}
			if cell.State == "signature" {
				gate := dependencyGateExclusions(t, index)
				if cell.Native.Outcome == "compile-rejected" {
					primary := cell.Native.Diagnostics.CompileProblem
					if len(cell.Native.Diagnostics.Compile) > 0 {
						primary = cell.Native.Diagnostics.Compile[0].Problem
					}
					if cell.Route == "istest" {
						if len(cell.Tests) != 1 {
							t.Fatal("one captured observation test required")
						}
						entry, excluded := gate.classEntry[strings.ToLower(cell.Tests[0])]
						if !excluded {
							t.Fatalf("native rejects observation test <%s>; no compile exclusion", primary)
						}
						observer := gate.excluded[entry]
						if dependencyGateCategoryOnly(cell) {
							want := cell.Native.Diagnostics.Compile[0]
							if observer.ClassName != want.Name || observer.Line != want.Line || observer.DependsOn != "" {
								t.Fatalf("native observer unit %s:%d; local exclusion %#v", want.Name, want.Line, observer)
							}
							t.Logf("category-only rejection: native <%s>; observer <%s>", primary, observer.Message)
						} else if observer.Message != primary {
							t.Fatalf("native <%s>; observer <%s>", primary, observer.Message)
						}
						return
					}
					// An anonymous reference to an invalid saved helper carries that
					// helper's diagnostic. Named observers above keep their own compile
					// unit's error before subset filtering removes it.
					source := cell.Anonymous
					units := newCompileGateUnits(index)
					for _, ref := range compileGateReferences("Observation.cls", source, units) {
						if entry, ok := gate.classEntry[ref]; ok {
							dependent := compileGateDependentPrefix + gate.excluded[entry].ClassName + " : " + gate.excluded[entry].Message
							if dependent != primary {
								t.Fatalf("native <%s>; gate <%s>", primary, dependent)
							}
							return
						}
					}
					index = filterCompileGateIndex(index, gate.excludedFiles)
				} else {
					// Independent anonymous observations use only valid source units. This
					// projection does not model a hosted Metadata test-validation lifecycle.
					index = filterCompileGateIndex(index, gate.excludedFiles)
				}
			}
			dependencyGateExecute(t, bases, cell, index)
		})
	}
}

// These rows assert rejection category, source unit, position and dependency
// scope. Native prose stays in the fixture; local diagnostics keep their prose.
func dependencyGateCategoryOnly(cell dependencyGateCell) bool {
	switch cell.ID {
	case "C180GS_013", "C180GS_015", "C180GS_029", "C180GS_031", "C180GS_034", "C180GS_035":
		return true
	}
	return false
}

func dependencyGateCheckSources(t *testing.T, cell dependencyGateCell) {
	t.Helper()
	sources := map[string]string{}
	for folder, entries := range map[string]map[string]string{"classes": cell.Classes, "triggers": cell.Triggers, "objects": cell.Objects} {
		for name, source := range entries {
			sources[folder+":"+name] = source
		}
	}
	if cell.Anonymous != "" {
		sources["anonymous"] = cell.Anonymous
	}
	if len(sources) != len(cell.SourceSHA256) {
		t.Fatal("source hash set differs")
	}
	for name, source := range sources {
		digest := sha256.Sum256([]byte(source))
		if hex.EncodeToString(digest[:]) != cell.SourceSHA256[name] {
			t.Fatalf("changed native source %s", name)
		}
	}
}

func dependencyGateIndex(t *testing.T, cell dependencyGateCell) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, cell.API))
	for folder, entries := range map[string]map[string]string{"classes": cell.Classes, "triggers": cell.Triggers, "objects": cell.Objects} {
		for name, source := range entries {
			extension, kind := ".cls", "ApexClass"
			if folder == "triggers" {
				extension, kind = ".trigger", "ApexTrigger"
			}
			if folder == "objects" {
				extension = ".object"
			}
			path := filepath.Join(root, "force-app/main/default", folder, name+extension)
			writeFile(t, path, source)
			if folder != "objects" {
				writeFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+cell.API+"</apiVersion><status>Active</status></"+kind+">")
			}
		}
	}
	return loadTestIndex(t, root)
}

func dependencyGateExclusions(t *testing.T, index typesys.Index) *compileGate {
	t.Helper()
	units := newCompileGateUnits(index)
	diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
	byFile, ok := units.attribute(diagnostics)
	if !ok {
		t.Fatal("compile errors fall back project-wide")
	}
	gate := &compileGate{excludedFiles: map[string]bool{}, fileEntry: map[string]int{}, classEntry: map[string]int{}, rootProblems: map[string]string{}}
	sources := newSourceCache()
	if !gate.exclude(index, units, byFile, sources) {
		t.Fatal("trigger dependency falls back project-wide")
	}
	return gate
}

func dependencyGatePackage(t *testing.T, cell dependencyGateCell, index typesys.Index) {
	t.Helper()
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
	if cell.ID == "C180GS_033" {
		// Native rejection here is coverage-only. Running the valid twin is a local
		// compile/runtime check, not an assertion of hosted deployment acceptance.
		t.Logf("gate scope an invalid trigger dependency or file less dependency_missing should not coverage carry: %s", cell.Carry)
		if run.Summary().Passed != 2 || len(run.ExcludedClasses) != 0 {
			t.Fatalf("valid package twin: %#v", run)
		}
		return
	}
	if run.Summary().Passed != 2 || run.Summary().OK() {
		t.Fatalf("partial-run policy: %#v", run)
	}
	expected := map[string]testreport.CompileProblem{}
	nativeChains := map[string][]string{}
	for _, failure := range cell.Native.ComponentFailures {
		expected[failure.FullName] = testreport.CompileProblem{Line: failure.LineNumber, Message: failure.Problem}
		if failure.ComponentType == "ApexClass" {
			nativeChains[failure.FullName] = compileGateNativeChain(failure.Problem)
		}
	}
	if len(run.ExcludedClasses) != len(expected) {
		t.Fatalf("excluded=%v native=%v", run.ExcludedClasses, expected)
	}
	for _, entry := range run.ExcludedClasses {
		want, exists := expected[entry.ClassName]
		if !exists || entry.Line != want.Line {
			t.Errorf("native excluded unit %s:%d; local %#v", entry.ClassName, want.Line, entry)
			continue
		}
		if chain, class := nativeChains[entry.ClassName]; class {
			if len(chain) == 0 {
				if entry.Root != entry.ClassName || entry.DependsOn != "" {
					t.Errorf("native root unit %s; local %#v", entry.ClassName, entry)
				}
			} else if entry.DependsOn != chain[0] || entry.Root != chain[len(chain)-1] {
				t.Errorf("native dependency chain %v; local %#v", chain, entry)
			}
			t.Logf("category-only rejection: %s native <%s>; gate <%s>", entry.ClassName, want.Message, entry.Message)
		} else if want.Message != entry.Message {
			t.Errorf("trigger %s native <%s>; gate <%s>", entry.ClassName, want.Message, entry.Message)
		}
	}
}

func dependencyGateExecute(t *testing.T, bases *conformanceSchemaCache, cell dependencyGateCell, index typesys.Index) {
	t.Helper()
	var diagnostics []diagnostic.Diagnostic
	body := cell.Anonymous
	if cell.Route == "anonymous" {
		diagnostics = sema.AnalyzeAnonymous(index, body, cell.API).Diagnostics
	} else {
		diagnostics = append(index.Diagnostics, sema.Analyze(index).Diagnostics...)
	}
	if cell.Native.Outcome == "compile-rejected" {
		primary := cell.Native.Diagnostics.CompileProblem
		if len(cell.Native.Diagnostics.Compile) > 0 {
			primary = cell.Native.Diagnostics.Compile[0].Problem
		}
		for _, diag := range diagnostics {
			if diag.Severity == diagnostic.Error {
				message := diag.NativeMessage
				if message == "" {
					message = diag.Message
				}
				if dependencyGateCategoryOnly(cell) {
					line := cell.Native.Diagnostics.Line
					if cell.Route == "istest" {
						want := cell.Native.Diagnostics.Compile[0]
						line = want.Line
						if filepath.Base(diag.File) != want.Name+".cls" {
							t.Fatalf("native rejects unit %s; local file %s", want.Name, diag.File)
						}
					}
					localLine := 0
					if diag.Range != nil {
						localLine = diag.Range.Start.Line
					}
					if diag.NativeLine != nil {
						localLine = *diag.NativeLine
					}
					if localLine != line {
						t.Fatalf("native rejection line %d; local %d", line, localLine)
					}
					t.Logf("category-only rejection: native <%s>; compiler <%s>", primary, message)
				} else if message != primary {
					t.Fatalf("native <%s>; compiler <%s>", primary, message)
				}
				return
			}
		}
		t.Fatalf("native rejects <%s>; no local compile error", primary)
	}
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error {
			t.Fatalf("native executes; compiler=%v", diagnostics)
		}
	}
	base := bases.get(t, cell.API, "", cell.Objects, func(t *testing.T) *conformanceRunner {
		metadata := dependencyGateCell{API: cell.API, Objects: cell.Objects}
		return conformanceSchemaBase(t, dependencyGateIndex(t, metadata), cell.API)
	})
	runtime := conformanceSchemaRequestRuntime(t, index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: base.orgSource})
	var execErr error
	if cell.Route == "anonymous" {
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: cell.API})
		if err != nil {
			t.Fatal(err)
		}
		_, execErr = runner.execute(program)
	} else {
		cases := Discover(index, Options{SelectedClasses: cell.Tests, SelectedMethod: "observe"})
		if len(cases) != 1 {
			t.Fatalf("observation discovery: %d", len(cases))
		}
		methods, methodErrs := compileTestMethods(cases)
		programs, programErrs := compileTestInvokePrograms(cases)
		key := testCaseKey(cases[0])
		if methodErrs[key] != nil || programErrs[key] != nil {
			t.Fatalf("test compile %v %v", methodErrs, programErrs)
		}
		machine := runner.newMachine()
		machine.EnableTestContext()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native marker missing: %v", execErr)
	}
	if runtimeErr.Type != "System.AssertException" {
		t.Fatalf("wrong marker transport: %v", execErr)
	}
	marker := "C126_RESULT|" + cell.ID + "|"
	_, tail, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
	if !found || strings.Count(runtimeErr.ExceptionMessage(), "C126_RESULT|") != 1 {
		t.Fatal("invalid observation marker")
	}
	var got, want map[string]any
	if err := json.Unmarshal([]byte(tail), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cell.Native.Value, &want); err != nil {
		t.Fatal(err)
	}
	// Only the exception type receives System qualification. Messages stay exact.
	for _, value := range []map[string]any{got, want} {
		if typ, ok := value["exceptionType"].(string); ok && !strings.Contains(typ, ".") {
			value["exceptionType"] = "System." + typ
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native %s; local %s", cell.Native.Value, tail)
	}
}
