package apextest

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Replay the exact owned named-test sources. Only allocated record identities
// are rebound; widths, literal controls, getter bytes and job messages remain
// independently compared with the captured native values.
func TestControllerIDTextObservationsConformance(t *testing.T) {
	var data struct {
		Objects map[string]string `json:"objects"`
		Rows    []struct {
			ID           string            `json:"id"`
			Shape        string            `json:"shape"`
			SourceAPI    string            `json:"sourceApi"`
			TransportAPI string            `json:"transportApi"`
			Route        string            `json:"route"`
			Classes      map[string]string `json:"classes"`
			Native       []any             `json:"native"`
		} `json:"rows"`
		Uncaptured []struct{ Scope, Reason string } `json:"uncaptured"`
	}
	raw, err := os.ReadFile("testdata/conformance/controller_id_text_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 2 || len(data.Uncaptured) != 2 {
		t.Fatal("incomplete controller text evidence")
	}
	for _, carry := range data.Uncaptured {
		if carry.Scope == "" || carry.Reason == "" {
			t.Fatal("uncaptured scope requires a reason")
		}
		t.Logf("UNCAPTURED %s: %s", carry.Scope, carry.Reason)
	}
	for _, row := range data.Rows {
		t.Run(row.ID, func(t *testing.T) {
			if row.SourceAPI != "66.0" || row.TransportAPI != "66.0" || row.Route != "istest" {
				t.Fatal("unexpected native scope")
			}
			index := nativeBoundaryIndex(t, row.SourceAPI, row.Classes, data.Objects)
			run := Run(index, Options{SelectedClasses: []string{"ControllerJobTextCapture"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("observation selection: %#v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || result.Problem.Type != "System.AssertException" {
				t.Fatalf("missing observation: %#v", result)
			}
			_, payload, ok := strings.Cut(result.Problem.Message, "Actual: ")
			if !ok || !strings.Contains(result.Problem.Message, "NATIVE_OBSERVATION") {
				t.Fatalf("missing observation frame: %s", result.Problem.Message)
			}
			var actual []any
			if err := json.Unmarshal([]byte(payload), &actual); err != nil {
				t.Fatalf("observation JSON: %v: %s", err, payload)
			}
			fullID := func(value []any) string {
				t.Helper()
				if row.Shape == "end-to-end" {
					if len(value) != 8 {
						t.Fatalf("partial end-to-end observation: %v", value)
					}
					text, _ := value[0].(string)
					return text
				}
				if row.Shape != "constructors" || len(value) != 2 {
					t.Fatalf("partial constructor observation: %v", value)
				}
				supplied, ok := value[1].([]any)
				if !ok || len(supplied) != 5 {
					t.Fatalf("partial supplied identity: %v", value)
				}
				text, _ := supplied[0].(string)
				return text
			}
			localID, nativeID := fullID(actual), fullID(row.Native)
			if len(localID) != 18 || len(nativeID) != 18 {
				t.Fatalf("field Id width: local=%q native=%q", localID, nativeID)
			}
			replacements := strings.NewReplacer(
				hex.EncodeToString([]byte(localID)), hex.EncodeToString([]byte(nativeID)),
				hex.EncodeToString([]byte(localID[:15])), hex.EncodeToString([]byte(nativeID[:15])),
				localID, nativeID, localID[:15], nativeID[:15],
			)
			var rebind func(any) any
			rebind = func(value any) any {
				switch value := value.(type) {
				case string:
					return replacements.Replace(value)
				case []any:
					out := make([]any, len(value))
					for i, item := range value {
						out[i] = rebind(item)
					}
					return out
				default:
					return value
				}
			}
			normalized := rebind(actual)
			if !reflect.DeepEqual(row.Native, normalized) {
				t.Fatalf("native=%v; local=%v", row.Native, normalized)
			}
		})
	}
}
