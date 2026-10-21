package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type residualQueryRoute struct {
	nativeBoundaryRoute
	ExpectedByAPI map[string]nativeObservationExpected `json:"expectedByAPI"`
	CarryReason   string                               `json:"carryReason"`
	MeasuredApex  string                               `json:"measuredApex"`
}

// Administrator controls are executable observations. Restricted wrapper
// compilation failures retain their diagnostics and never assert a query answer.
func TestResidualQueryOrgConformance(t *testing.T) {
	var data struct {
		APIVersions             []string `json:"apiVersions"`
		ShapeCount              int      `json:"shapeCount"`
		RuntimeObservationCount int      `json:"runtimeObservationCount"`
		CarriedObservationCount int      `json:"carriedObservationCount"`
		Rows                    []struct {
			ID     string                        `json:"id"`
			Routes map[string]residualQueryRoute `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", "residual_query.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 21 || len(data.Rows) != 21 ||
		data.RuntimeObservationCount != 66 || data.CarriedObservationCount != 30 {
		t.Fatal("incomplete query observation and carry matrix")
	}
	seen := map[string]bool{}
	exact, carried := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] {
			t.Fatalf("invalid or repeated query row: %s", row.ID)
		}
		seen[row.ID] = true
		for route, source := range row.Routes {
			if source.ID != row.ID || len(source.ExpectedByAPI) != 3 {
				t.Fatalf("incomplete query route: %s/%s", row.ID, route)
			}
			if source.Stage == "carried" {
				if !strings.HasSuffix(row.ID, "R") || route != "isTest" || len(row.Routes) != 1 ||
					!strings.Contains(source.CarryReason, "before any restricted query executed") ||
					!strings.Contains(source.MeasuredApex, "System.runAs(actor)") {
					t.Fatalf("invalid restricted wrapper carry: %s/%s", row.ID, route)
				}
			} else if source.Stage != "observed" || len(row.Routes) != 2 ||
				(route != "anonymous" && route != "isTest") || (route == "anonymous" && source.Anonymous == "") ||
				(route == "isTest" && len(source.Tests) != 1) {
				t.Fatalf("invalid executable query source: %s/%s", row.ID, route)
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || len(want.Value) == 0 {
					t.Fatalf("missing native query value: %s/%s/%s", row.ID, route, api)
				}
				if source.Stage == "carried" {
					carried++
					validateResidualQueryWrapperCarry(t, want)
				} else {
					exact++
					if want.Outcome != "runtime" {
						t.Fatalf("unexpected executable query outcome: %s", want.Outcome)
					}
				}
			}
		}
	}
	if exact != data.RuntimeObservationCount || carried != data.CarriedObservationCount {
		t.Fatalf("query observations=%d carries=%d", exact, carried)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ResidualQuery", api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for route, source := range row.Routes {
						t.Run(route, func(t *testing.T) {
							want := source.ExpectedByAPI[api]
							if source.Stage == "carried" {
								counts.TrackKind(t, route, "carried", 1)
								validateResidualQueryWrapperCarry(t, want)
								return
							}
							counts.TrackKind(t, route, "exact", 1)
							residualObservationObserve(t, api, route, source.nativeBoundaryRoute, want)
						})
					}
				})
			}
		})
	}
}

func validateResidualQueryWrapperCarry(t *testing.T, want nativeObservationExpected) {
	t.Helper()
	if want.Outcome != "compile-rejected" {
		t.Fatalf("restricted wrapper outcome is not a query observation: %s", want.Outcome)
	}
	var value struct {
		Diagnostics []struct {
			Type    string `json:"type"`
			Line    int    `json:"line"`
			Column  int    `json:"column"`
			Problem string `json:"problem"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(want.Value, &value); err != nil || len(value.Diagnostics) == 0 {
		t.Fatal("missing native wrapper diagnostic")
	}
	for _, diagnostic := range value.Diagnostics {
		if diagnostic.Type != "ApexClass" || diagnostic.Line != 1 || diagnostic.Column <= 32767 ||
			diagnostic.Problem != fmt.Sprintf("Column too large: %d", diagnostic.Column) {
			t.Fatalf("unexpected restricted wrapper diagnostic: %#v", diagnostic)
		}
	}
}
