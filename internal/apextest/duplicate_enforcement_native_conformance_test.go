package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

const duplicateEnforcementFixture = "testdata/conformance/duplicate_enforcement"

type duplicateNativeCase struct {
	ID      string   `json:"id"`
	Shape   string   `json:"shape"`
	Fixture string   `json:"fixture"`
	Method  string   `json:"method"`
	APIs    []string `json:"apis"`
	Routes  []string `json:"routes"`
}

type duplicateNativeData struct {
	Cases           []duplicateNativeCase        `json:"cases"`
	Projects        map[string]map[string]string `json:"projects"`
	HeaderRejection map[string]string            `json:"header_rejection"`
	Cleanup         struct {
		Deleted     bool `json:"org_deleted"`
		Verified    bool `json:"org_deletion_verified"`
		AuthRemoved bool `json:"auth_removed"`
	} `json:"cleanup"`
	Committed map[string]string `json:"committed"`
}

func readDuplicateNativeData(t *testing.T) (duplicateNativeData, map[string]map[string]any) {
	t.Helper()
	var data duplicateNativeData
	raw, err := os.ReadFile(filepath.Join(duplicateEnforcementFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(duplicateEnforcementFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "api\troute\tid\tvalue" {
		t.Fatal("invalid native row header")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatalf("invalid native row: %q", line)
		}
		key := cells[0] + "/" + cells[1] + "/" + cells[2]
		if rows[key] != nil {
			t.Fatalf("duplicate native row: %s", key)
		}
		var value map[string]any
		if err = json.Unmarshal([]byte(cells[3]), &value); err != nil || value == nil {
			t.Fatalf("invalid native value %s: %v", key, err)
		}
		rows[key] = value
	}
	return data, rows
}

func TestDuplicateEnforcementNativeRows(t *testing.T) {
	data, rows := readDuplicateNativeData(t)
	if !data.Cleanup.Deleted || !data.Cleanup.Verified || !data.Cleanup.AuthRemoved {
		t.Fatal("owned scratch deletion has not been verified")
	}
	if len(data.Cases) != 31 {
		t.Fatalf("case count=%d; want 31", len(data.Cases))
	}
	seen := map[string]bool{}
	expected := 0
	for _, c := range data.Cases {
		if c.ID == "" || c.Shape == "" || c.Method == "" || seen[c.ID] {
			t.Fatalf("invalid case: %+v", c)
		}
		seen[c.ID] = true
		if !reflect.DeepEqual(c.APIs, []string{"62.0", "67.0"}) {
			t.Fatalf("missing source API twin: %+v", c)
		}
		wantRoutes := []string{"anonymous", "istest"}
		if c.Fixture == "metadata" {
			wantRoutes = []string{"metadata"}
		}
		if c.Fixture == "committed" {
			wantRoutes = []string{"committed_anonymous"}
		}
		if !reflect.DeepEqual(c.Routes, wantRoutes) {
			t.Fatalf("missing native route twin: %+v", c)
		}
		for _, api := range c.APIs {
			for _, route := range c.Routes {
				key := api + "/" + route + "/" + c.ID
				if rows[key] == nil {
					t.Errorf("missing native row %s", key)
				}
				expected++
			}
		}
	}
	for i := 1; i <= 17; i++ {
		if !seen[fmt.Sprintf("DR%02d", i)] {
			t.Errorf("missing duplicate scenario DR%02d", i)
		}
	}
	if len(rows) != expected {
		t.Fatalf("native rows=%d; case table requires %d", len(rows), expected)
	}
}

func duplicateProject(t *testing.T, data duplicateNativeData, fixture, api string) string {
	t.Helper()
	if fixture == "existing" {
		return copyPlatformTransactionProject(t, "duplicate_rules", api)
	}
	files := data.Projects[fixture]
	if len(files) == 0 {
		t.Fatalf("missing project %s", fixture)
	}
	root := t.TempDir()
	for name, body := range files {
		if filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") {
			t.Fatalf("invalid fixture path %q", name)
		}
		writeFile(t, filepath.Join(root, name), strings.ReplaceAll(body, "62.0", api))
	}
	return root
}

func assertDuplicateNativePayload(t *testing.T, marker, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, marker)
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing observation %q: %s", marker, message)
	}
	if !reflect.DeepEqual(got, want) {
		native, _ := json.Marshal(want)
		t.Fatalf("native=%s; Glade=%s", native, payload)
	}
}

