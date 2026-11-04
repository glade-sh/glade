package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The storage package checks these lists against native REST describes. This
// checks local wire repair against the local complete definition, without
// claiming a native UI API picklist observation.
func TestStandardPolicyPicklistLightningRepairRoute(t *testing.T) {
	for _, tc := range []struct{ object, field string }{{"Asset", "StatusReason"}, {"Contact", "GenderIdentity"}, {"Contact", "Pronouns"}, {"Contract", "ShippingGeocodeAccuracy"}, {"Event", "Type"}, {"Lead", "GenderIdentity"}, {"Lead", "Pronouns"}, {"Task", "Type"}} {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, tc.object)
			state, ok := org.Objects[tc.object]
			if !ok {
				t.Fatal("missing object")
			}
			field, ok := state.Definition.Fields[tc.field]
			if !ok {
				t.Fatal("missing field")
			}
			var want []string
			for _, value := range field.PicklistValues {
				want = append(want, value.Value)
			}
			if len(want) == 0 {
				t.Fatal("missing complete local list")
			}
			field.PicklistValues = nil
			field.PicklistValuesConfigured = false
			state.Definition.Fields[tc.field] = field
			org.Objects[tc.object] = state
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/lightning/wire/getObjectInfo", strings.NewReader(fmt.Sprintf(`{"objectApiName":%q}`, tc.object)))
			req.Header.Set("Content-Type", "application/json")
			New(&org).ServeHTTP(rec, req)
			var result struct {
				Data struct {
					Fields map[string]struct {
						Values []struct {
							Value string `json:"value"`
						} `json:"picklistValues"`
					} `json:"fields"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); rec.Code != 200 || err != nil {
				t.Fatalf("wire %d: %s: %v", rec.Code, rec.Body.String(), err)
			}
			var got []string
			for _, v := range result.Data.Fields[tc.field].Values {
				got = append(got, v.Value)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("wire values=%v complete local definition=%v", got, want)
			}
		})
	}
}

// Compare the local wire's values with the separate captured native UI API
// picklist-values resource, retaining native errors as boundaries only.
func TestStandardPolicyPicklistLightningNativeValues(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_policy_picklist_routes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if row[1] != "ui-picklist-values" {
			continue
		}
		var native struct {
			Status int `json:"status"`
			Values []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			} `json:"values"`
		}
		if err := json.Unmarshal([]byte(row[3]), &native); err != nil {
			t.Fatal(err)
		}
		if native.Status != 200 {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			parts := strings.SplitN(row[2], ".", 2)
			org := storage.NewOrgState()
			org.APIVersion = row[0]
			storage.EnsureStandardObject(&org, parts[0])
			state := org.Objects[parts[0]]
			field := state.Definition.Fields[parts[1]]
			field.PicklistValues = nil
			field.PicklistValuesConfigured = false
			state.Definition.Fields[parts[1]] = field
			org.Objects[parts[0]] = state
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/lightning/wire/getObjectInfo", strings.NewReader(fmt.Sprintf(`{"objectApiName":%q}`, parts[0])))
			req.Header.Set("Content-Type", "application/json")
			New(&org).ServeHTTP(rec, req)
			var result struct {
				Data struct {
					Fields map[string]struct {
						Values []struct {
							Value string `json:"value"`
							Label string `json:"label"`
						} `json:"picklistValues"`
					} `json:"fields"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); rec.Code != 200 || err != nil {
				t.Fatalf("wire=%d: %v", rec.Code, err)
			}
			if got := result.Data.Fields[parts[1]].Values; !reflect.DeepEqual(got, native.Values) {
				t.Fatalf("wire=%v native UI=%v", got, native.Values)
			}
		})
	}
	if count != 14 {
		t.Fatalf("captured successful UI rows=%d want 14", count)
	}
}
