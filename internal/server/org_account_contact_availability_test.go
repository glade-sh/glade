package server

import (
	"fmt"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local LWC object-info uses object availability captured at API 62/67 in
// org_account_contact_availability/org.tsv and org_object_availability/org.tsv.
func TestOrgAccountContactAvailabilityObjectInfo(t *testing.T) {
	for _, tc := range []struct{ object, feature string }{
		{"AccountContactRelation", "ContactsToMultipleAccounts"},
		{"Quote", "Quotes"},
		{"WebStore", "OrderManagement"},
	} {
		t.Run(tc.object, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, tc.object)
			storage.ApplyOrgShape(&org, nil)
			if _, wireErr := getObjectInfoWireData(&org, tc.object); wireErr == nil {
				t.Fatalf("plain shape returned %s object info", tc.object)
			}
			storage.ApplyOrgShape(&org, []string{tc.feature})
			if _, wireErr := getObjectInfoWireData(&org, tc.object); wireErr != nil {
				t.Fatalf("feature shape rejected %s object info: %v", tc.object, wireErr)
			}
		})
	}
}

// These are local bridge contracts over the native object-availability rows;
// no Salesforce hosted error text is inferred from the Apex capture.
func TestOrgAccountContactAvailabilityListBridges(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			org := storage.NewOrgState()
			if enabled {
				storage.ApplyOrgShape(&org, []string{"ContactsToMultipleAccounts"})
			} else {
				storage.ApplyOrgShape(&org, nil)
			}
			s := New(&org)
			cfg := map[string]any{"objectApiName": "AccountContactRelation", "listViewApiName": "Missing"}
			parent := map[string]any{"parentObjectApiName": "AccountContactRelation", "relatedListId": "Missing"}
			tests := []struct {
				name string
				call func() *lwcbrowser.WireError
			}{
				{"delete", func() *lwcbrowser.WireError { return s.listDeleteData(cfg) }},
				{"write", func() *lwcbrowser.WireError { _, e := s.listWriteData("updateListInfo", cfg); return e }},
				{"related", func() *lwcbrowser.WireError { _, e := s.relatedListInfoData(parent); return e }},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					e := tc.call()
					if e == nil {
						t.Fatal("expected missing list error")
					}
					if e.Type == "INSUFFICIENT_ACCESS" && enabled {
						t.Fatalf("enabled shape rejected object: %+v", e)
					}
					if e.Type != "INSUFFICIENT_ACCESS" && !enabled {
						t.Fatalf("disabled shape not rejected: %+v", e)
					}
				})
			}
			_, readErr := s.listReadData("getListInfosByObjectName", cfg)
			if enabled && readErr != nil {
				t.Fatalf("enabled list collection: %+v", readErr)
			}
			if !enabled && (readErr == nil || readErr.Type != "INSUFFICIENT_ACCESS") {
				t.Fatalf("disabled list collection: %+v", readErr)
			}
			_, err := s.relatedListsInfoData(parent)
			if enabled && err != nil {
				t.Fatalf("enabled related lists: %+v", err)
			}
			if !enabled && (err == nil || err.Type != "INSUFFICIENT_ACCESS") {
				t.Fatalf("disabled related lists: %+v", err)
			}
			rec := httptest.NewRecorder()
			s.handleLightningLocalRecords(rec, httptest.NewRequest("GET", "/lightning/local/records.json?object=AccountContactRelation", nil))
			want := http.StatusNotFound
			if enabled {
				want = http.StatusOK
			}
			if rec.Code != want {
				t.Fatalf("local records status=%d want=%d", rec.Code, want)
			}
		})
	}
}
