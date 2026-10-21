package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Observed at source API65, anonymous and @IsTest, in the owned expanded-owner packet.
// Only the captured expanded envelopes are asserted; strict mode and other relationships stay unchanged.
func TestJSONExpandedOwnerNativeShapes(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		ownerType       string
		hasID           bool
		explicitType    bool
		ownerNull       bool
		serializedOwner string
	}{
		{`USER_ID_URL_NULL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":null}}}`, `User`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`USER_ID_URL_STRING`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v65.0/sobjects/User/005000000000017"}}}`, `User`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_ID_URL_NULL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","attributes":{"type":"Name","url":null}}}`, `Name`, true, false, false, `{"Id":"005000000000017","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_TYPE_ID_URL_NULL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":null}}}`, `Name`, true, true, false, `{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_TYPE_ID_URL_STRING`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v65.0/sobjects/User/005000000000017"}}}`, `Name`, true, true, false, `{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`USER_ID_URL_REST68`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017"}}}`, `User`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_ID_URL_REST68`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}}`, `Name`, true, false, false, `{"Id":"005000000000017","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_TYPE_ID_URL_REST68`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}}`, `Name`, true, true, false, `{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`USER_ID15_URL18`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}}`, `User`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`USER_ID18_URL18`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","Id":"005000000000017AAA","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}}`, `User`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017AAA","attributes":{"type":"User","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}`},
		{`NAME_ID15_URL18`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}}`, `Name`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_ID18_URL18`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017AAA","FirstName":"Owned","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}}`, `Name`, true, false, false, `{"FirstName":"Owned","Id":"005000000000017AAA","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}`},
		{`NAME_TYPE_ID15_URL18`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}}`, `Name`, true, true, false, `{"FirstName":"Owned","Id":"005000000000017","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017"}}`},
		{`NAME_TYPE_ID18_URL18`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"Id":"005000000000017AAA","FirstName":"Owned","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}}`, `Name`, true, true, false, `{"FirstName":"Owned","Id":"005000000000017AAA","Type":"User","attributes":{"type":"Name","url":"/services/data/v68.0/sobjects/User/005000000000017AAA"}}`},
		{`USER_BARE_CONTROL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":{"FirstName":"Owned","attributes":{"type":"User"}}}`, `User`, false, false, false, `{"FirstName":"Owned","attributes":{"type":"User"}}`},
		{`OWNER_NULL_CONTROL`, `{"attributes":{"type":"Case"},"Id":"500000000000017","OwnerId":"005000000000017","Subject":"Owned envelope","Owner":null}`, ``, false, false, true, `null`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertions := `System.assertEquals(null, owner);`
			if !tc.ownerNull {
				assertions = fmt.Sprintf(`
System.assertEquals('%s', String.valueOf(owner.getSObjectType()));
System.assertEquals('Owned', owner.get('FirstName'));
System.assertEquals(%t, owner.get('Id') == Id.valueOf('005000000000017'));
System.assertEquals(%t, owner.get('Id') == null);
`, tc.ownerType, tc.hasID, !tc.hasID)
				if tc.ownerType == "User" {
					assertions += `
Boolean caughtType=false;
try { owner.get('Type'); } catch(SObjectException e) {
 caughtType=true;
 System.assertEquals('System.SObjectException', e.getTypeName());
 System.assertEquals('Invalid field Type for User', e.getMessage());
}
System.assert(caughtType);
`
				} else if tc.explicitType {
					assertions += `System.assertEquals('User', owner.get('Type'));`
				} else {
					assertions += `System.assertEquals(null, owner.get('Type'));`
				}
			}
			source := fmt.Sprintf(`
Case record=(Case)JSON.deserialize('%s',Case.class);
System.assertEquals(Id.valueOf('500000000000017'),record.Id);
System.assertEquals(Id.valueOf('005000000000017'),record.OwnerId);
SObject owner=record.getSObject('Owner');
%s
Map<String,Object> output=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record));
System.assertEquals(JSON.deserializeUntyped('%s'),output.get('Owner'));
`, tc.input, assertions, tc.serializedOwner)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "65.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = "68.0"
			for _, name := range []string{"Case", "User", "Group", "Name"} {
				storage.EnsureStandardObject(&org, name)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Compose the independently observed User-ID generator and the six admitted
// owner envelopes. The generator keeps its 18-character ID/URL control while
// ordinary JSON.deserialize preserves the owner payload's lexical ID length.
func TestJSONGeneratedOwnerEnvelopeComposition(t *testing.T) {
	for _, ownerID := range []string{"005000000000017", "005000000000017AAA"} {
		for _, target := range []struct {
			name       string
			objectType string
			typeField  bool
		}{
			{"User", "User", false}, {"Name", "Name", false}, {"NameType", "Name", true},
		} {
			t.Run(target.name+"/"+ownerID, func(t *testing.T) {
				extra, extraJSON := "", ""
				if target.typeField {
					extra = "ownerData.put('Type','User');"
					extraJSON = `,"Type":"User"`
				}
				expected := fmt.Sprintf(`{"Id":"%s","FirstName":"Owned"%s,"attributes":{"type":"%s","url":"/services/data/v68.0/sobjects/User/%s"}}`, ownerID, extraJSON, target.objectType, ownerID)
				source := fmt.Sprintf(`
Id key=Id.valueOf('005000000000017');
JSONGenerator generator=JSON.createGenerator(false);
generator.writeObject(key.getSObjectType().newSObject(key));
Map<String,Object> generated=(Map<String,Object>)JSON.deserializeUntyped(generator.getAsString());
Map<String,Object> attributes=(Map<String,Object>)generated.get('attributes');
System.assertEquals('005000000000017AAA',generated.get('Id'));
System.assertEquals('/services/data/v68.0/sobjects/User/005000000000017AAA',attributes.get('url'));
attributes.put('type','%s');
Map<String,Object> ownerData=new Map<String,Object>{'Id'=>'%s','FirstName'=>'Owned','attributes'=>attributes};
%s
Map<String,Object> payload=new Map<String,Object>{'attributes'=>new Map<String,Object>{'type'=>'Case'},'Id'=>'500000000000017','OwnerId'=>'005000000000017','Subject'=>'Owned envelope','Owner'=>ownerData};
Case record=(Case)JSON.deserialize(JSON.serialize(payload),Case.class);
Map<String,Object> output=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record));
System.assertEquals(JSON.deserializeUntyped('%s'),output.get('Owner'));
`, target.objectType, ownerID, extra, expected)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "65.0"})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = "68.0"
				for _, name := range []string{"Case", "User", "Group", "Name"} {
					storage.EnsureStandardObject(&org, name)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
