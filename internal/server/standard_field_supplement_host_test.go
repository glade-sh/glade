package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStandardFieldSupplementLightningObjectInfoNative(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_field_supplement_routes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if row[1] != "ui-object-info" {
			continue
		}
		count++
		t.Run(row[0], func(t *testing.T) {
			var want map[string]map[string]any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = row[0]
			handler := New(&org)
			for _, state := range []string{"cold", "warm"} {
				t.Run(state, func(t *testing.T) {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest("POST", "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"Lead"}`))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					var payload struct {
						Data struct {
							Fields map[string]map[string]any `json:"fields"`
						} `json:"data"`
						Error any `json:"error"`
					}
					if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &payload) != nil || payload.Error != nil {
						t.Fatalf("object info: %d %s", rec.Code, rec.Body.String())
					}
					var nativeRejection string
					for _, control := range rows[1:] {
						if control[0] == row[0] && control[1] == "ui-rejection" {
							nativeRejection = control[2]
						}
					}
					_, present := payload.Data.Fields["Case251DefinitelyMissing__c"]
					if nativeRejection != "false" || present {
						t.Fatalf("unknown field: native=%s local=%v", nativeRejection, present)
					}
					for name, expected := range want {
						got, ok := payload.Data.Fields[name]
						if !ok {
							t.Errorf("missing %s", name)
							continue
						}
						for key, value := range expected {
							if got[key] != value {
								t.Errorf("%s.%s = %v, native %v", name, key, got[key], value)
							}
						}
					}
				})
			}
		})
	}
	if count != 2 {
		t.Fatalf("expected two native UI rows, got %d", count)
	}
}

// Compare repair through the wire with a complete local definition. Native
// Apex list values are checked separately on their captured Apex route.
func TestStandardFieldSupplementLightningPicklistRepair(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Lead")
	state := org.Objects["Lead"]
	field := state.Definition.Fields["CleanStatus"]
	var want []string
	for _, value := range field.PicklistValues {
		want = append(want, value.Value)
	}
	if len(want) == 0 {
		t.Fatal("missing complete local list")
	}
	field.Type = storage.FieldAny
	field.DisplayType = "ANY"
	field.PicklistValues = nil
	state.Definition.Fields["CleanStatus"] = field
	org.Objects["Lead"] = state
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"Lead"}`))
	req.Header.Set("Content-Type", "application/json")
	New(&org).ServeHTTP(rec, req)
	var payload struct {
		Data struct {
			Fields map[string]struct {
				PicklistValues []struct {
					Value string `json:"value"`
				} `json:"picklistValues"`
			} `json:"fields"`
		} `json:"data"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &payload) != nil {
		t.Fatalf("wire: %d %s", rec.Code, rec.Body.String())
	}
	var got []string
	for _, v := range payload.Data.Fields["CleanStatus"].PicklistValues {
		got = append(got, v.Value)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("wire values %v; complete local definition %v", got, want)
	}
}
