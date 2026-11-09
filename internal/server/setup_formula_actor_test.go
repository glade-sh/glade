package server

import (
	"encoding/json"
	"github.com/glade-sh/glade/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSetupFormulaLocalQueryUsesSelectedUser(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	actor := storage.ID("005000000000002")
	addUser(&org, actor, "synthetic@example.invalid", "synthetic@example.invalid", "Synthetic User")
	def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	addAccountForTest(&org, "001000000000001", "Probe")
	account := org.Objects["Account"]
	account.Definition.Fields["SetupFlag__c"] = storage.Field{APIName: "SetupFlag__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}
	org.Objects["Account"] = account
	handler := New(&org)
	query := "SELECT SetupFlag__c FROM Account"
	for _, tc := range []struct {
		name, user string
		want       bool
	}{{"default", "", true}, {"actor", string(actor), false}} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, serverTestDataPath+"/query?q="+url.QueryEscape(query), nil)
			if tc.user != "" {
				req.Header.Set("X-GLADE-User-Id", tc.user)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("REST status=%d body=%s", rec.Code, rec.Body.String())
			}
			var payload struct {
				Records []map[string]any `json:"records"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Records) != 1 || payload.Records[0]["SetupFlag__c"] != tc.want {
				t.Errorf("REST rows=%#v want=%v", payload.Records, tc.want)
			}
			body := `{"operation":"query","query":"SELECT SetupFlag__c FROM Account"}`
			bulk := httptest.NewRequest(http.MethodPost, serverTestDataPath+"/jobs/query", strings.NewReader(body))
			if tc.user != "" {
				bulk.Header.Set("X-GLADE-User-Id", tc.user)
			}
			created := httptest.NewRecorder()
			handler.ServeHTTP(created, bulk)
			if created.Code != http.StatusOK {
				t.Fatalf("bulk create status=%d body=%s", created.Code, created.Body.String())
			}
			var job struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(created.Body.Bytes(), &job); err != nil {
				t.Fatal(err)
			}
			rows := httptest.NewRecorder()
			handler.ServeHTTP(rows, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/jobs/query/"+job.ID+"/results", nil))
			expected := "false"
			if tc.want {
				expected = "true"
			}
			if rows.Code != http.StatusOK || !strings.Contains(strings.ToLower(rows.Body.String()), expected) {
				t.Fatalf("bulk status=%d rows=%s want=%s", rows.Code, rows.Body.String(), expected)
			}
		})
	}
}
