package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/vm"
)

func assertQueryRecordShape(t *testing.T, record map[string]any, objectName, id, url string) {
	t.Helper()
	attrs, ok := record["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("record attributes missing or wrong type: %#v", record)
	}
	if attrs["type"] != objectName || attrs["url"] != url {
		t.Fatalf("record attributes = %#v", attrs)
	}
	if record["Id"] != id {
		t.Fatalf("record Id = %#v", record["Id"])
	}
	for _, internal := range []string{"fields", "system", "children", "object", "id", "explicitNulls"} {
		if _, ok := record[internal]; ok {
			t.Fatalf("record leaked internal %q field: %#v", internal, record)
		}
	}
}

func TestVersionDiscoveryRoot(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{"/services/data", "/services/data/"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		var versions []apiVersionEntry
		if err := json.Unmarshal(rec.Body.Bytes(), &versions); err != nil {
			t.Fatalf("%s unmarshal: %v body=%s", path, err, rec.Body.String())
		}
		if len(versions) != 1 {
			t.Fatalf("%s versions = %#v", path, versions)
		}
		if versions[0].Version != "61.0" || versions[0].Label != "Summer '24" || versions[0].URL != "/services/data/v61.0" {
			t.Fatalf("%s version entry = %#v", path, versions[0])
		}
	}
}

func TestVersionDiscoveryRootMethodNotAllowed(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data", nil))
	assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if rec.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}

func TestSObjectCRUDAndQuery(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	body := `{"Name":{"kind":"string","string":"Acme"}}`
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID storage.ID `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "001000000000001" {
		t.Fatalf("created id = %s", created.ID)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	if get.Code != http.StatusOK || !bytes.Contains(get.Body.Bytes(), []byte(`"Acme"`)) {
		t.Fatalf("get status = %d body=%s", get.Code, get.Body.String())
	}

	patch := httptest.NewRecorder()
	handler.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/"+string(created.ID), strings.NewReader(`{"Name":{"kind":"string","string":"Changed"}}`)))
	if patch.Code != http.StatusNoContent {
		t.Fatalf("patch status = %d body=%s", patch.Code, patch.Body.String())
	}

	query := httptest.NewRecorder()
	handler.ServeHTTP(query, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id,%20Name%20FROM%20Account%20WHERE%20Name%20=%20'Changed'", nil))
	if query.Code != http.StatusOK || !bytes.Contains(query.Body.Bytes(), []byte(`"totalSize":1`)) {
		t.Fatalf("query status = %d body=%s", query.Code, query.Body.String())
	}

	del := httptest.NewRecorder()
	handler.ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", del.Code, del.Body.String())
	}
}

func TestQueryExplainReturnsUnsupportedFeature(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Acme")
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/query?explain=SELECT%20Id%20FROM%20Account",
		"/services/data/v61.0/query?explain=",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "SOQL query plan explain is not implemented in the local server")
	}

	normal := httptest.NewRecorder()
	handler.ServeHTTP(normal, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id,%20Name%20FROM%20Account%20WHERE%20Name%20=%20'Acme'", nil))
	if normal.Code != http.StatusOK || !bytes.Contains(normal.Body.Bytes(), []byte(`"totalSize":1`)) || !bytes.Contains(normal.Body.Bytes(), []byte(`"Name":"Acme"`)) {
		t.Fatalf("normal query status = %d body=%s", normal.Code, normal.Body.String())
	}

	wrongMethod := httptest.NewRecorder()
	handler.ServeHTTP(wrongMethod, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/query?explain=SELECT%20Id%20FROM%20Account", nil))
	assertSalesforceError(t, wrongMethod, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
}

func TestSObjectRecordGETFieldProjection(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Trail"),
			"Description":    storage.StringValue("Lake shore account"),
			"External_Id__c": storage.StringValue("EXT-1"),
		},
	}
	org.Objects["Account"] = object
	handler := New(&org)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/001000000000001?fields=Name,Description", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("projected get status = %d body=%s", get.Code, get.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	assertQueryRecordShape(t, payload, "Account", "001000000000001", "/services/data/v61.0/sobjects/Account/001000000000001")
	if payload["Name"] != "Trail" || payload["Description"] != "Lake shore account" {
		t.Fatalf("projected get payload = %#v", payload)
	}
	if _, ok := payload["External_Id__c"]; ok {
		t.Fatalf("projected get leaked unrequested field: %#v", payload)
	}
	if len(payload) != 4 {
		t.Fatalf("projected get field count = %d payload=%#v", len(payload), payload)
	}
}

func TestSObjectExternalIDRecordGETFieldProjection(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Trail"),
			"Description":    storage.StringValue("Lake shore account"),
			"External_Id__c": storage.StringValue("EXT-1"),
		},
	}
	org.Objects["Account"] = object
	handler := New(&org)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/sobjects/Account/External_Id__c/EXT-1?fields=Name", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("projected external id get status = %d body=%s", get.Code, get.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	assertQueryRecordShape(t, payload, "Account", "001000000000001", "/services/data/v60.0/sobjects/Account/001000000000001")
	if payload["Name"] != "Trail" {
		t.Fatalf("projected external id get payload = %#v", payload)
	}
	for _, field := range []string{"Description", "External_Id__c"} {
		if _, ok := payload[field]; ok {
			t.Fatalf("projected external id get leaked %s: %#v", field, payload)
		}
	}
	if len(payload) != 3 {
		t.Fatalf("projected external id get field count = %d payload=%#v", len(payload), payload)
	}
}

func TestSObjectRecordGETFieldProjectionRejectsUnknownField(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Trail")
	handler := New(&org)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/001000000000001?fields=Name,Nope__c", nil))
	if get.Code != http.StatusBadRequest || !bytes.Contains(get.Body.Bytes(), []byte(`"errorCode":"INVALID_FIELD"`)) {
		t.Fatalf("unknown projected field status = %d body=%s", get.Code, get.Body.String())
	}
}

func TestSObjectRecordGETBlankFieldsPreservesFullPayload(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Trail"),
			"External_Id__c": storage.StringValue("EXT-1"),
		},
		ExplicitNulls: map[string]bool{"Description": true},
	}
	org.Objects["Account"] = object
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/001000000000001",
		"/services/data/v61.0/sobjects/Account/001000000000001?fields=%20%20",
	} {
		get := httptest.NewRecorder()
		handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, path, nil))
		if get.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, get.Code, get.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["Name"] != "Trail" || payload["External_Id__c"] != "EXT-1" {
			t.Fatalf("%s full payload = %#v", path, payload)
		}
		if value, ok := payload["Description"]; !ok || value != nil {
			t.Fatalf("%s full payload Description = %#v ok=%v", path, value, ok)
		}
	}
}

func TestSObjectExternalIDRoutesCRUD(t *testing.T) {
	org := testOrg()
	addExternalIDFieldForTest(&org)
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/External_Id__c/EXT-1", strings.NewReader(`{"Name":"Trail","Description":null}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("external id create status = %d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID      storage.ID `json:"id"`
		Success bool       `json:"success"`
		Created bool       `json:"created"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "001000000000001" || !created.Success || !created.Created {
		t.Fatalf("external id create payload = %#v", created)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/sobjects/Account/External_Id__c/EXT-1", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("external id get status = %d body=%s", get.Code, get.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	attrs, ok := payload["attributes"].(map[string]any)
	if !ok || attrs["url"] != "/services/data/v60.0/sobjects/Account/001000000000001" {
		t.Fatalf("external id record attributes = %#v", payload["attributes"])
	}
	if payload["Id"] != "001000000000001" || payload["External_Id__c"] != "EXT-1" || payload["Description"] != nil {
		t.Fatalf("external id get payload = %#v", payload)
	}

	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/External_Id__c/EXT-1", strings.NewReader(`{"Name":"Trail Updated","External_Id__c":"EXT-1","Description":null}`)))
	if update.Code != http.StatusNoContent {
		t.Fatalf("external id update status = %d body=%s", update.Code, update.Body.String())
	}

	updatedGet := httptest.NewRecorder()
	handler.ServeHTTP(updatedGet, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/External_Id__c/EXT-1", nil))
	if updatedGet.Code != http.StatusOK || !bytes.Contains(updatedGet.Body.Bytes(), []byte(`"Trail Updated"`)) || !bytes.Contains(updatedGet.Body.Bytes(), []byte(`"Description":null`)) {
		t.Fatalf("external id updated get status = %d body=%s", updatedGet.Code, updatedGet.Body.String())
	}

	del := httptest.NewRecorder()
	handler.ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/sobjects/Account/External_Id__c/EXT-1", nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("external id delete status = %d body=%s", del.Code, del.Body.String())
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/External_Id__c/EXT-1", nil))
	if missing.Code != http.StatusNotFound || !bytes.Contains(missing.Body.Bytes(), []byte(`"errorCode":"NOT_FOUND"`)) {
		t.Fatalf("external id missing status = %d body=%s", missing.Code, missing.Body.String())
	}
}

func TestSObjectExternalIDRoutesRejectNonExternalFieldsAndMethods(t *testing.T) {
	org := testOrg()
	addExternalIDFieldForTest(&org)
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/Name/Acme",
		"/services/data/v61.0/sobjects/Account/Missing_Key__c/Acme",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"INVALID_FIELD"`)) {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account/External_Id__c/EXT-1", strings.NewReader(`{}`)))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("external id method status = %d body=%s", method.Code, method.Body.String())
	}
	if got := method.Header().Get("Allow"); got != "GET, PATCH, DELETE" {
		t.Fatalf("external id method Allow = %q", got)
	}
	if !bytes.Contains(method.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("external id method shape = %s", method.Body.String())
	}
}

func TestSObjectExternalIDRoutesHandleEscapedPathSegments(t *testing.T) {
	org := testOrg()
	addExternalIDFieldForTest(&org)
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/External_Id__c/DEPT%2F001", strings.NewReader(`{"Name":"Slash Trail"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("external id escaped create status = %d body=%s", create.Code, create.Body.String())
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/External_Id__c/DEPT%2F001", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("external id escaped get status = %d body=%s", get.Code, get.Body.String())
	}
	if !bytes.Contains(get.Body.Bytes(), []byte(`"External_Id__c":"DEPT/001"`)) || !bytes.Contains(get.Body.Bytes(), []byte(`"Slash Trail"`)) {
		t.Fatalf("external id escaped get body = %s", get.Body.String())
	}
}

func TestStorageValuesEqualCaseInsensitiveIDStringCoercion(t *testing.T) {
	field := storage.Field{Type: storage.FieldID, CaseSensitive: false}
	if !storageValuesEqual(field, storage.IDValue("ABC001"), storage.StringValue("abc001")) {
		t.Fatal("case-insensitive ID/string external ID comparison failed")
	}
	field.CaseSensitive = true
	if storageValuesEqual(field, storage.IDValue("ABC001"), storage.StringValue("abc001")) {
		t.Fatal("case-sensitive ID/string external ID comparison matched")
	}
}

func TestQueryPaginationNoBatchSizePreservesFullResult(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "A")
	addAccountForTest(&org, "001000000000002", "B")
	addAccountForTest(&org, "001000000000003", "C")
	handler := New(&org)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id,%20Name%20FROM%20Account", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("query status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		TotalSize      int              `json:"totalSize"`
		Done           bool             `json:"done"`
		Records        []map[string]any `json:"records"`
		NextRecordsURL string           `json:"nextRecordsUrl"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.TotalSize != 3 || !payload.Done || len(payload.Records) != 3 || payload.NextRecordsURL != "" {
		t.Fatalf("payload = %#v", payload)
	}
	assertQueryRecordShape(t, payload.Records[0], "Account", "001000000000001", "/services/data/v61.0/sobjects/Account/001000000000001")
}

func TestQueryPaginationFirstAndNextPage(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "A")
	addAccountForTest(&org, "001000000000002", "B")
	addAccountForTest(&org, "001000000000003", "C")
	handler := New(&org)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id,%20Name%20FROM%20Account&batchSize=2", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}
	var firstPayload struct {
		TotalSize      int              `json:"totalSize"`
		Done           bool             `json:"done"`
		Records        []map[string]any `json:"records"`
		NextRecordsURL string           `json:"nextRecordsUrl"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstPayload); err != nil {
		t.Fatal(err)
	}
	if firstPayload.TotalSize != 3 || firstPayload.Done || len(firstPayload.Records) != 2 {
		t.Fatalf("first payload = %#v", firstPayload)
	}
	if firstPayload.NextRecordsURL != "/services/data/v61.0/query/oaerql000001-2" {
		t.Fatalf("nextRecordsUrl = %q", firstPayload.NextRecordsURL)
	}
	assertQueryRecordShape(t, firstPayload.Records[0], "Account", "001000000000001", "/services/data/v61.0/sobjects/Account/001000000000001")

	next := httptest.NewRecorder()
	handler.ServeHTTP(next, httptest.NewRequest(http.MethodGet, firstPayload.NextRecordsURL, nil))
	if next.Code != http.StatusOK {
		t.Fatalf("next status = %d body=%s", next.Code, next.Body.String())
	}
	var nextPayload struct {
		TotalSize      int              `json:"totalSize"`
		Done           bool             `json:"done"`
		Records        []map[string]any `json:"records"`
		NextRecordsURL string           `json:"nextRecordsUrl"`
	}
	if err := json.Unmarshal(next.Body.Bytes(), &nextPayload); err != nil {
		t.Fatal(err)
	}
	if nextPayload.TotalSize != 3 || !nextPayload.Done || len(nextPayload.Records) != 1 || nextPayload.NextRecordsURL != "" {
		t.Fatalf("next payload = %#v", nextPayload)
	}
	if nextPayload.Records[0]["Id"] != "001000000000003" {
		t.Fatalf("next record = %#v", nextPayload.Records[0])
	}
	assertQueryRecordShape(t, nextPayload.Records[0], "Account", "001000000000003", "/services/data/v61.0/sobjects/Account/001000000000003")
}

