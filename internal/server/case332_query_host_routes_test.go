package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const case332HostApex = `List<Account> matches=[SELECT Id FROM Account WHERE Name LIKE 'A%'];
List<AggregateResult> groups=[SELECT Name, COUNT(Id) total FROM Account GROUP BY Name];
List<Account> parents=[SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Name='Alpha'];
List<Account> dates=[SELECT Id FROM Account WHERE CreatedDate=LAST_N_DAYS:2];
return String.valueOf(matches.size())+'|'+String.valueOf(groups.size())+'|'+String.valueOf(parents[0].Contacts.size())+'|'+String.valueOf(dates.size());`

func case332HostOrg() storage.OrgState {
	org := testOrg()
	addAccountForTest(&org, "001000000000001", "Alpha")
	account := org.Objects["Account"]
	row := account.Records["001000000000001"]
	row.System.CreatedDate = "2000-01-01T00:00:00Z"
	account.Records[row.ID] = row
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}, "AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}}}, Records: map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Child"), "AccountId": storage.IDValue("001000000000001")}}}}
	return org
}

func TestCase332ServerAndLightningQueryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/Case332Controller.cls"), `public class Case332Controller { @AuraEnabled public static String runQueries() { `+case332HostApex+` } }`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/Case332Controller.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := case332HostOrg()
			const userID = "005000000000777AAA"
			addUser(&org, userID, "owned@example.test", "owned@example.test", "Synthetic")
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(typesys.Build(p, schema))
			for _, route := range []struct{ path, body string }{{"/lightning/wire/apex", `{"className":"Case332Controller","method":"runQueries","params":{}}`}, {"/lightning/apex/Case332Controller/runQueries", `{}`}} {
				for i := 0; i < 2; i++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-GLADE-User-Id", userID)
					handler.ServeHTTP(rec, req)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || out.Error != nil || out.Data != "1|1|1|0" {
						t.Fatalf("%s run %d status %d result %+v", route.path, i, rec.Code, out)
					}
				}
			}
			rest := New(&org)
			for _, q := range []string{"SELECT Name FROM Account WHERE Name LIKE 'A%'", "SELECT Name, COUNT(Id) total FROM Account GROUP BY Name", "SELECT Id,(SELECT Id FROM Contacts) FROM Account", "SELECT Id FROM Account WHERE CreatedDate=LAST_N_DAYS:2"} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, serverTestDataPath+"/query?q="+url.QueryEscape(q), nil)
				rest.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("REST %s: %d %s", q, rec.Code, rec.Body.String())
				}
			}
		})
	}
}
