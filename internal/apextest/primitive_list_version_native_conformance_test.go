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

func TestPrimitiveListVersionBoundariesNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/primitive_list_version_boundaries"
	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 136 || rows[0] != "caller_api\thelper_api\tmethod\tobservation" {
		t.Fatal("incomplete native observations")
	}
	provenance, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	err = filepath.WalkDir(filepath.Join(directory, "project"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(filepath.Join(directory, "project"), path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(provenance), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
			t.Fatalf("capture source mismatch: %s", name)
		}
		writeFile(t, filepath.Join(root, name), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			class := "ScalarListTest" + strings.TrimSuffix(api, ".0")
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{class}})
			if len(cases) != 45 {
				t.Fatalf("expected 45 observations; found %d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, line := range rows[1:] {
				columns := strings.SplitN(line, "\t", 4)
				if len(columns) != 4 {
					t.Fatal("invalid native observation")
				}
				if columns[0] != api {
					continue
				}
				helper, method := columns[1], columns[2]
				if (helper != "53.0" && helper != "54.0" && helper != "55.0") || !strings.HasPrefix(method, "h"+strings.TrimSuffix(helper, ".0")+"_") || seen[api+method] {
					t.Fatal("unknown or duplicate observation")
				}
				seen[api+method] = true
				t.Run(method, func(t *testing.T) {
					key := ""
					for _, c := range cases {
						if c.MethodName == method {
							key = testCaseKey(c)
						}
					}
					if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
					}
					machine := runner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, err := machine.ExecuteInClass(programs[key], class)
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
						t.Fatalf("missing native assertion transport: %v", err)
					}
					_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "OBSERVED|"+method+"|")
					var got, want map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[3]), &want) != nil {
						t.Fatalf("invalid observation: %s", runtimeErr.ExceptionMessage())
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s", columns[3], payload)
					}
				})
			}
		})
	}
	if len(seen) != 135 {
		t.Fatal("missing caller/helper observations")
	}
}