func TestQueryPaginationInvalidBatchSize(t *testing.T) {
	tests := []string{"0", "-1", "abc", "2001"}
	for _, batchSize := range tests {
		t.Run(batchSize, func(t *testing.T) {
			org := testOrg()
			addAccountForTest(&org, "001000000000001", "A")
			handler := New(&org)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id%20FROM%20Account&batchSize="+batchSize, nil))
			if recorder.Code != http.StatusBadRequest || !bytes.Contains(recorder.Body.Bytes(), []byte("MALFORMED_QUERY")) {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestQueryMoreUnknownLocator(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query/oaerql999999-2", nil))
	if recorder.Code != http.StatusNotFound || !bytes.Contains(recorder.Body.Bytes(), []byte("NOT_FOUND")) {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestQueryMoreRejectsOffsetAtEnd(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "A")
	addAccountForTest(&org, "001000000000002", "B")
	addAccountForTest(&org, "001000000000003", "C")
	handler := New(&org)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id%20FROM%20Account&batchSize=2", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query/oaerql000001-3", nil))
	if recorder.Code != http.StatusNotFound || !bytes.Contains(recorder.Body.Bytes(), []byte("NOT_FOUND")) {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSObjectRecordGetShapeAndNullPatch(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"Acme","Description":"Old"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID storage.ID `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", get.Code, get.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	attrs, ok := payload["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes missing from %#v", payload)
	}
	if attrs["type"] != "Account" || attrs["url"] != "/services/data/v61.0/sobjects/Account/"+string(created.ID) {
		t.Fatalf("attributes = %#v", attrs)
	}
	if payload["Id"] != string(created.ID) || payload["Name"] != "Acme" || payload["Description"] != "Old" {
		t.Fatalf("record payload = %#v", payload)
	}
	if _, ok := payload["fields"]; ok {
		t.Fatalf("record payload exposed storage fields wrapper: %#v", payload)
	}

	patch := httptest.NewRecorder()
	handler.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/"+string(created.ID), strings.NewReader(`{"Description":null}`)))
	if patch.Code != http.StatusNoContent {
		t.Fatalf("patch status = %d body=%s", patch.Code, patch.Body.String())
	}
	stored := org.Objects["Account"].Records[created.ID]
	if _, ok := stored.Fields["Description"]; ok || !stored.ExplicitNulls["Description"] {
		t.Fatalf("stored null state = fields %#v explicitNulls %#v", stored.Fields, stored.ExplicitNulls)
	}

	get = httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	if err := json.Unmarshal(get.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if value, ok := payload["Description"]; !ok || value != nil {
		t.Fatalf("Description after null patch = %#v in %#v", value, payload)
	}
}

func TestSObjectCRUDMissingAndDeletedEdges(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	missingGet := httptest.NewRecorder()
	handler.ServeHTTP(missingGet, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/001000000000999", nil))
	assertSalesforceError(t, missingGet, http.StatusNotFound, "NOT_FOUND", "record not found")

	missingDelete := httptest.NewRecorder()
	handler.ServeHTTP(missingDelete, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/sobjects/Account/001000000000999", nil))
	assertSalesforceError(t, missingDelete, http.StatusNotFound, "NOT_FOUND", "record not found")

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"To Delete"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID storage.ID `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	del := httptest.NewRecorder()
	handler.ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", del.Code, del.Body.String())
	}
	getDeleted := httptest.NewRecorder()
	handler.ServeHTTP(getDeleted, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	assertSalesforceError(t, getDeleted, http.StatusNotFound, "NOT_FOUND", "record not found")

	recent := httptest.NewRecorder()
	handler.ServeHTTP(recent, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/recent", nil))
	if recent.Code != http.StatusOK {
		t.Fatalf("recent status = %d body=%s", recent.Code, recent.Body.String())
	}
	if bytes.Contains(recent.Body.Bytes(), []byte(`To Delete`)) || bytes.Contains(recent.Body.Bytes(), []byte(created.ID)) {
		t.Fatalf("recent exposed deleted record: %s", recent.Body.String())
	}

	allRecent := httptest.NewRecorder()
	handler.ServeHTTP(allRecent, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/recent", nil))
	if allRecent.Code != http.StatusOK {
		t.Fatalf("all recent status = %d body=%s", allRecent.Code, allRecent.Body.String())
	}
	if bytes.Contains(allRecent.Body.Bytes(), []byte(`To Delete`)) || bytes.Contains(allRecent.Body.Bytes(), []byte(created.ID)) {
		t.Fatalf("aggregate recent exposed deleted record: %s", allRecent.Body.String())
	}
}

func TestRecentResourcesHonorLimitQuery(t *testing.T) {
	org := testOrg()
	for i := 1; i <= 3; i++ {
		addAccountForTest(&org, storage.ID(fmt.Sprintf("001000000000%03d", i)), fmt.Sprintf("Account %d", i))
	}
	handler := New(&org)

	objectRecent := httptest.NewRecorder()
	handler.ServeHTTP(objectRecent, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/recent?limit=2", nil))
	if objectRecent.Code != http.StatusOK {
		t.Fatalf("object recent status = %d body=%s", objectRecent.Code, objectRecent.Body.String())
	}
	objectItems := decodeRecentItems(t, objectRecent)
	if len(objectItems) != 2 || objectItems[0]["Id"] != "001000000000003" || objectItems[1]["Id"] != "001000000000002" {
		t.Fatalf("object recent items = %#v", objectItems)
	}

	globalRecent := httptest.NewRecorder()
	handler.ServeHTTP(globalRecent, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/recent?limit=1", nil))
	if globalRecent.Code != http.StatusOK {
		t.Fatalf("global recent status = %d body=%s", globalRecent.Code, globalRecent.Body.String())
	}
	globalItems := decodeRecentItems(t, globalRecent)
	if len(globalItems) != 1 || globalItems[0]["Id"] != "001000000000003" {
		t.Fatalf("global recent items = %#v", globalItems)
	}
}

func TestRecentResourcesUseDefaultLimitForAbsentAndBlank(t *testing.T) {
	org := testOrg()
	for i := 1; i <= 26; i++ {
		addAccountForTest(&org, storage.ID(fmt.Sprintf("001000000000%03d", i)), fmt.Sprintf("Account %d", i))
	}
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/recent",
		"/services/data/v61.0/sobjects/Account/recent?limit=",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("recent status for %s = %d body=%s", path, rec.Code, rec.Body.String())
		}
		items := decodeRecentItems(t, rec)
		if len(items) != 25 || items[0]["Id"] != "001000000000026" || items[24]["Id"] != "001000000000002" {
			t.Fatalf("recent items for %s = len %d %#v", path, len(items), items)
		}
	}
}

func TestRecentResourcesRejectMalformedLimit(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Acme")
	handler := New(&org)

	for _, raw := range []string{"0", "-1", "abc", "1.5"} {
		path := "/services/data/v61.0/sobjects/Account/recent?limit=" + raw
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusBadRequest, "MALFORMED_QUERY", "limit must be a positive integer no greater than 200")
	}
}

func TestRecentResourcesRejectTooLargeLimit(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Acme")
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/recent?limit=201",
		"/services/data/v61.0/sobjects/Account/recent?limit=201",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusBadRequest, "MALFORMED_QUERY", "limit must be a positive integer no greater than 200")
	}
}

func TestRecentResourcesMethodHandlingIgnoresLimit(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Acme")
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/recent?limit=1",
		"/services/data/v61.0/sobjects/Account/recent?limit=1",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func TestSObjectUpdatedResourceReturnsNonDeletedIDs(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Old")},
		System: storage.SystemFields{CreatedDate: "2026-05-01T00:00:00Z"},
	}
	object.Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("New")},
		System: storage.SystemFields{LastModifiedDate: "2026-05-02T00:00:00Z"},
	}
	object.Records["001000000000003"] = storage.Record{
		ID:     "001000000000003",
		Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("Deleted")},
		System: storage.SystemFields{LastModifiedDate: "2026-05-03T00:00:00Z", IsDeleted: true},
	}
	org.Objects["Account"] = object
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/updated?start=2026-05-01T12:00:00Z&end=2026-05-02T12:00:00Z", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("updated status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		LatestDateCovered string   `json:"latestDateCovered"`
		IDs               []string `json:"ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.LatestDateCovered != "2026-05-02T00:00:00Z" || len(payload.IDs) != 1 || payload.IDs[0] != "001000000000002" {
		t.Fatalf("updated payload = %#v body=%s", payload, rec.Body.String())
	}

	endBoundary := httptest.NewRecorder()
	handler.ServeHTTP(endBoundary, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/updated?start=2026-05-01T00:00:00Z&end=2026-05-02T00:00:00Z", nil))
	if endBoundary.Code != http.StatusOK {
		t.Fatalf("updated end-boundary status = %d body=%s", endBoundary.Code, endBoundary.Body.String())
	}
	if bytes.Contains(endBoundary.Body.Bytes(), []byte("001000000000002")) {
		t.Fatalf("updated end boundary included exclusive end record: %s", endBoundary.Body.String())
	}

	emptyWindow := httptest.NewRecorder()
	handler.ServeHTTP(emptyWindow, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/updated?start=2026-05-02T00:00:00Z&end=2026-05-02T00:00:00Z", nil))
	if emptyWindow.Code != http.StatusOK {
		t.Fatalf("updated empty-window status = %d body=%s", emptyWindow.Code, emptyWindow.Body.String())
	}
	if !bytes.Contains(emptyWindow.Body.Bytes(), []byte(`"ids":[]`)) {
		t.Fatalf("updated start=end window returned ids: %s", emptyWindow.Body.String())
	}

	trailing := httptest.NewRecorder()
	handler.ServeHTTP(trailing, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/updated/", nil))
	if trailing.Code != http.StatusOK || !bytes.Contains(trailing.Body.Bytes(), []byte("001000000000001")) || !bytes.Contains(trailing.Body.Bytes(), []byte("001000000000002")) || bytes.Contains(trailing.Body.Bytes(), []byte("001000000000003")) {
		t.Fatalf("updated trailing status = %d body=%s", trailing.Code, trailing.Body.String())
	}
}

func TestSObjectDeletedResourceAfterDelete(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"Soft Gone"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID storage.ID `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	del := httptest.NewRecorder()
	handler.ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/sobjects/Account/"+string(created.ID), nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", del.Code, del.Body.String())
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/deleted/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("deleted status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		EarliestDateAvailable string `json:"earliestDateAvailable"`
		LatestDateCovered     string `json:"latestDateCovered"`
		DeletedRecords        []struct {
			ID          string `json:"id"`
			DeletedDate string `json:"deletedDate"`
		} `json:"deletedRecords"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.EarliestDateAvailable == "" || payload.LatestDateCovered == "" || len(payload.DeletedRecords) != 1 {
		t.Fatalf("deleted payload = %#v body=%s", payload, rec.Body.String())
	}
	if payload.DeletedRecords[0].ID != string(created.ID) || payload.DeletedRecords[0].DeletedDate == "" {
		t.Fatalf("deleted record = %#v body=%s", payload.DeletedRecords[0], rec.Body.String())
	}
}

func TestSObjectUpdatedDeletedResourceErrorsAndMethods(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c/updated", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")

	malformed := httptest.NewRecorder()
	handler.ServeHTTP(malformed, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/updated?start=not-a-date", nil))
	assertSalesforceError(t, malformed, http.StatusBadRequest, "MALFORMED_QUERY", "malformed start date")

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/updated",
		"/services/data/v61.0/sobjects/Account/deleted",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		if got := rec.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s Allow = %q", path, got)
		}
	}
}

func TestSObjectLayoutMetadataEdges(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/layouts",
		"/services/data/v61.0/sobjects/Account/describe/layouts",
		"/services/data/v61.0/sobjects/Account/describe/approvalLayouts",
		"/services/data/v61.0/sobjects/Account/describe/namedLayouts",
		"/services/data/v61.0/sobjects/Account/namedLayouts/Account%20Layout",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "layout")
	}

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/describe/approvalLayouts",
		"/services/data/v61.0/sobjects/Account/describe/namedLayouts",
		"/services/data/v61.0/sobjects/Account/namedLayouts/Account%20Layout",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		if got := rec.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s Allow = %q", path, got)
		}
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Missing__c/describe/approvalLayouts", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")

	unknown = httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c/namedLayouts/Account%20Layout", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")
}

func TestSObjectCompactLayoutsStub(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/compactLayouts",
		"/services/data/v61.0/sobjects/Account/describe/compactLayouts",
		"/services/data/v61.0/sobjects/Account/describe/compactLayouts/012000000000001",
	} {
		compact := httptest.NewRecorder()
		handler.ServeHTTP(compact, httptest.NewRequest(http.MethodGet, path, nil))
		if compact.Code != http.StatusOK {
			t.Fatalf("%s compact status = %d body=%s", path, compact.Code, compact.Body.String())
		}
		var payload struct {
			ObjectType             string           `json:"objectType"`
			CompactLayouts         []map[string]any `json:"compactLayouts"`
			DefaultCompactLayoutID *string          `json:"defaultCompactLayoutId"`
			Message                string           `json:"message"`
		}
		if err := json.Unmarshal(compact.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.ObjectType != "Account" || len(payload.CompactLayouts) != 0 || payload.DefaultCompactLayoutID != nil || !strings.Contains(payload.Message, "empty local stub") {
			t.Fatalf("%s compact payload = %#v", path, payload)
		}
	}

	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account/describe/compactLayouts/012000000000001", nil))
	assertSalesforceError(t, method, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if got := method.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want %q", got, http.MethodGet)
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c/describe/compactLayouts", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")

	unknown = httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c/describe/compactLayouts/012000000000001", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")
}

func TestSObjectCompactLayoutDescribeMethodBoundary(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/describe/compactLayouts",
		"/services/data/v61.0/sobjects/Account/describe/compactLayouts/012000000000001",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		if got := rec.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s Allow = %q, want %q", path, got, http.MethodGet)
		}
	}
}

func TestAdvertisedSObjectURLStubs(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/defaultValues",
		"/services/data/v61.0/sobjects/Account/defaultValues?recordTypeId&fields",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "default value metadata")
	}

	postDefaultValues := httptest.NewRecorder()
	handler.ServeHTTP(postDefaultValues, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account/defaultValues", nil))
	assertSalesforceError(t, postDefaultValues, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if got := postDefaultValues.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want %q", got, http.MethodGet)
	}

	rowTemplate := httptest.NewRecorder()
	handler.ServeHTTP(rowTemplate, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/%7BID%7D", nil))
	assertSalesforceError(t, rowTemplate, http.StatusBadRequest, "MALFORMED_ID", "rowTemplate placeholder")
}

func TestSObjectQuickActionRoutes(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/quickActions",
		"/services/data/v61.0/sobjects/Account/quickActions/NewTask",
		"/services/data/v61.0/sobjects/Account/quickActions/NewTask/defaultValues",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "quick action metadata")
	}

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/quickActions",
		"/services/data/v61.0/sobjects/Account/quickActions/NewTask",
		"/services/data/v61.0/sobjects/Account/quickActions/NewTask/defaultValues",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		if got := rec.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s Allow = %q, want %q", path, got, http.MethodGet)
		}
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c/quickActions", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")
}

func TestSObjectListViewRoutes(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	collection := httptest.NewRecorder()
	handler.ServeHTTP(collection, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/listviews", nil))
	if collection.Code != http.StatusOK {
		t.Fatalf("listviews collection status = %d body=%s", collection.Code, collection.Body.String())
	}
	var payload struct {
		Done      bool             `json:"done"`
		Size      int              `json:"size"`
		Listviews []map[string]any `json:"listviews"`
		Object    string           `json:"objectType"`
		URL       string           `json:"url"`
		Message   string           `json:"message"`
	}
	if err := json.Unmarshal(collection.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Done || payload.Size != 0 || len(payload.Listviews) != 0 || payload.Object != "Account" || payload.URL != "/services/data/v61.0/sobjects/Account/listviews" || !strings.Contains(payload.Message, "empty local stub") {
		t.Fatalf("listviews payload = %#v", payload)
	}

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/listviews/00B000000000001/describe",
		"/services/data/v61.0/sobjects/Account/listviews/00B000000000001/results",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "list view describe and result execution")
	}

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/listviews",
		"/services/data/v61.0/sobjects/Account/listviews/00B000000000001/describe",
		"/services/data/v61.0/sobjects/Account/listviews/00B000000000001/results",
	} {
		method := httptest.NewRecorder()
		handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, path, nil))
		assertSalesforceError(t, method, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		if got := method.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s Allow = %q, want %q", path, got, http.MethodGet)
		}
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c/listviews", nil))
	assertSalesforceError(t, unknown, http.StatusNotFound, "NOT_FOUND", "unknown object")
}

func TestDescribeEndpoints(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/services/data", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"url":"/services/data/v61.0"`)) {
		t.Fatalf("versions status = %d body=%s", list.Code, list.Body.String())
	}

	sobjects := httptest.NewRecorder()
	handler.ServeHTTP(sobjects, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects", nil))
	if sobjects.Code != http.StatusOK || !bytes.Contains(sobjects.Body.Bytes(), []byte(`"Account"`)) {
		t.Fatalf("sobjects status = %d body=%s", sobjects.Code, sobjects.Body.String())
	}
}

func TestDiscoveryIdentityAndUserInfoRejectUnsupportedMethods(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	handler := New(&org)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "version-discovery-post", method: http.MethodPost, path: "/services/data"},
		{name: "resource-discovery-post", method: http.MethodPost, path: "/services/data/v61.0"},
		{name: "identity-post", method: http.MethodPost, path: "/id/00D000000000001/005000000000001"},
		{name: "userinfo-post", method: http.MethodPost, path: "/services/oauth2/userinfo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			if got := rec.Header().Get("Allow"); got != http.MethodGet {
				t.Fatalf("Allow = %q, want %q", got, http.MethodGet)
			}
		})
	}
}

