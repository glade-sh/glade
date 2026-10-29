package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestReflectionConstructionNativeConformanceAnonymousAndIsTestProjectAPIRoutes(t *testing.T) {
	runMethodObservationNativeConformance(t, "reflection_construction", "ReflectConstructionProbeTest", 80)
}

func TestFinalizerExecutionNativeConformanceAnonymousAndIsTestProjectAPIRoutes(t *testing.T) {
	runMethodObservationNativeConformance(t, "finalizer_execution", "TailExecutionProbeTest", 24)
}

// Native rows cover only anonymous and @IsTest execution in an unpackaged
// scratch org. Namespaced package lookup and access cases remain pending.
func runMethodObservationNativeConformance(t *testing.T, family, class string, rowCount int) {
	t.Helper()
	fixture := filepath.Join("testdata", "conformance", family)
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != rowCount+1 || strings.Join(rows[0], ",") != "source_api,route,method,observation" {
		t.Fatal("incomplete native method matrix")
	}
	methodCount := 20
	if family == "finalizer_execution" {
		methodCount = 6
	}
	byAPI := map[string]map[string]map[string]string{
		"62.0": {"anonymous": {}, "istest": {}},
		"67.0": {"anonymous": {}, "istest": {}},
	}
	for _, row := range rows[1:] {
		if len(row) != 4 || byAPI[row[0]] == nil || byAPI[row[0]][row[1]] == nil || row[2] == "" || !json.Valid([]byte(row[3])) {
			t.Fatalf("invalid native row: %v", row)
		}
		if _, exists := byAPI[row[0]][row[1]][row[2]]; exists {
			t.Fatalf("duplicate native method: %v", row)
		}
		byAPI[row[0]][row[1]][row[2]] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			if len(byAPI[api][route]) != methodCount {
				t.Fatalf("API %s route %s rows=%d, want %d", api, route, len(byAPI[api][route]), methodCount)
			}
		}
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != methodCount || len(byAPI[api]["istest"])*2 != rowCount/2 {
				t.Fatalf("incomplete local method selection: %+v", run.Summary())
			}
			seen := map[string]bool{}
			for _, result := range run.Suites[0].Cases {
				t.Run("istest/"+result.MethodName, func(t *testing.T) {
					native, exists := byAPI[api]["istest"][result.MethodName]
					if !exists || seen[result.MethodName] {
						t.Fatalf("unexpected method: %s", result.MethodName)
					}
					seen[result.MethodName] = true
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing assertion transport: %+v", result.Problem)
					}
					compareMethodObservation(t, native, result.Problem.Message)
				})
			}
			for method := range byAPI[api]["anonymous"] {
				t.Run("anonymous/"+method, func(t *testing.T) {
					compareAnonymousMethodObservation(t, family, method, api, index, byAPI[api]["anonymous"][method])
				})
			}
		})
	}
}

func compareAnonymousMethodObservation(t *testing.T, family, method, api string, index typesys.Index, native string) {
	t.Helper()
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: func() *storage.OrgState {
		org := orgFromIndex(index)
		org.APIVersion = api
		return &org
	}()})
	if family == "reflection_construction" {
		source := reflectionAnonymousObservationSource(method)
		analysis := sema.AnalyzeAnonymous(index, source, api)
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
		if analysis.HasErrors() || err != nil {
			t.Fatalf("anonymous compile: %v %v", analysis.Diagnostics, err)
		}
		_, err = runner.execute(program)
		compareMethodObservation(t, native, anonymousAssertionMessage(t, err))
		return
	}

	if family != "finalizer_execution" {
		t.Fatalf("unknown method-observation family %q", family)
	}
	if method == "outside" {
		source := `String observed; try { System.attachFinalizer(new TailExecutionProbe.Tail('outside')); } catch (Exception error) { observed = error.getTypeName() + '|' + error.getMessage(); } System.assert(false, 'OBS|' + JSON.serialize(observed));`
		_, err := runAnonymousObservation(t, runner, index, source, api)
		compareMethodObservation(t, native, anonymousAssertionMessage(t, err))
		return
	}
	if !strings.Contains(",callerCounters,success,failure,child,duplicate,", ","+method+",") {
		t.Fatalf("unknown finalizer method %q", method)
	}
	enqueue := ""
	if method == "callerCounters" {
		enqueue = "insert new Account(Name = 'Tail observer caller');\n"
	}
	enqueue += "System.enqueueJob(new TailExecutionProbe.Work('" + method + "'));"
	machine := (*vm.VM)(nil)
	var result vm.Result
	result, err := runAnonymousObservationWithMachine(t, runner, index, enqueue, api, &machine)
	if err != nil {
		t.Fatalf("anonymous enqueue: %v", err)
	}
	drainErr := machine.DrainAsync(&result)
	if method == "failure" {
		if drainErr == nil || !strings.Contains(drainErr.Error(), "body interrupted") {
			t.Fatalf("failed queueable error = %v", drainErr)
		}
	} else if drainErr != nil {
		t.Fatalf("anonymous queueable drain: %v", drainErr)
	}
	_, err = runAnonymousObservationOnMachine(t, machine, index, "System.assert(false, 'OBS|' + TailExecutionProbe.report());", api)
	compareMethodObservation(t, native, anonymousAssertionMessage(t, err))
}

