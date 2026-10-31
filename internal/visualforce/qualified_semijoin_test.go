package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Supplemental VF host twin for the collision observed at APIs 62/67 in
// sobject_json3_qualified/org.tsv:2,10; no VF-native capture is claimed.
func TestVisualforceQualifiedSemiJoin(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/pages/Collision.page"), `<apex:page controller="CollisionController"><apex:pageMessages/></apex:page>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(`
Account a=[SELECT toLabel(Account.Industry) result FROM Account WHERE toLabel(Account.Industry) = 'Technologie' AND Id IN (SELECT AccountId FROM Opportunity WHERE toLabel(StageName) = 'Prospection FR') LIMIT 1];
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, (String)a.get('result'))); return null;
`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Features = []string{"TranslationLanguage-fr"}
			org.Metadata.ValueSetTranslations = []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Technologie"}, {ValueSet: "OpportunityStage", Language: "fr", MasterLabel: "Prospecting", Translation: "Prospection FR"}}
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Industry": {APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology"}}}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("One"), "Industry": storage.StringValue("Technology")}}}}
			org.Objects["Opportunity"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Opportunity", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"}, "StageName": {APIName: "StageName", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Prospecting", Label: "Prospecting"}}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account"}}}, Records: map[storage.ID]storage.Record{"006000000000001": {ID: "006000000000001", Object: "Opportunity", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001"), "StageName": storage.StringValue("Prospecting")}}}}

			machine := testRunner(t)
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LanguageLocaleKey": storage.StringValue("fr")}})
			if err := machine.RegisterClass(vm.Class{Name: "CollisionController", Methods: map[string]vm.Method{"save": {Name: "CollisionController.save", ClassName: "CollisionController", ReturnType: "PageReference", Program: program}}}); err != nil {
				t.Fatal(err)
			}
			result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "Collision", PageURL: "/apex/Collision", Action: "{!save}"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.HTML, "Technologie") {
				t.Fatalf("collision translation missing: %s", result.HTML)
			}
		})
	}
}
