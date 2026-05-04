package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/vm"
)

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

func TestDescribeEndpoints(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/services/data", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`/services/data/v61.0`)) {
		t.Fatalf("versions status = %d body=%s", list.Code, list.Body.String())
	}

	sobjects := httptest.NewRecorder()
	handler.ServeHTTP(sobjects, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects", nil))
	if sobjects.Code != http.StatusOK || !bytes.Contains(sobjects.Body.Bytes(), []byte(`"Account"`)) {
		t.Fatalf("sobjects status = %d body=%s", sobjects.Code, sobjects.Body.String())
	}
}

func TestResourceDiscoveryEndpoints(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	versions := httptest.NewRecorder()
	handler.ServeHTTP(versions, httptest.NewRequest(http.MethodGet, "/services/data", nil))
	if versions.Code != http.StatusOK {
		t.Fatalf("versions status = %d body=%s", versions.Code, versions.Body.String())
	}
	var versionRows []map[string]string
	if err := json.Unmarshal(versions.Body.Bytes(), &versionRows); err != nil {
		t.Fatal(err)
	}
	if len(versionRows) != 1 || versionRows[0]["version"] != "61.0" || versionRows[0]["label"] == "" || versionRows[0]["url"] != "/services/data/v61.0" {
		t.Fatalf("versions = %#v", versionRows)
	}

	root := httptest.NewRecorder()
	handler.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/services/data/v61.0", nil))
	if root.Code != http.StatusOK {
		t.Fatalf("root status = %d body=%s", root.Code, root.Body.String())
	}
	var resources map[string]string
	if err := json.Unmarshal(root.Body.Bytes(), &resources); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sobjects", "query", "queryAll", "tooling", "limits", "composite", "oaer"} {
		if resources[key] == "" {
			t.Fatalf("resource %q missing from %#v", key, resources)
		}
	}

	cases := []struct {
		path string
		keys []string
	}{
		{path: "/services/data/v61.0/tooling", keys: []string{"executeAnonymous", "query"}},
		{path: "/services/data/v61.0/composite", keys: []string{"sobjects"}},
		{path: "/services/data/v61.0/oaer", keys: []string{"fixture", "reset"}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if res.Code != http.StatusOK {
				t.Fatalf("%s status = %d body=%s", tc.path, res.Code, res.Body.String())
			}
			var payload map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			for _, key := range tc.keys {
				if payload[key] == "" {
					t.Fatalf("%s missing %q in %#v", tc.path, key, payload)
				}
			}
		})
	}

	for _, path := range []string{"/services/data/v61.0/tooling", "/services/data/v61.0/oaer"} {
		t.Run(path+" method", func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, path, nil))
			if res.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s status = %d body=%s", path, res.Code, res.Body.String())
			}
		})
	}

	versioned := httptest.NewRecorder()
	handler.ServeHTTP(versioned, httptest.NewRequest(http.MethodGet, "/services/data/v62.0/tooling", nil))
	if versioned.Code != http.StatusOK || !bytes.Contains(versioned.Body.Bytes(), []byte(`/services/data/v62.0/tooling/executeAnonymous`)) {
		t.Fatalf("versioned tooling status = %d body=%s", versioned.Code, versioned.Body.String())
	}

	versionedSObjects := httptest.NewRecorder()
	handler.ServeHTTP(versionedSObjects, httptest.NewRequest(http.MethodGet, "/services/data/v62.0/sobjects", nil))
	if versionedSObjects.Code != http.StatusOK || !bytes.Contains(versionedSObjects.Body.Bytes(), []byte(`/services/data/v62.0/sobjects/Account/describe`)) {
		t.Fatalf("versioned sobjects status = %d body=%s", versionedSObjects.Code, versionedSObjects.Body.String())
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

func TestSObjectRESTResponseShapes(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Active__c"] = storage.Field{APIName: "Active__c", Type: storage.FieldBoolean}
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldInteger}
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	account.Definition.Fields["Tags__c"] = storage.Field{APIName: "Tags__c", Type: storage.FieldAny}
	account.Definition.Fields["Rating__c"] = storage.Field{
		APIName: "Rating__c",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot", Active: true, Default: true},
		},
	}
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	handler := New(&org)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{
  "attributes":{"type":"Ignored"},
  "Name":"Acme",
  "Active__c":true,
  "Score__c":7,
  "Amount__c":12.75,
  "Tags__c":["a","b"],
  "Rating__c":"Hot",
  "External_Key__c":"ext-1"
}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}

	patch := httptest.NewRecorder()
	handler.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/001000000000001", strings.NewReader(`{"Name":null}`)))
	if patch.Code != http.StatusNoContent {
		t.Fatalf("patch status = %d body=%s", patch.Code, patch.Body.String())
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/001000000000001", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", get.Code, get.Body.String())
	}
	var record map[string]any
	if err := json.Unmarshal(get.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	attrs, ok := record["attributes"].(map[string]any)
	if !ok || attrs["type"] != "Account" || attrs["url"] != "/services/data/v61.0/sobjects/Account/001000000000001" {
		t.Fatalf("record attributes = %#v body=%s", record["attributes"], get.Body.String())
	}
	if record["Id"] != "001000000000001" || record["Name"] != nil || record["Active__c"] != true || record["Score__c"].(float64) != 7 || record["Amount__c"] != "12.75" {
		t.Fatalf("record payload = %#v", record)
	}
	if tags, ok := record["Tags__c"].([]any); !ok || len(tags) != 2 {
		t.Fatalf("Tags__c = %#v", record["Tags__c"])
	}

	query := httptest.NewRecorder()
	handler.ServeHTTP(query, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id,%20Name,%20Active__c%20FROM%20Account", nil))
	if query.Code != http.StatusOK {
		t.Fatalf("query status = %d body=%s", query.Code, query.Body.String())
	}
	var queryPayload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(query.Body.Bytes(), &queryPayload); err != nil {
		t.Fatal(err)
	}
	if len(queryPayload.Records) != 1 {
		t.Fatalf("query payload = %#v", queryPayload)
	}
	queryAttrs, ok := queryPayload.Records[0]["attributes"].(map[string]any)
	if !ok || queryAttrs["type"] != "Account" || queryAttrs["url"] != "/services/data/v61.0/sobjects/Account/001000000000001" {
		t.Fatalf("query attributes = %#v", queryPayload.Records[0]["attributes"])
	}

	recent := httptest.NewRecorder()
	handler.ServeHTTP(recent, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/recent", nil))
	if recent.Code != http.StatusOK {
		t.Fatalf("recent status = %d body=%s", recent.Code, recent.Body.String())
	}
	var recentRows []map[string]any
	if err := json.Unmarshal(recent.Body.Bytes(), &recentRows); err != nil {
		t.Fatal(err)
	}
	if len(recentRows) != 1 {
		t.Fatalf("recent rows = %#v", recentRows)
	}
	recentAttrs, ok := recentRows[0]["attributes"].(map[string]any)
	if !ok || recentAttrs["type"] != "Account" || recentAttrs["url"] != "/services/data/v61.0/sobjects/Account/001000000000001" {
		t.Fatalf("recent attributes = %#v", recentRows[0]["attributes"])
	}

	describe := httptest.NewRecorder()
	handler.ServeHTTP(describe, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/describe", nil))
	if describe.Code != http.StatusOK {
		t.Fatalf("describe status = %d body=%s", describe.Code, describe.Body.String())
	}
	var describePayload struct {
		Fields []map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(describe.Body.Bytes(), &describePayload); err != nil {
		t.Fatal(err)
	}
	fields := map[string]map[string]any{}
	for _, field := range describePayload.Fields {
		fields[field["name"].(string)] = field
	}
	if fields["Name"]["createable"] != true || fields["Name"]["updateable"] != true || fields["Name"]["length"].(float64) == 0 {
		t.Fatalf("Name field metadata = %#v", fields["Name"])
	}
	if fields["External_Key__c"]["externalId"] != true || fields["External_Key__c"]["unique"] != true {
		t.Fatalf("External_Key__c metadata = %#v", fields["External_Key__c"])
	}
	if picklist, ok := fields["Rating__c"]["picklistValues"].([]any); !ok || len(picklist) != 1 {
		t.Fatalf("Rating__c metadata = %#v", fields["Rating__c"])
	}

	account = org.Objects["Account"]
	deletedID := storage.ID("001000000000099")
	account.Records[deletedID] = storage.Record{
		ID:     deletedID,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Deleted"),
		},
		System: storage.SystemFields{IsDeleted: true},
	}
	org.Objects["Account"] = account

	queryDefault := httptest.NewRecorder()
	handler.ServeHTTP(queryDefault, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/query?q=SELECT%20Id,%20Name%20FROM%20Account", nil))
	if queryDefault.Code != http.StatusOK || bytes.Contains(queryDefault.Body.Bytes(), []byte(`Deleted`)) {
		t.Fatalf("query default status = %d body=%s", queryDefault.Code, queryDefault.Body.String())
	}

	queryAll := httptest.NewRecorder()
	handler.ServeHTTP(queryAll, httptest.NewRequest(http.MethodGet, "/services/data/v62.0/queryAll?q=SELECT%20Id,%20Name%20FROM%20Account%20WHERE%20Id%20!=%20'ALL%20ROWS%20Cafe'", nil))
	if queryAll.Code != http.StatusOK || !bytes.Contains(queryAll.Body.Bytes(), []byte(`Deleted`)) || !bytes.Contains(queryAll.Body.Bytes(), []byte(`/services/data/v62.0/sobjects/Account/001000000000099`)) {
		t.Fatalf("queryAll status = %d body=%s", queryAll.Code, queryAll.Body.String())
	}
}

func TestSOQLChildRelationshipRESTShape(t *testing.T) {
	org := testOrgWithAccount("001000000000001", "Acme")
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
				"LastName":  {APIName: "LastName", Type: storage.FieldString, Required: true},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {
				ID:     "003000000000001",
				Object: "Contact",
				Fields: map[string]storage.Value{
					"AccountId": storage.IDValue("001000000000001"),
					"LastName":  storage.StringValue("Alpha"),
				},
			},
		},
	}
	storage.RebuildIndexes(&org)
	handler := New(&org)

	query := httptest.NewRecorder()
	handler.ServeHTTP(query, httptest.NewRequest(http.MethodGet, "/services/data/v62.0/query?q=SELECT%20Id,%20(SELECT%20Id,%20LastName%20FROM%20Contacts)%20FROM%20Account", nil))
	if query.Code != http.StatusOK {
		t.Fatalf("query status = %d body=%s", query.Code, query.Body.String())
	}
	if !bytes.Contains(query.Body.Bytes(), []byte(`"Contacts"`)) || !bytes.Contains(query.Body.Bytes(), []byte(`/services/data/v62.0/sobjects/Contact/003000000000001`)) || !bytes.Contains(query.Body.Bytes(), []byte(`"LastName":"Alpha"`)) {
		t.Fatalf("query body missing child relationship REST shape: %s", query.Body.String())
	}
}

func TestLocalAuthUserSelectionAndDMLStamping(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	addTestUser(&org, "005000000000222", "local-user@example.test")
	handler := New(&org)

	bearer := httptest.NewRecorder()
	bearerReq := httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil)
	bearerReq.Header.Set("Authorization", "Bearer any-local-token")
	handler.ServeHTTP(bearer, bearerReq)
	if bearer.Code != http.StatusOK || !bytes.Contains(bearer.Body.Bytes(), []byte(`"user_id":"005000000000001"`)) || !bytes.Contains(bearer.Body.Bytes(), []byte(`"active":true`)) {
		t.Fatalf("bearer userinfo status = %d body=%s", bearer.Code, bearer.Body.String())
	}

	selected := httptest.NewRecorder()
	selectedReq := httptest.NewRequest(http.MethodGet, "/id/00D000000000001/005000000000222", nil)
	selectedReq.Header.Set("X-OAER-User-Id", "005000000000222")
	handler.ServeHTTP(selected, selectedReq)
	if selected.Code != http.StatusOK || !bytes.Contains(selected.Body.Bytes(), []byte(`"user_id":"005000000000222"`)) || !bytes.Contains(selected.Body.Bytes(), []byte(`local-user@example.test`)) {
		t.Fatalf("selected identity status = %d body=%s", selected.Code, selected.Body.String())
	}

	unknown := httptest.NewRecorder()
	unknownReq := httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil)
	unknownReq.Header.Set("X-OAER-User-Id", "005000000009999")
	handler.ServeHTTP(unknown, unknownReq)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown user status = %d body=%s", unknown.Code, unknown.Body.String())
	}
	errors := decodeServerErrors(t, unknown)
	if len(errors) != 1 || errors[0].ErrorCode != "INVALID_USER" {
		t.Fatalf("unknown user errors = %#v", errors)
	}
	if strings.Contains(errors[0].Message, "005000000009999") {
		t.Fatalf("invalid user message leaked requested id: %#v", errors)
	}

	create := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/sobjects/Account", strings.NewReader(`{"Name":"Stamped"}`))
	createReq.Header.Set("X-OAER-User-Id", "005000000000222")
	handler.ServeHTTP(create, createReq)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	record := org.Objects["Account"].Records["001000000000001"]
	if record.System.CreatedByID != "005000000000222" || record.System.LastModifiedByID != "005000000000222" || record.System.OwnerID != "005000000000222" {
		t.Fatalf("system fields = %#v", record.System)
	}

	composite := httptest.NewRecorder()
	compositeReq := httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"type":"Account"},"Name":"Composite Stamped"}
  ]
}`))
	compositeReq.Header.Set("X-OAER-User-Id", "005000000000222")
	handler.ServeHTTP(composite, compositeReq)
	if composite.Code != http.StatusOK {
		t.Fatalf("composite status = %d body=%s", composite.Code, composite.Body.String())
	}
	compositeRecord := org.Objects["Account"].Records["001000000000002"]
	if compositeRecord.System.CreatedByID != "005000000000222" || compositeRecord.System.LastModifiedByID != "005000000000222" || compositeRecord.System.OwnerID != "005000000000222" {
		t.Fatalf("composite system fields = %#v", compositeRecord.System)
	}
}

func TestCurrentUserFallsBackToSortedLocalUser(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	userObject := org.Objects["User"]
	delete(userObject.Records, "005000000000001")
	org.Objects["User"] = userObject
	addTestUser(&org, "005000000000333", "z-local@example.test")
	addTestUser(&org, "005000000000222", "a-local@example.test")
	handler := New(&org)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/services/oauth2/userinfo", nil))
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"user_id":"005000000000222"`)) {
		t.Fatalf("fallback userinfo status = %d body=%s", res.Code, res.Body.String())
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

func TestToolingExecuteAnonymousResponseShapes(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	getExec := httptest.NewRecorder()
	handler.ServeHTTP(getExec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/tooling/executeAnonymous?anonymousBody=System.debug%28%27from-get%27%29%3B", nil))
	if getExec.Code != http.StatusOK {
		t.Fatalf("GET executeAnonymous status = %d body=%s", getExec.Code, getExec.Body.String())
	}
	var success map[string]any
	if err := json.Unmarshal(getExec.Body.Bytes(), &success); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"compiled", "success", "compileProblem", "exceptionMessage", "exceptionStackTrace", "line", "column", "logs"} {
		if _, ok := success[key]; !ok {
			t.Fatalf("GET executeAnonymous missing %q in %#v", key, success)
		}
	}
	if success["compiled"] != true || success["success"] != true || !strings.Contains(success["logs"].(string), "from-get") {
		t.Fatalf("GET executeAnonymous payload = %#v", success)
	}

	compileFailure := httptest.NewRecorder()
	handler.ServeHTTP(compileFailure, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"public class Nope {"}`)))
	if compileFailure.Code != http.StatusOK {
		t.Fatalf("compile failure status = %d body=%s", compileFailure.Code, compileFailure.Body.String())
	}
	var compilePayload map[string]any
	if err := json.Unmarshal(compileFailure.Body.Bytes(), &compilePayload); err != nil {
		t.Fatal(err)
	}
	if compilePayload["compiled"] != false || compilePayload["success"] != false || compilePayload["compileProblem"] == nil || compilePayload["exceptionMessage"] != nil {
		t.Fatalf("compile failure payload = %#v", compilePayload)
	}

	runtimeFailure := httptest.NewRecorder()
	handler.ServeHTTP(runtimeFailure, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"insert new Account(Name = 'Rolled Back'); System.assert(false);"}`)))
	if runtimeFailure.Code != http.StatusOK {
		t.Fatalf("runtime failure status = %d body=%s", runtimeFailure.Code, runtimeFailure.Body.String())
	}
	var runtimePayload map[string]any
	if err := json.Unmarshal(runtimeFailure.Body.Bytes(), &runtimePayload); err != nil {
		t.Fatal(err)
	}
	if runtimePayload["compiled"] != true || runtimePayload["success"] != false || runtimePayload["compileProblem"] != nil || runtimePayload["exceptionMessage"] == nil {
		t.Fatalf("runtime failure payload = %#v", runtimePayload)
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("runtime failure leaked records = %#v", org.Objects["Account"].Records)
	}
}

