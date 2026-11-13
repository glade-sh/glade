package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE fuzzy DML and Find member sets testdata/conformance/duplicate_save_behavior/org.tsv:2 anonymous 62.0
// NATIVE changed upsert and unrelated edit testdata/conformance/duplicate_save_behavior/org.tsv:69 istest 67.0
// Replay the same Apex expressions as the native anonymous and named test runs.
// Match members are labelled by their seed IDs inside Apex, independently of
// platform-assigned IDs. A failed assertion transports the complete observation.
func TestDuplicateSaveBehaviorNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/duplicate_save_behavior"
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "api\troute\tgroup\tmethod\tobservation" {
		t.Fatal("invalid native row header")
	}
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 || groups[fields[2]][fields[3]] == "" {
			t.Fatalf("invalid row: %q", line)
		}
		key := strings.Join(fields[:4], "/")
		if rows[key] != "" || fields[4] == "" {
			t.Fatalf("invalid observation %s", key)
		}
		rows[key] = fields[4]
	}
	caseCount := 0
	for _, cases := range groups {
		caseCount += len(cases)
	}
	if caseCount == 0 || len(rows) != 4*caseCount {
		t.Fatalf("native matrix has %d observations for %d cases", len(rows), caseCount)
	}
	for _, group := range slices.Sorted(maps.Keys(groups)) {
		cases := groups[group]
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(group+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				helpers := []string{"StandardBehavior"}
				copyMeta := func(name string) {
					if err := os.CopyFS(filepath.Join(base, name), os.DirFS(filepath.Join(dir, name))); err != nil {
						t.Fatal(err)
					}
				}
				if group == "person" || group == "custom" || group == "personActive" || group == "personInactive" || group == "order" {
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
				}
				switch group {
				case "person":
					helpers = append(helpers, "PersonBehavior")
				case "custom", "personActive", "personInactive", "order":
					copyMeta("matchingRules")
					copyMeta("duplicateRules")
					if group == "personInactive" || group == "order" {
						path := filepath.Join(base, "duplicateRules", "PersonAccount.ProbePersonAlert.duplicateRule-meta.xml")
						body, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, path, strings.Replace(string(body), "<isActive>true</isActive>", "<isActive>false</isActive>", 1))
					}
					switch group {
					case "custom":
						helpers = append(helpers, "CustomDuplicateProbe")
					case "order":
						helpers = append(helpers, "OrderBehavior")
					default:
						helpers = append(helpers, "PersonLookupBehavior")
					}
				case "extra":
					helpers = append(helpers, "ExtraBehavior")
					copyMeta("objects")
					copyMeta("permissionsets")
					writeCapturedAssignments(t, root, "ProbeKeyAccess")
				case "limit":
					helpers = append(helpers, "LimitBehavior")
				case "bound":
					helpers = append(helpers, "BoundBehavior", "NonRuleBehavior")
				case "keyboard":
					helpers = append(helpers, "KeyboardBehavior")
				case "prefix":
					helpers = append(helpers, "PrefixBehavior")
				case "holdout":
					helpers = append(helpers, "HoldoutBehavior")
				case "keyboardBoundary":
					helpers = append(helpers, "BoundaryBehavior")
				case "leadNames":
					helpers = append(helpers, "LeadNamesBehavior")
				case "phoneCriteria":
					for _, name := range []string{"matchingRules", "duplicateRules"} {
						if err := os.CopyFS(filepath.Join(base, name), os.DirFS(filepath.Join(dir, "criteria", name))); err != nil {
							t.Fatal(err)
						}
					}
					helpers = append(helpers, "CriteriaBehavior")
				case "gradeHoldout":
					helpers = append(helpers, "GradeBehavior")
				case "localEdits":
					helpers = append(helpers, "LocalEditsBehavior")
				case "calibration":
					helpers = append(helpers, "CalibrationBehavior")
				case "names":
					helpers = append(helpers, "NamePanelBehavior")
				case "business":
				default:
					t.Fatalf("unknown fixture %s", group)
				}

				const class = "ObservationTest"
				var testClass strings.Builder
				fmt.Fprintf(&testClass, "@IsTest private class %s {\n", class)
				expressions := map[string]string{}
				for _, method := range slices.Sorted(maps.Keys(cases)) {
					call := cases[method]
					source := "String observation = " + call + "; System.assert(false, 'OBS|" + method + "|' + Test.isRunningTest() + '|' + observation.length() + '|' + observation + '|ENDOBS');"
					expressions[method] = source
					fmt.Fprintf(&testClass, "@IsTest static void %s() { %s }\n", method, source)
				}
				testClass.WriteString("}")
				for _, name := range append(helpers, class) {
					source := testClass.String()
					if name != class {
						data, err := os.ReadFile(filepath.Join(dir, name+".cls"))
						if err != nil {
							t.Fatal(err)
						}
						source = string(data)
					}
					writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
					writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				index := loadTestIndex(t, root)
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				check := func(t *testing.T, route, method, message string) {
					t.Helper()
					want := rows[strings.Join([]string{api, route, group, method}, "/")]
					if want == "" {
						t.Fatal("missing native row")
					}
					prefix := fmt.Sprintf("OBS|%s|%t|", method, route == "istest")
					_, frame, ok := strings.Cut(message, prefix)
					size, payload, sized := strings.Cut(frame, "|")
					got, ended := strings.CutSuffix(payload, "|ENDOBS")
					if !ok || !sized || !ended || size != strconv.Itoa(len(got)) {
						t.Fatalf("invalid observation frame: %s", message)
					}
					if group == "gradeHoldout" {
						compareShortEditHoldout(t, want, got)
					} else if got != want {
						t.Errorf("native=%s\nGlade=%s", want, got)
					}
				}
				for _, method := range slices.Sorted(maps.Keys(expressions)) {
					source := expressions[method]
					t.Run("anonymous/"+method, func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("missing Apex observation: %v", err)
						}
						check(t, "anonymous", method, fault.ExceptionMessage())
					})
				}
				options := Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1}
				selected := Discover(index, options)
				if len(selected) != len(cases) {
					t.Fatalf("discovered %d of %d methods", len(selected), len(cases))
				}
				report := RunCasesContext(context.Background(), index, options, selected)
				count := 0
				for _, suite := range report.Suites {
					for _, result := range suite.Cases {
						count++
						t.Run("istest/"+result.MethodName, func(t *testing.T) {
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing Apex observation: %+v", result.Problem)
							}
							check(t, "istest", result.MethodName, result.Problem.Message)
						})
					}
				}
				if count != len(cases) {
					t.Fatalf("ran %d of %d methods", count, len(cases))
				}
			})
		}
	}
}