func TestSObjectResourceShape(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("object resource status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Name           string            `json:"name"`
		Label          string            `json:"label"`
		KeyPrefix      string            `json:"keyPrefix"`
		Custom         bool              `json:"custom"`
		ObjectDescribe string            `json:"objectDescribe"`
		RecentItems    string            `json:"recentItems"`
		Describe       string            `json:"describe"`
		URL            string            `json:"url"`
		URLs           map[string]string `json:"urls"`
		Fields         any               `json:"fields"`
		Records        any               `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "Account" || payload.Label != "Account" || payload.KeyPrefix != "001" || payload.Custom {
		t.Fatalf("object identity payload = %#v", payload)
	}
	if payload.ObjectDescribe != "/services/data/v61.0/sobjects/Account/describe" ||
		payload.RecentItems != "/services/data/v61.0/sobjects/Account/recent" ||
		payload.Describe != payload.ObjectDescribe ||
		payload.URL != "/services/data/v61.0/sobjects/Account" {
		t.Fatalf("object URL payload = %#v", payload)
	}
	wantURLs := map[string]string{
		"rowTemplate":     "/services/data/v61.0/sobjects/Account/{ID}",
		"defaultValues":   "/services/data/v61.0/sobjects/Account/defaultValues?recordTypeId&fields",
		"describe":        "/services/data/v61.0/sobjects/Account/describe",
		"recent":          "/services/data/v61.0/sobjects/Account/recent",
		"updated":         "/services/data/v61.0/sobjects/Account/updated",
		"deleted":         "/services/data/v61.0/sobjects/Account/deleted",
		"items":           "/services/data/v61.0/sobjects/Account",
		"layouts":         "/services/data/v61.0/sobjects/Account/describe/layouts",
		"approvalLayouts": "/services/data/v61.0/sobjects/Account/describe/approvalLayouts",
		"compactLayouts":  "/services/data/v61.0/sobjects/Account/compactLayouts",
		"namedLayouts":    "/services/data/v61.0/sobjects/Account/namedLayouts/{LAYOUT_NAME}",
		"quickActions":    "/services/data/v61.0/sobjects/Account/quickActions",
		"listviews":       "/services/data/v61.0/sobjects/Account/listviews",
	}
	for name, url := range wantURLs {
		if payload.URLs[name] != url {
			t.Fatalf("urls[%s] = %q, want %q; payload=%#v", name, payload.URLs[name], url, payload)
		}
	}
	if payload.Fields != nil || payload.Records != nil {
		t.Fatalf("object resource leaked internal fields/records: %#v", payload)
	}
}

func TestSObjectDescribePayloadIncludesCommonMetadataShape(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.PluralLabel = "Accounts"
	account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Label: "Account ID", Type: storage.FieldID, Required: true}
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true}
	account.Definition.Fields["External_Id__c"] = storage.Field{APIName: "External_Id__c", Label: "External ID", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Rating"] = storage.Field{
		APIName: "Rating",
		Label:   "Rating",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot Label", Active: true, Default: true},
			{Value: "Cold", Active: false},
		},
	}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Name: "Business Account", Active: true, Available: true, Default: true}}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			Label:     "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"AccountId": {APIName: "AccountId", Label: "Account ID", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
				"LastName":  {APIName: "LastName", Label: "Last Name", Type: storage.FieldString, Required: true},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
				CascadeDelete:      true,
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/describe", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("describe status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Name               string           `json:"name"`
		Label              string           `json:"label"`
		LabelPlural        string           `json:"labelPlural"`
		Custom             bool             `json:"custom"`
		KeyPrefix          string           `json:"keyPrefix"`
		Searchable         bool             `json:"searchable"`
		Queryable          bool             `json:"queryable"`
		Createable         bool             `json:"createable"`
		Updateable         bool             `json:"updateable"`
		Deletable          bool             `json:"deletable"`
		Fields             []map[string]any `json:"fields"`
		RecordTypeInfos    []map[string]any `json:"recordTypeInfos"`
		ChildRelationships []map[string]any `json:"childRelationships"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Name != "Account" || payload.Label != "Account" || payload.LabelPlural != "Accounts" || payload.Custom || payload.KeyPrefix != "001" {
		t.Fatalf("object describe identity = %#v", payload)
	}
	if !payload.Searchable || !payload.Queryable || !payload.Createable || !payload.Updateable || !payload.Deletable {
		t.Fatalf("object flags = %#v", payload)
	}
	if len(payload.RecordTypeInfos) != 1 || payload.RecordTypeInfos[0]["recordTypeId"] != "012000000000001" || payload.RecordTypeInfos[0]["defaultRecordTypeMapping"] != true {
		t.Fatalf("recordTypeInfos = %#v", payload.RecordTypeInfos)
	}
	if len(payload.ChildRelationships) != 1 || payload.ChildRelationships[0]["childSObject"] != "Contact" || payload.ChildRelationships[0]["field"] != "AccountId" || payload.ChildRelationships[0]["relationshipName"] != "Contacts" || payload.ChildRelationships[0]["cascadeDelete"] != true {
		t.Fatalf("childRelationships = %#v", payload.ChildRelationships)
	}

	fieldByName := func(name string) map[string]any {
		t.Helper()
		for _, field := range payload.Fields {
			if field["name"] == name {
				return field
			}
		}
		t.Fatalf("missing field %s in %#v", name, payload.Fields)
		return nil
	}
	id := fieldByName("Id")
	if id["label"] != "Account ID" || id["type"] != "id" || id["nillable"] != false || id["createable"] != false || id["updateable"] != false || id["idLookup"] != true {
		t.Fatalf("Id field = %#v", id)
	}
	name := fieldByName("Name")
	if name["label"] != "Account Name" || name["nillable"] != false || name["createable"] != true || name["filterable"] != true || name["sortable"] != true {
		t.Fatalf("Name field = %#v", name)
	}
	external := fieldByName("External_Id__c")
	if external["externalId"] != true || external["unique"] != true || external["idLookup"] != true {
		t.Fatalf("external id field = %#v", external)
	}
	rating := fieldByName("Rating")
	values, ok := rating["picklistValues"].([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("Rating picklistValues = %#v", rating["picklistValues"])
	}
	hot, ok := values[0].(map[string]any)
	if !ok || hot["value"] != "Hot" || hot["label"] != "Hot Label" || hot["active"] != true || hot["defaultValue"] != true {
		t.Fatalf("hot picklist value = %#v", values[0])
	}
	cold, ok := values[1].(map[string]any)
	if !ok || cold["value"] != "Cold" || cold["label"] != "Cold" || cold["active"] != false || cold["defaultValue"] != false {
		t.Fatalf("cold picklist value = %#v", values[1])
	}
}

func TestRequestedAPIVersionAppearsInSObjectURLs(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Acme")
	addAccountForTest(&org, "001000000000002", "Beta")
	addAccountForTest(&org, "001000000000003", "Gamma")
	handler := New(&org)

	sobjects := httptest.NewRecorder()
	handler.ServeHTTP(sobjects, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/sobjects", nil))
	if sobjects.Code != http.StatusOK {
		t.Fatalf("sobjects status = %d body=%s", sobjects.Code, sobjects.Body.String())
	}
	if !bytes.Contains(sobjects.Body.Bytes(), []byte(`"/services/data/v60.0/sobjects/Account"`)) || bytes.Contains(sobjects.Body.Bytes(), []byte(`/services/data/v61.0/sobjects/Account`)) {
		t.Fatalf("sobjects versioned URLs = %s", sobjects.Body.String())
	}

	object := httptest.NewRecorder()
	handler.ServeHTTP(object, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/sobjects/Account", nil))
	if object.Code != http.StatusOK {
		t.Fatalf("object status = %d body=%s", object.Code, object.Body.String())
	}
	var objectPayload struct {
		ObjectDescribe string            `json:"objectDescribe"`
		RecentItems    string            `json:"recentItems"`
		URL            string            `json:"url"`
		URLs           map[string]string `json:"urls"`
	}
	if err := json.Unmarshal(object.Body.Bytes(), &objectPayload); err != nil {
		t.Fatal(err)
	}
	if objectPayload.URL != "/services/data/v60.0/sobjects/Account" ||
		objectPayload.ObjectDescribe != "/services/data/v60.0/sobjects/Account/describe" ||
		objectPayload.RecentItems != "/services/data/v60.0/sobjects/Account/recent" ||
		objectPayload.URLs["rowTemplate"] != "/services/data/v60.0/sobjects/Account/{ID}" {
		t.Fatalf("object versioned URLs = %#v", objectPayload)
	}

	record := httptest.NewRecorder()
	handler.ServeHTTP(record, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/sobjects/Account/001000000000001", nil))
	if record.Code != http.StatusOK {
		t.Fatalf("record status = %d body=%s", record.Code, record.Body.String())
	}
	var recordPayload map[string]any
	if err := json.Unmarshal(record.Body.Bytes(), &recordPayload); err != nil {
		t.Fatal(err)
	}
	attrs, ok := recordPayload["attributes"].(map[string]any)
	if !ok || attrs["url"] != "/services/data/v60.0/sobjects/Account/001000000000001" {
		t.Fatalf("record attributes = %#v", recordPayload["attributes"])
	}

	recent := httptest.NewRecorder()
	handler.ServeHTTP(recent, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/sobjects/Account/recent", nil))
	if recent.Code != http.StatusOK {
		t.Fatalf("recent status = %d body=%s", recent.Code, recent.Body.String())
	}
	if !bytes.Contains(recent.Body.Bytes(), []byte(`/services/data/v60.0/sobjects/Account/001000000000001`)) || bytes.Contains(recent.Body.Bytes(), []byte(`/services/data/v61.0/sobjects/Account/001000000000001`)) {
		t.Fatalf("recent versioned URLs = %s", recent.Body.String())
	}

	globalRecent := httptest.NewRecorder()
	handler.ServeHTTP(globalRecent, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/recent", nil))
	if globalRecent.Code != http.StatusOK {
		t.Fatalf("global recent status = %d body=%s", globalRecent.Code, globalRecent.Body.String())
	}
	if !bytes.Contains(globalRecent.Body.Bytes(), []byte(`/services/data/v60.0/sobjects/Account/001000000000001`)) || bytes.Contains(globalRecent.Body.Bytes(), []byte(`/services/data/v61.0/sobjects/Account/001000000000001`)) {
		t.Fatalf("global recent versioned URLs = %s", globalRecent.Body.String())
	}

	query := httptest.NewRecorder()
	handler.ServeHTTP(query, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/query?q=SELECT%20Id%20FROM%20Account&batchSize=2", nil))
	if query.Code != http.StatusOK {
		t.Fatalf("query status = %d body=%s", query.Code, query.Body.String())
	}
	var queryPayload struct {
		NextRecordsURL string `json:"nextRecordsUrl"`
	}
	if err := json.Unmarshal(query.Body.Bytes(), &queryPayload); err != nil {
		t.Fatal(err)
	}
	if queryPayload.NextRecordsURL != "/services/data/v60.0/query/oaerql000001-2" {
		t.Fatalf("query nextRecordsUrl = %q", queryPayload.NextRecordsURL)
	}
}

func TestResourceDiscoveryIncludesStableServerEndpoints(t *testing.T) {
	for _, version := range []string{"v61.0", "v60.0"} {
		t.Run(version, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/"+version, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("discovery status = %d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			base := "/services/data/" + version
			want := map[string]string{
				"actions":      base + "/actions",
				"analytics":    base + "/analytics",
				"appMenu":      base + "/appMenu",
				"apps":         base + "/apps",
				"chatter":      base + "/chatter",
				"composite":    base + "/composite",
				"connect":      base + "/connect",
				"jobs":         base + "/jobs",
				"limits":       base + "/limits",
				"metadata":     base + "/metadata",
				"oaer":         base + "/oaer",
				"process":      base + "/process",
				"query":        base + "/query",
				"queryAll":     base + "/queryAll",
				"quickActions": base + "/quickActions",
				"recent":       base + "/recent",
				"search":       base + "/search",
				"sobjects":     base + "/sobjects",
				"support":      base + "/support",
				"tabs":         base + "/tabs",
				"theme":        base + "/theme",
				"tooling":      base + "/tooling",
				"wave":         base + "/wave",
			}
			for name, url := range want {
				if payload[name] != url {
					t.Fatalf("discovery[%s] = %q, want %q; payload=%#v", name, payload[name], url, payload)
				}
			}
		})
	}
}

func TestUnsupportedDiscoveryNamespacesReturnStableErrors(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	for _, tc := range []struct {
		path string
		body string
	}{
		{
			path: "/services/data/v61.0/composite/batch",
			body: `{"batchRequests":[{"method":"GET","url":"/services/data/v61.0/limits"}]}`,
		},
		{
			path: "/services/data/v61.0/composite/tree/Account",
			body: `{"records":[{"attributes":{"referenceId":"AccountRef"},"Name":"Acme"}]}`,
		},
		{
			path: "/services/data/v61.0/composite/graph",
			body: `{"graphs":[{"graphId":"GraphOne","compositeRequest":[{"method":"GET","url":"/services/data/v61.0/limits","referenceId":"LimitsRef"}]}]}`,
		},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s status = %d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) {
			t.Fatalf("%s unsupported shape = %s", tc.path, rec.Body.String())
		}
	}
}

func TestCompositeGenericRouteFamiliesValidateEnvelopesBeforeUnsupported(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, tc := range []struct {
		name    string
		path    string
		body    string
		message string
	}{
		{
			name:    "generic missing compositeRequest",
			path:    "/services/data/v61.0/composite",
			body:    `{}`,
			message: "compositeRequest is required",
		},
		{
			name:    "generic missing reference id",
			path:    "/services/data/v61.0/composite",
			body:    `{"compositeRequest":[{"method":"GET","url":"/services/data/v61.0/limits"}]}`,
			message: "compositeRequest[0].referenceId is required",
		},
		{
			name:    "batch missing url",
			path:    "/services/data/v61.0/composite/batch",
			body:    `{"batchRequests":[{"method":"GET"}]}`,
			message: "batchRequests[0].url is required",
		},
		{
			name:    "tree missing record reference",
			path:    "/services/data/v61.0/composite/tree/Account",
			body:    `{"records":[{"attributes":{},"Name":"Acme"}]}`,
			message: "records[0].attributes.referenceId is required",
		},
		{
			name:    "tree missing object",
			path:    "/services/data/v61.0/composite/tree",
			body:    `{"records":[{"attributes":{"referenceId":"AccountRef"},"Name":"Acme"}]}`,
			message: "object name is required",
		},
		{
			name:    "graph missing graph id",
			path:    "/services/data/v61.0/composite/graph",
			body:    `{"graphs":[{"compositeRequest":[{"method":"GET","url":"/services/data/v61.0/limits","referenceId":"LimitsRef"}]}]}`,
			message: "graphs[0].graphId is required",
		},
		{
			name:    "graph missing subrequest",
			path:    "/services/data/v61.0/composite/graph",
			body:    `{"graphs":[{"graphId":"GraphOne"}]}`,
			message: "graphs[0].compositeRequest is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
			assertSalesforceError(t, rec, http.StatusBadRequest, "REQUIRED_FIELD_MISSING", tc.message)
		})
	}
}

func TestRESTSearchReturnsStableUnsupportedError(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/search?q=FIND%20%7BAcme%7D", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("REST search status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`Search/SOSL is not implemented`)) {
		t.Fatalf("REST search shape = %s", rec.Body.String())
	}
}

func TestRESTSearchRejectsNonGETMethod(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/search?q=FIND%20%7BAcme%7D", strings.NewReader(`{}`)))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("REST search method status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("REST search Allow = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("REST search method shape = %s", rec.Body.String())
	}
}

func TestToolingAndBulkJobsDiscoveryRoutes(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	tooling := httptest.NewRecorder()
	handler.ServeHTTP(tooling, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/tooling", nil))
	if tooling.Code != http.StatusOK {
		t.Fatalf("tooling discovery status = %d body=%s", tooling.Code, tooling.Body.String())
	}
	var toolingPayload map[string]string
	if err := json.Unmarshal(tooling.Body.Bytes(), &toolingPayload); err != nil {
		t.Fatal(err)
	}
	if toolingPayload["executeAnonymous"] != "/services/data/v60.0/tooling/executeAnonymous" || toolingPayload["sobjects"] != "/services/data/v60.0/tooling/sobjects" || toolingPayload["runTestsSynchronous"] != "/services/data/v60.0/tooling/runTestsSynchronous" {
		t.Fatalf("tooling discovery payload = %#v", toolingPayload)
	}
	toolingPost := httptest.NewRecorder()
	handler.ServeHTTP(toolingPost, httptest.NewRequest(http.MethodPost, "/services/data/v60.0/tooling", nil))
	assertSalesforceError(t, toolingPost, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if got := toolingPost.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("tooling Allow = %q", got)
	}

	jobs := httptest.NewRecorder()
	handler.ServeHTTP(jobs, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/jobs", nil))
	if jobs.Code != http.StatusOK {
		t.Fatalf("jobs discovery status = %d body=%s", jobs.Code, jobs.Body.String())
	}
	var jobsPayload map[string]string
	if err := json.Unmarshal(jobs.Body.Bytes(), &jobsPayload); err != nil {
		t.Fatal(err)
	}
	if jobsPayload["query"] != "/services/data/v60.0/jobs/query" || jobsPayload["ingest"] != "/services/data/v60.0/jobs/ingest" {
		t.Fatalf("jobs discovery payload = %#v", jobsPayload)
	}
	jobsPost := httptest.NewRecorder()
	handler.ServeHTTP(jobsPost, httptest.NewRequest(http.MethodPost, "/services/data/v60.0/jobs", nil))
	assertSalesforceError(t, jobsPost, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if got := jobsPost.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("jobs Allow = %q", got)
	}
}

func TestToolingCommonRoutesReturnStableUnsupportedErrors(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	for _, tc := range []struct {
		method  string
		path    string
		message string
	}{
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects", message: "Tooling sObject discovery"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexClass/describe", message: "Tooling sObject describe"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexClass", message: "Tooling ApexClass object collection"},
		{method: http.MethodPost, path: "/services/data/v61.0/tooling/sobjects/ApexClass", message: "Tooling ApexClass object collection"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexClass/01p000000000001", message: "Tooling ApexClass object record"},
		{method: http.MethodPatch, path: "/services/data/v61.0/tooling/sobjects/ApexTrigger/01q000000000001", message: "Tooling ApexTrigger object record"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexLog", message: "Tooling ApexLog object collection"},
		{method: http.MethodPost, path: "/services/data/v61.0/tooling/sobjects/TraceFlag", message: "Tooling TraceFlag object collection"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexTestResult/07M000000000001", message: "Tooling ApexTestResult object record"},
		{method: http.MethodPatch, path: "/services/data/v61.0/tooling/sobjects/ContainerAsyncRequest/1dr000000000001", message: "Tooling ContainerAsyncRequest object record"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ContainerMember", message: "Tooling ContainerMember object collection"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexClassMember", message: "Tooling ApexClassMember object collection"},
		{method: http.MethodPatch, path: "/services/data/v61.0/tooling/sobjects/ApexTriggerMember/01q000000000001", message: "Tooling ApexTriggerMember object record"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexTestRunResult", message: "Tooling ApexTestRunResult object collection"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/sobjects/ApexTestSuite/05F000000000001", message: "Tooling ApexTestSuite object record"},
		{method: http.MethodPost, path: "/services/data/v61.0/tooling/runTestsAsynchronous", message: "Tooling runTestsAsynchronous"},
		{method: http.MethodPost, path: "/services/data/v61.0/tooling/runTestsSynchronous", message: "Tooling runTestsSynchronous"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/coverage", message: "Tooling ApexCodeCoverage"},
		{method: http.MethodGet, path: "/services/data/v61.0/tooling/completions", message: "Tooling completions"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(tc.message)) {
			t.Fatalf("%s %s unsupported shape = %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestToolingTestOrchestrationStubsValidateBodies(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	queue := httptest.NewRecorder()
	handler.ServeHTTP(queue, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ApexTestQueueItem", strings.NewReader(`{"ApexClassId":"01p000000000001"}`)))
	assertSalesforceError(t, queue, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Tooling ApexTestQueueItem object collection")

	missingClass := httptest.NewRecorder()
	handler.ServeHTTP(missingClass, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ApexTestQueueItem", strings.NewReader(`{}`)))
	assertSalesforceError(t, missingClass, http.StatusBadRequest, "REQUIRED_FIELD_MISSING", "ApexTestQueueItem.ApexClassId is required")

	malformedQueue := httptest.NewRecorder()
	handler.ServeHTTP(malformedQueue, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ApexTestQueueItem", strings.NewReader(`{"ApexClassId":`)))
	assertSalesforceError(t, malformedQueue, http.StatusBadRequest, "JSON_PARSER_ERROR", "unexpected EOF")

	runTests := httptest.NewRecorder()
	handler.ServeHTTP(runTests, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/runTestsAsynchronous", strings.NewReader(`{"tests":[{"classId":"01p000000000001","testMethods":["testOne"]}]}`)))
	assertSalesforceError(t, runTests, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Tooling runTestsAsynchronous")

	malformedRunTests := httptest.NewRecorder()
	handler.ServeHTTP(malformedRunTests, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/runTestsSynchronous", strings.NewReader(`{"tests":`)))
	assertSalesforceError(t, malformedRunTests, http.StatusBadRequest, "JSON_PARSER_ERROR", "unexpected EOF")
}

func TestToolingDeployChainMemberStubsValidateBodies(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	member := httptest.NewRecorder()
	handler.ServeHTTP(member, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ApexClassMember", strings.NewReader(`{"MetadataContainerId":"1dc000000000001","ContentEntityId":"01p000000000001","Body":"public class LocalClass {}"}`)))
	assertSalesforceError(t, member, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Tooling ApexClassMember object collection")

	missingContainer := httptest.NewRecorder()
	handler.ServeHTTP(missingContainer, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ContainerMember", strings.NewReader(`{"ContentEntityId":"01p000000000001"}`)))
	assertSalesforceError(t, missingContainer, http.StatusBadRequest, "REQUIRED_FIELD_MISSING", "ContainerMember.MetadataContainerId is required")

	malformedMember := httptest.NewRecorder()
	handler.ServeHTTP(malformedMember, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ApexTriggerMember", strings.NewReader(`{"MetadataContainerId":`)))
	assertSalesforceError(t, malformedMember, http.StatusBadRequest, "JSON_PARSER_ERROR", "unexpected EOF")

	record := httptest.NewRecorder()
	handler.ServeHTTP(record, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/tooling/sobjects/ApexPageMember/066000000000001", strings.NewReader(`{"Body":"<apex:page/>"}`)))
	assertSalesforceError(t, record, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Tooling ApexPageMember object record")
}

func TestToolingQueryStillDelegatesToSOQL(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Tooling Query")
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/tooling/query?q=SELECT%20Id,%20Name%20FROM%20Account%20WHERE%20Name%20=%20'Tooling%20Query'",
		"/services/data/v61.0/tooling/queryAll?q=SELECT%20Id,%20Name%20FROM%20Account%20WHERE%20Name%20=%20'Tooling%20Query'",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"totalSize":1`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`Tooling Query`)) {
			t.Fatalf("%s body = %s", path, rec.Body.String())
		}
		var payload struct {
			Records []map[string]any `json:"records"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Records) != 1 || payload.Records[0]["Name"] != "Tooling Query" {
			t.Fatalf("%s records = %#v", path, payload.Records)
		}
		assertQueryRecordShape(t, payload.Records[0], "Account", "001000000000001", "/services/data/v61.0/sobjects/Account/001000000000001")
	}
}

func TestToolingQueryContinuationUsesToolingPath(t *testing.T) {
	for _, endpoint := range []string{"query", "queryAll"} {
		t.Run(endpoint, func(t *testing.T) {
			org := testOrg()
			addAccountForTest(&org, "001000000000001", "Tooling Page 1")
			addAccountForTest(&org, "001000000000002", "Tooling Page 2")
			handler := New(&org)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v62.0/tooling/"+endpoint+"?q=SELECT%20Id,%20Name%20FROM%20Account&batchSize=1", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("tooling %s pagination status = %d body=%s", endpoint, rec.Code, rec.Body.String())
			}
			var firstPayload struct {
				TotalSize      int                      `json:"totalSize"`
				Done           bool                     `json:"done"`
				NextRecordsURL string                   `json:"nextRecordsUrl"`
				Records        []map[string]interface{} `json:"records"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &firstPayload); err != nil {
				t.Fatal(err)
			}
			if firstPayload.TotalSize != 2 || firstPayload.Done || len(firstPayload.Records) != 1 {
				t.Fatalf("tooling %s first payload = %#v", endpoint, firstPayload)
			}
			if firstPayload.NextRecordsURL != "/services/data/v62.0/tooling/query/oaerql000001-1" {
				t.Fatalf("tooling %s nextRecordsUrl = %q", endpoint, firstPayload.NextRecordsURL)
			}

			disallowed := httptest.NewRecorder()
			handler.ServeHTTP(disallowed, httptest.NewRequest(http.MethodPost, firstPayload.NextRecordsURL, nil))
			if disallowed.Code != http.StatusMethodNotAllowed || disallowed.Header().Get("Allow") != http.MethodGet {
				t.Fatalf("tooling %s queryMore method boundary status=%d allow=%q body=%s", endpoint, disallowed.Code, disallowed.Header().Get("Allow"), disallowed.Body.String())
			}

			continuation := httptest.NewRecorder()
			handler.ServeHTTP(continuation, httptest.NewRequest(http.MethodGet, firstPayload.NextRecordsURL, nil))
			if continuation.Code != http.StatusOK {
				t.Fatalf("tooling %s queryMore status = %d body=%s", endpoint, continuation.Code, continuation.Body.String())
			}
			var nextPayload struct {
				TotalSize      int                      `json:"totalSize"`
				Done           bool                     `json:"done"`
				NextRecordsURL string                   `json:"nextRecordsUrl"`
				Records        []map[string]interface{} `json:"records"`
			}
			if err := json.Unmarshal(continuation.Body.Bytes(), &nextPayload); err != nil {
				t.Fatal(err)
			}
			if nextPayload.TotalSize != 2 || !nextPayload.Done || nextPayload.NextRecordsURL != "" || len(nextPayload.Records) != 1 {
				t.Fatalf("tooling %s continuation payload = %#v", endpoint, nextPayload)
			}
			if !bytes.Contains(continuation.Body.Bytes(), []byte(`"Name":"Tooling Page 2"`)) {
				t.Fatalf("tooling %s continuation body = %s", endpoint, continuation.Body.String())
			}
		})
	}
}

func TestToolingSearchReturnsStableUnsupportedError(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/tooling/search?q=FIND%20%7BAcme%7D", nil))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("tooling search status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`Tooling search is not implemented`)) {
		t.Fatalf("tooling search shape = %s", rec.Body.String())
	}
}

func TestToolingUnknownRouteStillReturnsNotFound(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/tooling/not-a-real-route",
		"/services/data/v61.0/tooling/sobjects/NoSuchToolingObject",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s unknown tooling status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"NOT_FOUND"`)) {
			t.Fatalf("%s unknown tooling shape = %s", path, rec.Body.String())
		}
	}
}

func TestToolingCommonRoutesMethodHandling(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "tooling completions", path: "/services/data/v61.0/tooling/completions"},
		{name: "tooling queryAll", path: "/services/data/v61.0/tooling/queryAll"},
		{name: "tooling search", path: "/services/data/v61.0/tooling/search"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tc.path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s method status = %d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s Allow = %q", tc.name, got)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
			t.Fatalf("%s method shape = %s", tc.name, rec.Body.String())
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/services/data/v61.0/tooling/sobjects/ApexClass/01p000000000001", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("tooling metadata record method status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != "GET, PATCH, DELETE" {
		t.Fatalf("tooling metadata record Allow = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("tooling metadata record method shape = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/services/data/v61.0/tooling/sobjects/ApexLog", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("tooling ApexLog collection method status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("tooling ApexLog collection Allow = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("tooling ApexLog collection method shape = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/sobjects/ApexTestResult", strings.NewReader(`{}`)))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("tooling ApexTestResult collection method status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("tooling ApexTestResult collection Allow = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("tooling ApexTestResult collection method shape = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/tooling/sobjects/ApexTestResult/07M000000000001", strings.NewReader(`{}`)))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("tooling ApexTestResult record method status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("tooling ApexTestResult record Allow = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("tooling ApexTestResult record method shape = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/tooling/runTestsSynchronous", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("tooling runTestsSynchronous method status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("tooling runTestsSynchronous Allow = %q", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
		t.Fatalf("tooling runTestsSynchronous method shape = %s", rec.Body.String())
	}
}

func TestBulkAPIJobsReturnStableUnsupportedErrors(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	for _, tc := range []struct {
		method  string
		path    string
		message string
	}{
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/query", message: "Bulk API v2 query jobs"},
		{method: http.MethodPost, path: "/services/data/v61.0/jobs/query", message: "Bulk API v2 query jobs"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/query/750000000000001", message: "Bulk API v2 query job records"},
		{method: http.MethodPatch, path: "/services/data/v61.0/jobs/query/750000000000001", message: "Bulk API v2 query job records"},
		{method: http.MethodDelete, path: "/services/data/v61.0/jobs/query/750000000000001", message: "Bulk API v2 query job records"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/query/750000000000001/results", message: "Bulk API v2 query job results"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/ingest", message: "Bulk API v2 ingest jobs"},
		{method: http.MethodPost, path: "/services/data/v61.0/jobs/ingest", message: "Bulk API v2 ingest jobs"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/ingest/750000000000001", message: "Bulk API v2 ingest job records"},
		{method: http.MethodPatch, path: "/services/data/v61.0/jobs/ingest/750000000000001", message: "Bulk API v2 ingest job records"},
		{method: http.MethodDelete, path: "/services/data/v61.0/jobs/ingest/750000000000001", message: "Bulk API v2 ingest job records"},
		{method: http.MethodPut, path: "/services/data/v61.0/jobs/ingest/750000000000001/batches", message: "Bulk API v2 ingest job batches"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/ingest/750000000000001/successfulResults", message: "Bulk API v2 ingest successful results"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/ingest/750000000000001/failedResults", message: "Bulk API v2 ingest failed results"},
		{method: http.MethodGet, path: "/services/data/v61.0/jobs/ingest/750000000000001/unprocessedrecords", message: "Bulk API v2 ingest unprocessed records"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(tc.message)) {
			t.Fatalf("%s %s unsupported shape = %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestBulkAPIJobsRejectDisallowedMethodsWithAllowHeader(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	for _, tc := range []struct {
		name   string
		method string
		path   string
		allow  string
	}{
		{name: "query collection rejects patch", method: http.MethodPatch, path: "/services/data/v61.0/jobs/query", allow: "GET, POST"},
		{name: "query results reject post", method: http.MethodPost, path: "/services/data/v61.0/jobs/query/750000000000001/results", allow: "GET"},
		{name: "ingest collection rejects patch", method: http.MethodPatch, path: "/services/data/v61.0/jobs/ingest", allow: "GET, POST"},
		{name: "ingest batches are put-only", method: http.MethodGet, path: "/services/data/v61.0/jobs/ingest/750000000000001/batches", allow: "PUT"},
		{name: "ingest failed results reject delete", method: http.MethodDelete, path: "/services/data/v61.0/jobs/ingest/750000000000001/failedResults", allow: "GET"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Fatalf("%s %s Allow = %q, want %q", tc.method, tc.path, got, tc.allow)
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"METHOD_NOT_ALLOWED"`)) {
				t.Fatalf("%s %s method shape = %s", tc.method, tc.path, rec.Body.String())
			}
		})
	}
}

func TestBulkAPIJobMutatorsValidateJSONBodiesBeforeUnsupported(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "query create", method: http.MethodPost, path: "/services/data/v61.0/jobs/query"},
		{name: "query update", method: http.MethodPatch, path: "/services/data/v61.0/jobs/query/750000000000001"},
		{name: "ingest create", method: http.MethodPost, path: "/services/data/v61.0/jobs/ingest"},
		{name: "ingest update", method: http.MethodPatch, path: "/services/data/v61.0/jobs/ingest/750000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			malformed := httptest.NewRecorder()
			handler.ServeHTTP(malformed, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"operation":`)))
			assertSalesforceError(t, malformed, http.StatusBadRequest, "JSON_PARSER_ERROR", "unexpected EOF")

			wellFormed := httptest.NewRecorder()
			handler.ServeHTTP(wellFormed, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"operation":"query"}`)))
			assertSalesforceError(t, wellFormed, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Bulk API v2")
		})
	}
}

func TestCommonRESTNamespaceStubsReturnStableUnsupportedErrors(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	for _, tc := range []struct {
		method  string
		path    string
		message string
	}{
		{method: http.MethodGet, path: "/services/data/v61.0/connect", message: "Connect REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/chatter/feed-elements", message: "Chatter REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/analytics/reports", message: "Analytics REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/wave", message: "Wave REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/metadata", message: "Metadata REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/support", message: "Support REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/process/approvals", message: "Process REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/actions/standard/emailSimple", message: "Actions REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/apps", message: "Apps REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/appMenu", message: "AppMenu REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/appMenu/AppSwitcher", message: "AppMenu REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/tabs", message: "Tabs REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/theme/brand", message: "Theme REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/quickActions", message: "QuickActions REST namespace"},
		{method: http.MethodGet, path: "/services/data/v61.0/quickActions/Account/NewTask", message: "QuickActions REST namespace"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Allow") != "" {
			t.Fatalf("%s %s unexpected Allow = %q", tc.method, tc.path, rec.Header().Get("Allow"))
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(tc.message)) {
			t.Fatalf("%s %s unsupported shape = %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

func TestMetadataRESTDeployRoutesReturnExplicitUnsupportedBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		wantStatus    int
		wantCode      string
		wantAllow     string
		wantMessageIn string
	}{
		{
			name:          "root unsupported",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/metadata",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Metadata REST namespace",
		},
		{
			name:          "deploy create unsupported",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/metadata/deployRequest",
			body:          `{"deployOptions":{"checkOnly":true}}`,
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Metadata REST deploy requests",
		},
		{
			name:          "deploy create malformed json",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/metadata/deployRequest",
			body:          `{"deployOptions":`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "JSON_PARSER_ERROR",
			wantMessageIn: "unexpected EOF",
		},
		{
			name:          "deploy create method boundary",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/metadata/deployRequest",
			wantStatus:    http.StatusMethodNotAllowed,
			wantCode:      "METHOD_NOT_ALLOWED",
			wantAllow:     http.MethodPost,
			wantMessageIn: "method not allowed",
		},
		{
			name:          "deploy status unsupported",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/metadata/deployRequest/0Af000000000001",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Metadata REST deploy status",
		},
		{
			name:          "deploy results unsupported",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/metadata/deployRequest/0Af000000000001/results",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Metadata REST deploy results retrieval",
		},
		{
			name:          "deploy details unsupported",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/metadata/deployRequest/0Af000000000001/deployDetails",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Metadata REST deploy details retrieval",
		},
		{
			name:          "deploy results method boundary",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/metadata/deployRequest/0Af000000000001/results",
			body:          `{}`,
			wantStatus:    http.StatusMethodNotAllowed,
			wantCode:      "METHOD_NOT_ALLOWED",
			wantAllow:     http.MethodGet,
			wantMessageIn: "method not allowed",
		},
		{
			name:       "unknown metadata endpoint",
			method:     http.MethodGet,
			path:       "/services/data/v61.0/metadata/unknown",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			assertSalesforceError(t, rec, tt.wantStatus, tt.wantCode, tt.wantMessageIn)
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Fatalf("Allow = %q, want %q", got, tt.wantAllow)
			}
		})
	}
}

func TestApexRestDispatchReturnsStableUnsupportedError(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{"/services/apexrest", "/services/apexrest/widgets/42"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
			t.Run(method+" "+path, func(t *testing.T) {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(`{"name":"Acme"}`)))
				assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Apex @RestResource dispatch is not implemented in the local server")
				if rec.Header().Get("Allow") != "" {
					t.Fatalf("unexpected Allow = %q", rec.Header().Get("Allow"))
				}
			})
		}
	}
}

func TestApexRestDispatchRejectsUnsupportedMethodsWithAllowHeader(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/services/apexrest/widgets/42", nil))
	assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	if got, want := rec.Header().Get("Allow"), "GET, POST, PATCH, PUT, DELETE"; got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func TestApexRestNearbyUnknownEndpointUnchanged(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/apexrestish/widgets/42", nil))
	assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "unknown endpoint")
	if rec.Header().Get("Allow") != "" {
		t.Fatalf("unexpected Allow = %q", rec.Header().Get("Allow"))
	}
}

func TestOAERFixtureAndResetEndpointsPersist(t *testing.T) {
	org := testOrg()
	store := &memoryStore{}
	handler := NewWithStore(&org, store)

	seed := httptest.NewRecorder()
	handler.ServeHTTP(seed, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/fixture", strings.NewReader(`{
  "version":"oaer.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)))
	if seed.Code != http.StatusOK {
		t.Fatalf("seed status = %d body=%s", seed.Code, seed.Body.String())
	}
	if store.saves != 1 || len(store.last.Objects["Account"].Records) != 1 {
		t.Fatalf("store after seed = %#v saves=%d", store.last, store.saves)
	}

	exported := httptest.NewRecorder()
	handler.ServeHTTP(exported, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/oaer/fixture", nil))
	if exported.Code != http.StatusOK || !bytes.Contains(exported.Body.Bytes(), []byte(`"Acme"`)) {
		t.Fatalf("export status = %d body=%s", exported.Code, exported.Body.String())
	}

	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset", nil))
	if reset.Code != http.StatusOK {
		t.Fatalf("reset status = %d body=%s", reset.Code, reset.Body.String())
	}
	if len(org.Objects["Account"].Records) != 0 || len(org.Objects["User"].Records) != 1 {
		t.Fatalf("org after reset = %#v", storage.InspectOrg("", org))
	}
}

func TestOAERScopedResetEndpoints(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Account"].Records["001000000000001"] = storage.Record{
		ID: "001000000000001",
		Fields: map[string]storage.Value{
			"Id":   storage.StringValue("001000000000001"),
			"Name": storage.StringValue("Acme"),
		},
	}
	userObject := org.Objects["User"]
	userObject.Records["005000000000999"] = storage.Record{
		ID: "005000000000999",
		Fields: map[string]storage.Value{
			"Id":    storage.StringValue("005000000000999"),
			"Alias": storage.StringValue("extra"),
		},
	}
	org.Objects["User"] = userObject
	handler := New(&org)

	resetData := httptest.NewRecorder()
	handler.ServeHTTP(resetData, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset/data", nil))
	if resetData.Code != http.StatusOK {
		t.Fatalf("reset data status = %d body=%s", resetData.Code, resetData.Body.String())
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("account records after data reset = %d", got)
	}
	if got := len(org.Objects["User"].Records); got != 2 {
		t.Fatalf("user records after data reset = %d", got)
	}

	resetUsers := httptest.NewRecorder()
	handler.ServeHTTP(resetUsers, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset", strings.NewReader(`{"scopes":["users","limits","async"]}`)))
	if resetUsers.Code != http.StatusOK {
		t.Fatalf("reset users status = %d body=%s", resetUsers.Code, resetUsers.Body.String())
	}
	if got := len(org.Objects["User"].Records); got != 1 {
		t.Fatalf("user records after users reset = %d", got)
	}
	if !bytes.Contains(resetUsers.Body.Bytes(), []byte(`"scopes":["users","limits","async"]`)) {
		t.Fatalf("reset users response missing scopes: %s", resetUsers.Body.String())
	}
	var usersPayload struct {
		NoOpScopes []string `json:"noOpScopes"`
	}
	if err := json.Unmarshal(resetUsers.Body.Bytes(), &usersPayload); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(usersPayload.NoOpScopes, ","), "limits,async"; got != want {
		t.Fatalf("no-op scopes = %q, want %q", got, want)
	}
}

func TestOAERStateEndpointReportsSummaryAndResetSupport(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Account"].Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Id":   storage.StringValue("001000000000001"),
			"Name": storage.StringValue("Acme"),
		},
	}
	handler := New(&org)

	state := httptest.NewRecorder()
	handler.ServeHTTP(state, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/oaer/state", nil))
	if state.Code != http.StatusOK {
		t.Fatalf("state status = %d body=%s", state.Code, state.Body.String())
	}
	var payload struct {
		LocalOnly   bool                   `json:"localOnly"`
		Summary     storage.InspectSummary `json:"summary"`
		ResetScopes []resetScopeInfo       `json:"resetScopes"`
	}
	if err := json.Unmarshal(state.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.LocalOnly || payload.Summary.ByObject["Account"] != 1 || payload.Summary.Users != 1 {
		t.Fatalf("state payload = %#v", payload)
	}
	if len(payload.ResetScopes) != 6 {
		t.Fatalf("reset scope support = %#v", payload.ResetScopes)
	}
	for _, scope := range payload.ResetScopes {
		if (scope.Name == "limits" || scope.Name == "async") && !scope.NoOp {
			t.Fatalf("scope %q should report no-op: %#v", scope.Name, payload.ResetScopes)
		}
	}
	inspect := httptest.NewRecorder()
	handler.ServeHTTP(inspect, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/oaer/inspect", nil))
	if inspect.Code != http.StatusOK || !bytes.Contains(inspect.Body.Bytes(), []byte(`"resetScopes"`)) {
		t.Fatalf("inspect status = %d body=%s", inspect.Code, inspect.Body.String())
	}
}

func TestOAERResetScopesPreserveAndClearExpectedState(t *testing.T) {
	tests := []struct {
		name         string
		scope        string
		wantAccounts int
		wantUsers    int
		wantProfiles int
		wantNoOps    string
	}{
		{name: "data", scope: "data", wantAccounts: 0, wantUsers: 2, wantProfiles: 1},
		{name: "users", scope: "users", wantAccounts: 1, wantUsers: 1, wantProfiles: 1},
		{name: "platform", scope: "platform", wantAccounts: 1, wantUsers: 1, wantProfiles: 1},
		{name: "all", scope: "all", wantAccounts: 0, wantUsers: 1, wantProfiles: 1},
		{name: "limits async", scope: "limits,async", wantAccounts: 1, wantUsers: 2, wantProfiles: 1, wantNoOps: "limits,async"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := resetScopeTestOrg()
			handler := New(&org)

			reset := httptest.NewRecorder()
			handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset?scope="+tt.scope, nil))
			if reset.Code != http.StatusOK {
				t.Fatalf("reset status = %d body=%s", reset.Code, reset.Body.String())
			}
			summary := storage.InspectOrg("", org)
			if summary.ByObject["Account"] != tt.wantAccounts || summary.Users != tt.wantUsers || summary.Profiles != tt.wantProfiles {
				t.Fatalf("summary = %#v", summary)
			}
			var payload struct {
				NoOpScopes []string `json:"noOpScopes"`
			}
			if err := json.Unmarshal(reset.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(payload.NoOpScopes, ","); got != tt.wantNoOps {
				t.Fatalf("no-op scopes = %q, want %q", got, tt.wantNoOps)
			}
		})
	}
}

func TestOAERScopedResetRejectsUnknownScope(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset/nope", nil))
	if reset.Code != http.StatusBadRequest {
		t.Fatalf("reset status = %d body=%s", reset.Code, reset.Body.String())
	}
}

func TestOAERScopedResetDeduplicatesAndAllWins(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Account"].Records["001000000000001"] = storage.Record{
		ID: "001000000000001",
		Fields: map[string]storage.Value{
			"Id":   storage.StringValue("001000000000001"),
			"Name": storage.StringValue("Acme"),
		},
	}
	handler := New(&org)

	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset?scope=all,data,data", nil))
	if reset.Code != http.StatusOK {
		t.Fatalf("reset status = %d body=%s", reset.Code, reset.Body.String())
	}
	if !bytes.Contains(reset.Body.Bytes(), []byte(`"scopes":["all","data"]`)) {
		t.Fatalf("reset response missing deduplicated scopes: %s", reset.Body.String())
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("account records after reset = %d", got)
	}
	if got := len(org.Objects["User"].Records); got != 1 {
		t.Fatalf("user records after reset = %d", got)
	}
}

func TestOAERFixtureAndResetDoNotMutateOrgOnStoreFailure(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		org := testOrg()
		handler := NewWithStore(&org, &failingStore{})

		seed := httptest.NewRecorder()
		handler.ServeHTTP(seed, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/fixture", strings.NewReader(`{
  "version":"oaer.storage.v1",
  "objects":[{"name":"Account","records":[{"alias":"acme","fields":{"Name":{"kind":"string","string":"Acme"}}}]}]
}`)))
		assertSalesforceError(t, seed, http.StatusInternalServerError, "SERVER_ERROR", "store failed")
		if got := len(org.Objects["Account"].Records); got != 0 {
			t.Fatalf("account records after failed fixture load = %d", got)
		}
	})
	t.Run("reset", func(t *testing.T) {
		org := resetScopeTestOrg()
		handler := NewWithStore(&org, &failingStore{})

		reset := httptest.NewRecorder()
		handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset/data", nil))
		assertSalesforceError(t, reset, http.StatusInternalServerError, "SERVER_ERROR", "store failed")
		summary := storage.InspectOrg("", org)
		if summary.ByObject["Account"] != 1 || summary.Users != 2 {
			t.Fatalf("summary after failed reset = %#v", summary)
		}
	})
}

func TestIdentityLimitsDescribeRecentAndNormalRESTPayloads(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"Acme"}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}

	identity := httptest.NewRecorder()
	handler.ServeHTTP(identity, httptest.NewRequest(http.MethodGet, "/id/00D000000000001/005000000000001", nil))
	if identity.Code != http.StatusOK || !bytes.Contains(identity.Body.Bytes(), []byte(`"user_id"`)) {
		t.Fatalf("identity status = %d body=%s", identity.Code, identity.Body.String())
	}

	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil))
	if userinfo.Code != http.StatusOK || !bytes.Contains(userinfo.Body.Bytes(), []byte(`"preferred_username"`)) {
		t.Fatalf("userinfo status = %d body=%s", userinfo.Code, userinfo.Body.String())
	}

	limits := httptest.NewRecorder()
	handler.ServeHTTP(limits, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/limits", nil))
	if limits.Code != http.StatusOK || !bytes.Contains(limits.Body.Bytes(), []byte(`DailyApiRequests`)) {
		t.Fatalf("limits status = %d body=%s", limits.Code, limits.Body.String())
	}

	describe := httptest.NewRecorder()
	handler.ServeHTTP(describe, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/describe", nil))
	if describe.Code != http.StatusOK || !bytes.Contains(describe.Body.Bytes(), []byte(`"fields"`)) {
		t.Fatalf("describe status = %d body=%s", describe.Code, describe.Body.String())
	}

	recent := httptest.NewRecorder()
	handler.ServeHTTP(recent, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/recent", nil))
	if recent.Code != http.StatusOK || !bytes.Contains(recent.Body.Bytes(), []byte(`"Acme"`)) {
		t.Fatalf("recent status = %d body=%s", recent.Code, recent.Body.String())
	}
}

func TestLimitsPayloadIncludesCommonStableKeys(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/limits", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("limits status = %d body=%s", res.Code, res.Body.String())
	}

	var payload map[string]localAPILimit
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{
		"DailyApiRequests",
		"DailyAsyncApexExecutions",
		"ConcurrentAsyncGetReportInstances",
		"ConcurrentSyncReportRuns",
		"DailyBulkApiBatches",
		"DailyBulkV2QueryJobs",
		"DailyBulkV2IngestJobs",
		"DailyDurableGenericStreamingApiEvents",
		"DailyStreamingApiEvents",
		"DataStorageMB",
		"FileStorageMB",
		"HourlyAsyncReportRuns",
		"HourlyDashboardRefreshes",
		"HourlyDashboardResults",
		"HourlyDashboardStatuses",
		"HourlyLongTermIdMapping",
		"HourlyODataCallout",
		"HourlyPublishedPlatformEvents",
		"HourlyPublishedStandardVolumePlatformEvents",
		"MassEmail",
		"Package2VersionCreates",
		"PermissionSets",
		"SingleEmail",
		"StreamingApiConcurrentClients",
	} {
		limit, ok := payload[key]
		if !ok {
			t.Fatalf("limits payload missing %s in %#v", key, payload)
		}
		if limit.Max <= 0 || limit.Remaining != limit.Max {
			t.Fatalf("limit %s = %#v, want positive Max with Remaining == Max", key, limit)
		}
	}

	if payload["DataStorageMB"].Max != 512 || payload["StreamingApiConcurrentClients"].Max != 20 {
		t.Fatalf("representative limits changed: DataStorageMB=%#v StreamingApiConcurrentClients=%#v", payload["DataStorageMB"], payload["StreamingApiConcurrentClients"])
	}
}

func TestLocalUserContextDefaultsToDeterministicUser(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000009", "other@example.test", "other@example.test", "Other User")
	handler := New(&org)

	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil))
	if userinfo.Code != http.StatusOK {
		t.Fatalf("userinfo status = %d body=%s", userinfo.Code, userinfo.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(userinfo.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["user_id"] != "005000000000001" || payload["preferred_username"] != "system@example.invalid" {
		t.Fatalf("userinfo payload = %#v", payload)
	}
}

func TestLocalUserContextAuthorizationBearerSelectsUser(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "ada@example.test", "ada.alias@example.test", "Ada Trail")
	handler := New(&org)

	req := httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer 005000000000123")
	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, req)
	if userinfo.Code != http.StatusOK {
		t.Fatalf("userinfo status = %d body=%s", userinfo.Code, userinfo.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(userinfo.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["user_id"] != "005000000000123" || payload["preferred_username"] != "ada@example.test" || payload["email"] != "ada.alias@example.test" || payload["name"] != "Ada Trail" {
		t.Fatalf("userinfo payload = %#v", payload)
	}
}

func TestLocalUserContextHeaderSelectsUserForIdentity(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "ada@example.test", "ada.alias@example.test", "Ada Trail")
	handler := New(&org)

	req := httptest.NewRequest(http.MethodGet, "/id/00D000000000001/005000000000001", nil)
	req.Header.Set("X-OAER-User-Id", "005000000000123")
	identity := httptest.NewRecorder()
	handler.ServeHTTP(identity, req)
	if identity.Code != http.StatusOK {
		t.Fatalf("identity status = %d body=%s", identity.Code, identity.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(identity.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["user_id"] != "005000000000123" || payload["username"] != "ada@example.test" || payload["display_name"] != "Ada Trail" {
		t.Fatalf("identity payload = %#v", payload)
	}
}

func TestLocalUserContextUnknownHeaderFallsBackToDeterministicDefault(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "ada@example.test", "ada.alias@example.test", "Ada Trail")
	handler := New(&org)

	req := httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil)
	req.Header.Set("X-OAER-User-Id", "005999999999999")
	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, req)
	if userinfo.Code != http.StatusOK {
		t.Fatalf("userinfo status = %d body=%s", userinfo.Code, userinfo.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(userinfo.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["user_id"] != "005000000000001" {
		t.Fatalf("userinfo payload = %#v", payload)
	}
}

func TestLocalUserContextUsesLexicographicUserWhenDefaultMissing(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	userObject := org.Objects["User"]
	delete(userObject.Records, "005000000000001")
	userObject.Records["005000000000200"] = userRecordForTest("005000000000200", "zulu@example.test", "zulu@example.test", "Zulu User")
	userObject.Records["005000000000100"] = userRecordForTest("005000000000100", "alpha@example.test", "alpha@example.test", "Alpha User")
	org.Objects["User"] = userObject
	handler := New(&org)

	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil))
	if userinfo.Code != http.StatusOK {
		t.Fatalf("userinfo status = %d body=%s", userinfo.Code, userinfo.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(userinfo.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["user_id"] != "005000000000100" || payload["preferred_username"] != "alpha@example.test" {
		t.Fatalf("userinfo payload = %#v", payload)
	}
}

func TestIdentityAndUserInfoExposeConservativeOpenIDShape(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addUser(&org, "005000000000123", "ada@example.test", "ada.alias@example.test", "Ada Trail")
	handler := New(&org)

	req := httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer 005000000000123")
	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, req)
	if userinfo.Code != http.StatusOK {
		t.Fatalf("userinfo status = %d body=%s", userinfo.Code, userinfo.Body.String())
	}
	var userPayload map[string]any
	if err := json.Unmarshal(userinfo.Body.Bytes(), &userPayload); err != nil {
		t.Fatal(err)
	}
	if userPayload["sub"] != "005000000000123" || userPayload["profile"] == "" || userPayload["email_verified"] != true || userPayload["zoneinfo"] != "UTC" || userPayload["locale"] != "en_US" {
		t.Fatalf("userinfo OpenID payload = %#v", userPayload)
	}
	if urls, ok := userPayload["urls"].(map[string]any); !ok || urls["rest"] == "" || urls["query"] == "" {
		t.Fatalf("userinfo urls = %#v", userPayload["urls"])
	}

	identityReq := httptest.NewRequest(http.MethodGet, "/id/00D000000000001/005000000000123", nil)
	identity := httptest.NewRecorder()
	handler.ServeHTTP(identity, identityReq)
	if identity.Code != http.StatusOK {
		t.Fatalf("identity status = %d body=%s", identity.Code, identity.Body.String())
	}
	var identityPayload map[string]any
	if err := json.Unmarshal(identity.Body.Bytes(), &identityPayload); err != nil {
		t.Fatal(err)
	}
	urls, ok := identityPayload["urls"].(map[string]any)
	if !ok || urls["enterprise"] == "" || urls["sobjects"] == "" || identityPayload["active"] != true {
		t.Fatalf("identity payload = %#v", identityPayload)
	}
}

func TestOAuthUnsupportedStubsAreExplicit(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	handler := New(&org)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "token", method: http.MethodPost, path: "/services/oauth2/token"},
		{name: "revoke", method: http.MethodPost, path: "/services/oauth2/revoke"},
		{name: "introspect", method: http.MethodPost, path: "/services/oauth2/introspect"},
		{name: "authorize", method: http.MethodGet, path: "/services/oauth2/authorize?response_type=code&client_id=local&redirect_uri=http://localhost/callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "Full OAuth flows and token issuance are not implemented")
		})
	}
}

func TestOAuthUnsupportedStubsMethodHandling(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	handler := New(&org)

	for _, tc := range []struct {
		name        string
		method      string
		path        string
		allowHeader string
	}{
		{name: "token-get", method: http.MethodGet, path: "/services/oauth2/token", allowHeader: http.MethodPost},
		{name: "revoke-get", method: http.MethodGet, path: "/services/oauth2/revoke", allowHeader: http.MethodPost},
		{name: "introspect-get", method: http.MethodGet, path: "/services/oauth2/introspect", allowHeader: http.MethodPost},
		{name: "authorize-post", method: http.MethodPost, path: "/services/oauth2/authorize", allowHeader: http.MethodGet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			assertSalesforceError(t, rec, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			if got := rec.Header().Get("Allow"); got != tc.allowHeader {
				t.Fatalf("Allow = %q, want %q", got, tc.allowHeader)
			}
		})
	}
}

func TestToolingExecuteAnonymousAndCompositeSObjects(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	exec := httptest.NewRecorder()
	handler.ServeHTTP(exec, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"insert new Account(Name = 'From Apex'); System.debug('ok');"}`)))
	if exec.Code != http.StatusOK || !bytes.Contains(exec.Body.Bytes(), []byte(`"success":true`)) || !bytes.Contains(exec.Body.Bytes(), []byte(`"ok"`)) {
		t.Fatalf("executeAnonymous status = %d body=%s", exec.Code, exec.Body.String())
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("records after executeAnonymous = %#v", org.Objects["Account"].Records)
	}

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"type":"Account"},"Name":"Composite"}
  ]
}`)))
	if composite.Code != http.StatusOK || !bytes.Contains(composite.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
}

func TestCompositeSObjectsEchoesReferenceIDAndPreservesOrder(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "records": [
    {"attributes":{"type":"Account","referenceId":"first"},"Name":"First"},
    {"attributes":{"type":"Account","referenceId":"second"},"Name":"Second"}
  ]
}`)))
	if composite.Code != http.StatusOK {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(composite.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	if results[0]["referenceId"] != "first" || results[0]["id"] != "001000000000001" {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[1]["referenceId"] != "second" || results[1]["id"] != "001000000000002" {
		t.Fatalf("second result = %#v", results[1])
	}
}

func TestCompositeSObjectsValidatesEdgeEnvelopes(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		wantStatus    int
		wantCode      string
		wantMessageIn string
	}{
		{
			name:          "generic malformed",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/composite",
			body:          `{"compositeRequest":`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "JSON_PARSER_ERROR",
			wantMessageIn: "unexpected EOF",
		},
		{
			name:          "batch malformed",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/composite/batch",
			body:          `{"batchRequests":`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "JSON_PARSER_ERROR",
			wantMessageIn: "unexpected EOF",
		},
		{
			name:          "missing records",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/composite/sobjects",
			body:          `{"allOrNone":true}`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "REQUIRED_FIELD_MISSING",
			wantMessageIn: "records is required",
		},
		{
			name:          "empty records",
			method:        http.MethodPatch,
			path:          "/services/data/v61.0/composite/sobjects",
			body:          `{"records":[]}`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "REQUIRED_FIELD_MISSING",
			wantMessageIn: "records is required",
		},
		{
			name:          "malformed attributes",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/composite/sobjects",
			body:          `{"records":[{"attributes":"Account","Name":"Bad"}]}`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "JSON_PARSER_ERROR",
			wantMessageIn: "attributes must be a JSON object",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			assertSalesforceError(t, rec, tt.wantStatus, tt.wantCode, tt.wantMessageIn)
		})
	}
}

func TestCompositeSObjectsPartialFailureCommitsSuccessWithRowError(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "records": [
    {"attributes":{"type":"Account","referenceId":"good"},"Name":"Good"},
    {"attributes":{"type":"Account","referenceId":"bad"}}
  ]
}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("partial composite status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := len(org.Objects["Account"].Records); got != 1 {
		t.Fatalf("partial composite committed records = %d", got)
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["success"] != true || rows[1]["success"] != false {
		t.Fatalf("partial composite rows = %#v", rows)
	}
	errors, ok := rows[1]["errors"].([]any)
	if !ok || len(errors) != 1 {
		t.Fatalf("partial composite errors = %#v", rows[1]["errors"])
	}
	firstError, ok := errors[0].(map[string]any)
	if !ok || firstError["statusCode"] != "REQUIRED_FIELD_MISSING" || !strings.Contains(firstError["message"].(string), "Account.Name") {
		t.Fatalf("partial composite error row = %#v", firstError)
	}
}

func TestCompositeSObjectsAllOrNoneRollsBackSuccessfulRows(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"type":"Account","referenceId":"good"},"Name":"Good"},
    {"attributes":{"type":"Account","referenceId":"bad"}}
  ]
}`)))
	if composite.Code != http.StatusBadRequest {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("allOrNone rollback left records = %d", got)
	}
	if !bytes.Contains(composite.Body.Bytes(), []byte(`"referenceId":"good"`)) || !bytes.Contains(composite.Body.Bytes(), []byte(`"referenceId":"bad"`)) {
		t.Fatalf("composite reference ids missing: %s", composite.Body.String())
	}
}

func TestCompositeSObjectsUpdateSuccess(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Before One")
	addAccountForTest(&org, "001000000000002", "Before Two")
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "records": [
    {"attributes":{"type":"Account","referenceId":"upper"},"Id":"001000000000001","Name":"After One"},
    {"attributes":{"type":"Account","referenceId":"lower"},"id":"001000000000002","Description":"Second description"}
  ]
}`)))
	if composite.Code != http.StatusOK {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "After One" {
		t.Fatalf("updated name = %q", got)
	}
	if got := org.Objects["Account"].Records["001000000000002"].Fields["Description"].String; got != "Second description" {
		t.Fatalf("updated description = %q", got)
	}
	if !bytes.Contains(composite.Body.Bytes(), []byte(`"referenceId":"upper"`)) || !bytes.Contains(composite.Body.Bytes(), []byte(`"referenceId":"lower"`)) {
		t.Fatalf("reference ids missing: %s", composite.Body.String())
	}
}

func TestCompositeSObjectsUpdateIgnoresDuplicateIDCaseFields(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Before")
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "records": [
    {"attributes":{"type":"Account"},"Id":"001000000000001","id":"001000000000999","Name":"After"}
  ]
}`)))
	if composite.Code != http.StatusOK {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "After" {
		t.Fatalf("updated name = %q", got)
	}
	if bytes.Contains(composite.Body.Bytes(), []byte("unknown field Account.id")) {
		t.Fatalf("lowercase id leaked into field validation: %s", composite.Body.String())
	}
}

func TestCompositeSObjectsUpdateAllOrNoneRollsBack(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Before")
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"type":"Account","referenceId":"good"},"Id":"001000000000001","Name":"After"},
    {"attributes":{"type":"Missing__c","referenceId":"bad"},"Id":"001000000000999","Name":"Bad"}
  ]
}`)))
	if composite.Code != http.StatusBadRequest {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "Before" {
		t.Fatalf("allOrNone rollback changed name = %q", got)
	}
	if !bytes.Contains(composite.Body.Bytes(), []byte(`"referenceId":"good"`)) || !bytes.Contains(composite.Body.Bytes(), []byte(`"referenceId":"bad"`)) {
		t.Fatalf("reference ids missing: %s", composite.Body.String())
	}
}

func TestCompositeSObjectsDeleteSuccessLocatesObjectsByID(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Delete One")
	addAccountForTest(&org, "001000000000002", "Delete Two")
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/composite/sobjects?ids=001000000000001,001000000000002&allOrNone=true", nil))
	if composite.Code != http.StatusOK {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	if !org.Objects["Account"].Records["001000000000001"].System.IsDeleted || !org.Objects["Account"].Records["001000000000002"].System.IsDeleted {
		t.Fatalf("records were not soft deleted: %#v", org.Objects["Account"].Records)
	}
	if !bytes.Contains(composite.Body.Bytes(), []byte(`"id":"001000000000001"`)) || !bytes.Contains(composite.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("delete result shape = %s", composite.Body.String())
	}
}

func TestCompositeSObjectsDeleteAllOrNoneRollsBackMissingRecord(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Keep One")
	addAccountForTest(&org, "001000000000002", "Keep Two")
	handler := New(&org)

	composite := httptest.NewRecorder()
	handler.ServeHTTP(composite, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/composite/sobjects?ids=001000000000001,001000000000999&allOrNone=true", nil))
	if composite.Code != http.StatusBadRequest {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	if org.Objects["Account"].Records["001000000000001"].System.IsDeleted || org.Objects["Account"].Records["001000000000002"].System.IsDeleted {
		t.Fatalf("allOrNone rollback soft deleted records: %#v", org.Objects["Account"].Records)
	}
	if !bytes.Contains(composite.Body.Bytes(), []byte(`"id":"001000000000999"`)) || !bytes.Contains(composite.Body.Bytes(), []byte(`ENTITY_IS_DELETED`)) {
		t.Fatalf("missing delete result shape = %s", composite.Body.String())
	}
}

func TestCompositeSObjectTypedRetrieveReturnsRecords(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "One")
	addAccountForTest(&org, "001000000000002", "Two")
	object := org.Objects["Account"]
	first := object.Records["001000000000001"]
	first.Fields["Description"] = storage.StringValue("First description")
	object.Records["001000000000001"] = first
	second := object.Records["001000000000002"]
	second.Fields["External_Id__c"] = storage.StringValue("ext-two")
	object.Records["001000000000002"] = second
	org.Objects["Account"] = object
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/composite/sobjects/Account?ids=001000000000001,001000000000002", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("retrieve status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Records) != 2 {
		t.Fatalf("records = %#v", payload.Records)
	}
	if payload.Records[0]["Id"] != "001000000000001" || payload.Records[0]["Name"] != "One" || payload.Records[0]["Description"] != "First description" {
		t.Fatalf("first record = %#v", payload.Records[0])
	}
	if payload.Records[1]["Id"] != "001000000000002" || payload.Records[1]["External_Id__c"] != "ext-two" {
		t.Fatalf("second record = %#v", payload.Records[1])
	}
}

func TestCompositeSObjectTypedRetrieveProjectsFields(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Projected")
	object := org.Objects["Account"]
	record := object.Records["001000000000001"]
	record.Fields["Description"] = storage.StringValue("Shown")
	record.Fields["External_Id__c"] = storage.StringValue("hidden")
	object.Records["001000000000001"] = record
	org.Objects["Account"] = object
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v60.0/composite/sobjects/Account?ids=001000000000001&fields=Name,Description", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("retrieve status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Records) != 1 {
		t.Fatalf("records = %#v", payload.Records)
	}
	row := payload.Records[0]
	if row["Id"] != "001000000000001" || row["Name"] != "Projected" || row["Description"] != "Shown" {
		t.Fatalf("projected row = %#v", row)
	}
	if _, ok := row["External_Id__c"]; ok {
		t.Fatalf("projection included External_Id__c: %#v", row)
	}
	attrs, ok := row["attributes"].(map[string]any)
	if !ok || attrs["url"] != "/services/data/v60.0/sobjects/Account/001000000000001" {
		t.Fatalf("attributes = %#v", row["attributes"])
	}
}

func TestCompositeSObjectTypedRetrieveValidatesRequest(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing ids",
			path:       "/services/data/v61.0/composite/sobjects/Account",
			wantStatus: http.StatusBadRequest,
			wantCode:   "REQUIRED_FIELD_MISSING",
		},
		{
			name:       "empty id",
			path:       "/services/data/v61.0/composite/sobjects/Account?ids=001000000000001,",
			wantStatus: http.StatusBadRequest,
			wantCode:   "MALFORMED_ID",
		},
		{
			name:       "unknown field",
			path:       "/services/data/v61.0/composite/sobjects/Account?ids=001000000000001&fields=Nope__c",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_FIELD",
		},
		{
			name:       "missing record",
			path:       "/services/data/v61.0/composite/sobjects/Account?ids=001000000000999&fields=Name",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "deleted record",
			path:       "/services/data/v61.0/composite/sobjects/Account?ids=001000000000002&fields=Name",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "unknown object",
			path:       "/services/data/v61.0/composite/sobjects/Missing__c?ids=a00000000000001",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			addAccountForTest(&org, "001000000000001", "One")
			addAccountForTest(&org, "001000000000002", "Deleted")
			object := org.Objects["Account"]
			deleted := object.Records["001000000000002"]
			deleted.System.IsDeleted = true
			object.Records["001000000000002"] = deleted
			org.Objects["Account"] = object
			handler := New(&org)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			assertSalesforceError(t, rec, tt.wantStatus, tt.wantCode, "")
		})
	}
}

func TestCompositeSObjectTypedRetrieveSkipsBlankFieldsParameter(t *testing.T) {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Full")
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/composite/sobjects/Account?ids=001000000000001&fields=%20", nil))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"Name":"Full"`)) {
		t.Fatalf("retrieve status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompositeSObjectTypedUpsertCreatesAndUpdates(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects/Account/External_Id__c", strings.NewReader(`{
  "records": [
    {"attributes":{"referenceId":"created"},"Name":"Created","External_Id__c":"ext-1"}
  ]
}`)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var createResults []map[string]any
	if err := json.Unmarshal(create.Body.Bytes(), &createResults); err != nil {
		t.Fatal(err)
	}
	if len(createResults) != 1 || createResults[0]["success"] != true || createResults[0]["created"] != true || createResults[0]["referenceId"] != "created" {
		t.Fatalf("create results = %#v", createResults)
	}
	id, _ := createResults[0]["id"].(string)
	if id == "" {
		t.Fatalf("created id missing: %#v", createResults[0])
	}

	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects/Account/External_Id__c", strings.NewReader(`{
  "records": [
    {"attributes":{"referenceId":"updated"},"Name":"Updated","External_Id__c":"EXT-1"}
  ]
}`)))
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", update.Code, update.Body.String())
	}
	var updateResults []map[string]any
	if err := json.Unmarshal(update.Body.Bytes(), &updateResults); err != nil {
		t.Fatal(err)
	}
	if len(updateResults) != 1 || updateResults[0]["success"] != true || updateResults[0]["created"] != false || updateResults[0]["id"] != id || updateResults[0]["referenceId"] != "updated" {
		t.Fatalf("update results = %#v", updateResults)
	}
	if got := org.Objects["Account"].Records[storage.ID(id)].Fields["Name"].String; got != "Updated" {
		t.Fatalf("updated name = %q", got)
	}
}

func TestCompositeSObjectTypedUpsertAllOrNoneRollsBack(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects/Account/External_Id__c", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"referenceId":"good"},"Name":"Good","External_Id__c":"good-1"},
    {"attributes":{"referenceId":"bad"},"External_Id__c":"bad-1"}
  ]
}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upsert status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("allOrNone rollback left records = %d", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"referenceId":"good"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"referenceId":"bad"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`REQUIRED_FIELD_MISSING`)) {
		t.Fatalf("upsert rollback body = %s", rec.Body.String())
	}
}

func TestCompositeSObjectTypedUpsertMissingExternalIDReturnsRowError(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects/Account/External_Id__c", strings.NewReader(`{
  "records": [
    {"attributes":{"referenceId":"missing"},"Name":"Missing External"}
  ]
}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("missing external id status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("missing external id inserted records = %d", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"success":false`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`MISSING_ARGUMENT`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`External_Id__c`)) {
		t.Fatalf("missing external id body = %s", rec.Body.String())
	}
}

func TestCompositeSObjectTypedUpsertRejectsNonExternalField(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects/Account/Description", strings.NewReader(`{"records":[]}`)))
	assertSalesforceError(t, rec, http.StatusBadRequest, "INVALID_FIELD", "not an external id")
}

func TestCompositeSObjectTypedUpsertRejectsUnknownExternalField(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/composite/sobjects/Account/Missing_Key__c", strings.NewReader(`{"records":[]}`)))
	assertSalesforceError(t, rec, http.StatusBadRequest, "INVALID_FIELD", "unknown external id field")
}

func TestCompositeNamespaceUnsupportedStubs(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		wantMessageIn string
	}{
		{
			name:          "generic composite subrequests",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/composite",
			body:          `{"compositeRequest":[{"method":"GET","url":"/services/data/v61.0/limits","referenceId":"LimitsRef"}]}`,
			wantMessageIn: "Generic Composite REST subrequest orchestration",
		},
		{
			name:          "composite discovery",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/composite",
			wantMessageIn: "Composite namespace discovery",
		},
		{
			name:          "composite sobject typed collection",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/composite/sobjects/Account",
			body:          `{}`,
			wantMessageIn: "Composite sObject typed collection routes",
		},
		{
			name:          "composite sobject typed collection delete",
			method:        http.MethodDelete,
			path:          "/services/data/v61.0/composite/sobjects/Account",
			wantMessageIn: "Composite sObject collection delete routes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", tt.wantMessageIn)
		})
	}
}

func TestToolingExecuteAnonymousUsesServerLimitMode(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	handler.LimitMode = vm.LimitModeStrict
	handler.LimitCaps = vm.LimitCaps{
		Queries:       100,
		QueryRows:     50000,
		DMLStatements: 150,
		DMLRows:       10000,
		HeapSize:      6000000,
		CPUTimeMS:     0,
		FutureCalls:   50,
		QueueableJobs: 50,
		BatchJobs:     5,
		ScheduledJobs: 100,
		AsyncJobs:     50,
		EmailInvokes:  10,
		Callouts:      100,
	}

	exec := httptest.NewRecorder()
	handler.ServeHTTP(exec, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"System.debug('limited');"}`)))
	if exec.Code != http.StatusOK || !bytes.Contains(exec.Body.Bytes(), []byte(`"success":false`)) || !bytes.Contains(exec.Body.Bytes(), []byte(`System.LimitException`)) {
		t.Fatalf("executeAnonymous status = %d body=%s", exec.Code, exec.Body.String())
	}
}

func TestToolingExecuteAnonymousAcceptsFormAnonymousBody(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	form := url.Values{"anonymousBody": {"System.debug('form body');"}}
	req := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) || !bytes.Contains(rec.Body.Bytes(), []byte("form body")) {
		t.Fatalf("executeAnonymous form status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToolingExecuteAnonymousAcceptsFormSourceFallback(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	form := url.Values{"source": {"System.debug('form source');"}}
	req := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) || !bytes.Contains(rec.Body.Bytes(), []byte("form source")) {
		t.Fatalf("executeAnonymous form source status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToolingExecuteAnonymousMalformedJSONReportsCompileProblem(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	req := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.Bytes()
	if rec.Code != http.StatusOK ||
		!bytes.Contains(body, []byte(`"success":false`)) ||
		!bytes.Contains(body, []byte(`"compiled":false`)) ||
		!bytes.Contains(body, []byte(`"compileProblem":"unexpected`)) ||
		bytes.Contains(body, []byte("anonymousBody is required")) {
		t.Fatalf("executeAnonymous malformed JSON status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToolingExecuteAnonymousStillAcceptsGetAnonymousBody(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	query := url.Values{"anonymousBody": {"System.debug('get body');"}}
	req := httptest.NewRequest(http.MethodGet, "/services/data/v61.0/tooling/executeAnonymous?"+query.Encode(), nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) || !bytes.Contains(rec.Body.Bytes(), []byte("get body")) {
		t.Fatalf("executeAnonymous GET status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestToolingExecuteAnonymousStillAcceptsJSONWithoutContentType(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	req := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"System.debug('json body');"}`))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) || !bytes.Contains(rec.Body.Bytes(), []byte("json body")) {
		t.Fatalf("executeAnonymous JSON without content type status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerRollsBackFailedRequestTransactions(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	exec := httptest.NewRecorder()
	handler.ServeHTTP(exec, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"insert new Account(Name = 'Transient'); System.assert(false);"}`)))
	if exec.Code != http.StatusOK || !bytes.Contains(exec.Body.Bytes(), []byte(`"success":false`)) {
		t.Fatalf("executeAnonymous status = %d body=%s", exec.Code, exec.Body.String())
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("executeAnonymous rollback left records = %#v", org.Objects["Account"].Records)
	}

	store := &failingStore{}
	handler = NewWithStore(&org, store)
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"PersistFail"}`)))
	if create.Code != http.StatusInternalServerError {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("persist failure rollback left records = %#v", org.Objects["Account"].Records)
	}
}

func TestExecuteAnonymousUsesHeaderUserContext(t *testing.T) {
	org := testOrg()
	userID := storage.ID("005000000000777")
	addUser(&org, userID, "trail@example.test", "trail-email@example.test", "Trail User")
	handler := New(&org)

	payload, err := json.Marshal(map[string]string{"anonymousBody": `
System.assertEquals('005000000000777', UserInfo.getUserId());
System.assertEquals('trail@example.test', UserInfo.getUserName());
System.assertEquals('trail-email@example.test', UserInfo.getUserEmail());
insert new Account(Name = 'Header User');
`})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(string(payload)))
	req.Header.Set("X-OAER-User-Id", string(userID))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("executeAnonymous status = %d body=%s", rec.Code, rec.Body.String())
	}

	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("account records = %d, want 1", len(org.Objects["Account"].Records))
	}
	for _, record := range org.Objects["Account"].Records {
		if record.System.CreatedByID != userID || record.System.OwnerID != userID {
			t.Fatalf("system user fields = %#v, want CreatedById/OwnerId %s", record.System, userID)
		}
	}
}

func TestExecuteAnonymousUsesBearerUserContext(t *testing.T) {
	org := testOrg()
	userID := storage.ID("005000000000778")
	addUser(&org, userID, "bearer@example.test", "bearer-email@example.test", "Bearer User")
	handler := New(&org)

	payload, err := json.Marshal(map[string]string{"anonymousBody": `
System.assertEquals('005000000000778', UserInfo.getUserId());
System.assertEquals('bearer@example.test', UserInfo.getUserName());
System.assertEquals('bearer-email@example.test', UserInfo.getUserEmail());
`})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(string(payload)))
	req.Header.Set("Authorization", "Bearer "+string(userID))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"success":true`)) {
		t.Fatalf("executeAnonymous status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerSerializesConcurrentMutations(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	const requests = 8
	var wg sync.WaitGroup
	ids := make(chan storage.ID, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			create := httptest.NewRecorder()
			handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"Concurrent"}`)))
			if create.Code != http.StatusCreated {
				t.Errorf("create status = %d body=%s", create.Code, create.Body.String())
				return
			}
			var payload struct {
				ID storage.ID `json:"id"`
			}
			if err := json.Unmarshal(create.Body.Bytes(), &payload); err != nil {
				t.Errorf("decode create body: %v", err)
				return
			}
			ids <- payload.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := make(map[storage.ID]bool)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
	if len(seen) != requests {
		t.Fatalf("created ids = %d, want %d", len(seen), requests)
	}
	if len(org.Objects["Account"].Records) != requests {
		t.Fatalf("stored records = %d, want %d", len(org.Objects["Account"].Records), requests)
	}
}

func TestUnsupportedRESTNamespaceMethodBoundary(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		wantStatus    int
		wantCode      string
		wantAllow     string
		wantMessageIn string
	}{
		{
			name:          "get root remains unsupported",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/connect",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Connect REST namespace",
		},
		{
			name:          "get subroute remains unsupported",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/quickActions/Account/NewTask",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "QuickActions REST namespace",
		},
		{
			name:       "post root method not allowed",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/actions",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet,
		},
		{
			name:       "patch subroute method not allowed",
			method:     http.MethodPatch,
			path:       "/services/data/v61.0/metadata/deployRequest/0Af000000000001",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet,
		},
		{
			name:       "unknown namespace still not found",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/notANamespace",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tt.method, tt.path, nil))
			assertSalesforceError(t, res, tt.wantStatus, tt.wantCode, tt.wantMessageIn)
			if got := res.Header().Get("Allow"); got != tt.wantAllow {
				t.Fatalf("Allow = %q, want %q", got, tt.wantAllow)
			}
		})
	}
}

func TestBulkJobResultRoutesPinUnsupportedBodyShapes(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		wantStatus    int
		wantCode      string
		wantAllow     string
		wantMessageIn string
	}{
		{
			name:          "query results with locator",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/jobs/query/750000000000001/results?locator=abc&maxRecords=2",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Bulk API v2 query job results",
		},
		{
			name:       "query unknown result subroute",
			method:     http.MethodGet,
			path:       "/services/data/v61.0/jobs/query/750000000000001/failedResults",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:          "ingest successful results",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/jobs/ingest/750000000000001/successfulResults",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Bulk API v2 ingest successful results",
		},
		{
			name:          "ingest failed results",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/jobs/ingest/750000000000001/failedResults",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Bulk API v2 ingest failed results",
		},
		{
			name:          "ingest unprocessed records lowercase",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/jobs/ingest/750000000000001/unprocessedrecords",
			wantStatus:    http.StatusNotImplemented,
			wantCode:      "UNSUPPORTED_FEATURE",
			wantMessageIn: "Bulk API v2 ingest unprocessed records",
		},
		{
			name:       "ingest result method",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/jobs/ingest/750000000000001/failedResults",
			body:       `{}`,
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			assertSalesforceError(t, rec, tt.wantStatus, tt.wantCode, tt.wantMessageIn)
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Fatalf("Allow = %q, want %q", got, tt.wantAllow)
			}
		})
	}
}

func TestSalesforceErrorResponses(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		store         interface{ Save(storage.OrgState) error }
		wantStatus    int
		wantCode      string
		wantAllow     string
		wantMessageIn string
	}{
		{
			name:       "unknown route",
			method:     http.MethodGet,
			path:       "/services/data/v61.0/nope",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/limits",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet,
		},
		{
			name:       "object method not allowed",
			method:     http.MethodPut,
			path:       "/services/data/v61.0/sobjects/Account",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet + ", " + http.MethodPost,
		},
		{
			name:          "unknown object resource",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/sobjects/Missing__c",
			wantStatus:    http.StatusNotFound,
			wantCode:      "NOT_FOUND",
			wantMessageIn: "unknown object",
		},
		{
			name:       "record method not allowed",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/sobjects/Account/001000000000001",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet + ", " + http.MethodPatch + ", " + http.MethodDelete,
		},
		{
			name:       "metadata method not allowed",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/sobjects/Account/compactLayouts",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet,
		},
		{
			name:       "composite root method not allowed",
			method:     http.MethodDelete,
			path:       "/services/data/v61.0/composite",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet + ", " + http.MethodPost,
		},
		{
			name:       "composite sobjects method not allowed",
			method:     http.MethodGet,
			path:       "/services/data/v61.0/composite/sobjects",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodPost + ", " + http.MethodPatch + ", " + http.MethodDelete,
		},
		{
			name:       "composite sobjects child method not allowed",
			method:     http.MethodPut,
			path:       "/services/data/v61.0/composite/sobjects/Account/External_Id__c",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "METHOD_NOT_ALLOWED",
			wantAllow:  http.MethodGet + ", " + http.MethodPost + ", " + http.MethodPatch + ", " + http.MethodDelete,
		},
		{
			name:          "invalid json",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/sobjects/Account",
			body:          `{"Name":`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "JSON_PARSER_ERROR",
			wantMessageIn: "unexpected EOF",
		},
		{
			name:          "malformed query",
			method:        http.MethodGet,
			path:          "/services/data/v61.0/query?q=SELECT%20FROM",
			wantStatus:    http.StatusBadRequest,
			wantCode:      "MALFORMED_QUERY",
			wantMessageIn: "expected",
		},
		{
			name:          "dml required field",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/sobjects/Account",
			body:          `{}`,
			wantStatus:    http.StatusBadRequest,
			wantCode:      "DML_EXCEPTION",
			wantMessageIn: "Name",
		},
		{
			name:          "store failure",
			method:        http.MethodPost,
			path:          "/services/data/v61.0/sobjects/Account",
			body:          `{"Name":"PersistFail"}`,
			store:         &failingStore{},
			wantStatus:    http.StatusInternalServerError,
			wantCode:      "SERVER_ERROR",
			wantMessageIn: "store failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := testOrg()
			handler := NewWithStore(&org, tt.store)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			assertSalesforceError(t, res, tt.wantStatus, tt.wantCode, tt.wantMessageIn)
			if tt.wantAllow != "" && res.Header().Get("Allow") != tt.wantAllow {
				t.Fatalf("Allow = %q, want %q", res.Header().Get("Allow"), tt.wantAllow)
			}
		})
	}
}

