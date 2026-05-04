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

func TestSObjectLayoutMetadataEdges(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	for _, path := range []string{
		"/services/data/v61.0/sobjects/Account/layouts",
		"/services/data/v61.0/sobjects/Account/describe/layouts",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertSalesforceError(t, rec, http.StatusNotImplemented, "UNSUPPORTED_FEATURE", "layout metadata")
	}

	compact := httptest.NewRecorder()
	handler.ServeHTTP(compact, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Account/compactLayouts", nil))
	if compact.Code != http.StatusOK {
		t.Fatalf("compact status = %d body=%s", compact.Code, compact.Body.String())
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
		t.Fatalf("compact payload = %#v", payload)
	}
}

func TestDescribeEndpoints(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/services/data", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"v61.0"`)) {
		t.Fatalf("versions status = %d body=%s", list.Code, list.Body.String())
	}

	sobjects := httptest.NewRecorder()
	handler.ServeHTTP(sobjects, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects", nil))
	if sobjects.Code != http.StatusOK || !bytes.Contains(sobjects.Body.Bytes(), []byte(`"Account"`)) {
		t.Fatalf("sobjects status = %d body=%s", sobjects.Code, sobjects.Body.String())
	}
}

func TestResourceDiscoveryIncludesStableServerEndpoints(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v61.0", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("discovery status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"composite": "/services/data/v61.0/composite",
		"limits":    "/services/data/v61.0/limits",
		"oaer":      "/services/data/v61.0/oaer",
		"query":     "/services/data/v61.0/query",
		"queryAll":  "/services/data/v61.0/queryAll",
		"recent":    "/services/data/v61.0/recent",
		"search":    "/services/data/v61.0/search",
		"sobjects":  "/services/data/v61.0/sobjects",
		"tooling":   "/services/data/v61.0/tooling",
	}
	for name, url := range want {
		if payload[name] != url {
			t.Fatalf("discovery[%s] = %q, want %q; payload=%#v", name, payload[name], url, payload)
		}
	}
}

func TestUnsupportedDiscoveryNamespacesReturnStableErrors(t *testing.T) {
	org := testOrg()
	handler := New(&org)
	for _, path := range []string{
		"/services/data/v61.0/search?q=FIND%20%7BAcme%7D",
		"/services/data/v61.0/composite/batch",
		"/services/data/v61.0/composite/tree/Account",
		"/services/data/v61.0/composite/graph",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if path == "/services/data/v61.0/search?q=FIND%20%7BAcme%7D" {
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		}
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) {
			t.Fatalf("%s unsupported shape = %s", path, rec.Body.String())
		}
	}
}

func TestApexRestDispatchReturnsStableUnsupportedError(t *testing.T) {
	org := testOrg()
	handler := New(&org)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/apexrest/widgets/42", strings.NewReader(`{"name":"Acme"}`)))
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("apexrest status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"errorCode":"UNSUPPORTED_FEATURE"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`RestResource dispatch`)) {
		t.Fatalf("apexrest unsupported shape = %s", rec.Body.String())
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
				"Name":        {APIName: "Name", Type: storage.FieldString, Required: true},
				"Description": {APIName: "Description", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
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
