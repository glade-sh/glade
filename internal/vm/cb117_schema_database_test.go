package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCB117DescribeMatchesRetainedAPI67Shape(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Probe__c.SObjectType.getDescribe();
System.assertEquals(29, describe.fields.getMap().size());
System.assertEquals(1, describe.fieldSets.getMap().size());
System.assertEquals(2, describe.getRecordTypeInfos().size());
System.assertEquals(SObjectDescribeOptions.DEFERRED, describe.getSObjectDescribeOption());
System.assertEquals(null, describe.getDataTranslationEnabled());

Schema.DescribeFieldResult text = Probe__c.Text__c.getDescribe();
System.assertEquals(80, text.getLength());
System.assertEquals(240, text.getByteLength());
System.assertEquals(null, text.getInlineHelpText());
System.assertEquals(null, text.getRelationshipOrder());

Schema.DescribeFieldResult textArea = Probe__c.TextArea__c.getDescribe();
System.assertEquals(255, textArea.getLength());
System.assertEquals(765, textArea.getByteLength());
System.assertEquals(true, textArea.isSortable());

Schema.DescribeFieldResult longText = Probe__c.LongText__c.getDescribe();
System.assertEquals(256, longText.getLength());
System.assertEquals(768, longText.getByteLength());
System.assertEquals(false, longText.isFilterable());
System.assertEquals(false, longText.isGroupable());
System.assertEquals(false, longText.isAggregatable());

Schema.DescribeFieldResult email = Probe__c.Email__c.getDescribe();
System.assertEquals('EMAIL', email.getType());
System.assertEquals(80, email.getLength());
System.assertEquals(240, email.getByteLength());
Schema.DescribeFieldResult url = Probe__c.Url__c.getDescribe();
System.assertEquals('URL', url.getType());
System.assertEquals(255, url.getLength());
System.assertEquals(765, url.getByteLength());

Schema.DescribeFieldResult autoNumber = Probe__c.Auto__c.getDescribe();
System.assertEquals('STRING', autoNumber.getType());
System.assertEquals(true, autoNumber.isAutoNumber());
System.assertEquals(30, autoNumber.getLength());
System.assertEquals(90, autoNumber.getByteLength());
System.assertEquals(false, autoNumber.isCreateable());
System.assertEquals(false, autoNumber.isUpdateable());
System.assertEquals(false, autoNumber.isNillable());
System.assertEquals(false, autoNumber.isGroupable());

Schema.DescribeFieldResult lookup = Probe__c.Lookup__c.getDescribe();
System.assertEquals(18, lookup.getLength());
System.assertEquals(18, lookup.getByteLength());
System.assertEquals(false, lookup.getFilteredLookupInfo().isDependent());
System.assertEquals(0, lookup.getFilteredLookupInfo().getControllingFields().size());
System.assertEquals(true, lookup.getFilteredLookupInfo().isOptionalFilter());

Schema.DescribeFieldResult formula = Probe__c.Formula__c.getDescribe();
System.assertEquals(1300, formula.getLength());
System.assertEquals(3900, formula.getByteLength());
System.assertEquals(false, formula.isGroupable());

System.assertEquals(true, Probe__c.External__c.getDescribe().isIdLookup());
System.assertEquals(120, Probe__c.External__c.getDescribe().getByteLength());
System.assertEquals(4099, Probe__c.Multi__c.getDescribe().getLength());
System.assertEquals(4099, Probe__c.Multi__c.getDescribe().getByteLength());
System.assertEquals(false, Probe__c.Multi__c.getDescribe().isGroupable());
System.assertEquals(false, Probe__c.Multi__c.getDescribe().isAggregatable());
System.assertEquals(false, Probe__c.Checkbox__c.getDescribe().isNillable());
System.assertEquals(false, Probe__c.Checkbox__c.getDescribe().isAggregatable());
System.assertEquals(null, Probe__c.Checkbox__c.getDescribe().getDefaultValueFormula());
System.assertEquals(0, Probe__c.Number__c.getDescribe().getDigits());
System.assertEquals(false, Probe__c.Number__c.getDescribe().isGroupable());
System.assertEquals(false, Probe__c.DateTime__c.getDescribe().isGroupable());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := cb117DescribeOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB117CursorDMLOptionsAndDateContracts(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Cursor One');
Database.PaginationCursor cursor = Database.getPaginationCursor('SELECT Id FROM Account');
Database.CursorFetchResult page = cursor.fetchPage(0, 10);
System.assertEquals(true, page.isDone());
System.assertEquals(0, page.getNextIndex());

Database.DMLOptions options = new Database.DMLOptions();
System.assertEquals(null, options.LocaleOptions);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func cb117DescribeOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Probe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Probe__c",
			Fields: map[string]storage.Field{
				"Text__c":     {APIName: "Text__c", Type: storage.FieldString, DisplayType: "STRING", Length: 80},
				"Number__c":   {APIName: "Number__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Precision: 12, Scale: 2},
				"Currency__c": {APIName: "Currency__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY", Precision: 12, Scale: 2},
				"Date__c":     {APIName: "Date__c", Type: storage.FieldDate, DisplayType: "DATE"},
				"DateTime__c": {APIName: "DateTime__c", Type: storage.FieldDateTime, DisplayType: "DATETIME"},
				"Checkbox__c": {APIName: "Checkbox__c", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false"},
				"Picklist__c": {APIName: "Picklist__c", Type: storage.FieldPicklist, DisplayType: "PICKLIST"},
				"Multi__c":    {APIName: "Multi__c", Type: storage.FieldMultiPicklist, DisplayType: "MULTIPICKLIST", Precision: 3},
				"External__c": {APIName: "External__c", Type: storage.FieldString, DisplayType: "STRING", Length: 40, ExternalID: true, Unique: true},
				"Formula__c":  {APIName: "Formula__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "Text__c"},
				"Lookup__c":   {APIName: "Lookup__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Account"}, FilteredLookupInfo: storage.FilteredLookupInfo{ControllingFields: []string{"Account.Name"}, Dependent: true, OptionalFilter: true}},
				"Auto__c":     {APIName: "Auto__c", Type: storage.FieldAny, DisplayType: "STRING", AutoNumber: true, DisplayFormat: "PROBE-{0000}"},
				"Email__c":    {APIName: "Email__c", Type: storage.FieldString, DisplayType: "EMAIL"},
				"Url__c":      {APIName: "Url__c", Type: storage.FieldString, DisplayType: "URL"},
				"TextArea__c": {APIName: "TextArea__c", Type: storage.FieldString, DisplayType: "TEXTAREA"},
				"LongText__c": {APIName: "LongText__c", Type: storage.FieldString, DisplayType: "TEXTAREA", Length: 256},
			},
			RecordTypes: []storage.RecordTypeInfo{{DeveloperName: "Business", Name: "Business", Active: true, Available: true}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{ObjectName: "Probe__c", Name: "Summary", Fields: []storage.FieldSetMemberMetadata{{Field: "Text__c"}}}}
	return org
}
