package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestJSONRecordDMLProjection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, source string }{
			{"implicitOwnerAfterInsert", `
Contact record=new Contact(FirstName='Harbor',LastName='Projection');
insert record;
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record,false));
System.assert(!payload.containsKey('OwnerId'),'implicit insert owner is not serialized');
System.assertEquals(record.Id,payload.get('Id'));
System.assertEquals('Harbor',payload.get('FirstName'));
System.assertEquals('Projection',payload.get('LastName'));
`},
			{"insertedRoundTrip", `
Contact record=new Contact(FirstName='Harbor',LastName='Roundtrip');
insert record;
Contact hydrated=(Contact)JSON.deserialize(JSON.serialize(record,false),Contact.class);
System.assert(record==hydrated,'inserted record and its JSON roundtrip have equal values');
System.assertEquals(record,hydrated);
`},
			{"selectedCreatedDate", `
Contact record=new Contact(LastName='Selected timestamp');
insert record;
Id recordId=record.Id;
Contact selected=[SELECT Id,CreatedDate FROM Contact WHERE Id=:recordId];
String wire=JSON.serialize(selected,false);
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(wire);
System.assert(payload.containsKey('CreatedDate'),'selected timestamp remains serialized');
Contact hydrated=(Contact)JSON.deserialize(wire,Contact.class);
System.assert(selected.CreatedDate==hydrated.CreatedDate,'selected timestamp retains its exact value');
System.assertEquals(selected.Id,hydrated.Id);
`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				runJSONRecordProjection(t, api, tc.source)
			})
		}
	}
}

func TestJSONRecordDMLProjectionControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, source string }{
			{"freshAssignedFields", `
Contact record=new Contact(FirstName='Harbor',LastName='Fresh');
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record,false));
System.assertEquals(new Set<String>{'attributes','FirstName','LastName'},payload.keySet());
System.assertEquals('Harbor',payload.get('FirstName'));
System.assertEquals('Fresh',payload.get('LastName'));
`},
			{"explicitOwnerBeforeAndAfterInsert", `
Id ownerId=UserInfo.getUserId();
Contact record=new Contact(LastName='Explicit owner',OwnerId=ownerId);
Map<String,Object> before=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record,false));
System.assertEquals(ownerId,before.get('OwnerId'));
insert record;
Map<String,Object> after=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record,false));
System.assertEquals(ownerId,after.get('OwnerId'));
`},
			{"ownerAssignedAfterInsert", `
Contact record=new Contact(LastName='Later owner assignment');
insert record;
record.OwnerId=UserInfo.getUserId();
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(record,false));
System.assertEquals(UserInfo.getUserId(),payload.get('OwnerId'));
`},
			{"selectedOwner", `
Contact record=new Contact(LastName='Selected owner');
insert record;
Id recordId=record.Id;
Contact selected=[SELECT Id,OwnerId FROM Contact WHERE Id=:recordId];
String wire=JSON.serialize(selected,false);
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(wire);
System.assert(payload.containsKey('OwnerId'),'selected owner remains serialized');
System.assertEquals(UserInfo.getUserId(),payload.get('OwnerId'));
Contact hydrated=(Contact)JSON.deserialize(wire,Contact.class);
System.assertEquals(selected.OwnerId,hydrated.OwnerId);
`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				runJSONRecordProjection(t, api, tc.source)
			})
		}
	}
}

func runJSONRecordProjection(t *testing.T, api, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