// These prospective panels record the ID and confidence separately in Find and
// in the DML error. Preserve the exact seed ID, multiplicity and save verdict
// for every input. Both accessors must return the same grade. Four inputs in
// each direction differ from the native grade, by at most two points.
var shortEditGrades = regexp.MustCompile(`=([0-9]+)`)

func compareShortEditHoldout(t *testing.T, want, got string) {
	t.Helper()
	native, actual := strings.Split(want, ","), strings.Split(got, ",")
	if len(native) != len(actual) {
		t.Fatalf("native %d inputs, Glade %d", len(native), len(actual))
	}
	// Only these four seed positions in each directional panel had a score residual.
	// Every other seed, and both accessors on every seed, must agree exactly.
	gradeBounds := map[int]int{1: 1, 10: 1, 13: 2, 15: 1}
	for i, expected := range native {
		result := actual[i]
		if expected == result {
			continue
		}
		if shortEditGrades.ReplaceAllString(result, "=#") != shortEditGrades.ReplaceAllString(expected, "=#") {
			t.Errorf("input %d: native %s, Glade %s", i, expected, result)
			continue
		}
		a, b := shortEditGrades.FindAllStringSubmatch(result, -1), shortEditGrades.FindAllStringSubmatch(expected, -1)
		if len(a) != 2 || len(b) != 2 || a[0][1] != a[1][1] || b[0][1] != b[1][1] {
			t.Errorf("input %d: inconsistent grades: native %s, Glade %s", i, expected, result)
			continue
		}
		x, ex := strconv.Atoi(a[0][1])
		y, ey := strconv.Atoi(b[0][1])
		bound := gradeBounds[i]
		if ex != nil || ey != nil || x > 100 || x-y > bound || y-x > bound {
			t.Errorf("input %d: native grade %s, Glade %s", i, b[0][1], a[0][1])
		}
	}
}
