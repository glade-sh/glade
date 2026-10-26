package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestLiteralSplitNativeStringAndPatternAnonymousAndIsTestRoutes(t *testing.T) {
	fixture := filepath.Join("testdata", "ipv4_validation", "literal_splits")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("native row count=%d want 4 plus header", len(rows)-1)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("invalid native row")
		}
		api, context := row[0], row[1]
		label := api + "/" + context
		if seen[label] {
			t.Fatal("duplicate native cell")
		}
		seen[label] = true
		t.Run(label, func(t *testing.T) {
			var want map[string][]string
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != 96 {
				t.Fatalf("native observation inventory=%d", len(want))
			}
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				if !strings.HasSuffix(path, ".cls-meta.xml") && entry.Name() != "sfdx-project.json" {
					return nil
				}
				text, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(text), "67.0", api)), 0600)
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("index=%v sema=%v", index.Diagnostics, analysis.Diagnostics)
			}
			var observed string
			if context == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(`String observation=JSON.serialize(DelimiterBoundaryProbe.observe());`, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				observed = machine.Globals["observation"].Text
			} else if context == "isTest" {
				result := Run(index, Options{SelectedClasses: []string{"DelimiterBoundaryProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				summary := result.Summary()
				if summary.Total != 1 || summary.Failed != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing native observation transport: %+v %s", summary, firstRunProblem(result))
				}
				message := result.Suites[0].Cases[0].Problem.Message
				var found bool
				_, observed, found = strings.Cut(message, ", Actual: ")
				if !found || !strings.Contains(message, "DELIMITER_ROWS") {
					t.Fatal("unexpected observation transport: ", message)
				}
			} else {
				t.Fatal("unexpected native context")
			}
			var got map[string][]string
			if err := json.Unmarshal([]byte(observed), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native split rows differ:\ngot %s\nwant %s", observed, row[3])
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "isTest"} {
			if !seen[api+"/"+context] {
				t.Fatal("missing native cell")
			}
		}
	}
}
