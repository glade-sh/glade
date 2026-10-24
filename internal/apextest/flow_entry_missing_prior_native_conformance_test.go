package apextest

import (
	"crypto/sha256"
	"encoding/csv"
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

func TestFlowEntryCreateAndUpdateNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_entry_missing_prior"
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil || !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatalf("incomplete native capture: %v", err)
	}
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 27 || strings.Join(rows[0], "\t") != "source_api\troute\tcell\tobservation" {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	native := map[string]map[string]any{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("invalid native row")
		}
		key := strings.Join(row[:3], "/")
		if _, exists := native[key]; exists {
			t.Fatalf("duplicate native row %s", key)
		}
		var value map[string]any
		if json.Unmarshal([]byte(row[3]), &value) != nil {
			t.Fatal("invalid native observation")
		}
		native[key] = value
	}
	control, err := os.ReadFile(filepath.Join(fixture, "direct-interview-entry-control.apex"))
	if err != nil || !strings.Contains(string(capture), fmt.Sprintf("direct_entry_control_sha256=%x\n", sha256.Sum256(control))) {
		t.Fatalf("detached direct-interview control: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			source := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"))
			root := t.TempDir()
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(rel), sha256.Sum256(body))) {
					t.Fatalf("native source changed: %s", rel)
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{"BirchEntryProbeTest"}})
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			check := func(t *testing.T, route, cell string, err error) {
				t.Helper()
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
					t.Fatalf("missing observation: %v", err)
				}
				_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "BIRCH|")
				var got map[string]any
				want := native[api+"/"+route+"/"+cell]
				if !found || json.Unmarshal([]byte(payload), &got) != nil || want == nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%v Glade=%v: %v", want, got, err)
				}
			}
			selected := 0
			for _, testCase := range cases {
				if strings.HasSuffix(testCase.MethodName, "_missing") {
					continue
				}
				selected++
				t.Run("istest/"+testCase.MethodName, func(t *testing.T) {
					key := testCaseKey(testCase)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
					}
					machine := base.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					_, err := machine.ExecuteTestInClass(programs[key], testCase.ClassName)
					check(t, "istest", testCase.MethodName, err)
				})
				t.Run("anonymous/"+testCase.MethodName, func(t *testing.T) {
					body, err := os.ReadFile(filepath.Join(root, "anonymous-"+testCase.MethodName+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = base.newMachine().Execute(program)
					check(t, "anonymous", testCase.MethodName, err)
				})
			}
			if selected != 4 {
				t.Fatalf("project routes=%d, want 4", selected)
			}
			// Direct record-triggered interviews are recorded separately. The native
			// after-save control bypasses start filters, so these starts do not
			// establish missing-prior entry semantics.
			for _, route := range []string{"anonymous", "istest"} {
				for _, phase := range []string{"before", "after"} {
					if row := native[api+"/"+route+"/"+phase+"_missing"]; row == nil || row["rejected"] != false {
						t.Fatalf("missing direct-interview control: %v", row)
					}
				}
			}
		})
	}
}
