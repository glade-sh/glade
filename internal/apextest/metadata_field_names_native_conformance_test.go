package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestMetadataFieldNamesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/metadata_field_names"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 25 {
		t.Fatalf("native matrix: rows=%d err=%v", len(rows), err)
	}
	expected := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0" && row[0] != "68.0") || (row[1] != "anonymous" && row[1] != "istest") {
			t.Fatalf("invalid native row %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, ok := expected[key]; ok {
			t.Fatalf("duplicate native row %s", key)
		}
		expected[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "fixture"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "fixture"), path)
				if err != nil {
					return err
				}
				if strings.Contains(rel, string(filepath.Separator)+".") {
					return nil
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(source)
				if strings.HasSuffix(path, ".cls-meta.xml") {
					text = fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>", api)
				}
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					var assertions strings.Builder
					for _, id := range []string{"custom-column", "standard-column", "entity", "filtered-groups"} {
						want, ok := expected[api+"/"+route+"/"+id]
						if !ok {
							t.Fatal("missing native row")
						}
						fmt.Fprintf(&assertions, "System.assertEquals('%s',MeadowProbe.observe('%s'),'%s');\n", want, id, id)
					}
					if route == "istest" {
						writeFile(t, filepath.Join(root, "force-app/main/default/classes/MeadowNativeTest.cls"), "@IsTest private class MeadowNativeTest { @IsTest static void verifyRows(){"+assertions.String()+"}}")
						run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
						if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
							t.Fatalf("native named assertions: summary=%+v problem=%s", got, firstRunProblem(run))
						}
						return
					}
					body := assertions.String()
					if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
						t.Fatalf("native anonymous accepted: %+v", result.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err := runner.execute(program); err != nil {
						t.Fatalf("native anonymous assertions: %v", err)
					}
				})
			}
		})
	}
}
