package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFlowVersionCatalogLinksDefinition(t *testing.T) {
	org := testDataOrg()
	org.Metadata.Flows = []storage.FlowRule{{Name: "SignalRoute", Active: true, SourceStatus: "Active", APIVersion: "62.0"}}
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
FlowDefinitionView definition=[SELECT ActiveVersionId FROM FlowDefinitionView WHERE ApiName='SignalRoute'];
Id active=definition.ActiveVersionId;
FlowVersionView version=[SELECT DurableId FROM FlowVersionView WHERE DurableId=:active];
System.assertNotEquals(null,version.DurableId);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
