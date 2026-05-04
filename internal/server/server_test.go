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
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"v61.0"`)) {
		t.Fatalf("versions status = %d body=%s", list.Code, list.Body.String())
	}

	sobjects := httptest.NewRecorder()
	handler.ServeHTTP(sobjects, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects", nil))
	if sobjects.Code != http.StatusOK || !bytes.Contains(sobjects.Body.Bytes(), []byte(`"Account"`)) {
		t.Fatalf("sobjects status = %d body=%s", sobjects.Code, sobjects.Body.String())
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

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/services/data/v61.0/sobjects/Missing__c", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	var errors []map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &errors); err != nil {
		t.Fatal(err)
	}
	if len(errors) != 1 || errors[0]["errorCode"] != "NOT_FOUND" || errors[0]["message"] == "" {
		t.Fatalf("errors = %#v", errors)
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
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
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
