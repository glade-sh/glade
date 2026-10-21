package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The API65 native packet admits these exact incoming URL65 envelopes. Native
// serialization ran at REST68; this regression retains the existing configurable
// REST URL projection by running Glade at REST65. It is not a native REST65 run.
func TestJSONOwnerCanonicalURL65NativeShapes(t *testing.T) {
	tests := []struct {
		name, input, ownerType, ownerID, expectedOwner string
		explicitType                                   bool
	}{
		{`USER_ID15_URL65_CANONICAL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}}`, `User`, `005000000000017`, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v65.0/sobjects/User/005000000000017"}}`, false},
		{`USER_ID18_URL65_CANONICAL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017AAA","attributes":{"type":"User","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}}`, `User`, `005000000000017AAA`, `{"FirstName":"Owned","Id":"005000000000017AAA","attributes":{"type":"User","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}`, false},
		{`NAME_ID15_URL65_CANONICAL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}}`, `Name`, `005000000000017`, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017"}}`, false},
		{`NAME_ID18_URL65_CANONICAL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017AAA","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}}`, `Name`, `005000000000017AAA`, `{"FirstName":"Owned","Id":"005000000000017AAA","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}`, false},
		{`NAME_TYPE_ID15_URL65_CANONICAL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}}`, `Name`, `005000000000017`, `{"FirstName":"Owned","Id":"005000000000017","Type":"User","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017"}}`, true},
		{`NAME_TYPE_ID18_URL65_CANONICAL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017AAA","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}}`, `Name`, `005000000000017AAA`, `{"FirstName":"Owned","Id":"005000000000017AAA","Type":"User","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017AAA"}}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			extra := ""
			typeRead := `System.assertEquals(null,owner.get('Type'));`
			if tc.explicitType {
				extra = `ownerData.put('Type','User');`
				typeRead = `System.assertEquals('User',owner.get('Type'));`
			}
			if tc.ownerType == "User" {
				typeRead = `
Boolean caught=false;
try {owner.get('Type');} catch(SObjectException e) {
 caught=true;
 System.assertEquals('System.SObjectException',e.getTypeName());
 System.assertEquals('Invalid field Type for User',e.getMessage());
}
System.assert(caught);
`
			}
			source := fmt.Sprintf(`
Id key=Id.valueOf('005000000000017');
JSONGenerator generator=JSON.createGenerator(false);
generator.writeObject(key.getSObjectType().newSObject(key));
Map<String,Object> generated=(Map<String,Object>)JSON.deserializeUntyped(generator.getAsString());
Map<String,Object> attributes=(Map<String,Object>)generated.get('attributes');
System.assertEquals('005000000000017AAA',generated.get('Id'));
System.assertEquals('/services/data/v65.0/sobjects/User/005000000000017AAA',attributes.get('url'));
attributes.put('type','%s');
Map<String,Object> ownerData=new Map<String,Object>{'Id'=>'%s','FirstName'=>'Owned','attributes'=>attributes};
%s
Map<String,Object> composed=new Map<String,Object>{'attributes'=>new Map<String,Object>{'type'=>'Case'},'Id'=>'500000000000017','OwnerId'=>'005000000000017','Subject'=>'Owned envelope','Owner'=>ownerData};
Case direct=(Case)JSON.deserialize('%s',Case.class);
Case assembled=(Case)JSON.deserialize(JSON.serialize(composed),Case.class);
for(Case record:new List<Case>{direct,assembled}) {
 System.assertEquals(Id.valueOf('500000000000017'),record.Id);
 System.assertEquals(Id.valueOf('005000000000017'),record.OwnerId);
 SObject owner=record.getSObject('Owner');
 System.assertEquals('%s',String.valueOf(owner.getSObjectType()));
 System.assertEquals('Owned',owner.get('FirstName'));
 System.assertEquals(Id.valueOf('005000000000017'),owner.get('Id'));
 %s
 Map<String,Object> output=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record));
 System.assertEquals(JSON.deserializeUntyped('%s'),output.get('Owner'));
}
`, tc.ownerType, tc.ownerID, extra, tc.input, tc.ownerType, typeRead, tc.expectedOwner)
			runOwnerURL65Program(t, "65.0", source)
		})
	}
}

// These independent bare/null owner controls were already observed; no strict
// or legacy URL shape is inferred from the expanded ordinary-deserialize rows.
func TestJSONOwnerCanonicalURL65ObservedControls(t *testing.T) {
	for _, source := range []string{`
Case record=(Case)JSON.deserialize('{"Owner":{"FirstName":"Owned","attributes":{"type":"User"}}}',Case.class);
SObject owner=record.getSObject('Owner');
System.assertEquals('User',String.valueOf(owner.getSObjectType()));
System.assertEquals('Owned',owner.get('FirstName'));
System.assertEquals(null,owner.get('Id'));
`, `Case record=(Case)JSON.deserialize('{"Owner":null}',Case.class);System.assertEquals(null,record.getSObject('Owner'));`} {
		runOwnerURL65Program(t, "65.0", source)
	}
}

func runOwnerURL65Program(t *testing.T, api, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.APIVersion = "65.0"
	for _, name := range []string{"Case", "User", "Group", "Name"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
