package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCase332VisualforceQueryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/pages/Case332.page"), `<apex:page controller="Case332Controller"><apex:pageMessages/></apex:page>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(`
List<Account> matches=[SELECT Id FROM Account WHERE Name LIKE 'A%'];
List<AggregateResult> groups=[SELECT Name, COUNT(Id) total FROM Account GROUP BY Name];
List<Account> parents=[SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Name='Alpha'];
List<Account> dates=[SELECT Id FROM Account WHERE CreatedDate=LAST_N_DAYS:2];
String result=String.valueOf(matches.size())+'|'+String.valueOf(groups.size())+'|'+String.valueOf(parents[0].Contacts.size())+'|'+String.valueOf(dates.size());
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO,result)); return null;
`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Alpha")}, System: storage.SystemFields{CreatedDate: "2000-01-01T00:00:00Z"}}}}
			org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}, Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}}}, Records: map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001")}}}}
			machine := testRunner(t)
			machine.SetOrg(&org)
			if err := machine.RegisterClass(vm.Class{Name: "Case332Controller", Methods: map[string]vm.Method{"save": {Name: "Case332Controller.save", ClassName: "Case332Controller", ReturnType: "PageReference", Program: program}}}); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "Case332", PageURL: "/apex/Case332", Action: "{!save}"})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(result.HTML, "1|1|1|0") {
					t.Fatalf("run %d: %s", i, result.HTML)
				}
			}
		})
	}
}
