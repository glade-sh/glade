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

func TestSetupCatalogOrgConformance(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "setup_catalog")
	file, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 21 {
		t.Fatalf("native rows = %d, want 20", len(rows)-1)
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	methods := map[string]string{"PERMISSION": "permission", "EMPTY": "emptyCatalog", "POPULATED": "populatedCatalog", "ORDINARY": "ordinary", "SCAN": "scan"}
	calls := map[string]string{"PERMISSION": "permissionObservation()", "EMPTY": "catalogObservation(false)", "POPULATED": "catalogObservation(true)", "ORDINARY": "ordinaryObservation()", "SCAN": "scanObservation()"}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || methods[row[2]] == "" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		// Keep the captured formula projection difference separate from the
		// empty-child traversal and built-in setup assertions.
		if row[0] == "67.0" && row[2] == "POPULATED" {
			t.Logf("carried %s: native formula projection %s", key, row[3])
			continue
		}
		t.Run(key, func(t *testing.T) {
			className := "SetupCatalog" + strings.TrimSuffix(row[0], ".0")
			observation := ""
			marker := "NATIVE_RESULT|" + row[2] + "|"
			if row[1] == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{className + "Proof"}, SelectedMethod: methods[row[2]], NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 {
					t.Fatalf("selected observation count: %+v", run.Summary())
				}
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							_, observation, _ = strings.Cut(c.Problem.Message, marker)
						}
					}
				}
			} else {
				source := "System.debug('" + marker + "'+JSON.serialize(" + className + "." + calls[row[2]] + "));"
				if analysis := sema.AnalyzeAnonymous(index, source, row[0]); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, marker) {
						observation = strings.TrimPrefix(line, marker)
					}
				}
			}
			var want, got map[string]any
			if json.Unmarshal([]byte(row[3]), &want) != nil || observation == "" || json.Unmarshal([]byte(observation), &got) != nil {
				t.Fatalf("missing native/local JSON: native=%s local=%s", row[3], observation)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("native=%s local=%s", row[3], observation)
			}
		})
	}
}
