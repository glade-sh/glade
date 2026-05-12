package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestExecSObjectDMLAndSOQL(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
System.assertEquals('Acme', a.Name);
a.put('Name', 'Changed');
System.assertEquals('Changed', a.get('Name'));
insert a;
String wanted = 'Changed';
List<Account> rows = [SELECT Id, Name, MasterRecordId FROM Account WHERE Name = :wanted];
System.assertEquals(1, rows.size());
Account row = rows.get(0);
System.assertEquals('Changed', row.Name);
System.assertEquals(null, row.MasterRecordId);
row.Name = 'Updated';
update row;
List<Account> updated = Database.query('SELECT Id, Name FROM Account WHERE Name = ''Updated''');
System.assertEquals(1, updated.size());
Account updatedRow = updated.get(0);
Id updatedId = updatedRow.Id;
Database.delete(new List<Id>{updatedId});
List<Account> empty = [SELECT Id FROM Account];
System.assertEquals(0, empty.size());
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

func TestExecSObjectMapKeySetKeepsSObjectKeys(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Map<Account, String> byAccount = new Map<Account, String>();
byAccount.put(a, 'seen');
for (Account key : byAccount.keySet()) {
    System.assertEquals(a.Id, key.Id);
}
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

func TestExecDatabaseQueryCanAssignSingleRowToSObject(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Single');
insert a;
Id accountId = a.Id;
SObject row = Database.query('SELECT Id, Name FROM Account WHERE Id = :accountId LIMIT 1');
System.assertEquals(a.Id, row.Id);
System.assertEquals('Single', row.get('Name'));
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

func TestExecSchemaSObjectTypeFieldMapPath(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
System.assert(fields.containsKey('Name'));
System.assert(fields.containsKey('MasterRecordId'));
System.assertNotEquals(null, Account.SObjectType.fields.Id);
System.assertNotEquals(null, Account.SObjectType.fields.id);
System.assertEquals('Name', Account.SObjectType.fields.Name.Name);
System.assertNotEquals(null, OpportunityLineItem.SObjectType);
Schema.SObjectField ownerField = fields.get('OwnerId');
System.assertEquals(Schema.SOAPType.ID, ownerField.getDescribe().getSOAPType());
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

func TestQueriedSObjectFieldsMarksLookupForRelationshipProjection(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Parent__r",
				}},
			},
		},
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName: "Account",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
	}}
	machine.SetOrg(&org)

	fields := machine.queriedSObjectFields("SELECT Id, Parent__r.Id FROM Child__c")
	if !fields["parent__c"] {
		t.Fatalf("relationship projection did not mark lookup field: %#v", fields)
	}
}

func TestExecRelationshipProjectionHydratesLookupField(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> rows = Database.query('SELECT Parent__r.Id FROM Child__c');
System.assertEquals(1, rows.size());
System.assertEquals('001000000000001AAA', rows[0].get('Parent__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName:   "Account",
				KeyPrefix: "001",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
			Records: map[storage.ID]storage.Record{
				"001000000000001AAA": {Object: "Account", ID: "001000000000001AAA", Fields: map[string]storage.Value{}},
			},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Parent__r",
				}},
			},
			Records: map[storage.ID]storage.Record{
				"a00000000000001AAA": {
					Object: "Child__c",
					ID:     "a00000000000001AAA",
					Fields: map[string]storage.Value{"Parent__c": storage.IDValue("001000000000001AAA")},
				},
			},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecParentRelationshipCanLoadFromLookupID(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = '001000000000001AAA');
