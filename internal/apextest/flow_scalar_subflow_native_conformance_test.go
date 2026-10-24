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

func TestFlowScalarSubflowNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_scalar_subflow"
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil || !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatalf("incomplete native capture: %v", err)
	}
	for _, api := range []string{"62", "67"} {
		path := "reject" + api + "/force-app/main/default/flows/CedarScalarUnknown.flow-meta.xml"
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil || !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256(body))) {
			t.Fatalf("native rejection source changed: %s: %v", path, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), fmt.Sprintf("org_tsv_sha256=%x\n", sha256.Sum256(raw))) {
		t.Fatal("native rows changed")
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 51 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	native := map[string]map[string]any{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("invalid native row")
		}
		key := strings.Join(row[:3], "/")
		if native[key] != nil {
			t.Fatalf("duplicate row %s", key)
		}
		var value map[string]any
		if json.Unmarshal([]byte(row[3]), &value) != nil {
			t.Fatal("invalid native observation")
		}
		native[key] = value
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			source := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"))
			root := t.TempDir()
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				pin := fmt.Sprintf("source_sha256:api%s/%s=%x\n", strings.TrimSuffix(api, ".0"), filepath.ToSlash(rel), sha256.Sum256(body))
				if !strings.Contains(string(capture), pin) {
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
			cases := Discover(index, Options{SelectedClasses: []string{"CedarScalarProbeTest"}})
			if len(cases) != 12 {
				t.Fatalf("named routes=%d, want 12", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			check := func(t *testing.T, route, cell string, err error) {
				t.Helper()
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
					t.Fatalf("missing native marker: %v", err)
				}
				_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "CEDAR|"+cell+"|")
				var got map[string]any
				want := native[api+"/"+route+"/"+cell]
				if !found || json.Unmarshal([]byte(payload), &got) != nil || want == nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%v Glade=%v: %v", want, got, err)
				}
			}
			for _, tc := range cases {
				t.Run("istest/"+tc.MethodName, func(t *testing.T) {
					key := testCaseKey(tc)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
					}
					machine := base.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					_, err := machine.ExecuteTestInClass(programs[key], tc.ClassName)
					check(t, "istest", tc.MethodName, err)
				})
				t.Run("anonymous/"+tc.MethodName, func(t *testing.T) {
					body, err := os.ReadFile(filepath.Join(root, tc.MethodName+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					if analysis := sema.AnalyzeAnonymous(index, string(body), api); analysis.HasErrors() {
						t.Fatalf("native anonymous compiled: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = base.newMachine().Execute(program)
					check(t, "anonymous", tc.MethodName, err)
				})
			}
			t.Run("test_runner", func(t *testing.T) {
				path := filepath.Join(root, "force-app/main/default/classes/CedarScalarProbeTest.cls")
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				text := string(body)
				for _, tc := range cases {
					cell := tc.MethodName
					want := native[api+"/istest/"+cell]
					if len(want) != 1 {
						t.Fatalf("native success observation: %v", want)
					}
					var literal string
					switch value := want["value"].(type) {
					case bool:
						literal = fmt.Sprint(value)
					case float64:
						literal = fmt.Sprint(value)
					case string:
						literal = conformanceApexString(value)
					default:
						t.Fatalf("unexpected native value %v", want)
					}
					transport := "System.assert(false,'CEDAR|" + cell + "|'+JSON.serialize(observed));"
					if strings.Count(text, transport) != 1 {
						t.Fatalf("missing captured transport %s", cell)
					}
					text = strings.Replace(text, transport, "System.assertEquals(1,observed.size()); System.assertEquals("+literal+",observed.get('value'));", 1)
				}
				writeFile(t, path, text)
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"CedarScalarProbeTest"}, NoDiskCache: true})
				if summary := run.Summary(); summary.Total != 12 || summary.Passed != 12 {
					t.Fatalf("native project runner: %#v: %s", summary, firstRunProblem(run))
				}
			})
			if rejection := native[api+"/deploy/unknown_target"]; rejection == nil || rejection["accepted"] != false {
				t.Fatal("missing native rejection twin")
			}
		})
	}
}
