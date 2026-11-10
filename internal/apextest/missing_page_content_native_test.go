package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestMissingPageContentNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "missing_page_content")
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
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join("testdata", "conformance", "pdf_rendering", "project"), api)
			index := loadTestIndex(t, root)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				row := row
				t.Run(row[2], func(t *testing.T) {
					source, err := os.ReadFile(filepath.Join(dir, "anon", strings.TrimSuffix(api, ".0")+"-"+row[2]+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.execute(program, func(machine *vm.VM) {
						visualforce.SetVMRenderEnvironment(machine, p)
						t.Cleanup(func() { visualforce.ClearVMRenderEnvironment(machine) })
					})
					got := map[string]string{}
					if err != nil {
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) {
							t.Fatal(err)
						}
						got["outcome"] = "uncaught-runtime"
						got["message"] = runtimeErr.ExceptionMessage()
					} else {
						for _, line := range result.Debug {
							if strings.HasPrefix(line, "CAPTURE|caught|") {
								parts := strings.SplitN(line, "|", 4)
								if len(parts) == 4 {
									got["outcome"] = "caught"
									got["type"] = parts[2]
									got["message"] = parts[3]
								}
							}
						}
					}
					var want map[string]string
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
				})
			}
		})
	}
}
