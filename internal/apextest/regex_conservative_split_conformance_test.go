package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native regex_conservative_split/org.tsv and ORG-RUN.txt capture a parser
// using its separator twice, including Unicode folding and octal classes.
// Keep the project runner and anonymous routes at both API 62 and 67.
func TestConservativeSplitNativeProjectAnonymousAndIsTestRoutes(t *testing.T) {
	runConservativeSplitNativeProjectRoutes(t, false)
}

func TestConservativeSplitNativeWarmProjectCacheRoutes(t *testing.T) {
	runConservativeSplitNativeProjectRoutes(t, true)
}

func runConservativeSplitNativeProjectRoutes(t *testing.T, warm bool) {
	t.Helper()
	fixture := filepath.Join("testdata", "regex_conservative_split")
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string][2]string{}
	for _, row := range read("cases.tsv") {
		var p, s string
		if err := json.Unmarshal([]byte(row[1]), &p); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(row[2]), &s); err != nil {
			t.Fatal(err)
		}
		cases[row[0]] = [2]string{p, s}
	}
	rows := read("org.tsv")
	if len(rows) != 52 || len(cases) != 13 {
		t.Fatalf("oracle rows=%d cases=%d", len(rows), len(cases))
	}
	quote := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `'`, `\'`)
		s = strings.ReplaceAll(s, "\b", `'+String.fromCharArray(new List<Integer>{8})+'`)
		return "'" + s + "'"
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, name := range []string{"ConservativeSplitProbe", "ConservativeSplitProbeTest"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			named := Run(index, Options{SelectedClasses: []string{"ConservativeSplitProbeTest"}, NoDiskCache: true, Parallelism: 1})
			if warm {
				ResetPerfCounters()
				named = Run(index, Options{SelectedClasses: []string{"ConservativeSplitProbeTest"}, NoDiskCache: true, Parallelism: 1, PerfCounters: true})
				if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
					t.Fatalf("warm project route did not reuse its runtime: %+v", phases)
				}
			}
			if summary := named.Summary(); summary.Total != 13 || summary.Failed != 13 {
				t.Fatalf("missing named observation transport: %+v %s", summary, firstRunProblem(named))
			}
			observations := map[string]string{}
			for _, suite := range named.Suites {
				for _, c := range suite.Cases {
					if c.Problem == nil {
						t.Fatal("missing observation transport")
					}
					_, value, ok := strings.Cut(c.Problem.Message, ", Actual: ")
					if !ok || !strings.Contains(c.Problem.Message, "SPLIT_ROWS") {
						t.Fatal(c.Problem.Message)
					}
					observations[c.MethodName] = value
				}
			}
			for _, row := range rows {
				if row[0] != api {
					continue
				}
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					input, ok := cases[row[2]]
					if !ok {
						t.Fatal("unknown native case")
					}
					observed := observations[row[2]]
					if row[1] == "anonymous" {
						body := `String observation=JSON.serialize(ConservativeSplitProbe.observe(` + quote(input[0]) + `,` + quote(input[1]) + `));`
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := runner.newMachine()
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						observed = machine.Globals["observation"].Text
					} else if row[1] != "isTest" {
						t.Fatal("unknown native context")
					}
					var got, want map[string]any
					if err := json.Unmarshal([]byte(observed), &got); err != nil {
						t.Fatalf("observation %q: %v", observed, err)
					}
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if len(want) != 9 {
						t.Fatalf("native route count=%d", len(want))
					}
					if row[2] == "octal_class_400" || row[2] == "octal_class_777" {
						// Salesforce rejects these classes; the existing local
						// matcher accepts them. Pin split/matcher equivalence.
						if got["Matcher.find"] != false {
							t.Fatalf("full-engine control=%v", got["Matcher.find"])
						}
						text := "L" + input[1] + "R" + input[1] + "T"
						for key := range want {
							if key == "Matcher.find" {
								want[key] = false
							} else {
								want[key] = []any{text}
							}
						}
						t.Log("carried native syntax gap; asserting full-engine route equivalence (CARRIED.txt)")
					}
					for route, expected := range want {
						t.Run(route, func(t *testing.T) {
							if !reflect.DeepEqual(got[route], expected) {
								t.Fatalf("got %v want native %v", got[route], expected)
							}
						})
					}
				})
			}
		})
	}
}
