package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecRecordHunterSalesforceScalarDescribeTypes(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('COMBOBOX', Schema.getGlobalDescribe().get('Event').getDescribe().fields.getMap().get('Subject').getDescribe().getType().name());
System.assertEquals('COMBOBOX', Schema.getGlobalDescribe().get('Task').getDescribe().fields.getMap().get('Subject').getDescribe().getType().name());
System.assertEquals(true, Schema.getGlobalDescribe().get('Event').getDescribe().fields.getMap().get('Subject').getDescribe().isNameField());
System.assertEquals(true, Schema.getGlobalDescribe().get('Task').getDescribe().fields.getMap().get('Subject').getDescribe().isNameField());
System.assertEquals('BASE64', Schema.getGlobalDescribe().get('ContentVersion').getDescribe().fields.getMap().get('VersionData').getDescribe().getType().name());
`)
	if err != nil {
		t.Fatal(err)
	}
	vm := New(nil)
	org := storage.NewOrgState()
	for _, objectName := range []string{"Event", "Task", "ContentVersion"} {
		storage.EnsureStandardObject(&org, objectName)
	}
	vm.SetOrg(&org)
	if _, err := vm.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordHunterJSONDatetimePreservesMilliseconds(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime value = (Datetime)JSON.deserialize('"2001-01-01T00:00:00.001Z"', Datetime.class);
System.assertEquals('2001-01-01T00:00:00.001Z', value.formatGmt('yyyy-MM-dd\'T\'HH:mm:ss.SSS\'Z\''));
`)
	if err != nil {
		t.Fatal(err)
	}
	vm := New(nil)
	org := storage.NewOrgState()
	vm.SetOrg(&org)
	if _, err := vm.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordHunterIDCastSupportsSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
String source = '001000000000001AAA';
String objectName = ((ID)source).getSObjectType().getDescribe().getName();
System.assertEquals('Account', objectName);
`)
	if err != nil {
		t.Fatal(err)
	}
	vm := New(nil)
	org := storage.NewOrgState()
	vm.SetOrg(&org)
	if _, err := vm.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordHunterFixedSearchResultsMatchIDWidth(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Fixed Search Record');
insert row;
Test.setFixedSearchResults(new List<Id>{row.Id});
String query = 'FIND {Fixed Search Record} IN ALL FIELDS RETURNING Account(Id WHERE Id IN (\'' + row.Id + '\'))';
List<List<SObject>> rows = Search.query(query);
System.assertEquals(1, rows[0].size());
System.assertEquals(row.Id, rows[0][0].Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	vm := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	vm.SetOrg(&org)
	vm.EnableTestContext()
	if _, err := vm.Execute(program); err != nil {
		t.Fatal(err)
	}
}
