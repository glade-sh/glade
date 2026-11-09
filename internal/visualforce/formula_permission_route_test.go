package visualforce

import (
	"fmt"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"path/filepath"
	"testing"
)

// VF save routes through the same native-captured validation-rule semantics.
func TestVisualforceSaveUsesFormulaPermission(t *testing.T) {
	for _, version := range []string{"59.0", "67.0"} {
		for _, granted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/granted=%v", version, granted), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Name}"/><apex:commandButton value="Save" action="{!save}"/></apex:form></apex:page>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+version+`</apiVersion><label>Edit</label></ApexPage>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				idx, err := LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				const accountID storage.ID = "001000000000001"
				org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString}}, ValidationRules: []storage.ValidationRule{{Name: "Guard", Active: true, ErrorConditionFormula: "NOT($Permission.Bypass)", ErrorMessage: "denied"}}}, Records: map[storage.ID]storage.Record{accountID: {ID: accountID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Original")}}}}
				machine := vm.New(nil)
				machine.SetOrg(&org)
				authorizeFieldRenderingFixture(t, &org, machine)
				authorizeFieldRenderingWriteFixture(t, &org)
				const set storage.ID = "0PS000000000001"
				const cp storage.ID = "0CP000000000001"
				org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{set: {ID: set, Fields: map[string]storage.Value{}}}}
				org.Objects["CustomPermission"] = storage.ObjectState{Records: map[storage.ID]storage.Record{cp: {ID: cp, Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Bypass")}}}}
				org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0SE000000000001": {Fields: map[string]storage.Value{"ParentId": storage.IDValue(set), "SetupEntityId": storage.IDValue(cp)}}}}
				if granted {
					org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(fieldRenderingTestUserID), "PermissionSetId": storage.IDValue(set)}}}}
				}
				_, err = RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Org: &org, Machine: machine, PageName: "Edit", PageURL: "/apex/Edit?id=001000000000001", Action: "{!save}", FormValues: map[string]string{"Name": "Updated"}})
				if granted && err != nil {
					t.Fatalf("granted save: %v", err)
				}
				got := org.Objects["Account"].Records[accountID].Fields["Name"].String
				want := "Original"
				if granted {
					want = "Updated"
				}
				if got != want {
					t.Fatalf("name=%q want=%q err=%v", got, want, err)
				}
			})
		}
	}
}
