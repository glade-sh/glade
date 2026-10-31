package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Supplemental LWC/server host twins for sobject_json3_qualified/org.tsv:2,10;
// native capture is Apex, not a hosted Lightning observation.
func TestLightningQualifiedSemiJoin(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/CollisionController.cls"), `public class CollisionController { @AuraEnabled public static String readLabel() { Account a=[SELECT toLabel(Account.Industry) result FROM Account WHERE toLabel(Account.Industry) = 'Technologie' AND Id IN (SELECT AccountId FROM Opportunity WHERE toLabel(StageName) = 'Prospection FR') LIMIT 1]; return (String)a.get('result'); } }`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/CollisionController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
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
			org := storage.NewOrgState()
			org.Features = []string{"TranslationLanguage-fr"}
			org.Metadata.ValueSetTranslations = []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Technologie"}, {ValueSet: "OpportunityStage", Language: "fr", MasterLabel: "Prospecting", Translation: "Prospection FR"}}
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Industry": {APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology"}}}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("One"), "Industry": storage.StringValue("Technology")}}}}
			org.Objects["Opportunity"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Opportunity", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"}, "StageName": {APIName: "StageName", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Prospecting", Label: "Prospecting"}}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account"}}}, Records: map[storage.ID]storage.Record{"006000000000001": {ID: "006000000000001", Object: "Opportunity", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001"), "StageName": storage.StringValue("Prospecting")}}}}

			const userID = "005000000000777AAA"
			addUser(&org, userID, "owned@example.test", "owned@example.test", "Synthetic")
			users := org.Objects["User"]
			user := users.Records[userID]
			user.Fields["LanguageLocaleKey"] = storage.StringValue("fr")
			users.Records[userID] = user
			org.Objects["User"] = users
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(typesys.Build(p, schema))
			for _, route := range []struct{ path, body string }{{"/lightning/wire/apex", `{"className":"CollisionController","method":"readLabel","params":{}}`}, {"/lightning/apex/CollisionController/readLabel", `{}`}} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-GLADE-User-Id", userID)
				handler.ServeHTTP(rec, req)
				var out lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusOK || out.Error != nil || out.Data != "Technologie" {
					t.Fatalf("%s status=%d result=%+v", route.path, rec.Code, out)
				}
			}
		})
	}
}
