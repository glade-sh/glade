package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native linked-constructor and linked-put observations, API 62 and 67:
// testdata/linked_checkbox_default_true/org.tsv.
func TestLinkedCustomCheckboxDefaultTrueNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, fieldName := range []string{"Flag__c", "Other__c", "Third__c"} {
			for _, linked := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/linked=%t", api, fieldName, linked), func(t *testing.T) {
					org := storage.NewOrgState()
					org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{
						fieldName: {APIName: fieldName, Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)},
					}}}
					machine := New(nil)
					machine.SetOrg(&org)
					machine.currentMethod.APIVersion = api
					if linked {
						if err := machine.RegisterClass(Class{Name: "Parent__c", PlatformType: true, SuperClass: "SObject", Fields: map[string]Field{
							fieldName: {Name: fieldName, Type: "Boolean", Access: "public"},
						}}); err != nil {
							t.Fatal(err)
						}
					}
					row := Object("Parent__c")
					setExplicitSObjectField(&row, fieldName, Null)
					got, err := machine.lookupPath(row, []string{fieldName})
					if err != nil || got.Kind != ValueBool || got.Bool {
						t.Fatalf("dotted explicit null = %#v, err=%v; want false", got, err)
					}
					got, handled, err := machine.callSObjectMember(row, "get", []Value{String(fieldName)})
					if err != nil || !handled || got.Kind != ValueNull {
						t.Fatalf("dynamic explicit null = %#v, handled=%t, err=%v", got, handled, err)
					}
				})
			}
		}
	}
}

// The native rows distinguish the in-memory dotted and dynamic views for
// omitted, null, false, and true, before and after insert and through links.
// testdata/linked_checkbox_default_true/org.tsv:2-5,10-13,17-23,25-28,33-36,43-49.
func TestCustomCheckboxMemoryValuesNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name     string
			value    Value
			explicit bool
			dotted   bool
			dynamic  Value
		}{
			{"omitted", Null, false, false, Null}, {"null", Null, true, false, Null},
			{"false", Bool(false), true, false, Bool(false)}, {"true", Bool(true), true, true, Bool(true)},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := storage.NewOrgState()
				org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)}}}}
				machine := New(nil)
				machine.SetOrg(&org)
				machine.currentMethod.APIVersion = api
				row := Object("Parent__c")
				if tc.explicit {
					setExplicitSObjectField(&row, "Flag__c", tc.value)
				}
				got, err := machine.lookupPath(row, []string{"Flag__c"})
				if err != nil || got.Kind != ValueBool || got.Bool != tc.dotted {
					t.Fatalf("dotted=%#v err=%v", got, err)
				}
				got, handled, err := machine.callSObjectMember(row, "get", []Value{String("Flag__c")})
				if err != nil || !handled || got.Kind != tc.dynamic.Kind || (got.Kind == ValueBool && got.Bool != tc.dynamic.Bool) {
					t.Fatalf("dynamic=%#v handled=%t err=%v", got, handled, err)
				}
			})
		}
	}
}

// Anonymous relationship and put routes mirror the captured linked rows at
// testdata/linked_checkbox_default_true/org.tsv:17-23,43-49.
func TestCustomCheckboxLinkedAnonymousNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{
				"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)},
			}}, Records: map[storage.ID]storage.Record{}}
			org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Child__c", Fields: map[string]storage.Field{
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			}}, Records: map[storage.ID]storage.Record{}}
			m := New(nil)
			m.SetOrg(&org)
			source := `Parent__c p = new Parent__c(Flag__c=null);
Child__c c = new Child__c(Parent__r=p);
System.assertEquals(false, c.Parent__r.Flag__c);
System.assertEquals(null, c.Parent__r.get('Flag__c'));
p.put('Flag__c', false);
System.assertEquals(false, c.Parent__r.Flag__c);
System.assertEquals(false, c.Parent__r.get('Flag__c'));
p.put('Flag__c', true);
System.assertEquals(true, c.Parent__r.Flag__c);
System.assertEquals(true, c.Parent__r.get('Flag__c'));
p.put('Flag__c', null);
System.assertEquals(false, c.Parent__r.Flag__c);
System.assertEquals(null, c.Parent__r.get('Flag__c'));`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// API 62/67 native insert and SOQL rows: testdata/linked_checkbox_default_true/org.tsv:6-16,29-42.
func TestCustomCheckboxInsertQueryAnonymousNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString},
				"Flag__c": {APIName: "Flag__c", Label: "Flag", Type: storage.FieldBoolean, DefaultValue: "true", Createable: storage.BoolFlag(true), Nillable: storage.BoolFlag(false)},
			}}, Records: map[storage.ID]storage.Record{}}
			org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Child__c", KeyPrefix: "a02", Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			}}, Records: map[storage.ID]storage.Record{}}
			source := `Parent__c omitted = new Parent__c(Name='omitted');
Parent__c nulled = new Parent__c(Name='null', Flag__c=null);
Parent__c no = new Parent__c(Name='false', Flag__c=false);
Parent__c yes = new Parent__c(Name='true', Flag__c=true);
insert omitted; insert no; insert yes;
System.assertEquals(false, omitted.Flag__c); System.assertEquals(null, omitted.get('Flag__c'));
Boolean rejected=false; try { insert nulled; } catch (DmlException e) { rejected=true; System.assertEquals('INVALID_TYPE_ON_FIELD_IN_RECORD', String.valueOf(e.getDmlStatusCode(0)));  } System.assert(rejected);
Child__c a=new Child__c(Name='omitted',Parent__c=omitted.Id); Child__c b=new Child__c(Name='false',Parent__c=no.Id); Child__c c=new Child__c(Name='true',Parent__c=yes.Id); insert new List<Child__c>{a,b,c};
List<Child__c> rows=[SELECT Name, Parent__r.Flag__c FROM Child__c]; System.assertEquals(3, rows.size());
for (Child__c row : rows) { Boolean want=row.Name!='false'; System.assertEquals(want,row.Parent__r.Flag__c); System.assertEquals(want,row.getSObject('Parent__r').get('Flag__c')); }
Boolean wrong=false; try { omitted.put('Flag__c','bad'); } catch (SObjectException e) {wrong=true;} System.assert(wrong);`
			m := New(nil)
			m.SetOrg(&org)
			p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Execute(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}
