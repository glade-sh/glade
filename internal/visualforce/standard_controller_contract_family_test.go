package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderStandardControllerContractFamilyRequestLifecycle(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ControllerFamily.page"), `<apex:page standardController="Account">
  <apex:form>
    <apex:outputText value="{!Account.Id}"/>
    <apex:outputText value="{!Account.Name}"/>
    <apex:inputField value="{!Account.Name}"/>
    <apex:commandButton value="Save" action="{!save}"/>
    <apex:commandButton value="Delete" action="{!delete}"/>
  </apex:form>
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ControllerFamily.page-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>67.0</apiVersion>
  <label>ControllerFamily</label>
</ApexPage>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}

	const recordID storage.ID = "001000000000001"
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			recordID: {
				ID:     recordID,
				Object: "Account",
				Fields: map[string]storage.Value{"Name": storage.StringValue("Request Seed")},
			},
		},
	}
	pageURL := "/apex/ControllerFamily?id=" + string(recordID)
	render := func(action string, formValues map[string]string) PageRenderResult {
		t.Helper()
		machine := vm.New(nil)
		machine.SetOrg(&org)
		authorizeFieldRenderingFixture(t, &org, machine)
		authorizeFieldRenderingWriteFixture(t, &org)
		result, err := RenderPage(PageRenderRequest{
			Project:    p,
			VFIndex:    idx,
			Org:        &org,
			Machine:    machine,
			PageName:   "ControllerFamily",
			PageURL:    pageURL,
			Action:     action,
			FormValues: formValues,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	initial := render("", nil)
	if !strings.Contains(initial.HTML, string(recordID)) || !strings.Contains(initial.HTML, "Request Seed") {
		t.Fatalf("initial standard-controller page did not bind the URL record: %s", initial.HTML)
	}

	_ = render("{!save}", map[string]string{"Name": "Request Saved"})
	stored := org.Objects["Account"].Records[recordID]
	if got := stored.Fields["Name"].String; got != "Request Saved" {
		t.Fatalf("stored Account.Name = %q, want request-bound save", got)
	}

	_ = render("{!delete}", nil)
	deleted, exists := org.Objects["Account"].Records[recordID]
	if !exists || !deleted.System.IsDeleted {
		t.Fatalf("standard-controller delete did not soft-delete record %s", recordID)
	}
	remaining, err := soql.ParseAndExecute(org, "SELECT Id FROM Account WHERE Id = '"+string(recordID)+"'")
	if err != nil {
		t.Fatal(err)
	}
	if remaining.Rows != 0 {
		t.Fatalf("standard-controller deleted record remains in ordinary SOQL: %#v", remaining)
	}
}
