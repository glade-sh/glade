package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCIPRPackageSelection(t *testing.T) {
	root := t.TempDir()
	graph := `[{"ImportPath":"example/a","Dir":"/repo/a"},{"ImportPath":"example/b","Dir":"/repo/b","Imports":["example/a","github.com/glade-sh/apex-parser/parser"]},{"ImportPath":"example/c","Dir":"/repo/c","TestImports":["example/a"]},{"ImportPath":"example/d","Dir":"/repo/d","Imports":["example/b"]}]`
	manifest := `{"version":1,"lanes":{"remaining-go":["example/a","example/b","example/c","example/d"]}}`
	for name, data := range map[string]string{"graph.json": graph, "manifest.json": manifest} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, paths string
		want        []string
	}{
		{"direct and test importers", "a/a.go\n", []string{"example/a", "example/b", "example/c"}},
		{"non go fixture", "a/testdata/input.cls\n", []string{"example/a", "example/b", "example/c"}},
		{"unrelated docs", "docs/readme.md\n", []string{}},
		{"multiple", "a/a.go\nd/d.go\n", []string{"example/a", "example/b", "example/c", "example/d"}},
		{"nested parser", "third_party/glade-apex-parser/parser/file.go\n", []string{"example/b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("python3", "ci-pr-select.py", "--root", "/repo", "--graph", filepath.Join(root, "graph.json"), "--manifest", filepath.Join(root, "manifest.json"))
			cmd.Stdin = strings.NewReader(tc.paths)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("selector: %v\n%s", err, out)
			}
			var got []string
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("invalid output: %v: %s", err, out)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCIPRWorkflowSelection(t *testing.T) {
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
	for _, name := range []string{"site", "test", "test-binaries", "apextest", "vm", "gladecli", "sema", "nested-sources", "smoke-runtime", "smoke-distribution"} {
		if !strings.Contains(workflow.Jobs[name].If, "github.event_name != 'pull_request'") {
			t.Errorf("%s does not exclude PRs", name)
		}
	}
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
	run("pull_request", true)
	run("merge_group", false)
	results["site"]["result"] = "success"
	results["pr-tests"]["result"] = "skipped"
	for _, event := range []string{"merge_group", "push", "schedule", "workflow_dispatch"} {
		run(event, true)
	}
	run("pull_request", false)
	results["pr-tests"]["result"] = "success"
	results["vet"]["result"] = "failure"
	run("pull_request", false)
	run("merge_group", false)
}