func runAnonymousObservation(t *testing.T, runner *conformanceRunner, index typesys.Index, source, api string) (vm.Result, error) {
	var machine *vm.VM
	return runAnonymousObservationWithMachine(t, runner, index, source, api, &machine)
}

func runAnonymousObservationWithMachine(t *testing.T, runner *conformanceRunner, index typesys.Index, source, api string, machine **vm.VM) (vm.Result, error) {
	t.Helper()
	analysis := sema.AnalyzeAnonymous(index, source, api)
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
	if analysis.HasErrors() || err != nil {
		t.Fatalf("anonymous compile: %v %v", analysis.Diagnostics, err)
	}
	return runner.execute(program, func(current *vm.VM) { *machine = current })
}

func runAnonymousObservationOnMachine(t *testing.T, machine *vm.VM, index typesys.Index, source, api string) (vm.Result, error) {
	t.Helper()
	analysis := sema.AnalyzeAnonymous(index, source, api)
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
	if analysis.HasErrors() || err != nil {
		t.Fatalf("anonymous compile: %v %v", analysis.Diagnostics, err)
	}
	return machine.Execute(program)
}

func anonymousAssertionMessage(t *testing.T, err error) string {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing anonymous assertion transport: %v", err)
	}
	return runtimeErr.ExceptionMessage()
}

func compareMethodObservation(t *testing.T, native, local string) {
	t.Helper()
	_, observed, found := strings.Cut(local, "OBS|")
	var got, want any
	if !found || json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(native), &want) != nil {
		t.Fatalf("invalid observation transport: native=%s; local=%q", native, local)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%s; Glade=%s", native, observed)
	}
}

func reflectionAnonymousObservationSource(method string) string {
	targets := map[string]string{
		"userInfo":                  "Type.forName('UserInfo')",
		"qualifiedUserInfo":         "Type.forName('System.UserInfo')",
		"systemNamespace":           "Type.forName('System', 'UserInfo')",
		"plain":                     "Type.forName('ReflectConstructionProbe.Plain')",
		"privateConstructor":        "Type.forName('ReflectConstructionProbe.PrivateConstructor')",
		"protectedConstructor":      "Type.forName('ReflectConstructionProbe.ProtectedConstructor')",
		"argumentConstructor":       "Type.forName('ReflectConstructionProbe.ArgumentConstructor')",
		"abstractValue":             "Type.forName('ReflectConstructionProbe.AbstractValue')",
		"contract":                  "Type.forName('ReflectConstructionProbe.Contract')",
		"throwingConstructor":       "Type.forName('ReflectConstructionProbe.ThrowingConstructor')",
		"emptyNamespace":            "Type.forName('', 'ReflectConstructionProbe.Plain')",
		"nullNamespace":             "Type.forName(null, 'ReflectConstructionProbe.Plain')",
		"missingNamespace":          "Type.forName('missingzone', 'ReflectConstructionProbe.Plain')",
		"literalUserInfo":           "UserInfo.class",
		"splitNested":               "Type.forName('ReflectConstructionProbe', 'Plain')",
		"repeatedOuter":             "Type.forName('ReflectConstructionProbe', 'ReflectConstructionProbe.Plain')",
		"qualifiedMissingNamespace": "Type.forName('missingzone', 'System.UserInfo')",
		"emptyQualifiedUserInfo":    "Type.forName('', 'System.UserInfo')",
		"nullQualifiedUserInfo":     "Type.forName(null, 'System.UserInfo')",
		"systemQualifiedUserInfo":   "Type.forName('System', 'System.UserInfo')",
	}
	target, ok := targets[method]
	if !ok {
		return "System.assert(false, 'OBS|unknown-method');"
	}
	return "System.assert(false, 'OBS|' + ReflectConstructionProbe.capture(" + target + "));"
}