System.assertEquals('Acme', child.Parent__r.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName:   "Account",
				KeyPrefix: "001",
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{
				"001000000000001AAA": {Object: "Account", ID: "001000000000001AAA", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Parent__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSObjectParentRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = (Child__c)JSON.deserialize('{"Id":"a00000000000001AAA","Parent__r":{"Id":"001000000000001AAA","Name":"Acme"}}', Child__c.class);
System.assertEquals('Parent__r', Child__c.Parent__c.getDescribe().getRelationshipName());
System.assertEquals('Acme', child.Parent__r.Name);
System.assertEquals('001000000000001AAA', child.Parent__r.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName:   "Account",
				KeyPrefix: "001",
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "ChildRecords"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Parent__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldTypeCanCompareUnqualifiedDisplayType(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult describe = Account.Name.getDescribe();
System.assertEquals(Schema.DisplayType.STRING, describe.getType());
System.assertEquals(DisplayType.STRING, describe.getType());
System.assertEquals('STRING', describe.getType());
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

func TestExecSObjectTypeNewSObject(t *testing.T) {
	program, err := CompileAnonymous(`
Account emptyAccount = (Account)Account.SObjectType.newSObject();
System.assertEquals('Account', emptyAccount.getSObjectType().getDescribe().getName());
Account accountWithRecordTypeId = (Account)Account.SObjectType.newSObject('012000000000001AAA');
System.assertEquals('012000000000001AAA', accountWithRecordTypeId.RecordTypeId);
Account accountWithId = (Account)Account.SObjectType.newSObject('001000000000001AAA');
System.assertEquals('001000000000001AAA', accountWithId.Id);
Account accountWithDefaults = (Account)Account.SObjectType.newSObject(null, true);
System.assertEquals('Account', accountWithDefaults.getSObjectType().getDescribe().getName());
TemplateSettings__c settings = (TemplateSettings__c)TemplateSettings__c.SObjectType.newSObject(null, true);
System.assertEquals('resetcss', settings.DefaultCSS__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["TemplateSettings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "TemplateSettings__c",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"DefaultCSS__c": {APIName: "DefaultCSS__c", Type: storage.FieldString, DefaultValue: "'resetcss'"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHierarchyCustomSettingInsertGetsOrgDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
insert new TemplateSettings__c();
TemplateSettings__c settings = TemplateSettings__c.getInstance();
System.assertEquals('00D000000000001', settings.Name);
System.assertEquals('resetcss', settings.DefaultCSS__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["TemplateSettings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "TemplateSettings__c",
			KeyPrefix: "a00",
			Metadata: map[string]string{
				"kind":               "customSetting",
				"customSettingsType": "Hierarchy",
			},
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DefaultCSS__c": {APIName: "DefaultCSS__c", Type: storage.FieldString, DefaultValue: "'resetcss'"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapLiteralCoercesStringBackedEnumKey(t *testing.T) {
	program, err := CompileAnonymous(`
Map<OperationType,String> byOperation = new Map<OperationType,String> {
	OperationType.ON_INSERT => 'insert',
	OperationType.ON_UPDATE => 'update'
};
System.assertEquals('insert', byOperation.get(OperationType.ON_INSERT));
System.assertEquals('update', byOperation.get(OperationType.ON_UPDATE));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "OperationType", EnumValues: []string{"ON_INSERT", "ON_UPDATE"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapLiteralCoercesNestedEnumKeyByLocalName(t *testing.T) {
	program, err := CompileAnonymous(`
Map<OperationType,String> byOperation = new Map<OperationType,String> {
	SyntheticContainer.OperationType.ON_INSERT => 'insert'
};
System.assertEquals('insert', byOperation.get(SyntheticContainer.OperationType.ON_INSERT));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "SyntheticContainer.OperationType", EnumValues: []string{"ON_INSERT", "ON_UPDATE"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertAppliesCheckboxDefaultsBeforeTriggers(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.CopyFromPrimaryAffiliationBilling__c && a.Name != null) {
		a.Name = 'copied';
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Account row = [SELECT CopyFromPrimaryAffiliationBilling__c, Name FROM Account WHERE Id = :a.Id][0];
System.assertEquals(false, row.CopyFromPrimaryAffiliationBilling__c);
System.assertEquals('Acme', row.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertDefaults",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingSObjectCheckboxFieldsReadAsFalse(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
System.assertEquals(false, a.UpdatePrimaryLocation__c);
System.assertEquals(false, a.get('UpdatePrimaryLocation__c'));
System.assertEquals(false, a.UpdatePrimaryLocation__c && true);
System.assertEquals(true, true || a.UpdatePrimaryLocation__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["UpdatePrimaryLocation__c"] = storage.Field{APIName: "UpdatePrimaryLocation__c", Type: storage.FieldBoolean}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLBindPlatformId(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Id accountId = a.Id;
List<Account> rows = [SELECT Id, Name FROM Account WHERE Id = :accountId];
System.assertEquals(1, rows.size());
System.assertEquals('Acme', rows[0].Name);
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

func TestExecSingleAmpersandAndPipeBooleanOperators(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean both = true & true;
Boolean either = false | true;
System.assert(both);
System.assert(either);
System.assertEquals(false, false && null);
System.assertEquals(true, true || null);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformCasingAliases(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, System.Test.isRunningTest());
System.assertEquals(Date.today(), Date.Today());
System.assertEquals(false, Test.Database.hasRecords());
System.assert(Date.newInstance(2026, 1, 1) <= Date.newInstance(2026, 1, 2));
System.assert(Date.newInstance(2026, 1, 2) >= Date.newInstance(2026, 1, 1));
Date missingDate = null;
System.assertEquals(false, missingDate < Date.newInstance(2026, 1, 1));
System.assertEquals('Local_Message', Label.Local_Message);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectListGetSObjectTypeAndMapValuesProperty(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = (List<SObject>)JSON.deserialize('[{"attributes":{"type":"Account"},"Name":"Acme"}]', List<SObject>.class);
System.assertEquals('Account', records.getSObjectType().getDescribe().getName());
List<SObject> emptyRecords = new List<SObject>();
System.assertEquals(null, emptyRecords.getSObjectType());
Map<Id, Account> accounts = new Map<Id, Account>();
Account a = new Account(Name = 'Spruce');
insert a;
accounts.put(a.Id, a);
System.assertEquals(1, accounts.values.size());
System.assertEquals('Spruce', accounts.values[0].Name);
System.assertEquals('Spruce', a.get(Account.Name));
a.put(Account.Name, 'Birch');
System.assertEquals('Birch', a.get('Name'));
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

func TestExecSObjectFieldShape(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account();
System.assert(!a.isSet('Name'), 'new sobject should not have Name set');
System.assertEquals(null, a.get('Name'));
Object previous = a.put('Name', 'Acme');
System.assertEquals(null, previous);
System.assert(a.isSet('Name'), 'put should set Name');
previous = a.put('Name', 'Changed');
System.assertEquals('Acme', previous);
a.put('Rating', null);
System.assert(a.isSet('Rating'), 'explicit null should count as set');
Map<String,Object> populated = a.getPopulatedFieldsAsMap();
System.assertEquals(2, populated.size());
System.assert(populated.containsKey('Name'), 'populated fields should include Name');
System.assert(populated.containsKey('Rating'), 'populated fields should include explicit null Rating');
insert a;
System.assert(a.isSet('Id'), 'insert should set Id');
a.clear();
System.assert(!a.isSet('Name'), 'clear should unset Name');
System.assert(!a.isSet('Id'), 'clear should unset Id');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectCloneAndRelationshipAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Parent');
Account row = new Account(Name = 'Child');
row.put('Id', '001000000000001');
row.put('Parent', parent);
row.put('Contacts', new List<Contact>{new Contact(LastName = 'Smith')});
Account parentRow = row.getSObject('Parent');
System.assertEquals('Parent', parentRow.Name);
List<Contact> contacts = row.getSObjects('Contacts');
System.assertEquals(1, contacts.size());
Account cloneNoId = row.clone();
System.assertEquals(null, cloneNoId.get('Id'));
System.assertEquals('Child', cloneNoId.Name);
Account cloneWithId = row.clone(true, true, false, false);
System.assertEquals(row.get('Id'), cloneWithId.get('Id'));
cloneWithId.Name = 'Clone';
System.assertEquals('Child', row.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID}
	org.Objects["Account"] = account
	contact := storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Contact"] = contact
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAttachmentBlobRoundTrip(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Attachment att = new Attachment(
    Name = 'note.txt',
    ParentId = a.Id,
    Body = Blob.valueOf('hello')
);
insert att;
List<Attachment> rows = [SELECT Id, ParentId, Body FROM Attachment WHERE Id = :att.Id];
System.assertEquals(1, rows.size());
Attachment row = rows.get(0);
System.assertEquals(a.Id, row.ParentId);
System.assertEquals('hello', row.Body.toString());
System.assertEquals('aGVsbG8=', EncodingUtil.base64Encode(row.Body));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDocumentBlobRoundTripAndDelete(t *testing.T) {
	program, err := CompileAnonymous(`
Document doc = new Document(
    Name = 'Terms.pdf',
    DeveloperName = 'Terms',
    Body = Blob.valueOf('document bytes'),
    ContentType = 'application/pdf',
    Type = 'pdf',
    IsPublic = true
);
insert doc;
List<Document> rows = [SELECT Id, DeveloperName, Body, ContentType, Type, IsPublic FROM Document WHERE Id = :doc.Id];
System.assertEquals(1, rows.size());
Document stored = rows.get(0);
System.assertEquals('Terms', stored.DeveloperName);
System.assertEquals('document bytes', stored.Body.toString());
System.assertEquals('application/pdf', stored.ContentType);
System.assertEquals('pdf', stored.Type);
System.assert(stored.IsPublic);
delete doc;
System.assertEquals(0, [SELECT Id FROM Document WHERE Id = :doc.Id].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecContentVersionCreatesDocumentAndLink(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
ContentVersion version = new ContentVersion(
    Title = 'Spec',
    PathOnClient = 'docs/spec.txt',
    VersionData = EncodingUtil.base64Decode('aGVsbG8='),
    FirstPublishLocationId = a.Id
);
insert version;
List<ContentVersion> versions = [SELECT Id, ContentDocumentId, VersionData FROM ContentVersion WHERE Id = :version.Id];
System.assertEquals(1, versions.size());
ContentVersion stored = versions.get(0);
System.assertEquals('hello', stored.VersionData.toString());
Id docId = stored.ContentDocumentId;
List<ContentDocument> docs = [SELECT Id, Title, LatestPublishedVersionId FROM ContentDocument WHERE Id = :docId];
System.assertEquals(1, docs.size());
ContentDocument doc = docs.get(0);
System.assertEquals('Spec', doc.Title);
System.assertEquals(version.Id, doc.LatestPublishedVersionId);
List<ContentDocumentLink> links = [SELECT Id, ContentDocumentId, LinkedEntityId FROM ContentDocumentLink WHERE ContentDocumentId = :docId];
System.assertEquals(1, links.size());
ContentDocumentLink link = links.get(0);
System.assertEquals(a.Id, link.LinkedEntityId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribePicklistValues(t *testing.T) {
	program, err := CompileAnonymous(`
Object describe = Account.Rating.getDescribe();
System.assertEquals('Rating', describe.getName());
System.assertEquals('PICKLIST', describe.getType());
List<Object> values = describe.getPicklistValues();
System.assertEquals(2, values.size());
Object hot = values.get(0);
System.assertEquals('Hot', hot.getValue());
System.assertEquals('Hot Label', hot.getLabel());
System.assert(hot.isDefaultValue());
System.assert(hot.isActive());
Object cold = values.get(1);
System.assertEquals('Cold', cold.getValue());
System.assert(!cold.isDefaultValue());
System.assert(!cold.isActive());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{
		APIName: "Rating",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot Label", Default: true, Active: true},
			{Value: "Cold", Label: "Cold", Active: false},
		},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeRecordTypeInfos(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> describes = Schema.getGlobalDescribe();
Object accountType = describes.get('Account');
Object accountDescribe = accountType.getDescribe();
System.assertEquals('Account', accountDescribe.getName());
List<Object> infos = accountDescribe.getRecordTypeInfos();
System.assertEquals(2, infos.size());
Map<String,Object> byName = accountDescribe.getRecordTypeInfosByName();
Map<String,Object> byDeveloperName = accountDescribe.getRecordTypeInfosByDeveloperName();
Map<String,Object> byId = accountDescribe.getRecordTypeInfosById();
Object business = byName.get('Business Account');
System.assertEquals('Business Account', business.getName());
System.assertEquals('Business', business.getDeveloperName());
System.assertEquals('012000000000001', business.getRecordTypeId());
System.assert(business.isActive());
System.assert(business.isAvailable());
System.assert(business.isDefaultRecordTypeMapping());
Object consumer = byDeveloperName.get('Consumer');
System.assertEquals('Consumer Account', consumer.getName());
System.assertEquals('Consumer', consumer.getDeveloperName());
System.assertEquals('012000000000002', consumer.getRecordTypeId());
System.assertEquals('Business', byId.get('012000000000001').getDeveloperName());
System.assertEquals('Consumer', byId.get('012000000000002').getDeveloperName());
System.assert(!consumer.isActive());
System.assert(!consumer.isAvailable());
System.assert(!consumer.isDefaultRecordTypeMapping());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{ID: "012000000000001", DeveloperName: "Business", Name: "Business Account", Active: true, Available: true, Default: true},
		{ID: "012000000000002", DeveloperName: "Consumer", Name: "Consumer Account", Active: false, Available: false},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeAccountRecordTypeInfosUsesPersonAccountFallback(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> byName = Account.SObjectType.getDescribe().getRecordTypeInfosByName();
Object individual = byName.get('Individual');
System.assertNotEquals(null, individual);
System.assertEquals('Individual', individual.getName());
System.assertNotEquals(null, individual.getRecordTypeId());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["PersonAccount"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "PersonAccount",
			KeyPrefix: "001",
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000201", DeveloperName: "Individual", Name: "Individual", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeRecordTypeInfosByNameForCustomObject(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> byName = Schema.SObjectType.Batch__c.getRecordTypeInfosByName();
System.assertEquals(2, byName.size());
Object scheduled = byName.get('Scheduled Batch');
System.assertEquals('Scheduled Batch', scheduled.getName());
System.assertEquals('Scheduled', scheduled.getDeveloperName());
System.assertEquals('012000000000101', scheduled.getRecordTypeId());
System.assert(scheduled.isActive());
System.assert(scheduled.isAvailable());
System.assert(scheduled.isDefaultRecordTypeMapping());
Object adHoc = byName.get('Ad_Hoc');
System.assertEquals('Ad_Hoc', adHoc.getName());
System.assertEquals('Ad_Hoc', adHoc.getDeveloperName());
System.assertEquals('012000000000102', adHoc.getRecordTypeId());
System.assert(!adHoc.isDefaultRecordTypeMapping());
System.assertEquals('Scheduled Batch', byName.get('Scheduled').getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			Label:     "Batch",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true, Available: true, Default: true},
				{ID: "012000000000102", DeveloperName: "Ad_Hoc", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeInfoActiveRecordTypeDefaultsAvailable(t *testing.T) {
	program, err := CompileAnonymous(`
Object info = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Scheduled Batch');
System.assert(info.isActive());
System.assert(info.isAvailable());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeInfoIdComparesToIdValues(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Scheduled Batch').getRecordTypeId();
for (Schema.RecordTypeInfo info : Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().values()) {
	if (info.getRecordTypeId() == recordTypeId) {
		System.assertEquals('Scheduled Batch', info.getName());
		return;
	}
}
System.assert(false);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeInfoIdComparesFifteenAndEighteenCharacterIds(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordTypeId = '012000000000101AAA';
for (Schema.RecordTypeInfo info : Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().values()) {
	if (info.getRecordTypeId() == recordTypeId) {
		System.assertEquals('Scheduled Batch', info.getName());
		return;
	}
}
System.assert(false);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeIdFieldTokenOnCustomObjectWithRecordTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectField field = Batch__c.RecordTypeId;
System.assertEquals('RecordTypeId', field.getDescribe().getName());
Batch__c record = Batch__c.SObjectType.newSObject(null, true);
Id recordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Scheduled Batch').getRecordTypeId();
record.put(field, recordTypeId);
System.assertEquals(recordTypeId, record.RecordTypeId);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeIdSurvivesChildRelationshipQuery(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordTypeId = Child__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Scheduled Batch').getRecordTypeId();
Child__c defaultChild = (Child__c)Child__c.SObjectType.newSObject(null, true);
System.assertEquals(recordTypeId, defaultChild.RecordTypeId);
Parent__c parent = new Parent__c(Name = 'P');
insert parent;
insert new Child__c(Name = 'C', Parent__c = parent.Id, RecordTypeId = recordTypeId);
Parent__c queried = [SELECT Id, (SELECT Id, Parent__c, RecordTypeId FROM Children__r) FROM Parent__c WHERE Id = :parent.Id];
Child__c child = queried.Children__r[0];
Map<Id, Parent__c> parentsById = new Map<Id, Parent__c>(new List<Parent__c>{queried});
System.assertNotEquals(null, parentsById.get(child.Parent__c));
System.assertNotEquals(null, parentsById.get(String.valueOf(child.Parent__c) + 'AAA'));
update queried;
Schema.RecordTypeInfo matched;
for (Schema.RecordTypeInfo info : Child__c.SObjectType.getDescribe().getRecordTypeInfosByName().values()) {
	if (info.getRecordTypeId() == child.RecordTypeId) {
		matched = info;
	}
}
System.assertNotEquals(null, matched);
System.assertEquals('Scheduled Batch', matched.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Parent__c",
				ParentObjects:      []string{"Parent__c"},
				ParentRelationship: "Parent__r",
				ChildRelationship:  "Children__r",
			}},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeRecordTypeInfosByNameEmptyForCustomObject(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> byName = Schema.SObjectType.Batch__c.getRecordTypeInfosByName();
System.assertEquals(0, byName.size());
System.assertEquals(null, byName.get('Scheduled Batch'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			Label:     "Batch",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGlobalDescribeReturnsSObjectTypeTokens(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> describes = Schema.getGlobalDescribe();
System.assert(describes.containsKey('Account'));
Object accountType = describes.get('Account');
Object accountDescribe = accountType.getDescribe();
System.assertEquals('Account', accountDescribe.getName());
List<String> names = new List<String>{'Account'};
List<Object> byName = Schema.describeSObjects(names);
System.assertEquals('Account', byName.get(0).getName());
List<Object> tokens = new List<Object>{accountType};
List<Object> byToken = Schema.describeSObjects(tokens);
System.assertEquals('Account', byToken.get(0).getName());
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

func TestExecDescribeFieldsMapFromSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
Object accountType = Account.SObjectType;
Object accountDescribe = accountType.getDescribe();
Object fieldsToken = accountDescribe.fields;
Map<String,Object> fields = fieldsToken.getMap();
System.assert(fields.containsKey('Name'));
System.assert(fields.containsKey('name'));
Object nameField = fields.get('Name');
Object lowercaseNameField = fields.get('name');
Object nameDescribe = nameField.getDescribe();
Object lowercaseNameDescribe = lowercaseNameField.getDescribe();
System.assertEquals('Name', nameDescribe.getName());
System.assertEquals('Name', nameField.getName());
System.assertEquals('Name', lowercaseNameDescribe.getName());
System.assert(nameDescribe.isNameField());
System.assert(!nameDescribe.isEncrypted());
System.assert(!nameDescribe.isCalculated());
System.assert(!nameDescribe.isCustom());
System.assertEquals('STRING', nameDescribe.getType());
System.assert(nameField.isAccessible());
System.assert(nameField.isCreateable());
System.assert(nameField.isUpdateable());
System.assert(Schema.sObjectType.Account.fields.Name.isUpdateable());
Object secretField = fields.get('Secret__c');
Object secretDescribe = secretField.getDescribe();
System.assert(secretDescribe.isEncrypted());
System.assert(secretDescribe.isCustom());
Object totalField = fields.get('Total__c');
Object totalDescribe = totalField.getDescribe();
System.assert(totalDescribe.isCalculated());
Object schemaType = Schema.SObjectType.Account;
System.assertEquals('Account', String.valueOf(schemaType));
Object schemaDescribe = schemaType.getDescribe();
System.assertEquals('Account', schemaDescribe.getName());
Object describedType = schemaDescribe.getSObjectType();
System.assertEquals('Account', describedType.getDescribe().getName());
System.assert(schemaDescribe.isAccessible());
System.assert(schemaDescribe.isCreateable());
System.assert(schemaDescribe.isUpdateable());
System.assert(schemaDescribe.isDeletable());
System.assert(schemaDescribe.isQueryable());
System.assert(schemaDescribe.isSearchable());
System.assert(!schemaDescribe.isCustom());
List<String> names = new List<String>{'Account'};
List<Object> describedByName = Schema.describeSObjects(names);
System.assertEquals(1, describedByName.size());
Object describedAccount = describedByName.get(0);
System.assertEquals('Account', describedAccount.getName());
List<Object> childRelationships = schemaDescribe.getChildRelationships();
System.assertEquals(1, childRelationships.size());
Object contacts = childRelationships.get(0);
System.assertEquals('Contacts', contacts.getRelationshipName());
Object childField = contacts.getField();
Object childFieldDescribe = childField.getDescribe();
System.assertEquals('AccountId', childFieldDescribe.getName());
System.assert(childFieldDescribe.isAccessible());
System.assert(childFieldDescribe.isCreateable());
System.assert(childFieldDescribe.isUpdateable());
System.assert(!childFieldDescribe.isNameField());
System.assert(!childFieldDescribe.isEncrypted());
Object childType = contacts.getChildSObject();
Object childDescribe = childType.getDescribe();
System.assertEquals('Contact', childDescribe.getName());
System.assert(contacts.isCascadeDelete());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldString, Encrypted: true}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
				CascadeDelete:      true,
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []struct {
		name     string
		category string
	}{
		{"apex.describe.sobject", "apex.describe"},
		{"apex.describe.fields", "apex.describe"},
		{"apex.describe.field", "apex.describe"},
		{"apex.describe.sobjects", "apex.describe"},
	} {
		if !traceHas(result.Trace, event.name, event.category) {
			t.Fatalf("trace missing %s/%s: %#v", event.name, event.category, result.Trace)
		}
	}
}

func TestExecDescribePermissionsHonorLocalObjectAndFieldPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'ReadAccountContact', Label = 'Read Account Contact');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Account', PermissionsRead = true);
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Contact', PermissionsRead = true);
insert new FieldPermissions(ParentId = ps.Id, SObjectType = 'Contact', Field = 'Contact.Email', PermissionsRead = true);
Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce'];
User u = new User(
	Username = 'minimum@example.invalid',
	Alias = 'minimum',
	Email = 'minimum@example.invalid',
	LastName = 'Minimum',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
System.runAs(u) {
	System.assert(Account.SObjectType.getDescribe().isAccessible());
	System.assert(!Account.SObjectType.getDescribe().isCreateable());
	System.assert(!Lead.SObjectType.getDescribe().isUpdateable());
	System.assert(!Opportunity.SObjectType.getDescribe().isDeletable());
	System.assert(!Account.Name.getDescribe().isCreateable());
	System.assert(Contact.LastName.getDescribe().isAccessible());
	System.assert(Contact.Email.getDescribe().isAccessible());
	System.assert(!Lead.Company.getDescribe().isUpdateable());
}
Profile admin = [SELECT Id FROM Profile WHERE Name = 'System Administrator'];
User sys = new User(
	Username = 'admin@example.invalid',
	Alias = 'admin',
	Email = 'admin@example.invalid',
	LastName = 'Admin',
	ProfileId = admin.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert sys;
System.runAs(sys) {
	System.assert(Account.SObjectType.getDescribe().isCreateable());
	System.assert(Account.Name.getDescribe().isCreateable());
	System.assert(Lead.Company.getDescribe().isUpdateable());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "Lead")
	storage.EnsureStandardObject(&org, "Opportunity")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLUserModeChecksLocalObjectAndFieldPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'ReadAccount', Label = 'Read Account');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Account', PermissionsRead = true);
Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce'];
User u = new User(
	Username = 'query-user@example.invalid',
	Alias = 'quser',
	Email = 'query-user@example.invalid',
	LastName = 'Query',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
System.runAs(u) {
	List<Account> readable = Database.query('SELECT Id, Name FROM Account WITH USER_MODE');
	System.assertEquals(1, readable.size());
	Boolean caught = false;
	try {
		Database.query('SELECT Id, Score__c FROM Account WITH USER_MODE');
	} catch (QueryException qe) {
		caught = qe.getMessage().contains('Score__c');
	}
	System.assert(caught);
	List<Account> systemRows = Database.query('SELECT Id, Score__c FROM Account WITH SYSTEM_MODE');
	System.assertEquals(1, systemRows.size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldDecimal}
	account.Records["001000000000901AAA"] = storage.Record{
		ID:     "001000000000901AAA",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":     storage.StringValue("Acme"),
			"Score__c": storage.DecimalValue("7"),
		},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomObjectIdDescribeNameFeedsDynamicFieldList(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> fields = new List<String>();
fields.add(Invoice__c.Amount__c.getDescribe().getName());
fields.add(Invoice__c.Id.getDescribe().getName());
fields.add(Invoice__c.Name.getDescribe().getName());
String query = 'SELECT ' + String.join(fields, ',') + ' FROM Invoice__c';
System.assertEquals('SELECT Amount__c,Id,Name FROM Invoice__c', query);
List<Invoice__c> rows = Database.query(query);
System.assertEquals(1, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Invoice__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Invoice__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
				"Name":      {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001AAA": {
				ID:     "a00000000000001AAA",
				Object: "Invoice__c",
				Fields: map[string]storage.Value{
					"Amount__c": storage.DecimalValue("42"),
					"Name":      storage.StringValue("INV-001"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSchemaMetadataEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Object accountDescribe = Account.SObjectType.getDescribe();
System.assertEquals('Account', accountDescribe.getLabel());
System.assertEquals('Accounts', accountDescribe.getLabelPlural());
System.assertEquals('001', accountDescribe.getKeyPrefix());
Object fieldsToken = accountDescribe.getFields();
Map<String,Object> fields = fieldsToken.getMap();
System.assert(fields.containsKey('Name'));
Object contactFieldDescribe = Contact.AccountId.getDescribe();
System.assertEquals('Account', contactFieldDescribe.getLabel());
System.assertEquals('REFERENCE', contactFieldDescribe.getType());
System.assert(contactFieldDescribe.isNillable());
System.assert(contactFieldDescribe.isAccessible());
System.assert(contactFieldDescribe.isCreateable());
System.assert(contactFieldDescribe.isUpdateable());
System.assertEquals('Account', contactFieldDescribe.getRelationshipName());
List<Object> references = contactFieldDescribe.getReferenceTo();
System.assertEquals(1, references.size());
Object accountType = references.get(0);
Object referencedDescribe = accountType.getDescribe();
System.assertEquals('Account', referencedDescribe.getName());
Object nameDescribe = Account.Name.getDescribe();
System.assertEquals('Account Name', nameDescribe.getLabel());
System.assertEquals(null, nameDescribe.getRelationshipName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.PluralLabel = "Accounts"
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Contact",
			Label:       "Contact",
			PluralLabel: "Contacts",
			KeyPrefix:   "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Label: "Last Name", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Label: "Account", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeTabsFromLocalMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> tabSets = Schema.describeTabs();
System.assertEquals(1, tabSets.size());
Object tabSet = tabSets.get(0);
System.assertEquals('All Tabs', tabSet.getLabel());
System.assertEquals('AllTabs', tabSet.getName());
System.assert(!tabSet.isSelected());
List<Object> tabs = tabSet.getTabs();
System.assertEquals(1, tabs.size());
Object tab = tabs.get(0);
System.assertEquals('Widget__c', tab.getName());
System.assertEquals('Widgets', tab.getLabel());
System.assertEquals('Widget__c', tab.getSObjectName());
System.assert(tab.isCustom());
System.assertEquals('Custom1: Heart', tab.getIconUrl());
System.assertEquals('/lightning/o/Widget__c/list', tab.getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.Tabs = []storage.TabMetadata{{
		Name:        "Widget__c",
		Label:       "Widgets",
		SObjectName: "Widget__c",
		Custom:      true,
		Motif:       "Custom1: Heart",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeUnsupportedMetadataEdges(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "dependent picklist controller metadata",
			source: `Account.Rating.getDescribe().getController();`,
			want:   `unsupported call "Schema.DescribeFieldResult.getController dependent picklist controller metadata"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldPicklist}
			org.Objects["Account"] = account
			machine.SetOrg(&org)
			_, err = machine.Execute(program)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestExecDescribeFieldSetsFromMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Object accountDescribe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = accountDescribe.fieldSets.getMap();
System.assert(fieldSets.containsKey('Summary'));
Object summary = fieldSets.get('Summary');
System.assertEquals('Account Summary', summary.getLabel());
List<Object> members = summary.getFields();
System.assertEquals(2, members.size());
Object nameMember = members.get(0);
System.assertEquals('Name', nameMember.getFieldPath());
System.assertEquals('Account Name', nameMember.getLabel());
System.assertEquals(Schema.SOAPType.STRING, nameMember.getType());
System.assert(nameMember.getRequired());
System.assert(nameMember.getDbRequired());
Object ratingMember = Schema.SObjectType.Account.fieldSets.Summary.getFields().get(1);
System.assertEquals('Rating', ratingMember.getFieldPath());
System.assertEquals('Rating', ratingMember.getLabel());
System.assert(!ratingMember.getRequired());
System.assert(!ratingMember.getDbRequired());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true}
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Label: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "Account",
		Name:       "Summary",
		Label:      "Account Summary",
		Fields: []storage.FieldSetMemberMetadata{
			{Field: "Name", Required: true},
			{Field: "Rating"},
		},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLCountAndSingleSObjectAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Integer countRows = [SELECT COUNT() FROM Account];
System.assertEquals(1, countRows);
Account row = [SELECT Id, Name FROM Account WHERE Name = 'Acme'];
System.assertEquals('Acme', row.Name);
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

func TestExecDynamicSOQLBindsAndQueryException(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', RenewalDate__c = Date.today());
insert new Account(Name = 'Beta', RenewalDate__c = Date.today());
String wanted = 'Acme';
List<Account> rows = Database.query('SELECT Id, Name FROM Account WHERE Name=:wanted');
System.assertEquals(1, rows.size());
Account first = rows.get(0);
System.assertEquals('Acme', first.Name);
List<String> names = new List<String>{'Acme', 'Beta'};
rows = Database.query('SELECT Id FROM Account WHERE Name IN :names ORDER BY Name');
System.assertEquals(2, rows.size());
rows = Database.query('SELECT Id FROM Account WHERE Name = :names ORDER BY Name');
System.assertEquals(2, rows.size());
Account probe = new Account(Name = 'Beta');
rows = Database.query('SELECT Id FROM Account WHERE Name = :probe.Name');
System.assertEquals(1, rows.size());
Map<Id, Account> accountsById = new Map<Id, Account>();
accountsById.put(rows[0].Id, rows[0]);
rows = Database.query('SELECT Id FROM Account WHERE Id IN :accountsById.values()');
System.assertEquals(1, rows.size());
rows = Database.query('SELECT Id FROM Account WHERE RenewalDate__c = LAST_N_DAYS:2');
System.assertEquals(2, rows.size());
Boolean caught = false;
try {
    Database.query('SELECT FROM Account');
} catch (QueryException qe) {
    caught = true;
    String message = qe.getMessage();
    System.assert(message != null);
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLCoercesDateIntoDateTimeField(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', LastSeen__c = Date.newInstance(2026, 5, 2));
Account row = [SELECT LastSeen__c FROM Account LIMIT 1];
System.assertEquals(Datetime.newInstanceGmt(2026, 5, 2, 0, 0, 0), row.LastSeen__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["LastSeen__c"] = storage.Field{APIName: "LastSeen__c", Type: storage.FieldDateTime}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSummaryFieldSumsChildRecords(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Acme');
insert parent;
insert new WidgetLine__c(Account__c = parent.Id, Amount__c = 4, IsCoupon__c = false);
insert new WidgetLine__c(Account__c = parent.Id, Amount__c = 3, IsCoupon__c = false);
insert new WidgetLine__c(Account__c = parent.Id, Amount__c = 9, IsCoupon__c = true);
Account row = [SELECT SubTotal__c FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assertEquals(7, row.SubTotal__c);
row.Name = 'Changed';
update row;
Account changed = [SELECT Name FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assertEquals('Changed', changed.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["SubTotal__c"] = storage.Field{
		APIName:           "SubTotal__c",
		Type:              storage.FieldSummary,
		DisplayType:       "DECIMAL",
		SummarizedField:   "WidgetLine__c.Amount__c",
		SummaryForeignKey: "WidgetLine__c.Account__c",
		SummaryOperation:  "sum",
		SummaryFilterItems: []storage.SummaryFilterItem{{
			Field:     "WidgetLine__c.IsCoupon__c",
			Operation: "equals",
			Value:     "False",
		}},
	}
	org.Objects["Account"] = account
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Account__c":  {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
				"Amount__c":   {APIName: "Amount__c", Type: storage.FieldDecimal},
				"IsCoupon__c": {APIName: "IsCoupon__c", Type: storage.FieldBoolean},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseQueryWithBinds(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot');
insert new Account(Name = 'Beta', Rating = 'Warm');
Map<String,Object> binds = new Map<String,Object>();
binds.put('wanted', 'Acme');
List<Account> rows = Database.queryWithBinds('SELECT Id, Name FROM Account WHERE Name = :wanted', binds);
System.assertEquals(1, rows.size());
Account row = rows.get(0);
System.assertEquals('Acme', row.Name);
List<String> ratings = new List<String>{'Hot', 'Warm'};
binds.put('ratings', ratings);
rows = Database.queryWithBinds('SELECT Id FROM Account WHERE Rating IN :ratings ORDER BY Name', binds, AccessLevel.USER_MODE);
System.assertEquals(2, rows.size());
Boolean caught = false;
try {
    Database.queryWithBinds('SELECT Id FROM Account WHERE Name = :missing', binds);
} catch (QueryException qe) {
    caught = true;
    String message = qe.getMessage();
    System.assert(message.contains('missing'));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	badProgram, err := CompileAnonymous(`
Map<String,Object> binds = new Map<String,Object>();
binds.put('wanted', 'Acme');
Database.queryWithBinds('SELECT Id FROM Account WHERE Name = :wanted', binds, 'USER_MODE');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(badProgram); err == nil || !strings.Contains(err.Error(), "AccessLevel") {
		t.Fatalf("expected AccessLevel error, got %v", err)
	}
}

func TestExecSOQLBindStaticMethodCall(t *testing.T) {
	program, err := CompileAnonymous(`
User row = [SELECT Id FROM User WHERE Id = :UserInfo.getUserId() LIMIT 1];
System.assertEquals(UserInfo.getUserId(), row.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	user := org.Objects["User"]
	user.Records["system"] = storage.Record{ID: "system", Object: "User", Fields: map[string]storage.Value{"LastName": storage.StringValue("System")}}
	org.Objects["User"] = user
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLOrderByDesc(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot');
insert new Account(Name = 'Beta', Rating = 'Hot');
insert new Account(Name = 'Gamma');
List<Account> rows = [SELECT Id, Name FROM Account ORDER BY Rating ASC NULLS LAST, Name DESC LIMIT 1];
System.assertEquals(1, rows.size());
Account row = rows.get(0);
System.assertEquals('Beta', row.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Account"].Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLFieldsFunction(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot', Score__c = 7);
Account row = [SELECT FIELDS(ALL) FROM Account WHERE Name = 'Acme'];
System.assertEquals('Acme', row.Name);
System.assertEquals('Hot', row.Rating);
System.assertEquals(7, row.Score__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldInteger}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLTypeofProjection(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Task t = new Task(Subject = 'Call', WhatId = a.Id);
insert t;
Task row = [SELECT Id, TYPEOF What WHEN Account THEN Name END FROM Task WHERE Id = :t.Id];
System.assertEquals('Acme', row.What.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Task",
			KeyPrefix: "00T",
			Fields: map[string]storage.Field{
				"Subject": {APIName: "Subject", Type: storage.FieldString},
				"WhatId":  {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "What",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLPolymorphicRelationshipProjection(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Contact c = new Contact(LastName = 'Smith');
insert c;
Task accountTask = new Task(Subject = 'A', WhoId = a.Id);
Task contactTask = new Task(Subject = 'C', WhoId = c.Id);
insert new List<Task>{accountTask, contactTask};
List<Task> rows = [SELECT Id, Subject, Who.Name, TYPEOF Who WHEN Account THEN Name WHEN Contact THEN LastName END FROM Task ORDER BY Subject];
System.assertEquals(2, rows.size());
Task first = rows.get(0);
System.assertEquals('Acme', first.Who.Name);
Task second = rows.get(1);
System.assertEquals('Smith', second.Who.LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Task",
			KeyPrefix: "00T",
			Fields: map[string]storage.Field{
				"Subject": {APIName: "Subject", Type: storage.FieldString},
				"WhoId":   {APIName: "WhoId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Contact"}, RelationshipName: "Who"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhoId",
				ParentObjects:      []string{"Account", "Contact"},
				ParentRelationship: "Who",
				Polymorphic:        true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLForUpdate(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Acme' WITH SECURITY_ENFORCED FOR UPDATE];
System.assertEquals(1, rows.size());
Account row = rows.get(0);
System.assertEquals('Acme', row.Name);
Boolean caught = false;
try {
    Database.query('SELECT Missing__c FROM Account WITH SECURITY_ENFORCED');
} catch (QueryException qe) {
    caught = true;
    String message = qe.getMessage();
    System.assert(message.contains('Missing__c'));
}
System.assert(caught);
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

func TestExecSOQLSecurityRelationshipProjectionRequiresAllParentTargets(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
    Database.query('SELECT Id, What.Secret__c FROM Task WITH SECURITY_ENFORCED');
} catch (QueryException qe) {
    caught = true;
    String message = qe.getMessage();
    System.assert(message.contains('What.Secret__c'));
    System.assert(message.contains('SECURITY_ENFORCED'));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Opportunity",
			KeyPrefix: "006",
			Fields: map[string]storage.Field{
				"Amount": {APIName: "Amount", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Task",
			KeyPrefix: "00T",
			Fields: map[string]storage.Field{
				"WhatId": {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Opportunity"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account", "Opportunity"},
				ParentRelationship: "What",
				Polymorphic:        true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLSecurityRelationshipWhereRequiresAllParentTargets(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
    Database.query('SELECT Id FROM Task WHERE What.Secret__c = ''hidden'' WITH SECURITY_ENFORCED');
} catch (QueryException qe) {
    caught = true;
    String message = qe.getMessage();
    System.assert(message.contains('What.Secret__c'));
    System.assert(message.contains('SECURITY_ENFORCED'));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Opportunity",
			KeyPrefix: "006",
			Fields: map[string]storage.Field{
				"Amount": {APIName: "Amount", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Task"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Task",
			KeyPrefix: "00T",
			Fields: map[string]storage.Field{
				"WhatId": {APIName: "WhatId", Type: storage.FieldReference, ReferenceTo: []string{"Account", "Opportunity"}, RelationshipName: "What"},
			},
			Relations: []storage.Relationship{{
				Field:              "WhatId",
				ParentObjects:      []string{"Account", "Opportunity"},
				ParentRelationship: "What",
				Polymorphic:        true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLForUpdateLockContentionIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
    List<Account> rows = [SELECT Id FROM Account WHERE Id = '001000000000001' FOR UPDATE];
} catch (QueryException qe) {
    caught = true;
    String message = qe.getMessage();
    System.assert(message.contains('unable to lock row 001000000000001'));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Locked"),
		},
		System: storage.SystemFields{Locked: true},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLRowJSONAttributesURL(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Account row = [SELECT Id, Name FROM Account WHERE Id = :a.Id];
String text = JSON.serialize(row);
Object decoded = JSON.deserializeUntyped(text);
Object attrs = decoded.get('attributes');
System.assertEquals('Account', attrs.get('type'));
System.assertEquals('/services/data/v60.0/sobjects/Account/' + a.Id, attrs.get('url'));
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

func TestExecInsertedSObjectMissingFieldReturnsNull(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
System.assertEquals(null, a.MasterRecordId);
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

func TestExecMissingCalculatedNumericFieldDefaultsToZero(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme', Amount__c = 2, Paid__c = 3);
insert a;
System.assertEquals('-1', String.valueOf(a.Balance__c));
Account row = [SELECT Id, Balance__c FROM Account WHERE Id = :a.Id];
System.assertEquals('-1', String.valueOf(row.Balance__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	account.Definition.Fields["Paid__c"] = storage.Field{APIName: "Paid__c", Type: storage.FieldDecimal}
	account.Definition.Fields["Balance__c"] = storage.Field{APIName: "Balance__c", Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: "Amount__c - Paid__c"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLAllRows(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
delete a;
List<Account> visible = [SELECT Id FROM Account WHERE Id = :a.Id];
System.assertEquals(0, visible.size());
List<Account> rows = [SELECT Id, IsDeleted FROM Account WHERE Id = :a.Id ALL ROWS];
System.assertEquals(1, rows.size());
Account row = rows.get(0);
System.assert(row.IsDeleted);
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

func TestExecDatabaseEmptyRecycleBinResult(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
delete a;
Database.EmptyRecycleBinResult result = Database.emptyRecycleBin(a, false);
System.assert(result.isSuccess());
System.assertEquals(a.Id, result.getId());
System.assertEquals(0, result.getErrors().size());
List<Account> rows = [SELECT Id FROM Account WHERE Id = :a.Id ALL ROWS];
System.assertEquals(0, rows.size());
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

func TestExecDatabaseLockUnlockResultShapes(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Database.LockResult locked = Database.lock(a, false);
System.assert(locked.isSuccess());
System.assertEquals(a.Id, locked.getId());
System.assertEquals(0, locked.getErrors().size());
Database.UnlockResult unlocked = Database.unlock(a, false);
System.assert(unlocked.isSuccess());
System.assertEquals(a.Id, unlocked.getId());
System.assertEquals(0, unlocked.getErrors().size());
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

func TestExecApprovalAndConvertLeadReturnUnsupportedFeature(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{
			name:    "convertLead",
			source:  "Database.convertLead(new Database.LeadConvert());",
			message: `unsupported call "Database.convertLead local lead conversion surface"`,
		},
		{
			name:    "approvalProcess",
			source:  "Approval.process(null);",
			message: `unsupported call "Approval.process local approval process and lock surface"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.message {
				t.Fatalf("error = %#v, want UnsupportedFeature %q", err, tc.message)
			}
		})
	}
}

func TestExecSOQLSemiJoinPredicates(t *testing.T) {
	program, err := CompileAnonymous(`
Account acme = new Account(Name = 'Acme');
insert acme;
insert new Account(Name = 'Beta');
insert new Contact(LastName = 'Smith', AccountId = acme.Id);
List<Account> matched = [SELECT Id, Name FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')];
System.assertEquals(1, matched.size());
Account matchedRow = matched.get(0);
System.assertEquals('Acme', matchedRow.Name);
List<Account> unmatched = [SELECT Id, Name FROM Account WHERE Id NOT IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')];
System.assertEquals(1, unmatched.size());
Account unmatchedRow = unmatched.get(0);
System.assertEquals('Beta', unmatchedRow.Name);
List<Contact> contacts = [SELECT Id FROM Contact WHERE AccountId IN (SELECT Id FROM Account WHERE Name = 'Acme')];
System.assertEquals(1, contacts.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLAggregateResultFields(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', AnnualRevenue = 100, Rating = 'Hot');
insert new Account(Name = 'Beta', AnnualRevenue = 250, Rating = 'Warm');
insert new Account(Name = 'Gamma', AnnualRevenue = 300, Rating = 'Hot');
List<Object> rows = [SELECT COUNT(Name) namedCount, COUNT_DISTINCT(Rating), SUM(AnnualRevenue) totalRevenue, MIN(AnnualRevenue), MAX(AnnualRevenue), AVG(AnnualRevenue) averageRevenue FROM Account];
System.assertEquals(1, rows.size());
Object row = rows.get(0);
System.assertEquals(3, row.expr0);
System.assertEquals(2, row.expr1);
System.assertEquals(650.0, row.expr2);
System.assertEquals(100.0, row.expr3);
System.assertEquals(300.0, row.expr4);
System.assertEquals(216.6666666667, row.expr5);
System.assertEquals(3, row.namedCount);
System.assertEquals(650.0, row.totalRevenue);
System.assertEquals(216.6666666667, row.averageRevenue);
List<Object> grouped = [SELECT Rating, COUNT(Id) accountCount, SUM(AnnualRevenue) totalRevenue FROM Account GROUP BY Rating HAVING accountCount > 1 ORDER BY totalRevenue];
System.assertEquals(1, grouped.size());
Object groupRow = grouped.get(0);
System.assertEquals('Hot', groupRow.Rating);
System.assertEquals(2, groupRow.expr0);
System.assertEquals(400.0, groupRow.expr1);
System.assertEquals(2, groupRow.accountCount);
System.assertEquals(400.0, groupRow.totalRevenue);
List<Object> hiddenHaving = [SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING SUM(AnnualRevenue) > 300];
System.assertEquals(1, hiddenHaving.size());
Object hiddenRow = hiddenHaving.get(0);
System.assertEquals('Hot', hiddenRow.Rating);
System.assertEquals(2, hiddenRow.accountCount);
List<Object> groupedOnlyHiddenHaving = [SELECT Rating FROM Account GROUP BY Rating HAVING SUM(AnnualRevenue) > 300];
System.assertEquals(1, groupedOnlyHiddenHaving.size());
Object groupedOnlyHiddenRow = groupedOnlyHiddenHaving.get(0);
System.assertEquals('Hot', groupedOnlyHiddenRow.Rating);
List<Object> rollupRows = [SELECT Rating, COUNT(Id) accountCount, GROUPING(Rating) ratingGrouped FROM Account GROUP BY ROLLUP(Rating) ORDER BY ratingGrouped];
System.assertEquals(3, rollupRows.size());
Object totalRow = rollupRows.get(2);
System.assertEquals(null, totalRow.Rating);
System.assertEquals(3, totalRow.accountCount);
System.assertEquals(1, totalRow.ratingGrouped);
List<Object> cubeRows = [SELECT Rating, Name, COUNT(Id) accountCount, GROUPING(Rating) ratingGrouped, GROUPING(Name) nameGrouped FROM Account GROUP BY CUBE(Rating, Name) HAVING accountCount >= 2];
System.assertEquals(2, cubeRows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLAggregateValidationQueryExceptions(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', AnnualRevenue = 100, Rating = 'Hot');
insert new Account(Name = 'Beta', AnnualRevenue = 250, Rating = 'Warm');
Boolean caughtHaving = false;
try {
    Database.query('SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Missing__c = ''x''');
} catch (QueryException qe) {
    caughtHaving = qe.getMessage().contains('Missing__c');
}
System.assert(caughtHaving);
Boolean caughtUngrouped = false;
try {
    Database.query('SELECT Rating, COUNT(Id) accountCount FROM Account GROUP BY Rating HAVING Name = ''Acme''');
} catch (QueryException qe) {
    caughtUngrouped = qe.getMessage().contains('must be grouped or aggregated');
}
System.assert(caughtUngrouped);
Boolean caughtAggregateField = false;
try {
    Database.query('SELECT SUM(Name) bad FROM Account');
} catch (QueryException qe) {
    caughtAggregateField = qe.getMessage().contains('SUM requires numeric field Name');
}
System.assert(caughtAggregateField);
Boolean caughtAlias = false;
try {
    Database.query('SELECT Rating, COUNT(Id) Rating FROM Account GROUP BY Rating');
} catch (QueryException qe) {
    caughtAlias = qe.getMessage().contains('conflicts with grouped field');
}
System.assert(caughtAlias);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLDateLiterals(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Today', RenewalDate__c = Date.today());
Date oldDate = Date.today();
oldDate = oldDate.addDays(-2);
insert new Account(Name = 'Old', RenewalDate__c = oldDate);
Date priorMonth = Date.newInstance(2026, 4, 15);
insert new Account(Name = 'Prior Month', RenewalDate__c = priorMonth);
List<Account> todayRows = [SELECT Id FROM Account WHERE RenewalDate__c = TODAY];
System.assertEquals(1, todayRows.size(), 'TODAY should match Date.today row');
List<Account> recentRows = [SELECT Id FROM Account WHERE RenewalDate__c = LAST_N_DAYS:2];
System.assertEquals(2, recentRows.size(), 'LAST_N_DAYS should include today and prior rows');
List<Account> monthRows = [SELECT Id FROM Account WHERE RenewalDate__c = LAST_N_MONTHS:1];
System.assertEquals(2, monthRows.size(), 'LAST_N_MONTHS should cover the complete prior month');
List<Account> quarterRows = [SELECT Id FROM Account WHERE RenewalDate__c = THIS_QUARTER];
System.assertEquals(3, quarterRows.size(), 'THIS_QUARTER should cover the current quarter');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMaterializesStoredDateFieldsForMethodDispatch(t *testing.T) {
	echo, err := CompileAnonymous("return input.format();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> rows = [SELECT Id, RenewalDate__c FROM Account WHERE Name = 'Acme'];
System.assertEquals('2026-05-02', DateWorker.echo(rows[0].RenewalDate__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate}
	record := storage.Record{ID: "001000000000001AAA", Object: "Account", Fields: map[string]storage.Value{
		"Name": storage.StringValue("Acme"),
	}}
	record.Fields["RenewalDate__c"] = storage.DateValue("2026-05-02")
	account.Records["001000000000001AAA"] = record
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "DateWorker",
		Methods: map[string]Method{
			"echo": {Name: "DateWorker.echo", ClassName: "DateWorker", ReturnType: "String", Params: []Param{{Name: "input", Type: "Date"}}, IsStatic: true, Access: "public", Program: echo},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLUnsupportedDateLiteralDiagnostic(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> rows = [SELECT Id FROM Account WHERE CreatedDate = THIS_WEEK];
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != "soql: date literal THIS_WEEK is not supported" {
		t.Fatalf("err = %#v", err)
	}
}

func TestExecDMLCoercesAndRejectsFieldValues(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
a.AnnualRevenue = 42;
insert a;
Account row = [SELECT Id, AnnualRevenue FROM Account WHERE Id = :a.Id];
System.assertEquals(42.5, row.AnnualRevenue + 0.5);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Account"].Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	badProgram, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
a.AnnualRevenue = 'forty-two';
insert a;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	org.Objects["Account"].Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	machine.SetOrg(&org)
	if _, err := machine.Execute(badProgram); err == nil {
		t.Fatalf("expected field coercion error")
	}
}

func TestExecSOQLParentRelationshipProjection(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Account child = new Account(Name = 'Child', ParentId = a.Id);
insert child;
Contact c = new Contact(AccountId = child.Id, LastName = 'Smith');
insert c;
Contact row = [SELECT Id, Account.Name, Account.Parent.Name FROM Contact WHERE Account.Parent.Name = 'Acme'];
System.assertEquals(child.Id, row.Account.Id);
System.assertEquals('Child', row.Account.Name);
System.assertEquals(a.Id, row.Account.Parent.Id);
System.assertEquals('Acme', row.Account.Parent.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ParentId"] = storage.Field{APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent"}
	account.Definition.Relations = append(account.Definition.Relations, storage.Relationship{Field: "ParentId", ParentObjects: []string{"Account"}, ParentRelationship: "Parent"})
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectSystemFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'System Fields');
insert a;
System.assert(a.CreatedDate != null);
System.assert(a.LastModifiedDate != null);
System.assert(a.SystemModstamp != null);
System.assertEquals('005000000000001', a.CreatedById);
System.assertEquals('005000000000001', a.LastModifiedById);
System.assertEquals('005000000000001', a.OwnerId);
System.assert(!a.IsDeleted);
System.assertEquals('2026-05-02T12:00:00Z', a.CreatedDate.format());
a.Name = 'System Fields Updated';
update a;
System.assert(a.LastModifiedDate != null);
Account row = [SELECT Id, CreatedDate, CreatedById, LastModifiedDate, LastModifiedById, SystemModstamp, OwnerId, IsDeleted FROM Account WHERE Id = :a.Id];
System.assertEquals('2026-05-02T12:00:00Z', row.CreatedDate.format());
System.assertEquals('005000000000001', row.CreatedById);
System.assertEquals('005000000000001', row.LastModifiedById);
System.assertEquals('005000000000001', row.OwnerId);
System.assert(!row.IsDeleted);
delete row;
System.assert(row.IsDeleted);
Account deletedRow = [SELECT Id, IsDeleted, LastModifiedDate, SystemModstamp FROM Account WHERE Id = :a.Id ALL ROWS];
System.assert(deletedRow.IsDeleted);
System.assert(deletedRow.LastModifiedDate != null);
System.assert(deletedRow.SystemModstamp != null);
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

func TestExecSOQLChildRelationshipSubquery(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
insert new Contact(AccountId = a.Id, LastName = 'Zulu');
insert new Contact(AccountId = a.Id, LastName = 'Alpha');
Account row = [SELECT Id, (SELECT Id, LastName FROM Contacts WHERE LastName != 'Zulu' ORDER BY LastName LIMIT 1) FROM Account WHERE Id = :a.Id];
List<Contact> contacts = row.Contacts;
System.assertEquals(1, contacts.size());
Contact contact = contacts.get(0);
System.assertEquals('Alpha', contact.LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
			},
			Relations: []storage.Relationship{{
				Field:              "AccountId",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account",
				ChildRelationship:  "Contacts",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedCustomObjectAndFieldAliases(t *testing.T) {
	program, err := CompileAnonymous(`
pkg__Thing__c item = new pkg__Thing__c(pkg__Name__c = 'Acme');
System.assertEquals('Acme', item.pkg__Name__c);
item.put('pkg__Name__c', 'Changed');
System.assertEquals('Changed', item.get('pkg__Name__c'));
insert item;
List<pkg__Thing__c> rows = [SELECT Id, pkg__Name__c FROM pkg__Thing__c WHERE pkg__Name__c = 'Changed'];
System.assertEquals(1, rows.size());
pkg__Thing__c row = rows.get(0);
System.assertEquals('Changed', row.pkg__Name__c);
row.pkg__Name__c = 'Updated';
update row;
List<pkg__Thing__c> updated = [SELECT Id FROM pkg__Thing__c WHERE pkg__Name__c = 'Updated'];
System.assertEquals(1, updated.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Thing__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name__c": {APIName: "Name__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseDMLAllOrNoneResults(t *testing.T) {
	program, err := CompileAnonymous(`
Account good = new Account(Name = 'Acme');
Account bad = new Account(Bogus__c = 'nope');
List<Account> records = new List<Account>{good, bad};
List<Object> results = Database.insert(records, false);
System.assertEquals(2, results.size());
Object first = results.get(0);
Object second = results.get(1);
System.assert(first.success);
System.assert(!second.success);
List<Account> rows = [SELECT Id FROM Account];
System.assertEquals(1, rows.size());
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

func TestExecExceptionMessageAndGetMessage(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	throw new DmlException('blocked');
} catch (DmlException e) {
	System.assertEquals('blocked', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLRejectsCalculatedFieldWrites(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
a.put('Score__c', null);
Object result = Database.insert(a, false);
System.assert(!result.isSuccess());
List<Object> errors = result.getErrors();
System.assertEquals(1, errors.size());
Object err = errors.get(0);
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE', err.getStatusCode());
System.assertEquals('Score__c', err.getFields().get(0));
List<Account> rows = [SELECT Id FROM Account];
System.assertEquals(0, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseErrorDetailsAndDmlExceptionParity(t *testing.T) {
	program, err := CompileAnonymous(`
Account existing = new Account(Name = 'Existing', Code__c = 'A');
insert existing;

Account missing = new Account(Code__c = 'M');
Account duplicate = new Account(Name = 'Duplicate', Code__c = 'a');
Account blocked = new Account(Name = 'Blocked', Code__c = 'B');
List<Account> records = new List<Account>{missing, duplicate, blocked};

List<Object> partial = Database.insert(records, false);
System.assertEquals(3, partial.size());
System.assertEquals('REQUIRED_FIELD_MISSING', partial.get(0).getErrors().get(0).getStatusCode());
System.assertEquals('Name', partial.get(0).getErrors().get(0).getFields().get(0));
System.assertEquals('DUPLICATE_VALUE', partial.get(1).getErrors().get(0).getStatusCode());
System.assertEquals('Code__c', partial.get(1).getErrors().get(0).getFields().get(0));
System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', partial.get(2).getErrors().get(0).getStatusCode());
System.assertEquals('Name', partial.get(2).getErrors().get(0).getFields().get(0));

Boolean caught = false;
try {
	Database.insert(records, true);
} catch (DmlException e) {
	caught = true;
	System.assertEquals(3, e.getNumDml());
	System.assertEquals(0, e.getDmlIndex(0));
	System.assertEquals('REQUIRED_FIELD_MISSING', e.getDmlStatusCode(0));
	System.assertEquals(partial.get(0).getErrors().get(0).getMessage(), e.getDmlMessage(0));
	System.assertEquals(partial.get(0).getErrors().get(0).getFields().get(0), e.getDmlFields(0).get(0));
	System.assertEquals(null, e.getDmlId(0));
	System.assertEquals(1, e.getDmlIndex(1));
	System.assertEquals('DUPLICATE_VALUE', e.getDmlStatusCode(1));
	System.assertEquals('Code__c', e.getDmlFields(1).get(0));
	System.assertEquals(2, e.getDmlIndex(2));
	System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', e.getDmlStatusCode(2));
	System.assertEquals('Name', e.getDmlFields(2).get(0));
}
System.assert(caught);
List<Account> rows = [SELECT Id FROM Account WHERE Code__c IN ('M', 'B')];
System.assertEquals(0, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
	account.Definition.Fields["Code__c"] = storage.Field{APIName: "Code__c", Type: storage.FieldString, Unique: true}
	account.Definition.ValidationRules = []storage.ValidationRule{{
		Name:                  "BlockBadName",
		Active:                true,
		ErrorConditionFormula: `Name = "Blocked"`,
		ErrorMessage:          "blocked by validation rule",
		ErrorDisplayField:     "Name",
	}}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLStatementValidationFailureThrowsDmlException(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	insert new Opportunity();
} catch (DmlException e) {
	caught = true;
	System.assert(e.getMessage().contains('REQUIRED_FIELD_MISSING'));
	System.assertEquals('REQUIRED_FIELD_MISSING', e.getDmlStatusCode(0));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLIgnoresQueriedChildRelationshipLists(t *testing.T) {
	program, err := CompileAnonymous(`
Opportunity opp = new Opportunity(Name = 'Original', StageName = 'Open', CloseDate = System.today());
insert opp;
Opportunity queried = [SELECT Id, Name, (SELECT Id FROM OpportunityLineItems) FROM Opportunity WHERE Id = :opp.Id];
queried.Name = 'Changed';
update queried;
Opportunity updated = [SELECT Id, Name FROM Opportunity WHERE Id = :opp.Id];
System.assertEquals('Changed', updated.Name);

Account acct = new Account(Name = 'Acme');
insert acct;
Contact contact = new Contact(AccountId = acct.Id, LastName = 'Original');
insert contact;
Contact queriedContact = [SELECT Id, LastName, Account.Name FROM Contact WHERE Id = :contact.Id];
System.assertEquals(acct.Id, queriedContact.Account.Id);
queriedContact.LastName = 'Changed';
update queriedContact;
Contact updatedContact = [SELECT Id, LastName FROM Contact WHERE Id = :contact.Id];
System.assertEquals('Changed', updatedContact.LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "Opportunity")
	storage.EnsureStandardObject(&org, "OpportunityLineItem")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLInvokesTriggersAndRollsBack(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.Name == 'Block') {
		throw new DmlException();
	}
	a.Name = a.Name + '!';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account ok = new Account(Name = 'Acme');
insert ok;
List<Account> rows = [SELECT Name FROM Account WHERE Id = :ok.Id];
Account row = rows.get(0);
System.assertEquals('Acme!', row.Name);
try {
	insert new Account(Name = 'Block');
} catch (DmlException e) {
	List<Account> survivors = [SELECT Id FROM Account];
	System.assertEquals(1, survivors.size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsert",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if !traceHas(result.Trace, "apex.trigger.AccountBeforeInsert", "apex.trigger") {
		t.Fatalf("trace missing trigger event: %#v", result.Trace)
	}
}

func TestExecTriggerAddErrorProducesDMLResults(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.Name == 'Block') {
		a.Name.addError('blocked by trigger');
		System.assert(a.hasErrors());
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account ok = new Account(Name = 'Acme');
Account blocked = new Account(Name = 'Block');
List<Account> records = new List<Account>{ok, blocked};
List<Object> results = Database.insert(records, false);
System.assertEquals(2, results.size());
Object first = results.get(0);
Object second = results.get(1);
System.assert(first.isSuccess());
System.assert(!second.isSuccess());
List<Object> errors = second.getErrors();
System.assertEquals(1, errors.size(), 'failed row should expose addError');
Object err = errors.get(0);
System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', err.getStatusCode());
System.assertEquals('blocked by trigger', err.getMessage());
List<Object> fields = err.getFields();
System.assertEquals(1, fields.size());
System.assertEquals('Name', fields.get(0));
List<Account> survivors = [SELECT Id FROM Account];
System.assertEquals(1, survivors.size(), 'partial insert should keep only unblocked row');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertAddError",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultipleAddErrorsProduceMultipleDatabaseErrors(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.Name == 'Multi Block') {
		a.addError('blocked at record');
		a.Name.addError('blocked at name');
		a.Rating.addError('blocked at rating');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account blocked = new Account(Name = 'Multi Block', Rating = 'Hot');
List<Object> results = Database.insert(new List<Account>{blocked}, false);
Object result = results.get(0);
System.assert(!result.isSuccess());
List<Object> errors = result.getErrors();
System.assertEquals(3, errors.size());
Object recordError = errors.get(0);
System.assertEquals('blocked at record', recordError.getMessage());
List<Object> recordFields = recordError.getFields();
System.assertEquals(0, recordFields.size());
Object nameError = errors.get(1);
System.assertEquals('blocked at name', nameError.getMessage());
List<Object> nameFields = nameError.getFields();
System.assertEquals(1, nameFields.size());
System.assertEquals('Name', nameFields.get(0));
Object ratingError = errors.get(2);
System.assertEquals('blocked at rating', ratingError.getMessage());
List<Object> ratingFields = ratingError.getFields();
System.assertEquals(1, ratingFields.size());
System.assertEquals('Rating', ratingFields.get(0));
List<Account> survivors = [SELECT Id FROM Account WHERE Name = 'Multi Block'];
System.assertEquals(0, survivors.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertMultiAddError",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAddErrorOverloadsAndUnsetField(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.Name == 'Overload Block') {
		a.addError('object overload', false);
		a.Rating.addError('unset field overload', true);
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account blocked = new Account(Name = 'Overload Block');
Object result = Database.insert(blocked, false);
System.assert(!result.isSuccess());
List<Object> errors = result.getErrors();
System.assertEquals(2, errors.size());
Object objectError = errors.get(0);
System.assertEquals('object overload', objectError.getMessage());
List<Object> objectFields = objectError.getFields();
System.assertEquals(0, objectFields.size());
Object fieldError = errors.get(1);
System.assertEquals('unset field overload', fieldError.getMessage());
List<Object> fieldFields = fieldError.getFields();
System.assertEquals(1, fieldFields.size());
System.assertEquals('Rating', fieldFields.get(0));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertAddErrorOverload",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPartialDeleteAfterTriggerSeesOnlySuccessfulRows(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account oldAccount : Trigger.old) {
	insert new Contact(LastName = 'after-delete-fired');
}
System.assertEquals(1, Trigger.size);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account blocked = new Account(Name = 'Blocked');
Account free = new Account(Name = 'Free');
insert new List<Account>{blocked, free};
insert new Contact(LastName = 'Child', AccountId = blocked.Id);
List<Object> results = Database.delete(new List<Account>{blocked, free}, false);
System.assertEquals(2, results.size());
Object first = results.get(0);
Object second = results.get(1);
System.assert(!first.isSuccess());
System.assert(second.isSuccess());
List<Contact> markers = [SELECT Id FROM Contact WHERE LastName = 'after-delete-fired'];
System.assertEquals(1, markers.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:            "AccountId",
				ParentObjects:    []string{"Account"},
				RestrictedDelete: true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterDelete",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "delete",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPartialDeleteAfterTriggerAlignsBeforeAndEngineFailures(t *testing.T) {
	beforeTrigger, err := CompileAnonymous(`
for (Account oldAccount : Trigger.old) {
	if (oldAccount.Name == 'Before Block') {
		oldAccount.addError('blocked before delete');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	afterTrigger, err := CompileAnonymous(`
System.assertEquals(1, Trigger.size);
Account oldAccount = Trigger.old.get(0);
System.assertEquals('Free', oldAccount.Name);
insert new Contact(LastName = 'after-delete-combined');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account beforeBlock = new Account(Name = 'Before Block');
Account free = new Account(Name = 'Free');
Account engineBlock = new Account(Name = 'Engine Block');
insert new List<Account>{beforeBlock, free, engineBlock};
insert new Contact(LastName = 'Child', AccountId = engineBlock.Id);
List<Object> results = Database.delete(new List<Account>{beforeBlock, free, engineBlock}, false);
System.assertEquals(3, results.size());
Object first = results.get(0);
Object second = results.get(1);
Object third = results.get(2);
System.assert(!first.isSuccess());
System.assert(second.isSuccess());
System.assert(!third.isSuccess());
List<Contact> markers = [SELECT Id FROM Contact WHERE LastName = 'after-delete-combined'];
System.assertEquals(1, markers.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:            "AccountId",
				ParentObjects:    []string{"Account"},
				RestrictedDelete: true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeDelete",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "delete",
		Program:   beforeTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterDeleteCombined",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "delete",
		Program:   afterTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTriggerContextMapsAndOperationFlags(t *testing.T) {
	updateTrigger, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isBefore);
System.assert(Trigger.isUpdate);
TriggerFlagProbe.checkBefore();
System.assertEquals(1, Trigger.size);
Account newer = Trigger.new.get(0);
Account older = Trigger.oldMap.get(newer.Id);
System.assertEquals('Before', older.Name);
System.assertEquals('After', newer.Name);
Account byMap = Trigger.newMap.get(newer.Id);
byMap.Rating = 'Warm';
newer.Name = 'After!';
`)
	if err != nil {
		t.Fatal(err)
	}
	deleteTrigger, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isBefore);
System.assert(Trigger.isDelete);
System.assertEquals(null, Trigger.new);
System.assertEquals(null, Trigger.newMap);
Account oldRow = Trigger.old.get(0);
System.assert(Trigger.oldMap.containsKey(oldRow.Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Before');
insert a;
a.Name = 'After';
update a;
Account updated = [SELECT Id, Name, Rating FROM Account WHERE Id = :a.Id];
System.assertEquals('After!', updated.Name);
System.assertEquals('Warm', updated.Rating);
delete updated;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	helper, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isBefore);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "TriggerFlagProbe",
		Methods: map[string]Method{
			"checkBefore": {Name: "TriggerFlagProbe.checkBefore", ReturnType: "void", Program: helper, IsStatic: true, ClassName: "TriggerFlagProbe"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeUpdateContext",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   updateTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeDeleteContext",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "delete",
		Program:   deleteTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUndeleteRunsOnlyAfterTrigger(t *testing.T) {
	beforeTrigger, err := CompileAnonymous(`
System.assert(false);
`)
	if err != nil {
		t.Fatal(err)
	}
	afterTrigger, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isAfter);
System.assert(Trigger.isUndelete);
System.assert(Trigger.isUnDelete);
System.assertEquals(1, Trigger.size);
System.assertEquals(null, Trigger.old);
System.assertEquals(null, Trigger.oldMap);
Account newer = Trigger.new.get(0);
System.assert(Trigger.newMap.containsKey(newer.Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Gone');
insert a;
delete a;
undelete a;
System.assert(!a.IsDeleted);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeUndelete",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "undelete",
		Program:   beforeTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterUndelete",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "undelete",
		Program:   afterTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpsertFiresInsertAndUpdateTriggers(t *testing.T) {
	beforeInsert, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	System.assert(Trigger.isInsert);
	System.assertEquals(1, a.Defaulted__c);
	if (a.External_Key__c == 'ext-2') {
		a.Rating = 'before-insert';
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	afterInsert, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	System.assert(Trigger.isAfter);
	System.assert(Trigger.isInsert);
	if (a.External_Key__c == 'ext-2') {
		insert new Contact(LastName = 'upsert-insert');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	beforeUpdate, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	System.assert(Trigger.isUpdate);
	Account oldAccount = Trigger.oldMap.get(a.Id);
	System.assertEquals('Existing', oldAccount.Name);
	System.assertEquals(1, a.Defaulted__c);
	a.Rating = 'before-update';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	afterUpdate, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	System.assert(Trigger.isAfter);
	System.assert(Trigger.isUpdate);
	insert new Contact(LastName = 'upsert-update');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account existing = new Account(Name = 'Existing', External_Key__c = 'ext-1');
insert existing;
Account inserted = new Account(Name = 'Inserted', External_Key__c = 'ext-2');
Account updated = new Account(Name = 'Updated', External_Key__c = 'EXT-1');
upsert new List<Account>{inserted, updated} External_Key__c;
Account insertedRow = [SELECT Id, Rating FROM Account WHERE External_Key__c = 'ext-2'];
System.assertEquals('before-insert', insertedRow.Rating);
Account updatedRow = [SELECT Id, Name, Rating FROM Account WHERE Id = :existing.Id];
System.assertEquals('Updated', updatedRow.Name);
System.assertEquals('before-update', updatedRow.Rating);
List<Contact> insertMarkers = [SELECT Id FROM Contact WHERE LastName = 'upsert-insert'];
System.assertEquals(1, insertMarkers.size());
List<Contact> updateMarkers = [SELECT Id FROM Contact WHERE LastName = 'upsert-update'];
System.assertEquals(1, updateMarkers.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	account.Definition.Fields["Defaulted__c"] = storage.Field{APIName: "Defaulted__c", Type: storage.FieldDecimal, Required: true, DefaultValue: "IF($RecordType.Name == 'Merchandise', 999, 1)"}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	for _, trigger := range []Trigger{
		{Name: "AccountBeforeInsertUpsert", Object: "Account", Timing: triggerTimingBefore, Operation: "insert", Program: beforeInsert},
		{Name: "AccountAfterInsertUpsert", Object: "Account", Timing: triggerTimingAfter, Operation: "insert", Program: afterInsert},
		{Name: "AccountBeforeUpdateUpsert", Object: "Account", Timing: triggerTimingBefore, Operation: "update", Program: beforeUpdate},
		{Name: "AccountAfterUpdateUpsert", Object: "Account", Timing: triggerTimingAfter, Operation: "update", Program: afterUpdate},
	} {
		if err := machine.RegisterTrigger(trigger); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestContextDefaultsCustomObjectTextName(t *testing.T) {
	program, err := CompileAnonymous(`
MembershipType__c membershipType = new MembershipType__c();
insert membershipType;
MembershipType__c stored = [SELECT Name FROM MembershipType__c WHERE Id = :membershipType.Id];
System.assertEquals('Membership Type', stored.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	org.Objects["MembershipType__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "MembershipType__c",
			Label:     "Membership Type",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTriggerBulkPartialSuccessKeepsRowAlignment(t *testing.T) {
	beforeInsert, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isBefore);
System.assert(Trigger.isInsert);
System.assert(trigger.isInsert);
System.assertEquals(3, Trigger.size);
System.assertEquals(null, Trigger.old);
System.assertEquals(null, Trigger.newMap);
for (Account a : Trigger.new) {
	if (a.Name == 'Block') {
		a.Name.addError('blocked by bulk trigger');
	} else {
		a.Rating = 'Bulk';
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	afterInsert, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isAfter);
System.assert(Trigger.isInsert);
System.assertEquals(2, Trigger.size);
Account firstNew = Trigger.new.get(0);
System.assert(Trigger.newMap.containsKey(firstNew.Id));
for (Account a : Trigger.new) {
	insert new Contact(LastName = 'after-' + a.Name);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account blocked = new Account(Name = 'Block');
Account first = new Account(Name = 'First');
Account second = new Account(Name = 'Second');
List<Account> records = new List<Account>{blocked, first, second};
List<Object> results = Database.insert(records, false);
System.assertEquals(3, results.size());
Object r0 = results.get(0);
Object r1 = results.get(1);
Object r2 = results.get(2);
System.assert(!r0.isSuccess());
System.assert(r1.isSuccess());
System.assert(r2.isSuccess());
System.assertEquals(null, blocked.get('Id'));
System.assert(first.get('Id') != null);
System.assert(second.get('Id') != null);
List<Account> rows = [SELECT Id, Name, Rating FROM Account ORDER BY Name];
System.assertEquals(2, rows.size());
Account row0 = rows.get(0);
Account row1 = rows.get(1);
System.assertEquals('First', row0.Name);
System.assertEquals('Bulk', row0.Rating);
System.assertEquals('Second', row1.Name);
System.assertEquals('Bulk', row1.Rating);
List<Contact> markers = [SELECT Id FROM Contact WHERE LastName IN ('after-First', 'after-Second')];
System.assertEquals(2, markers.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	for _, trigger := range []Trigger{
		{Name: "AccountBulkBeforeInsert", Object: "Account", Timing: triggerTimingBefore, Operation: "insert", Program: beforeInsert},
		{Name: "AccountBulkAfterInsert", Object: "Account", Timing: triggerTimingAfter, Operation: "insert", Program: afterInsert},
	} {
		if err := machine.RegisterTrigger(trigger); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTriggerRecursionLimit(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	insert new Account(Name = 'Recursive');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	insert new Account(Name = 'Root');
} catch (DmlException e) {
	caught = true;
	String message = e.getMessage();
	System.assert(message.contains('maximum trigger depth'));
}
System.assert(caught);
List<Account> rows = [SELECT Id FROM Account];
System.assertEquals(0, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountRecursiveBeforeInsert",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMergeInvokesUpdateAndDeleteTriggers(t *testing.T) {
	beforeUpdate, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isBefore);
System.assert(Trigger.isUpdate);
System.assertEquals(1, Trigger.size);
Account newer = Trigger.new.get(0);
Account older = Trigger.oldMap.get(newer.Id);
System.assertEquals('Merge Master', older.Name);
System.assertEquals('Merged Name', newer.Name);
newer.Rating = 'before-update';
`)
	if err != nil {
		t.Fatal(err)
	}
	afterUpdate, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isAfter);
System.assert(Trigger.isUpdate);
Account newer = Trigger.new.get(0);
Account older = Trigger.oldMap.get(newer.Id);
System.assertEquals('Merge Master', older.Name);
System.assertEquals('before-update', newer.Rating);
insert new Contact(LastName = 'merge-update-fired');
`)
	if err != nil {
		t.Fatal(err)
	}
	afterDelete, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isAfter);
System.assert(Trigger.isDelete);
System.assertEquals(null, Trigger.new);
Account oldRow = Trigger.old.get(0);
System.assertEquals('Merge Duplicate', oldRow.Name);
System.assert(Trigger.oldMap.containsKey(oldRow.Id));
insert new Contact(LastName = 'merge-delete-fired');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account master = new Account(Name = 'Merge Master');
insert master;
Account duplicate = new Account(Name = 'Merge Duplicate');
insert duplicate;
Account mergeMaster = new Account(Id = master.Id, Name = 'Merged Name');
Object merged = Database.merge(mergeMaster, duplicate, false);
System.assert(merged.isSuccess());
Account row = [SELECT Id, Name, Rating FROM Account WHERE Id = :master.Id];
System.assertEquals('Merged Name', row.Name);
System.assertEquals('before-update', row.Rating);
List<Contact> updateMarkers = [SELECT Id FROM Contact WHERE LastName = 'merge-update-fired'];
System.assertEquals(1, updateMarkers.size());
List<Contact> deleteMarkers = [SELECT Id FROM Contact WHERE LastName = 'merge-delete-fired'];
System.assertEquals(1, deleteMarkers.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	for _, trigger := range []Trigger{
		{Name: "AccountMergeBeforeUpdate", Object: "Account", Timing: triggerTimingBefore, Operation: "update", Program: beforeUpdate},
		{Name: "AccountMergeAfterUpdate", Object: "Account", Timing: triggerTimingAfter, Operation: "update", Program: afterUpdate},
		{Name: "AccountMergeAfterDelete", Object: "Account", Timing: triggerTimingAfter, Operation: "delete", Program: afterDelete},
	} {
		if err := machine.RegisterTrigger(trigger); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMergeHonorsBeforeDeleteAddError(t *testing.T) {
	beforeDelete, err := CompileAnonymous(`
for (Account oldRow : Trigger.old) {
	if (oldRow.get('Name') == 'Blocked Duplicate') {
		oldRow.addError('blocked duplicate merge');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account master = new Account(Name = 'Merge Master');
insert master;
Account duplicate = new Account(Name = 'Blocked Duplicate');
insert duplicate;
Account mergeMaster = new Account(Id = master.Id, Name = 'Should Not Apply');
Object merged = Database.merge(mergeMaster, duplicate, false);
System.assert(!merged.isSuccess());
List<Object> errors = merged.getErrors();
System.assertEquals(1, errors.size());
Object err = errors.get(0);
System.assertEquals('blocked duplicate merge', err.getMessage());
List<Account> duplicateRows = [SELECT Id FROM Account WHERE Id = :duplicate.Id];
System.assertEquals(1, duplicateRows.size());
Account masterRow = [SELECT Id, Name FROM Account WHERE Id = :master.Id];
System.assertEquals('Merge Master', masterRow.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountMergeBeforeDeleteAddError",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "delete",
		Program:   beforeDelete,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLExternalIDValidationAndUndelete(t *testing.T) {
	program, err := CompileAnonymous(`
Account first = new Account(Name = 'Acme', External_Key__c = 'ext-1', Code__c = 'A');
Object created = Database.upsert(first, false);
System.assert(created.isSuccess());
System.assert(created.isCreated());

Account second = new Account(External_Key__c = 'EXT-1', Name = 'Changed');
Object updated = Database.upsert(second, false);
System.assert(updated.isSuccess());
System.assert(!updated.isCreated());
System.assertEquals(created.getId(), updated.getId());

Account explicit = new Account(Other_Key__c = 'other-1', Name = 'Explicit');
Object explicitCreated = Database.upsert(explicit, Account.Other_Key__c, false);
System.assert(explicitCreated.isSuccess());
System.assert(explicitCreated.isCreated());
Account explicitUpdate = new Account(Other_Key__c = 'OTHER-1', Name = 'Explicit Changed');
Object explicitUpdated = Database.upsert(explicitUpdate, Account.Fields.Other_Key__c, false);
System.assert(explicitUpdated.isSuccess());
System.assert(!explicitUpdated.isCreated());
System.assertEquals(explicitCreated.getId(), explicitUpdated.getId());
Account statementCreate = new Account(Other_Key__c = 'other-2', Name = 'Statement');
upsert statementCreate Other_Key__c;
Account statementUpdate = new Account(Other_Key__c = 'OTHER-2', Name = 'Statement Changed');
upsert statementUpdate Other_Key__c;
System.assertEquals(statementCreate.Id, statementUpdate.Id);

List<Account> changed = [SELECT Id, Name FROM Account WHERE External_Key__c = 'EXT-1'];
System.assertEquals(1, changed.size(), 'external-id upsert should leave one active account');
Account changedRow = changed.get(0);
System.assertEquals('Changed', changedRow.Name);
List<Account> explicitChanged = [SELECT Id, Name FROM Account WHERE Other_Key__c = 'OTHER-1'];
System.assertEquals(1, explicitChanged.size(), 'explicit external-id upsert should update by requested field');
Account explicitRow = explicitChanged.get(0);
System.assertEquals('Explicit Changed', explicitRow.Name);
List<Account> statementChanged = [SELECT Id, Name FROM Account WHERE Other_Key__c = 'OTHER-2'];
System.assertEquals(1, statementChanged.size(), 'statement upsert should update by requested field');
Account statementRow = statementChanged.get(0);
System.assertEquals('Statement Changed', statementRow.Name);

Account duplicate = new Account(Name = 'Dup', Code__c = 'a');
Object duplicateResult = Database.insert(duplicate, false);
System.assert(!duplicateResult.isSuccess());
List<Object> duplicateErrors = duplicateResult.getErrors();
Object duplicateError = duplicateErrors.get(0);
System.assertEquals('DUPLICATE_VALUE', duplicateError.getStatusCode());

Contact bad = new Contact(LastName = 'Smith', AccountId = '001999999999999');
Object badResult = Database.insert(bad, false);
System.assert(!badResult.isSuccess());
List<Object> badErrors = badResult.getErrors();
Object badError = badErrors.get(0);
System.assertEquals('FIELD_INTEGRITY_EXCEPTION', badError.getStatusCode());

Contact good = new Contact(LastName = 'Jones', AccountId = created.getId());
insert good;
delete good;
List<Contact> deleted = [SELECT Id FROM Contact WHERE Id = :good.Id];
System.assertEquals(0, deleted.size());
Object undeleteResult = Database.undelete(good, false);
System.assert(undeleteResult.isSuccess());
List<Contact> restored = [SELECT Id FROM Contact WHERE Id = :good.Id];
System.assertEquals(1, restored.size(), 'undelete should restore query visibility');

Account mergeDuplicate = new Account(Name = 'Merge Duplicate');
insert mergeDuplicate;
Contact mergeChild = new Contact(LastName = 'Merge Child', AccountId = mergeDuplicate.Id);
insert mergeChild;
Account mergeMaster = new Account(Id = created.getId());
Object mergeResult = Database.merge(mergeMaster, mergeDuplicate, false);
System.assert(mergeResult.isSuccess());
System.assertEquals(created.getId(), mergeResult.getId());
String mergedMasterId = created.getId();
List<Object> mergedIds = mergeResult.getMergedRecordIds();
System.assertEquals(1, mergedIds.size());
List<Object> updatedRelatedIds = mergeResult.getUpdatedRelatedIds();
System.assertEquals(1, updatedRelatedIds.size());
System.assertEquals(mergeChild.Id, updatedRelatedIds.get(0));
List<Account> activeDuplicate = [SELECT Id FROM Account WHERE Id = :mergeDuplicate.Id];
System.assertEquals(0, activeDuplicate.size(), 'merge duplicate should be hidden from default SOQL');
List<Account> deletedDuplicate = [SELECT Id, IsDeleted FROM Account WHERE Id = :mergeDuplicate.Id ALL ROWS];
System.assertEquals(1, deletedDuplicate.size(), 'merge duplicate should remain available with ALL ROWS');
Account deletedDuplicateRow = deletedDuplicate.get(0);
System.assert(deletedDuplicateRow.IsDeleted);
List<Contact> reparented = [SELECT Id FROM Contact WHERE Id = :mergeChild.Id AND AccountId = :mergedMasterId];
System.assertEquals(1, reparented.size(), 'merge should reparent child lookups');
Account statementMergeDuplicate = new Account(Name = 'Statement Merge Duplicate');
insert statementMergeDuplicate;
Contact statementMergeChild = new Contact(LastName = 'Statement Merge Child', AccountId = statementMergeDuplicate.Id);
insert statementMergeChild;
Account statementMergeMaster = new Account(Id = created.getId());
merge statementMergeMaster statementMergeDuplicate;
List<Account> statementDeletedDuplicate = [SELECT Id FROM Account WHERE Id = :statementMergeDuplicate.Id ALL ROWS];
System.assertEquals(1, statementDeletedDuplicate.size(), 'merge statement should soft-delete duplicate');
List<Contact> statementReparented = [SELECT Id FROM Contact WHERE Id = :statementMergeChild.Id AND AccountId = :mergedMasterId];
System.assertEquals(1, statementReparented.size(), 'merge statement should reparent child lookups');

Account parentDelete = new Account(Id = created.getId());
delete parentDelete;
List<Contact> cascadeDeleted = [SELECT Id FROM Contact WHERE Id = :good.Id];
System.assertEquals(0, cascadeDeleted.size(), 'deleting parent should cascade soft-delete child records');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Other_Key__c"] = storage.Field{APIName: "Other_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Code__c"] = storage.Field{APIName: "Code__c", Type: storage.FieldString, Unique: true}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{
				Field:         "AccountId",
				ParentObjects: []string{"Account"},
				CascadeDelete: true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseMergeResultMergedRecordIdsForPartialList(t *testing.T) {
	program, err := CompileAnonymous(`
Account master = new Account(Name = 'Master');
insert master;
Account duplicate = new Account(Name = 'Duplicate');
insert duplicate;
Account deletedDuplicate = new Account(Name = 'Deleted Duplicate');
insert deletedDuplicate;
delete deletedDuplicate;
List<Account> duplicates = new List<Account>{duplicate, deletedDuplicate};
List<Object> results = Database.merge(master, duplicates, false);
System.assertEquals(2, results.size());
Object success = results.get(0);
Object failure = results.get(1);
System.assert(success.isSuccess());
List<Object> mergedIds = success.getMergedRecordIds();
System.assertEquals(1, mergedIds.size());
System.assertEquals(duplicate.Id, mergedIds.get(0));
System.assert(!failure.isSuccess());
System.assertEquals(0, failure.getMergedRecordIds().size());
System.assertEquals('ENTITY_IS_DELETED', failure.getErrors().get(0).getStatusCode());
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

func testDataOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":                                 {APIName: "Name", Type: storage.FieldString},
				"MasterRecordId":                       {APIName: "MasterRecordId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "MasterRecord"},
				"CopyFromPrimaryAffiliationBilling__c": {APIName: "CopyFromPrimaryAffiliationBilling__c", Type: storage.FieldBoolean, DefaultValue: "false"},
				"Total__c":                             {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "DECIMAL"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func TestExecCustomMetadataAndCustomSettingStatics(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Feature__mdt> allMetadata = Feature__mdt.getAll();
System.assertEquals(1, allMetadata.size());
Feature__mdt cfg = Feature__mdt.getInstance('pkg__Default');
System.assertEquals('Default', cfg.DeveloperName);
System.assert(cfg.pkg__Enabled__c);
Feature__mdt byID = pkg__Feature__mdt.getInstance(cfg.Id);
System.assertEquals('Default Label', byID.MasterLabel);
Map<String,Local_Setting__c> allSettings = Local_Setting__c.getAll();
System.assertEquals(1, allSettings.size());
Local_Setting__c setting = Local_Setting__c.getInstance('Default');
System.assertEquals('Default', setting.Name);
System.assert(!setting.pkg__Enabled__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLCustomMetadataRelationshipProjection(t *testing.T) {
	program, err := CompileAnonymous(`
List<Binding__mdt> rows = [SELECT DeveloperName, Target__r.QualifiedApiName FROM Binding__mdt WHERE Target__r.QualifiedApiName = 'Target'];
System.assertEquals(1, rows.size());
System.assertEquals('Default', rows[0].DeveloperName);
System.assertEquals('Target', rows[0].Target__r.QualifiedApiName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customMetadataRelationshipOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomDataStaticRecordsAreReadOnly(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"field assignment", "Feature__mdt cfg = Feature__mdt.getInstance('Default'); cfg.Enabled__c = false;", "cannot modify read-only custom metadata"},
		{"put", "Local_Setting__c setting = Local_Setting__c.getInstance('Default'); setting.put('Enabled__c', true);", "cannot modify read-only custom setting"},
		{"dml", "Feature__mdt cfg = Feature__mdt.getInstance('Default'); update cfg;", "DML cannot modify read-only custom metadata"},
		{"field assignment without org resolution", "Ghost__mdt cfg = new Ghost__mdt(); cfg.__oaer_readonly = 'custom metadata'; cfg.Enabled__c = false;", "cannot modify read-only custom metadata"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := customDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecHierarchyCustomSettingStaticsUseOrgDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(true, Hierarchy_Setting__c.getInstance().Enabled__c);
System.assertEquals(true, Hierarchy_Setting__c.getOrgDefaults().Enabled__c);
System.assertEquals(true, Hierarchy_Setting__c.getValues('00D000000000001').Enabled__c);
System.assertEquals(null, Hierarchy_Setting__c.getValues('005000000000001').Enabled__c);
System.assertEquals(false, Hierarchy_Setting__c.getValues('005000000000001').Defaulted__c);
System.assertEquals(true, Hierarchy_Setting__c.getInstance('005000000000001').Enabled__c);
System.assertEquals(true, Hierarchy_SETTING__c.getInstance().Enabled__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHierarchyCustomSettingGetAllUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`Hierarchy_Setting__c.getAll();`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), `unsupported call "Hierarchy_Setting__c.getAll hierarchy custom setting merge behavior"`) {
		t.Fatalf("error = %v, want hierarchy getAll unsupported", err)
	}
}

func TestExecHierarchyCustomSettingOrgDefaultsIgnoreUserRecords(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, Hierarchy_Setting__c.getOrgDefaults().Enabled__c);
System.assertEquals(false, Hierarchy_Setting__c.getOrgDefaults().Defaulted__c);
System.assertEquals(null, Hierarchy_Setting__c.getInstance().Enabled__c);
System.assertEquals(false, Hierarchy_Setting__c.getInstance().Defaulted__c);
System.assertEquals(null, Hierarchy_Setting__c.getInstance('a02000000000002').Enabled__c);
System.assertEquals(true, Hierarchy_Setting__c.getValues('005000000000001').Enabled__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	hierarchy := org.Objects["Hierarchy_Setting__c"]
	hierarchy.Records = map[storage.ID]storage.Record{
		"a02000000000002": {ID: "a02000000000002", Object: "Hierarchy_Setting__c", Fields: map[string]storage.Value{
			"SetupOwnerId": storage.StringValue("005000000000001"),
			"Enabled__c":   storage.BooleanValue(true),
		}},
	}
	org.Objects["Hierarchy_Setting__c"] = hierarchy
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCustomDataRecordSortingUsesIDTieBreaker(t *testing.T) {
	definition := storage.ObjectDefinition{
		APIName:  "Local_Setting__c",
		Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "List"},
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}
	records := map[storage.ID]storage.Record{
		"a01000000000002": {ID: "a01000000000002", Object: "Local_Setting__c"},
		"a01000000000001": {ID: "a01000000000001", Object: "Local_Setting__c"},
	}

	sorted := sortedCustomDataRecords(records, definition, "custom setting", "")
	if len(sorted) != 2 {
		t.Fatalf("len(sorted) = %d, want 2", len(sorted))
	}
	if sorted[0].ID != "a01000000000001" || sorted[1].ID != "a01000000000002" {
		t.Fatalf("sorted IDs = %q, %q; want ID order for equal keys", sorted[0].ID, sorted[1].ID)
	}
}

func customDataOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Feature__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Feature__mdt",
			KeyPrefix: "a00",
			Metadata:  map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"DeveloperName":    {APIName: "DeveloperName", Type: storage.FieldString},
				"MasterLabel":      {APIName: "MasterLabel", Type: storage.FieldString},
				"NamespacePrefix":  {APIName: "NamespacePrefix", Type: storage.FieldString},
				"QualifiedApiName": {APIName: "QualifiedApiName", Type: storage.FieldString},
				"Enabled__c":       {APIName: "Enabled__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Feature__mdt", Fields: map[string]storage.Value{
				"DeveloperName":    storage.StringValue("Default"),
				"MasterLabel":      storage.StringValue("Default Label"),
				"NamespacePrefix":  storage.StringValue("pkg"),
				"QualifiedApiName": storage.StringValue("pkg__Default"),
				"Enabled__c":       storage.BooleanValue(true),
			}},
		},
	}
	org.Objects["Hierarchy_Setting__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Hierarchy_Setting__c",
			KeyPrefix: "a02",
			Metadata:  map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
			Fields: map[string]storage.Field{
				"SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldString},
				"Enabled__c":   {APIName: "Enabled__c", Type: storage.FieldBoolean},
				"Defaulted__c": {APIName: "Defaulted__c", Type: storage.FieldBoolean, DefaultValue: "false"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a02000000000001": {ID: "a02000000000001", Object: "Hierarchy_Setting__c", Fields: map[string]storage.Value{
				"SetupOwnerId": storage.StringValue("00D000000000001"),
				"Enabled__c":   storage.BooleanValue(true),
			}},
		},
	}
	org.Objects["Local_Setting__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Local_Setting__c",
			KeyPrefix: "a01",
			Metadata:  map[string]string{"kind": "customSetting", "customSettingsType": "List"},
			Fields: map[string]storage.Field{
				"Name":       {APIName: "Name", Type: storage.FieldString},
				"Enabled__c": {APIName: "Enabled__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Local_Setting__c", Fields: map[string]storage.Value{
				"Name":       storage.StringValue("Default"),
				"Enabled__c": storage.BooleanValue(false),
			}},
		},
	}
	return org
}

func customMetadataRelationshipOrg() storage.OrgState {
	org := storage.NewOrgState()
	targetDefinition := storage.ObjectDefinition{
		APIName:   "Target__mdt",
		KeyPrefix: "a10",
		Metadata:  map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{
			"Name__c": {APIName: "Name__c", Type: storage.FieldString},
		},
	}
	storage.EnsureStandardObjectFields(&targetDefinition)
	bindingDefinition := storage.ObjectDefinition{
		APIName:   "Binding__mdt",
		KeyPrefix: "a11",
		Metadata:  map[string]string{"kind": "customMetadata"},
		Fields: map[string]storage.Field{
			"Target__c": {APIName: "Target__c", Type: storage.FieldReference, ReferenceTo: []string{"Target__mdt"}},
		},
	}
	storage.EnsureStandardObjectFields(&bindingDefinition)
	org.Objects["Target__mdt"] = storage.ObjectState{
		Definition: targetDefinition,
		Records: map[storage.ID]storage.Record{
			"a10000000000001": {ID: "a10000000000001", Object: "Target__mdt", Fields: map[string]storage.Value{
				"DeveloperName":    storage.StringValue("Target"),
				"QualifiedApiName": storage.StringValue("Target"),
			}},
		},
	}
	org.Objects["Binding__mdt"] = storage.ObjectState{
		Definition: bindingDefinition,
		Records: map[storage.ID]storage.Record{
			"a11000000000001": {ID: "a11000000000001", Object: "Binding__mdt", Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue("Default"),
				"Target__c":     storage.IDValue("a10000000000001"),
			}},
		},
	}
	return org
}
