package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordListViewsOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Routes                 []string `json:"routes"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		StillPendingCount      int      `json:"stillPendingCount"`
		UnobservedShapes       []struct {
			ID           string   `json:"id"`
			ReferenceRow string   `json:"referenceRow"`
			Reason       string   `json:"reason"`
			MeasuredApex string   `json:"measuredApex"`
			Routes       []string `json:"routes"`
		} `json:"unobservedShapes"`
		Rows []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				nativeBoundaryRoute
				ExpectedByAPI map[string]nativeObservationExpected `json:"expectedByAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", "record_list_views.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" || data.ShapeCount != 21 || len(data.Rows) != 21 || data.NativeObservationCount != 126 || data.StillPendingCount != 24 || len(data.UnobservedShapes) != 4 {
		t.Fatal("incomplete record-list observation matrix")
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "RecordListViews", api)
			base := conformanceSchemaBase(t, nativeBoundaryIndex(t, api, nil, nil), api)
			for _, pending := range data.UnobservedShapes {
				if strings.Join(pending.Routes, ",") != "anonymous,isTest" || pending.ID == "" || pending.Reason == "" || !strings.Contains(pending.MeasuredApex, "List<Account>.class") {
					t.Fatal("missing Account carry measurement or reason")
				}
				for _, route := range pending.Routes {
					t.Run(pending.ID+"/"+route, func(t *testing.T) {
						counts.TrackKind(t, route, "carried", 1)
						found := false
						for _, row := range data.Rows {
							if row.ID == pending.ReferenceRow {
								reference := row.Routes[route].ExpectedByAPI[api]
								if reference.Outcome != "runtime" || len(reference.Value) == 0 {
									t.Fatal("missing native Lead assertion reference")
								}
								t.Logf("Account unobserved: %s; native Lead reference %s=%s", pending.Reason, row.ID, reference.Value)
								found = true
							}
						}
						if !found {
							t.Fatal("missing native Lead reference row")
						}
					})
				}
			}
			for _, row := range data.Rows {
				for _, route := range data.Routes {
					t.Run(row.ID+"/"+route, func(t *testing.T) {
						source, ok := row.Routes[route]
						want, present := source.ExpectedByAPI[api]
						if !ok || !present || source.Stage != "observed" || len(source.ExpectedByAPI) != 3 || len(row.Routes) != 2 {
							t.Fatal("missing native record-list cell")
						}
						counts.TrackKind(t, route, "exact", 1)
						residualObservationObserveCore(t, api, route, source.nativeBoundaryRoute, want, true, base)
					})
				}
			}
		})
	}
}
