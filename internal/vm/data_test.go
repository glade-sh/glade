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
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = :wanted];
System.assertEquals(1, rows.size());
Account row = rows.get(0);
System.assertEquals('Changed', row.Name);
row.Name = 'Updated';
update row;
List<Account> updated = Database.query('SELECT Id, Name FROM Account WHERE Name = ''Updated''');
System.assertEquals(1, updated.size());
Account updatedRow = updated.get(0);
delete updatedRow;
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
System.assertEquals('SObject', emptyRecords.getSObjectType().getDescribe().getName());
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
System.assertEquals('picklist', describe.getType());
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
Object nameField = fields.get('Name');
Object nameDescribe = nameField.getDescribe();
System.assertEquals('Name', nameDescribe.getName());
System.assertEquals('string', nameDescribe.getType());
Object schemaType = Schema.SObjectType.Account;
Object schemaDescribe = schemaType.getDescribe();
System.assertEquals('Account', schemaDescribe.getName());
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
System.assertEquals('reference', contactFieldDescribe.getType());
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

func TestExecDescribeUnsupportedMetadataEdges(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "field sets",
			source: `
Object accountDescribe = Account.SObjectType.getDescribe();
Object fieldSets = accountDescribe.fieldSets;
fieldSets.getMap();
`,
			want: `unsupported call "Schema.DescribeSObjectResult.fieldSets local field set metadata"`,
		},
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
System.assertEquals('Child', row.Account.Name);
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

func TestExecTriggerBulkPartialSuccessKeepsRowAlignment(t *testing.T) {
	beforeInsert, err := CompileAnonymous(`
System.assert(Trigger.isExecuting);
System.assert(Trigger.isBefore);
System.assert(Trigger.isInsert);
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
				"Name": {APIName: "Name", Type: storage.FieldString},
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

func TestExecHierarchyCustomSettingStaticsUnsupported(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"getInstance", "Hierarchy_Setting__c.getInstance();", `unsupported call "Hierarchy_Setting__c.getInstance hierarchy custom setting merge behavior"`},
		{"getAll", "Hierarchy_Setting__c.getAll();", `unsupported call "Hierarchy_Setting__c.getAll hierarchy custom setting merge behavior"`},
		{"getOrgDefaults", "Hierarchy_Setting__c.getOrgDefaults();", `unsupported call "Hierarchy_Setting__c.getOrgDefaults hierarchy custom setting merge behavior"`},
		{"getValues", "Hierarchy_Setting__c.getValues('005000000000001');", `unsupported call "Hierarchy_Setting__c.getValues hierarchy custom setting merge behavior"`},
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
