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

func TestSameSpelledCustomFieldRuntimeNativeConformance(t *testing.T) {
	const fixtures = "../sema/testdata/same_spelled_custom_field"
	f, err := os.Open(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("incomplete native field matrix: rows=%d err=%v", len(rows), err)
	}
	methods := map[string]string{"read": "readMarker", "assign": "assignMarker", "lowercase": "lowerCaseControl"}
	for _, namespace := range []string{"", "garden"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixtures, "project"))); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(root, "sfdx-project.json")
			raw, err := os.ReadFile(config)
			if err != nil {
				t.Fatal(err)
			}
			var project map[string]any
			if err := json.Unmarshal(raw, &project); err != nil {
				t.Fatal(err)
			}
			project["namespace"] = namespace
			raw, err = json.Marshal(project)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, config, string(raw))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native field source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range rows[1:] {
				if len(row) != 4 {
					t.Fatalf("invalid native row: %v", row)
				}
				if row[2] == "false" {
					continue // Compiler rejection is checked by the semantic conformance test.
				}
				t.Run(row[0]+"/"+row[1], func(t *testing.T) {
					method, ok := methods[row[1]]
					if !ok {
						t.Fatalf("unknown native field row: %v", row)
					}
					source := "System.debug('OBS|' + GardenFieldRead" + strings.TrimSuffix(row[0], ".0") + "." + method + "());"
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.execute(program, func(machine *vm.VM) { machine.SetCurrentNamespace(namespace) })
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 1 || result.Debug[0] != "OBS|"+row[3] {
						t.Fatalf("native=%q; Glade=%v", row[3], result.Debug)
					}
				})
			}
		})
	}
}
