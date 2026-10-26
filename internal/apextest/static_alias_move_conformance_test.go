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

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStaticRecordAliasMoveNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/static_alias_move"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("incomplete native matrix: %d rows, %v", len(rows), err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(raw), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			check := func(route, variant, payload string) {
				t.Helper()
				var got, want any
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid %s/%s payload: %q", route, variant, payload)
				}
				for _, row := range rows[1:] {
					if len(row) != 4 || row[0] != api || row[1] != route || row[2] != variant {
						continue
					}
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s/%s native=%s local=%s", route, variant, row[3], payload)
					}
					canonical, _ := json.Marshal(got)
					t.Logf("RESULT|%s/%s/%s|%s", api, route, variant, canonical)
					return
				}
				t.Fatalf("missing native row %s/%s/%s", api, route, variant)
			}
			t.Run("anonymous", func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(project, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				_, err = runner.execute(program)
				if err == nil {
					t.Fatal("missing anonymous assertion observation")
				}
				_, payload, ok := strings.Cut(err.Error(), "OBS|")
				observed := map[string]json.RawMessage{}
				if !ok || json.Unmarshal([]byte(payload), &observed) != nil || len(observed) != 4 {
					t.Fatalf("invalid anonymous observation: %v", err)
				}
				for variant, body := range observed {
					check("anonymous", variant, string(body))
				}
			})
			t.Run("named_runner_lsp", func(t *testing.T) {
				result := Run(index, Options{SelectedClasses: []string{"StaticMoveProbeTest"}, NoDiskCache: true, Parallelism: 1})
				if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 4 {
					t.Fatalf("missing native methods: %+v", result.Summary())
				}
				for _, cell := range result.Suites[0].Cases {
					if cell.Problem == nil || !strings.HasSuffix(cell.Problem.Type, "AssertException") {
						t.Fatalf("missing observation: %+v", cell)
					}
					_, payload, ok := strings.Cut(cell.Problem.Message, "OBS|"+cell.MethodName+"|")
					if !ok {
						t.Fatalf("missing payload: %s", cell.Problem.Message)
					}
					check("istest", cell.MethodName, payload)
				}
				diagnostics := lsp.TestDiagnostics(result)
				if len(diagnostics) != 4 {
					t.Fatalf("LSP observation diagnostics=%d, want 4", len(diagnostics))
				}
				for _, diag := range diagnostics {
					if diag.Code != "GLADETEST001" || !strings.Contains(diag.Message, "OBS|") {
						t.Fatalf("LSP lost assertion observation: %+v", diag)
					}
				}
			})
		})
	}
}
