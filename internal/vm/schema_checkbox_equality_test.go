package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func schemaCheckboxEqualityTestOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["LyricTile__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "LyricTile__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
			"Name":            {APIName: "Name", Type: storage.FieldString},
			"VerseSwitch__c":  {APIName: "VerseSwitch__c", Type: storage.FieldBoolean, DefaultValue: "false"},
			"TempoZero__c":    {APIName: "TempoZero__c", Type: storage.FieldDecimal, DefaultValue: "0"},
			"BlankCaption__c": {APIName: "BlankCaption__c", Type: storage.FieldString, DefaultValue: `""`},
		}},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

// Native loaded-default Checkbox rows confirm equality and unmutated clone
// Set/Map lookup behavior. Missing versus explicit null controls come from
// the owned SObject value observations.
func TestSchemaLoadedCheckboxDefaultEquality(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, isTest := range []bool{false, true} {
			for _, assignment := range []string{
				"left.put('VerseSwitch__c',left.get('VerseSwitch__c'));",
				"left.put(LyricTile__c.VerseSwitch__c,false);",
				"left.put('vErSeSwItCh__c',false);",
			} {
				t.Run(fmt.Sprintf("%s/isTest=%t/%s", api, isTest, assignment), func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(`
SObject left=LyricTile__c.SObjectType.newSObject(null,true);
left.put('Name','Equality seed');
SObject right=left.clone(true,true,true,true);
Set<SObject> records=new Set<SObject>{right};
Map<SObject,String> values=new Map<SObject,String>{right=>'present'};
Integer originalHash=left.hashCode();
String serialized=JSON.serialize(left);
Map<String,Object> populated=left.getPopulatedFieldsAsMap();
`+assignment+`
Boolean equal=left==right;
Boolean reverse=right.equals(left);
Boolean sameHash=left.hashCode()==right.hashCode();
Boolean hashStable=originalHash==left.hashCode();
Boolean member=records.contains(left);
String mapped=values.get(left);

Account missing=new Account();
Account explicitNull=new Account(Name=null);
System.assertNotEquals(missing,explicitNull);
System.assertEquals(false,missing.equals(explicitNull));
System.assertNotEquals(missing.hashCode(),explicitNull.hashCode());
`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := schemaCheckboxEqualityTestOrg()
					machine := New(nil)
					machine.SetOrg(&org)
					if isTest {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"equal", "reverse", "sameHash", "hashStable", "member"} {
						if value := machine.Globals[name]; value.Kind != ValueBool || !value.Bool {
							t.Fatalf("loaded Checkbox equality/hash %s: %#v", name, value)
						}
					}
					if value := machine.Globals["mapped"]; value.Kind != ValueString || value.Text != "present" {
						t.Fatalf("loaded Checkbox map lookup: %#v", value)
					}
					if text := machine.Globals["serialized"].Text; strings.Contains(text, "__glade_") {
						t.Fatalf("equality marker leaked into JSON: %s", text)
					}
					for _, key := range machine.Globals["populated"].MapKeys {
						if strings.HasPrefix(key.Text, "__glade_") {
							t.Fatalf("equality marker became a field: %#v", key)
						}
					}
				})
			}
		}
	}
}

// Auxiliary equality presence must survive unmarking unrelated system fields,
// including a Schema value with no caller-assigned fields at all.
func TestSchemaCheckboxEqualityPresenceSurvivesSystemBookkeeping(t *testing.T) {
	program, err := CompileAnonymous(`SObject row=LyricTile__c.SObjectType.newSObject(null,true);`)
	if err != nil {
		t.Fatal(err)
	}
	org := schemaCheckboxEqualityTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	row := machine.Globals["row"]
	before := sObjectHashCode(row, map[uint64]bool{})
	putSystemFields(row, storage.SystemFields{})
	unmarkSObjectFieldMarker(&row, sobjectExplicitFieldsField, "Name")
	if !schemaLoadedCheckboxEqualityField(row, "VerseSwitch__c", Bool(false)) {
		t.Fatal("system bookkeeping erased loaded Checkbox equality presence")
	}
	if after := sObjectHashCode(row, map[uint64]bool{}); after != before {
		t.Fatalf("system bookkeeping changed default-only hash: %d -> %d", before, after)
	}
}

func TestSchemaCheckboxEqualityWithoutLoadedDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
SObject left=LyricTile__c.SObjectType.newSObject(null,false);
SObject right=left.clone(true,true,true,true);
System.assertEquals(false,left.getPopulatedFieldsAsMap().containsKey('VerseSwitch__c'));
Map<String,Object> serialized=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(left));
System.assertEquals(false,serialized.containsKey('VerseSwitch__c'));
left.put('VerseSwitch__c',false);
System.assertNotEquals(left,right);
System.assertEquals(false,left.getPopulatedFieldsAsMap().get('VerseSwitch__c'));
LyricTile__c ordinary=new LyricTile__c(Name='ordinary');
LyricTile__c before=ordinary.clone(true,true,true,true);
ordinary.put('VerseSwitch__c',false);
System.assertNotEquals(ordinary,before);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := schemaCheckboxEqualityTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaLoadedZeroAndEmptyMapLookupsRespectEquality(t *testing.T) {
	for _, field := range []string{"TempoZero__c", "BlankCaption__c"} {
		t.Run(field, func(t *testing.T) {
			program, err := CompileAnonymous(`
SObject left=LyricTile__c.SObjectType.newSObject(null,true);
SObject right=left.clone(true,true,true,true);
Set<SObject> records=new Set<SObject>{right};
Map<SObject,String> keyed=new Map<SObject,String>{right=>'present'};
left.put('` + field + `',left.get('` + field + `'));
System.assertNotEquals(left,right);
System.assertEquals(false,records.contains(left));
System.assertEquals(null,keyed.get(left));
System.assertEquals('present',keyed.get(right));
`)
			if err != nil {
				t.Fatal(err)
			}
			org := schemaCheckboxEqualityTestOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
