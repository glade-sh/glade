package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestIterableInheritanceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "iterable_inheritance", "TraversalShapesTest")
}

// Replay the retained source against each independently captured API and route.
func runInnerTypeNativeConformance(t *testing.T, family, class string, apiVersions ...string) {
	runInnerTypeNativeConformanceWithComparison(t, family, class, func(t *testing.T, got, want any) {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
	}, apiVersions...)
}

func runInnerTypeNativeConformanceWithComparison(t *testing.T, family, class string, compare func(*testing.T, any, any), apiVersions ...string) {
	runInnerTypeNativeRoutes(t, family, class, []string{"anonymous", "isTest"}, compare, apiVersions...)
}

func runInnerTypeNativeRoutes(t *testing.T, family, class string, routes []string, compare func(*testing.T, any, any), apiVersions ...string) {
	t.Helper()
	if compare == nil {
		compare = func(t *testing.T, got, want any) {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		}
	}
	if len(apiVersions) == 0 {
		apiVersions = []string{"62.0", "67.0", "68.0"}
	}
	dir := filepath.Join("testdata", "conformance", family)
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+2*len(apiVersions) || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] || !slices.Contains(apiVersions, api) || (route != "anonymous" && route != "isTest") {
			t.Fatalf("invalid native cell %s", key)
		}
		seen[key] = true
		if !slices.Contains(routes, route) {
			continue
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
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
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			observed := ""
			if route == "anonymous" {
				source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				analysis := sema.AnalyzeAnonymous(index, string(source), api)
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if analysis.HasErrors() || err != nil {
					t.Fatalf("native anonymous compiled; diagnostics=%v compile=%v", analysis.Diagnostics, err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				triggers, failures := compileProjectTriggers(index)
				if len(failures) != 0 {
					t.Fatal(failures)
				}
				for _, trigger := range triggers {
					if err := runner.base.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
				}
				result, err := runner.execute(program, func(machine *vm.VM) {
					for _, page := range index.VisualforcePageNames {
						machine.RegisterPageReference(page)
					}
				})
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if at := strings.Index(line, "OBS|"); at >= 0 {
						if observed != "" {
							t.Fatal("duplicate observation")
						}
						observed = line[at+len("OBS|"):]
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one observation: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing assertion transport: %+v; problem: %+v", result, result.Problem)
				}
				_, observed, _ = strings.Cut(result.Problem.Message, "OBS|")
			}
			var got, want any
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid observation: %q", observed)
			}
			compare(t, got, want)
		})
	}
}
