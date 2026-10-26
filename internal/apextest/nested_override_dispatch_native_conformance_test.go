package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNestedOverrideDispatchAnonymousAndIsTestNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "nested_override_dispatch", "DispatchProbeTest", "62.0", "67.0")
}

func TestNestedOverrideDispatchAnonymousAsyncNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "nested_override_dispatch")
	file, err := os.Open(filepath.Join(dir, "async-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 3 || strings.Join(rows[0], ",") != "source_api,route,result" {
		t.Fatalf("incomplete native async observations: %v", err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			api := row[0]
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
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
			source, err := os.ReadFile(filepath.Join(dir, "anonymous-async.apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.DrainAsync(&result); err != nil {
				t.Fatalf("native anonymous finalizer returned %s: %v", row[2], err)
			}
			query, err := vm.CompileAnonymous("System.debug([SELECT Name FROM Account LIMIT 1].Name);")
			if err != nil {
				t.Fatal(err)
			}
			observed, err := machine.Execute(query)
			if err != nil || len(observed.Debug) != 1 || observed.Debug[0] != row[2] {
				t.Fatalf("native=%s; Glade=%v; error=%v", row[2], observed.Debug, err)
			}
		})
	}
}
