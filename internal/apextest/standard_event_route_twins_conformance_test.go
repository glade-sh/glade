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

// Replay the exact owned sources, including the project runner's @IsTest route.
func TestStandardEventRouteTwinsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/standard_event_route_twins"
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
	if len(rows) != 7 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatal("invalid native row")
		}
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] || (api != "62.0" && api != "67.0") || (route != "anonymous" && route != "namedAnonymous" && route != "isTest") {
			t.Fatalf("invalid native cell %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(dir, "project"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(filepath.Join(dir, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(raw), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			observed := ""
			if route == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{"StandardEventRouteProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one observation: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing native assertion transport: %+v", result)
				}
				_, observed, _ = strings.Cut(result.Problem.Message, "OBS|")
			} else {
				name := "anonymous.apex"
				if route == "namedAnonymous" {
					name = "named_anonymous.apex"
				}
				raw, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				analysis := sema.AnalyzeAnonymous(index, string(raw), api)
				program, err := vm.CompileAnonymousWithOptions(string(raw), vm.CompileOptions{APIVersion: api})
				if analysis.HasErrors() || err != nil {
					t.Fatalf("native accepted anonymous; Glade: %v %v", analysis.Diagnostics, err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				result, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, "OBS|") {
						if observed != "" {
							t.Fatal("duplicate observation")
						}
						observed = strings.TrimPrefix(line, "OBS|")
					}
				}
			}
			var got, want map[string]string
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil || len(want) != 11 || len(got) != len(want) {
				t.Fatalf("invalid observations: %s", observed)
			}
			for id, expected := range want {
				t.Run(id, func(t *testing.T) {
					if got[id] != expected {
						t.Fatalf("native <%s>; Glade <%s>", expected, got[id])
					}
				})
			}
		})
	}
}
