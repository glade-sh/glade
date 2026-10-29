package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestLookupFilterValidationNativeConformanceIsTestProjectAPIRoutes(t *testing.T) {
	const fixture = "testdata/conformance/lookup_filter_validation"
	cases := readLookupFilterValidationTSV(t, filepath.Join(fixture, "cases.tsv"))
	caseHash := lookupFilterValidationFileSHA256(t, filepath.Join(fixture, "cases.tsv"))
	classPath := filepath.Join(fixture, "project/force-app/main/default/classes/LookupFilterObservationTest.cls")
	classMetaPath := filepath.Join(fixture, "project/force-app/main/default/classes/LookupFilterObservationTest.cls-meta.xml")
	classHash := lookupFilterValidationFileSHA256(t, classPath)

	var sharedNative map[string]lookupFilterValidationOutcome
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("API"+strings.TrimSuffix(api, ".0"), func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			nativeDir := filepath.Join(fixture, "native", version)
			classMeta, err := os.ReadFile(classMetaPath)
			if err != nil {
				t.Fatal(err)
			}
			metaHash := lookupFilterValidationContentSHA256([]byte(strings.ReplaceAll(string(classMeta), "67.0", api)))
			projectHash := lookupFilterValidationProjectSHA256(t, filepath.Join(fixture, "project"), api)
			validateLookupFilterValidationRun(t, filepath.Join(nativeDir, "RUN-FACTS.txt"), api, len(cases), caseHash, classHash, metaHash, projectHash, filepath.Join(fixture, "project"))
			native := readLookupFilterValidationOutcomes(t, filepath.Join(nativeDir, "org.tsv"))
			if len(native) != len(cases) {
				t.Fatalf("native rows = %d, cases = %d", len(native), len(cases))
			}
			for _, row := range cases {
				if _, ok := native[row.ID]; !ok {
					t.Fatalf("native rows missing case %s", row.ID)
				}
			}
			if sharedNative == nil {
				sharedNative = native
			} else if !reflect.DeepEqual(native, sharedNative) {
				t.Fatalf("API %s native rows differ from the other captured API context", api)
			}

			projectRoot := copyLookupFilterValidationProject(t, filepath.Join(fixture, "project"), api)
			index := loadTestIndex(t, projectRoot)
			// API 62 is retained as source metadata and native evidence. Glade's
			// REST endpoint contexts currently start at 60.0, 65.0, 66.0, 67.0,
			// and 68.0, so replay the source-62 class at the default 65.0 endpoint.
			restAPI := api
			if api == "62.0" {
				restAPI = "65.0"
			}
			run := Run(index, Options{
				SelectedClasses:       []string{"LookupFilterObservationTest"},
				SelectedMethod:        "observeInsertOutcomes",
				RuntimeRESTAPIVersion: restAPI,
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Summary().Total != 1 {
				t.Fatalf("expected one selected observation test, got summary=%#v suites=%#v", run.Summary(), run.Suites)
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing observation assertion: result=%#v problem=%#v", result, result.Problem)
			}
			const marker = "LOOKUP_FILTER_OBSERVATION|"
			if strings.Count(result.Problem.Message, marker) != 1 {
				t.Fatalf("expected one native observation marker, got %q", result.Problem.Message)
			}
			_, payload, found := strings.Cut(result.Problem.Message, marker)
			if !found {
				t.Fatal("missing native observation payload")
			}
			var gotRows []lookupFilterValidationOutcome
			if err := json.Unmarshal([]byte(payload), &gotRows); err != nil {
				t.Fatalf("invalid observation payload %q: %v", payload, err)
			}
			got := make(map[string]lookupFilterValidationOutcome, len(gotRows))
			for _, row := range gotRows {
				if row.ID == "" {
					t.Fatal("observation row has no case id")
				}
				if _, duplicate := got[row.ID]; duplicate {
					t.Fatalf("duplicate observation case %s", row.ID)
				}
				got[row.ID] = normalizeLookupFilterValidationOutcome(row)
			}
			if len(got) != len(cases) {
				t.Fatalf("observation rows = %d, cases = %d", len(got), len(cases))
			}
			for _, testCase := range cases {
				want, ok := native[testCase.ID]
				if !ok {
					t.Fatalf("native rows missing case %s", testCase.ID)
				}
				observed, ok := got[testCase.ID]
				if !ok {
					t.Errorf("Glade observation missing case %s", testCase.ID)
					continue
				}
				if !reflect.DeepEqual(observed, want) {
					t.Errorf("%s native=%#v; Glade=%#v", testCase.ID, want, observed)
				}
			}
		})
	}
}

type lookupFilterValidationCase struct {
	ID string
}

type lookupFilterValidationOutcome struct {
	ID      string   `json:"id"`
	Success bool     `json:"success"`
	Status  string   `json:"status,omitempty"`
	Fields  []string `json:"fields,omitempty"`
	Message string   `json:"message,omitempty"`
}

