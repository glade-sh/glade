package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/glade-sh/glade/internal/vm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStubRecordListArgumentsNativeAnonymousAndProjectIsTestRoutes(t *testing.T) {
	const fixture = "testdata/conformance/stub_record_list_arguments"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	if len(rows) != 13 {
		t.Fatalf("native rows=%d, want 12", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
		metas, err := filepath.Glob(filepath.Join(root, "force-app/main/default/classes/*-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range metas {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, strings.ReplaceAll(string(body), "67.0", api))
		}
		index := loadTestIndex(t, root)
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				want := map[string]string{}
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == route {
						want[row[2]] = row[3]
					}
				}
				var message string
				if route == "anonymous" {
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					machine := runner.newMachine()
					body, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					message = strings.Join(result.Debug, "\n")
				} else {
					run := Run(index, Options{SelectedClasses: []string{"CallbackShapeProbeTest"}, NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Failed != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing capture: %+v %+v", summary, run.Suites)
					}
					message = run.Suites[0].Cases[0].Problem.Message
				}
				_, payload, ok := strings.Cut(strings.ReplaceAll(message, "&#124;", "|"), "OBS|")
				if !ok {
					t.Fatalf("no observation: %q", message)
				}
				got := map[string]string{}
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%v; Glade=%v", want, got)
				}
			})
		}
	}
}
