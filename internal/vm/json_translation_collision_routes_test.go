package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Supplemental route test: collision output is captured in
// sobject_json3_collision/org.tsv:2; static, locator and child toLabel paths
// are independently captured in sobject_json3/routes-org.tsv:2-5.
func TestJSONStandardValueTranslationCollisionProjectionRoutes(t *testing.T) {
	const source = `
Account parent = new Account(Name='Parent'); insert parent;
Account child = new Account(Name='Child', ParentId=parent.Id, Industry='Agriculture'); insert child;
Id childId=child.Id, parentId=parent.Id;
Account direct=[SELECT toLabel(Industry) label FROM Account WHERE Id=:childId];
System.assertEquals('Premier FR', direct.get('label'));
Database.QueryLocatorIterator it=Database.getQueryLocator('SELECT toLabel(Industry) label FROM Account WHERE Id=:childId').iterator();
System.assertEquals('Premier FR', it.next().get('label'));
Account p=[SELECT Id, (SELECT toLabel(Industry) label FROM ChildAccounts) FROM Account WHERE Id=:parentId];
System.assertEquals('Premier FR', p.getSObjects('ChildAccounts')[0].get('label'));
`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			account := org.Objects["Account"]
			parent := account.Definition.Fields["ParentId"]
			parent.ChildRelationshipName = "ChildAccounts"
			account.Definition.Fields["ParentId"] = parent
			account.Definition.Fields["Industry"] = storage.Field{APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Agriculture", Label: "Banking"}, {Value: "Banking", Label: "Technology"}}}
			org.Objects["Account"] = account
			org.Features = []string{"TranslationLanguage-fr"}
			org.Metadata.ValueSetTranslations = []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Banking", Translation: "Premier FR"}, {ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Deuxieme FR"}}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LanguageLocaleKey": storage.StringValue("fr")}})
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
