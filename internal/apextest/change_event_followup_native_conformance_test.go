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

const changeEventFollowupFixture = changeEventDescribeFixture + "/followups"

type changeEventFollowupCase struct {
	API           string
	Configuration string
	Route         string
	ID            string
	Observation   json.RawMessage
}

func readChangeEventFollowups(t *testing.T) []changeEventFollowupCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(changeEventFollowupFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Rows    int               `json:"rows"`
		Sources map[string]string `json:"sources"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Rows == 0 || len(manifest.Sources) == 0 {
		t.Fatal("missing native case manifest")
	}
	for path, digest := range manifest.Sources {
		body, err := os.ReadFile(filepath.Join(changeEventFollowupFixture, path))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(body)) != digest {
			t.Errorf("native source changed: %s", path)
		}
	}
	raw, err = os.ReadFile(filepath.Join(changeEventFollowupFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "api\tconfiguration\troute\tid\tobservation" || len(lines) != manifest.Rows+1 {
		t.Fatal("incomplete native observations")
	}
	seen := map[string]bool{}
	var rows []changeEventFollowupCase
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 || !json.Valid([]byte(cells[4])) {
			t.Fatal("invalid native observation")
		}
		key := strings.Join(cells[:4], "/")
		if seen[key] {
			t.Fatalf("duplicate native observation: %s", key)
		}
		seen[key] = true
		rows = append(rows, changeEventFollowupCase{cells[0], cells[1], cells[2], cells[3], json.RawMessage(cells[4])})
	}
	return rows
}

func changeEventFollowupNative(t *testing.T, api, configuration, route string) map[string]json.RawMessage {
	t.Helper()
	rows := map[string]json.RawMessage{}
	for _, row := range readChangeEventFollowups(t) {
		if row.API == api && row.Configuration == configuration && row.Route == route {
			rows[row.ID] = row.Observation
		}
	}
	if len(rows) != 8 {
		t.Fatalf("expected eight native observations for %s/%s/%s; found %d", api, configuration, route, len(rows))
	}
	return rows
}

func changeEventFollowupInstalledChildren(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var children []string
	if err := json.Unmarshal(raw, &children); err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, child := range children {
		object, _, _ := strings.Cut(child, ".")
		switch object {
		case "AccountChangeEvent", "ContactChangeEvent", "CaseChangeEvent", "Parcel__ChangeEvent":
			out = append(out, child)
		}
	}
	return out
}

func assertChangeEventFollowupShapesAndUserChildren(t *testing.T, got, native map[string]json.RawMessage) {
	t.Helper()
	for _, id := range []string{"shape.FirstName", "shape.LastName", "shape.Salutation"} {
		var actual, want map[string]any
		if json.Unmarshal(got[id], &actual) != nil || json.Unmarshal(native[id], &want) != nil {
			t.Fatalf("missing shape observation: %s", id)
		}
		if !reflect.DeepEqual(actual, want) {
			t.Errorf("%s: Glade=%s native=%s", id, got[id], native[id])
		}
	}
	actual := changeEventFollowupInstalledChildren(t, got["children.User"])
	want := changeEventFollowupInstalledChildren(t, native["children.User"])
	if len(want) == 0 {
		t.Fatal("native User relationships lack installed standard events")
	}
	for _, child := range want {
		if strings.HasPrefix(child, "Parcel__ChangeEvent.") || !strings.HasSuffix(child, ":null") {
			t.Fatalf("unexpected native User relationship: %s", child)
		}
	}
	changeEventDescribeDiff(t, "User children of installed events", changeEventDescribeSet(actual), changeEventDescribeSet(want))
	var event, base, actualEvent map[string]bool
	if json.Unmarshal(native["features.AccountChangeEvent"], &event) != nil || json.Unmarshal(native["features.Account"], &base) != nil || json.Unmarshal(got["features.AccountChangeEvent"], &actualEvent) != nil {
		t.Fatal("missing native feature observations")
	}
	// Compare fields enabled on the native base. The captured disabled states
	// remain controls; this test does not emulate org feature provisioning.
	for field, present := range event {
		if (base[field] || field == "IsCustomerPortal") && actualEvent[field] != present {
			t.Errorf("AccountChangeEvent.%s present=%v; native=%v", field, actualEvent[field], present)
		}
	}
}

// These are capture checks, not assertions of Glade's FLS implementation.
// In particular, field-map membership and field accessibility are distinct.
func TestChangeEventFollowupNativeRowsAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, route := range []string{"administrator.anonymous", "administrator.istest", "no-field-access.anonymous", "no-field-access.istest"} {
				rows := changeEventFollowupNative(t, api, "division-on", route)
				for _, object := range []string{"Parcel__c", "Parcel__ChangeEvent"} {
					var view struct{ Fields, Readable []string }
					if err := json.Unmarshal(rows["fls."+object], &view); err != nil {
						t.Fatal(err)
					}
					if len(view.Fields) != 15 {
						t.Fatalf("%s/%s: native map has %d fields", route, object, len(view.Fields))
					}
					if strings.HasPrefix(route, "no-field-access.") && len(view.Readable) != 0 {
						t.Fatalf("ungranted object has accessible fields: %s/%s", route, object)
					}
					if strings.HasPrefix(route, "administrator.") && !reflect.DeepEqual(view.Fields, view.Readable) {
						t.Fatalf("permission-set control lacks access: %s/%s", route, object)
					}
				}
				anonymous := changeEventFollowupNative(t, api, "division-on", strings.ReplaceAll(route, ".istest", ".anonymous"))
				assertChangeEventFollowupShapesAndUserChildren(t, rows, anonymous)
				enabled := changeEventFollowupNative(t, api, "features-on", route)
				assertChangeEventFollowupShapesAndUserChildren(t, enabled, changeEventFollowupNative(t, api, "features-on", strings.ReplaceAll(route, ".istest", ".anonymous")))
				for _, id := range []string{"fls.Parcel__c", "fls.Parcel__ChangeEvent"} {
					if !reflect.DeepEqual(rows[id], enabled[id]) {
						t.Fatalf("independent repeat changed %s/%s", route, id)
					}
				}
			}
		})
	}
}

// A feature-enabled native base/event pair supplies exclusions that cannot be
// observed in the earlier feature-disabled field maps.
func applyChangeEventFollowupNativeExclusions(t *testing.T, api string, fields map[string]bool) {
	t.Helper()
	native := changeEventFollowupNative(t, api, "features-on", "administrator.anonymous")
	var base, event map[string]bool
	if json.Unmarshal(native["features.Account"], &base) != nil || json.Unmarshal(native["features.AccountChangeEvent"], &event) != nil {
		t.Fatal("missing native feature pair")
	}
	for field, present := range base {
		if present && !event[field] {
			delete(fields, strings.ToLower(field))
		}
	}
}

func copyChangeEventFollowupProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	for _, source := range []string{filepath.Join(changeEventDescribeFixture, "project"), filepath.Join(changeEventFollowupFixture, "project")} {
		if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(root, name), strings.ReplaceAll(strings.ReplaceAll(string(body), "68.0", api), "62.0", api))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestChangeEventFollowupShapesAndUserChildrenNativeConformanceAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, copyChangeEventFollowupProject(t, api))
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			for _, route := range []string{"anonymous", "istest", "project_istest"} {
				t.Run(route, func(t *testing.T) {
					nativeRoute := "administrator." + route
					if route == "project_istest" {
						nativeRoute = "administrator.istest"
					}
					native := changeEventFollowupNative(t, api, "division-on", nativeRoute)
					got := map[string]json.RawMessage{}
					if route == "anonymous" {
						body, err := os.ReadFile(filepath.Join(changeEventFollowupFixture, "anonymous", "describe.apex"))
						if err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.newMachine().Execute(program)
						if err != nil {
							t.Fatal(err)
						}
						for _, line := range result.Debug {
							if payload, ok := strings.CutPrefix(line, "FOLLOWUP "); ok {
								id, value, ok := strings.Cut(payload, "\t")
								if !ok || !json.Valid([]byte(value)) {
									t.Fatal("invalid describe observation")
								}
								got[id] = json.RawMessage(value)
							}
						}
					} else {
						var message string
						if route == "project_istest" {
							run := Run(index, Options{SelectedClasses: []string{"ChangeEventFollowupTest"}, SelectedMethod: "administratorDescribe", NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
								t.Fatalf("missing project observation: %+v", run.Summary())
							}
							message = run.Suites[0].Cases[0].Problem.Message
						} else {
							selected := Discover(index, Options{SelectedClasses: []string{"ChangeEventFollowupTest"}, SelectedMethod: "administratorDescribe"})
							if len(selected) != 1 {
								t.Fatalf("expected one named test, found %d", len(selected))
							}
							programs, compileErrors := compileTestInvokePrograms(selected)
							methods, methodErrors := compileTestMethodsForIndex(&index, selected)
							key := testCaseKey(selected[0])
							if compileErrors[key] != nil || methodErrors[key] != nil {
								t.Fatalf("named compile: %v %v", compileErrors[key], methodErrors[key])
							}
							machine := runner.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, err := machine.ExecuteTestInClass(programs[key], selected[0].ClassName)
							var fault *vm.RuntimeError
							if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
								t.Fatalf("missing named observation: %v", err)
							}
							message = fault.ExceptionMessage()
						}
						_, payload, ok := strings.Cut(message, "OBSERVED|")
						if !ok || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid named observation: %s", message)
						}
					}
					assertChangeEventFollowupShapesAndUserChildren(t, got, native)
				})
			}
		})
	}
}
