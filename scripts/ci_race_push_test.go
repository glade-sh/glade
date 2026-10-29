package scripts

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Follow local helpers too: concurrency is often started by a fixture helper.
func concurrentRaceTests(t *testing.T, dir string) []string {
	t.Helper()
	functions := map[string]*ast.FuncDecl{}
	concurrent := map[string]bool{}
	calls := map[string][]string{}
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			functions[name] = fn
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.GoStmt:
					concurrent[name] = true
				case *ast.CallExpr:
					// The Apex project runner starts workers in production code;
					// its tests need race coverage even without a Go statement.
					if f, ok := x.Fun.(*ast.Ident); ok && f.Name == "Run" {
						for _, arg := range x.Args {
							literal, ok := arg.(*ast.CompositeLit)
							if !ok {
								continue
							}
							typ, ok := literal.Type.(*ast.Ident)
							if !ok || typ.Name != "Options" {
								continue
							}
							for _, elt := range literal.Elts {
								kv, ok := elt.(*ast.KeyValueExpr)
								if !ok {
									continue
								}
								key, keyOK := kv.Key.(*ast.Ident)
								value, valueOK := kv.Value.(*ast.Ident)
								if keyOK && valueOK && key.Name == "ParallelMethods" && value.Name == "true" {
									concurrent[name] = true
								}
							}
						}
					}
					switch f := x.Fun.(type) {
					case *ast.Ident:
						calls[name] = append(calls[name], f.Name)
					case *ast.SelectorExpr:
						if f.Sel.Name == "Parallel" {
							concurrent[name] = true
						}
					}
				}
				return true
			})
		}
	}
	for changed := true; changed; {
		changed = false
		for name := range functions {
			for _, called := range calls[name] {
				if concurrent[called] && !concurrent[name] {
					concurrent[name] = true
					changed = true
				}
			}
		}
	}
	var tests []string
	for name := range functions {
		if strings.HasPrefix(name, "Test") && concurrent[name] {
			tests = append(tests, name)
		}
	}
	sort.Strings(tests)
	return tests
}

func TestCIRacePushRetainsConcurrentTests(t *testing.T) {
	data, err := os.ReadFile("ci-race-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Packages) == 0 {
		t.Fatal("empty push race scope")
	}
	for _, pkg := range []string{"./internal/apextest", "./internal/gladecli", "./internal/lwc/compile", "./internal/visualforce"} {
		want := concurrentRaceTests(t, filepath.Join("..", pkg))
		if pkg == "./internal/lwc/compile" {
			want = []string{"TestBrowserAPIParallelism", "TestRunBrowserPassesKeepsPassOrder"}
		}
		got, ok := cfg.Packages[pkg]
		if !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("%s concurrent tests = %v, want %v", pkg, got, want)
		}
	}
}

func TestCIRacePushPlanUnionAndWorkflow(t *testing.T) {
	out, err := exec.Command("python3", "ci-race-push.py", "plan").CombinedOutput()
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	var plan struct {
		Include []struct {
			Packages         []string `json:"packages"`
			EstimatedSeconds int      `json:"estimatedSeconds"`
			Runs             []struct {
				Package string
				Tests   []string
			} `json:"runs"`
		} `json:"include"`
	}
	if err := json.Unmarshal(out, &plan); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("ci-race-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	selections := map[string]map[string]bool{}
	for _, batch := range plan.Include {
		if batch.EstimatedSeconds > 900 {
			t.Errorf("batch estimate %d exceeds 15 minutes", batch.EstimatedSeconds)
		}
		for _, pkg := range batch.Packages {
			seen[pkg] = true
		}
		for _, run := range batch.Runs {
			if selections[run.Package] == nil {
				selections[run.Package] = map[string]bool{}
			}
			for _, name := range run.Tests {
				if selections[run.Package][name] {
					t.Fatalf("duplicated selection %s %s", run.Package, name)
				}
				selections[run.Package][name] = true
			}
		}
	}
	for pkg, names := range cfg.Packages {
		for _, name := range names {
			found := false
			for selected := range selections[pkg] {
				if selected == name || strings.HasPrefix(selected, name+"/") {
					found = true
				}
			}
			if !found {
				t.Errorf("missing selected test %s %s", pkg, name)
			}
		}
	}
	if len(seen) != len(cfg.Packages) || len(plan.Include) > 256 {
		t.Fatalf("invalid plan %s", out)
	}
	for pkg := range cfg.Packages {
		if !seen[pkg] {
			t.Fatalf("missing %s", pkg)
		}
	}
	workflow, err := os.ReadFile("../.github/workflows/race.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"mode=concurrency", "ci-race-push.py run", "race-push:", "schedule|workflow_dispatch)", "scripts/ci-race-packages.sh full"} {
		if !strings.Contains(string(workflow), marker) {
			t.Errorf("workflow missing %q", marker)
		}
	}
}

