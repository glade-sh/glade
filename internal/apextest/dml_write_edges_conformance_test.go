package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

const dmlWriteEdgesFixture = "testdata/conformance/dml_write_edges"

func TestCustomLookupInsertNativeConformance(t *testing.T) {
	runDMLWriteEdgesNative(t, "lookup")
}

func TestExternalIDUpsertKeyLengthNativeConformance(t *testing.T) {
	runDMLWriteEdgesNative(t, "upsert")
}

func TestBeforeInsertTextWriteNativeConformance(t *testing.T) {
	runDMLWriteEdgesNative(t, "text")
}

func runDMLWriteEdgesNative(t *testing.T, family string) {
	t.Helper()
	rows := readDMLWriteEdgeRows(t, "org.tsv", 4)
	cases := readDMLWriteEdgeRows(t, "cases.tsv", 5)
	if len(rows) != len(cases) {
		t.Fatalf("captured %d observations for %d cases", len(rows), len(cases))
	}
	for i, row := range rows {
		spec := cases[i]
		if row[0] != spec[0] || row[1] != spec[1] || row[2] != spec[2] || row[3] != spec[4] || spec[3] == "" {
			t.Fatalf("case table does not match the capture for %s/%s/%s", row[0], row[1], row[2])
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := dmlWriteEdgesProject(t, api)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})

			t.Run("anonymous", func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(root, family+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 {
					t.Fatalf("expected one observation, got %v", result.Debug)
				}
				checkDMLWriteEdges(t, rows, family, api, "anonymous", result.Debug[0])
			})

			t.Run("isTest", func(t *testing.T) {
				cases := Discover(index, Options{SelectedClasses: []string{"DmlWriteProbeTest"}, SelectedMethod: family + "Route"})
				if len(cases) != 1 {
					t.Fatalf("expected one test method, found %d", len(cases))
				}
				tc := cases[0]
				key := testCaseKey(tc)
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("test compilation: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				machine.EnableTestContext()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				_, err := machine.ExecuteInClass(programs[key], tc.ClassName)
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
					t.Fatalf("missing terminal assertion: %v", err)
				}
				checkDMLWriteEdges(t, rows, family, api, "isTest", runtimeErr.ExceptionMessage())
			})
		})
	}
}

func dmlWriteEdgesProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(dmlWriteEdgesFixture, "project"))); err != nil {
		t.Fatal(err)
	}
	files := []string{
		"sfdx-project.json",
		"force-app/main/default/classes/DmlWriteProbe.cls-meta.xml",
		"force-app/main/default/classes/DmlWriteProbeTest.cls-meta.xml",
		"force-app/main/default/triggers/PebbleTextProbe.trigger-meta.xml",
	}
	for _, name := range files {
		path := filepath.Join(root, name)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, strings.ReplaceAll(string(source), "62.0", api))
	}
	return root
}

func readDMLWriteEdgeRows(t *testing.T, file string, width int) [][]string {
	t.Helper()
	data, err := os.Open(filepath.Join(dmlWriteEdgesFixture, file))
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	reader := csv.NewReader(data)
	reader.Comma = '\t'
	reader.FieldsPerRecord = width
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || rows[0][0] != "case" {
		t.Fatalf("missing case rows in %s", file)
	}
	return rows[1:]
}

func checkDMLWriteEdges(t *testing.T, rows [][]string, family, api, route, message string) {
	t.Helper()
	_, payload, found := strings.Cut(message, "R563")
	if !found {
		t.Fatalf("missing terminal marker: %s", message)
	}
	var actual map[string]string
	if err := json.Unmarshal([]byte(payload), &actual); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, row := range rows {
		name, version, context, want := row[0], row[1], row[2], row[3]
		if version != api || context != route || !strings.HasPrefix(name, family+"_") {
			continue
		}
		if seen[name] {
			t.Fatalf("duplicate native observation for %s", name)
		}
		seen[name] = true
		t.Run(strings.TrimPrefix(name, family+"_"), func(t *testing.T) {
			got, present := actual[name]
			if !present {
				t.Fatalf("missing observation %s", name)
			}
			if family == "text" {
				checkInsertTrigger := name != "text_update"
				got = plainTextWriteObservation(t, got, checkInsertTrigger)
				want = plainTextWriteObservation(t, want, checkInsertTrigger)
			}
			if got != want {
				t.Fatalf("Glade %q, native %q", got, want)
			}
		})
	}
	if len(seen) == 0 || len(seen) != len(actual) {
		t.Fatalf("captured %d observations, replayed %d", len(seen), len(actual))
	}
}

// Replay the plain Text caller and storage fields, plus the before-insert
// trigger values. The fixture also records Text Area and before-update values.
func plainTextWriteObservation(t *testing.T, value string, checkInsertTrigger bool) string {
	t.Helper()
	fields := strings.Split(value, "|")
	if len(fields) != 11 {
		t.Fatalf("expected 11 framed text values, got %q", value)
	}
	callerAndStored := strings.Join(fields[:5], "|")
	if checkInsertTrigger {
		return callerAndStored + "|" + strings.Join(fields[7:9], "|")
	}
	return callerAndStored
}
