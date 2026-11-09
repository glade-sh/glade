package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native rows for the default administrator and an ungranted optional custom
// field at class APIs 62.0-67.0: the implicit mode of plain DML and SOQL below
// API 67 and the explicit USER_MODE answers at every API. See
// testdata/conformance/admin_optional_field_explicit_mode/ORG-RUN.txt.
func TestAdminOptionalFieldExplicitModeNativeConformance(t *testing.T) {
	const family = "testdata/conformance/admin_optional_field_explicit_mode"
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 14 || strings.Join(rows[0], ",") != "source_api,method,observation" {
		t.Fatal("incomplete native rows")
	}
	for _, row := range rows[1:] {
		api, method := row[0], row[1]
		class := "AdminProbe"
		if strings.HasPrefix(method, "userMode") {
			class = "UserDml"
		}
		permSet := strings.HasSuffix(method, "PermSet") && !strings.HasSuffix(method, "NoPermSet")
		var want map[string]any
		if json.Unmarshal([]byte(row[2]), &want) != nil {
			t.Fatalf("bad row %s %s", api, method)
		}
		runConformanceSubtestParallel(t, api+"/"+method, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(family, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if permSet {
				writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [CapFls]\n")
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "adminDirect", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED|adminDirect|")
			var got map[string]any
			if !ok || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("invalid transport: %s", run.Suites[0].Cases[0].Problem.Message)
			}
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("native=%s\nGlade=%s", row[2], gotJSON)
			}
		})
	}
}