func readLookupFilterValidationTSV(t *testing.T, path string) []lookupFilterValidationCase {
	t.Helper()
	rows := readLookupFilterValidationTable(t, path)
	if len(rows) < 2 || len(rows[0]) == 0 || rows[0][0] != "id" {
		t.Fatalf("invalid case table %s", path)
	}
	cases := make([]lookupFilterValidationCase, 0, len(rows)-1)
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != len(rows[0]) || row[0] == "" || seen[row[0]] {
			t.Fatalf("invalid or duplicate case row in %s: %#v", path, row)
		}
		seen[row[0]] = true
		cases = append(cases, lookupFilterValidationCase{ID: row[0]})
	}
	return cases
}

func readLookupFilterValidationOutcomes(t *testing.T, path string) map[string]lookupFilterValidationOutcome {
	t.Helper()
	rows := readLookupFilterValidationTable(t, path)
	if len(rows) < 2 || !reflect.DeepEqual(rows[0], []string{"case_id", "success", "status", "fields", "message"}) {
		t.Fatalf("invalid native outcome header in %s: %#v", path, rows[0])
	}
	outcomes := make(map[string]lookupFilterValidationOutcome, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != 5 || row[0] == "" {
			t.Fatalf("invalid native outcome row in %s: %#v", path, row)
		}
		success, err := strconv.ParseBool(row[1])
		if err != nil {
			t.Fatalf("invalid success value in %s: %q", path, row[1])
		}
		outcome := lookupFilterValidationOutcome{ID: row[0], Success: success, Status: row[2], Message: row[4]}
		if outcome.Status == "-" {
			outcome.Status = ""
		}
		if outcome.Message == "-" {
			outcome.Message = ""
		}
		if row[3] != "" && row[3] != "-" {
			outcome.Fields = strings.Split(row[3], ",")
		}
		if _, duplicate := outcomes[outcome.ID]; duplicate {
			t.Fatalf("duplicate native outcome %s in %s", outcome.ID, path)
		}
		outcomes[outcome.ID] = normalizeLookupFilterValidationOutcome(outcome)
	}
	return outcomes
}

func readLookupFilterValidationTable(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return rows
}

func validateLookupFilterValidationRun(t *testing.T, path, api string, count int, caseHash, classHash, classMetaHash, projectHash, project string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = value
		}
	}
	for key, value := range map[string]string{
		"api":                             api,
		"context":                         "istest",
		"api_endpoint":                    api,
		"rows":                            strconv.Itoa(count),
		"native_complete":                 "true",
		"deployment_success":              "true",
		"class_api_version":               api,
		"test_class":                      "LookupFilterObservationTest",
		"test_method":                     "observeInsertOutcomes",
		"test_level":                      "RunSpecifiedTests",
		"result_marker":                   "LOOKUP_FILTER_OBSERVATION",
		"assertion_marker_is_intentional": "true",
		"cli_exit_code":                   "100",
		"case_table_sha256":               caseHash,
		"class_source_sha256":             classHash,
		"class_meta_sha256":               classMetaHash,
		"project_hash_method":             "sorted-path-nul-content-nul-v1",
		"project_sha256":                  projectHash,
	} {
		if values[key] != value {
			t.Errorf("%s: %s = %q, want %q", path, key, values[key], value)
		}
	}
	defaultAccess := "SYSTEM_MODE"
	permissionSet := "not_required"
	if api == "67.0" {
		defaultAccess = "USER_MODE"
		permissionSet = "LookupFilterValidationAccess"
		permissionSetPath := filepath.Join(project, "force-app/main/default/permissionsets/LookupFilterValidationAccess.permissionset-meta.xml")
		if values["permission_set_assigned"] != "true" || values["permission_set_sha256"] != lookupFilterValidationFileSHA256(t, permissionSetPath) {
			t.Errorf("%s: API 67 permission set assignment or source hash is invalid", path)
		}
	}
	if values["default_dml_access"] != defaultAccess || values["permission_set"] != permissionSet {
		t.Errorf("%s: default DML access / permission set = %q / %q, want %q / %q", path, values["default_dml_access"], values["permission_set"], defaultAccess, permissionSet)
	}
}

func lookupFilterValidationProjectSHA256(t *testing.T, root, api string) string {
	t.Helper()
	type sourceFile struct {
		path    string
		content []byte
	}
	var files []sourceFile
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, sourceFile{path: filepath.ToSlash(rel), content: []byte(strings.ReplaceAll(string(content), "67.0", api))})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	hash := sha256.New()
	for _, file := range files {
		_, _ = hash.Write([]byte(file.path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(file.content)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func copyLookupFilterValidationProject(t *testing.T, source, api string) string {
	t.Helper()
	project := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = []byte(strings.ReplaceAll(string(content), "67.0", api))
		destination := filepath.Join(project, rel)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destination, content, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func lookupFilterValidationFileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return lookupFilterValidationContentSHA256(data)
}

func lookupFilterValidationContentSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeLookupFilterValidationOutcome(outcome lookupFilterValidationOutcome) lookupFilterValidationOutcome {
	if len(outcome.Fields) == 0 {
		outcome.Fields = nil
	}
	return outcome
}