func decodeRecentItems(t *testing.T, res *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var items []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func assertSalesforceError(t *testing.T, res *httptest.ResponseRecorder, status int, code, messageContains string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q", got)
	}
	var errors []salesforceError
	if err := json.Unmarshal(res.Body.Bytes(), &errors); err != nil {
		t.Fatal(err)
	}
	if len(errors) != 1 || errors[0].ErrorCode != code || errors[0].Message == "" {
		t.Fatalf("errors = %#v", errors)
	}
	if messageContains != "" && !strings.Contains(errors[0].Message, messageContains) {
		t.Fatalf("message = %q, want to contain %q", errors[0].Message, messageContains)
	}
}

type memoryStore struct {
	saves int
	last  storage.OrgState
}

func (s *memoryStore) Save(org storage.OrgState) error {
	s.saves++
	s.last = org.Clone()
	return nil
}

type failingStore struct{}

func (s *failingStore) Save(storage.OrgState) error {
	return errors.New("store failed")
}

func testOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString, Required: true},
				"Description":    {APIName: "Description", Type: storage.FieldString},
				"External_Id__c": {APIName: "External_Id__c", Type: storage.FieldString, ExternalID: true, Unique: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func addExternalIDFieldForTest(org *storage.OrgState) {
	object := org.Objects["Account"]
	object.Definition.Fields["External_Id__c"] = storage.Field{APIName: "External_Id__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = object
}

func addAccountForTest(org *storage.OrgState, id storage.ID, name string) {
	object := org.Objects["Account"]
	object.Records[id] = storage.Record{
		ID:     id,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue(name),
		},
	}
	org.Objects["Account"] = object
}

func resetScopeTestOrg() storage.OrgState {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Account"].Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Id":   storage.StringValue("001000000000001"),
			"Name": storage.StringValue("Acme"),
		},
	}
	userObject := org.Objects["User"]
	userObject.Records["005000000000999"] = storage.Record{
		ID:     "005000000000999",
		Object: "User",
		Fields: map[string]storage.Value{
			"Id":       storage.StringValue("005000000000999"),
			"Username": storage.StringValue("extra@example.test"),
			"Alias":    storage.StringValue("extra"),
			"Email":    storage.StringValue("extra@example.test"),
			"IsActive": storage.BooleanValue(true),
			"UserType": storage.StringValue("Standard"),
		},
	}
	org.Objects["User"] = userObject
	return org
}

func addUser(org *storage.OrgState, id storage.ID, username, email, name string) {
	storage.EnsureDeterministicPlatformData(org)
	userObject := org.Objects["User"]
	userObject.Records[id] = userRecordForTest(id, username, email, name)
	org.Objects["User"] = userObject
}

func userRecordForTest(id storage.ID, username, email, name string) storage.Record {
	return storage.Record{
		ID:     id,
		Object: "User",
		Fields: map[string]storage.Value{
			"Username": storage.StringValue(username),
			"Email":    storage.StringValue(email),
			"Name":     storage.StringValue(name),
			"IsActive": storage.BooleanValue(true),
			"UserType": storage.StringValue("Standard"),
		},
	}
}
