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

	"github.com/glade-sh/glade/internal/vm"
)

func TestRecordLifecycleNativeConformance(t *testing.T) {
	const root = "testdata/conformance/stripped_feature_json"
	file, err := os.Open(filepath.Join(root, "org.tsv"))
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
	if len(rows) != 5 {
		t.Fatalf("expected four native observations, got %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			if row[2] != "istest" || row[3] != "OBSERVED" {
				t.Fatal("invalid native context")
			}
			project := t.TempDir()
			err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				if !strings.HasSuffix(rel, ".cls") && !strings.HasSuffix(rel, ".xml") && rel != "sfdx-project.json" {
					return nil
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(content), "67.0", row[1])
				writeFile(t, filepath.Join(project, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, project)
			org := orgFromIndex(index)
			// Native JSON URLs reflect the org endpoint version, independently of
			// the source API versions recorded in the class metadata and row table.
			org.APIVersion = "68.0"
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			cases := Discover(index, Options{SelectedClasses: []string{row[0]}, SelectedMethod: "collect"})
			if len(cases) != 1 {
				t.Fatalf("expected one observation, got %d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatal(methodErrors[key], programErrors[key])
			}
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatalf("missing observation: %v", execErr)
			}
			marker := "FEATURE_JSON|"
			if row[0] == "ShareDeleteObservation" {
				marker = "SHARE_DELETE|"
			}
			_, payload, found := strings.Cut(fault.ExceptionMessage(), marker)
			if !found {
				t.Fatal("missing observation marker")
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[4]), &want) != nil {
				t.Fatal("invalid observation JSON")
			}
			if row[0] == "FeatureJsonObservation" {
				decodeFeatureRecordJSON(t, got)
				decodeFeatureRecordJSON(t, want)
			}
			for key, expected := range want {
				if !reflect.DeepEqual(got[key], expected) {
					t.Errorf("%s: got %#v; native %#v", key, got[key], expected)
				}
			}
		})
	}
}

// Mock IDs use each org's assigned custom-object prefix. Compare the JSON
// structure after normalizing that identity; pretty-print whitespace is separate.
func decodeFeatureRecordJSON(t *testing.T, observation map[string]any) {
	t.Helper()
	var record struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal([]byte(observation["single"].(string)), &record); err != nil || record.ID == "" {
		t.Fatalf("missing mock identity: %v", err)
	}
	for _, key := range []string{"single", "list", "map", "blank"} {
		var decoded any
		raw := strings.ReplaceAll(observation[key].(string), record.ID, "<record>")
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			t.Fatal(err)
		}
		observation[key] = decoded
	}
}
