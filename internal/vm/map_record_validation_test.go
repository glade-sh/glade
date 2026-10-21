package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestMapRecordListStageValidation(t *testing.T) {
	// Native named API 62 with helper sources 46 and 62: construction accepts
	// the declared list, but get and putAll validate the actual record identity.
	for _, tc := range []struct{ name, source string }{
		{"get", `
Map<Id,GlyphCard__c> keyed=new Map<Id,GlyphCard__c>(declared);
System.assertEquals(1,keyed.size());
System.assert(keyed.containsKey(seed.Id));
Boolean rejected=false;
try { Object boxed=keyed.get(seed.Id); }
catch(TypeException ex) {
 rejected=true;
 System.assertEquals('Invalid conversion from runtime type AtlasCard__c to GlyphCard__c',ex.getMessage());
}
System.assert(rejected);
System.assertEquals(null,keyed.get('a01000000000009'));
`},
		{"putAll", `
Map<Id,GlyphCard__c> appended=new Map<Id,GlyphCard__c>();
Boolean rejected=false;
try { appended.putAll(declared); }
catch(TypeException ex) {
 rejected=true;
 System.assertEquals('Collection store exception putting all List<SObject> into Map<Id,GlyphCard__c>',ex.getMessage());
}
System.assert(rejected);
System.assertEquals(0,appended.size());
`},
		{"matching control", `
GlyphCard__c matching=new GlyphCard__c(Id='a02000000000001');
List<SObject> matchingRaw=new List<SObject>{matching};
List<GlyphCard__c> matchingDeclared=(List<GlyphCard__c>)matchingRaw;
Map<Id,GlyphCard__c> keyed=new Map<Id,GlyphCard__c>(matchingDeclared);
System.assertEquals(matching.Id,keyed.get(matching.Id).Id);
Map<Id,GlyphCard__c> appended=new Map<Id,GlyphCard__c>();
appended.putAll(matchingDeclared);
System.assertEquals(matching.Id,appended.get(matching.Id).Id);
`},
		{"generic control", `
Map<Id,SObject> keyed=new Map<Id,SObject>(raw);
System.assertEquals(seed.Id,keyed.get(seed.Id).Id);
Map<Id,SObject> appended=new Map<Id,SObject>();
appended.putAll(raw);
System.assertEquals(seed.Id,appended.get(seed.Id).Id);
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			for _, name := range []string{"AtlasCard__c", "GlyphCard__c"} {
				org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}}}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymousWithOptions(`
AtlasCard__c seed=new AtlasCard__c(Id='a01000000000001');
List<SObject> raw=new List<SObject>{seed};
List<GlyphCard__c> declared=(List<GlyphCard__c>)raw;
`+tc.source, CompileOptions{APIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// This local identity control bounds the Map validator. It is not a native
// observation of a provider-local constructor or a cross-package list cast.
func TestMapRecordValidationKeepsInstalledIdentitiesDistinct(t *testing.T) {
	org := testDataOrg()
	for _, name := range []string{"oak__GlyphCard__c", "fir__GlyphCard__c"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		name, actual                string
		classInstance, wantMismatch bool
	}{
		{name: "same installed record", actual: "fir__GlyphCard__c"},
		{name: "different installed record", actual: "oak__GlyphCard__c", wantMismatch: true},
		{name: "allocated class", actual: "fir__GlyphCard__c", classInstance: true, wantMismatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := Object(tc.actual)
			value.classInstance = tc.classInstance
			target, mismatch := machine.mapRecordValueMismatch("Map<Id,fir__GlyphCard__c>", value)
			if target != "fir__GlyphCard__c" || mismatch != tc.wantMismatch {
				t.Fatalf("target=%q mismatch=%t", target, mismatch)
			}
		})
	}
}