func TestDuplicateEnforcementNativeConformanceAPIRoutes(t *testing.T) {
	data, rows := readDuplicateNativeData(t)
	for _, fixture := range []string{"existing", "extension", "standard", "standard_extended"} {
		class, helper, tag := "StandardDuplicateProbeTest", "StandardDuplicateProbe", "STANDARD_OBS|"
		if fixture == "existing" {
			class, helper, tag = "CapturePlatformProbeTest", "CapturePlatformProbe", "PLATFORM_OBS|"
		}
		if fixture == "extension" {
			class, helper, tag = "PathObservationTest", "PathObservation", "PATH_OBS|"
		}
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(fixture+"/"+api, func(t *testing.T) {
				var cases []duplicateNativeCase
				for _, c := range data.Cases {
					if c.Fixture == fixture {
						cases = append(cases, c)
					}
				}
				if len(cases) == 0 {
					t.Fatal("empty fixture selection")
				}
				index := loadTestIndex(t, duplicateProject(t, data, fixture, api))
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				// Native anonymous and named routes run the deployed triggers (DR13).
				triggers, failures := compileProjectTriggers(index)
				if len(failures) != 0 {
					t.Fatal(failures)
				}
				for _, trigger := range triggers {
					if err := runner.base.RegisterTrigger(trigger); err != nil {
						t.Fatal(err)
					}
				}
				discovered := map[string]TestCase{}
				for _, c := range Discover(index, Options{SelectedClasses: []string{class}}) {
					discovered[c.MethodName] = c
				}
				var selected []TestCase
				used := map[string]bool{}
				for _, c := range cases {
					row, ok := discovered[c.Method]
					if !ok {
						t.Fatalf("missing native named route %s.%s", class, c.Method)
					}
					if !used[c.Method] {
						selected = append(selected, row)
						used[c.Method] = true
					}
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, selected)
				programs, programErrors := compileTestInvokePrograms(selected)
				for _, c := range cases {
					marker := tag + c.Method + "|"
					t.Run(c.ID+"/anonymous", func(t *testing.T) {
						source := helper + ".emit('" + c.Method + "');"
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("native observation completed; Glade=%v", err)
						}
						assertDuplicateNativePayload(t, marker, fault.ExceptionMessage(), rows[api+"/anonymous/"+c.ID])
					})
					t.Run(c.ID+"/named_istest", func(t *testing.T) {
						row := discovered[c.Method]
						key := testCaseKey(row)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("native observation completed; Glade=%v", err)
						}
						assertDuplicateNativePayload(t, marker, fault.ExceptionMessage(), rows[api+"/istest/"+c.ID])
					})
				}
				t.Run("project_test_runner", func(t *testing.T) {
					report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1}, selected)
					results := map[string]testreport.Case{}
					for _, suite := range report.Suites {
						for _, row := range suite.Cases {
							results[row.MethodName] = row
						}
					}
					for _, c := range cases {
						t.Run(c.ID, func(t *testing.T) {
							result, ok := results[c.Method]
							if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing native observation; result=%+v exclusions=%v", result, report.ExcludedClasses)
							}
							assertDuplicateNativePayload(t, tag+c.Method+"|", result.Problem.Message, rows[api+"/istest/"+c.ID])
						})
					}
				})
			})
		}
	}
}

