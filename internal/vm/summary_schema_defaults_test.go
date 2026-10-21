package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The record_default_and_list_type conformance fixture captures constructor
// SUM/COUNT values and describe defaults on both routes at API 62/67.
// Bookkeeping flags, populated keys, JSON, null flags, record-type variants,
// inserted-versus-queried records, MIN controls and API 68 remain local coverage.
func TestSummarySchemaDefaultsPendingRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := summarySchemaDefaultsMachine(api)
			token := Object("Schema.SObjectType")
			token.Fields["object"] = String("ParcelSeed__c")
			row, _, _, handled, err := machine.callPlatformObjectMember(token, "newSObject", []Value{Null, Bool(true)}, &Result{})
			if err != nil || !handled {
				t.Fatalf("schema constructor: handled=%v error=%v", handled, err)
			}
			for _, field := range []string{"AmberTotal__c", "BlueTotal__c"} {
				_, value, present := objectFieldValue(row, field)
				if !present || value.Kind != ValueDecimal || value.Decimal != 0 {
					t.Fatalf("%s implicit SUM default=%v, present=%v; want Decimal zero", field, value, present)
				}
				if !isDefaultedSObjectField(row, field) || isExplicitSObjectField(row, field) || isUserSetSObjectField(row, field) {
					t.Fatalf("%s must use implicit-default bookkeeping", field)
				}
			}
			_, count, present := objectFieldValue(row, "CountValue__c")
			if !present || count.Kind != ValueDecimal || count.Decimal != 0 {
				t.Fatalf("COUNT default=%v, present=%v; want Decimal zero", count, present)
			}
			for _, field := range []string{"MinimumValue__c", "MaximumValue__c", "OrdinaryAmount__c"} {
				if _, _, present := objectFieldValue(row, field); present {
					t.Fatalf("unrelated field %s unexpectedly defaulted", field)
				}
			}
			_, configured, present := objectFieldValue(row, "ConfiguredAmount__c")
			if !present || configured.Kind != ValueDecimal || configured.Decimal != 7 {
				t.Fatalf("configured default changed: %v, present=%v", configured, present)
			}
			stored, err := machine.recordFromValue(&row)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"AmberTotal__c", "BlueTotal__c"} {
				if _, present := stored.Fields[field]; present {
					t.Fatalf("implicit readonly SUM %s leaked into DML payload", field)
				}
			}
		})
	}
}

func TestSummarySchemaDefaultsConstructorControls(t *testing.T) {
	for _, args := range [][]Value{nil, {Null}, {Null, Bool(false)}, {platformScalar("Id", "012000000000001"), Bool(true)}} {
		machine := summarySchemaDefaultsMachine("62.0")
		token := Object("Schema.SObjectType")
		token.Fields["object"] = String("ParcelSeed__c")
		row, _, _, handled, err := machine.callPlatformObjectMember(token, "newSObject", args, &Result{})
		if err != nil || !handled {
			t.Fatalf("constructor args=%v: handled=%v error=%v", args, handled, err)
		}
		for _, field := range []string{"AmberTotal__c", "BlueTotal__c"} {
			if _, _, present := objectFieldValue(row, field); present {
				t.Fatalf("constructor args=%v unexpectedly initialized %s", args, field)
			}
		}
	}
}

func TestSummarySchemaDefaultsDMLQueryControls(t *testing.T) {
	program, err := CompileAnonymous(`
ParcelSeed__c ordinary = new ParcelSeed__c(Name='ordinary');
System.assertEquals(null, ordinary.AmberTotal__c);
System.assertEquals(null, ordinary.get('BlueTotal__c'));
insert ordinary;
System.assertEquals(null, ordinary.AmberTotal__c);
System.assertEquals(null, ordinary.get('BlueTotal__c'));
ParcelSeed__c queried = [SELECT AmberTotal__c,BlueTotal__c FROM ParcelSeed__c WHERE Id=:ordinary.Id];
System.assertEquals(0, queried.AmberTotal__c);
System.assertEquals(0, queried.BlueTotal__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := summarySchemaDefaultsMachine("62.0")
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func summarySchemaDefaultsMachine(api string) *VM {
	org := testDataOrg()
	org.APIVersion = api
	org.Objects["ParcelSeed__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "ParcelSeed__c", KeyPrefix: "a53",
			Fields: map[string]storage.Field{
				"Name":                {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId":        {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
				"AmberTotal__c":       {APIName: "AmberTotal__c", Type: storage.FieldSummary, SummaryOperation: "sum", SummarizedField: "ParcelLineSeed__c.Amount__c", SummaryForeignKey: "ParcelLineSeed__c.Parent__c"},
				"BlueTotal__c":        {APIName: "BlueTotal__c", Type: storage.FieldSummary, SummaryOperation: " SuM ", SummarizedField: "ParcelLineSeed__c.Amount__c", SummaryForeignKey: "ParcelLineSeed__c.Parent__c", SummaryFilterItems: []storage.SummaryFilterItem{{Field: "ParcelLineSeed__c.Included__c", Operation: "equals", Value: "True"}}},
				"CountValue__c":       {APIName: "CountValue__c", Type: storage.FieldSummary, SummaryOperation: "count"},
				"MinimumValue__c":     {APIName: "MinimumValue__c", Type: storage.FieldSummary, SummaryOperation: "min"},
				"MaximumValue__c":     {APIName: "MaximumValue__c", Type: storage.FieldSummary, SummaryOperation: "max"},
				"OrdinaryAmount__c":   {APIName: "OrdinaryAmount__c", Type: storage.FieldDecimal},
				"ConfiguredAmount__c": {APIName: "ConfiguredAmount__c", Type: storage.FieldDecimal, DefaultValue: "7"},
			},
			RecordTypes: []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["ParcelLineSeed__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "ParcelLineSeed__c", KeyPrefix: "a54",
			Fields: map[string]storage.Field{
				"Parent__c":   {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"ParcelSeed__c"}, RelationshipName: "Parent__r"},
				"Amount__c":   {APIName: "Amount__c", Type: storage.FieldDecimal},
				"Included__c": {APIName: "Included__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	return machine
}
