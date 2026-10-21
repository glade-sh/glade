package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These cells retain native answers for behavior outside the implemented
// candidate. Checking their completeness records carries, not execution parity.
func TestResidualBoundaryNativeEvidenceCarries(t *testing.T) {
	var data struct {
		APIVersions               []string `json:"apiVersions"`
		Routes                    []string `json:"routes"`
		ShapeCount                int      `json:"shapeCount"`
		NativeObservationCount    int      `json:"nativeObservationCount"`
		CarriedCount              int      `json:"carriedCount"`
		StillPendingCount         int      `json:"stillPendingCount"`
		ConstructorCarryCount     int      `json:"constructorCarryCount"`
		RuntimeBoundaryCarryCount int      `json:"runtimeBoundaryCarryCount"`
		Rows                      []struct {
			ID          string `json:"id"`
			Family      string `json:"family"`
			Question    string `json:"question"`
			CarryReason string `json:"carryReason"`
			Routes      map[string]struct {
				MeasuredApex  string            `json:"measuredApex"`
				Helpers       map[string]string `json:"helpers"`
				Anonymous     string            `json:"anonymous"`
				Classes       map[string]string `json:"classes"`
				Stage         string            `json:"stage"`
				ExpectedByAPI map[string]struct {
					Outcome     string          `json:"outcome"`
					Value       json.RawMessage `json:"value"`
					Diagnostics json.RawMessage `json:"diagnostics"`
				} `json:"expectedByAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", "residual_boundary_carries.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIVersions, []string{"62.0", "67.0", "68.0"}) ||
		!reflect.DeepEqual(data.Routes, []string{"anonymous", "isTest"}) ||
		data.ShapeCount != 48 || len(data.Rows) != 48 || data.NativeObservationCount != 288 ||
		data.CarriedCount != 288 || data.StillPendingCount != 0 ||
		data.ConstructorCarryCount != 24 || data.RuntimeBoundaryCarryCount != 264 {
		t.Fatal("incomplete native boundary carry matrix")
	}
	wantIDs := map[string]bool{"C01": true, "C02": true, "C09": true, "C01D": true, "E-O01": true, "E-O02": true, "E-E01": true}
	for i := 1; i <= 13; i++ {
		wantIDs[fmt.Sprintf("E-C%02d", i)] = true
	}
	for i := 1; i <= 10; i++ {
		wantIDs[fmt.Sprintf("E-T%02d", i)] = true
		wantIDs[fmt.Sprintf("E-T%02dL", i)] = true
	}
	for i := 1; i <= 6; i++ {
		wantIDs[fmt.Sprintf("E-F01-%d", i)] = true
	}
	for i := 1; i <= 2; i++ {
		wantIDs[fmt.Sprintf("E-F02-%d", i)] = true
	}
	for _, row := range data.Rows {
		if !wantIDs[row.ID] || len(row.Routes) != 2 || row.Family == "" || row.Question == "" || row.CarryReason == "" {
			t.Fatalf("invalid native carry row: %s", row.ID)
		}
		delete(wantIDs, row.ID)
	}
	if len(wantIDs) != 0 {
		t.Fatalf("missing carry shapes: %v", wantIDs)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "BoundaryEvidence", api)
			for _, row := range data.Rows {
				for _, route := range data.Routes {
					t.Run(row.ID+"/"+route, func(t *testing.T) {
						counts.TrackKind(t, route, "carried", 1)
						source, ok := row.Routes[route]
						if !ok || source.Stage != "carried" || strings.TrimSpace(source.MeasuredApex) == "" || len(source.ExpectedByAPI) != 3 {
							t.Fatal("missing measured source, carry reason or native API cell")
						}
						if route == "anonymous" && strings.TrimSpace(source.Anonymous) == "" {
							t.Fatal("missing anonymous measurement")
						}
						if route == "isTest" && len(source.Classes) == 0 {
							t.Fatal("missing named measurement")
						}
						want, present := source.ExpectedByAPI[api]
						if !present {
							t.Fatal("native API cell absent")
						}
						if row.ID == "C01D" {
							if want.Outcome != "compile-rejected" || len(want.Diagnostics) == 0 || len(want.Value) != 0 {
								t.Fatal("default-access constructor needs its native compiler rejection")
							}
							residualBoundaryCheckConstructorDiagnostic(t, route, want.Diagnostics)
							t.Logf("carried: %s; native compiler diagnostic=%s", row.CarryReason, want.Diagnostics)
							return
						}
						var value map[string]any
						if want.Outcome != "runtime" || len(want.Diagnostics) != 0 || json.Unmarshal(want.Value, &value) != nil || len(value) == 0 {
							t.Fatal("missing native runtime observation")
						}
						if _, ok := value["exceptionType"]; !ok {
							t.Fatal("native exception type field absent")
						}
						if _, ok := value["exceptionMessage"]; !ok {
							t.Fatal("native exception message field absent")
						}
						if row.ID == "E-F01-6" {
							if api == "62.0" {
								if value["value"] != "{}" || value["exceptionType"] != nil || value["exceptionMessage"] != nil {
									t.Fatal("API 62 exception JSON observation changed")
								}
							} else if value["exceptionType"] != "System.JSONException" || value["exceptionMessage"] != "Type unsupported in JSON: IllegalArgumentException" {
								t.Fatal("API 67/68 exception JSON rejection changed")
							}
						}
						t.Logf("carried: %s; native value=%s", row.CarryReason, want.Value)
					})
				}
			}
		})
	}
}

func residualBoundaryCheckConstructorDiagnostic(t *testing.T, route string, raw json.RawMessage) {
	t.Helper()
	const problem = "Method is not visible: void CourierDefault_SObjectUnitOfWork.<init>()"
	if route == "anonymous" {
		var diagnostic struct {
			Problem string `json:"compileProblem"`
			Line    int    `json:"line"`
			Column  int    `json:"column"`
		}
		if json.Unmarshal(raw, &diagnostic) != nil || diagnostic.Problem != problem || diagnostic.Line != 1 || diagnostic.Column != 142 {
			t.Fatalf("anonymous constructor diagnostic changed: %s", raw)
		}
		return
	}
	var diagnostics []struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Problem string `json:"problem"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
	}
	if json.Unmarshal(raw, &diagnostics) != nil || len(diagnostics) != 1 ||
		diagnostics[0].Name != "BoundaryC01DTest" || diagnostics[0].Type != "ApexClass" ||
		diagnostics[0].Problem != problem || diagnostics[0].Line != 1 || diagnostics[0].Column != 240 {
		t.Fatalf("named constructor diagnostic changed: %s", raw)
	}
}
