package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRedirectTargetNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/redirect_target"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 17 || strings.Join(rows[0], ",") != "source_api,route,case,observation" {
		t.Fatal("incomplete native matrix")
	}
	want := map[string]map[string]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		if want[key] == nil {
			want[key] = map[string]string{}
		}
		if _, duplicate := want[key][row[2]]; duplicate {
			t.Fatalf("duplicate native cell %s/%s", key, row[2])
		}
		want[key][row[2]] = row[3]
	}
	if len(want) != 4 {
		t.Fatal("expected four API/route pairs")
	}
	classes := map[string]string{}
	for _, name := range []string{"RedirectTargetProbe", "RedirectTargetProbeTest"} {
		classes[name] = read("project/force-app/main/default/classes/" + name + ".cls")
	}
	anonymous := read("anonymous.apex")
	for _, api := range []string{"62.0", "67.0"} {
		index := nativeBoundaryIndex(t, api, classes, nil)
		if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native declarations compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
		}
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				observed := ""
				if route == "anonymous" {
					program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
					analysis := sema.AnalyzeAnonymous(index, anonymous, api)
					if err != nil || analysis.HasErrors() {
						t.Fatalf("native anonymous compiled; Glade: %v %v", err, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					result, err := runner.execute(program)
					if err != nil || len(result.Debug) != 1 {
						t.Fatalf("missing observation: %v %v", err, result.Debug)
					}
					_, observed, _ = strings.Cut(result.Debug[0], "OBS|")
				} else {
					run := Run(index, Options{SelectedClasses: []string{"RedirectTargetProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected one observation: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing assertion transport: %+v", problem)
					}
					_, observed, _ = strings.Cut(problem.Message, "OBS|")
				}
				var got map[string]string
				if err := json.Unmarshal([]byte(observed), &got); err != nil || len(got) != 4 {
					t.Fatalf("invalid observation %q: %v", observed, err)
				}
				for _, id := range []string{"null", "relative", "absolute", "login"} {
					t.Run(id, func(t *testing.T) {
						expected, ok := want[api+"/"+route][id]
						if !ok || got[id] != expected {
							t.Fatalf("native=%q; Glade=%q", expected, got[id])
						}
					})
				}
			})
		}
	}
}