func TestToolingQuerySupportedAndUnsupportedObjects(t *testing.T) {
	org := testOrgWithAccount("001000000000001", "Acme")
	handler := New(&org)

	supported := httptest.NewRecorder()
	handler.ServeHTTP(supported, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/tooling/query?q=SELECT%20Id,%20Name%20FROM%20Account", nil))
	if supported.Code != http.StatusOK || !bytes.Contains(supported.Body.Bytes(), []byte(`"totalSize":1`)) || !bytes.Contains(supported.Body.Bytes(), []byte(`"attributes"`)) {
		t.Fatalf("supported tooling query status = %d body=%s", supported.Code, supported.Body.String())
	}

	unsupported := httptest.NewRecorder()
	handler.ServeHTTP(unsupported, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/tooling/query?q=SELECT%20Id%20FROM%20ApexClass", nil))
	if unsupported.Code != http.StatusBadRequest {
		t.Fatalf("unsupported tooling query status = %d body=%s", unsupported.Code, unsupported.Body.String())
	}
	errors := decodeServerErrors(t, unsupported)
	if len(errors) != 1 || errors[0].ErrorCode != "UNSUPPORTED_TOOLING_OBJECT" || !strings.Contains(errors[0].Message, "ApexClass") {
		t.Fatalf("unsupported tooling errors = %#v", errors)
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

func TestServerPersistFailuresDoNotLeakMutations(t *testing.T) {
	t.Run("update", func(t *testing.T) {
		org := testOrgWithAccount("001000000000001", "Original")
		handler := NewWithStore(&org, &failingStore{})
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodPatch, "/services/data/v61.0/sobjects/Account/001000000000001", strings.NewReader(`{"Name":"Changed"}`)))
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != "Original" {
			t.Fatalf("record leaked update = %q", got)
		}
	})
	t.Run("delete", func(t *testing.T) {
		org := testOrgWithAccount("001000000000001", "Original")
		handler := NewWithStore(&org, &failingStore{})
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodDelete, "/services/data/v61.0/sobjects/Account/001000000000001", nil))
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		if org.Objects["Account"].Records["001000000000001"].System.IsDeleted {
			t.Fatalf("record leaked delete = %#v", org.Objects["Account"].Records["001000000000001"])
		}
	})
	t.Run("execute anonymous", func(t *testing.T) {
		org := testOrg()
		handler := NewWithStore(&org, &failingStore{})
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"insert new Account(Name = 'Transient');"}`)))
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		if len(org.Objects["Account"].Records) != 0 {
			t.Fatalf("executeAnonymous persist failure leaked records = %#v", org.Objects["Account"].Records)
		}
	})
	t.Run("composite", func(t *testing.T) {
		org := testOrg()
		handler := NewWithStore(&org, &failingStore{})
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{"records":[{"attributes":{"type":"Account"},"Name":"Transient"}]}`)))
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		if len(org.Objects["Account"].Records) != 0 {
			t.Fatalf("composite persist failure leaked records = %#v", org.Objects["Account"].Records)
		}
	})
	t.Run("fixture", func(t *testing.T) {
		org := testOrg()
		handler := NewWithStore(&org, &failingStore{})
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/fixture", strings.NewReader(`{
  "version":"oaer.storage.v1",
  "objects":[{"name":"Account","records":[{"fields":{"Name":{"kind":"string","string":"Transient"}}}]}]
}`)))
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		if len(org.Objects["Account"].Records) != 0 {
			t.Fatalf("fixture persist failure leaked records = %#v", org.Objects["Account"].Records)
		}
	})
	t.Run("reset", func(t *testing.T) {
		org := testOrgWithAccount("001000000000001", "Original")
		handler := NewWithStore(&org, &failingStore{})
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/oaer/reset/data", nil))
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		if got := len(org.Objects["Account"].Records); got != 1 {
			t.Fatalf("reset persist failure leaked records = %d", got)
		}
	})
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

func TestSalesforceErrorShape(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		status     int
		errorCode  string
		wantField  string
		wantPhrase string
	}{
		{
			name:      "unknown object",
			method:    http.MethodGet,
			path:      "/services/data/v61.0/sobjects/Missing__c",
			status:    http.StatusNotFound,
			errorCode: "NOT_FOUND",
		},
		{
			name:      "missing record",
			method:    http.MethodGet,
			path:      "/services/data/v61.0/sobjects/Account/001000000000999",
			status:    http.StatusNotFound,
			errorCode: "NOT_FOUND",
		},
		{
			name:      "malformed json",
			method:    http.MethodPost,
			path:      "/services/data/v61.0/sobjects/Account",
			body:      "{",
			status:    http.StatusBadRequest,
			errorCode: "JSON_PARSER_ERROR",
		},
		{
			name:      "method mismatch",
			method:    http.MethodPost,
			path:      "/services/data",
			status:    http.StatusMethodNotAllowed,
			errorCode: "METHOD_NOT_ALLOWED",
		},
		{
			name:      "malformed query",
			method:    http.MethodGet,
			path:      "/services/data/v61.0/query?q=SELECT%20FROM",
			status:    http.StatusBadRequest,
			errorCode: "MALFORMED_QUERY",
		},
		{
			name:      "unsupported route",
			method:    http.MethodGet,
			path:      "/services/data/v61.0/nope",
			status:    http.StatusNotFound,
			errorCode: "NOT_FOUND",
		},
		{
			name:       "dml missing required field",
			method:     http.MethodPost,
			path:       "/services/data/v61.0/sobjects/Account",
			body:       "{}",
			status:     http.StatusBadRequest,
			errorCode:  "REQUIRED_FIELD_MISSING",
			wantField:  "Name",
			wantPhrase: "missing required field",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if res.Code != tc.status {
				t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
			}
			errors := decodeServerErrors(t, res)
			if len(errors) != 1 || errors[0].ErrorCode != tc.errorCode || errors[0].Message == "" {
				t.Fatalf("errors = %#v", errors)
			}
			if tc.wantPhrase != "" && !strings.Contains(errors[0].Message, tc.wantPhrase) {
				t.Fatalf("message = %q, want phrase %q", errors[0].Message, tc.wantPhrase)
			}
			if tc.wantField != "" && !containsString(errors[0].Fields, tc.wantField) {
				t.Fatalf("fields = %#v, want %q", errors[0].Fields, tc.wantField)
			}
		})
	}
}

func TestCompositeSObjectErrorsUseDMLStatusAndFields(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"type":"Account"}}
  ]
}`)))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	var rows []struct {
		Success bool `json:"success"`
		Errors  []struct {
			StatusCode string   `json:"statusCode"`
			Message    string   `json:"message"`
			Fields     []string `json:"fields,omitempty"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Success || len(rows[0].Errors) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	errPayload := rows[0].Errors[0]
	if errPayload.StatusCode != "REQUIRED_FIELD_MISSING" || errPayload.Message == "" || !containsString(errPayload.Fields, "Name") {
		t.Fatalf("error payload = %#v", errPayload)
	}
}

func TestCompositeSObjectsPartialSuccessReferenceIdsAndRollback(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	partial := httptest.NewRecorder()
	handler.ServeHTTP(partial, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": false,
  "records": [
    {"attributes":{"type":"Account"},"referenceId":"first","Name":"Composite One"},
    {"attributes":{"type":"Account"},"referenceId":"missing-name"}
  ]
}`)))
	if partial.Code != http.StatusOK {
		t.Fatalf("partial status = %d body=%s", partial.Code, partial.Body.String())
	}
	var partialRows []map[string]any
	if err := json.Unmarshal(partial.Body.Bytes(), &partialRows); err != nil {
		t.Fatal(err)
	}
	if len(partialRows) != 2 {
		t.Fatalf("partial rows = %#v", partialRows)
	}
	if partialRows[0]["referenceId"] != "first" || partialRows[0]["success"] != true || partialRows[1]["referenceId"] != "missing-name" || partialRows[1]["success"] != false {
		t.Fatalf("partial rows = %#v", partialRows)
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("partial success records = %#v", org.Objects["Account"].Records)
	}

	rollback := httptest.NewRecorder()
	handler.ServeHTTP(rollback, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{
  "allOrNone": true,
  "records": [
    {"attributes":{"type":"Account"},"referenceId":"rolled","Name":"Rolled Back"},
    {"attributes":{"type":"Account"},"referenceId":"bad"}
  ]
}`)))
	if rollback.Code != http.StatusBadRequest {
		t.Fatalf("rollback status = %d body=%s", rollback.Code, rollback.Body.String())
	}
	var rollbackRows []map[string]any
	if err := json.Unmarshal(rollback.Body.Bytes(), &rollbackRows); err != nil {
		t.Fatal(err)
	}
	if len(rollbackRows) != 2 || rollbackRows[0]["referenceId"] != "rolled" || rollbackRows[1]["referenceId"] != "bad" {
		t.Fatalf("rollback rows = %#v", rollbackRows)
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("allOrNone rollback leaked records = %#v", org.Objects["Account"].Records)
	}

	missingType := httptest.NewRecorder()
	handler.ServeHTTP(missingType, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite/sobjects", strings.NewReader(`{"records":[{"Name":"No Type"}]}`)))
	if missingType.Code != http.StatusBadRequest {
		t.Fatalf("missing type status = %d body=%s", missingType.Code, missingType.Body.String())
	}
	missingTypeErrors := decodeServerErrors(t, missingType)
	if len(missingTypeErrors) != 1 || missingTypeErrors[0].ErrorCode != "REQUIRED_FIELD_MISSING" {
		t.Fatalf("missing type errors = %#v", missingTypeErrors)
	}
}

func TestCompositeBatchEndpointIsExplicitlyUnsupported(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/services/data/v61.0/composite", strings.NewReader(`{"compositeRequest":[]}`)))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	errors := decodeServerErrors(t, res)
	if len(errors) != 1 || errors[0].ErrorCode != "UNSUPPORTED_COMPOSITE" {
		t.Fatalf("errors = %#v", errors)
	}
}

type serverErrorPayload struct {
	ErrorCode string   `json:"errorCode"`
	Message   string   `json:"message"`
	Fields    []string `json:"fields,omitempty"`
}

func decodeServerErrors(t *testing.T, res *httptest.ResponseRecorder) []serverErrorPayload {
	t.Helper()
	var errors []serverErrorPayload
	if err := json.Unmarshal(res.Body.Bytes(), &errors); err != nil {
		t.Fatal(err)
	}
	return errors
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func testOrgWithAccount(id storage.ID, name string) storage.OrgState {
	org := testOrg()
	account := org.Objects["Account"]
	account.Records[id] = storage.Record{
		ID:     id,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue(name),
		},
	}
	org.Objects["Account"] = account
	return org
}

func addTestUser(org *storage.OrgState, id storage.ID, username string) {
	userObject := org.Objects["User"]
	if userObject.Records == nil {
		userObject.Records = make(map[storage.ID]storage.Record)
	}
	userObject.Records[id] = storage.Record{
		ID:     id,
		Object: "User",
		Fields: map[string]storage.Value{
			"Username": storage.StringValue(username),
			"Email":    storage.StringValue(username),
			"Alias":    storage.StringValue("local"),
			"IsActive": storage.BooleanValue(true),
			"UserType": storage.StringValue("Standard"),
		},
	}
	org.Objects["User"] = userObject
}
