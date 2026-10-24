package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowOperandsFactoryTypedActionAndSaveNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/flow_operand_routes"
	read := func(name string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	capture := string(read("RUN-FACTS.txt"))
	rows := read("org.tsv")
	if !strings.Contains(capture, "native_complete=true\n") || !strings.Contains(capture, fmt.Sprintf("org_tsv_sha256=%x\n", sha256.Sum256(rows))) {
		t.Fatal("native capture is incomplete or changed")
	}
	want := map[string]map[string]any{}
	for _, row := range strings.Split(strings.TrimSuffix(string(rows), "\n"), "\n")[1:] {
		parts := strings.SplitN(row, "\t", 4)
		if len(parts) != 4 {
			t.Fatal("invalid native row")
		}
		key := parts[0] + "/" + parts[1]
		if want[key] == nil {
			want[key] = map[string]any{}
		}
		var value any
		if err := json.Unmarshal([]byte(parts[3]), &value); err != nil {
			t.Fatal(err)
		}
		if _, exists := want[key][parts[2]]; exists {
			t.Fatal("duplicate native row")
		}
		want[key][parts[2]] = value
	}
	if len(want) != 4 {
		t.Fatal("native API/context matrix is incomplete")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			project := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"))
			root := t.TempDir()
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				relative, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				body := read(relative)
				if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(relative), sha256.Sum256(body))) {
					t.Fatalf("captured source changed: %s", relative)
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					var message string
					if route == "anonymous" {
						name := "api" + strings.TrimSuffix(api, ".0") + "/anonymous.apex"
						body := read(name)
						if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256(body))) {
							t.Fatal("anonymous source changed")
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var observed *vm.RuntimeError
						if !errors.As(err, &observed) || flowRefsExceptionType(observed.Type) != "System.AssertException" {
							t.Fatalf("missing observation: %v", err)
						}
						message = observed.ExceptionMessage()
					} else {
						result := Run(index, Options{SelectedClasses: []string{"CedarOperandProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
						if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("unexpected test selection: %+v", result.Summary())
						}
						problem := result.Suites[0].Cases[0].Problem
						if problem == nil || flowRefsExceptionType(problem.Type) != "System.AssertException" {
							t.Fatalf("missing observation: %+v", problem)
						}
						message = problem.Message
					}
					_, payload, found := strings.Cut(message, "CEDAR_ROUTES|")
					var got map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil {
						t.Fatalf("invalid observation: %s", message)
					}
					expected := want[api+"/"+route]
					if len(expected) != 6 {
						t.Fatal("native route twins are incomplete")
					}
					for name, value := range expected {
						t.Run(name, func(t *testing.T) {
							if !reflect.DeepEqual(got[name], value) {
								t.Errorf("native=%v; Glade=%v", value, got[name])
							}
						})
					}
				})
			}
		})
	}
}
