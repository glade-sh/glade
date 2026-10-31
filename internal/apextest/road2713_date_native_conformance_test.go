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

func TestRoad2713DateRouteNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "road2713_async_date")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected four native rows, got %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(b), "62.0", row[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			var payload string
			if row[1] == "anonymous" {
				src, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				p, err := vm.CompileAnonymousWithOptions(string(src), vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(p)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, ok := strings.Cut(line, "DATE_BOUNDARY|"); ok {
						payload = value
					}
				}
			} else if row[1] == "istest" {
				run := Run(index, Options{SelectedClasses: []string{"DateBoundaryProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one test result: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing assertion observation: %+v", result.Problem)
				}
				_, payload, _ = strings.Cut(result.Problem.Message, "DATE_BOUNDARY|")
			} else {
				t.Fatalf("unexpected route %q", row[1])
			}
			var got, want []string
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("payload %q: %v", payload, err)
			}
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native %v; Glade %v", want, got)
			}
		})
	}
}
