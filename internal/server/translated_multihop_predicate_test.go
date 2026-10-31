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

// Supplemental LWC/server host twins for sobject_json3_multihop/org.tsv:2;
// native capture is Apex, not a hosted Lightning observation.
func TestLightningTranslatedMultiHopPredicate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/CollisionController.cls"), `public class CollisionController { @AuraEnabled public static String readLabel() { Contact c=[SELECT toLabel(Contact.Account.Parent.Industry) result FROM Contact WHERE toLabel(Contact.Account.Parent.Industry) = 'Technologie' LIMIT 1]; return (String)c.getSObject('Account').getSObject('Parent').get('result'); } }`)
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
			org.Metadata.ValueSetTranslations = []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Technologie"}}
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent"}, "Industry": {APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology"}}}}, Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Account"}, ParentRelationship: "Parent"}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Child"), "ParentId": storage.IDValue("001000000000002")}}, "001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent"), "Industry": storage.StringValue("Technology")}}}}
			org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account"}}}, Records: map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001")}}}}
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
