package scripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A synthetic repository for selection tests: a root Go module (example/*)
// plus the nested Apex parser module under third_party/glade-apex-parser. The
// chains mirror the real ones: b is a direct parser importer, d imports b in
// production while e and f import d in internal and external tests, sema
// imports b, vm imports version and the parser, and runner imports vm — the
// internal/apexversion -> internal/vm -> internal/apextest chain. cli imports
// sema the way internal/gladecli imports internal/sema, and testutil is a
// helper package like internal/testutil whose only importers are e's and f's
// tests.
const (
	selectionGraph = `[
{"ImportPath":"example/a","Dir":"/repo/a"},
{"ImportPath":"example/b","Dir":"/repo/b","Imports":["example/a","github.com/glade-sh/apex-parser"]},
{"ImportPath":"example/c","Dir":"/repo/c","TestImports":["example/a"]},
{"ImportPath":"example/d","Dir":"/repo/d","Imports":["example/b"]},
{"ImportPath":"example/e","Dir":"/repo/e","TestImports":["example/d","example/testutil"]},
{"ImportPath":"example/f","Dir":"/repo/f","XTestImports":["example/d","example/testutil"]},
{"ImportPath":"example/sema","Dir":"/repo/sema","Imports":["example/b"]},
{"ImportPath":"example/cli","Dir":"/repo/cli","Imports":["example/sema"]},
{"ImportPath":"example/testutil","Dir":"/repo/testutil"},
{"ImportPath":"example/version","Dir":"/repo/version"},
{"ImportPath":"example/vm","Dir":"/repo/vm","Imports":["example/version","github.com/glade-sh/apex-parser"]},
{"ImportPath":"example/runner","Dir":"/repo/runner","Imports":["example/vm"]},
{"ImportPath":"example/newpkg","Dir":"/repo/newpkg"},
{"ImportPath":"example/renamednew","Dir":"/repo/renamednew"},
{"ImportPath":"github.com/glade-sh/glade/internal/playground","Dir":"/repo/internal/playground"},
{"ImportPath":"github.com/glade-sh/glade/internal/gladecli","Dir":"/repo/internal/gladecli"}
]`
	selectionParserGraph = `[
{"ImportPath":"github.com/glade-sh/apex-parser","Dir":"/repo/third_party/glade-apex-parser","Imports":["github.com/glade-sh/apex-parser/internal/tsapex"]},
{"ImportPath":"github.com/glade-sh/apex-parser/internal/tsapex","Dir":"/repo/third_party/glade-apex-parser/internal/tsapex"}
]`
	selectionManifest = `{"version":1,"lanes":{"remaining-go":[
"example/a","example/b","example/c","example/cli","example/d","example/e",
"example/f","example/newpkg","example/renamednew","example/runner",
"example/sema","example/testutil","example/version","example/vm",
"github.com/glade-sh/glade/internal/playground","github.com/glade-sh/glade/internal/gladecli"
]}}`
)

var (
	selectionAll = []string{
		"example/a", "example/b", "example/c", "example/cli", "example/d", "example/e",
		"example/f", "example/newpkg", "example/renamednew", "example/runner",
		"example/sema", "example/testutil", "example/version", "example/vm",
		"github.com/glade-sh/glade/internal/gladecli", "github.com/glade-sh/glade/internal/playground",
	}
	// A changed a reaches b, c and then, transitively, d, e, f, sema and
	// sema's importer cli.
	selectionFromA = []string{"example/a", "example/b", "example/c", "example/cli", "example/d", "example/e", "example/f", "example/sema"}
	// Any parser path reaches every root package that imports the parser (b
	// and vm) and closes over the reverse import graph from there.
	selectionFromParser = []string{"example/b", "example/cli", "example/d", "example/e", "example/f", "example/runner", "example/sema", "example/vm"}
)

// runPRSelect runs scripts/ci-pr-select.py over fixture graph inputs and
// returns the selected package list, or the command error.
func runPRSelect(t *testing.T, files map[string]string, paths string) ([]string, error) {
	t.Helper()
	root := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("python3", "ci-pr-select.py", "--root", "/repo",
		"--graph", filepath.Join(root, "graph.json"),
		"--parser-graph", filepath.Join(root, "parser-graph.json"),
		"--manifest", filepath.Join(root, "manifest.json"))
	cmd.Stdin = strings.NewReader(paths)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("selector: %w\n%s", err, stderr.String())
	}
	var got []string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid output: %v: %s", err, stdout.String())
	}
	sort.Strings(got)
	return got, nil
}

func TestCIPRPackageSelection(t *testing.T) {
	files := map[string]string{
		"graph.json":        selectionGraph,
		"parser-graph.json": selectionParserGraph,
		"manifest.json":     selectionManifest,
	}
	cases := []struct {
		name, paths string
		want        []string
	}{
		{"direct and transitive importers", "a/a.go\n", selectionFromA},
		{"second order edges", "b/b.go\n", []string{"example/b", "example/cli", "example/d", "example/e", "example/f", "example/sema"}},
		// A *_test.go file in a package directory compiles only into that
		// package's own test binary, so the change selects the package
		// alone: importers, production or test, cannot see another
		// package's test files.
		{"sema test file selects only sema", "sema/parallel_unknown_test.go\n", []string{"example/sema"}},
		{"sema production change keeps its importers", "sema/sema.go\n", []string{"example/cli", "example/sema"}},
		{"sema test and production change is transitive", "sema/sema.go\nsema/parallel_unknown_test.go\n", []string{"example/cli", "example/sema"}},
		{"external test file selects only its package", "d/export_test.go\n", []string{"example/d"}},
		// A helper package imported only by tests keeps the reverse walk
		// over test edges for its production files; only its own test
		// files narrow to the helper itself.
		{"test helper production file reaches its test importers", "testutil/helper.go\n", []string{"example/e", "example/f", "example/testutil"}},
		{"test helper test file selects only the helper", "testutil/helper_test.go\n", []string{"example/testutil"}},
		// Testdata below a package keeps the transitive package mapping,
		// whatever the file is named: other packages' tests may read it.
		{"sema testdata keeps the transitive selection", "sema/testdata/case.tsv\n", []string{"example/cli", "example/sema"}},
		{"sema testdata test file keeps the transitive selection", "sema/testdata/captured_test.go\n", []string{"example/cli", "example/sema"}},
		// No root lane executes the nested parser module's packages, so a
		// parser test file selects nothing; the pr-guards worker runs the
		// parser module's own tests whenever any parser path changes.
		{"parser test file selects no root package", "third_party/glade-apex-parser/parser_test.go\n", []string{}},
		{"version chain reaches the runner", "version/version.go\n", []string{"example/runner", "example/version", "example/vm"}},
		{"parser file reaches semantic and runtime consumers", "third_party/glade-apex-parser/parser.go\n", selectionFromParser},
		{"parser internal file reaches consumers", "third_party/glade-apex-parser/internal/tsapex/binding.go\n", selectionFromParser},
		{"parser non package file", "third_party/glade-apex-parser/README.md\n", selectionFromParser},
		{"non go fixture", "a/testdata/input.cls\n", selectionFromA},
		{"native fixture in a package", "runner/testdata/conformance/x/org.tsv\n", []string{"example/runner"}},
		{"deleted file in a surviving package", "vm/vm.go\n", []string{"example/runner", "example/vm"}},
		{"new package", "newpkg/newpkg.go\n", []string{"example/newpkg"}},
		{"removed package", "gone/gone.go\n", selectionAll},
		{"renamed package", "oldname/helper.go\nrenamednew/helper.go\n", selectionAll},
		{"unrelated docs", "docs/readme.md\n", []string{}},
		{"docs tree non markdown", "docs/img/diagram.png\n", []string{}},
		{"root markdown", "README.md\n", []string{}},
		{"dependency manifest", "go.mod\n", selectionAll},
		{"dependency checksums", "go.sum\n", selectionAll},
		{"lwcruntime input", "lwcruntime/src/shell/workspace-service.mjs\n", selectionAll},
		{"third party lwc", "third_party/lwc/package.json\n", selectionAll},
		{"root testdata", "testdata/conformance/x/org.tsv\n", selectionAll},
		{"root testdata markdown", "testdata/conformance/x/native-run.md\n", selectionAll},
		{"site", "site/src/index.html\n", []string{}},
		{"hosting config", ".do/app.yaml\n", []string{"github.com/glade-sh/glade/internal/playground"}},
		{"attributes", ".gitattributes\n", []string{}},
		{"issue template", ".github/ISSUE_TEMPLATE/bug.yml\n", []string{}},
		{"editor extension", "contrib/vscode-glade/src/main.ts\n", []string{"github.com/glade-sh/glade/internal/gladecli"}},
		{"similar site prefix", "site-other/a.js\n", selectionAll},
		{"similar config prefix", ".do-other/app.yaml\n", selectionAll},
		{"site and go", "site/src/index.html\na/a.go\n", selectionFromA},
		{"root file", "glade.yml\n", selectionAll},
		{"workflow", ".github/workflows/ci.yml\n", selectionAll},
		{"docs plus unmapped", "docs/a.md\nlwcruntime/x.mjs\n", selectionAll},
		{"multiple", "a/a.go\nd/d.go\n", selectionFromA},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runPRSelect(t, files, tc.paths)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// A planning error must fail the selection instead of producing an empty
// successful one: a malformed graph or manifest can never narrow what runs.
func TestCIPRSelectionRejectsMalformedInputs(t *testing.T) {
	cases := []struct{ name, graph, parser, manifest string }{
		{"graph is not a list", "{}", selectionParserGraph, selectionManifest},
		{"graph is invalid json", "[{\"ImportPath\":", selectionParserGraph, selectionManifest},
		{"graph entry is not an object", "[\"example/a\"]", selectionParserGraph, selectionManifest},
		{"graph entry has no import path", "[{\"Dir\":\"/repo/a\"}]", selectionParserGraph, selectionManifest},
		{"graph entry has no dir", "[{\"ImportPath\":\"example/a\"}]", selectionParserGraph, selectionManifest},
		{"graph lists a package twice", "[{\"ImportPath\":\"example/a\",\"Dir\":\"/repo/a\"},{\"ImportPath\":\"example/a\",\"Dir\":\"/repo/a2\"}]", selectionParserGraph, selectionManifest},
		{"graph import list is malformed", "[{\"ImportPath\":\"example/a\",\"Dir\":\"/repo/a\",\"Imports\":\"example/b\"}]", selectionParserGraph, selectionManifest},
		{"graph is empty", "[]", selectionParserGraph, selectionManifest},
		{"graph does not match the manifest", selectionGraph, selectionParserGraph, `{"version":1,"lanes":{"remaining-go":["example/a"]}}`},
		{"manifest is not an object", selectionGraph, selectionParserGraph, "[]"},
		{"manifest has no lanes", selectionGraph, selectionParserGraph, `{"version":1}`},
		{"manifest lane is malformed", selectionGraph, selectionParserGraph, `{"version":1,"lanes":{"remaining-go":"example/a"}}`},
		{"manifest lists an unknown package", selectionGraph, selectionParserGraph, `{"version":1,"lanes":{"remaining-go":["example/ghost"]}}`},
		{"parser graph is empty", selectionGraph, "[]", selectionManifest},
		{"parser graph is not a list", "{}", selectionParserGraph, selectionManifest},
		{"parser graph package outside the module", selectionGraph, `[{"ImportPath":"example/a","Dir":"/repo/third_party/glade-apex-parser/x"}]`, selectionManifest},
		{"parser graph package outside the module directory", selectionGraph, `[{"ImportPath":"github.com/glade-sh/apex-parser","Dir":"/repo/other"}]`, selectionManifest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"graph.json": tc.graph, "parser-graph.json": tc.parser, "manifest.json": tc.manifest}
			if _, err := runPRSelect(t, files, "a/a.go\n"); err == nil {
				t.Fatal("selector accepted a malformed input")
			}
		})
	}
}

func TestCIPRWorkflowSelection(t *testing.T) {
	t.Setenv("SMOKE_REQUIRED", "false")
	t.Setenv("PR_BROWSER", "false")
	t.Setenv("PR_PLAN", `{"include":[{"shard":-1,"packages":[],"pool":[],"browser":false}]}`)
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			If    string    `yaml:"if"`
			Needs yaml.Node `yaml:"needs"`
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test", "apextest", "vm", "gladecli", "sema", "nested-sources", "smoke-runtime", "smoke-distribution"} {
		if !strings.Contains(workflow.Jobs[name].If, "github.event_name != 'pull_request'") {
			t.Errorf("%s does not exclude PRs", name)
		}
	}
	// test-binaries and test-binaries-apextest also run on PRs: pr-tests reuses
	// their binaries (TestCIPRLaneUsesSharedTestBinaries).
	if !strings.Contains(workflow.Jobs["pr-tests"].If, "github.event_name == 'pull_request'") {
		t.Fatal("PR tests do not select PRs")
	}
	pr := string(data)
	if !strings.Contains(pr, "TestRunDoctorReportsParser|TestRunDoctorJSON|TestRunDoctorShortFlags|TestRunDoctorReportsProjectLocalDataEnvironment") {
		t.Error("PR CLI route must run the tests delegated by the CLI shards")
	}
	job := workflow.Jobs["required-ci"]
	results := make(map[string]map[string]string)
	var needs []string
	if err := job.Needs.Decode(&needs); err != nil {
		t.Fatal(err)
	}
	for _, name := range needs {
		results[name] = map[string]string{"result": "success"}
	}
	run := func(event string, want bool) {
		t.Helper()
		encoded, _ := json.Marshal(results)
		cmd := exec.Command("bash", "-c", job.Steps[0].Run)
		cmd.Env = append(os.Environ(), "REQUIRED_RESULTS="+string(encoded), "EVENT_NAME="+event)
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatalf("%s results %s: %v\n%s", event, encoded, err, out)
		}
	}
	results["site"]["result"] = "skipped"
	run("pull_request", false)
	run("merge_group", false)
	results["site"]["result"] = "success"
	results["pr-tests"]["result"] = "skipped"
	for _, event := range []string{"merge_group", "push", "schedule", "workflow_dispatch"} {
		run(event, true)
	}
	run("pull_request", false)
	// The repository guards moved to their own PR worker: a missing or
	// skipped guard worker must fail the pull request gate, and the full
	// routes keep excluding it because their test job owns the guards.
	if _, ok := results["pr-guards"]; !ok {
		t.Fatal("required-ci does not need pr-guards; the aggregate must expect the split guard worker")
	}
	results["pr-guards"]["result"] = "skipped"
	run("pull_request", false)
	for _, event := range []string{"merge_group", "push", "schedule", "workflow_dispatch"} {
		run(event, true)
	}
	results["pr-guards"]["result"] = "success"
	results["pr-tests"]["result"] = "success"
	results["early-guards"]["result"] = "skipped"
	run("pull_request", false)
	run("push", true)
	results["early-guards"]["result"] = "success"
	run("pull_request", true)
	results["pr-select"]["result"] = "failure"
	run("pull_request", false)
	results["pr-select"]["result"] = "success"
	t.Setenv("SMOKE_REQUIRED", "true")
	results["smoke-runtime"]["result"] = "skipped"
	run("pull_request", false)
	results["smoke-runtime"]["result"] = "success"
	results["smoke-distribution"]["result"] = "skipped"
	run("pull_request", false)
	results["smoke-distribution"]["result"] = "success"
	run("pull_request", true)
	t.Setenv("SMOKE_REQUIRED", "invalid")
	run("pull_request", false)
	t.Setenv("SMOKE_REQUIRED", "false")
	t.Setenv("PR_BROWSER", "true")
	results["pr-browser-evidence"]["result"] = "skipped"
	run("pull_request", false)
	delete(results, "pr-browser-evidence")
	run("pull_request", false)
	results["pr-browser-evidence"] = map[string]string{"result": "success"}
	run("pull_request", true)
	t.Setenv("PR_BROWSER", "false")
	t.Setenv("HEAD_REF", "train/example")
	results["early-guards"]["result"] = "skipped"
	run("pull_request", false)
	results["early-guards"]["result"] = "success"
	results["pr-browser-full"]["result"] = "skipped"
	run("pull_request", false)
	delete(results, "pr-browser-full")
	run("pull_request", false)
	results["pr-browser-full"] = map[string]string{"result": "success"}
	run("pull_request", true)
	t.Setenv("HEAD_REF", "")
	results["vet"]["result"] = "failure"
	run("pull_request", false)
	run("merge_group", false)
}

// The PR lane runs each large-package shard on its own worker with the full
// suite's 12-way Apex split; a 2-way default would time out and reject index 2+.
func TestCIPRWorkflowShardRoutes(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "matrix: ${{ fromJSON(needs.pr-select.outputs.matrix) }}") {
		t.Fatal("PR workers must consume the select job's matrix")
	}
	if !strings.Contains(string(data), "export CI_APEXTEST_SHARDS=12") {
		t.Fatal("PR lane must select the 12-way Apex shard route")
	}
}
