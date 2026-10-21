package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPersonAccountWithoutFeatureNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/person_account_without_feature"

	caseFile, err := os.Open(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer caseFile.Close()
	caseReader := csv.NewReader(caseFile)
	caseReader.Comma = '\t'
	cases, err := caseReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 23 || strings.Join(cases[0], ",") != "case_id,api,native_route,native_source,glade_source,operation" {
		t.Fatal("incomplete Person Account feature case table")
	}
	gladeSources := map[string]string{}
	for _, row := range cases[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid case row: %v", row)
		}
		if row[0] == "ACCOUNT_CTOR" {
			gladeSources[row[1]] = row[4]
		}
	}

	orgFile, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer orgFile.Close()
	orgReader := csv.NewReader(orgFile)
	orgReader.Comma = '\t'
	nativeRows, err := orgReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(nativeRows) != 23 || strings.Join(nativeRows[0], ",") != "case_id,api,route,status,observed" {
		t.Fatal("incomplete Person Account native observations")
	}
	observations := make(map[string]struct {
		status string
		value  map[string]any
	})
	for _, row := range nativeRows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[1]
		if _, exists := observations[key]; exists {
			t.Fatalf("duplicate native observation %s", key)
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(row[4]), &value); err != nil {
			t.Fatalf("invalid observation %s: %v", key, err)
		}
		observations[key] = struct {
			status string
			value  map[string]any
		}{row[3], value}
	}

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for caseID, want := range map[string]struct {
				status   string
				contains []string
			}{
				"ACCOUNT_CTOR_CLASS":  {"COMPILE_REJECTED", []string{"Field does not exist: FirstName on Account", "Field does not exist: LastName on Account"}},
				"PERSONEMAIL_DOT":     {"COMPILE_REJECTED", []string{"Variable does not exist: PersonEmail"}},
				"ISPERSON_DOT":        {"COMPILE_REJECTED", []string{"Variable does not exist: IsPersonAccount"}},
				"PERSONEMAIL_SOQL":    {"COMPILE_REJECTED", []string{"No such column 'PersonEmail' on entity 'Account'"}},
				"ISPERSON_SOQL":       {"COMPILE_REJECTED", []string{"No such column 'IsPersonAccount' on entity 'Account'"}},
				"PERSONEMAIL_DYNAMIC": {"EXECUTED_CAUGHT", []string{"System.QueryException", "No such column 'PersonEmail' on entity 'Account'"}},
				"ISPERSON_DYNAMIC":    {"EXECUTED_CAUGHT", []string{"System.QueryException", "No such column 'IsPersonAccount' on entity 'Account'"}},
				"PERSON_RECORDTYPE":   {"COMPILE_REJECTED", []string{"No such column 'IsPersonType' on entity 'RecordType'"}},
			} {
				observation := observations[caseID+"/"+api]
				if observation.status != want.status {
					t.Errorf("native %s status = %q, want %q", caseID, observation.status, want.status)
				}
				encoded, err := json.Marshal(observation.value)
				if err != nil {
					t.Fatal(err)
				}
				for _, expected := range want.contains {
					if !strings.Contains(string(encoded), expected) {
						t.Errorf("native %s observation %s does not contain %q", caseID, encoded, expected)
					}
				}
			}

			feature := observations["FEATURE_ABSENT/"+api]
			if feature.status != "OBSERVED" || feature.value["FirstName"] != false || feature.value["LastName"] != false ||
				feature.value["PersonEmail"] != false || feature.value["IsPersonAccount"] != false {
				t.Fatalf("native feature absence row = %#v", feature)
			}
			constructor := observations["ACCOUNT_CTOR/"+api]
			if constructor.status != "COMPILE_REJECTED" || constructor.value["message"] != "Field does not exist: FirstName on Account" {
				t.Fatalf("native constructor row = %#v", constructor)
			}
			dynamicDML := observations["DYNAMIC_DML/"+api]
			if dynamicDML.status != "EXECUTED_CAUGHT" {
				t.Fatalf("native DML row = %#v", dynamicDML)
			}
			firstName, _ := dynamicDML.value["FirstName"].(map[string]any)
			lastName, _ := dynamicDML.value["LastName"].(map[string]any)
			if firstName["message"] != "Invalid field FirstName for Account" || lastName["message"] != "Invalid field LastName for Account" {
				t.Fatalf("native DML field errors = %#v", dynamicDML.value)
			}

			relativeSource := gladeSources[api]
			if relativeSource == "" || relativeSource == "-" {
				t.Fatal("missing Glade constructor source")
			}
			source, err := os.ReadFile(filepath.Join(dir, relativeSource))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			scratchDefinition, err := os.ReadFile(filepath.Join(dir, "native/project/config/project-scratch-def.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(scratchDefinition))
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.AnalyzeAnonymous(index, string(source), api); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			_, execErr := runner.execute(program)
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) {
				t.Fatalf("insert error = %v; want DML rejection", execErr)
			}
			message := runtimeErr.ExceptionMessage()
			if strings.Contains(message, "IsPersonAccount") {
				t.Fatalf("DML normalization leaked an internal field: %s", message)
			}
			if !strings.Contains(message, "FirstName") && !strings.Contains(message, "LastName") {
				t.Errorf("DML diagnostic %q does not identify the rejected Person Account field", message)
			}
			for _, want := range []string{"PersonAccounts", "org.features", "config/project-scratch-def.json", "features"} {
				if !strings.Contains(message, want) {
					t.Errorf("DML diagnostic %q does not contain %q", message, want)
				}
			}
		})
	}
}
