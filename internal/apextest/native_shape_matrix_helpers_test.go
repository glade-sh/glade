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

// runNativeShapeMatrix replays a native capture whose rows are
// source_api, route, shape, observation. Each shape is a static method of
// probe returning a Map that the capture serialized. The anonymous route runs
// the method in a fresh runner; the isTest route runs the same-named test
// method of the retained test class and reads the AssertException transport.
func runNativeShapeMatrix(t *testing.T, family, probe string, skip map[string]bool) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "source_api,route,shape,observation" {
		t.Fatal("incomplete native matrix")
	}
	for _, row := range rows[1:] {
		api, route, shape, native := row[0], row[1], row[2], row[3]
		if skip[route+"/"+shape] {
			continue
		}
		t.Run(api+"/"+route+"/"+shape, func(t *testing.T) {
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
				source := "System.debug('OBS|' + JSON.serialize(" + probe + "." + shape + "()));"
				anon := sema.AnalyzeAnonymous(index, source, api)
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if anon.HasErrors() || err != nil {
					t.Fatalf("native anonymous compiled; diagnostics=%v compile=%v", anon.Diagnostics, err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program, func(*vm.VM) {})
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if at := strings.Index(line, "OBS|"); at >= 0 {
						observed = line[at+len("OBS|"):]
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{probe + "Test"}, SelectedMethod: shape, NoDiskCache: true, Parallelism: 1})
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
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(native), &want) != nil {
				t.Fatalf("invalid observation: %q", observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
