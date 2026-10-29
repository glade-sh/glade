package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetEmailResidualOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		PlainExecutableCount   int      `json:"plainExecutableCount"`
		PlainCarryCount        int      `json:"plainCarryCount"`
		NamespaceCarryCount    int      `json:"namespaceCarryCount"`
		Rows                   []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				nativeBoundaryRoute
				ExpectedByAPI map[string]nativeObservationExpected `json:"expectedByAPI"`
				CarryReason   string                               `json:"carryReason"`
				Pages         []string                             `json:"pages,omitempty"`
			} `json:"routes"`
		} `json:"rows"`
		NamespaceCarries []struct {
			ID                  string                               `json:"id"`
			Route               string                               `json:"route"`
			ReferenceRow        string                               `json:"referenceRow"`
			ReferenceContext    string                               `json:"referenceContext"`
			CarryReason         string                               `json:"carryReason"`
			PlainReferenceByAPI map[string]nativeObservationExpected `json:"plainReferenceByAPI"`
		} `json:"namespaceCarries"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", "residual_set_email.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || data.ShapeCount != 17 || len(data.Rows) != 17 ||
		data.NativeObservationCount != 84 || data.PlainExecutableCount != 84 || data.PlainCarryCount != 0 || data.NamespaceCarryCount != 42 {
		t.Fatal("incomplete set/email native matrix")
	}
	seen := map[string]bool{}
	plainCount, plainCarries := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) == 0 || len(row.Routes) > 2 {
			t.Fatalf("invalid native row: %s", row.ID)
		}
		seen[row.ID] = true
		for route, source := range row.Routes {
			if route != "anonymous" && route != "isTest" || source.ID != row.ID || len(source.ExpectedByAPI) != 3 {
				t.Fatalf("invalid native route: %s/%s", row.ID, route)
			}
			if source.Stage == "carried" {
				if strings.TrimSpace(source.CarryReason) == "" {
					t.Fatal("plain carry needs its reason")
				}
				plainCarries += 3
			} else if source.Stage != "observed" {
				t.Fatal("unrecognized native stage")
			}
			if route == "anonymous" && source.Anonymous == "" || route == "isTest" && (len(source.Tests) != 1 || source.Classes[source.Tests[0]] == "") {
				t.Fatal("missing native measurement source")
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || want.Outcome != "runtime" || len(want.Value) == 0 {
					t.Fatalf("missing native value: %s/%s/%s", row.ID, route, api)
				}
			}
			plainCount += 3
		}
	}
	if plainCount != data.NativeObservationCount || plainCarries != data.PlainCarryCount || len(data.NamespaceCarries)*3 != data.NamespaceCarryCount {
		t.Fatal("native and carried totals disagree")
	}
	seenCarries := map[string]bool{}
	for _, carry := range data.NamespaceCarries {
		key := carry.ID + "/" + carry.Route
		if seenCarries[key] || carry.ReferenceContext != "plain" || strings.TrimSpace(carry.CarryReason) == "" || len(carry.PlainReferenceByAPI) != 3 {
			t.Fatalf("invalid namespace carry: %s", key)
		}
		seenCarries[key] = true
		found := false
		for _, row := range data.Rows {
			if row.ID != carry.ReferenceRow {
				continue
			}
			if source, ok := row.Routes[carry.Route]; ok {
				found = true
				for _, api := range data.APIVersions {
					plain, ok := source.ExpectedByAPI[api]
					reference, referenceOK := carry.PlainReferenceByAPI[api]
					var plainValue, referenceValue any
					if !ok || !referenceOK || reference.Outcome != plain.Outcome ||
						json.Unmarshal(plain.Value, &plainValue) != nil || json.Unmarshal(reference.Value, &referenceValue) != nil ||
						!reflect.DeepEqual(plainValue, referenceValue) {
						found = false
					}
				}
			}
		}
		if !found {
			t.Fatalf("namespace carry lost its plain native reference: %s", key)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SetEmailResidual", api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for route, source := range row.Routes {
						t.Run(route, func(t *testing.T) {
							if source.Stage == "carried" {
								counts.TrackKind(t, route, "carried", 1)
								value, _ := json.Marshal(source.ExpectedByAPI[api])
								t.Logf("plain carry: %s; native=%s", source.CarryReason, value)
								return
							}
							counts.TrackKind(t, route, "exact", 1)
							residualObservationObserve(t, api, route, source.nativeBoundaryRoute, source.ExpectedByAPI[api], source.Pages...)
						})
					}
				})
			}
			for _, carry := range data.NamespaceCarries {
				t.Run("namespace/"+carry.ID+"/"+carry.Route, func(t *testing.T) {
					counts.TrackKind(t, "namespace/"+carry.Route, "carried", 1)
					value, _ := json.Marshal(carry.PlainReferenceByAPI[api])
					t.Logf("namespace unobserved: %s; plain reference %s=%s", carry.CarryReason, carry.ReferenceRow, value)
				})
			}
		})
	}
}
