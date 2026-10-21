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
	"github.com/glade-sh/glade/internal/vm"
)

func TestWideInterfaceOverloadNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "wide_interface_overload", "PlanProbeTest", 3)
}

func runQueryCompositionNativeConformance(t *testing.T, family, class string, methods int) {
	t.Helper()
	runNativeProjectRoutes(t, family, class, []string{"62.0", "67.0", "68.0"}, methods)
}

func runNativeProjectRoutes(t *testing.T, family, class string, apis []string, methods int) {
	t.Helper()
	runNativeProjectRouteCounts(t, family, class, apis, methods, methods)
}

func runNativeProjectRouteCounts(t *testing.T, family, class string, apis []string, methods, anonymousMethods int) {
	t.Helper()
	t.Run("named", func(t *testing.T) { runNativeObservationProject(t, family, class, apis, methods) })
	t.Run("anonymous", func(t *testing.T) {
		dir := filepath.Join("testdata/conformance", family)
		file, err := os.Open(filepath.Join(dir, "anonymous.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1+len(apis)*anonymousMethods || strings.Join(rows[0], ",") != "source_api,method,observation" {
			t.Fatal("incomplete anonymous native observations")
		}
		source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
		if err != nil {
			t.Fatal(err)
		}
		for _, api := range apis {
			t.Run(api, func(t *testing.T) {
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
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				seen := map[string]bool{}
				for _, row := range rows[1:] {
					if row[0] != api {
						continue
					}
					marker := "OBSERVED|" + row[1] + "|"
					var payload string
					for _, message := range result.Debug {
						if _, value, found := strings.Cut(message, marker); found {
							payload = value
						}
					}
					var got, want map[string]any
					if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatalf("missing native observation %s: debug=%v", row[1], result.Debug)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s native=%s; Glade=%s", row[1], row[2], payload)
					}
					if seen[row[1]] {
						t.Fatal("duplicate observation", row[1])
					}
					seen[row[1]] = true
				}
				if len(seen) != anonymousMethods {
					t.Fatal("incomplete API observations")
				}
			})
		}
	})
}