func TestDuplicateHeaderNativeRejectionAPIRoutes(t *testing.T) {
	data, rows := readDuplicateNativeData(t)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				want := rows[api+"/"+route+"/HD01"]
				if want["compiled"] != false || !strings.Contains(fmt.Sprint(want["problem"]), "includeRecordDetails") {
					t.Fatalf("missing native field rejection: %v", want)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/HeaderAnchor.cls"), "public class HeaderAnchor {}")
				if route == "istest" {
					writeFile(t, filepath.Join(root, "force-app/main/default/classes/HeaderRecordDetailsRejectionTest.cls"), data.HeaderRejection["named"])
					writeFile(t, filepath.Join(root, "force-app/main/default/classes/HeaderRecordDetailsRejectionTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				if route == "anonymous" {
					analysis = sema.AnalyzeAnonymous(index, data.HeaderRejection["anonymous"], api)
				}
				if (!index.HasErrors() && !analysis.HasErrors()) || !strings.Contains(fmt.Sprint(index.Diagnostics, analysis.Diagnostics), "includeRecordDetails") {
					t.Fatalf("native rejected includeRecordDetails; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
				}
			})
		}
	}
}

func TestDuplicateStandardRuleMetadataNativeConformance(t *testing.T) {
	data, rows := readDuplicateNativeData(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, duplicateProject(t, data, "standard", api))
			org := orgFromIndex(index)
			native := rows[api+"/metadata/MD01"]
			documents, ok := native["matchingRuleDocuments"].(map[string]any)
			if !ok {
				t.Fatal("missing native matching-rule retrieval")
			}
			for _, object := range []string{"Account", "Contact", "Lead"} {
				if documents[object] != float64(0) {
					t.Fatalf("native standard matching-rule retrieval for %s is no longer empty: %v", object, documents[object])
				}
			}
			rules, ok := native["rules"].([]any)
			if !ok || len(rules) != 5 {
				t.Fatalf("missing native default duplicate rules: %v", native)
			}
			for _, value := range rules {
				want := value.(map[string]any)
				object := want["object"].(string)
				name := want["name"].(string)
				t.Run(object+"/"+name, func(t *testing.T) {
					for _, rule := range org.Objects[object].Definition.DuplicateRules {
						if rule.Name != name {
							continue
						}
						matches := []any{}
						for _, match := range rule.MatchRules {
							matches = append(matches, map[string]any{"object": match.Object, "name": match.Name})
						}
						insertOps := []any{}
						for _, op := range rule.OperationsOnInsert {
							insertOps = append(insertOps, op)
						}
						updateOps := []any{}
						for _, op := range rule.OperationsOnUpdate {
							updateOps = append(updateOps, op)
						}
						got := map[string]any{"object": object, "name": name, "active": rule.Active, "insert": rule.ActionOnInsert, "update": rule.ActionOnUpdate, "alert": rule.AlertText, "insertOperations": insertOps, "updateOperations": updateOps, "security": rule.SecurityOption, "order": float64(rule.SortOrder), "matches": matches}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%v; Glade=%v", want, got)
						}
						return
					}
					t.Fatalf("native default duplicate rule exists without project source; Glade has no definition for %s.%s", object, name)
				})
			}
		})
	}
}

func TestDuplicateReportCommittedAnonymousNativeConformance(t *testing.T) {
	data, rows := readDuplicateNativeData(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, duplicateProject(t, data, "standard", api))
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			want := rows[api+"/committed_anonymous/CM01"]
			for _, stage := range []string{"write", "readback"} {
				t.Run(stage, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(data.Committed[stage], vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := runner.newMachine()
					machine.SetOrg(&org)
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					org = *machine.Org
					var payload string
					for _, message := range result.Debug {
						if strings.Contains(message, "COMMITTED_OBS|") {
							payload = message
						}
					}
					expected, ok := want[stage].(map[string]any)
					if !ok {
						t.Fatalf("missing native committed %s", stage)
					}
					assertDuplicateNativePayload(t, "COMMITTED_OBS|", payload, expected)
				})
			}
		})
	}
}
