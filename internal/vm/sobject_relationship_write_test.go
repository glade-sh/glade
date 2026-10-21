package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSObjectPutSObjectWrongTargetPreservesRelationship(t *testing.T) {
	machine := sobjectRelationshipWriteMachine(t)
	aliasSObjectPruneExecute(t, machine, `
Contact child = new Contact();
Account before = new Account(Name='before');
child.putSObject('Account', before);
for (Boolean token : new List<Boolean>{false, true}) {
    String exceptionType;
    String message;
    try {
        if (token) child.putSObject(Contact.AccountId, new User(LastName='after'));
        else child.putSObject('Account', new User(LastName='after'));
    } catch (SObjectException e) { exceptionType=e.getTypeName(); message=e.getMessage(); }
    System.assertEquals('System.SObjectException', exceptionType);
    System.assertEquals('Illegal assignment from User to Contact', message);
    System.assertEquals(before, child.getSObject('Account'));
}
Account after = new Account(Name='after');
System.assertEquals(before, child.putSObject(Contact.AccountId, after));
System.assertEquals(after, child.getSObject('Account'));
System.assertEquals(after, child.putSObject('Account', null));
System.assertEquals(null, child.getSObject('Account'));

Task activity = new Task();
Contact original = new Contact(LastName='before');
activity.putSObject('Who', original);
String polymorphicMessage;
try { activity.putSObject('Who', new Account(Name='wrong')); }
catch (SObjectException e) { polymorphicMessage=e.getMessage(); }
System.assertEquals('Illegal assignment from Account to Task', polymorphicMessage);
System.assertEquals(original, activity.getSObject('Who'));
Lead lead = new Lead(LastName='after');
System.assertEquals(original, activity.putSObject('Who', lead));
System.assertEquals(lead, activity.getSObject('Who'));
System.assertEquals(lead, activity.putSObject('Who', original));
`)
}

func TestSObjectPutSObjectMetadataRelationshipReadOnly(t *testing.T) {
	machine := sobjectRelationshipWriteMachine(t)
	aliasSObjectPruneExecute(t, machine, `
PutConfig__mdt config = new PutConfig__mdt(TextValue__c='guard');
for (Boolean token : new List<Boolean>{false, true}) {
    String message;
    try {
        if (token) config.putSObject(PutConfig__mdt.Entity__c, null);
        else config.putSObject('Entity__r', new User(LastName='after'));
    } catch (SObjectException e) { message=e.getMessage(); }
    System.assertEquals('Relationship Entity__r is not editable', message);
    System.assertEquals(null, config.getSObject('Entity__r'));
    System.assertEquals('guard', config.TextValue__c);
}
String parentMessage;
try { config.putSObject('ParentConfig__r', new PutParentConfig__mdt(TextValue__c='after')); }
catch (SObjectException e) { parentMessage=e.getMessage(); }
System.assertEquals('Relationship ParentConfig__r is not editable', parentMessage);
System.assertEquals(null, config.getSObject('ParentConfig__r'));
config.put('Entity__c', 'Account');
System.assertEquals('Account', config.get('Entity__c'));
System.assertEquals('guard', config.put('TextValue__c', 'after'));
System.assertEquals('after', config.get('TextValue__c'));
`)
}

func sobjectRelationshipWriteMachine(t *testing.T) *VM {
	t.Helper()
	org := testDataOrg()
	for _, name := range []string{"Account", "Contact", "User", "Task", "Lead", "EntityDefinition"} {
		storage.EnsureStandardObject(&org, name)
	}
	org.Objects["PutParentConfig__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "PutParentConfig__mdt", Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{"TextValue__c": {APIName: "TextValue__c", Type: storage.FieldString}},
	}}
	org.Objects["PutConfig__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "PutConfig__mdt", Metadata: map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{
			"TextValue__c":    {APIName: "TextValue__c", Type: storage.FieldString},
			"Entity__c":       {APIName: "Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"EntityDefinition"}, RelationshipName: "Entity__r"},
			"ParentConfig__c": {APIName: "ParentConfig__c", Type: storage.FieldReference, ReferenceTo: []string{"PutParentConfig__mdt"}, RelationshipName: "ParentConfig__r"},
		},
		Relations: []storage.Relationship{
			{Field: "Entity__c", ParentObjects: []string{"EntityDefinition"}, ParentRelationship: "Entity__r"},
			{Field: "ParentConfig__c", ParentObjects: []string{"PutParentConfig__mdt"}, ParentRelationship: "ParentConfig__r"},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	return machine
}
