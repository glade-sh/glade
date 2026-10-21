package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestModifiedQueryOmissionNamespacedNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/modified_query_omission/namespaced"
	const namespace = "omissionns"
	// Carry: API 67 has no answers after fixture DML accessibility failures;
	// API 68 is unprobed. Only the complete API 62 capture is replayed here.
	t.Log("native carries: capture API 67 NO_ANSWER; capture API 68 UNPROBED")
	root := t.TempDir()
	err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), string(source))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":"62.0","namespace":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace))
	file, err := os.Open(filepath.Join(fixture, "native/api62/istest/org.tsv"))
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
	if len(rows) != 71 || strings.Join(rows[0], "\t") != "id\tstatus\tvalue" {
		t.Fatalf("expected 70 native namespaced matrix cells, got %d", len(rows)-1)
	}
	index := loadTestIndex(t, root)
	if index.HasErrors() {
		t.Fatal(index.Diagnostics)
	}
	org := orgFromIndex(index)
	if org.Namespace != namespace {
		t.Fatalf("namespace got %q; want %q", org.Namespace, namespace)
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	visited := make(map[string]bool)
	for i, api := range []string{"24", "40", "43", "44", "62"} {
		t.Run("reader"+api, func(t *testing.T) {
			id := fmt.Sprintf("R%03d", i+1)
			class := "Api43OchreTest62R" + api
			cases := Discover(index, Options{SelectedClasses: []string{class}})
			if len(cases) != 1 {
				t.Fatalf("expected one namespaced observer, got %d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatal(methodErrors[key], programErrors[key])
			}
			machine := runner.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatalf("missing native observation: %v", execErr)
			}
			_, payload, ok := strings.Cut(fault.ExceptionMessage(), "API43_NS|"+id+"|")
			if !ok {
				t.Fatalf("missing observation payload: %v", execErr)
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, row := range rows[1:] {
				if !strings.HasPrefix(row[0], id+"_") {
					continue
				}
				if row[1] != "OBSERVED" || visited[row[0]] {
					t.Fatalf("invalid native cell: %v", row)
				}
				visited[row[0]] = true
				count++
				mode := strings.TrimPrefix(row[0], id+"_")
				t.Run(mode, func(t *testing.T) {
					if got[mode] != row[2] {
						t.Errorf("got %s; native %s", got[mode], row[2])
					}
				})
			}
			if count != 14 || count != len(got) {
				t.Fatalf("native cells %d; local cells %d", count, len(got))
			}
		})
	}
	if len(visited) != len(rows)-1 {
		t.Fatalf("replayed %d of %d native cells", len(visited), len(rows)-1)
	}
}

func TestModifiedQueryOmissionNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/modified_query_omission"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if entry.Name() == "MxTest.cls-meta.xml" || entry.Name() == "MyTest.cls-meta.xml" {
					source = []byte(strings.ReplaceAll(string(source), "67.0", api))
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), string(source))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			file, err := os.Open(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv"))
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
			if len(rows) != 413 || strings.Join(rows[0], "\t") != "class\tmethod\ttag\tmode\tresult" {
				t.Fatalf("expected 412 native matrix cells, got %d", len(rows)-1)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{"MxTest", "MyTest"}})
			if len(cases) != 46 {
				t.Fatalf("expected 46 native observation methods, got %d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			visited := make(map[string]bool)
			for _, test := range cases {
				key := testCaseKey(test)
				t.Run(test.ClassName+"/"+test.MethodName, func(t *testing.T) {
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatal(methodErrors[key], programErrors[key])
					}
					machine := runner.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					_, execErr := machine.ExecuteInClass(programs[key], test.ClassName)
					var fault *vm.RuntimeError
					if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("missing native observation: %v", execErr)
					}
					_, payload, ok := strings.Cut(fault.ExceptionMessage(), "OBS|")
					if !ok {
						t.Fatalf("missing observation payload: %v", execErr)
					}
					tag, payload, ok := strings.Cut(payload, "|")
					if !ok {
						t.Fatalf("invalid observation: %s", payload)
					}
					got := make(map[string]string)
					for _, part := range strings.Fields(payload) {
						mode, result, ok := strings.Cut(part, "=")
						if !ok {
							t.Fatalf("invalid mode observation: %s", part)
						}
						if _, duplicate := got[mode]; duplicate {
							t.Fatalf("duplicate mode: %s", mode)
						}
						got[mode] = result
					}
					count := 0
					for _, row := range rows[1:] {
						if row[0] != test.ClassName || row[1] != test.MethodName {
							continue
						}
						cell := strings.Join(row[:4], "/")
						if visited[cell] {
							t.Fatalf("duplicate native cell: %s", cell)
						}
						visited[cell] = true
						count++
						if tag != row[2] {
							t.Fatalf("tag got %s; native %s", tag, row[2])
						}
						t.Run(row[3], func(t *testing.T) {
							if got[row[3]] != row[4] {
								t.Errorf("got %s; native %s", got[row[3]], row[4])
							}
						})
					}
					if count == 0 || count != len(got) {
						t.Fatalf("native cells %d; local cells %d", count, len(got))
					}
				})
			}
			if len(visited) != len(rows)-1 {
				t.Fatalf("replayed %d of %d native cells", len(visited), len(rows)-1)
			}
		})
	}
	for _, legacy := range []struct {
		fixture, entryAPI string
		carried           int
	}{
		{"legacy_lookup_projection", "62.0", 55},
		{"legacy_lookup_entry40_restrict", "40.0", 5},
	} {
		t.Run(legacy.fixture, func(t *testing.T) {
			runLegacyLookupProjectionOrgConformance(t, "testdata/conformance/"+legacy.fixture, legacy.entryAPI, legacy.carried)
		})
	}
}
