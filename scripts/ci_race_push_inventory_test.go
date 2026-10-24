package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCIRacePushConcurrentInventoryRoutes(t *testing.T) {
	dir := t.TempDir()
	source := `package fixture
import "testing"
func TestDirectGoroutine(t *testing.T) { go func() {}() }
func TestDirectParallel(t *testing.T) { t.Parallel() }
func TestHelperGoroutine(t *testing.T) { startWorker() }
func TestHelperParallel(t *testing.T) { runParallel(t) }
func TestTransitiveHelper(t *testing.T) { runHelper(t) }
func TestSerial(t *testing.T) {}
`
	helper := `package fixture
import "testing"
func startWorker() { go func() {}() }
func runParallel(t *testing.T) { t.Parallel() }
func runHelper(t *testing.T) { runParallel(t) }
`
	for name, body := range map[string]string{"routes_test.go": source, "helpers_test.go": helper} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"TestDirectGoroutine", "TestDirectParallel", "TestHelperGoroutine", "TestHelperParallel", "TestTransitiveHelper"}
	if got := concurrentRaceTests(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("concurrent routes = %v, want %v", got, want)
	}
}

func TestCIRacePushRunnerRetainsConcurrentRoutes(t *testing.T) {
	// Exercise the real manifest, planner, runner and result validator while a
	// fake Go command records which concurrent tests the push lane would run.
	const program = `
import importlib.util, json, pathlib, sys, tempfile
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("race_push", "ci-race-push.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
config = m.scope()
package = sys.argv[1]
names = json.loads(sys.argv[2])
selected = config["packages"][package]
with tempfile.TemporaryDirectory() as root:
    m.ROOT = pathlib.Path(root)
    fake = m.ROOT / "go"
    fake.write_text('''#!/usr/bin/env python3
import json, re, sys
names = ''' + repr(names) + '''
package = ''' + repr("github.com/glade-sh/glade/" + package.removeprefix("./")) + '''
if "-list" in sys.argv:
    print("\\n".join(names) + "\\nok  " + package + " 0.01s")
else:
    pattern = sys.argv[sys.argv.index("-run") + 1]
    for name in names:
        if re.search(pattern, name):
            print(json.dumps({"Action": "pass", "Package": package, "Test": name}))
    print(json.dumps({"Action": "pass", "Package": package}))
''')
    fake.chmod(0o700)
    m.GO = str(fake)
    m.run({"packages": {package: selected}, "estimatedSeconds": {package: 1}}, 0)
    summary = json.loads(next((m.ROOT / "ci-artifacts/race-push").glob("*/validation-summary.json")).read_text())
    missing = sorted(set(names) - set(summary["passed"]))
    assert not missing, f"push runner omitted concurrent tests: {missing}"
`
	for _, pkg := range []string{"./internal/apextest", "./internal/gladecli"} {
		t.Run(pkg, func(t *testing.T) {
			names, err := json.Marshal(concurrentRaceTests(t, filepath.Join("..", pkg)))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "-c", program, pkg, string(names))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("push runner: %v\n%s", err, out)
			}
		})
	}
}
