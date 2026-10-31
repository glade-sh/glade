package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The tooling endpoint carries the same captured admin statement into the VM.
// Native source: review2/org.tsv and diagnostics.tsv at APIs 62 and 67.
func TestPersonAccountBusinessRecordTypeServerNativeConformance(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "conformance", "person_account_followups", "review2")
	read := func(name string) [][]string {
		f, err := os.Open(filepath.Join(fixture, name))
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
		return rows
	}
	outcomes, messages := read("org.tsv"), read("diagnostics.tsv")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			expectedStatus, expectedMessage := "", ""
			for _, row := range outcomes[1:] {
				if row[0] == api && row[1] == "anon" && row[2] == "statement_insert_null_business_type" {
					expectedStatus = row[3]
				}
			}
			for _, row := range messages[1:] {
				if row[0] == api && row[1] == "anon" && row[2] == "statement_insert_null_business_type" {
					expectedMessage = row[3]
				}
			}
			if expectedStatus != "INVALID_FIELD_FOR_INSERT_UPDATE:LastName" || expectedMessage == "" {
				t.Fatal("missing native row")
			}
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business_Default", Name: "Business Default", Active: true, Available: true}, {ID: "012000000000002", DeveloperName: "Patient", Name: "Patient", Active: true, Available: true, PersonAccount: true}}
			storage.EnsureStandardObjectFieldsForFeatures(&account.Definition, []string{"PersonAccounts"})
			if _, ok := storage.ResolveFieldName(account.Definition, "", "RecordTypeId"); !ok {
				t.Fatal("person account fixture missing RecordTypeId")
			}
			org.Objects["Account"] = account
			source := `Account a=new Account(Name='Business',RecordTypeId='012000000000001'); a.LastName=null; try { insert a; System.assert(false); } catch(DmlException e) { System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE:LastName',String.valueOf(e.getDmlStatusCode(0))+':'+String.join(e.getDmlFieldNames(0),',')); System.assertEquals('Account: bad field names on insert/update call: LastName',e.getDmlMessage(0)); }`
			payload, err := json.Marshal(map[string]string{"anonymousBody": source})
			if err != nil {
				t.Fatal(err)
			}
			handler := New(&org)
			// The server analyzes against a project index, independently of the
			// runtime org; expose the configured Account fields to both layers.
			handler.SetProjectIndex(typesys.Index{Objects: []schema.Object{{Name: "Account", Partial: true, Fields: []schema.Field{
				{Name: "RecordTypeId", Type: "Lookup", ReferenceTo: []string{"RecordType"}},
				{Name: "LastName", Type: "Text"},
			}}}})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
			if api == "62.0" {
				assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "unsupported API version v62.0")
				return
			}
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
