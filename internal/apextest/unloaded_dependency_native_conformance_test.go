package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestUnloadedDependencyNamedTestRunnerAndLSPNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "unloaded_dependency")
	var cases []struct {
		ID, State, Source, Outcome string
		Trigger                    bool
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	// The same native Apex slice also covers a local package identity matching
	// the unresolved qualifier, which prepends a resolver problem in the gate.
	for _, cell := range append(cases[:0:0], cases...) {
		if cell.State == "absent" && cell.Outcome == "compile-rejected" {
			twin := cell
			twin.ID += "_resolver_first"
			cases = append(cases, twin)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		f, err := os.Open(filepath.Join(dir, "api"+api[:2], "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		f.Close()
		if err != nil || len(rows) != 17 {
			t.Fatalf("native matrix: rows=%d error=%v", len(rows), err)
		}
		native := map[string][]string{}
		for _, row := range rows[1:] {
			if len(row) != 5 || row[1] != api || row[4] == "" {
				t.Fatal("invalid native row", row)
			}
			native[row[0]+"/"+row[2]] = row
		}
		for _, cell := range cases {
			t.Run(api+"/"+cell.ID+"/named_test_runner_lsp", func(t *testing.T) {
				for _, route := range []string{"anonymous", "istest"} {
					row := native[strings.TrimSuffix(cell.ID, "_resolver_first")+"/"+route]
					if len(row) != 5 || row[3] != cell.Outcome {
						t.Fatalf("native %s: %v", route, row)
					}
				}
				dependencyName := "cedar-library"
				if strings.HasSuffix(cell.ID, "_resolver_first") {
					dependencyName = "CedarLibrary" + api[:2]
				}
				dependencies := `[{"package":"` + dependencyName + `"}]`
				if cell.ID == "member_rejection" {
					dependencies = `[{"package":"cedar-library"},{"package":"cedar-optional"}]`
				}
				workspace := t.TempDir()
				root := filepath.Join(workspace, "consumer")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":`+dependencies+`}]}`)
				subst := func(s string) string {
					return strings.ReplaceAll(strings.ReplaceAll(s, "CedarLibrary", "CedarLibrary"+api[:2]), "CedarGate", "CedarGate"+api[:2])
				}
				writeClass := func(directory, name, source string) {
					path := filepath.Join(directory, "force-app/main/default/classes", name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				if cell.State == "loaded" {
					supplier := filepath.Join(workspace, "supplier")
					writeFile(t, filepath.Join(supplier, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"cedar-library"}]}`)
					body, err := os.ReadFile(filepath.Join(dir, "sources/CedarLibrary.cls"))
					if err != nil {
						t.Fatal(err)
					}
					writeClass(supplier, "CedarLibrary"+api[:2], subst(string(body)))
				}
				if cell.Trigger {
					body, err := os.ReadFile(filepath.Join(dir, "sources/CedarGate.trigger"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default/triggers", "CedarGate"+api[:2]+".trigger")
					writeFile(t, path, subst(string(body)))
					writeFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
				}
				name := "CedarObserve" + api[:2]
				writeClass(root, name, "@IsTest private class "+name+" { @IsTest static void observe() { "+subst(cell.Source)+" } }")
				writeClass(root, "CedarIndependent", "@IsTest private class CedarIndependent { @IsTest static void observe() { System.assertEquals(7,7); } }")
				index := loadTestIndex(t, root)
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				var observed *testreport.Case
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.ClassName == "CedarIndependent" && result.Status != testreport.StatusPass {
							t.Fatalf("unrelated route: %+v", result)
						}
						if result.ClassName == name {
							copy := result
							observed = &copy
						}
					}
				}
				if observed == nil {
					t.Fatalf("observer missing: %+v", run.Summary())
				}
				if cell.Outcome == "accepted" {
					if observed.Status != testreport.StatusPass || observed.Problem != nil || !run.Summary().OK() {
						t.Fatalf("native assertions passed; local=%+v summary=%+v", observed, run.Summary())
					}
					return
				}
				if cell.State == "loaded" {
					if observed.Status == testreport.StatusPass || observed.Problem == nil || observed.Problem.Type == "dependency_missing" {
						t.Fatalf("known-class rejection: %+v", observed)
					}
					return
				}
				if observed.Status == testreport.StatusPass || observed.Problem == nil || observed.Problem.Type != "dependency_missing" || !strings.Contains(observed.Problem.Message, dependencyName) {
					t.Fatalf("native rejects absent dependency; want dependency_missing, got %+v problem=%+v exclusions=%+v", observed, observed.Problem, run.ExcludedClasses)
				}
				if observed.Problem.Detail == "" || len(observed.Problem.Stack) == 0 {
					t.Fatalf("dependency context lost the original detail or stack: %+v", observed.Problem)
				}
				if strings.HasPrefix(cell.ID, "missing_static") || strings.HasPrefix(cell.ID, "missing_constructor") {
					if observed.Status != testreport.StatusCompileError {
						t.Fatalf("native compile rejection must retain its phase: %+v", observed)
					}
				}
				if strings.HasPrefix(cell.ID, "missing_trigger") {
					if observed.Status != testreport.StatusRuntimeError || string(observed.Reason) != "dependency-missing" {
						t.Fatalf("runtime dependency mapping: %+v", observed)
					}
				}
				diagnostics := lsp.TestDiagnostics(run)
				found := false
				for _, diag := range diagnostics {
					if strings.Contains(diag.Message, "dependency_missing") && strings.Contains(diag.Message, dependencyName) {
						found = true
					}
				}
				if !found {
					t.Fatalf("LSP lost dependency diagnostic: %+v", diagnostics)
				}
			})
		}
	}
}

func TestUnloadedDependencyRuntimeReasonAndPreparationRoute(t *testing.T) {
	err := &vm.RuntimeError{Type: "dependency_missing", Message: "unresolved Apex reference; unloaded declared dependency"}
	if got := terminalReason(err); string(got) != "dependency-missing" {
		t.Fatalf("runtime reason=%q, want dependency-missing", got)
	}
	got := preInvocationErrorCase(TestCase{ClassName: "CedarSetup", MethodName: "observe"}, err)
	if got.Status != testreport.StatusRuntimeError || string(got.Reason) != "dependency-missing" || got.Problem.Type != "dependency_missing" {
		t.Fatalf("setup dependency diagnostic: %+v", got)
	}
}

// Local diagnostic policy: a declared unavailable package does not turn a
// known platform type's missing member into an absent dependency.
func TestUnloadedDependencyKnownPlatformNamedPolicy(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/named_test_runner_lsp", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-optional"}]}]}`)
			path := filepath.Join(root, "force-app/classes/CedarPlatformTest.cls")
			writeFile(t, path, `@IsTest private class CedarPlatformTest { @IsTest static void observe() { System.String.absentMember(); } }`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("missing platform test: %+v", run.Summary())
			}
			observed := run.Suites[0].Cases[0]
			if observed.Status == testreport.StatusPass || observed.Problem == nil || observed.Problem.Type == "dependency_missing" {
				t.Fatalf("known platform type relabeled: %+v problem=%+v", observed, observed.Problem)
			}
			for _, diag := range lsp.TestDiagnostics(run) {
				if strings.Contains(diag.Message, "dependency_missing") {
					t.Fatalf("known type mislabeled in LSP: %+v", diag)
				}
			}
		})
	}
}

func TestUnloadedDependencyExcludedTriggerSnapshotRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/excluded_trigger_test_runner", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-library"}]}]}`)
			path := filepath.Join(root, "force-app/classes/CedarSnapshotTest.cls")
			writeFile(t, path, `@IsTest private class CedarSnapshotTest { @IsTest static void observe() { insert new Account(Name='boundary'); } }`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			trigger := filepath.Join(root, "force-app/triggers/CedarGate.trigger")
			writeFile(t, trigger, `trigger CedarGate on Account (before insert) { CedarLibrary.apply(Trigger.new); }`)
			writeFile(t, trigger+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion></ApexTrigger>")
			index := loadTestIndex(t, root)
			changed := false
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1, Progress: func(progress TestProgress) {
				if !changed && progress.Event == "compile_done" && progress.Status == "fail" {
					changed = true
					writeFile(t, trigger, `trigger CedarGate on Account (before insert) { }`)
				}
			}})
			found := false
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					if result.Problem != nil && strings.Contains(result.Problem.Message, "source snapshot mismatch") {
						found = true
					}
				}
			}
			if !changed || !found {
				t.Fatalf("excluded source generation change went undetected: changed=%v run=%+v", changed, run)
			}
		})
	}
}
