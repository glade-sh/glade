package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native anonymous inserts and updates, API 62 and 67:
// standard_webstore_preserve_null/org.tsv:2-7. REST is a local transport of
// the same DML engine and is served at the supported REST API version.
func TestWebStoreForcedInsertRESTRoute(t *testing.T) {
	definition, ok := storage.StandardObjectDefinition("WebStore")
	if !ok {
		t.Fatal("missing WebStore")
	}
	storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"B2BCommerce", "Communities", "CommerceEnabled"})
	org := testOrg()
	org.Objects["WebStore"] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	handler := New(&org)
	for _, body := range []string{`{"Name":"unset"}`, `{"Name":"false","OptionsPreserveGuestCartEnabled":false}`, `{"Name":"null","OptionsPreserveGuestCartEnabled":null}`} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, serverTestDataPath+"/sobjects/WebStore", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, req)
		var result struct {
			ID      string `json:"id"`
			Success bool   `json:"success"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusCreated || !result.Success {
			t.Fatalf("insert %s: %d %s %v", body, rec.Code, rec.Body.String(), err)
		}
		stored, found := handler.Org.Objects["WebStore"].Records[storage.ID(result.ID)]
		if !found {
			t.Fatalf("record missing: %s", body)
		}
		value, _ := stored.GetField("OptionsPreserveGuestCartEnabled")
		if value.Kind != storage.ValueBoolean || !value.Boolean {
			t.Fatalf("%s: stored %#v", body, value)
		}
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPatch, serverTestDataPath+"/sobjects/WebStore/"+result.ID, strings.NewReader(`{"OptionsPreserveGuestCartEnabled":false}`))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("update %s: %d %s", body, rec.Code, rec.Body.String())
		}
		stored = handler.Org.Objects["WebStore"].Records[storage.ID(result.ID)]
		value, _ = stored.GetField("OptionsPreserveGuestCartEnabled")
		if value.Kind != storage.ValueBoolean || value.Boolean {
			t.Fatalf("%s: updated %#v", body, value)
		}
	}
}
