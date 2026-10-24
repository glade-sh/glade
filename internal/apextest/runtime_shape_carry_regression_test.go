package apextest

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeShapeCarryFieldComparison(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   map[string]any
		got    map[string]any
		fields []string
	}{
		{"taskReference", map[string]any{"valid": true, "missing": false, "missingStatus": "INVALID_CROSS_REFERENCE_KEY"}, map[string]any{"valid": true, "missing": false, "missingStatus": "FIELD_INTEGRITY_EXCEPTION"}, []string{"missingStatus"}},
		{"accessIdentity_anonymous", map[string]any{"cloneEqual": false, "cloneSame": false, "json": "System.JSONException"}, map[string]any{"cloneEqual": true, "cloneSame": true, "json": "System.JSONException"}, []string{"cloneEqual", "cloneSame"}},
		{"accessIdentity_isTest_namedVM", map[string]any{"cloneEqual": false, "cloneSame": false, "queueSame": false, "json": "System.JSONException"}, map[string]any{"cloneEqual": true, "cloneSame": true, "queueSame": true, "json": "System.JSONException"}, []string{"cloneEqual", "cloneSame", "queueSame"}},
		{"activityFailure", map[string]any{"errors": 1, "sent": false, "tasks": 0}, map[string]any{"errors": 0, "sent": true, "tasks": 0}, []string{"errors", "sent"}},
		{"activitySuccess", map[string]any{"errors": 0, "sent": true, "tasks": 1}, map[string]any{"errors": 0, "sent": true, "tasks": 0}, []string{"tasks"}},
		{"asyncStamp_isTest_namedVM", map[string]any{"count": 1, "rootStamp": true, "terminal": false, "drainError": "System.AsyncException"}, map[string]any{"count": 1, "rootStamp": true, "terminal": false, "drainError": nil}, []string{"drainError"}},
		{"hierarchyDefault_67_isTest_namedVM", map[string]any{"errorType": "System.QueryException"}, map[string]any{"orgRows": 0, "disabled": true}, []string{"errorType", "orgRows", "disabled"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			carry := &runtimeShapeCarry{Fields: tc.fields}
			if !runtimeShapeObservationsEqual(tc.got, tc.want, carry) {
				t.Fatal("explicitly carried differences rejected")
			}
			for field, value := range tc.got {
				if slicesContainRuntimeShapeField(tc.fields, field) {
					continue
				}
				t.Run(field, func(t *testing.T) {
					mutated := cloneRuntimeShapeObservation(tc.got)
					mutated[field] = "unexpected regression"
					if runtimeShapeObservationsEqual(mutated, tc.want, carry) {
						t.Errorf("carry bypassed matching field %s=%v", field, value)
					}
					delete(mutated, field)
					if runtimeShapeObservationsEqual(mutated, tc.want, carry) {
						t.Errorf("carry bypassed missing matching field %s", field)
					}
				})
			}
			mutated := cloneRuntimeShapeObservation(tc.got)
			mutated["unexpected"] = true
			if runtimeShapeObservationsEqual(mutated, tc.want, carry) {
				t.Error("carry bypassed unexpected field")
			}
			before := cloneRuntimeShapeObservation(tc.got)
			runtimeShapeObservationsEqual(tc.got, tc.want, carry)
			if !reflect.DeepEqual(tc.got, before) {
				t.Error("comparison mutated observation")
			}
		})
	}
}

func slicesContainRuntimeShapeField(fields []string, field string) bool {
	for _, candidate := range fields {
		if candidate == field {
			return true
		}
	}
	return false
}

func cloneRuntimeShapeObservation(value map[string]any) map[string]any {
	copy := make(map[string]any, len(value))
	for field, item := range value {
		copy[field] = item
	}
	return copy
}

func TestRuntimeShapeCarryRejectsInvalidFieldScope(t *testing.T) {
	got := map[string]any{"valid": true, "missingStatus": "FIELD_INTEGRITY_EXCEPTION"}
	want := map[string]any{"valid": true, "missingStatus": "INVALID_CROSS_REFERENCE_KEY"}
	for _, fields := range [][]string{
		nil,
		{"missingStatus", ""},
		{"missingStatus", "missingStatus"},
		{"missingStatus", "typo"},
		{"missingStatus", "valid"},
	} {
		t.Run(fmt.Sprint(fields), func(t *testing.T) {
			if runtimeShapeObservationsEqual(got, want, &runtimeShapeCarry{Fields: fields}) {
				t.Fatalf("invalid or already matching carry field accepted: %v", fields)
			}
		})
	}
	if runtimeShapeObservationsEqual(got, want, nil) {
		t.Fatal("uncarried difference accepted")
	}
	if !runtimeShapeObservationsEqual(want, want, nil) {
		t.Fatal("uncarried matching observation rejected")
	}
}

// The child runs the real synthetic project through every route, then corrupts
// matching fields. Each corruption must fail despite its sibling field carry.
func TestRuntimeShapeCarryProjectRouteTwins(t *testing.T) {
	const childFlag = "GLADE_TEST_RUNTIME_SHAPE_CARRY_CHILD"
	if os.Getenv(childFlag) == "1" {
		runRuntimeShapeSlicesNativeConformance(t, func(method string, got map[string]any) {
			switch method {
			case "taskReference":
				got["valid"] = false
			case "accessIdentity":
				got["json"] = "unexpected regression"
			case "activityFailure":
				got["tasks"] = float64(99)
			case "activitySuccess":
				got["sent"] = false
			case "asyncStamp":
				if _, exists := got["rootStamp"]; exists {
					got["rootStamp"] = false
				}
			}
		})
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestRuntimeShapeCarryProjectRouteTwins$", "-test.v", "-test.timeout=150s")
	cmd.Env = append(os.Environ(), childFlag+"=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("matching-field mutations passed every synthetic project route")
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest", "namedVM"} {
			for _, method := range []string{"taskReference", "accessIdentity", "activityFailure", "activitySuccess", "asyncStamp"} {
				if method == "asyncStamp" && route == "anonymous" {
					continue // Anonymous native row is an explicit test-drain boundary.
				}
				name := fmt.Sprintf("%s/%s/%s/%s", t.Name(), api, route, method)
				t.Run(api+"/"+route+"/"+method, func(t *testing.T) {
					if !strings.Contains(string(output), "--- FAIL: "+name+" (") {
						t.Errorf("matching-field mutation did not fail on %s; child output:\n%s", name, output)
					}
				})
			}
		}
	}
}
