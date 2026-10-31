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

// Supplemental LWC/server host twins for sobject_json3_collision/org.tsv:2;
// native capture is Apex, not a hosted Lightning observation.
func TestLightningTranslatedQueryCollision(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/CollisionController.cls"), `public class CollisionController { @AuraEnabled public static String readLabel() { Account a=[SELECT toLabel(Industry) label FROM Account WHERE Name='One' LIMIT 1]; return (String)a.get('label'); } }`)
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
			org.Metadata.ValueSetTranslations = []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Banking", Translation: "Premier FR"}, {ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Deuxieme FR"}}
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Industry": {APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Agriculture", Label: "Banking"}, {Value: "Banking", Label: "Technology"}}}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("One"), "Industry": storage.StringValue("Agriculture")}}}}
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
				if rec.Code != http.StatusOK || out.Error != nil || out.Data != "Premier FR" {
					t.Fatalf("%s status=%d result=%+v", route.path, rec.Code, out)
				}
			}
		})
	}
}
