package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRecordCountResourceCountsLocalRecordsByObject(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "First")
	addAccountForTest(&org, "001000000000002", "Second")
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {
				ID:     "003000000000001",
				Object: "Contact",
				Fields: map[string]storage.Value{
					"LastName": storage.StringValue("Lake"),
				},
			},
		},
	}

	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/limits/recordCount", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("record count status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		SObjects []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"sObjects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.SObjects) != 2 {
		t.Fatalf("record count objects = %#v body=%s", payload.SObjects, rec.Body.String())
	}
	if payload.SObjects[0].Name != "Account" || payload.SObjects[0].Count != 2 ||
		payload.SObjects[1].Name != "Contact" || payload.SObjects[1].Count != 1 {
		t.Fatalf("record count payload = %#v body=%s", payload.SObjects, rec.Body.String())
	}
}

func TestRecordCountResourceFiltersRequestedObjects(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "First")

	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/limits/recordCount?sObjects=Missing__c,Account", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("filtered record count status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, `"name":"Account"`) || strings.Contains(got, "Missing__c") {
		t.Fatalf("filtered record count body=%s", got)
	}
}

func TestRecordCountResourceRequiresGET(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, serverTestDataPath+"/limits/recordCount", nil))

	assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("record count Allow = %q", got)
	}
}
