package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSystemExceptionHierarchyNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/system_exception_hierarchy"
	read := func(name string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	var specs []struct {
		Type, Parent, Operation, Method, Class, Source string
		Classes                                        map[string]string
	}
	caseSource := read("cases.json")
	if err := json.Unmarshal(caseSource, &specs); err != nil || len(specs) != 229 {
		t.Fatalf("incomplete captured cases: %v", err)
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			var capture struct {
				SourceAPI, CasesSHA256  string
				Types, Cases, Rows      int
				TestMethods             int
				ScratchDeleted          bool
				RuntimeSourceSHA256     map[string]string
				ScratchDeletionVerified struct {
					ScratchOrgInfo        string
					ActiveScratchOrgCount int
				}
			}
			if err := json.Unmarshal(read("native/api"+api+"/RUN-FACTS.txt"), &capture); err != nil {
				t.Fatal(err)
			}
			if capture.SourceAPI != api+".0" || capture.Types != 52 || capture.Cases != len(specs) || capture.Rows != 458 || capture.TestMethods != 168 ||
				false || // scratch cleanup facts are not published
				capture.CasesSHA256 != fmt.Sprintf("%x", sha256.Sum256(caseSource)) {
				t.Fatal("incomplete or changed native capture")
			}
			type nativeResult struct {
				Compiled, Success                bool
				CompileProblem, ExceptionMessage string
			}
			reader := csv.NewReader(strings.NewReader(string(read("native/api" + api + "/org.tsv"))))
			reader.Comma, reader.LazyQuotes = '\t', true
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != capture.Rows+1 || strings.Join(rows[0], "\t") != "type\toperation\troute\tresult" {
				t.Fatalf("incomplete native rows: %v", err)
			}
			native := map[string]nativeResult{}
			key := func(name, operation, route string) string { return name + "/" + operation + "/" + route }
			for _, row := range rows[1:] {
				var result nativeResult
				if len(row) != 4 || json.Unmarshal([]byte(row[3]), &result) != nil {
					t.Fatal("invalid native row")
				}
				k := key(row[0], row[1], row[2])
				if _, exists := native[k]; exists {
					t.Fatalf("duplicate native row: %s", k)
				}
				native[k] = result
			}
			classes := map[string]string{}
			if err := json.Unmarshal(read("classes.json"), &classes); err != nil {
				t.Fatal(err)
			}
			var source strings.Builder
			source.WriteString("@IsTest private class LarchHierarchyTest {\n")
			for _, spec := range specs {
				if spec.Operation != "extend" && native[key(spec.Type, spec.Operation, "isTest")].Compiled {
					fmt.Fprintf(&source, "    @IsTest static void %s() { %s }\n", spec.Method, spec.Source)
				}
			}
			source.WriteString("}\n")
			classes["LarchHierarchyTest"] = source.String()
			if len(classes) != len(capture.RuntimeSourceSHA256) {
				t.Fatal("incomplete native source set")
			}
			for name, body := range classes {
				if fmt.Sprintf("%x", sha256.Sum256([]byte(body))) != capture.RuntimeSourceSHA256[name] {
					t.Fatalf("native source changed: %s", name)
				}
			}
			index := nativeBoundaryIndex(t, api+".0", classes, nil)
			t.Run("isTest/projectCompilation", func(t *testing.T) {
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native project compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
			})
			cases := Discover(index, Options{SelectedClasses: []string{"LarchHierarchyTest"}})
			if len(cases) != capture.TestMethods {
				t.Fatalf("discovered %d methods; native=%d", len(cases), capture.TestMethods)
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			bareIndex := nativeBoundaryIndex(t, api+".0", nil, nil)
			anonymousRunner := newConformanceRunner(t, bareIndex, conformanceRunnerOptions{LinkProject: true})
			for _, route := range []string{"anonymous", "isTest"} {
				for _, spec := range specs {
					t.Run(route+"/"+spec.Type+"/"+spec.Operation, func(t *testing.T) {
						want, exists := native[key(spec.Type, spec.Operation, route)]
						if !exists {
							t.Fatal("missing native route")
						}
						if spec.Operation == "extend" {
							declarations := nativeBoundaryIndex(t, api+".0", map[string]string{spec.Class: spec.Source}, nil)
							analysis := sema.Analyze(declarations)
							if route == "anonymous" {
								analysis = sema.AnalyzeAnonymousDeclarations(sema.WithAnonymousDeclarationContext(declarations))
							}
							if got := !declarations.HasErrors() && !analysis.HasErrors(); got != want.Compiled {
								t.Fatalf("extension compiled=%v; native=%v (%s); local=%v %v", got, want.Compiled, want.CompileProblem, declarations.Diagnostics, analysis.Diagnostics)
							}
							if strings.HasPrefix(want.CompileProblem, "Non-virtual and non-abstract type cannot be extended:") {
								found := false
								for _, d := range analysis.Diagnostics {
									found = found || d.NativeMessage == want.CompileProblem
								}
								if !found {
									t.Fatalf("missing native final-type diagnostic: %s; local=%v", want.CompileProblem, analysis.Diagnostics)
								}
							}
							return
						}
						if !want.Compiled {
							analysis := sema.AnalyzeAnonymous(bareIndex, spec.Source, api+".0")
							if route == "isTest" {
								rejected := nativeBoundaryIndex(t, api+".0", map[string]string{"LarchRejectedTest": "@IsTest private class LarchRejectedTest { @IsTest static void observe() { " + spec.Source + " } }"}, nil)
								analysis = sema.Analyze(rejected)
							}
							if !analysis.HasErrors() {
								t.Fatalf("native rejected %s; Glade accepted", want.CompileProblem)
							}
							return
						}
						var execErr error
						if route == "isTest" {
							for _, c := range cases {
								if c.MethodName != spec.Method {
									continue
								}
								k := testCaseKey(c)
								if methodErrors[k] != nil || programErrors[k] != nil {
									t.Fatalf("compile: %v %v", methodErrors[k], programErrors[k])
								}
								machine := runner.newMachine()
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[k]); err != nil {
									t.Fatal(err)
								}
								_, execErr = machine.ExecuteInClass(programs[k], c.ClassName)
								break
							}
						} else {
							localIndex, localRunner := bareIndex, anonymousRunner
							if len(spec.Classes) != 0 {
								localIndex = sema.WithAnonymousDeclarationContext(nativeBoundaryIndex(t, api+".0", spec.Classes, nil))
								if analysis := sema.AnalyzeAnonymousDeclarations(localIndex); analysis.HasErrors() {
									t.Fatalf("anonymous declarations: %v", analysis.Diagnostics)
								}
								localRunner = newConformanceRunner(t, localIndex, conformanceRunnerOptions{LinkProject: true, Base: anonymousRunner})
							}
							if analysis := sema.AnalyzeAnonymous(localIndex, spec.Source, api+".0"); analysis.HasErrors() {
								t.Fatalf("anonymous diagnostics: %v", analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(spec.Source, vm.CompileOptions{APIVersion: api + ".0"})
							if err != nil {
								t.Fatal(err)
							}
							_, execErr = localRunner.newMachine().Execute(program)
						}
						var fault *vm.RuntimeError
						if !errors.As(execErr, &fault) {
							t.Fatalf("missing terminal observation: %v", execErr)
						}
						_, nativeValue, observed := strings.Cut(want.ExceptionMessage, "OBSERVED|")
						if observed {
							_, localValue, found := strings.Cut(fault.ExceptionMessage(), "OBSERVED|")
							if !found || localValue != nativeValue {
								t.Fatalf("native observation=%s; Glade=%s: %s", nativeValue, fault.Type, fault.ExceptionMessage())
							}
						} else {
							nativeType, nativeMessage, _ := strings.Cut(want.ExceptionMessage, ": ")
							localType := fault.Type
							if !strings.Contains(localType, ".") {
								localType = "System." + localType
							}
							if localType != nativeType || fault.ExceptionMessage() != nativeMessage {
								t.Fatalf("native uncaught=%s; Glade=%s: %s", want.ExceptionMessage, localType, fault.ExceptionMessage())
							}
						}
					})
				}
			}
		})
	}
}