func TestCIRacePushRunnerRejectsMissingSkippedAndExtraResults(t *testing.T) {
	program := `
import importlib.util, json, os, pathlib, tempfile, sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location("race_push", "ci-race-push.py")
m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
with tempfile.TemporaryDirectory() as root:
 m.ROOT=pathlib.Path(root)
 fake=m.ROOT/"go"
 fake.write_text('''#!/usr/bin/env python3
import json, os, sys
p=os.getenv("FIXTURE_PACKAGE", "github.com/glade-sh/glade/internal/example")
name=os.getenv("FIXTURE_NAME", "TestConcurrent")
if "-list" in sys.argv:
 print(name+"\\nTestOther\\nok  "+p+" 0.01s");sys.exit(0)
mode=os.environ.get("FIXTURE_MODE", "pass")
print(json.dumps({"Action":"start", "Package":p}))
if mode!="missing": print(json.dumps({"Action":"run", "Package":p,"Test":name}))
if mode=="deferred": print(json.dumps({"Action":"output", "Package":p, "Test":name, "Output":"    dataweave_legacy_test.go:23: requires explicitly installed DataWeave toolchain\\n"}))
if mode!="missing": print(json.dumps({"Action": "skip" if mode in ("skip", "deferred") else "pass", "Package":p, "Test":name}))
if mode=="extra":
 print(json.dumps({"Action":"run", "Package":p,"Test":"TestOther"}))
 print(json.dumps({"Action":"pass", "Package":p, "Test":"TestOther"}))
print(json.dumps({"Action":"pass", "Package":p}))
''')
 fake.chmod(0o700);m.GO=str(fake)
 cfg={"packages":{"./internal/example":["TestConcurrent"]},"estimatedSeconds":{"./internal/example":1}}
 m.run(cfg,0)
 for mode in ("missing","skip","extra"):
  os.environ["FIXTURE_MODE"]=mode
  try: m.run(cfg,0)
  except ValueError: pass
  else: raise AssertionError("accepted "+mode)
  assert not (m.ROOT/"ci-artifacts/race-push/internal-example/validation-summary.json").exists()
 os.environ.update(FIXTURE_MODE="deferred", FIXTURE_PACKAGE="github.com/glade-sh/glade/internal/vm", FIXTURE_NAME="TestExecDataWeaveScriptResultCarriers")
 os.environ.pop("GLADE_DATAWEAVE_APEX_TEST_HOME", None)
 m.run({"packages":{"./internal/vm":["TestExecDataWeaveScriptResultCarriers"]},"estimatedSeconds":{"./internal/vm":1}},0)
 summary=json.loads((m.ROOT/"ci-artifacts/race-push/internal-vm/validation-summary.json").read_text())
 assert summary["deferred"]==["TestExecDataWeaveScriptResultCarriers"] and summary["passed"]==[]
`
	if out, err := exec.Command("python3", "-c", program).CombinedOutput(); err != nil {
		t.Fatalf("runner fixture: %v\n%s", err, out)
	}
}

func TestCIRacePushProductionConcurrencyScope(t *testing.T) {
	data, err := os.ReadFile("ci-race-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var full struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(data, &full); err != nil {
		t.Fatal(err)
	}
	for _, packages := range full.Lanes {
		for _, imp := range packages {
			pkg := "." + strings.TrimPrefix(imp, "github.com/glade-sh/glade")
			files, err := filepath.Glob(filepath.Join("..", pkg, "*.go"))
			if err != nil {
				t.Fatal(err)
			}
			concurrent := false
			for _, path := range files {
				if strings.HasSuffix(path, "_test.go") {
					continue
				}
				f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				for _, i := range f.Imports {
					if i.Path.Value == `"sync"` || i.Path.Value == `"sync/atomic"` {
						concurrent = true
					}
				}
				ast.Inspect(f, func(n ast.Node) bool {
					if _, ok := n.(*ast.GoStmt); ok {
						concurrent = true
					}
					return true
				})
			}
			if concurrent {
				if _, ok := cfg.Packages[pkg]; !ok {
					t.Errorf("production concurrency package absent from push race: %s", pkg)
				}
			}
		}
	}
}
