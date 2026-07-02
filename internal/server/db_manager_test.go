package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dbmanager"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDBManagerServesBrowserAssets(t *testing.T) {
	org := dbManagerTestOrg()
	handler := New(&org)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/db", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d body=%s", page.Code, page.Body.String())
	}
	if got := page.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("page Content-Type = %q", got)
	}
	if body := page.Body.String(); !strings.Contains(body, "Glade Local Data") || !strings.Contains(body, "/db/assets/app.js") {
		t.Fatalf("page body missing DB manager shell markers: %s", body)
	}

	app := httptest.NewRecorder()
	handler.ServeHTTP(app, httptest.NewRequest(http.MethodGet, "/db/assets/app.js", nil))
	if app.Code != http.StatusOK {
		t.Fatalf("app status = %d body=%s", app.Code, app.Body.String())
	}
	body := app.Body.String()
	for _, marker := range []string{"renderObjectRail", "renderObjectFilters", "objectMatchesFilter", "Local Data", "Data", "Setup", "renderRecordForm", "createRecord", "updateRecord", "lookupSearch"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("app.js missing marker %q", marker)
		}
	}
}

func TestDBManagerAPIListsObjectsFieldsAndRecords(t *testing.T) {
	org := dbManagerTestOrg()
	handler := New(&org)

	discovery := httptest.NewRecorder()
	handler.ServeHTTP(discovery, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/glade/db-manager", nil))
	if discovery.Code != http.StatusOK || !strings.Contains(discovery.Body.String(), "/glade/db-manager/objects") {
		t.Fatalf("discovery status=%d body=%s", discovery.Code, discovery.Body.String())
	}

	objects := httptest.NewRecorder()
	handler.ServeHTTP(objects, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/glade/db-manager/objects?q=Acc", nil))
	if objects.Code != http.StatusOK {
		t.Fatalf("objects status = %d body=%s", objects.Code, objects.Body.String())
	}
	var objectList dbmanager.ObjectList
	if err := json.Unmarshal(objects.Body.Bytes(), &objectList); err != nil {
		t.Fatal(err)
	}
	if len(objectList.Objects) != 1 || objectList.Objects[0].Name != "Account" || objectList.Objects[0].Records != 1 {
		t.Fatalf("object list = %#v", objectList)
	}
	if objectList.Objects[0].Category != "standard" || !objectList.Objects[0].Capabilities.Createable {
		t.Fatalf("object metadata = %#v", objectList.Objects[0])
	}

	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/glade/db-manager/objects/Account", nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detail.Code, detail.Body.String())
	}
	var objectDetail dbmanager.ObjectDetail
	if err := json.Unmarshal(detail.Body.Bytes(), &objectDetail); err != nil {
		t.Fatal(err)
	}
	if objectDetail.Category != "standard" || !objectDetail.Capabilities.Createable {
		t.Fatalf("object detail metadata = %#v", objectDetail)
	}
	assertDBManagerField(t, objectDetail, "Industry", "picklist")
	assertDBManagerField(t, objectDetail, "Services__c", "multipicklist")
	assertDBManagerField(t, objectDetail, "OwnerId", "lookup")

	records := httptest.NewRecorder()
	handler.ServeHTTP(records, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/glade/db-manager/objects/Account/records?q=Acme", nil))
	if records.Code != http.StatusOK {
		t.Fatalf("records status = %d body=%s", records.Code, records.Body.String())
	}
	var recordList dbmanager.RecordList
	if err := json.Unmarshal(records.Body.Bytes(), &recordList); err != nil {
		t.Fatal(err)
	}
	if recordList.Total != 1 || len(recordList.Records) != 1 || recordList.Records[0].Title != "Acme" {
		t.Fatalf("record list = %#v", recordList)
	}
}

func TestDBManagerAPIMutationsPersistThroughStore(t *testing.T) {
	org := dbManagerTestOrg()
	store := &memoryStore{}
	handler := NewWithStore(&org, store)

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, serverTestDataPath+"/glade/db-manager/objects/Account/records", strings.NewReader(`{"fields":{"Name":{"state":"value","value":"New Account"},"Industry":{"state":"value","value":"Technology"}}}`)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", create.Code, create.Body.String())
	}
	var created dbmanager.MutationResult
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !created.Success || created.ID == "" || !created.Created || store.saves != 1 {
		t.Fatalf("created = %#v saves=%d", created, store.saves)
	}

	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPatch, serverTestDataPath+"/glade/db-manager/objects/Account/records/"+created.ID, strings.NewReader(`{"fields":{"Description":{"state":"null"}}}`)))
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", update.Code, update.Body.String())
	}
	if store.saves != 2 {
		t.Fatalf("store saves after update = %d", store.saves)
	}
	stored := org.Objects["Account"].Records[storage.ID(created.ID)]
	if !stored.ExplicitNulls["Description"] {
		t.Fatalf("Description null not stored: %#v", stored)
	}

	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, httptest.NewRequest(http.MethodDelete, serverTestDataPath+"/glade/db-manager/objects/Account/records/"+created.ID, nil))
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	if store.saves != 3 || !org.Objects["Account"].Records[storage.ID(created.ID)].System.IsDeleted {
		t.Fatalf("delete did not persist: saves=%d record=%#v", store.saves, org.Objects["Account"].Records[storage.ID(created.ID)])
	}
}

func TestDBManagerAPILookupSearch(t *testing.T) {
	org := dbManagerTestOrg()
	handler := New(&org)

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/glade/db-manager/lookup?object=Account&field=OwnerId&q=Ada", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("lookup status = %d body=%s", res.Code, res.Body.String())
	}
	var result dbmanager.LookupResult
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 1 || result.Records[0].ID != "005000000000001" || result.Records[0].Title != "Ada User" {
		t.Fatalf("lookup = %#v", result)
	}
}

func assertDBManagerField(t *testing.T, detail dbmanager.ObjectDetail, name, control string) {
	t.Helper()
	for _, field := range detail.Fields {
		if field.Name == name {
			if field.Control != control {
				t.Fatalf("%s control = %q, want %q", name, field.Control, control)
			}
			return
		}
	}
	t.Fatalf("field %s not found in %#v", name, detail.Fields)
}

func dbManagerTestOrg() storage.OrgState {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.PluralLabel = "Accounts"
	account.Definition.Fields["Industry"] = storage.Field{
		APIName: "Industry",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Technology", Label: "Technology", Active: true},
		},
	}
	account.Definition.Fields["Services__c"] = storage.Field{
		APIName: "Services__c",
		Type:    storage.FieldMultiPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Implementation", Label: "Implementation", Active: true},
			{Value: "Support", Label: "Support", Active: true},
		},
	}
	account.Definition.Fields["OwnerId"] = storage.Field{APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"}}
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Acme"),
			"Industry": storage.StringValue("Technology"),
			"OwnerId":  storage.IDValue("005000000000001"),
		},
	}
	account.Records["001000000000002"] = storage.Record{
		ID:     "001000000000002",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Deleted Account"),
		},
		System: storage.SystemFields{IsDeleted: true},
	}
	org.Objects["Account"] = account
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "User",
			Label:     "User",
			KeyPrefix: "005",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"Username": {APIName: "Username", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"005000000000001": {
				ID:     "005000000000001",
				Object: "User",
				Fields: map[string]storage.Value{
					"Name":     storage.StringValue("Ada User"),
					"Username": storage.StringValue("ada@example.test"),
				},
			},
		},
	}
	return org
}
