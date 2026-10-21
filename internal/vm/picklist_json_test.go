package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func picklistJSONTestOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["ChoiceGrid__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "ChoiceGrid__c", Fields: map[string]storage.Field{
		"Controlling__c": {APIName: "Controlling__c", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "North", Active: true}, {Value: "South", Active: true}, {Value: "East", Active: true}, {Value: "West", Active: true}}},
		"Dependent__c": {APIName: "Dependent__c", Type: storage.FieldPicklist, PicklistController: "Controlling__c", PicklistValues: []storage.PicklistValue{{Value: "Unmapped", Active: true}, {Value: "First", Active: true}, {Value: "Second", Active: true}, {Value: "Third", Active: true}}, PicklistValueSettings: []storage.PicklistSetting{
			{ValueName: "First", ControllingFieldValues: []string{"South", "East", "West"}},
			{ValueName: "Second", ControllingFieldValues: []string{"East", "West"}},
			{ValueName: "Third", ControllingFieldValues: []string{"West"}},
		}},
	}}}
	return org
}

func TestPicklistJSONCacheAndSourceBoundary(t *testing.T) {
	org := picklistJSONTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	// API68 is an unchanged local control, not a captured native omission claim.
	for i, api := range []string{"62.0", "61.0", "68.0", "67.0", "61.0"} {
		t.Run(fmt.Sprintf("%d/%s", i, api), func(t *testing.T) {
			focal := `
System.assertEquals(true,ordinary.containsKey('validFor'));
System.assertEquals(null,ordinary.get('validFor'));
System.assertEquals(true,unmapped.containsKey('validFor'));
System.assertEquals('',unmapped.get('validFor'));
System.assertEquals('cAAA',mapped.get('validFor'));
`
			if api == "68.0" {
				focal = `
System.assertEquals(false,ordinary.containsKey('validFor'));
System.assertEquals(false,unmapped.containsKey('validFor'));
System.assertEquals(false,mapped.containsKey('validFor'));
`
			}
			program, err := CompileAnonymousWithOptions(`
List<Schema.PicklistEntry> entries=ChoiceGrid__c.Dependent__c.getDescribe().getPicklistValues();
List<Schema.PicklistEntry> controls=ChoiceGrid__c.Controlling__c.getDescribe().getPicklistValues();
Map<String,Object> ordinary=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(controls[0]));
Map<String,Object> unmapped=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(entries[0]));
Map<String,Object> mapped=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(entries[1]));
`+focal+`
System.assertEquals(false,mapped.containsKey('__glade_picklist_valid_for'));
System.assertEquals('First',mapped.get('value'));
System.assertEquals('First',mapped.get('label'));
System.assertEquals(true,mapped.get('active'));
System.assertEquals(false,mapped.get('defaultValue'));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			for _, entry := range machine.Globals["entries"].List {
				if _, _, present := objectFieldValue(entry, "validFor"); present {
					t.Fatal("JSON projection changed a direct entry property")
				}
			}
		})
	}
}

func TestPicklistJSONPreservesOtherObjects(t *testing.T) {
	machine := New(nil)
	org := picklistJSONTestOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "PicklistCarrierDTO", Fields: map[string]Field{
		"validFor":                   {Name: "validFor", Type: "String", Access: "public"},
		"__glade_picklist_valid_for": {Name: "__glade_picklist_valid_for", Type: "String", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymousWithOptions(`
Map<String,Object> unrelated=new Map<String,Object>{'validFor'=>'raw','__glade_picklist_valid_for'=>'keep'};
PicklistCarrierDTO dto=new PicklistCarrierDTO();
dto.validFor='raw';dto.__glade_picklist_valid_for='keep';
for(Boolean suppress:new List<Boolean>{false,true}) {
 Map<String,Object> m=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(unrelated,suppress));
 Map<String,Object> d=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(dto,suppress));
 System.assertEquals('raw',m.get('validFor'));System.assertEquals('keep',m.get('__glade_picklist_valid_for'));
 System.assertEquals('raw',d.get('validFor'));System.assertEquals('keep',d.get('__glade_picklist_valid_for'));
}
System.assertEquals('"validFor"',JSON.serialize('validFor'));
`, CompileOptions{APIVersion: "61.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// These are preservation controls for shapes without native answers.
func TestPicklistJSONPreservesUnobservedShapes(t *testing.T) {
	for _, name := range []string{"suppressed nulls", "checkbox", "inactive controller", "inactive dependent", "unresolved controller", "explicit empty mapping", "unknown mapping", "duplicate mapping", "duplicate controller", "duplicate mapping value", "wide controller"} {
		t.Run(name, func(t *testing.T) {
			org := picklistJSONTestOrg()
			fields := org.Objects["ChoiceGrid__c"].Definition.Fields
			c, d := fields["Controlling__c"], fields["Dependent__c"]
			switch name {
			case "checkbox":
				c.Type = storage.FieldBoolean
			case "inactive controller":
				c.PicklistValues = append([]storage.PicklistValue(nil), c.PicklistValues...)
				c.PicklistValues[0].Active = false
			case "inactive dependent":
				d.PicklistValues = append([]storage.PicklistValue(nil), d.PicklistValues...)
				d.PicklistValues[1].Active = false
			case "explicit empty mapping":
				d.PicklistValueSettings = []storage.PicklistSetting{{ValueName: "First"}}
			case "unknown mapping":
				d.PicklistValueSettings = []storage.PicklistSetting{{ValueName: "First", ControllingFieldValues: []string{"Missing"}}}
			case "duplicate mapping":
				d.PicklistValueSettings = append(d.PicklistValueSettings, d.PicklistValueSettings[0])
			case "duplicate controller":
				c.PicklistValues = append(c.PicklistValues, c.PicklistValues[0])
			case "duplicate mapping value":
				d.PicklistValueSettings = []storage.PicklistSetting{{ValueName: "First", ControllingFieldValues: []string{"North", "North"}}}
			case "wide controller":
				c.PicklistValues = make([]storage.PicklistValue, 102)
				for i := range c.PicklistValues {
					c.PicklistValues[i] = storage.PicklistValue{Value: fmt.Sprintf("Point%03d", i), Active: true}
				}
				d.PicklistValueSettings = []storage.PicklistSetting{{ValueName: "First", ControllingFieldValues: []string{"Point000"}}}
			}
			fields["Controlling__c"], fields["Dependent__c"] = c, d
			if name == "unresolved controller" {
				delete(fields, "Controlling__c")
			}
			call := "JSON.serialize(entries[1])"
			if name == "suppressed nulls" {
				call = "JSON.serialize(entries[1],true)"
			}
			program, err := CompileAnonymousWithOptions(`
List<Schema.PicklistEntry> entries=ChoiceGrid__c.Dependent__c.getDescribe().getPicklistValues();
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(`+call+`);
System.assertEquals(false,payload.containsKey('validFor'));
System.assertEquals(false,payload.containsKey('__glade_picklist_valid_for'));
`, CompileOptions{APIVersion: "61.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A JSON-only carrier must not appear in existing nested entry text.
func TestPicklistJSONKeepsEntryTextPrivate(t *testing.T) {
	org := picklistJSONTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymousWithOptions(`List<Schema.PicklistEntry> entries=ChoiceGrid__c.Dependent__c.getDescribe().getPicklistValues();`, CompileOptions{APIVersion: "61.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	entries := machine.Globals["entries"]
	if strings.Contains(entries.String(), "__glade") {
		t.Fatal("JSON-only carrier leaked into nested entry text")
	}
	holder := Object("EntryHolder")
	holder.Fields["entry"] = entries.List[1]
	if strings.Contains(holder.String(), "__glade") {
		t.Fatal("JSON-only carrier leaked through an enclosing object")
	}
	if got := entries.List[1].String(); got != "First" {
		t.Fatalf("entry text=%q", got)
	}
}
