package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecDescribeRecordTypeInfoAndFeedContracts(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(true, Account.SObjectType.getDescribe().isFeedEnabled());
System.assertEquals('Map{Master=Schema.RecordTypeInfo[getDeveloperName=Master;getName=Master;getRecordTypeId=012000000000000AAA;isActive=true;isAvailable=true;isDefaultRecordTypeMapping=true;isMaster=true;]}',
		String.valueOf(Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	definition, ok := storage.StandardObjectDefinition("Account")
	if !ok {
		t.Fatal("Account standard definition missing")
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
