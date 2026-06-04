package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
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
System.assert(rows.size() == 1, 'target where rows ' + String.valueOf(rows.size()));
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
	machine.EnableTestContext()
	org := testDataOrg()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["ProductFrequencyLink__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ProductFrequencyLink__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Product__c": {APIName: "Product__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product__r", ChildRelationshipName: "ProductFrequencyLinks__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if relationshipType, ok := machine.jsonSObjectChildRelationshipType("Product__c", "ProductFrequencyLinks__r"); !ok || relationshipType != "List<ProductFrequencyLink__c>" {
		t.Fatalf("derived child relationship type = %q, ok=%v", relationshipType, ok)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeNamespacePrefixMatchesUnmanagedEmptyString(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = accountWithBusinessRecordType()
	storage.EnsureDeterministicPlatformData(&org)
	program, err := CompileAnonymous(`
RecordType rt = [SELECT Id, NamespacePrefix FROM RecordType WHERE DeveloperName = 'Business_Account' AND NamespacePrefix = ''];
System.assertEquals('', rt.NamespacePrefix);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestExecRecordTypeNamespacePrefixMatchesProjectNamespace(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Account"] = accountWithBusinessRecordType()
	storage.EnsureDeterministicPlatformData(&org)
	program, err := CompileAnonymous(`
RecordType rt = [SELECT Id, NamespacePrefix FROM RecordType WHERE DeveloperName = 'Business_Account' AND NamespacePrefix = 'pkg'];
System.assertEquals('pkg', rt.NamespacePrefix);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestExecSchemaPrefixedSObjectUsesOrgDefinition(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.EmailTemplate template = new Schema.EmailTemplate();
template.NamespacePrefix = 'pkg';
template.DeveloperName = 'Welcome';
System.assertEquals('pkg', template.NamespacePrefix);
System.assertEquals('Welcome', template.DeveloperName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "EmailTemplate")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecordTypeRowsAssignToSchemaRecordTypeList(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Account"] = accountWithBusinessRecordType()
	storage.EnsureDeterministicPlatformData(&org)
	program, err := CompileAnonymous(`
List<Schema.RecordType> recordTypes = [SELECT Id, Description FROM RecordType WHERE SObjectType = 'Account'];
System.assertEquals(1, recordTypes.size());
System.assertEquals('012000000000001', recordTypes[0].Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func accountWithBusinessRecordType() storage.ObjectState {
	return storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
			RecordTypes: []storage.RecordTypeInfo{{
				DeveloperName: "Business_Account",
				Name:          "Business Account",
				Active:        true,
				Available:     true,
				Default:       true,
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
}

func TestExecUserRecordAccessQuerySynthesizesAccessRows(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{
  new Account(Name = 'One'),
  new Account(Name = 'Two')
};
insert accounts;
Set<Id> ids = new Map<Id, Account>(accounts).keySet();
List<UserRecordAccess> accessRows = [
  SELECT RecordId, HasReadAccess, HasEditAccess, HasDeleteAccess
  FROM UserRecordAccess
  WHERE RecordId IN :ids AND UserId = :UserInfo.getUserId()
];
System.assertEquals(2, accessRows.size());
Map<Id, UserRecordAccess> accessByRecord = new Map<Id, UserRecordAccess>();
for (UserRecordAccess row : accessRows) {
  accessByRecord.put(row.RecordId, row);
}
for (Account account : accounts) {
  UserRecordAccess access = accessByRecord.get(account.Id);
  System.assertNotEquals(null, access);
  System.assertEquals(true, access.HasReadAccess);
  System.assertEquals(true, access.HasEditAccess);
  System.assertEquals(true, access.HasDeleteAccess);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "UserRecordAccess")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpdateCanClearQueriedSObjectFieldFromListElement(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme', External_Key__c = 'import-1');
insert a;
List<Account> rows = [SELECT Id, External_Key__c FROM Account WHERE Id = :a.Id];
rows[0].External_Key__c = null;
update rows;
Account updated = [SELECT Id, External_Key__c FROM Account WHERE Id = :a.Id LIMIT 1];
System.assertEquals(null, updated.External_Key__c);
System.assert(String.isBlank(updated.External_Key__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIntegerValueOfReturnsNullForSObjectNumericNulls(t *testing.T) {
	program, err := CompileAnonymous(`
Account missingField = new Account(Name = 'Acme');
Integer missingEmployees = Integer.valueOf(missingField.NumberOfEmployees);
System.assertEquals(null, missingEmployees);

Account safeMissing = null;
Integer safeEmployees = Integer.valueOf(safeMissing?.NumberOfEmployees);
System.assertEquals(null, safeEmployees);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIntegerValueOfReturnsNullForSafeNavigationCustomNumberNull(t *testing.T) {
	program, err := CompileAnonymous(`
Link__c link = null;
Integer term = Integer.valueOf(link?.TermOverrideMonths__c);
System.assertEquals(null, term);
Integer parentTerm = Integer.valueOf(link?.Parent__r.Term__c);
System.assertEquals(null, parentTerm);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Parent__c",
			Fields: map[string]storage.Field{
				"Term__c": {APIName: "Term__c", Type: storage.FieldDecimal},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Link__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Link__c",
			Fields: map[string]storage.Field{
				"TermOverrideMonths__c": {APIName: "TermOverrideMonths__c", Type: storage.FieldDecimal},
				"Parent__c":             {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Parent__c",
				ParentObjects:      []string{"Parent__c"},
				ParentRelationship: "Parent__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpdateUniqueFieldSwapInSameDMLStatement(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{
	new Account(Name = 'One', Code__c = 'one'),
	new Account(Name = 'Two', Code__c = 'two')
};
insert accounts;
accounts[0].Code__c = 'two';
accounts[1].Code__c = 'one';
update accounts;
Account first = [SELECT Code__c FROM Account WHERE Id = :accounts[0].Id LIMIT 1];
Account second = [SELECT Code__c FROM Account WHERE Id = :accounts[1].Id LIMIT 1];
System.assertEquals('two', first.Code__c);
System.assertEquals('one', second.Code__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Code__c"] = storage.Field{APIName: "Code__c", Type: storage.FieldString, Unique: true}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpdateCanClearNamespacedCustomFieldSetByPut(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
a.put('verifiable__External_Key__c', 'import-1');
insert a;
List<Account> rows = [SELECT Id, External_Key__c FROM Account WHERE Id = :a.Id];
rows[0].External_Key__c = null;
update rows;
Account updated = [SELECT Id, External_Key__c FROM Account WHERE Id = :a.Id LIMIT 1];
System.assertEquals(null, updated.External_Key__c);
System.assert(String.isBlank(updated.External_Key__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDynamicSOQLInBindWithoutSpaceAndNotNull(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Parent');
insert parent;
Account a = new Account(Name = 'Acme', External_Key__c = 'board-1');
a.ParentId = parent.Id;
insert a;
Set<String> externalIds = new Set<String>{ 'board-1' };
List<Account> rows = Database.query('SELECT Id, External_Key__c, ParentId FROM Account WHERE External_Key__c IN:externalIds AND ParentId != NULL');
System.assertEquals(1, rows.size());
System.assertEquals('board-1', rows[0].External_Key__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDynamicSOQLFindsStandardObjectFieldsUsedByDeleteRetry(t *testing.T) {
	program, err := CompileAnonymous(`
Contact c = new Contact(LastName = 'Hopkins', MobilePhone = 'provider-externalId');
insert c;
Asset board = new Asset(Name = '123456');
board.ExternalIdentifier = 'board-externalId';
board.ContactId = c.Id;
insert board;
Set<String> externalIds = new Set<String>{ 'board-externalId' };
List<SObject> rows = Database.query('SELECT Id, ExternalIdentifier, ContactId FROM Asset WHERE ExternalIdentifier IN:externalIds AND ContactId != NULL');
System.assertEquals(1, rows.size());
SObject row = rows[0];
System.assertEquals(board.Id, row.Id);
System.assertEquals(c.Id, row.get('ContactId'));
Map<Id, SObject> byId = new Map<Id, SObject>(rows);
System.assertEquals(1, byId.size());
System.assert(byId.containsKey(board.Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Asset"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Asset",
			KeyPrefix: "02i",
			Fields: map[string]storage.Field{
				"Name":               {APIName: "Name", Type: storage.FieldString},
				"ExternalIdentifier": {APIName: "ExternalIdentifier", Type: storage.FieldString},
				"ContactId":          {APIName: "ContactId", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Contact"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":    {APIName: "LastName", Type: storage.FieldString},
				"MobilePhone": {APIName: "MobilePhone", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseDeleteListOfIdsFallsBackToExistingRecordForUnknownPrefix(t *testing.T) {
	program, err := CompileAnonymous(`
Id detailId = 'zzz000000000001';
Database.delete(new List<Id>{detailId});
System.assertEquals(0, [SELECT Id FROM Detail__c].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Detail__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Detail__c",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"zzz000000000001": {
				ID:     "zzz000000000001",
				Object: "Detail__c",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("detail"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCloneRuntimeDoesNotShareJSONChildRelationshipTypeCache(t *testing.T) {
	parentOnly := New(nil)
	parentOnly.SetOrg(&storage.OrgState{
		Objects: map[string]storage.ObjectState{
			"Parent__c": {
				Definition: storage.ObjectDefinition{
					APIName: "Parent__c",
					Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
				},
			},
		},
	})
	if _, ok := parentOnly.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); ok {
		t.Fatalf("parent-only org unexpectedly resolved child relationship")
	}

	withChild := parentOnly.CloneRuntime(nil)
	withChild.SetOrg(&storage.OrgState{
		Objects: map[string]storage.ObjectState{
			"Parent__c": {
				Definition: storage.ObjectDefinition{
					APIName: "Parent__c",
					Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
				},
			},
			"Child__c": {
				Definition: storage.ObjectDefinition{
					APIName: "Child__c",
					Fields: map[string]storage.Field{
						"Parent__c": {
							APIName:               "Parent__c",
							Type:                  storage.FieldReference,
							ReferenceTo:           []string{"Parent__c"},
							RelationshipName:      "Parent__r",
							ChildRelationshipName: "Children__r",
						},
					},
				},
			},
		},
	})
	if got, ok := withChild.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
		t.Fatalf("clone child relationship type = %q, %v; want List<Child__c>, true", got, ok)
	}
}

func TestCloneRuntimeSharesChildRelationshipCachesForSameSchemaStamp(t *testing.T) {
	base := New(nil)
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}}
	base.SetOrg(&org)
	clone := base.CloneRuntime(nil)

	if clone.childRelCache == nil || clone.jsonChildRelTypeCache == nil {
		t.Fatalf("clone relationship caches were not initialized")
	}
	if clone.childRelCache != base.childRelCache {
		t.Fatalf("CloneRuntime did not share immutable child relationship cache")
	}
	if clone.jsonChildRelTypeCache != base.jsonChildRelTypeCache {
		t.Fatalf("CloneRuntime did not share immutable JSON child relationship cache")
	}
}

func TestCloneRuntimeForksChildRelationshipCachesWhenSchemaStampChanges(t *testing.T) {
	base := New(nil)
	first := storage.NewOrgState()
	first.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Label:   "First",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}}
	second := storage.NewOrgState()
	second.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Label:   "Second",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}}

	base.SetOrg(&first)
	clone := base.CloneRuntime(nil)
	sharedChildRelCache := clone.childRelCache
	sharedJSONChildRelTypeCache := clone.jsonChildRelTypeCache
	clone.SetOrg(&second)

	if clone.childRelCache == sharedChildRelCache {
		t.Fatalf("SetOrg kept shared child relationship cache after schema stamp changed")
	}
	if clone.jsonChildRelTypeCache == sharedJSONChildRelTypeCache {
		t.Fatalf("SetOrg kept shared JSON child relationship cache after schema stamp changed")
	}
}

func TestDescribeChildRelationshipCacheReturnsIsolatedValues(t *testing.T) {
	machine := New(nil)
	machine.SetOrg(&storage.OrgState{
		Objects: map[string]storage.ObjectState{
			"Parent__c": {
				Definition: storage.ObjectDefinition{
					APIName: "Parent__c",
					Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
				},
			},
			"Child__c": {
				Definition: storage.ObjectDefinition{
					APIName: "Child__c",
					Fields: map[string]storage.Field{
						"Parent__c": {
							APIName:               "Parent__c",
							Type:                  storage.FieldReference,
							ReferenceTo:           []string{"Parent__c"},
							RelationshipName:      "Parent__r",
							ChildRelationshipName: "Children__r",
						},
					},
					Relations: []storage.Relationship{{
						Field:              "Parent__c",
						ParentObjects:      []string{"Parent__c"},
						ParentRelationship: "Parent__r",
						ChildRelationship:  "Children__r",
					}},
				},
			},
		},
	})

	first := machine.describeChildRelationships("Parent__c")
	if len(first) != 1 {
		t.Fatalf("child relationships = %d, want 1", len(first))
	}
	first[0].Fields["childSObject"] = sObjectTypeToken("Wrong__c")

	second := machine.describeChildRelationships("Parent__c")
	if len(second) != 1 {
		t.Fatalf("cached child relationships = %d, want 1", len(second))
	}
	childType := second[0].Fields["childSObject"]
	childName := childType.Fields["object"]
	if childName.Text != "Child__c" {
		t.Fatalf("cached child relationship childSObject = %q, want Child__c", childName.Text)
	}
}

func TestSetOrgSameCountsClearsDescribeCachesWhenMetadataChanges(t *testing.T) {
	machine := New(nil)
	first := storage.NewOrgState()
	first.Objects["Thing__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Thing__c",
		Label:   "First Thing",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}}
	second := storage.NewOrgState()
	second.Objects["Thing__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Thing__c",
		Label:   "Second Thing",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		},
	}}

	machine.SetOrg(&first)
	name, definition, ok := machine.describeObjectDefinition("Thing__c")
	if !ok {
		t.Fatalf("first object definition not found")
	}
	firstDescribe := machine.describeSObjectValue(name, definition)
	if got := firstDescribe.Fields["label"].Text; got != "First Thing" {
		t.Fatalf("first label = %q, want First Thing", got)
	}

	machine.SetOrg(&second)
	name, definition, ok = machine.describeObjectDefinition("Thing__c")
	if !ok {
		t.Fatalf("second object definition not found")
	}
	secondDescribe := machine.describeSObjectValue(name, definition)
	if got := secondDescribe.Fields["label"].Text; got != "Second Thing" {
		t.Fatalf("second label = %q, want Second Thing", got)
	}
}

func TestSObjectFieldMapLookupAliasesUsesWarmCache(t *testing.T) {
	machine := New(nil)
	machine.SetOrg(&storage.OrgState{Namespace: "pkg", Objects: map[string]storage.ObjectState{}})
	if aliases := machine.sObjectFieldMapLookupAliases("pkg__Parent__c.Name"); len(aliases) == 0 {
		t.Fatalf("expected aliases")
	}

	allocs := testing.AllocsPerRun(1000, func() {
		_ = machine.sObjectFieldMapLookupAliases("pkg__Parent__c.Name")
	})
	if allocs > 0 {
		t.Fatalf("warm alias lookup allocated %.2f times per call, want 0", allocs)
	}
}

func TestExecSObjectMapKeySetKeepsSObjectKeys(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
Map<Account, String> byAccount = new Map<Account, String>();
byAccount.put(a, 'seen');
insert a;
List<String> ids = new List<String>{a.Id};
System.assertEquals(a.Id.to18(), ids[0]);
for (Account key : byAccount.keySet()) {
    System.assertEquals(a.Id, key.Id);
}
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
				"Name":      {APIName: "Name", Type: storage.FieldString},
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

func TestExecDatabaseQueryCanAssignSingleRowToSObject(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Single');
insert a;
Id accountId = a.Id;
SObject row = Database.query('SELECT Id, Name FROM Account WHERE Id = :accountId LIMIT 1');
System.assertEquals(a.Id, row.Id);
System.assertEquals('Single', row.get('Name'));
System.assertEquals(a, (Account)row);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "EmailMessage")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapConstructorAcceptsDynamicQueryResultRecords(t *testing.T) {
	program, err := CompileAnonymous(`
Account acme = new Account(Name = 'Acme');
insert acme;
Map<Id, Account> byId = new Map<Id, Account>(Database.query('SELECT Id, Name FROM Account WHERE Id = :acme.Id'));
System.assertEquals('Acme', byId.get(acme.Id).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "EmailMessage")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertEmailMessageCreatesToRelations(t *testing.T) {
	program, err := CompileAnonymous(`
EmailMessage message = new EmailMessage(
    Subject = 'Test Email',
    FromAddress = 'sender@example.invalid',
    ToAddress = 'system@example.invalid',
    toIds = new List<String>{ UserInfo.getUserId() },
    Incoming = true,
    Status = '3',
    IsClientManaged = true,
    MessageDate = DateTime.now()
);
Database.SaveResult result = Database.insert(message, false);
System.assert(result.isSuccess(), String.valueOf(result.getErrors()));
System.assertEquals(1, [SELECT COUNT() FROM EmailMessage]);
System.assertEquals(1, [SELECT COUNT() FROM EmailMessageRelation]);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "EmailMessage")
	storage.EnsureStandardObject(&org, "EmailMessageRelation")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTextFieldAssignedFromIdUsesStringCoercion(t *testing.T) {
	program, err := CompileAnonymous(`
Id jobId = '001000000000001';
Account a = new Account(Name = 'Acme', Job_Text__c = jobId);
insert a;
List<Account> rows = [SELECT Id, Job_Text__c FROM Account WHERE Job_Text__c = :jobId];
System.assertEquals(1, rows.size());
System.assertEquals(jobId.to18(), rows[0].Job_Text__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Job_Text__c"] = storage.Field{APIName: "Job_Text__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaSObjectTypeFieldMapPath(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
System.assert(fields.containsKey('Name'), 'Name key');
System.assert(fields.containsKey('billingaddress'), 'billingaddress key');
System.assert(fields.containsKey('BillingStreet'), 'BillingStreet key');
System.assert(fields.containsKey('MasterRecordId'), 'MasterRecordId key');
System.assertNotEquals(null, Account.SObjectType.fields.Id);
System.assertNotEquals(null, Account.SObjectType.fields.id);
System.assertEquals('Name', Account.SObjectType.fields.Name.Name);
System.assertEquals('Name', Schema.Account.Name.getDescribe().getName());
Schema.DescribeFieldResult billingStreet = fields.get('BillingStreet').getDescribe();
Schema.DescribeFieldResult billingCity = fields.get('BillingCity').getDescribe();
Schema.DescribeFieldResult billingAddress = fields.get('BillingAddress').getDescribe();
System.assertEquals('BillingAddress', billingStreet.getCompoundFieldName());
System.assertEquals('BillingAddress', billingCity.compoundFieldName);
System.assertEquals(null, billingAddress.getCompoundFieldName());
System.assertNotEquals(null, OpportunityLineItem.SObjectType);
Schema.SObjectField ownerField = fields.get('OwnerId');
System.assertEquals(Schema.SOAPType.ID, ownerField.getDescribe().getSOAPType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializedCustomSObjectGetByFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
OrderItem__c row = (OrderItem__c)JSON.deserialize('{"TransactionDate__c":"2026-05-02"}', OrderItem__c.class);
System.assertEquals(Date.newInstance(2026, 5, 2), row.TransactionDate__c);
System.assertEquals(Date.newInstance(2026, 5, 2), row.get(OrderItem__c.TransactionDate__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["OrderItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "OrderItem__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":                 {APIName: "Id", Type: storage.FieldID},
				"TransactionDate__c": {APIName: "TransactionDate__c", Type: storage.FieldCalculated, DisplayType: "DATE", Formula: "Order__r.TransactionDate__c"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypedNullSObjectPropertyFieldAccessReturnsNullValue(t *testing.T) {
	program, err := CompileAnonymous(`
Holder holder = new Holder();
System.assertEquals(null, holder.CartItem.Customer__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "CartItem__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id":          {APIName: "Id", Type: storage.FieldID},
				"Customer__c": {APIName: "Customer__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"CartItem": {Name: "CartItem", Type: "CartItem__c", Property: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaSObjectFieldMapKeySetReturnsCanonicalFieldNames(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
System.assertNotEquals(null, fields.get('account.billingaddress'));
System.assertNotEquals(null, fields.get('nu__BillingAddress'));
System.assert(fields.keySet().contains('billingaddress'));
System.assert(!fields.keySet().contains('BillingAddress'));
System.assert(fields.keySet().contains('nu__copyfromprimaryaffiliationbilling__c'));
System.assert(!fields.keySet().contains('copyfromprimaryaffiliationbilling__c'));
System.assert(!fields.keySet().contains('account.billingaddress'));
System.assert(!fields.keySet().contains('nu__BillingAddress'));
for (String fieldName : fields.keySet()) {
    if (fields.get(fieldName) == null) {
        System.assert(false, 'field map key did not round trip: ' + fieldName);
    }
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "nu"
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaAccountAddressFieldsExposeFirstComponent(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
for (String fieldName : fields.keySet()) {
    Schema.DescribeFieldResult describe = fields.get(fieldName).getDescribe();
    if (describe.getType() != Schema.DisplayType.Address) {
        continue;
    }
    String prefix = fieldName.removeEnd('address');
    String firstComponent = prefix + 'street';
    System.assertNotEquals(null, fields.get(firstComponent), fieldName + ' missing ' + firstComponent);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{DeveloperName: "PersonAccount", Name: "Person Account", Active: true}}
	org.Objects["Account"] = account
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestQueriedSObjectFieldsMarksLookupForRelationshipProjection(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
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

func TestQueriedSObjectFieldsMarksLookupForDerivedRelationshipProjection(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Children"},
				},
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
		t.Fatalf("derived relationship projection did not mark lookup field: %#v", fields)
	}
}

func TestQueriedSObjectFieldsTreatsObjectQualifiedFieldAsDirectField(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"OrderItem__c": {
			Definition: storage.ObjectDefinition{
				APIName: "OrderItem__c",
				Fields: map[string]storage.Field{
					"Id":               {APIName: "Id", Type: storage.FieldID},
					"TotalShipping__c": {APIName: "TotalShipping__c", Type: storage.FieldDecimal},
				},
			},
		},
	}}
	machine.SetOrg(&org)

	fields := machine.queriedSObjectFields("SELECT OrderItem__c.Id, OrderItem__c.TotalShipping__c FROM OrderItem__c")
	if !fields["totalshipping__c"] {
		t.Fatalf("object-qualified projection did not mark direct field: %#v", fields)
	}
	if fields["orderitem__c"] {
		t.Fatalf("object-qualified projection was treated as a relationship: %#v", fields)
	}
}

func TestUnqueriedFieldCheckResolvesNamespacedSelectedField(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{
		Namespace: "nu",
		Objects: map[string]storage.ObjectState{
			"OrderItem__c": {
				Definition: storage.ObjectDefinition{
					APIName: "OrderItem__c",
					Fields: map[string]storage.Field{
						"Id":               {APIName: "Id", Type: storage.FieldID},
						"TotalShipping__c": {APIName: "TotalShipping__c", Type: storage.FieldDecimal},
					},
				},
			},
		},
	}
	machine.SetOrg(&org)
	row := Object("OrderItem__c")
	row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("OrderItem__c", map[string]bool{
		"nu__totalshipping__c": true,
	})

	if err := machine.unqueriedSObjectFieldError(row, "TotalShipping__c", true); err != nil {
		t.Fatalf("unqualified access to selected namespaced field returned error: %v", err)
	}
}

func TestExplicitAliasObjectFieldValueResolvesNamespacedSelectedField(t *testing.T) {
	row := Object("OrderItem__c")
	row.Fields["nu__TotalShipping__c"] = Decimal(12.34)
	markExplicitSObjectField(&row, "nu__TotalShipping__c")

	field, value, ok := explicitAliasObjectFieldValue(row, "TotalShipping__c")
	if !ok {
		t.Fatalf("expected namespaced selected field alias to resolve")
	}
	if field != "nu__TotalShipping__c" {
		t.Fatalf("resolved field = %q, want nu__TotalShipping__c", field)
	}
	if value.Kind != ValueDecimal || value.Decimal != 12.34 {
		t.Fatalf("resolved value = %#v", value)
	}
}

func TestObjectFieldValuePrefersNamespacedAliasOverUnqualifiedNull(t *testing.T) {
	row := Object("pkg__Child__c")
	setExplicitSObjectField(&row, "Parent__c", Null)
	setExplicitSObjectField(&row, "pkg__Parent__c", platformScalar("Id", "a00000000000001AAA"))

	field, value, ok := objectFieldValue(row, "Parent__c")
	if !ok {
		t.Fatalf("expected namespaced field alias to resolve")
	}
	if field != "pkg__Parent__c" {
		t.Fatalf("resolved field = %q, want pkg__Parent__c", field)
	}
	if value.Kind != ValueObject || value.String() != "a00000000000001AAA" {
		t.Fatalf("resolved value = %#v", value)
	}
}

func TestRecordFromValueKeepsNonNullAliasOverGeneratedNull(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Line__c",
			Fields: map[string]storage.Field{
				"pkg__Parent__c": {APIName: "pkg__Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Parent__c"}, RelationshipName: "pkg__Parent__r"},
			},
		},
	}
	machine.SetOrg(&org)

	row := Object("pkg__Line__c")
	row.Fields["pkg__Parent__c"] = platformScalar("Id", "a00000000000001AAA")
	row.Fields["Parent__c"] = Null

	record, err := machine.recordFromValue(&row)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := record.Fields["pkg__Parent__c"]
	if !ok {
		t.Fatalf("record fields missing pkg__Parent__c: %#v explicitNulls=%#v", record.Fields, record.ExplicitNulls)
	}
	if value.Kind != storage.ValueID || value.ID != "a00000000000001AAA" {
		t.Fatalf("record parent = %#v, want id", value)
	}
	if record.ExplicitNulls["pkg__Parent__c"] {
		t.Fatalf("non-null alias was overwritten by generated null: explicitNulls=%#v", record.ExplicitNulls)
	}
}

func TestRecordFromValueKeepsNonUserSetNullAliasFromClearingNonNullAlias(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Line__c",
			Fields: map[string]storage.Field{
				"pkg__Parent__c": {APIName: "pkg__Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Parent__c"}, RelationshipName: "pkg__Parent__r"},
			},
		},
	}
	machine.SetOrg(&org)

	row := Object("pkg__Line__c")
	row.Fields["pkg__Parent__c"] = platformScalar("Id", "a00000000000001AAA")
	markExplicitSObjectField(&row, "pkg__Parent__c")
	row.Fields["Parent__c"] = Null
	markExplicitSObjectField(&row, "Parent__c")

	record, err := machine.recordFromValue(&row)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := record.Fields["pkg__Parent__c"]
	if !ok {
		t.Fatalf("record fields missing pkg__Parent__c: %#v explicitNulls=%#v", record.Fields, record.ExplicitNulls)
	}
	if value.Kind != storage.ValueID || value.ID != "a00000000000001AAA" {
		t.Fatalf("record parent = %#v, want id", value)
	}
}

func TestRecordFromValueExplicitLookupNullSuppressesRelationshipAliasID(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Line__c",
			Fields: map[string]storage.Field{
				"pkg__Parent__c": {APIName: "pkg__Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Parent__c"}, RelationshipName: "pkg__Parent__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "pkg__Parent__c",
				ParentObjects:      []string{"pkg__Parent__c"},
				ParentRelationship: "pkg__Parent__r",
			}},
		},
	}
	machine.SetOrg(&org)

	parent := Object("pkg__Parent__c")
	parent.Fields["Id"] = platformScalar("Id", "a00000000000001AAA")
	row := Object("pkg__Line__c")
	setExplicitSObjectField(&row, "Parent__c", Null)
	markUserSetSObjectField(&row, "Parent__c")
	row.Fields["Parent__r"] = parent

	record, err := machine.recordFromValue(&row)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := record.GetField("pkg__Parent__c"); ok {
		t.Fatalf("record parent = %#v, want explicit null only", value)
	}
	if !recordHasExplicitNullAlias(record, "pkg__Parent__c") {
		t.Fatalf("missing explicit null for parent lookup: %#v", record.ExplicitNulls)
	}
}

func TestPreserveMissingRecordFieldsKeepsUntouchedLookup(t *testing.T) {
	record := storage.Record{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Changed"),
		},
	}
	original := storage.Record{
		Object: "Child__c",
		Fields: map[string]storage.Value{
			"Name":      storage.StringValue("Original"),
			"Parent__c": storage.IDValue("a00000000000001AAA"),
		},
	}
	preserveMissingRecordFields(&record, original)
	if got := record.Fields["Name"]; got.Kind != storage.ValueString || got.String != "Changed" {
		t.Fatalf("changed field = %#v", got)
	}
	if got := record.Fields["Parent__c"]; got.Kind != storage.ValueID || got.ID != "a00000000000001AAA" {
		t.Fatalf("preserved lookup = %#v", got)
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
	raw := map[string]any{
		"Parent__r":      map[string]any{"Id": "001000000000001AAA", "Name": "First"},
		"pkg__Parent__r": map[string]any{"Id": "001000000000002AAA", "Name": "Second"},
	}
	decoded, err := machine.typedValueFromJSON("Child__c", raw, false)
	if err != nil {
		t.Fatal(err)
	}
	_, parent, ok := objectFieldValue(decoded, "Parent__r")
	if !ok || parent.Kind != ValueObject {
		t.Fatalf("Parent__r = %#v, ok=%v", parent, ok)
	}
	if got := parent.Fields["Name"]; got.Kind != ValueString || got.Text != "Second" {
		t.Fatalf("Parent__r.Name = %#v, want Second", got)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRelationshipProjectionWithNullLookupKeepsRelationshipNull(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> rows = Database.query('SELECT Id, Parent__r.Id, Parent__r.Name FROM Child__c');
System.assertEquals(1, rows.size());
Child__c child = (Child__c)rows[0];
System.assertEquals(null, child.Parent__r);
System.assertEquals(null, child.Parent__r.Name);
Integer touched = 0;
if (child.Parent__r != null) {
	touched++;
}
System.assertEquals(0, touched);
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
					Fields: map[string]storage.Value{},
				},
			},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	queried := Object("Child__c")
	queried.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Child__c", map[string]bool{"parent__r": true})
	parent := Object("Account")
	parent.Fields["Id"] = String("001000000000001AAA")
	setExplicitSObjectField(&queried, "Parent__r", parent)
	markQueriedSObjectField(&queried, "Parent__r")
	value, err := machine.lookupPath(queried, []string{"Parent__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("explicit parent relationship backfilled lookup: %#v", value)
	}
}

func TestExecNestedRelationshipProjectionWithExplicitNullMiddleLookupUsesTypedNull(t *testing.T) {
	program, err := CompileAnonymous(`
List<dep__Batch__c> rows = [SELECT dep__Entity__c, dep__Entity__r.dep__BatchExportConfiguration__r.pkg__GLSettingName__c FROM dep__Batch__c];
System.assertEquals(1, rows.size());
String metadataRecordName = rows[0].dep__Entity__r.dep__BatchExportConfiguration__r.pkg__GLSettingName__c;
System.assertEquals(null, metadataRecordName);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{
		Namespace: "pkg",
		Objects: map[string]storage.ObjectState{
			"dep__BatchExportConfiguration__c": {
				Definition: storage.ObjectDefinition{
					APIName: "dep__BatchExportConfiguration__c",
					Fields: map[string]storage.Field{
						"pkg__GLSettingName__c": {APIName: "pkg__GLSettingName__c", Type: storage.FieldString},
					},
				},
				Records: map[storage.ID]storage.Record{},
			},
			"dep__Entity__c": {
				Definition: storage.ObjectDefinition{
					APIName: "dep__Entity__c",
					Fields: map[string]storage.Field{
						"dep__BatchExportConfiguration__c": {APIName: "dep__BatchExportConfiguration__c", Type: storage.FieldReference, ReferenceTo: []string{"dep__BatchExportConfiguration__c"}, RelationshipName: "dep__BatchExportConfiguration__r"},
					},
					Relations: []storage.Relationship{{
						Field:              "dep__BatchExportConfiguration__c",
						ParentObjects:      []string{"dep__BatchExportConfiguration__c"},
						ParentRelationship: "dep__BatchExportConfiguration__r",
					}},
				},
				Records: map[storage.ID]storage.Record{
					"aNq000000000001AAA": {
						Object: "dep__Entity__c",
						ID:     "aNq000000000001AAA",
						Fields: map[string]storage.Value{},
						ExplicitNulls: map[string]bool{
							"dep__BatchExportConfiguration__c": true,
						},
					},
				},
			},
			"dep__Batch__c": {
				Definition: storage.ObjectDefinition{
					APIName: "dep__Batch__c",
					Fields: map[string]storage.Field{
						"dep__Entity__c": {APIName: "dep__Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"dep__Entity__c"}, RelationshipName: "dep__Entity__r"},
					},
					Relations: []storage.Relationship{{
						Field:              "dep__Entity__c",
						ParentObjects:      []string{"dep__Entity__c"},
						ParentRelationship: "dep__Entity__r",
					}},
				},
				Records: map[storage.ID]storage.Record{
					"aN900000000001AAA": {
						Object: "dep__Batch__c",
						ID:     "aN900000000001AAA",
						Fields: map[string]storage.Value{
							"dep__Entity__c": storage.IDValue("aNq000000000001AAA"),
						},
					},
				},
			},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecParentRelationshipMutationThroughMethodPersistsWhenRelationshipReadAgain(t *testing.T) {
	mutatorProgram, err := CompileAnonymous(`
if (Mutator.hasValueChanged('Eddy', profile.FirstName__c)) {
	profile.FirstName__c = 'Eddy';
}
if (Mutator.hasValueChanged('Edwardson', profile.LastName__c)) {
	profile.LastName__c = 'Edwardson';
}
if (Mutator.hasValueChanged('5555', profile.AccountNumber__c)) {
	profile.AccountNumber__c = '5555';
}
if (Mutator.hasValueChanged('06', profile.ExpirationMonth__c)) {
	profile.ExpirationMonth__c = '06';
}
if (Mutator.hasValueChanged('2030', profile.ExpirationYear__c)) {
	profile.ExpirationYear__c = '2030';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	hasValueChangedProgram, err := CompileAnonymous(`
return String.isNotBlank(newValue) && !newValue.equalsIgnoreCase(oldValue);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Payment__c built = new Payment__c();
built.put(Payment__c.ExternalPaymentProfile__c, 'a10000000000001AAA');
insert built;
Payment__c payment = [
	SELECT Id,
		ExternalPaymentProfile__c,
		ExternalPaymentProfile__r.FirstName__c,
		ExternalPaymentProfile__r.LastName__c,
		ExternalPaymentProfile__r.AccountNumber__c,
		ExternalPaymentProfile__r.ExpirationMonth__c,
		ExternalPaymentProfile__r.ExpirationYear__c
	FROM Payment__c
	WHERE Id = :built.Id
	LIMIT 1
];
System.assertEquals('a10000000000001AAA', payment.ExternalPaymentProfile__c);
Mutator.rename(payment.ExternalPaymentProfile__r);
List<SObject> touched = new List<SObject>();
touched.add(payment);
System.assertEquals(false, touched.contains(payment.ExternalPaymentProfile__r));
payment.SettlementDate__c = Date.today();
List<SObject> reconciled = new List<SObject>();
reconciled.add(payment);
reconciled.add(payment.ExternalPaymentProfile__r);
Map<String, List<SObject>> updatesByType = new Map<String, List<SObject>>();
List<String> updateOrder = new List<String>();
for (SObject record : reconciled) {
	String sObjectType = record.getSObjectType().getDescribe().getName();
	List<SObject> records = updatesByType.get(sObjectType);
	if (records == null) {
		records = new List<SObject>();
		updatesByType.put(sObjectType, records);
		updateOrder.add(sObjectType);
	}
	records.add(record);
}
for (String sObjectType : updateOrder) {
	update updatesByType.get(sObjectType);
}
ExternalPaymentProfile__c profile = [SELECT FirstName__c FROM ExternalPaymentProfile__c LIMIT 1];
System.assertEquals('Eddy', profile.FirstName__c);
profile = [SELECT FirstName__c, LastName__c, AccountNumber__c, ExpirationMonth__c, ExpirationYear__c FROM ExternalPaymentProfile__c LIMIT 1];
System.assertEquals('Edwardson', profile.LastName__c);
System.assertEquals('5555', profile.AccountNumber__c);
System.assertEquals('06', profile.ExpirationMonth__c);
System.assertEquals('2030', profile.ExpirationYear__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"ExternalPaymentProfile__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "ExternalPaymentProfile__c",
				KeyPrefix: "a10",
				Fields: map[string]storage.Field{
					"Id":                 {APIName: "Id", Type: storage.FieldID},
					"FirstName__c":       {APIName: "FirstName__c", Type: storage.FieldString},
					"LastName__c":        {APIName: "LastName__c", Type: storage.FieldString},
					"AccountNumber__c":   {APIName: "AccountNumber__c", Type: storage.FieldString},
					"ExpirationMonth__c": {APIName: "ExpirationMonth__c", Type: storage.FieldString},
					"ExpirationYear__c":  {APIName: "ExpirationYear__c", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{
				"a10000000000001AAA": {
					Object: "ExternalPaymentProfile__c",
					ID:     "a10000000000001AAA",
					Fields: map[string]storage.Value{
						"NU__FirstName__c":       storage.StringValue("Eric"),
						"NU__LastName__c":        storage.StringValue("Clapton"),
						"NU__AccountNumber__c":   storage.StringValue("4444"),
						"NU__ExpirationMonth__c": storage.StringValue("07"),
						"NU__ExpirationYear__c":  storage.StringValue("2031"),
					},
				},
			},
		},
		"Payment__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Payment__c",
				KeyPrefix: "a20",
				Fields: map[string]storage.Field{
					"Id":                {APIName: "Id", Type: storage.FieldID},
					"SettlementDate__c": {APIName: "SettlementDate__c", Type: storage.FieldDate},
					"ExternalPaymentProfile__c": {
						APIName:          "ExternalPaymentProfile__c",
						Type:             storage.FieldReference,
						ReferenceTo:      []string{"ExternalPaymentProfile__c"},
						RelationshipName: "ExternalPaymentProfile__r",
					},
				},
				Relations: []storage.Relationship{{
					Field:              "ExternalPaymentProfile__c",
					ParentObjects:      []string{"ExternalPaymentProfile__c"},
					ParentRelationship: "ExternalPaymentProfile__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{
		Name:       "Mutator.rename",
		ClassName:  "Mutator",
		ReturnType: "void",
		Params:     []Param{{Name: "profile", Type: "ExternalPaymentProfile__c"}},
		Program:    mutatorProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Mutator.hasValueChanged",
		ClassName:  "Mutator",
		ReturnType: "Boolean",
		Params: []Param{
			{Name: "newValue", Type: "String"},
			{Name: "oldValue", Type: "String"},
		},
		Program: hasValueChangedProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLoadedRelationshipAllowsMissingLookupRepair(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName:   "Account",
				KeyPrefix: "001",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
			Records: map[storage.ID]storage.Record{
				"001000000000001AAA": {Object: "Account", ID: "001000000000001AAA"},
			},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Children"},
				},
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
	child := Object("Child__c")
	child.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Child__c", map[string]bool{"id": true})
	child.Fields["Parent__r"] = Object("Account")
	if err := machine.unqueriedSObjectFieldError(child, "Parent__c", true); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingLookupFieldUsesLoadedRelationshipID(t *testing.T) {
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
	child := Object("Child__c")
	child.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Child__c", map[string]bool{"parent__c": true, "parent__r": true})
	parent := Object("Account")
	parent.Fields["Id"] = String("001000000000001AAA")
	child.Fields["Parent__r"] = parent
	value, ok := machine.missingSObjectFieldValue(child, "Parent__c")
	if !ok || value.Kind != ValueString || value.Text != "001000000000001AAA" {
		t.Fatalf("missing lookup did not use loaded relationship id: %#v ok=%v", value, ok)
	}
	child.Fields["Parent__c"] = Null
	value, err := machine.lookupPath(child, []string{"Parent__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueString || value.Text != "001000000000001AAA" {
		t.Fatalf("null lookup did not use loaded relationship id: %#v", value)
	}
}

func TestExecAssignedParentRelationshipDoesNotPopulateLookupField(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Id = '001000000000001AAA');
Child__c child = new Child__c();
child.Parent__r = parent;
System.assertEquals(parent, child.Parent__r);
System.assertEquals(null, child.Parent__c);
`)
	if err != nil {
		t.Fatal(err)
	}
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
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignedParentRelationshipUsesLookupDerivedAliasWhenChildRelationshipDiffers(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Id = '001000000000001AAA');
Child__c child = new Child__c();
child.Parent__r = parent;
System.assertEquals(parent, child.Parent__r);
System.assertEquals(null, child.Parent__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "ChildRelationshipName"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "ChildRelationshipName",
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
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignedNamespacedParentRelationshipDoesNotPopulateLookupField(t *testing.T) {
	program, err := CompileAnonymous(`
DeferredSchedule__c deferredSchedule = new DeferredSchedule__c(Id = 'a010000000000001AAA');
Transaction__c transactionRecord = new Transaction__c();
transactionRecord.DeferredSchedule__r = deferredSchedule;
System.assertEquals(deferredSchedule, transactionRecord.DeferredSchedule__r);
System.assertEquals(null, transactionRecord.DeferredSchedule__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"Transaction__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Transaction__c",
				Fields: map[string]storage.Field{
					"Id":                  {APIName: "Id", Type: storage.FieldID},
					"DeferredSchedule__c": {APIName: "DeferredSchedule__c", Type: storage.FieldReference, ReferenceTo: []string{"DeferredSchedule__c"}, RelationshipName: "DeferredRevenueTransactions"},
				},
				Relations: []storage.Relationship{{
					Field:              "DeferredSchedule__c",
					ParentObjects:      []string{"DeferredSchedule__c"},
					ParentRelationship: "DeferredRevenueTransactions",
				}},
			},
		},
		"DeferredSchedule__c": {
			Definition: storage.ObjectDefinition{
				APIName: "DeferredSchedule__c",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestLookupExplicitParentRelationshipDoesNotBackfillLookupWhenChildRelationshipDiffers(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"Transaction__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Transaction__c",
				Fields: map[string]storage.Field{
					"Id":                  {APIName: "Id", Type: storage.FieldID},
					"DeferredSchedule__c": {APIName: "DeferredSchedule__c", Type: storage.FieldReference, ReferenceTo: []string{"DeferredSchedule__c"}, RelationshipName: "DeferredRevenueTransactions"},
				},
				Relations: []storage.Relationship{{
					Field:              "DeferredSchedule__c",
					ParentObjects:      []string{"DeferredSchedule__c"},
					ParentRelationship: "DeferredRevenueTransactions",
				}},
			},
		},
		"DeferredSchedule__c": {
			Definition: storage.ObjectDefinition{
				APIName: "DeferredSchedule__c",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
	}}
	machine.SetOrg(&org)
	transaction := Object("Transaction__c")
	transaction.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Transaction__c", map[string]bool{
		"deferredschedule__c": true,
		"deferredschedule__r": true,
	})
	deferredSchedule := Object("DeferredSchedule__c")
	deferredSchedule.Fields["Id"] = String("a010000000000001AAA")
	setExplicitSObjectField(&transaction, "DeferredSchedule__r", deferredSchedule)
	value, err := machine.lookupPath(transaction, []string{"DeferredSchedule__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("explicit parent relationship backfilled lookup: %#v", value)
	}
	value, err = machine.lookupPath(transaction, []string{"NU__DeferredSchedule__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("explicit unqualified parent relationship backfilled namespaced lookup: %#v", value)
	}
}

func TestAssignReferenceFieldWithSObjectStoresParentRelationshipSlot(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"Transaction__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Transaction__c",
				Fields: map[string]storage.Field{
					"Id":                  {APIName: "Id", Type: storage.FieldID},
					"DeferredSchedule__c": {APIName: "DeferredSchedule__c", Type: storage.FieldReference, ReferenceTo: []string{"DeferredSchedule__c"}, RelationshipName: "DeferredRevenueTransactions"},
				},
			},
		},
		"DeferredSchedule__c": {
			Definition: storage.ObjectDefinition{
				APIName: "DeferredSchedule__c",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
	}}
	machine.SetOrg(&org)
	transaction := Object("Transaction__c")
	deferredSchedule := Object("DeferredSchedule__c")
	deferredSchedule.Fields["Id"] = String("a010000000000001AAA")
	if err := machine.assignPath(transaction, []string{"DeferredSchedule__c"}, deferredSchedule); err != nil {
		t.Fatal(err)
	}
	lookup, err := machine.lookupPath(transaction, []string{"DeferredSchedule__c"})
	if err != nil {
		t.Fatal(err)
	}
	if lookup.Kind != ValueNull {
		t.Fatalf("relationship assignment populated lookup: %#v", lookup)
	}
	relationship, err := machine.lookupPath(transaction, []string{"DeferredSchedule__r"})
	if err != nil {
		t.Fatal(err)
	}
	if relationship.Kind != ValueObject || relationship.Type != "DeferredSchedule__c" {
		t.Fatalf("relationship slot = %#v", relationship)
	}
}

func TestLookupExplicitNullParentRelationshipWithLookupIDUsesTypedShellForNestedField(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"CartItemLine__c": {
			Definition: storage.ObjectDefinition{
				APIName: "CartItemLine__c",
				Fields: map[string]storage.Field{
					"Id":          {APIName: "Id", Type: storage.FieldID},
					"Product2__c": {APIName: "Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product2__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Product2__c",
					ParentObjects:      []string{"Product__c"},
					ParentRelationship: "Product2__r",
				}},
			},
		},
		"Product__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Product__c",
				Fields: map[string]storage.Field{
					"Id":                   {APIName: "Id", Type: storage.FieldID},
					"RecurringEligible__c": {APIName: "RecurringEligible__c", Type: storage.FieldBoolean, DefaultValue: "false"},
				},
			},
		},
	}}
	machine.SetOrg(&org)
	line := Object("CartItemLine__c")
	line.Fields["Product2__c"] = String("aHA000000000006")
	setExplicitSObjectField(&line, "Product2__r", Null)
	relationship, err := machine.lookupPath(line, []string{"Product2__r"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := machine.lookupPath(relationship, []string{"RecurringEligible__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueBool || value.Bool {
		t.Fatalf("nested checkbox through explicit null relationship = %#v, want false", value)
	}
}

func TestLookupParentRelationshipReturnsReferencedSObjectType(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "pkg", Objects: map[string]storage.ObjectState{
		"pkg__Line__c": {
			Definition: storage.ObjectDefinition{
				APIName: "pkg__Line__c",
				Fields: map[string]storage.Field{
					"Id":            {APIName: "Id", Type: storage.FieldID},
					"pkg__Event__c": {APIName: "pkg__Event__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Event__c"}, RelationshipName: "pkg__Event__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "pkg__Event__c",
					ParentObjects:      []string{"pkg__Event__c"},
					ParentRelationship: "pkg__Event__r",
				}},
			},
		},
		"pkg__Event__c": {
			Definition: storage.ObjectDefinition{
				APIName: "pkg__Event__c",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
	}}
	machine.SetOrg(&org)
	line := Object("pkg__Line__c")
	line.Fields["pkg__Event__r"] = Object("pkg__Event__r")

	relationship, err := machine.lookupPath(line, []string{"pkg__Event__r"})
	if err != nil {
		t.Fatal(err)
	}
	if relationship.Kind != ValueObject || relationship.Type != "pkg__Event__c" {
		t.Fatalf("relationship = %#v, want pkg__Event__c object", relationship)
	}
	if _, err := machine.coerceAssignable("pkg__Event__c", relationship); err != nil {
		t.Fatalf("relationship did not coerce to parent object type: %v", err)
	}
}

func TestLookupParentRelationshipWithOnlyNullProjectedFieldsReturnsNull(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "pkg", Objects: map[string]storage.ObjectState{
		"pkg__Line__c": {
			Definition: storage.ObjectDefinition{
				APIName: "pkg__Line__c",
				Fields: map[string]storage.Field{
					"Id":            {APIName: "Id", Type: storage.FieldID},
					"pkg__Event__c": {APIName: "pkg__Event__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Event__c"}, RelationshipName: "pkg__Event__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "pkg__Event__c",
					ParentObjects:      []string{"pkg__Event__c"},
					ParentRelationship: "pkg__Event__r",
				}},
			},
		},
		"pkg__Event__c": {
			Definition: storage.ObjectDefinition{
				APIName: "pkg__Event__c",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "pkg__StartDate__c": {APIName: "pkg__StartDate__c", Type: storage.FieldDateTime}},
			},
		},
	}}
	machine.SetOrg(&org)
	event := Object("pkg__Event__c")
	event.Fields["pkg__StartDate__c"] = Null
	line := Object("pkg__Line__c")
	line.Fields["pkg__Event__r"] = event

	relationship, err := machine.lookupPath(line, []string{"pkg__Event__r"})
	if err != nil {
		t.Fatal(err)
	}
	if relationship.Kind != ValueNull || relationship.Type != "pkg__Event__c" || relationship.Runtime != relationshipNullRuntime {
		t.Fatalf("relationship = %#v, want typed null", relationship)
	}
}

func TestVMValueFromRecordCollapsesMissingParentRelationshipProjection(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "pkg", Objects: map[string]storage.ObjectState{
		"pkg__Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "pkg__Child__c",
				Fields: map[string]storage.Field{
					"Id":             {APIName: "Id", Type: storage.FieldID},
					"pkg__Parent__c": {APIName: "pkg__Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Parent__c"}, RelationshipName: "pkg__Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "pkg__Parent__c",
					ParentObjects:      []string{"pkg__Parent__c"},
					ParentRelationship: "pkg__Parent__r",
				}},
			},
		},
		"pkg__Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName: "pkg__Parent__c",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "pkg__Name__c": {APIName: "pkg__Name__c", Type: storage.FieldString}},
			},
		},
	}}
	machine.SetOrg(&org)

	value := machine.vmValueFromRecord(storage.Record{
		ID:     "a01000000000001",
		Object: "pkg__Child__c",
		Fields: map[string]storage.Value{
			"pkg__Parent__r.pkg__Name__c": storage.NullValue(),
		},
	})

	_, relationship, ok := objectFieldValue(value, "pkg__Parent__r")
	if !ok {
		t.Fatalf("missing relationship field: %#v", value.Fields)
	}
	if relationship.Kind != ValueNull || relationship.Type != "pkg__Parent__c" || relationship.Runtime != relationshipNullRuntime {
		t.Fatalf("relationship = %#v, want typed null", relationship)
	}
}

func TestGenericFindSObjectInListByIdStaticHelper(t *testing.T) {
	first := Object("Account")
	first.Fields["Id"] = platformScalar("Id", "001000000000001")
	second := Object("Account")
	second.Fields["Id"] = platformScalar("Id", "001000000000002")
	records := List(first, second)

	found := findSObjectInListByID(platformScalar("Id", "001000000000002"), records)
	if found.Kind != ValueObject {
		t.Fatalf("found = %#v, want object", found)
	}
	if got := sObjectIDFromFields(found.Fields); got != "001000000000002" {
		t.Fatalf("found Id = %q", got)
	}
	missing := findSObjectInListByID(platformScalar("Id", "001000000000003"), records)
	if missing.Kind != ValueNull {
		t.Fatalf("missing = %#v, want null", missing)
	}
}

func TestExecSObjectListEqualityTreatsStringAndPlatformIDFieldsAsEqual(t *testing.T) {
	program, err := CompileAnonymous(`Account first = new Account(Name = 'Before');
first.Id = '001000000000001';
Account second = first.clone(true, true, true, true);
List<Account> left = new List<Account>{ first };
List<Account> right = new List<Account>{ second };
System.assertEquals(true, left == right);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExplicitEmptyParentRelationshipObjectIsPreserved(t *testing.T) {
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
	child := Object("Child__c")
	child.Fields["Parent__r"] = Object("Account")
	value, err := machine.lookupPath(child, []string{"Parent__r"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueObject || value.Type != "Account" {
		t.Fatalf("empty parent relationship = %#v, want Account object", value)
	}
}

func TestExecLookupIDDoesNotHydrateUnloadedParentRelationship(t *testing.T) {
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
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{
				"001000000000001": {
					ID:     "001000000000001",
					Object: "Account",
					Fields: map[string]storage.Value{
						"Name": storage.StringValue("Loaded Parent"),
					},
				},
			},
		},
	}}
	machine.SetOrg(&org)
	child := Object("Child__c")
	child.Fields["Parent__c"] = String("001000000000001")
	value, err := machine.lookupPath(child, []string{"Parent__r"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("unloaded parent relationship = %#v, want null", value)
	}
}

func TestExecUnsavedChildRelationshipShellsNestedField(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
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
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
		},
	}}
	machine.SetOrg(&org)
	child := Object("Child__c")
	value, err := machine.lookupPath(child, []string{"Parent__r", "Name"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("unsaved parent nested field = %#v, want null", value)
	}
}

func TestRelationshipTargetsObjectMatchesNamespacedLocalNames(t *testing.T) {
	relationship := storage.Relationship{ParentObjects: []string{"Education__c"}}
	if !relationshipTargetsObject(relationship, "verifiable__Education__c") {
		t.Fatal("relationship did not match namespaced object API name")
	}
}

func TestRecordFromValueSkipsDerivedParentRelationshipField(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Children"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Children",
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
	child := Object("Child__c")
	child.Fields["Parent__r"] = Object("Account")
	record, err := machine.recordFromValue(&child)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := record.Fields["Parent__r"]; ok {
		t.Fatalf("derived parent relationship was persisted as field: %#v", record.Fields)
	}
}

func TestRecordFromValueDerivesLookupFromParentRelationshipID(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
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
	child := Object("Child__c")
	parent := Object("Account")
	parent.Fields["Id"] = String("001000000000001AAA")
	child.Fields["Parent__r"] = parent
	record, err := machine.recordFromValue(&child)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := record.Fields["Parent__c"]; !ok || value.ID != "001000000000001AAA" {
		t.Fatalf("parent relationship id did not derive lookup: %#v", record.Fields)
	}
	if _, ok := record.Fields["Parent__r"]; ok {
		t.Fatalf("parent relationship object was persisted as field: %#v", record.Fields)
	}
}

func TestExecUpsertResolvesParentRelationshipExternalID(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Parent', External_Key__c = 'parent-ext');
upsert parent External_Key__c;

Contact child = new Contact(LastName = 'Child');
child.putSObject('Account', new Account(External_Key__c = 'parent-ext'));
upsert child;

Contact row = [SELECT Id, AccountId FROM Contact WHERE Id = :child.Id];
System.assertEquals(parent.Id, row.AccountId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true}
	org.Objects["Account"] = account
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpsertResolvesCustomParentRelationshipExternalID(t *testing.T) {
	program, err := CompileAnonymous(`
Provider__c parent = new Provider__c(Name = 'Parent', LookupKey__c = 'parent-ext');
upsert parent;

ProviderAddress__c child = new ProviderAddress__c();
child.putSObject('Provider__r', new Provider__c(LookupKey__c = 'parent-ext'));
upsert child;

ProviderAddress__c row = [SELECT Id, Name, Provider__c FROM ProviderAddress__c WHERE Id = :child.Id];
System.assertEquals(parent.Id, row.Provider__c);
System.assertEquals('ProviderAddress', row.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Provider__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Provider__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"LookupKey__c": {APIName: "LookupKey__c", Type: storage.FieldString, ExternalID: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["ProviderAddress__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ProviderAddress__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Name":        {APIName: "Name", Type: storage.FieldString, Required: true},
				"Provider__c": {APIName: "Provider__c", Type: storage.FieldReference, ReferenceTo: []string{"Provider__c"}, RelationshipName: "Provider__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Provider__c",
				ParentObjects:      []string{"Provider__c"},
				ParentRelationship: "Provider__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.EnableTestContext()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertResolvesCustomParentRelationshipName(t *testing.T) {
	program, err := CompileAnonymous(`
PriceClass__c priceClass = new PriceClass__c(Name = 'Default');
insert priceClass;

OrderItem__c item = new OrderItem__c(PriceClass__r = new PriceClass__c(Name = 'Default'));
insert item;

OrderItem__c row = [SELECT Id, PriceClass__c FROM OrderItem__c WHERE Id = :item.Id];
System.assertEquals(priceClass.Id, row.PriceClass__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["PriceClass__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "PriceClass__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["OrderItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "OrderItem__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"PriceClass__c": {APIName: "PriceClass__c", Type: storage.FieldReference, ReferenceTo: []string{"PriceClass__c"}, RelationshipName: "PriceClass__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "PriceClass__c",
				ParentObjects:      []string{"PriceClass__c"},
				ParentRelationship: "PriceClass__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseUpsertUpdateResolvesCustomParentRelationshipExternalID(t *testing.T) {
	program, err := CompileAnonymous(`
Provider__c oldParent = new Provider__c(Name = 'Old Parent', LookupKey__c = 'old-parent-ext');
Provider__c newParent = new Provider__c(Name = 'New Parent', LookupKey__c = 'new-parent-ext');
insert new List<Provider__c>{oldParent, newParent};

ProviderAddress__c child = new ProviderAddress__c(Name = 'Child', External_Key__c = 'child-ext', Provider__c = oldParent.Id);
insert child;

ProviderAddress__c updateChild = new ProviderAddress__c(
	External_Key__c = 'child-ext',
	Provider__r = new Provider__c(LookupKey__c = 'new-parent-ext')
);
Database.upsert(new List<SObject>{updateChild}, ProviderAddress__c.External_Key__c, false);

ProviderAddress__c row = [SELECT Id, Provider__c FROM ProviderAddress__c WHERE Id = :child.Id];
System.assertEquals(newParent.Id, row.Provider__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Provider__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Provider__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"LookupKey__c": {APIName: "LookupKey__c", Type: storage.FieldString, ExternalID: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["ProviderAddress__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ProviderAddress__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Type: storage.FieldString},
				"External_Key__c": {APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true},
				"Provider__c":     {APIName: "Provider__c", Type: storage.FieldReference, ReferenceTo: []string{"Provider__c"}, RelationshipName: "Provider__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Provider__c",
				ParentObjects:      []string{"Provider__c"},
				ParentRelationship: "Provider__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.EnableTestContext()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFromValueSkipsDerivedParentRelationshipFieldWithoutRelation(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Parent__c": {APIName: "Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}},
				},
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
	child := Object("Child__c")
	child.Fields["Parent__r"] = Object("Account")
	record, err := machine.recordFromValue(&child)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := record.Fields["Parent__r"]; ok {
		t.Fatalf("derived parent relationship without relation was persisted as field: %#v", record.Fields)
	}
}

func TestRecordFromValueRejectsUnknownCustomRelationshipObjectField(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
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
	child := Object("Child__c")
	child.Fields["UnmodeledParent__r"] = Object("Account")
	if _, err := machine.recordFromValue(&child); err == nil || !strings.Contains(err.Error(), "Child__c.UnmodeledParent__r") {
		t.Fatalf("err = %v, want unknown relationship field error", err)
	}
}

func TestExecMissingFieldUsesStoredValueWhenNoQueryMarker(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"},
				},
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
	machine.SetOrg(&org)
	child := Object("Child__c")
	child.Fields["Id"] = String("a00000000000001AAA")
	value, ok := machine.missingSObjectFieldValue(child, "Parent__c")
	text, _ := platformScalarText(value, "Id")
	if !ok || text != "001000000000001AAA" {
		t.Fatalf("missing field did not use stored value: %#v ok=%v", value, ok)
	}
	child.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Child__c", map[string]bool{"id": true})
	value, ok = machine.missingSObjectFieldValue(child, "Parent__c")
	if !ok || value.Kind != ValueNull {
		t.Fatalf("queried field marker should preserve unqueried null default: %#v ok=%v", value, ok)
	}
}

func TestExecInsertedSObjectDoesNotExposeUnqueriedDefaultField(t *testing.T) {
	program, err := CompileAnonymous(`
Thing__c thing = new Thing__c(Name = 'A');
insert thing;
System.assertEquals(null, thing.get(Thing__c.Due__c));
Thing__c queried = [SELECT Due__c FROM Thing__c WHERE Id = :thing.Id LIMIT 1];
System.assertNotEquals(null, queried.Due__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Thing__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Thing__c",
		KeyPrefix: "a00",
		Fields: map[string]storage.Field{
			"Id":     {APIName: "Id", Type: storage.FieldID},
			"Name":   {APIName: "Name", Type: storage.FieldString},
			"Due__c": {APIName: "Due__c", Type: storage.FieldDate, DefaultValue: "TODAY()"},
		},
	}, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUnqueriedLookupFieldDefaultsNull(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
					"Name":      {APIName: "Name", Type: storage.FieldString},
				},
			},
		},
	}}
	machine.SetOrg(&org)
	child := Object("Child__c")
	child.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Child__c", map[string]bool{"id": true})
	if err := machine.unqueriedSObjectFieldError(child, "Parent__c", true); err != nil {
		t.Fatal(err)
	}
	if err := machine.unqueriedSObjectFieldError(child, "Parent__r", true); err != nil {
		t.Fatal(err)
	}
	if err := machine.unqueriedSObjectFieldError(child, "Name", true); err == nil {
		t.Fatal("expected unqueried non-lookup field to error")
	}
}

func TestQueriedSObjectFieldsIncludeNamespaceQualifiedFieldAlias(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "namz"
	account := org.Objects["Account"]
	if account.Definition.Fields == nil {
		account.Definition.Fields = make(map[string]storage.Field)
	}
	account.Definition.Fields["pkg__JoinOn__c"] = storage.Field{APIName: "pkg__JoinOn__c", Type: storage.FieldDate}
	org.Objects["Account"] = account
	machine.SetOrg(&org)

	acct := Object("Account")
	acct.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Account", map[string]bool{"joinon__c": true})
	if err := machine.unqueriedSObjectFieldError(acct, "pkg__JoinOn__c", true); err != nil {
		t.Fatal(err)
	}
}

func TestExecDirectSObjectFieldAccessThrowsForUnqueriedField(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Secret__c = 'Hidden');
Account acct = [SELECT Id FROM Account WHERE Name = 'Acme' LIMIT 1];
Boolean caught = false;
try {
	String secret = acct.Secret__c;
} catch (SObjectException e) {
	caught = e.getMessage().contains('without querying the requested field');
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
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDirectSObjectFieldAccessThrowsForStoredNullUnqueriedCustomField(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Cart__c(Name = 'Cart');
Cart__c cart = [SELECT Id FROM Cart__c WHERE Name = 'Cart' LIMIT 1];
Boolean caught = false;
try {
	String purpose = cart.Purpose__c;
} catch (SObjectException e) {
	caught = e.getMessage().contains('without querying the requested field');
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Cart__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":       {APIName: "Name", Type: storage.FieldString},
				"Purpose__c": {APIName: "Purpose__c", Type: storage.FieldString},
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

func TestFieldSetMemberGetTypeReturnsDisplayType(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString, DisplayType: "STRING"},
			},
		},
	}
	machine.SetOrg(&org)
	member := machine.fieldSetMemberValue("Account", org.Objects["Account"].Definition, storage.FieldSetMemberMetadata{Field: "Name"})
	fieldType := member.Fields["type"]
	if fieldType.Type != "Schema.DisplayType" || !strings.EqualFold(fieldType.Text, "STRING") {
		t.Fatalf("field set member type = %#v, want Schema.DisplayType.STRING", fieldType)
	}
}

func TestExecDirectSObjectFieldAccessAllowsAssignedUnqueriedField(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Account acct = [SELECT Id FROM Account WHERE Name = 'Acme' LIMIT 1];
acct.Name = 'Renamed';
System.assertEquals('Renamed', acct.Name);
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

func TestExecConstructedSObjectWithExistingIDDoesNotBackfillStoredFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account stored = new Account(Name = 'Stored');
insert stored;

Account fabricated = new Account();
fabricated.Id = stored.Id;
System.assertEquals(null, fabricated.get(Account.Name));
System.assertEquals(null, fabricated.Name);
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

func TestQueriedSObjectFieldsIncludeUnqualifiedManagedField(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Cart__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "pkg__Cart__c",
		KeyPrefix: "a00",
		Fields: map[string]storage.Field{
			"Id":              {APIName: "Id", Type: storage.FieldID},
			"pkg__Purpose__c": {APIName: "pkg__Purpose__c", Type: storage.FieldString},
		},
	}}
	machine.SetOrg(&org)
	cart := Object("pkg__Cart__c")
	cart.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("pkg__Cart__c", map[string]bool{"purpose__c": true})
	if !machine.queriedSObjectFieldsIncludes(cart, "pkg__Purpose__c") {
		t.Fatalf("unqualified managed field was not treated as queried")
	}
}

func TestExecDirectSObjectFieldAccessAllowsStoredMetadataDefault(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Account acct = [SELECT Id FROM Account WHERE Name = 'Acme' LIMIT 1];
System.assertEquals(0, acct.Balance__c);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Balance__c"] = storage.Field{APIName: "Balance__c", Type: storage.FieldDecimal, DefaultValue: "0"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecParentRelationshipFieldAccessThrowsForUnqueriedField(t *testing.T) {
	program, err := CompileAnonymous(`
List<Merchandise__c> rows = [SELECT Id, Product2__r.Id FROM Merchandise__c WHERE Id = 'a01000000000001AAA'];
Product__c product = rows[0].Product2__r;
System.assertEquals(0, product.ProductFrequencyLinks__r.size());
Boolean caught = false;
try {
	String description = product.Description__c;
} catch (SObjectException e) {
	caught = e.getMessage().contains('without querying the requested field');
}
System.assert(caught);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id":             {APIName: "Id", Type: storage.FieldID},
				"Description__c": {APIName: "Description__c", Type: storage.FieldString},
			},
			Relations: []storage.Relationship{{
				Field:             "Product__c",
				ParentObjects:     []string{"Product__c"},
				ChildRelationship: "ProductFrequencyLinks__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a02000000000001AAA": {
				Object: "Product__c",
				ID:     "a02000000000001AAA",
				Fields: map[string]storage.Value{
					"Description__c": storage.StringValue("Hidden"),
				},
			},
		},
	}
	org.Objects["ProductFrequencyLink__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ProductFrequencyLink__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Id":         {APIName: "Id", Type: storage.FieldID},
				"Product__c": {APIName: "Product__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Product__c",
				ParentObjects:      []string{"Product__c"},
				ParentRelationship: "Product__r",
				ChildRelationship:  "ProductFrequencyLinks__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Merchandise__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Merchandise__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":          {APIName: "Id", Type: storage.FieldID},
				"Product2__c": {APIName: "Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product2__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Product2__c",
				ParentObjects:      []string{"Product__c"},
				ParentRelationship: "Product2__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001AAA": {
				Object: "Merchandise__c",
				ID:     "a01000000000001AAA",
				Fields: map[string]storage.Value{
					"Product2__c": storage.IDValue("a02000000000001AAA"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecParentRelationshipQueriedFieldIsReadable(t *testing.T) {
	program, err := CompileAnonymous(`
List<Merchandise__c> rows = [SELECT Id, Product2__r.Description__c FROM Merchandise__c WHERE Id = 'a01000000000001AAA'];
Product__c product = rows[0].Product2__r;
System.assertEquals('Hidden', product.Description__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id":             {APIName: "Id", Type: storage.FieldID},
				"Description__c": {APIName: "Description__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a02000000000001AAA": {
				Object: "Product__c",
				ID:     "a02000000000001AAA",
				Fields: map[string]storage.Value{
					"Description__c": storage.StringValue("Hidden"),
				},
			},
		},
	}
	org.Objects["Merchandise__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Merchandise__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":          {APIName: "Id", Type: storage.FieldID},
				"Product2__c": {APIName: "Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}, RelationshipName: "Product2__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Product2__c",
				ParentObjects:      []string{"Product__c"},
				ParentRelationship: "Product2__r",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001AAA": {
				Object: "Merchandise__c",
				ID:     "a01000000000001AAA",
				Fields: map[string]storage.Value{
					"Product2__c": storage.IDValue("a02000000000001AAA"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardParentRelationshipQueriedNamespacedFieldIsReadable(t *testing.T) {
	program, err := CompileAnonymous(`
List<OpportunityContactRole> roles = [SELECT Id, ContactId, Contact.pkg__SelectedDate__c FROM OpportunityContactRole WHERE Id = '00K000000000001AAA'];
Contact contact = roles[0].Contact;
System.assertEquals(Date.newInstance(2026, 5, 2), contact.pkg__SelectedDate__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"Id":                   {APIName: "Id", Type: storage.FieldID},
				"pkg__SelectedDate__c": {APIName: "pkg__SelectedDate__c", Type: storage.FieldDate},
			},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001AAA": {
				Object: "Contact",
				ID:     "003000000000001AAA",
				Fields: map[string]storage.Value{
					"pkg__SelectedDate__c": storage.DateValue("2026-05-02"),
				},
			},
		},
	}
	org.Objects["OpportunityContactRole"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "OpportunityContactRole",
			KeyPrefix: "00K",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"ContactId": {APIName: "ContactId", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Contact"},
			},
			Relations: []storage.Relationship{{
				Field:              "ContactId",
				ParentObjects:      []string{"Contact"},
				ParentRelationship: "Contact",
			}},
		},
		Records: map[storage.ID]storage.Record{
			"00K000000000001AAA": {
				Object: "OpportunityContactRole",
				ID:     "00K000000000001AAA",
				Fields: map[string]storage.Value{
					"ContactId": storage.IDValue("003000000000001AAA"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLSingletonFieldAccessUnwrapsOneRow(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
String name = [SELECT Name FROM Account LIMIT 1].Name;
System.assertEquals('Acme', name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecParentRelationshipDoesNotLoadFromLookupID(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = '001000000000001AAA');
System.assertEquals(null, child.Parent__r.Name);
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

func TestExecDMLReturnedParentRelationshipDoesNotLoadFromLookupID(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Acme');
insert parent;
Child__c child = new Child__c(Parent__c = parent.Id);
insert child;
System.assertEquals(null, child.Parent__r.Name);
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

func TestExecMissingOptionalParentRelationshipNestedFieldIsNull(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c();
System.assertEquals(null, child.Parent__r);
System.assertEquals(null, child.Parent__r.Name);
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

func TestRelationshipNullRequiresMetadataForCustomRelationshipHops(t *testing.T) {
	machine := New(nil)
	orderItem, ok := machine.relationshipNullFieldAccessValue("pkg__OrderItemLine__c", "pkg__OrderItem__r")
	if ok {
		t.Fatalf("order item relationship = %#v, ok=%v; want no inferred relationship without metadata", orderItem, ok)
	}
}

func TestExecDerivedParentRelationshipWithLookupIDMissingParentShellsNestedField(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = '001000000000001AAA', Parent__r = null);
System.assertEquals(null, child.Parent__r.Name);
Child__c nested = new Child__c(Parent__c = '001000000000001AAA', Parent__r = null);
Wrapper__c wrapper = new Wrapper__c(Children__r = new List<Child__c>{ nested });
System.assertEquals(null, wrapper.Children__r[0].Parent__r.Name);
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
		"Wrapper__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Wrapper__c",
				KeyPrefix: "a01",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Children"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Children",
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

func TestExecJSONDeserializeSObjectSyntheticParentRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = (Child__c)JSON.deserialize('{"Parent__r":{"Id":"a01000000000001AAA","Name":"Parent"}}', Child__c.class);
System.assertEquals('Parent', child.Parent__r.Name);
System.assertEquals('a01000000000001AAA', child.Parent__r.Id);

Subscription__c membership = (Subscription__c)JSON.deserialize('{"StartDate__c":"2026-05-01","EndDate__c":"2026-05-31T00:00:00.000Z","OrderItemLine__r":{"Id":"a02000000000001AAA","OrderItem__c":"a03000000000001AAA"}}', Subscription__c.class);
System.assertEquals(Date.newInstance(2026, 5, 1), membership.StartDate__c);
System.assertEquals('a03000000000001AAA', membership.OrderItemLine__r.OrderItem__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a01",
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
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Subscription__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Subscription__c",
				KeyPrefix: "a04",
				Fields: map[string]storage.Field{
					"Id":               {APIName: "Id", Type: storage.FieldID},
					"StartDate__c":     {APIName: "StartDate__c", Type: storage.FieldDate},
					"EndDate__c":       {APIName: "EndDate__c", Type: storage.FieldDateTime},
					"OrderItemLine__c": {APIName: "OrderItemLine__c", Type: storage.FieldReference, ReferenceTo: []string{"OrderItemLine__c"}},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"OrderItemLine__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "OrderItemLine__c",
				KeyPrefix: "a02",
				Fields: map[string]storage.Field{
					"Id":           {APIName: "Id", Type: storage.FieldID},
					"OrderItem__c": {APIName: "OrderItem__c", Type: storage.FieldReference, ReferenceTo: []string{"OrderItem__c"}},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"OrderItem__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "OrderItem__c",
				KeyPrefix: "a03",
				Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
				},
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

func TestExecLookupIdOnlyParentRelationshipComparesNull(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = 'a01000000000001AAA');
System.assertEquals(null, child.Parent__r);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
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
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
				},
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

func TestExecJSONDeserializeParentRelationshipAliasesShareLoadedParent(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = (Child__c)JSON.deserialize('{"pkg__Parent__r":{"Id":"001000000000001AAA","Name":"First"},"Parent__r":{"Id":"001000000000002AAA","Name":"Second"}}', Child__c.class);
System.assertEquals('First', child.Parent__r.Name);
System.assertEquals('First', child.getSObject('pkg__Parent__r').get('Name'));
System.assertEquals(null, child.Parent__c);

Child__c explicitLookup = (Child__c)JSON.deserialize('{"Parent__c":"001000000000003AAA","Parent__r":{"Id":"001000000000004AAA","Name":"Loaded"}}', Child__c.class);
System.assertEquals('001000000000003AAA', explicitLookup.Parent__c);
System.assertEquals('Loaded', explicitLookup.Parent__r.Name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Namespace: "pkg", Objects: map[string]storage.ObjectState{
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
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "pkg__Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "pkg__Parent__r",
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

func TestExecSObjectMapCovariancePreservesValueReferences(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Id = '001000000000001AAA', Name = 'Before');
Map<Id, Account> typed = new Map<Id, Account>{ account.Id => account };
Map<Id, SObject> genericRecords = typed;
Account fromGeneric = (Account)genericRecords.get(account.Id);
fromGeneric.Name = 'After';
System.assertEquals('After', account.Name);
System.assertEquals('After', typed.get(account.Id).Name);
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

func TestExecSecurityStripInaccessibleReturnsDecision(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(Name = 'Acme') };
SObjectAccessDecision decision = Security.stripInaccessible(AccessType.CREATABLE, accounts);
System.assertEquals(accounts, decision.getRecords());
System.assertEquals(0, decision.getRemovedFields().size());
System.assertEquals(0, decision.getModifiedIndexes().size());
System.assertEquals('CREATABLE', AccessType.CREATABLE.name());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSecurityStripInaccessibleTracksRemovedFields(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> rows = [SELECT Id, Name, Secret__c FROM Account];
SObjectAccessDecision decision = Security.stripInaccessible(AccessType.READABLE, rows);
Map<String, Set<String>> removed = decision.getRemovedFields();
System.assertEquals(1, removed.size());
System.assert(removed.containsKey('Account'));
System.assert(removed.get('Account').contains('Secret__c'));
System.assertEquals(1, decision.getModifiedIndexes().size());
System.assert(decision.getModifiedIndexes().contains(0));
List<Account> stripped = (List<Account>)decision.getRecords();
System.assertEquals('Acme', stripped[0].Name);
System.assertEquals('Hidden', rows[0].Secret__c);
Boolean caught = false;
try {
	System.debug(stripped[0].Secret__c);
} catch (SObjectException e) {
	caught = e.getMessage().contains('without querying the requested field');
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := stripInaccessibleTestOrg()
	machine.SetOrg(&org)
	machine.executionUser = stripInaccessibleTestUser()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSecurityStripInaccessibleEnforcesRootCRUD(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	Security.stripInaccessible(AccessType.CREATABLE, new List<Account>{ new Account(Name = 'Acme') });
} catch (NoAccessException e) {
	caught = e.getMessage().containsIgnoreCase('no access to entity');
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := stripInaccessibleTestOrg()
	delete(org.Objects["ObjectPermissions"].Records, "110000000000001")
	machine.SetOrg(&org)
	machine.executionUser = stripInaccessibleTestUser()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSecurityStripInaccessibleUsesProfileFieldPermissionAllowlist(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Acme', TradeStyle = 'Hidden');
SObjectAccessDecision decision = Security.stripInaccessible(AccessType.CREATABLE, new List<Account>{ row });
Map<String, Set<String>> removed = decision.getRemovedFields();
System.assert(removed.get('Account').contains('TradeStyle'));
List<Account> stripped = (List<Account>) decision.getRecords();
System.assertEquals('Acme', stripped[0].Name);
System.assertEquals(null, stripped[0].TradeStyle);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	account := org.Objects["Account"]
	account.Definition.Fields["TradeStyle"] = storage.Field{APIName: "TradeStyle", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	user := Object("User")
	user.Fields["Id"] = String("005000000000999")
	user.Fields["ProfileId"] = String("00e000000000006")
	machine.executionUser = user
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSecurityStripInaccessibleKeepsRequiredNameFieldsForCreate(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'CreateContact', Label = 'Create Contact');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Contact', PermissionsRead = true, PermissionsCreate = true);
insert new FieldPermissions(ParentId = ps.Id, SObjectType = 'Contact', Field = 'Contact.Email', PermissionsRead = true, PermissionsEdit = true);
Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce'];
User u = new User(
	Username = 'create-contact@example.invalid',
	Alias = 'cuser',
	Email = 'create-contact@example.invalid',
	LastName = 'Create',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
System.assert([SELECT Id FROM PermissionSetAssignment].size() >= 1);
System.runAs(u) {
	Contact row = new Contact(LastName = 'Allowed', Email = 'allowed@example.invalid', Title = 'Hidden');
	SObjectAccessDecision decision = Security.stripInaccessible(AccessType.CREATABLE, new List<Contact>{ row });
	List<Contact> stripped = (List<Contact>) decision.getRecords();
	System.assertEquals('Allowed', stripped[0].LastName);
	System.assertEquals('allowed@example.invalid', stripped[0].Email);
	System.assertEquals(null, stripped[0].Title);
	System.assert(decision.getRemovedFields().get('Contact').contains('Title'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecWithSharingRunAsCanSeeOwnPermissionSetAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(
	Name = 'CustomizeApp',
	Label = 'Customize App',
	PermissionsCustomizeApplication = true
);
insert ps;
Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard Platform User'];
User u = new User(
	Username = 'customize@example.invalid',
	Alias = 'custom',
	Email = 'customize@example.invalid',
	LastName = 'Customize',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
System.runAs(u) {
	Id userId = UserInfo.getUserId();
	System.assertEquals(u.Id, userId);
	System.assert([
		SELECT Id
		FROM PermissionSetAssignment
	].size() >= 1);
	System.assert([
		SELECT Id
		FROM PermissionSetAssignment
		WHERE AssigneeId = :userId
	].size() >= 1);
	List<PermissionSetAssignment> assignments = [
		SELECT Id
		FROM PermissionSetAssignment
		WHERE AssigneeId = :userId
		AND PermissionSetId IN (
			SELECT Id
			FROM PermissionSet
			WHERE PermissionsCustomizeApplication = TRUE
		)
	];
	System.assertEquals(1, assignments.size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentClass = "SharingProbe"
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAdministratorProfileExposesPermissionSetAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'System Administrator'];
User u = new User(
	Username = 'sysadmin@example.invalid',
	Alias = 'sysadm',
	Email = 'sysadmin@example.invalid',
	LastName = 'Admin',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
System.runAs(u) {
	Id userId = UserInfo.getUserId();
	System.assertEquals(1, [
		SELECT Id
		FROM PermissionSetAssignment
		WHERE AssigneeId = :userId
		AND PermissionSetId IN (
			SELECT Id
			FROM PermissionSet
			WHERE PermissionsCustomizeApplication = TRUE
			AND PermissionsModifyAllData = TRUE
			AND PermissionsAuthorApex = TRUE
		)
	].size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentClass = "SharingProbe"
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsMaterializedSystemAdministratorHasProfilePermissionSetAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'System Administrator'];
User u = new User(
	Username = 'runas-sysadmin@example.invalid',
	Alias = 'rsysad',
	Email = 'runas-sysadmin@example.invalid',
	LastName = 'RunAsAdmin',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
System.runAs(u) {
	Id userId = UserInfo.getUserId();
	System.assertEquals(1, [
		SELECT Id
		FROM PermissionSetAssignment
		WHERE AssigneeId = :userId
		AND PermissionSetId IN (
			SELECT Id
			FROM PermissionSet
			WHERE PermissionsCustomizeApplication = TRUE
			AND PermissionsModifyAllData = TRUE
			AND PermissionsAuthorApex = TRUE
		)
	].size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentClass = "SharingProbe"
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardPlatformUserCannotReadCase(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard Platform User'];
User u = new User(
	Username = 'platform-user@example.invalid',
	Alias = 'puser',
	Email = 'platform-user@example.invalid',
	LastName = 'Platform',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
System.runAs(u) {
	System.assertEquals(false, Case.SObjectType.getDescribe().isAccessible());
	System.assertEquals(false, Case.CaseNumber.getDescribe().isAccessible());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Case")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMinimumAccessCanReadRecordTypeAndCustomMetadataDescribe(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce'];
User u = new User(
	Username = 'metadata-readable@example.invalid',
	Alias = 'mread',
	Email = 'metadata-readable@example.invalid',
	LastName = 'Metadata',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
System.runAs(u) {
	System.assert(RecordType.SObjectType.getDescribe().isAccessible(), 'RecordType should be readable metadata');
	System.assert(Filter__mdt.SObjectType.getDescribe().isAccessible(), 'custom metadata should be readable');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Filter__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Filter__mdt",
		Fields: map[string]storage.Field{
			"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			"MasterLabel":   {APIName: "MasterLabel", Type: storage.FieldString},
		},
		Metadata: map[string]string{"kind": "customMetadata"},
	}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSObjectTypeSurvivesConcreteListAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
List<Opportunity> opportunities = new List<Opportunity>{ new Opportunity(Name = 'Acme') };
List<SObject> records = opportunities;
System.assertEquals('Opportunity', records.getSObjectType().getDescribe().getName());
records = new Opportunity[] { new Opportunity(Name = 'Array') };
System.assertEquals('Opportunity', records.getSObjectType().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Opportunity")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSObjectTypeSurvivesMethodParameterWidening(t *testing.T) {
	accept, err := CompileAnonymous(`
System.assertEquals('Opportunity', records.getSObjectType().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Probe.accept(new Opportunity[] { new Opportunity(Name = 'Acme') });
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Opportunity")
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{
		Name:       "Probe.accept",
		ClassName:  "Probe",
		IsStatic:   true,
		Params:     []Param{{Name: "records", Type: "List<SObject>"}},
		ReturnType: "void",
		Program:    accept,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestWithConcreteSObjectListRuntimeUsesItemRuntime(t *testing.T) {
	item := Object("SObject")
	item.Runtime = "Opportunity"
	records := List(item)
	records.Type = "List<SObject>"
	records = withConcreteSObjectListRuntime(records)
	if got, want := records.Runtime, "List<Opportunity>"; got != want {
		t.Fatalf("runtime = %q, want %q", got, want)
	}
}

func TestListSObjectTypeNameUsesItemObjectMarker(t *testing.T) {
	item := Object("SObject")
	item.Fields["object"] = String("Opportunity")
	records := List(item)
	records.Type = "List<SObject>"
	if got, want := listSObjectTypeName(records), "Opportunity"; got != want {
		t.Fatalf("object type = %q, want %q", got, want)
	}
}

func TestExecPermissionSetGroupAssignmentGrantsComponentPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'ReadAccountShipping', Label = 'Read Account Shipping');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Account', PermissionsRead = true);
insert new FieldPermissions(ParentId = ps.Id, SObjectType = 'Account', Field = 'Account.ShippingAddress', PermissionsRead = true);
PermissionSetGroup psg = new PermissionSetGroup(DeveloperName = 'Read_Account_Group', MasterLabel = 'Read Account Group');
insert psg;
insert new PermissionSetGroupComponent(PermissionSetGroupId = psg.Id, PermissionSetId = ps.Id);
Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce'];
User u = new User(
	Username = 'psg-user@example.invalid',
	Alias = 'psguser',
	Email = 'psg-user@example.invalid',
	LastName = 'PSG',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetGroupId = psg.Id);
System.assertEquals(1, [SELECT Id FROM PermissionSetGroupComponent WHERE PermissionSetGroupId = :psg.Id].size(), 'expected group component record');
System.assertEquals(1, [SELECT Id FROM PermissionSetAssignment WHERE PermissionSetGroupId = :psg.Id].size(), 'expected group assignment record');
System.runAs(u) {
	System.assert(Account.SObjectType.getDescribe().isAccessible(), 'expected PSG component object permission to grant Account read');
	System.assert(Account.ShippingStreet.getDescribe().isAccessible(), 'expected PSG component compound field permission to grant ShippingStreet read');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSetGroup")
	storage.EnsureStandardObject(&org, "PermissionSetGroupComponent")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSecurityStripInaccessibleRemovesInaccessibleChildSubquery(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'ReadAccountOnly', Label = 'Read Account Only');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Account', PermissionsRead = true);
insert new FieldPermissions(ParentId = ps.Id, SObjectType = 'Account', Field = 'Account.Name', PermissionsRead = true);
Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce'];
User u = new User(
	Username = 'subquery-user@example.invalid',
	Alias = 'squser',
	Email = 'subquery-user@example.invalid',
	LastName = 'Subquery',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
System.runAs(u) {
	insert new Account(Name = 'Acme');
	Account acct = [SELECT Id FROM Account WHERE Name = 'Acme'];
	insert new Contact(AccountId = acct.Id, LastName = 'Child');
	SObjectAccessDecision decision = Security.stripInaccessible(
		AccessType.READABLE,
		[SELECT Id, Name, (SELECT LastName FROM Contacts) FROM Account WHERE Id = :acct.Id]
	);
	List<Account> rows = (List<Account>) decision.getRecords();
	Boolean caught = false;
	try {
		rows[0].getSObjects('Contacts');
	} catch (SObjectException e) {
		caught = e.getMessage().contains('without querying the requested field');
	}
	System.assert(caught);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecChildRelationshipPropertyReturnsShallowListCopy(t *testing.T) {
	program, err := CompileAnonymous(`
String payload = '{"attributes":{"type":"Account"},"Name":"Acme","Contacts":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Contact"},"LastName":"Original"}]}}';
Account account = (Account)JSON.deserialize(payload, Account.class);
List<Contact> contacts = account.Contacts;
contacts.add(new Contact(LastName = 'Added'));
System.assertEquals(1, account.Contacts.size());
contacts[0].LastName = 'Changed';
System.assertEquals('Changed', account.Contacts[0].LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecChildRelationshipPropertySupportsListGetMember(t *testing.T) {
	program, err := CompileAnonymous(`
Wrapper__c wrapper = new Wrapper__c(
	Children__r = new List<Child__c>{ new Child__c(Name = 'Smith') }
);
System.assertEquals('Smith', wrapper.Children__r.get(0).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"Wrapper__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Wrapper__c",
				KeyPrefix: "a01",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":         {APIName: "Id", Type: storage.FieldID},
					"Name":       {APIName: "Name", Type: storage.FieldString},
					"Wrapper__c": {APIName: "Wrapper__c", Type: storage.FieldReference, ReferenceTo: []string{"Wrapper__c"}, RelationshipName: "Wrapper__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Wrapper__c",
					ParentObjects:      []string{"Wrapper__c"},
					ParentRelationship: "Wrapper__r",
					ChildRelationship:  "Children__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestLookupChildRelationshipPrefersNamespacedLoadedList(t *testing.T) {
	machine := New(nil)
	org := childRelationshipNamespaceTestOrg()
	machine.SetOrg(&org)
	child := Object("Child__c")
	child.Fields["Name"] = String("Jones")
	children := List(child)
	children.Type = "List<Child__c>"
	wrapper := Object("Wrapper__c")
	wrapper.Fields["Children__r"] = Decimal(5)
	wrapper.Fields["NU__Children__r"] = children

	got, err := machine.lookupPath(wrapper, []string{"Children__r"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || len(got.List) != 1 {
		t.Fatalf("Children__r lookup = %#v, want one child list", got)
	}
	if name := got.List[0].Fields["Name"]; name.Kind != ValueString || name.Text != "Jones" {
		t.Fatalf("child name = %#v, want Jones", name)
	}
}

func TestLookupChildRelationshipDecimalPlaceholderDefaultsToEmptyList(t *testing.T) {
	machine := New(nil)
	org := childRelationshipNamespaceTestOrg()
	machine.SetOrg(&org)
	wrapper := Object("Wrapper__c")
	wrapper.Fields["Children__r"] = Decimal(0)

	got, err := machine.lookupPath(wrapper, []string{"Children__r"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueList || len(got.List) != 0 {
		t.Fatalf("Children__r lookup = %#v, want empty child list", got)
	}
}

func childRelationshipNamespaceTestOrg() storage.OrgState {
	return storage.OrgState{Namespace: "NU", Objects: map[string]storage.ObjectState{
		"Wrapper__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Wrapper__c",
				KeyPrefix: "a01",
				Fields:    map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id":         {APIName: "Id", Type: storage.FieldID},
					"Name":       {APIName: "Name", Type: storage.FieldString},
					"Wrapper__c": {APIName: "Wrapper__c", Type: storage.FieldReference, ReferenceTo: []string{"Wrapper__c"}, RelationshipName: "Wrapper__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Wrapper__c",
					ParentObjects:      []string{"Wrapper__c"},
					ParentRelationship: "Wrapper__r",
					ChildRelationship:  "Children__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
}

func TestExecSOQLNegativeDecimalBindMatchesDecimalField(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Refund');
account.put('PaymentAmount__c', -10);
insert account;
Decimal refundPrice = -10;
System.assertEquals(1, [SELECT Id FROM Account WHERE PaymentAmount__c = :refundPrice].size());
System.assertEquals(1, [SELECT Id FROM Account WHERE PaymentAmount__c = -10.00].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["PaymentAmount__c"] = storage.Field{APIName: "PaymentAmount__c", Type: storage.FieldDecimal}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExplicitNullBooleanSObjectFieldDefaultsFalse(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account();
account.put('Active__c', null);
System.assertEquals(false, account.Active__c);
if (account.Active__c) {
  System.assert(false, 'default false checkbox should not enter if branch');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Active__c"] = storage.Field{APIName: "Active__c", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", DefaultValue: "false"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSynchronizeFabricatedSObjectRelationshipsClearsStaleChildren(t *testing.T) {
	staleChild := Object("sfab_FabricatedSObject")
	staleNode := Object("sfab_ChildRelationshipNode")
	staleNode.Fields["fieldName"] = String("Lines__r")
	staleNode.Fields["children"] = List(staleChild)
	otherNode := Object("sfab_FieldValuePairNode")
	otherNode.Fields["fieldName"] = String("Name")

	fabricated := Object("sfab_FabricatedSObject")
	fabricated.Fields["nodes"] = List(otherNode, staleNode)

	childrenByRelation := Map()
	relationKey := mapKey(String("Lines__r"))
	childrenByRelation.Map[relationKey] = List()
	childrenByRelation.MapKeys[relationKey] = String("Lines__r")

	wrapper := Object("AnyFabricator")
	wrapper.Fields["childrenByRelation"] = childrenByRelation
	wrapper.Fields["fabricatedSObject"] = fabricated

	synced := New(nil).synchronizeFabricatedSObjectRelationships(wrapper)
	_, syncedFabricated, ok := objectFieldValue(synced, "fabricatedSObject")
	if !ok {
		t.Fatal("expected fabricatedSObject")
	}
	_, syncedNodes, ok := objectFieldValue(syncedFabricated, "nodes")
	if !ok || syncedNodes.Kind != ValueList {
		t.Fatal("expected synchronized nodes")
	}
	if len(syncedNodes.List) != 2 {
		t.Fatalf("expected stale child node replaced, got %d nodes", len(syncedNodes.List))
	}
	_, children, ok := objectFieldValue(syncedNodes.List[1], "children")
	if !ok || children.Kind != ValueList {
		t.Fatal("expected child relationship node children")
	}
	if len(children.List) != 0 {
		t.Fatalf("expected empty current relationship children, got %d", len(children.List))
	}
}

func TestExecDateDefaultFormulaCoercesToDate(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Account row = [SELECT RenewalDate__c FROM Account WHERE Name = 'Acme'];
Date renewal = row.RenewalDate__c;
System.assertEquals(Date.today(), renewal);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["RenewalDate__c"] = storage.Field{APIName: "RenewalDate__c", Type: storage.FieldDate, DefaultValue: "Today()"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCalculatedDatetimeFormulaBlankNumericResultIsNull(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account();
System.assertEquals(null, account.get('CompletedAt__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["StartedAt__c"] = storage.Field{APIName: "StartedAt__c", Type: storage.FieldDateTime, DisplayType: "DATETIME"}
	account.Definition.Fields["ElapsedMs__c"] = storage.Field{APIName: "ElapsedMs__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"}
	account.Definition.Fields["CompletedAt__c"] = storage.Field{
		APIName:     "CompletedAt__c",
		Type:        storage.FieldCalculated,
		DisplayType: "DATETIME",
		Formula:     "StartedAt__c + (ElapsedMs__c / 86400000)",
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingDefaultFieldDoesNotOverwriteSparseUpdate(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Defaulted__c = 7);
Account row = [SELECT Id FROM Account WHERE Name = 'Acme'];
Account sparse = new Account(Id = row.Id, Name = 'Changed');
System.assertEquals(7, sparse.get('Defaulted__c'));
upsert sparse;
Account updated = [SELECT Defaulted__c FROM Account WHERE Id = :row.Id];
System.assertEquals(7, updated.Defaulted__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Defaulted__c"] = storage.Field{APIName: "Defaulted__c", Type: storage.FieldDecimal, DefaultValue: "0"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpsertHydratesSparseUpdateBeforeTrigger(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
	for (Account a : Trigger.new) {
		System.assertEquals(7, a.Defaulted__c);
	if (a.Defaulted__c == null) {
		a.Defaulted__c.addError('defaulted missing');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account existing = new Account(Name = 'Acme', External_Key__c = 'ext-1', Defaulted__c = 7);
insert existing;
Account sparse = new Account(External_Key__c = 'EXT-1', Name = 'Changed');
upsert sparse External_Key__c;
Account updated = [SELECT Name, Defaulted__c FROM Account WHERE Id = :existing.Id];
System.assertEquals('Changed', updated.Name);
System.assertEquals(7, updated.Defaulted__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Defaulted__c"] = storage.Field{APIName: "Defaulted__c", Type: storage.FieldDecimal, Required: true}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeUpdateSparseUpsert",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpdateTriggerCanReadUnqueriedFieldFromQueriedDMLRecord(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Contact c : Trigger.new) {
	System.assertEquals(null, c.Salutation);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Contact contact = new Contact(LastName = 'Lovelace');
insert contact;
Contact queried = [SELECT Id FROM Contact WHERE Id = :contact.Id LIMIT 1];
queried.LastName = 'Byron';
update queried;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "ContactBeforeUpdateProjection",
		Object:    "Contact",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDMLAccessibleMarkerOverridesExistingSOQLProjection(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	contact := Object("Contact")
	contact.Fields["Id"] = platformScalar("Id", "003000000000001AAA")
	contact.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Contact", map[string]bool{"id": true})
	markDMLAccessibleFields(&contact)
	_, handled, err := machine.callSObjectMember(contact, "get", []Value{String("Salutation")})
	if !handled {
		t.Fatal("SObject.get was not handled")
	}
	if err != nil {
		t.Fatalf("DML-accessible projection read returned error: %v", err)
	}
}

func TestDMLAccessibleDefaultedFieldDoesNotTripSOQLProjection(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	contact := Object("Contact")
	contact.Fields["Id"] = platformScalar("Id", "003000000000001AAA")
	contact.Fields["HasOptedOutOfEmail"] = Bool(false)
	markDefaultedSObjectField(&contact, "HasOptedOutOfEmail")
	markDMLAccessibleFields(&contact)
	value, handled, err := machine.callSObjectMember(contact, "get", []Value{String("HasOptedOutOfEmail")})
	if !handled {
		t.Fatal("SObject.get was not handled")
	}
	if err != nil {
		t.Fatalf("DML-accessible defaulted field read returned error: %v", err)
	}
	if value.Kind != ValueBool || value.Bool {
		t.Fatalf("defaulted field value = %#v, want false", value)
	}
}

func TestExecUpdatePreservesExplicitNullThroughBeforeTrigger(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	a.Rating = 'Hot';
}
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme', Payload__c = 'payload');
insert account;
account.Payload__c = null;
update account;
Account updated = [SELECT Payload__c, Rating FROM Account WHERE Id = :account.Id];
System.assertEquals(null, updated.Payload__c);
System.assertEquals('Hot', updated.Rating);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	account.Definition.Fields["Payload__c"] = storage.Field{APIName: "Payload__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeUpdatePreserveExplicitNull",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpsertPreservesExplicitNullThroughBeforeUpdateTrigger(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	a.Rating = 'Hot';
}
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account existing = new Account(Name = 'Acme', External_Key__c = 'ext-1', Payload__c = 'payload');
insert existing;
Account updateRow = new Account(External_Key__c = 'EXT-1', Payload__c = null);
upsert updateRow External_Key__c;
Account updated = [SELECT Payload__c, Rating FROM Account WHERE Id = :existing.Id];
System.assertEquals(null, updated.Payload__c);
System.assertEquals('Hot', updated.Rating);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	account.Definition.Fields["Payload__c"] = storage.Field{APIName: "Payload__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeUpsertUpdatePreserveExplicitNull",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
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
System.assertEquals(false, describe.isAutoNumber());
System.assertEquals('Name', describe.compoundFieldName);
System.assertEquals('Name', describe.getCompoundFieldName());
System.assertEquals('Account', Account.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED).getName());
System.assertEquals(Account.SObjectType, Account.SObjectType.SObjectType);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeReferenceTargetsPreserveMetadataForProviderFields(t *testing.T) {
	program, err := CompileAnonymous(`
List<Schema.SObjectType> targets = Credentialing_Workflow__c.Provider__c.getDescribe().getReferenceTo();
System.assert(targets.contains(Contact.SObjectType));
System.assertEquals(false, targets.contains(User.SObjectType));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "User")
	org.Objects["Credentialing_Workflow__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Credentialing_Workflow__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name":        {APIName: "Name", Type: storage.FieldString},
				"Provider__c": {APIName: "Provider__c", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Provider"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLContentDocumentLinksChildRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
ContentVersion version = new ContentVersion(
	Title = 'Credential',
	PathOnClient = 'credential.pdf',
	VersionData = Blob.valueOf('pdf'),
	FirstPublishLocationId = account.Id
);
insert version;
Account row = [SELECT Id, (SELECT ContentDocumentId, ContentDocument.Title FROM ContentDocumentLinks) FROM Account WHERE Id = :account.Id];
System.assertEquals(1, row.ContentDocumentLinks.size());
System.assertEquals('Credential', row.ContentDocumentLinks[0].ContentDocument.Title);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "ContentVersion")
	storage.EnsureStandardObject(&org, "ContentDocument")
	storage.EnsureStandardObject(&org, "ContentDocumentLink")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectResultStubBackedAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Account.SObjectType.getDescribe();
System.assertEquals('Account', describe.getLocalName());
System.assertEquals(0, describe.getFieldSets().getMap().size());
System.assertEquals(false, describe.isFeedEnabled());
System.assertEquals(false, Schema.SObjectType.User.isFeedEnabled());
System.assertEquals(false, describe.isMergeable());
System.assertEquals(true, describe.isMruEnabled());
System.assertEquals(true, describe.isUndeletable());
System.assertEquals(false, describe.isDeprecatedAndHidden());
System.assertEquals(false, describe.getDataTranslationEnabled());
System.assertEquals(null, describe.getDefaultImplementation());
System.assertEquals(false, describe.getHasSubtypes());
System.assertEquals(null, describe.getImplementedBy());
System.assertEquals(null, describe.getImplementsInterfaces());
System.assertEquals(false, describe.getIsInterface());
System.assertEquals(Account.SObjectType, describe.getSobjectType());
System.assertEquals(SObjectDescribeOptions.FULL, describe.getSObjectDescribeOption());
Schema.DescribeSObjectResult deferredDescribe = Account.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED);
System.assertEquals(SObjectDescribeOptions.DEFERRED, deferredDescribe.getSObjectDescribeOption());
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

func TestExecDescribeSObjectResultMergeableUsesObjectMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Mergeable__c.SObjectType.getDescribe();
System.assertEquals(true, describe.isMergeable());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Mergeable__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Mergeable__c",
			Fields:  map[string]storage.Field{},
			Metadata: map[string]string{
				"mergeable": "true",
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectResultNamespacedCustomObjectName(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Setup_Data__c.SObjectType.getDescribe();
System.assertEquals('verifiable__Setup_Data__c', describe.getName());
System.assertEquals('Setup_Data__c', describe.getLocalName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	org.Objects["Setup_Data__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Setup_Data__c",
			KeyPrefix: "a00",
			Fields:    map[string]storage.Field{},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectResultAssociatedObjectMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult accountDescribe = Account.SObjectType.getDescribe();
System.assertEquals(null, accountDescribe.getAssociateEntityType());
System.assertEquals(null, accountDescribe.getAssociateParentEntity());
Schema.DescribeSObjectResult historyDescribe = AccountHistory.SObjectType.getDescribe();
System.assertEquals('History', historyDescribe.getAssociateEntityType());
System.assertEquals('Account', historyDescribe.getAssociateParentEntity());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["AccountHistory"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "AccountHistory",
			Label:       "Account History",
			PluralLabel: "Account History",
			Fields:      map[string]storage.Field{},
			Metadata: map[string]string{
				"associateEntityType":   "History",
				"associateParentEntity": "Account",
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldResultStubBackedAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult describe = Account.SObjectType.getDescribe().fields.getMap().get('Defaulted__c').getDescribe();
System.assertEquals('Defaulted__c', describe.getLocalName());
System.assertEquals('Fallback', describe.getDefaultValue());
System.assertEquals('\'Fallback\'', describe.getDefaultValueFormula());
System.assertEquals(true, describe.isDefaultedOnCreate());
System.assertEquals(true, describe.isFilterable());
System.assertEquals(true, describe.isGroupable());
System.assertEquals(true, describe.isSortable());
System.assertEquals(false, describe.isDeprecatedAndHidden());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Defaulted__c"] = storage.Field{APIName: "Defaulted__c", Type: storage.FieldString, DefaultValue: "'Fallback'"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldTokenCoercesToDescribeFieldResult(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult field = Schema.SObjectType.Account.fields.Name;
System.assertEquals('Name', field.getName());
System.assertEquals(Schema.DisplayType.STRING, field.getType());
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

func TestExecDescribeFieldResultDefaultFormulaCanStripQuoteCharacters(t *testing.T) {
	program, err := CompileAnonymous(`
String ns = String.valueOf(Account.Advancement_Namespace__c.getDescribe().getDefaultValueFormula()).remove('\"');
System.assertEquals('gem', ns);
Schema.DescribeSObjectResult[] desc = Schema.describeSObjects(new String[]{ns + '__Advancement_Setting__mdt'});
System.assertEquals('gem__Advancement_Setting__mdt', desc[0].getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Advancement_Namespace__c"] = storage.Field{APIName: "Advancement_Namespace__c", Type: storage.FieldString, DefaultValue: `"gem"`}
	org.Objects["Account"] = account
	org.Objects["gem__Advancement_Setting__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "gem__Advancement_Setting__mdt", Fields: map[string]storage.Field{}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeSObjectsUnknownObjectIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	Schema.describeSObjects(new String[]{'Missing__c'});
	System.assert(false, 'expected describe failure');
} catch (Exception e) {
	caught = e.getTypeName() + ':' + e.getMessage();
}
System.assertEquals('System.SObjectException:Schema.describeSObjects unknown object Missing__c', caught);
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

func TestExecSwitchOnDisplayTypeMatchesUnqualifiedCaseNames(t *testing.T) {
	program, err := CompileAnonymous(`
String label = '';
switch on Account.Name.getDescribe().getType() {
	when String {
		label = 'string';
	}
	when Boolean {
		label = 'boolean';
	}
	when else {
		label = 'other';
	}
}

System.assertEquals('string', label);

label = '';
Schema.DisplayType missing = null;
switch on missing {
	when String {
		label = 'string';
	}
	when null {
		label = 'null';
	}
	when else {
		label = 'other';
	}
}
System.assertEquals('null', label);

label = '';
Schema.DescribeFieldResult missingDescribe = null;
switch on missingDescribe?.getType() {
	when String {
		label = 'string';
	}
	when null {
		label = 'null';
	}
	when else {
		label = 'other';
	}
}
System.assertEquals('null', label);

label = '';
Object stringDisplayType = 'Boolean';
switch on stringDisplayType {
	when Boolean {
		label = 'boolean';
	}
	when else {
		label = 'other';
	}
}
System.assertEquals('boolean', label);
System.assertEquals('STRING', Account.Name.getDescribe().getType().name());
System.assertEquals('Boolean', ((String) stringDisplayType).name());
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

func TestExecStandardObjectDescribeMapDoesNotSynthesizeMissingNameField(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, Event.SObjectType.getDescribe().fields.getMap().get('Name'));
System.assertEquals('Subject', Event.SObjectType.getDescribe().fields.getMap().get('Subject').getDescribe().getName());
System.assertEquals('Name', Account.SObjectType.getDescribe().fields.getMap().get('Name').getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Event")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomObjectDescribeMapIncludesOwnerSystemField(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectField ownerField = Widget__c.SObjectType.getDescribe().fields.getMap().get('OwnerId');
System.assertEquals('OwnerId', ownerField.getDescribe().getName());
System.assertEquals('Owner', ownerField.getDescribe().getRelationshipName());
System.assertEquals(User.SObjectType, ownerField.getDescribe().getReferenceTo()[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Widget__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldResultUsesStorageDescribeFlags(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult describe = Account.SObjectType.getDescribe().fields.getMap().get('Hidden__c').getDescribe();
System.assertEquals(false, describe.isNillable());
System.assertEquals(false, describe.isAccessible());
System.assertEquals(false, describe.isCreateable());
System.assertEquals(false, describe.isUpdateable());
System.assertEquals(false, describe.isFilterable());
System.assertEquals(false, describe.isGroupable());
System.assertEquals(false, describe.isSortable());
System.assertEquals(false, describe.isAggregatable());
System.assertEquals(false, describe.isPermissionable());
System.assertEquals(true, describe.isDeprecatedAndHidden());
System.assertEquals(true, describe.isDefaultedOnCreate());
System.assertEquals(2, describe.getRelationshipOrder());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Hidden__c"] = storage.Field{
		APIName:             "Hidden__c",
		Label:               "Hidden",
		Type:                storage.FieldString,
		DisplayType:         "STRING",
		Nillable:            storage.BoolFlag(false),
		DefaultedOnCreate:   storage.BoolFlag(true),
		Accessible:          storage.BoolFlag(false),
		Createable:          storage.BoolFlag(false),
		Updateable:          storage.BoolFlag(false),
		Filterable:          storage.BoolFlag(false),
		Groupable:           storage.BoolFlag(false),
		Sortable:            storage.BoolFlag(false),
		Aggregatable:        storage.BoolFlag(false),
		Permissionable:      storage.BoolFlag(false),
		DeprecatedAndHidden: storage.BoolFlag(true),
		RelationshipOrder:   storage.IntFlag(2),
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldResultUsesCompoundFieldMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult component = Account.SObjectType.getDescribe().fields.getMap().get('ExternalLatitude__c').getDescribe();
Schema.DescribeFieldResult container = Account.SObjectType.getDescribe().fields.getMap().get('ExternalLocation__c').getDescribe();
System.assertEquals('ExternalLocation__c', component.getCompoundFieldName());
System.assertEquals(null, container.getCompoundFieldName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ExternalLocation__c"] = storage.Field{
		APIName:     "ExternalLocation__c",
		Label:       "External Location",
		Type:        storage.FieldLocation,
		DisplayType: "LOCATION",
	}
	account.Definition.Fields["ExternalLatitude__c"] = storage.Field{
		APIName:           "ExternalLatitude__c",
		Label:             "External Latitude",
		Type:              storage.FieldDecimal,
		DisplayType:       "DOUBLE",
		CompoundFieldName: "ExternalLocation__c",
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldNumericAndTextMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult amount = Account.Amount__c.getDescribe();
System.assertEquals(12, amount.getPrecision());
System.assertEquals(2, amount.getScale());
System.assertEquals(12, amount.getDigits());
System.assertEquals(0, amount.getLength());
System.assert(!amount.isHtmlFormatted());
System.assert(amount.isSortable());
Schema.DescribeFieldResult notes = Account.Notes__c.getDescribe();
System.assertEquals(1024, notes.getLength());
System.assertEquals(1024, notes.getByteLength());
System.assertEquals(1024, Schema.SObjectType.Account.fields.Notes__c.getLength());
System.assertEquals(0, notes.getPrecision());
System.assertEquals(0, notes.getScale());
System.assert(!notes.isHtmlFormatted());
System.assert(!notes.isSortable());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY", Precision: 12, Scale: 2}
	account.Definition.Fields["Notes__c"] = storage.Field{APIName: "Notes__c", Type: storage.FieldString, DisplayType: "TEXTAREA", Length: 1024}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldResultFormulaAndCaseMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult formula = Account.Score__c.getDescribe();
System.assert(formula.isCalculated());
System.assertEquals('AnnualRevenue * 2', formula.getCalculatedFormula());
Schema.DescribeFieldResult external = Account.External_Id__c.getDescribe();
System.assert(external.isCaseSensitive());
System.assertEquals('External Id Help', external.getInlineHelpText());
System.assertEquals('External_Id__c', external.getLocalName());
System.assertEquals(Account.External_Id__c, external.getSObjectField());
System.assertEquals(null, external.getReferenceTargetField());
System.assertEquals(0, external.getRelationshipOrder());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Label: "Score", Type: storage.FieldCalculated, DisplayType: "DOUBLE", Formula: "AnnualRevenue * 2"}
	account.Definition.Fields["External_Id__c"] = storage.Field{APIName: "External_Id__c", Label: "External Id", Type: storage.FieldString, DisplayType: "STRING", InlineHelpText: "External Id Help", ExternalID: true, Unique: true, CaseSensitive: true}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldResultMetadataBackedBooleanRows(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeFieldResult status = Account.Status__c.getDescribe();
System.assert(status.isRestrictedPicklist());
Schema.DescribeFieldResult external = Account.External_Id__c.getDescribe();
System.assert(external.isIdLookup());
Schema.DescribeFieldResult owner = Account.OwnerId.getDescribe();
System.assert(owner.isNamePointing());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Status__c"] = storage.Field{APIName: "Status__c", Type: storage.FieldPicklist, DisplayType: "PICKLIST", RestrictedPicklist: true}
	account.Definition.Fields["External_Id__c"] = storage.Field{APIName: "External_Id__c", Type: storage.FieldString, DisplayType: "STRING", ExternalID: true, IDLookup: true}
	account.Definition.Fields["OwnerId"] = storage.Field{APIName: "OwnerId", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User", "Group"}, RelationshipName: "Owner"}
	org.Objects["Account"] = account
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
User userFromConstructor = new User(FirstName = 'Local', LastName = 'User');
System.assert(!userFromConstructor.getPopulatedFieldsAsMap().containsKey('Id'));
TemplateSettings__c settings = (TemplateSettings__c)TemplateSettings__c.SObjectType.newSObject(null, true);
System.assertEquals('resetcss', settings.DefaultCSS__c);
GLAccount__c account = (GLAccount__c)GLAccount__c.SObjectType.newSObject(null, true);
System.assertEquals('Active', account.Status__c);
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
	org.Objects["GLAccount__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "GLAccount__c",
			Fields: map[string]storage.Field{
				"Id":           {APIName: "Id", Type: storage.FieldID},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
				"Status__c": {
					APIName: "Status__c",
					Type:    storage.FieldPicklist,
					PicklistValues: []storage.PicklistValue{
						{Value: "Active", Default: true, Active: true},
						{Value: "Inactive", Active: true},
					},
				},
			},
			RecordTypes: []storage.RecordTypeInfo{{
				ID:            "012000000000002AAA",
				DeveloperName: "Default",
				Name:          "Default",
				Active:        true,
				Available:     true,
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFromValueIgnoresImplicitWrongPrefixSObjectID(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "User")
	machine := New(nil)
	machine.SetOrg(&org)

	user := Object("User")
	user.Fields["Id"] = platformScalar("Id", "a00000000000001")
	user.Fields["LastName"] = String("Local")
	record, err := machine.recordFromValue(&user)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "" {
		t.Fatalf("implicit wrong-prefix ID should be ignored, got %s", record.ID)
	}

	setExplicitSObjectField(&user, "Id", platformScalar("Id", "a00000000000001"))
	record, err = machine.recordFromValue(&user)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "a00000000000001" {
		t.Fatalf("explicit ID should be preserved, got %s", record.ID)
	}
}

func TestDescribeProviderLookupIncludesUserCompatibilityTarget(t *testing.T) {
	program, err := CompileAnonymous(`
List<Schema.SObjectType> targets = External_Provider_Profile__c.Provider_Id__c.getDescribe().getReferenceTo();
System.assert(targets.contains(Contact.SObjectType));
System.assert(targets.contains(User.SObjectType));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["External_Provider_Profile__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "External_Provider_Profile__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Provider_Id__c": {APIName: "Provider_Id__c", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Provider_Id__r"},
				"Provider__c":    {APIName: "Provider__c", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}, RelationshipName: "Provider__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	field, err := machine.describeFieldValue("External_Provider_Profile__c", "Provider__c")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(field.Fields["referenceTo"].List); got != 2 {
		t.Fatalf("Provider__c reference targets = %d", got)
	}
}

func TestStorageValueFromVMForFieldPreservesEmptyStringText(t *testing.T) {
	value, err := storageValueFromVMForField(String(""), storage.FieldString)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != storage.ValueString || value.String != "" {
		t.Fatalf("empty text storage value = %#v", value)
	}

	picklist, err := storageValueFromVMForField(String(""), storage.FieldPicklist)
	if err != nil {
		t.Fatal(err)
	}
	if picklist.Kind != storage.ValueNull {
		t.Fatalf("empty picklist storage value = %#v", picklist)
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

func TestExecListCustomSettingInsertMissingNameDoesNotUseTestNameDefault(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	insert new Relationship_Lookup__c();
	System.assert(false, 'expected missing Name failure');
} catch (DmlException e) {
	System.assert(e.getMessage().contains('Name'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Relationship_Lookup__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Relationship_Lookup__c",
			Label:     "Relationship Lookup",
			KeyPrefix: "a00",
			Metadata:  map[string]string{"kind": "customSetting", "customSettingsType": "List"},
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Label: "Name", Type: storage.FieldString, Required: true},
				"Male__c": {APIName: "Male__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
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

func TestExecDatabaseUpsertWithoutExplicitExternalIDInsertsAndChecksUnique(t *testing.T) {
	program, err := CompileAnonymous(`
Account existing = new Account(Name = 'Acme', External_Key__c = 'ext-1');
insert existing;
Account duplicate = new Account(Name = 'Other', External_Key__c = 'ext-1');
List<Database.UpsertResult> results = Database.upsert(new List<Account>{ duplicate }, false);
System.assertEquals(false, results[0].isSuccess());
System.assertEquals('DUPLICATE_VALUE', String.valueOf(results[0].getErrors()[0].getStatusCode()));
System.assertEquals(null, duplicate.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSetConstructorDeduplicatesEquivalentSObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Account first = new Account(Name = 'Acme', Site = 'North');
Account second = new Account(Site = 'North', Name = 'Acme');
List<Account> rows = new List<Account>{ first, second };
Set<Account> unique = new Set<Account>(rows);
System.assertEquals(1, unique.size());
System.assertEquals(1, new List<Account>(new Set<Account>(rows)).size());
List<Account> aliases = new List<Account>{ first, first };
System.assertEquals(1, new Set<Account>(aliases).size());
Account changing = new Account(Name = 'Before');
List<Account> snapshots = new List<Account>{ changing };
changing.Site = 'After';
snapshots.add(changing);
System.assertEquals(1, new Set<Account>(snapshots).size());
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

func TestExecSOQLBindInvalidStringIdReturnsNoRows(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
String missingId = 'a9302003536469';
List<Account> rows = [SELECT Id FROM Account WHERE Id = :missingId];
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

func TestExecSOQLBindDoesNotSkipBadIDLiteralValidation(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
String name = 'Acme';
Boolean caught = false;
try {
	Database.query('SELECT Id FROM Account WHERE Id = \'bad data dot com\' AND Name = :name');
} catch (QueryException qe) {
	caught = true;
}
System.assert(caught, 'bad ID literal should throw even when another predicate has a bind');
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

func TestExecSOQLBindSObjectListIgnoresNullIds(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
Account missing = new Account();
missing.put('Id', null);
List<Account> accounts = new List<Account>{ missing, account };
List<Account> rows = [SELECT Id, Name FROM Account WHERE Id IN :accounts];
System.assertEquals(1, rows.size());
System.assertEquals(account.Id, rows[0].Id);
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

func TestExecReturnedSObjectListPropagatesNestedRelationshipMutation(t *testing.T) {
	program, err := CompileAnonymous(`
Product__c product = new Product__c(WeightInPounds__c = 2004);
Line__c line = new Line__c(Product2__r = product);
Holder holder = new Holder();
holder.PrivateRows = new List<Line__c>{ line };

List<Line__c> rows = holder.Rows;
rows[0].Product2__r.WeightInPounds__c = null;

System.assertEquals(null, holder.firstWeight());
`)
	if err != nil {
		t.Fatal(err)
	}
	getRows, err := CompileAnonymous("return PrivateRows;")
	if err != nil {
		t.Fatal(err)
	}
	firstWeight, err := CompileAnonymous("return PrivateRows[0].Product2__r.WeightInPounds__c;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "pkg"
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Product__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":                     {APIName: "Id", Type: storage.FieldID},
				"pkg__WeightInPounds__c": {APIName: "pkg__WeightInPounds__c", Type: storage.FieldDecimal},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["pkg__Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Line__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id":               {APIName: "Id", Type: storage.FieldID},
				"pkg__Product2__c": {APIName: "pkg__Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Product__c"}, RelationshipName: "pkg__Product2__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"PrivateRows": {Name: "PrivateRows", Type: "List<Line__c>", Access: "public"},
			"Rows":        {Name: "Rows", Type: "List<Line__c>", Access: "public", Getter: &Method{Name: "Holder.getRows", ClassName: "Holder", ReturnType: "List<Line__c>", Program: getRows}},
		},
		Methods: map[string]Method{
			"getRows":     {Name: "Holder.getRows", ClassName: "Holder", ReturnType: "List<Line__c>", Program: getRows},
			"firstWeight": {Name: "Holder.firstWeight", ClassName: "Holder", ReturnType: "Decimal", Program: firstWeight},
		},
	}); err != nil {
		t.Fatal(err)
	}
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

func TestExecSObjectPutPropagatesThroughObjectFieldAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Account contact = new Account();
Map<String, Object> holder = new Map<String, Object>();
holder.put('Record', contact);
SObject alias = (SObject)holder.get('Record');
alias.put(Account.Name, 'Smith');
System.assertEquals('Smith', contact.Name);
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

func TestExecDMLInsertPropagatesIDThroughObjectAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
List<SObject> accounts = new List<SObject>();
accounts.add(account);
Map<String, Object> aliases = new Map<String, Object>();
aliases.put('account', account);
insert accounts;
System.assert(account.Id != null);
System.assertEquals(account.Id, accounts[0].Id);
SObject alias = (SObject)aliases.get('account');
System.assertEquals(account.Id, alias.Id);
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

func TestExecSObjectRelationshipResolutionThroughObjectMapAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c account = new Parent__c(Name = 'Parent');
Child__c contact = new Child__c(Name = 'Child');
Map<String, Object> relationship = new Map<String, Object>();
relationship.put('Record', contact);
relationship.put('RelatedTo', account);
List<SObject> accounts = new List<SObject>();
accounts.add(account);
insert accounts;
SObject record = (SObject)relationship.get('Record');
SObject relatedTo = (SObject)relationship.get('RelatedTo');
System.assertEquals('Child__c', record.getSObjectType().getDescribe().getName());
System.assertEquals(account.Id, relatedTo.Id);
record.put('Parent__c', relatedTo.Id);
System.assertEquals(account.Id, contact.Parent__c);
insert contact;
Child__c stored = [SELECT Parent__c FROM Child__c WHERE Id = :contact.Id];
System.assertEquals(account.Id, stored.Parent__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedParentRelationshipUsesStoredLookupField(t *testing.T) {
	program, err := CompileAnonymous(`
Grandparent__c grandparent = new Grandparent__c(Name = 'Grand');
insert grandparent;
Parent__c parent = new Parent__c(Name = 'Parent', Grandparent__c = grandparent.Id);
insert parent;
Child__c child = new Child__c(Name = 'Child', Parent__c = parent.Id);
insert child;
Child__c stored = [SELECT Parent__c FROM Child__c WHERE Id = :child.Id];
System.assertEquals('Grand', stored.Parent__r.Grandparent__r.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Grandparent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Grandparent__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":             {APIName: "Id", Type: storage.FieldID},
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"Grandparent__c": {APIName: "Grandparent__c", Type: storage.FieldReference, ReferenceTo: []string{"Grandparent__c"}},
			},
			Relations: []storage.Relationship{
				{Field: "Grandparent__c", ParentObjects: []string{"Grandparent__c"}, ParentRelationship: "Grandparent__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			},
			Relations: []storage.Relationship{
				{Field: "Parent__c", ParentObjects: []string{"Parent__c"}, ParentRelationship: "Parent__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
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
	Schema.SObjectField missingField = null;
	try {
		a.put(missingField, 'ignored');
		System.assert(false, 'null field token should throw');
	} catch (System.NullPointerException e) {
		System.assertEquals('Argument cannot be null.', e.getMessage());
	}
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

func TestExecGetPopulatedFieldsAsMapDoesNotDuplicateNamespaceAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Widget__c widget = new Widget__c(Name = 'Acme', Score__c = 0);
Map<String,Object> populated = widget.getPopulatedFieldsAsMap();
System.assertEquals(2, populated.size(), String.valueOf(populated.keySet()));
System.assert(populated.containsKey('Name'), String.valueOf(populated.keySet()));
System.assert(populated.containsKey('Score__c'), String.valueOf(populated.keySet()));
System.assert(populated.containsKey('verifiable__Score__c'), String.valueOf(populated.keySet()));
System.assertEquals(0, populated.get('verifiable__Score__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "verifiable"
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__c", Fields: map[string]storage.Field{
		"Name":     {APIName: "Name", Type: storage.FieldString},
		"Score__c": {APIName: "Score__c", Type: storage.FieldDecimal},
	}}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomObjectLastActivityDateIsReadOnlyAndReadable(t *testing.T) {
	program, err := CompileAnonymous(`
Widget__c widget = new Widget__c(Name = 'Acme');
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Widget__c.fields.getMap();
System.assertEquals(false, fields.get('lastactivitydate').getDescribe().isUpdateable());
System.assertEquals(null, widget.get('lastactivitydate'));
System.assertEquals(null, widget.get('recordtypeid'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "verifiable"
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__c", Fields: map[string]storage.Field{
		"Name": {APIName: "Name", Type: storage.FieldString},
	}}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomSettingDescribeOmitsCustomObjectOnlyStandardFields(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Schema.SObjectType.Widget__c.fields.getMap();
System.assert(fields.containsKey('Name'), String.valueOf(fields.keySet()));
System.assertEquals(false, fields.containsKey('lastactivitydate'), String.valueOf(fields.keySet()));
System.assertEquals(false, fields.containsKey('recordtypeid'), String.valueOf(fields.keySet()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Widget__c",
		Fields:  map[string]storage.Field{},
		Metadata: map[string]string{
			"kind": "customSetting",
		},
	}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGetPopulatedFieldsAsMapMatchesNamespacedFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
Subscription__c membership = new Subscription__c(Account2__c = account.Id);
insert membership;
Subscription__c queried = [SELECT Id, Account2__c FROM Subscription__c WHERE Id = :membership.Id LIMIT 1];
Map<String,Object> populated = queried.getPopulatedFieldsAsMap();
String accountField = String.valueOf(Subscription__c.Account2__c);
System.assert(populated.containsKey('verifiable__Account2__c'), String.valueOf(populated.keySet()));
System.assert(populated.containsKey(accountField), accountField + ':' + populated.keySet());
System.assertEquals(account.Id, queried.get(accountField));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	org.Objects["Subscription__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Subscription__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
				"Account2__c": {
					APIName:          "Account2__c",
					Type:             storage.FieldReference,
					ReferenceTo:      []string{"Account"},
					RelationshipName: "Account2__r",
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecQueryLocatorPopulatedFieldsMatchesNamespacedFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
Subscription__c membership = new Subscription__c(Account2__c = account.Id);
insert membership;
Database.QueryLocator locator = Database.getQueryLocator('SELECT Id, Account2__c FROM Subscription__c');
Object iterator = locator.iterator();
SObject row = (SObject)iterator.next();
Map<String,Object> populated = row.getPopulatedFieldsAsMap();
String accountField = String.valueOf(Subscription__c.Account2__c);
System.assert(populated.containsKey('verifiable__Account2__c'), String.valueOf(populated.keySet()));
System.assert(populated.containsKey(accountField), accountField + ':' + populated.keySet());
System.assertEquals(account.Id, row.get(accountField));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	org.Objects["Subscription__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Subscription__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
				"Account2__c": {
					APIName:          "Account2__c",
					Type:             storage.FieldReference,
					ReferenceTo:      []string{"Account"},
					RelationshipName: "Account2__r",
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestPopulatedFieldsKeySetContainsNamespaceAlias(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "NU"
	machine.SetOrg(&org)
	keys := Set(String("Account2__c"))
	keys.Runtime = "sobject-populated-fields:Subscription__c:keyset"

	contains, handled := machine.populatedFieldsKeySetContains(keys, String("NU__Account2__c"))
	if !handled || !contains {
		t.Fatalf("contains=%v handled=%v", contains, handled)
	}
}

func TestExecGetPopulatedFieldsAsMapIncludesQueriedNullFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Account queried = [SELECT Id, Name, Phone FROM Account WHERE Id = :a.Id];
Map<String,Object> populated = queried.getPopulatedFieldsAsMap();
System.assert(populated.containsKey('Id'));
System.assert(populated.containsKey('Name'));
System.assert(populated.containsKey('Phone'));
System.assertEquals(null, populated.get('Phone'));
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

func TestExecGetPopulatedFieldsAsMapOmitsDMLAuditFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Map<String,Object> populated = a.getPopulatedFieldsAsMap();
System.assert(populated.containsKey('Id'));
System.assert(populated.containsKey('Name'));
System.assert(!populated.containsKey('CreatedDate'));
System.assert(!populated.containsKey('LastModifiedDate'));
System.assert(!populated.containsKey('SystemModstamp'));
System.assert(!populated.containsKey('OwnerId'));
System.assert(!populated.containsKey('IsDeleted'));
Account queried = [SELECT Id, CreatedDate, SystemModstamp FROM Account WHERE Id = :a.Id];
Map<String,Object> queriedPopulated = queried.getPopulatedFieldsAsMap();
System.assert(queriedPopulated.containsKey('CreatedDate'));
System.assert(queriedPopulated.containsKey('SystemModstamp'));
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

func TestSObjectFieldArgAcceptsUnqualifiedFieldTokenType(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	token := sObjectFieldToken("Account", "Name")
	token.Type = "SObjectField"
	field, err := machine.sObjectFieldArg("Account", token)
	if err != nil {
		t.Fatal(err)
	}
	if field != "Name" {
		t.Fatalf("field = %q, want Name", field)
	}
}

func TestExecClassMethodWinsWhenClassNameResolvesAsSObjectAlias(t *testing.T) {
	getTypeProgram, err := CompileAnonymous("return Coupon__c.SObjectType;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Coupon c = new Coupon();
SObject row = c.getSObjectType().newSObject();
row.put(Coupon__c.Account2__c, '001000000000001AAA');
System.assertEquals(Coupon__c.SObjectType, row.getSObjectType());
System.assertEquals('001000000000001AAA', row.get(Coupon__c.Account2__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Coupon",
		Methods: map[string]Method{
			"getSObjectType": {
				Name:       "Coupon.getSObjectType",
				ClassName:  "Coupon",
				ReturnType: "Schema.SObjectType",
				Program:    getTypeProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Coupon"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Coupon",
		Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
	}}
	org.Objects["Coupon__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Coupon__c",
		Fields: map[string]storage.Field{
			"Id":          {APIName: "Id", Type: storage.FieldID},
			"Account2__c": {APIName: "Account2__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGetSObjectWithLookupFieldTokenRequiresLoadedParent(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = '001000000000001AAA');
Account parent = child.getSObject(Child__c.Parent__c);
System.assertEquals(null, parent);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
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
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGetSObjectWithDerivedParentRelationshipNameRequiresLoadedParent(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = '001000000000001AAA');
Account parent = child.getSObject('Parent__r');
System.assertEquals(null, parent);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Children"},
		},
		Relations: []storage.Relationship{{
			Field:              "Parent__c",
			ParentObjects:      []string{"Account"},
			ParentRelationship: "Children",
		}},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGetSObjectWithLookupFieldTokenPrefersLoadedParent(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c(Parent__c = '001000000000001AAA');
child.put('Parent__r', new Account(Id = '001000000000001AAA', Name = 'Loaded Parent'));
Account parent = child.getSObject(Child__c.Parent__c);
System.assertEquals('Loaded Parent', parent.Name);
System.assertEquals('Parent__r', Child__c.Parent__c.getDescribe().getRelationshipName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
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
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPutSObjectStoresLoadedParentRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Child__c child = new Child__c();
child.putSObject(Child__c.Parent__c, new Account(Id = '001000000000001AAA', Name = 'Token Parent'));
System.assertEquals('Token Parent', child.getSObject(Child__c.Parent__c).Name);
child.putSObject('Parent__r', new Account(Id = '001000000000002AAA', Name = 'String Parent'));
System.assertEquals('String Parent', child.getSObject('Parent__r').Name);
System.assertEquals('String Parent', child.getSobject('Parent__r').Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
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
	}}
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
Account cloneTwoArgs = row.clone(false, true);
System.assertEquals(null, cloneTwoArgs.get('Id'));
Account cloneThreeArgs = row.clone(true, true, false);
System.assertEquals(row.get('Id'), cloneThreeArgs.get('Id'));
Account lowerIdRow = new Account(Name = 'Lower');
lowerIdRow.put('id', '001000000000002');
Account lowerIdClone = lowerIdRow.clone(false, true, false, false);
System.assertEquals(null, lowerIdClone.get('Id'));
System.assertEquals(null, lowerIdClone.get('id'));
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

func TestDeleteObjectFieldRemovesAllCaseVariants(t *testing.T) {
	fields := map[string]Value{
		"Id":   String("001000000000001"),
		"id":   String("001000000000002"),
		"Name": String("Acme"),
	}
	deleteObjectField(fields, "Id")
	if _, ok := fields["Id"]; ok {
		t.Fatalf("Id field remained: %#v", fields)
	}
	if _, ok := fields["id"]; ok {
		t.Fatalf("id field remained: %#v", fields)
	}
	if got := fields["Name"].Text; got != "Acme" {
		t.Fatalf("Name = %q", got)
	}
}

func TestExecSObjectGetSObjectsUsesCanonicalChildRelationshipValue(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Parent');
row.put('Contacts__r', new List<Contact>{new Contact(LastName = 'Child')});
List<Contact> contacts = row.getSObjects('Contacts');
System.assertEquals(1, contacts.size());
System.assertEquals('Child', contacts[0].LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	contact := org.Objects["Contact"]
	contact.Definition.Relations = append(contact.Definition.Relations, storage.Relationship{
		Field:              "AccountId",
		ParentObjects:      []string{"Account"},
		ParentRelationship: "Account",
		ChildRelationship:  "Contacts__r",
	})
	org.Objects["Contact"] = contact
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListRelationshipBookkeeping(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> rows = new List<SObject>();
Account account = new Account(Name = 'Acme');
Contact contact = new Contact(LastName = 'Smith');
rows.addToRelationship(account);
rows.addToRelationship(new List<SObject>{contact});
System.assertEquals(2, rows.getAddedToRelationship().size());
System.assertEquals('Acme', ((Account)rows.getAddedToRelationship().get(0)).Name);
rows.markForDelete(contact);
rows.markForDelete(new List<SObject>{account});
System.assertEquals(2, rows.getMarkedForDeletion().size());
System.assertEquals('Smith', ((Contact)rows.getMarkedForDeletion().get(0)).LastName);
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
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectQuickActionNameAccessor(t *testing.T) {
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Child');
System.assertEquals(null, row.getQuickActionName());
row.put('QuickActionName', 'Account.New');
System.assertEquals('Account.New', row.getQuickActionName());
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

func TestExecNoteParentIdIsWriteable(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Note note = new Note(Title = 'Follow up', Body = 'Call back', ParentId = a.Id);
insert note;
List<Note> rows = [SELECT Id, ParentId, Title, Body FROM Note WHERE Id = :note.Id];
System.assertEquals(1, rows.size());
System.assertEquals(a.Id, rows[0].ParentId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Note")
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
System.assertEquals('012000000000001', business.getRecordTypeID());
System.assert(business.isActive());
System.assert(business.isAvailable());
System.assert(business.isDefaultRecordTypeMapping());
System.assert(!business.isMaster());
Object consumer = byDeveloperName.get('Consumer');
System.assertEquals('Consumer Account', consumer.getName());
System.assertEquals('Consumer', consumer.getDeveloperName());
System.assertEquals('012000000000002', consumer.getRecordTypeId());
System.assertEquals('Business', byId.get('012000000000001').getDeveloperName());
System.assertEquals('Consumer', byId.get('012000000000002').getDeveloperName());
System.assert(!consumer.isActive());
System.assert(!consumer.isAvailable());
System.assert(!consumer.isDefaultRecordTypeMapping());
System.assert(!consumer.isMaster());
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

func TestExecRecordTypeInfoIsMaster(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.RecordTypeInfo master = Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Master');
System.assert(master.isMaster());
System.assertEquals('Master', master.getDeveloperName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{ID: "012000000000000AAA", DeveloperName: "Master", Name: "Master", Active: true, Available: true, Default: true},
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
System.assertEquals(3, byName.size());
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
System.assert(byName.containsKey('Credit Card'));
System.assertEquals('012000000000103', byName.get('Credit Card').getRecordTypeId());
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
				{ID: "012000000000103", DeveloperName: "CreditCard", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeResolvesUnqualifiedCustomObjectInExecutingNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.RecordTypeInfo> byName = Schema.SObjectType.Product__c.getRecordTypeInfosByName();
System.assert(byName.containsKey('Subscription'), 'expected dependency namespace record type');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "namz"
	org.Objects["namz__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "namz__Product__c", Fields: map[string]storage.Field{}},
		Records:    map[storage.ID]storage.Record{},
	}
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Product__c",
			Fields:  map[string]storage.Field{},
			RecordTypes: []storage.RecordTypeInfo{{
				DeveloperName: "Subscription",
				Name:          "Subscription",
				Active:        true,
				Available:     true,
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Harness", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Harness"); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingCustomObjectDescribeUsesExecutingNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = ManagedAppSettings__c.SObjectType.getDescribe();
System.assertEquals('pkg__ManagedAppSettings__c', describe.getName());
System.assertEquals('ManagedAppSettings__c', describe.getLocalName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "namz"
	org.Objects["ManagedAppSettings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "ManagedAppSettings__c", Fields: map[string]storage.Field{}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Harness", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Harness"); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedStringMapLookupUsesExecutingNamespaceAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Boolean> flags = new Map<String, Boolean>();
flags.put('pkg__StoredPaymentMethodsSUM17', true);
System.assert(flags.containsKey('StoredPaymentMethodsSUM17'));
System.assert(flags.containsKey('NU__StoredPaymentMethodsSUM17'));
System.assertEquals(true, flags.get('StoredPaymentMethodsSUM17'));
System.assertEquals(true, flags.get('NU__StoredPaymentMethodsSUM17'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "namz"
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Harness", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Harness"); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedStringMapLookupDoesNotAliasStringValues(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, String> operations = new Map<String, String>();
operations.put('InstallmentPayment', 'Create');
System.assertEquals(false, operations.containsKey('NS__InstallmentPayment'));
System.assertEquals(null, operations.get('NS__InstallmentPayment'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Harness", Namespace: "NU"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Harness"); err != nil {
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
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000101": {Object: "RecordType", ID: "012000000000101"},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertAppliesRecordTypeFormulaDefault(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Scheduled Batch').getRecordTypeId();
insert new Batch__c(Name = 'B', RecordTypeId = recordTypeId);
Batch__c batch = [SELECT Id, TypeName__c FROM Batch__c LIMIT 1];
System.assertEquals('Scheduled Batch', batch.TypeName__c);
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
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
				"TypeName__c":  {APIName: "TypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000101": {Object: "RecordType", ID: "012000000000101"},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRawRecordTypeNameDefaultReadsAsNullBeforeDML(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Scheduled Batch').getRecordTypeId();
Batch__c batch = new Batch__c(TypeName__c = '$RecordType.Name');
System.assertEquals(null, batch.TypeName__c);
batch.RecordTypeId = recordTypeId;
System.assertEquals(null, batch.TypeName__c);
insert batch;
Batch__c stored = [SELECT Id, TypeName__c FROM Batch__c LIMIT 1];
System.assertEquals('Scheduled Batch', stored.TypeName__c);
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
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
				"TypeName__c":  {APIName: "TypeName__c", Type: storage.FieldString, DefaultValue: "$RecordType.Name"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000101": {Object: "RecordType", ID: "012000000000101"},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectGetDoesNotAliasComponentServiceFields(t *testing.T) {
	program, err := CompileAnonymous(`
Settings__c settings = new Settings__c(FooterComponentService__c = 'MockComponentService');
System.assertEquals(null, settings.get('FooterScriptsComponentService__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Settings__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Settings__c",
			Fields: map[string]storage.Field{
				"FooterComponentService__c":        {APIName: "FooterComponentService__c", Type: storage.FieldString},
				"FooterScriptsComponentService__c": {APIName: "FooterScriptsComponentService__c", Type: storage.FieldString},
			},
			Metadata: map[string]string{"kind": "customSetting"},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertAppliesDefaultRecordTypeId(t *testing.T) {
	program, err := CompileAnonymous(`
Batch__c batch = new Batch__c(Name = 'B');
insert batch;
Batch__c stored = [SELECT RecordTypeId FROM Batch__c WHERE Id = :batch.Id];
System.assertEquals('012000000000101', stored.RecordTypeId);
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
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000100", DeveloperName: "Inactive", Name: "Inactive", Active: false, Available: false},
				{ID: "012000000000101", DeveloperName: "Scheduled", Name: "Scheduled Batch", Active: true, Available: true, Default: true},
				{ID: "012000000000102", DeveloperName: "AdHoc", Name: "Ad Hoc", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000101": {Object: "RecordType", ID: "012000000000101"},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertAccountDefaultRecordTypeHonorsPersonSignals(t *testing.T) {
	program, err := CompileAnonymous(`
Account business = new Account(Name = 'Business');
insert business;
Account storedBusiness = [SELECT RecordType.DeveloperName, IsPersonAccount FROM Account WHERE Id = :business.Id];
System.assertEquals('Business_Account', storedBusiness.RecordType.DeveloperName);
System.assertEquals(false, storedBusiness.IsPersonAccount);

Account person = new Account(FirstName = 'Ada', LastName = 'Lovelace', PersonEmail = 'ada@example.invalid');
insert person;
Account storedPerson = [SELECT RecordType.DeveloperName, IsPersonAccount FROM Account WHERE Id = :person.Id];
System.assertEquals('Individual', storedPerson.RecordType.DeveloperName);
System.assertEquals(true, storedPerson.IsPersonAccount);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = append(account.Definition.RecordTypes, storage.RecordTypeInfo{
		ID:            "012000000000003",
		DeveloperName: "Individual",
		Name:          "Individual",
		Active:        true,
		Available:     true,
	})
	org.Objects["Account"] = account
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLFiltersExplicitRecordTypeIdWithNoObjectDefault(t *testing.T) {
	program, err := CompileAnonymous(`
Id cvoRecordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('CVO').getRecordTypeId();
Id internalRecordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Internal').getRecordTypeId();
SObject cvo = Batch__c.SObjectType.newSObject();
cvo.put('Name', 'CVO');
cvo.put('RecordTypeId', cvoRecordTypeId);
SObject blank = Batch__c.SObjectType.newSObject();
blank.put('Name', 'Blank');
insert new List<SObject>{cvo, blank};

List<SObject> cvoRows = Database.query('SELECT Id, RecordTypeId FROM Batch__c WHERE RecordTypeId = :cvoRecordTypeId ORDER BY Name');
System.assertEquals(1, cvoRows.size());
System.assertEquals(cvoRecordTypeId, (Id)cvoRows[0].get('RecordTypeId'));

List<SObject> internalRows = Database.query('SELECT Id, RecordTypeId FROM Batch__c WHERE RecordTypeId = :internalRecordTypeId');
System.assertEquals(0, internalRows.size());

List<SObject> nonCvoRows = Database.query('SELECT Id, Name FROM Batch__c WHERE RecordType.Name != \'CVO\' ORDER BY Name');
System.assertEquals(1, nonCvoRows.size());
System.assertEquals('Blank', (String)nonCvoRows[0].get('Name'));
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
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			},
			Relations: []storage.Relationship{
				{Field: "RecordTypeId", ParentObjects: []string{"RecordType"}, ParentRelationship: "RecordType"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "CVO", Name: "CVO", Active: true, Available: true},
				{ID: "012000000000102", DeveloperName: "Internal", Name: "Internal", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
				"SobjectType":   {APIName: "SobjectType", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000101": {Object: "RecordType", ID: "012000000000101", Fields: map[string]storage.Value{"Name": storage.StringValue("CVO"), "DeveloperName": storage.StringValue("CVO"), "SobjectType": storage.StringValue("Batch__c")}},
			"012000000000102": {Object: "RecordType", ID: "012000000000102", Fields: map[string]storage.Value{"Name": storage.StringValue("Internal"), "DeveloperName": storage.StringValue("Internal"), "SobjectType": storage.StringValue("Batch__c")}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecManagedSOQLFiltersRecordTypeNameWithCurrentPackageObject(t *testing.T) {
	program, err := CompileAnonymous(`
Id manualRecordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Manual').getRecordTypeId();
Entity__c entity = new Entity__c(Name = 'Primary');
insert entity;
Batch__c batch = new Batch__c(Name = 'Manual Batch', Entity__c = entity.Id, Status__c = 'Open', RecordTypeId = manualRecordTypeId);
insert batch;
List<Batch__c> rows = [SELECT Id, RecordType.Name FROM Batch__c WHERE RecordType.Name = 'Manual' AND Status__c = 'Open' AND Entity__c = :entity.Id];
System.assertEquals(1, rows.size());
System.assertEquals('Manual', rows[0].RecordType.Name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["NU__Entity__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "NU__Entity__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["NU__Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "NU__Batch__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"NU__Status__c": {APIName: "NU__Status__c", Type: storage.FieldString},
				"NU__Entity__c": {APIName: "NU__Entity__c", Type: storage.FieldReference, ReferenceTo: []string{"NU__Entity__c"}, RelationshipName: "NU__Entity__r"},
				"RecordTypeId":  {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			},
			Relations: []storage.Relationship{{Field: "RecordTypeId", ParentObjects: []string{"RecordType"}, ParentRelationship: "RecordType"}},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "Manual", Name: "Manual", Active: true, Available: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectClonePreservesStoredRecordTypeID(t *testing.T) {
	program, err := CompileAnonymous(`
Id cvoRecordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('CVO').getRecordTypeId();
insert new Batch__c(Name = 'Source', RecordTypeId = cvoRecordTypeId);
Batch__c source = [SELECT id, Name FROM Batch__c WHERE RecordType.Name = 'CVO' LIMIT 1];
Batch__c cloned = source.clone(false, true, false, false);
cloned.Name = 'Clone';
insert cloned;
Batch__c stored = [SELECT Id, RecordType.Name FROM Batch__c WHERE Name = 'Clone' LIMIT 1];
System.assertEquals('CVO', stored.RecordType.Name);
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
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			},
			Relations: []storage.Relationship{
				{Field: "RecordTypeId", ParentObjects: []string{"RecordType"}, ParentRelationship: "RecordType"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "CVO", Name: "CVO", Active: true, Available: true},
				{ID: "012000000000102", DeveloperName: "Internal", Name: "Internal", Active: true, Available: true, Default: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
				"SobjectType":   {APIName: "SobjectType", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000101": {Object: "RecordType", ID: "012000000000101", Fields: map[string]storage.Value{"Name": storage.StringValue("CVO"), "DeveloperName": storage.StringValue("CVO"), "SobjectType": storage.StringValue("Batch__c")}},
			"012000000000102": {Object: "RecordType", ID: "012000000000102", Fields: map[string]storage.Value{"Name": storage.StringValue("Internal"), "DeveloperName": storage.StringValue("Internal"), "SobjectType": storage.StringValue("Batch__c")}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDynamicSOQLFiltersRecordTypeAndCreatedByToken(t *testing.T) {
	program, err := CompileAnonymous(`
Id cvoRecordTypeId = Batch__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('CVO').getRecordTypeId();
System.assertEquals(cvoRecordTypeId, Batch__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('CVO').recordTypeId);
Id providerId = UserInfo.getUserId();
SObject cvo = Batch__c.SObjectType.newSObject();
cvo.put('Name', 'CVO');
cvo.put('RecordTypeId', cvoRecordTypeId);
insert cvo;
Set<Id> providerIdsSet = new Set<Id>{ providerId };
String query = 'SELECT Id, CreatedById, RecordTypeId FROM Batch__c WHERE ' +
	Batch__c.CreatedById +
	' != NULL AND ' +
	Batch__c.CreatedById +
	' IN :providerIdsSet AND RecordTypeId = :cvoRecordTypeId';
List<SObject> rows = Database.query(query);
System.assertEquals(1, rows.size());
System.assertEquals(providerId, (Id)rows[0].get('CreatedById'));
System.assertEquals(cvoRecordTypeId, (Id)rows[0].get('RecordTypeId'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			},
			Relations: []storage.Relationship{
				{Field: "RecordTypeId", ParentObjects: []string{"RecordType"}, ParentRelationship: "RecordType"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "CVO", Name: "CVO", Active: true, Available: true},
				{ID: "012000000000102", DeveloperName: "Internal", Name: "Internal", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	batch := org.Objects["Batch__c"]
	storage.EnsureStandardObjectFields(&batch.Definition)
	org.Objects["Batch__c"] = batch
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBeforeInsertTriggerPreservesRecordTypeId(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Batch__c batch : Trigger.new) {
	batch.Name = 'Touched';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Batch__c batch = new Batch__c(Name = 'Initial', RecordTypeId = '012000000000101');
insert batch;
Batch__c stored = [SELECT Name, RecordTypeId FROM Batch__c WHERE Id = :batch.Id][0];
System.assertEquals('Touched', stored.Name);
System.assertEquals('012000000000101', stored.RecordTypeId);
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
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
			},
			RecordTypes: []storage.RecordTypeInfo{
				{ID: "012000000000101", DeveloperName: "CVO", Name: "CVO", Active: true, Available: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "BatchBeforeInsert",
		Object:    "Batch__c",
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

func TestExecSObjectFieldTokenConcatenatesAsFieldName(t *testing.T) {
	program, err := CompileAnonymous(`
String condition = Account.CreatedById + ' != NULL';
System.assertEquals('CreatedById != NULL', condition);
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

func TestRelationshipTargetsObjectMatchesNamespacedObjectName(t *testing.T) {
	relationship := storage.Relationship{ParentObjects: []string{"Education__c"}}
	if !relationshipTargetsObject(relationship, "verifiable__Education__c") {
		t.Fatal("expected local parent target to match namespaced object API name")
	}

	relationship = storage.Relationship{ParentObjects: []string{"verifiable__Account"}}
	if !relationshipTargetsObject(relationship, "Account") {
		t.Fatal("expected namespaced standard parent target to match standard object API name")
	}
}

func TestDescribeChildRelationshipNameUsesNamespacedCustomRelationshipAPIName(t *testing.T) {
	got := describeChildRelationshipName("verifiable", "EducationDegree__c", "EducationDegrees")
	if got != "verifiable__EducationDegrees__r" {
		t.Fatalf("relationship name = %q, want verifiable__EducationDegrees__r", got)
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

func TestExecChildRelationshipAccessAcceptsRuntimeSuffixAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c parent = new Parent__c(Name = 'P');
insert parent;
insert new Child__c(Name = 'C', Parent__c = parent.Id);
Parent__c queried = [SELECT Id, (SELECT Id, Name FROM Children) FROM Parent__c WHERE Id = :parent.Id];
System.assertEquals(1, queried.Children__r.size());
System.assertEquals('C', queried.Children__r[0].Name);
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
				ChildRelationship:  "Children",
			}},
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

func TestExecRecordTypeRelationshipSynthesizesFromObjectMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Id couponTypeId = Product__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Coupon').getRecordTypeId();
Product__c inMemory = new Product__c(RecordTypeId = couponTypeId);
System.assertEquals(null, inMemory.RecordType.Name);
Product__c explicitRelationship = new Product__c(RecordTypeId = couponTypeId);
explicitRelationship.RecordType = new RecordType(Name = 'Explicit');
System.assertEquals('Explicit', explicitRelationship.RecordType.Name);
Product__c insertedRelationship = new Product__c(Name = 'Inserted Coupon', RecordTypeId = couponTypeId);
insert insertedRelationship;
System.assertEquals(null, insertedRelationship.RecordType.Name);
insert new Product__c(Name = 'Coupon Product', RecordTypeId = couponTypeId);
Product__c product = [SELECT Id, RecordType.Name FROM Product__c LIMIT 1];
System.assertEquals('Coupon', product.RecordType.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Product__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Product__c",
		KeyPrefix: "a02",
		Fields: map[string]storage.Field{
			"Id":           {APIName: "Id", Type: storage.FieldID},
			"Name":         {APIName: "Name", Type: storage.FieldString},
			"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"},
		},
		Relations: []storage.Relationship{{
			Field:              "RecordTypeId",
			ParentObjects:      []string{"RecordType"},
			ParentRelationship: "RecordType",
		}},
		RecordTypes: []storage.RecordTypeInfo{{
			ID:            "012000000000010AAA",
			Name:          "Coupon",
			DeveloperName: "Coupon",
		}},
	}}
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":            {APIName: "Id", Type: storage.FieldID},
				"Name":          {APIName: "Name", Type: storage.FieldString},
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000010AAA": {Object: "RecordType", ID: "012000000000010AAA"},
		},
	}
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
Schema.SObjectType packageLicenseType = Schema.getGlobalDescribe().get('PackageLicense');
System.assertNotEquals(null, packageLicenseType);
System.assertEquals('PackageLicense', packageLicenseType.getDescribe().getName());
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "PackageLicense")
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
	System.assertEquals('Account Name', nameField.label);
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
System.assert(Schema.SObjectType.Account.isAccessible());
System.assert(Schema.SObjectType.Account.isCreateable());
System.assert(Schema.SObjectType.Account.isUpdateable());
System.assert(Schema.SObjectType.Account.isDeletable());
System.assert(Schema.SObjectType.Account.isQueryable());
System.assert(Schema.SObjectType.Account.isSearchable());
System.assert(!Schema.SObjectType.Account.isCustom());
System.assert(SObjectType.Account.isAccessible());
System.assert(SObjectType.Account.isCreateable());
System.assert(SObjectType.Account.isUpdateable());
System.assert(SObjectType.Account.isDeletable());
List<String> names = new List<String>{'Account'};
List<Object> describedByName = Schema.describeSObjects(names);
System.assertEquals(1, describedByName.size());
Object describedAccount = describedByName.get(0);
System.assertEquals('Account', describedAccount.getName());
List<Object> childRelationships = schemaDescribe.getChildRelationships();
Object contacts = null;
for (Object childRelationship : childRelationships) {
  if (childRelationship.getRelationshipName() == 'Contacts') {
    contacts = childRelationship;
  }
}
System.assertNotEquals(null, contacts);
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
System.assert(contacts.isRestrictedDelete());
System.assert(contacts.isDeprecatedAndHidden());
System.assertEquals(1, contacts.getJunctionIdListNames().size());
System.assertEquals('AccountContactRelationIds', contacts.getJunctionIdListNames().get(0));
System.assertEquals(1, contacts.getJunctionReferenceTo().size());
System.assertEquals('AccountContactRelation', contacts.getJunctionReferenceTo().get(0).getDescribe().getName());
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
				Field:               "AccountId",
				ParentObjects:       []string{"Account"},
				ParentRelationship:  "Account",
				ChildRelationship:   "Contacts",
				CascadeDelete:       true,
				RestrictedDelete:    true,
				DeprecatedAndHidden: true,
				JunctionIDListNames: []string{"AccountContactRelationIds"},
				JunctionReferenceTo: []string{"AccountContactRelation"},
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["AccountContactRelation"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "AccountContactRelation", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}},
		Records:    map[storage.ID]storage.Record{},
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

func TestExecSObjectDynamicFieldNamesAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account();
account.put('name', 'Acme');
System.assert(account.isSet('NAME'));
System.assertEquals('Acme', account.get('Name'));
System.assertEquals('Acme', account.Name);
account.Name = 'Changed';
System.assertEquals('Changed', account.get('name'));
account.put(Schema.SObjectType.Account.fields.Name, 'Token');
System.assertEquals('Token', account.get('NAME'));
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

func TestExecJSONDeserializedSObjectDynamicAddressGet(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('ShippingPostalCode', Account.ShippingPostalCode.getDescribe().getName());
Account account = (Account)JSON.deserialize('{"ShippingStreet":"Test Street","ShippingCity":"Test City","ShippingState":"Test State","ShippingPostalCode":"60611","ShippingCountry":"United States"}', Account.class);
System.assertEquals('Test Street', account.get('ShippingStreet'));
System.assertEquals('United States', account.get('ShippingCountry'));
System.assertEquals(account.ShippingStreet, account.get('ShippingStreet'));
Map<String, Object> fields = new Map<String, Object>{ 'ShippingPostalCode' => '60611', 'ShippingCountry' => 'United States' };
Account fromMapJSON = (Account)JSON.deserialize(JSON.serialize(fields), Account.class);
System.assertEquals('60611', fromMapJSON.get('ShippingPostalCode'));
System.assertEquals('United States', fromMapJSON.get('ShippingCountry'));
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

func TestExecSObjectGetSynthesizesCompoundAddress(t *testing.T) {
	program, err := CompileAnonymous(`
Contact contact = new Contact(LastName = 'Trail');
contact.put('MailingStreet', '1234 Main');
contact.put('MailingCity', 'Austin');
contact.put('MailingState', 'Texas');
contact.put('MailingPostalCode', '73301');
System.assertEquals('1234 Main', contact.get('MailingStreet'));
System.Address address = (System.Address)contact.get('MailingAddress');
System.assertEquals('1234 Main', address.street);
System.assertEquals('1234 Main', address.getStreet());
System.assertEquals('Austin', address.getCity());
System.assertEquals('Texas', address.getState());
System.assertEquals('73301', address.getPostalCode());
Contact projected = (Contact)JSON.deserialize('{"MailingAddress":null,"MailingStreet":"500 Market","MailingPostalCode":"94105"}', Contact.class);
System.Address projectedAddress = (System.Address)projected.get('MailingAddress');
System.assertEquals('500 Market', projectedAddress.getStreet());
System.assertEquals('94105', projectedAddress.getPostalCode());
insert contact;
Contact queried = [SELECT Id, MailingAddress FROM Contact WHERE Id = :contact.Id];
System.Address queriedAddress = (System.Address)queried.get('MailingAddress');
System.assertEquals('1234 Main', queriedAddress.getStreet());
System.assertEquals('Austin', queriedAddress.getCity());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAddressFluentSettersPopulateComponents(t *testing.T) {
	program, err := CompileAnonymous(`
Address address = Address.newInstance()
    .withStreet('12 Lake Road')
    .withCity('Port Alsworth')
    .withState('Alaska')
    .withPostalCode('99653')
    .withCountry('United States');
System.assertEquals('12 Lake Road', address.getStreet());
System.assertEquals('Port Alsworth', address.getCity());
System.assertEquals('Alaska', address.getState());
System.assertEquals('99653', address.getPostalCode());
System.assertEquals('United States', address.getCountry());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedAddressClassWinsOverPlatformAddressForUnqualifiedStaticCall(t *testing.T) {
	newInstanceProgram, err := CompileAnonymous(`
Address address = new Address();
address.Street = 'custom';
return address;
`)
	if err != nil {
		t.Fatal(err)
	}
	runProgram, err := CompileAnonymous(`
Address custom = Address.newInstance();
System.assertEquals('custom', custom.Street);
System.Address platformAddress = System.Address.newInstance();
System.assertEquals(null, platformAddress.getStreet());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Address",
		Namespace: "pkg",
		Fields: map[string]Field{
			"Street": {Name: "Street", Type: "String"},
		},
		Methods: map[string]Method{
			"newInstance": {Name: "Address.newInstance", ClassName: "Address", ReturnType: "Address", IsStatic: true, Access: "global", Program: newInstanceProgram},
		},
		Access: "global",
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Runner",
		Namespace: "pkg",
		Methods: map[string]Method{
			"run": {Name: "Runner.run", ClassName: "Runner", ReturnType: "void", IsStatic: true, Access: "global", Program: runProgram},
		},
		Access: "global",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.call("Runner.run", nil, nil, &Result{}); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLCompoundAddressSupportsDirectFieldAccess(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(
	Name = 'Acme',
	BillingStreet = '12 Lake Road',
	BillingCity = 'Port Alsworth',
	BillingState = 'Alaska',
	BillingPostalCode = '99653',
	BillingCountry = 'United States'
);
insert account;
Account queried = [SELECT Id, BillingAddress FROM Account WHERE Id = :account.Id LIMIT 1];
System.assertEquals('12 Lake Road', queried.BillingAddress.street);
System.assertEquals('Port Alsworth', queried.BillingAddress.getCity());
System.assertEquals('Alaska', queried.BillingAddress.state);
System.assertEquals('99653', queried.BillingAddress.postalCode);
System.assertEquals('United States', queried.BillingAddress.country);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAddressCastWorksWhenUserAddressClassExists(t *testing.T) {
	program, err := CompileAnonymous(`
Contact contact = new Contact(LastName = 'Trail');
contact.put('MailingStreet', '1234 Main');
Object raw = contact.get('MailingAddress');
System.Address address = (System.Address)raw;
System.assertEquals('1234 Main', address.getStreet());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Address"}); err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCoerceCastAcceptsSystemQualifiedPlatformObject(t *testing.T) {
	machine := New(nil)
	machine.currentClass = "MergeAccountsController"
	if err := machine.RegisterClass(Class{Name: "MergeAccountsController.Address"}); err != nil {
		t.Fatal(err)
	}
	address := Object("Address")
	address.Fields["street"] = String("1234 Main")
	coerced, err := machine.coerceCast("System.Address", address)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != "Address" || coerced.Fields["street"].Text != "1234 Main" {
		t.Fatalf("coerced = %#v", coerced)
	}
}

func TestValueWithTypesResolvedInClassPreservesSystemQualifiedPlatformObject(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "MergeAccountsController.Address"}); err != nil {
		t.Fatal(err)
	}
	address := Object("Address")
	address.Static = "System.Address"
	address.Fields["street"] = String("1234 Main")
	resolved := machine.valueWithTypesResolvedInClass("MergeAccountsController.Address", address)
	if resolved.Type != "Address" || resolved.Static != "System.Address" {
		t.Fatalf("resolved = %#v", resolved)
	}
	coerced, err := machine.coerceAssignable("System.Address", resolved)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != "Address" || coerced.Fields["street"].Text != "1234 Main" {
		t.Fatalf("coerced = %#v", coerced)
	}
}

func TestValueWithTypesResolvedInClassPreservesSchemaQualifiedPlatformObject(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "MergeAccountsController.FieldSetMember"}); err != nil {
		t.Fatal(err)
	}
	member := Object("Schema.FieldSetMember")
	member.Static = "Schema.FieldSetMember"
	member.Fields["fieldPath"] = String("Name")
	resolved := machine.valueWithTypesResolvedInClass("MergeAccountsController", member)
	if resolved.Type != "Schema.FieldSetMember" || resolved.Static != "Schema.FieldSetMember" {
		t.Fatalf("resolved = %#v", resolved)
	}
	coerced, err := machine.coerceAssignable("Schema.FieldSetMember", resolved)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != "Schema.FieldSetMember" || coerced.Fields["fieldPath"].Text != "Name" {
		t.Fatalf("coerced = %#v", coerced)
	}
}

func TestResolveTypeNameInClassPrefersSchemaFieldSetMemberOutsideLexicalNestedOwner(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "MergeAccountsController.FieldSetMember"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "FieldSetService"}); err != nil {
		t.Fatal(err)
	}
	if got := machine.resolveTypeNameInClass("FieldSetService", "FieldSetMember"); got != "Schema.FieldSetMember" {
		t.Fatalf("FieldSetService FieldSetMember resolved to %q", got)
	}
	if got := machine.resolveTypeNameInClass("MergeAccountsController", "FieldSetMember"); got != "MergeAccountsController.FieldSetMember" {
		t.Fatalf("MergeAccountsController FieldSetMember resolved to %q", got)
	}
}

func TestExecJSONDeserializedParentRelationshipPopulatesLookup(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Id = '001000000000001AAA', Name = 'Parent');
Child__c child = (Child__c)JSON.deserialize(JSON.serialize(new Map<String, Object>{ 'Parent__r' => parent }), Child__c.class);
System.assertEquals(parent.Id, child.Parent__c);
System.assertEquals(parent.Id, child.Parent__r.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Child__c"] = storage.ObjectState{
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
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLConvertsEmptyTextToNull(t *testing.T) {
	program, err := CompileAnonymous(`
Widget__c widget = new Widget__c(Name = 'Widget', Note__c = '');
insert widget;
Widget__c loaded = [SELECT Note__c FROM Widget__c WHERE Id = :widget.Id];
System.assertEquals(null, loaded.Note__c);
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a42",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Type: storage.FieldString},
				"Note__c": {APIName: "Note__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLStoresIdAssignedToTextFieldAsComparableText(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
ActionRequest__c request = new ActionRequest__c(SourceRecordId__c = account.Id);
insert request;
System.assertEquals(1, [SELECT COUNT() FROM ActionRequest__c WHERE SourceRecordId__c = :account.Id]);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	org.Objects["ActionRequest__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "ActionRequest__c", KeyPrefix: "a00", Fields: map[string]storage.Field{"SourceRecordId__c": {APIName: "SourceRecordId__c", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUnqueriedChildRelationshipHydratesFromOrg(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c parent = new Parent__c(Name = 'P');
insert parent;
insert new Child__c(Name = 'C', Parent__c = parent.Id);
Parent__c queried = [SELECT Id FROM Parent__c WHERE Id = :parent.Id];
System.assertEquals(1, queried.Children__r.size());
System.assertEquals('C', queried.Children__r[0].Name);
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
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if len(machine.lazyChildRelCache) == 0 {
		t.Fatalf("lazy child relationship lookup cache was not populated")
	}
}

func TestExecGetSObjectsDoesNotHydrateUnqueriedChildRelationship(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c parent = new Parent__c(Name = 'P');
insert parent;
insert new Child__c(Name = 'C', Parent__c = parent.Id);
Parent__c queried = [SELECT Id FROM Parent__c WHERE Id = :parent.Id];
System.assertEquals(0, queried.getSObjects('Children__r').size());
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

func TestExecQueriedChildRelationshipMapExposesRuntimeSuffixAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c parent = new Parent__c(Name = 'P');
insert parent;
insert new Child__c(Name = 'C', Parent__c = parent.Id);
Parent__c queried = [SELECT Id, (SELECT Id, Name FROM Children) FROM Parent__c WHERE Id = :parent.Id];
Map<String,Object> populated = queried.getPopulatedFieldsAsMap();
System.assert(populated.containsKey('Children__r'));
System.assertEquals(1, ((List<Child__c>)populated.get('Children__r')).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a00",
			Fields:    map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
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
				ChildRelationship:  "Children",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypedNullParentRelationshipFieldAccessReturnsNull(t *testing.T) {
	program, err := CompileAnonymous(`
Contact contactRecord;
contactRecord = new Contact();
System.assertEquals(null, contactRecord.Account.Name);
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
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
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

func TestExecTypedNullSObjectVariableFieldAccessThrows(t *testing.T) {
	program, err := CompileAnonymous(`
Account accountRecord;
String name = accountRecord.Name;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil {
		t.Fatal("expected null SObject variable dereference to fail")
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

func TestExecWithSharingHonorsPublicReadObjectSharingModel(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(3, [SELECT Id FROM Account].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.SharingModel = "ReadWrite"
	account.Records = map[storage.ID]storage.Record{
		"001000000000001": {ID: "001000000000001", Object: "Account", System: storage.SystemFields{OwnerID: "005000000000001"}},
		"001000000000002": {ID: "001000000000002", Object: "Account", System: storage.SystemFields{OwnerID: "005000000000001"}},
		"001000000000003": {ID: "001000000000003", Object: "Account", System: storage.SystemFields{OwnerID: "005000000000002"}},
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentClass = "SharingProbe"
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecWithSharingTreatsCampaignAsPublicReadByDefault(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005000000000999')) {
	System.assertEquals(2, [SELECT Id FROM Campaign].size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Campaign")
	campaign := org.Objects["Campaign"]
	campaign.Records = map[storage.ID]storage.Record{
		"701000000000001": {ID: "701000000000001", Object: "Campaign", System: storage.SystemFields{OwnerID: "005000000000001"}},
		"701000000000002": {ID: "701000000000002", Object: "Campaign", System: storage.SystemFields{OwnerID: "005000000000002"}},
	}
	org.Objects["Campaign"] = campaign
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
		t.Fatal(err)
	}
}

func TestExecWithSharingPortalUserSeesContactAccount(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(1, [SELECT Id FROM Account].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	account.Records = map[storage.ID]storage.Record{
		"001000000000001": {ID: "001000000000001", Object: "Account", System: storage.SystemFields{OwnerID: "005000000000001"}},
		"001000000000002": {ID: "001000000000002", Object: "Account", System: storage.SystemFields{OwnerID: "005000000000001"}},
	}
	org.Objects["Account"] = account
	contact := org.Objects["Contact"]
	contact.Records = map[storage.ID]storage.Record{
		"003000000000001": {
			ID:     "003000000000001",
			Object: "Contact",
			Fields: map[string]storage.Value{
				"AccountId": storage.IDValue("001000000000002"),
			},
		},
	}
	org.Objects["Contact"] = contact
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{
		ID:     "005000000000002",
		Object: "User",
		Fields: map[string]storage.Value{
			"Id":        storage.IDValue("005000000000002"),
			"ContactId": storage.IDValue("003000000000001"),
		},
	})
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestContextWithSharingSeesRunAsOwnedTestData(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(2, [SELECT Id FROM Widget__c].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:      "Widget__c",
			KeyPrefix:    "a00",
			SharingModel: "Private",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000001"}},
			"a00000000000002": {ID: "a00000000000002", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000002"}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentClass = "SharingProbe"
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecWithSharingRunAsSystemAdministratorBypassesPrivateSharing(t *testing.T) {
	program, err := CompileAnonymous(`
Profile admin = [SELECT Id FROM Profile WHERE Name = 'System Administrator'];
User u = new User(
	Username = 'sharing-admin@example.invalid',
	Alias = 'sadmin',
	Email = 'sharing-admin@example.invalid',
	LastName = 'Admin',
	ProfileId = admin.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
System.runAs(u) {
	System.assertEquals(2, [SELECT Id FROM Widget__c].size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:      "Widget__c",
			KeyPrefix:    "a00",
			SharingModel: "Private",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000001"}},
			"a00000000000002": {ID: "a00000000000002", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000002"}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
		t.Fatal(err)
	}
}

func TestExecWithoutSharingCalleeOverridesWithSharingCaller(t *testing.T) {
	selectorProgram, err := CompileAnonymous(`
return Database.query('SELECT Id FROM Widget__c ORDER BY Name');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(2, Selector.rows().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:      "Widget__c",
			KeyPrefix:    "a00",
			SharingModel: "Private",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Owned")}},
			"a00000000000002": {ID: "a00000000000002", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000002"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Other")}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000001")}})
	if err := machine.RegisterClass(Class{Name: "Controller", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Selector", Modifiers: []string{"without sharing"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Selector.rows", ClassName: "Selector", IsStatic: true, ReturnType: "List<SObject>", Program: selectorProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Controller"); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentClassSharingModeUsesNearestExplicitFrame(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Controller", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Selector", Modifiers: []string{"without sharing"}}); err != nil {
		t.Fatal(err)
	}
	machine.callStack = []callFrame{
		{Symbol: "Controller.run"},
		{Symbol: "Selector.rows"},
	}
	if machine.currentClassHasSharingMode("with sharing") {
		t.Fatal("with sharing leaked past explicit without sharing callee")
	}
}

func TestExecRunAsInsertPreservesExistingCustomObjectRows(t *testing.T) {
	program, err := CompileAnonymous(`
insert new List<Widget__c>{new Widget__c(Name = 'one'), new Widget__c(Name = 'two')};
User u = [SELECT Id FROM User WHERE Id != :UserInfo.getUserId() LIMIT 1];
System.runAs(u) {
	insert new Widget__c(Name = 'three');
}
System.assertEquals(3, [SELECT Id FROM Widget__c].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:      "Widget__c",
			KeyPrefix:    "a00",
			SharingModel: "ReadWrite",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	users := org.Objects["User"]
	users.Records["005000000000002"] = storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000002")}}
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsUserCanQueryCurrentUserWithSharing(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
User u = new User(
	Username = 'visible-user@example.invalid',
	Alias = 'vuser',
	Email = 'visible-user@example.invalid',
	LastName = 'Visible',
	ProfileId = p.Id,
	TimeZoneSidKey = 'UTC',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	EmailEncodingKey = 'UTF-8'
);
insert u;
System.runAs(u) {
	User row = [SELECT Id FROM User WHERE Id = :UserInfo.getUserId() LIMIT 1];
	System.assertEquals(UserInfo.getUserId(), row.Id);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsUserCanQueryOwnContactAccountWithSharing(t *testing.T) {
	program, err := CompileAnonymous(`
User u = [SELECT Id, ContactId FROM User WHERE Id = '005000000000777' LIMIT 1];
System.runAs(u) {
	System.assertEquals('005000000000777', UserInfo.getUserId());
	List<Account> rows = [SELECT Id FROM Account WHERE Id = '001000000000777'];
	System.assertEquals(1, rows.size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	account.Records["001000000000777"] = storage.Record{
		ID:     "001000000000777",
		Object: "Account",
		System: storage.SystemFields{OwnerID: "005000000000001"},
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Member Account"),
		},
	}
	org.Objects["Account"] = account
	contact := org.Objects["Contact"]
	if contact.Records == nil {
		contact.Records = make(map[storage.ID]storage.Record)
	}
	contact.Records["003000000000777"] = storage.Record{
		ID:     "003000000000777",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"AccountId": storage.IDValue("001000000000777"),
			"LastName":  storage.StringValue("Member"),
		},
	}
	org.Objects["Contact"] = contact
	user := org.Objects["User"]
	if user.Definition.Fields == nil {
		user.Definition.Fields = make(map[string]storage.Field)
	}
	user.Definition.Fields["ContactId"] = storage.Field{APIName: "ContactId", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}}
	if user.Records == nil {
		user.Records = make(map[storage.ID]storage.Record)
	}
	user.Records["005000000000777"] = storage.Record{
		ID:     "005000000000777",
		Object: "User",
		Fields: map[string]storage.Value{
			"Id":        storage.IDValue("005000000000777"),
			"ContactId": storage.IDValue("003000000000777"),
			"Username":  storage.StringValue("member@example.test"),
		},
	}
	org.Objects["User"] = user
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
		t.Fatal(err)
	}
}

func TestSOQLSharingAllowsCurrentUsersContactAccount(t *testing.T) {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	accountRecord := storage.Record{
		ID:     "001000000000777",
		Object: "Account",
		System: storage.SystemFields{OwnerID: "005000000000001"},
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Member Account"),
		},
	}
	account.Records["001000000000777"] = accountRecord
	org.Objects["Account"] = account
	contact := org.Objects["Contact"]
	if contact.Records == nil {
		contact.Records = make(map[storage.ID]storage.Record)
	}
	contact.Records["003000000000777"] = storage.Record{
		ID:     "003000000000777",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"AccountId": storage.IDValue("001000000000777"),
			"LastName":  storage.StringValue("Member"),
		},
	}
	org.Objects["Contact"] = contact
	userRecord := storage.Record{
		ID:     "005000000000777",
		Object: "User",
		Fields: map[string]storage.Value{
			"Id":        storage.IDValue("005000000000777"),
			"ContactId": storage.IDValue("003000000000777"),
			"Username":  storage.StringValue("member@example.test"),
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.RunAsDepth = 1
	machine.testContext.CurrentUser = vmValueFromRecord(userRecord)
	machine.currentClass = "SharingProbe"
	if err := machine.RegisterClass(Class{Name: "SharingProbe", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if !machine.currentClassHasSharingMode("with sharing") {
		t.Fatal("test setup lost with sharing mode")
	}
	if machine.soqlObjectHasPublicReadSharing("Account") {
		t.Fatal("test setup left Account publicly readable")
	}
	result := soql.Result{Rows: 1, Records: []storage.Record{accountRecord}}
	filtered := machine.applySOQLSharing(soql.Query{Object: "Account"}, result)
	if len(filtered.Records) != 1 {
		t.Fatalf("filtered records = %d, want current user's contact account visible", len(filtered.Records))
	}
}

func TestExecBeforeTriggerPreservesExplicitNullForDefaultedField(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Widget__c row : Trigger.new) {
	Date observed = row.RenewalDate__c;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Widget__c row = new Widget__c(Name = 'Explicit Null', RenewalDate__c = null);
insert row;
Widget__c stored = [SELECT Id, RenewalDate__c FROM Widget__c WHERE Id = :row.Id];
System.assertEquals(null, stored.RenewalDate__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"RenewalDate__c": {APIName: "RenewalDate__c", Type: storage.FieldDate, DefaultValue: "TODAY()"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{Name: "WidgetBeforeInsert", Object: "Widget__c", Timing: triggerTimingBefore, Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTriggerUsesTriggerNamespaceForPublicHandlerConstruction(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
new ProductTriggerHandlers();
`)
	if err != nil {
		t.Fatal(err)
	}
	runProgram, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
return 'ok';
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`return namz.Runner.run();`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "ProductTriggerHandlers", Namespace: "pkg", Access: "public"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Runner",
		Namespace: "namz",
		Access:    "global",
		Methods: map[string]Method{
			"run": {Name: "Runner.run", ClassName: "Runner", ReturnType: "String", Access: "global", IsStatic: true, Program: runProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "ProductTrigger", Namespace: "pkg", Object: "Account", Timing: triggerTimingBefore, Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedSObjectFieldMapIncludesBareFieldAlias(t *testing.T) {
	program, err := CompileAnonymous(`
String fieldName = Schema.SObjectType.pkg__Subscription__c.Fields.Account2__c.Name;
System.assertEquals('pkg__Account2__c', fieldName);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Namespace = "namz"
	org.Objects["pkg__Subscription__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Subscription__c",
			Fields: map[string]storage.Field{
				"pkg__Account2__c": {APIName: "pkg__Account2__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
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

func TestExecDatabaseUserModeChecksLocalObjectAndFieldPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
Database.SaveResult result = Database.update(new Account(Id = '001000000000901AAA', ShippingStreet = 'Baker'), AccessLevel.USER_MODE);
System.assert(result.isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}

	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	account := org.Objects["Account"]
	account.Definition.Fields["ShippingStreet"] = storage.Field{APIName: "ShippingStreet", Type: storage.FieldString}
	account.Records["001000000000901AAA"] = storage.Record{
		ID:     "001000000000901AAA",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}
	org.Objects["Account"] = account

	user := Object("User")
	user.Fields["Id"] = String("005000000000998")
	user.Fields["profileId"] = String("00e000000000002")

	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = user
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "Access to entity 'Account' denied") {
		t.Fatalf("err = %v, want Account access denial", err)
	}

	storage.EnsureStandardObject(&org, "PermissionSetAssignment")
	org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{
		ID:     "110000000000998",
		Object: "ObjectPermissions",
		Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue("0PS000000000998"),
			"SObjectType":     storage.StringValue("Account"),
			"PermissionsRead": storage.BooleanValue(true),
			"PermissionsEdit": storage.BooleanValue(true),
		},
	}
	org.Objects["PermissionSetAssignment"].Records["0Pa000000000998"] = storage.Record{
		ID:     "0Pa000000000998",
		Object: "PermissionSetAssignment",
		Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue("005000000000998"),
			"PermissionSetId": storage.IDValue("0PS000000000998"),
		},
	}
	machine = New(nil)
	machine.SetOrg(&org)
	machine.executionUser = user
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "Access to field 'Account.ShippingStreet' denied") {
		t.Fatalf("err = %v, want ShippingStreet access denial", err)
	}

	org.Objects["FieldPermissions"].Records["0FP000000000998"] = storage.Record{
		ID:     "0FP000000000998",
		Object: "FieldPermissions",
		Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue("0PS000000000998"),
			"SObjectType":     storage.StringValue("Account"),
			"Field":           storage.StringValue("Account.ShippingAddress"),
			"PermissionsRead": storage.BooleanValue(true),
			"PermissionsEdit": storage.BooleanValue(true),
		},
	}
	machine = New(nil)
	machine.SetOrg(&org)
	machine.executionUser = user
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
System.assertEquals('005', User.SObjectType.getDescribe().getKeyPrefix());
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
System.assert(Schema.getAppDescribe('Sales').containsKey('Account'));
System.assert(Schema.getModuleDescribe().containsKey('Account'));
System.assert(Schema.getModuleDescribe('Sales').containsKey('Account'));
Object tabSet = tabSets.get(0);
System.assertEquals('All Tabs', tabSet.getLabel());
System.assertEquals('AllTabs', tabSet.getName());
System.assert(!tabSet.isSelected());
List<Object> tabs = tabSet.getTabs();
System.assertEquals(2, tabs.size());
Object tab;
for (Object candidate : tabs) {
	if (candidate.getName() == 'Widget__c') {
		tab = candidate;
	}
}
System.assertNotEquals(null, tab);
System.assertEquals('Widget__c', tab.getName());
System.assertEquals('Widgets', tab.getLabel());
System.assertEquals('Widget__c', tab.getSObjectName());
System.assert(tab.isCustom());
System.assertEquals('Custom1: Heart', tab.getIconUrl());
List<Object> icons = tab.getIcons();
System.assertEquals(1, icons.size());
Object icon = icons.get(0);
System.assertEquals('image/svg+xml', icon.getContentType());
System.assertEquals('/img/icon/t4v35/custom/widget_120.png.svg', icon.getUrl());
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

func TestExecDescribeDataCategoriesFromLocalMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> groups = Schema.describeDataCategoryGroups(new List<String>{'Knowledge__kav'});
System.assertEquals(1, groups.size());
Object group = groups[0];
System.assertEquals('Products', group.getName());
System.assertEquals('Products', group.getLabel());
System.assertEquals('Knowledge__kav', group.getSobject());
System.assertEquals(2, group.getCategoryCount());

Schema.DataCategoryGroupSobjectTypePair pair = new Schema.DataCategoryGroupSobjectTypePair();
pair.setSobject('Knowledge__kav');
pair.setDataCategoryGroupName('Products');
List<Object> structures = Schema.describeDataCategoryGroupStructures(new List<Schema.DataCategoryGroupSobjectTypePair>{pair}, false);
System.assertEquals(1, structures.size());
Object structure = structures[0];
List<Object> topCategories = structure.getTopCategories();
System.assertEquals(1, topCategories.size());
Object hardware = topCategories[0];
System.assertEquals('Hardware', hardware.getName());
System.assertEquals(1, hardware.getChildCategories().size());

List<Object> topOnly = Schema.describeDataCategoryGroupStructures(new List<Schema.DataCategoryGroupSobjectTypePair>{pair}, true)[0].getTopCategories();
System.assertEquals(0, topOnly[0].getChildCategories().size());
System.assertEquals(0, Schema.describeDataCategoryGroups(new List<String>{'Missing__kav'}).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.DataCategoryGroups = []storage.DataCategoryGroup{{
		Name:        "Products",
		Label:       "Products",
		SObjectName: "Knowledge__kav",
		Categories: []storage.DataCategory{{
			Name:  "Hardware",
			Label: "Hardware",
			Children: []storage.DataCategory{{
				Name:  "Laptops",
				Label: "Laptops",
			}},
		}},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeDependentPicklistControllerMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Object stage = Account.Stage__c.getDescribe();
System.assert(stage.isDependentPicklist());
System.assertEquals(Account.Rating, stage.getController());
System.assertEquals(Account.Rating, stage.controller);
Map<String,Integer> values = stage.getControllerValues();
System.assertEquals(0, values.get('Hot'));
System.assertEquals(1, values.get('Cold'));
System.assertEquals(2, values.size());
System.assertEquals(values.size(), stage.controllerValues.size());
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
			{Value: "Hot", Label: "Hot"},
			{Value: "Cold", Label: "Cold"},
		},
	}
	account.Definition.Fields["Stage__c"] = storage.Field{
		APIName:            "Stage__c",
		Type:               storage.FieldPicklist,
		PicklistController: "Rating",
		PicklistValues: []storage.PicklistValue{
			{Value: "Open", Label: "Open"},
			{Value: "Closed", Label: "Closed"},
		},
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeFieldSetsFromMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Object accountDescribe = Account.SObjectType.getDescribe();
Map<String,Object> fieldSets = accountDescribe.fieldSets.getMap();
System.assert(fieldSets.containsKey('Summary'));
System.assert(fieldSets.containsKey('pkg__Summary'));
System.assertEquals(fieldSets.get('Summary'), fieldSets.get('pkg__Summary'));
Object summary = fieldSets.get('Summary');
System.assertEquals('Account Summary', summary.getLabel());
System.assertEquals('Summary', summary.getName());
System.assertEquals('pkg', summary.getNamespace());
System.assertEquals(Account.SObjectType, summary.getSObjectType());
List<Object> members = summary.getFields();
System.assertEquals(2, members.size());
Object nameMember = members.get(0);
System.assertEquals('Name', nameMember.getFieldPath());
System.assertEquals('Account Name', nameMember.getLabel());
System.assertEquals(Schema.DisplayType.STRING, nameMember.getType());
System.assertEquals(Account.Name, nameMember.getSObjectField());
System.assert(nameMember.getRequired());
System.assert(nameMember.getDbRequired());
Object ratingMember = Schema.SObjectType.Account.fieldSets.Summary.getFields().get(1);
System.assertEquals('Rating', ratingMember.getFieldPath());
System.assertEquals('Rating', ratingMember.getLabel());
System.assertEquals(Account.Rating, ratingMember.getSObjectField());
System.assert(!ratingMember.getRequired());
System.assert(!ratingMember.getDbRequired());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "pkg"
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

func TestFieldSetMemberAccountLastNameIsDBRequired(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	storage.EnsureStandardObjectFieldsForFeatures(&account.Definition, []string{"PersonAccounts"})
	org.Objects["Account"] = account
	machine.SetOrg(&org)

	member := machine.fieldSetMemberValue("Account", org.Objects["Account"].Definition, storage.FieldSetMemberMetadata{Field: "LastName"})
	if got := member.Fields["dbRequired"]; got.Kind != ValueBool || !got.Bool {
		t.Fatalf("Account.LastName dbRequired = %#v", got)
	}
}

func TestExecDescribeFieldSetMemberRelationshipPathReturnsLeafFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
Object affiliationDescribe = AccountAffiliation__c.SObjectType.getDescribe();
Object chapterDirectory = affiliationDescribe.fieldSets.getMap().get('ChapterDirectory');
Object member = chapterDirectory.getFields().get(0);
System.assertEquals('Account__r.Name', member.getFieldPath());
System.assertEquals(Schema.DisplayType.STRING, member.getType());
System.assertEquals(Account.Name, member.getSObjectField());
System.assertEquals('Name', member.getSObjectField().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["AccountAffiliation__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "AccountAffiliation__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Account__c": {
					APIName:          "Account__c",
					Type:             storage.FieldReference,
					DisplayType:      "REFERENCE",
					ReferenceTo:      []string{"Account"},
					RelationshipName: "Account__r",
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Metadata.FieldSets = []storage.FieldSetMetadata{{
		ObjectName: "AccountAffiliation__c",
		Name:       "ChapterDirectory",
		Label:      "Chapter Directory",
		Fields: []storage.FieldSetMemberMetadata{
			{Field: "Account__r.Name"},
		},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingSObjectChildRelationshipDefaultsToEmptyList(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
Integer count = 0;
for (Child__c child : account.Children__r) {
	count++;
}
System.assertEquals(0, count);
System.assertEquals(0, account.Children__r.size());
for (Child__c child : account.pkg__Children__r) {
	count++;
}
System.assertEquals(0, account.pkg__Children__r.size());
account.put('Children__r', null);
for (Child__c child : account.Children__r) {
	count++;
}
System.assertEquals(0, account.Children__r.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "pkg"
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Child__c",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", ChildRelationshipName: "Children__r"},
			},
			Relations: []storage.Relationship{{
				Field:              "Account__c",
				ParentObjects:      []string{"Account"},
				ParentRelationship: "Account__r",
				ChildRelationship:  "Children__r",
			}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingChildRelationshipDerivedFromLookupFieldDefaultsToEmptyList(t *testing.T) {
	program, err := CompileAnonymous(`
Product__c product = new Product__c(Name = 'Widget');
System.assert(product.ProductFrequencyLinks__r.isEmpty());
Integer count = 0;
for (ProductFrequencyLink__c link : product.ProductFrequencyLinks__r) {
	count++;
}
System.assertEquals(0, count);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "Product__c",
			PluralLabel: "Products",
			KeyPrefix:   "a01",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["ProductFrequencyLink__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:     "ProductFrequencyLink__c",
			Label:       "Product Frequency Link",
			PluralLabel: "Product Frequency Links",
			KeyPrefix:   "a02",
			Fields: map[string]storage.Field{
				"Product__c": {
					APIName:          "Product__c",
					Type:             storage.FieldReference,
					ReferenceTo:      []string{"Product__c"},
					RelationshipName: "ProductFrequencyLinks",
				},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
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
System.assertEquals(1, [SELECT COUNT() FROM Account]);
Account row = [SELECT Id, Name FROM Account WHERE Name = 'Acme'];
System.assertEquals('Acme', row.Name);
System.assertEquals('Acme', [SELECT Id, Name FROM Account WHERE Name = 'Acme'].Name);
try {
	String missingName = [SELECT Id, Name FROM Account WHERE Name = 'Missing'].Name;
	System.assert(false, 'expected QueryException');
} catch (QueryException qe) {
	System.assert(qe.getMessage().containsIgnoreCase('list has no rows'));
}
List<Account> limited = [SELECT Id FROM Account LIMIT :Limits.getLimitDMLRows()];
System.assertEquals(1, limited.size());
List<Account> limitedWithDate = [SELECT Id FROM Account WHERE CreatedDate <= :DateTime.now().addDays(1) LIMIT :Limits.getLimitDMLRows()];
System.assertEquals(1, limitedWithDate.size());
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

func TestExecDMLPersistsListStorageValues(t *testing.T) {
	program, err := CompileAnonymous(`
EmailMessage message = new EmailMessage();
message.toIds = new List<String>{'one@example.test', 'two@example.test'};
insert message;
List<EmailMessage> rows = [SELECT Id FROM EmailMessage];
System.assertEquals(1, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "EmailMessage")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseTreeSaveInsertsParentAndChildren(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Tree Parent');
account.put('Contacts', new List<Contact>{
    new Contact(LastName = 'One'),
    new Contact(LastName = 'Two')
});
Database.NestedSaveResult result = Database.treeSave(account);
System.assert(result.isSuccess());
System.assert(result.getId() != null);
System.assertEquals(1, result.getRelationshipSaveResults().size());
Database.RelationshipSaveResult relationship = result.getRelationshipSaveResults()[0];
System.assertEquals('Contacts', relationship.getRelationshipName());
System.assertEquals(2, relationship.getSaveResults().size());
System.assert(relationship.getSaveResults()[0].isSuccess());

List<Account> accounts = [SELECT Id, Name FROM Account WHERE Name = 'Tree Parent'];
System.assertEquals(1, accounts.size());
List<Contact> contacts = [SELECT Id, LastName, AccountId FROM Contact ORDER BY LastName];
System.assertEquals(2, contacts.size());
System.assertEquals(accounts[0].Id, contacts[0].AccountId);
System.assertEquals(accounts[0].Id, contacts[1].AccountId);
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
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseTreeSaveUpdatesParentAndInsertsChildren(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Tree Parent');
insert account;
account.Name = 'Tree Parent Updated';
account.put('Contacts', new List<Contact>{new Contact(LastName = 'New Child')});

Database.NestedSaveResult result = Database.treeSave(account);
System.assert(result.isSuccess());
System.assertEquals(account.Id, result.getId());
System.assertEquals(1, result.getRelationshipSaveResults().size());
Database.RelationshipSaveResult relationship = result.getRelationshipSaveResults()[0];
System.assertEquals('Contacts', relationship.getRelationshipName());
System.assertEquals(1, relationship.getSaveResults().size());
System.assert(relationship.getSaveResults()[0].isSuccess());

Account saved = [SELECT Id, Name FROM Account WHERE Id = :account.Id];
System.assertEquals('Tree Parent Updated', saved.Name);
Contact child = [SELECT Id, LastName, AccountId FROM Contact WHERE LastName = 'New Child'];
System.assertEquals(account.Id, child.AccountId);
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
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseTreeSaveUpdatesFirstLevelChildren(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Tree Parent');
insert account;
Contact contact = new Contact(LastName = 'Old Child', AccountId = account.Id);
insert contact;

account.Name = 'Tree Parent Updated';
contact.LastName = 'Updated Child';
account.put('Contacts', new List<Contact>{contact});

Database.NestedSaveResult result = Database.treeSave(account);
System.assert(result.isSuccess());
System.assertEquals(1, result.getRelationshipSaveResults().size());
Database.RelationshipSaveResult relationship = result.getRelationshipSaveResults()[0];
System.assertEquals(1, relationship.getSaveResults().size());
System.assert(relationship.getSaveResults()[0].isSuccess());

Account saved = [SELECT Id, Name FROM Account WHERE Id = :account.Id];
System.assertEquals('Tree Parent Updated', saved.Name);
Contact savedChild = [SELECT Id, LastName, AccountId FROM Contact WHERE Id = :contact.Id];
System.assertEquals('Updated Child', savedChild.LastName);
System.assertEquals(account.Id, savedChild.AccountId);
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
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseConvertLeadCreatesAccountAndContact(t *testing.T) {
	program, err := CompileAnonymous(`
Lead lead = new Lead(FirstName = 'Ada', LastName = 'Lovelace', Company = 'Analytical Engines', Status = 'Open');
insert lead;
lead = [SELECT Id, FirstName, LastName, Company FROM Lead WHERE LastName = 'Lovelace'];
Database.LeadConvert convert = new Database.LeadConvert();
convert.setLeadId(lead.Id);
convert.setConvertedStatus('Qualified');
convert.setDoNotCreateOpportunity(true);
System.assert(convert.getLeadId() != null, 'lead convert id set');
Database.LeadConvertResult result = Database.convertLead(convert);
System.assert(result.isSuccess(), 'convert success');
System.assertEquals(lead.Id, result.getLeadId());
System.assert(result.getAccountId() != null, 'account id');
System.assert(result.getContactId() != null, 'contact id');
System.assertEquals(null, result.getOpportunityId());

Account account = [SELECT Id, Name FROM Account WHERE Id = :result.getAccountId()];
System.assertEquals('Analytical Engines', account.Name);
Contact contact = [SELECT Id, FirstName, LastName, AccountId FROM Contact WHERE Id = :result.getContactId()];
System.assertEquals('Ada', contact.FirstName);
System.assertEquals('Lovelace', contact.LastName);
System.assertEquals(account.Id, contact.AccountId);
Lead converted = [SELECT Id, IsConverted, ConvertedAccountId, ConvertedContactId, Status FROM Lead WHERE Id = :lead.Id];
System.assertEquals(true, converted.IsConverted);
System.assertEquals(account.Id, converted.ConvertedAccountId);
System.assertEquals(contact.Id, converted.ConvertedContactId);
System.assertEquals('Qualified', converted.Status);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testLeadConvertOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseConvertLeadRecordsJournalMutation(t *testing.T) {
	program, err := CompileAnonymous(`
Lead lead = new Lead(FirstName = 'Ada', LastName = 'Lovelace', Company = 'Analytical Engines', Status = 'Open');
insert lead;
Database.LeadConvert convert = new Database.LeadConvert();
convert.setLeadId(lead.Id);
convert.setDoNotCreateOpportunity(true);
Database.LeadConvertResult result = Database.convertLead(convert);
System.assert(result.isSuccess(), 'convert success');
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testLeadConvertOrg()
	journal := storage.NewIsolationJournal(&org)
	mark := journal.Mark()
	machine.SetOrg(&org)
	machine.SetIsolationJournal(journal)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	for _, record := range org.Objects["Lead"].Records {
		if value := record.Fields["IsConverted"]; value.Kind == storage.ValueBoolean && value.Boolean {
			t.Fatalf("lead conversion survived journal rollback: %#v", record)
		}
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("account records after rollback = %d, want 0", got)
	}
	if got := len(org.Objects["Contact"].Records); got != 0 {
		t.Fatalf("contact records after rollback = %d, want 0", got)
	}
}

func TestExecDatabaseConvertLeadCreatesOpportunity(t *testing.T) {
	program, err := CompileAnonymous(`
Lead lead = new Lead(FirstName = 'Ada', LastName = 'Lovelace', Company = 'Analytical Engines', Status = 'Open');
insert lead;
Database.LeadConvert convert = new Database.LeadConvert();
convert.setLeadId(lead.Id);
convert.setConvertedStatus('Qualified');
convert.setOpportunityName('Difference Engine');
Database.LeadConvertResult result = Database.convertLead(convert);
System.assert(result.isSuccess(), 'convert success');
System.assert(result.getAccountId() != null, 'account id');
System.assert(result.getContactId() != null, 'contact id');
System.assert(result.getOpportunityId() != null, 'opportunity id');

Opportunity opportunity = [SELECT Id, Name, AccountId, StageName, CloseDate FROM Opportunity WHERE Id = :result.getOpportunityId()];
System.assertEquals('Difference Engine', opportunity.Name);
System.assertEquals(result.getAccountId(), opportunity.AccountId);
System.assertEquals('Prospecting', opportunity.StageName);
System.assertEquals(Date.today(), opportunity.CloseDate);
Lead converted = [SELECT Id, ConvertedOpportunityId FROM Lead WHERE Id = :lead.Id];
System.assertEquals(opportunity.Id, converted.ConvertedOpportunityId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testLeadConvertOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func testLeadConvertOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"FirstName": {APIName: "FirstName", Type: storage.FieldString},
				"LastName":  {APIName: "LastName", Type: storage.FieldString, Required: true},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Lead"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Lead",
			KeyPrefix: "00Q",
			Fields: map[string]storage.Field{
				"FirstName":              {APIName: "FirstName", Type: storage.FieldString},
				"LastName":               {APIName: "LastName", Type: storage.FieldString, Required: true},
				"Company":                {APIName: "Company", Type: storage.FieldString},
				"Status":                 {APIName: "Status", Type: storage.FieldString},
				"IsConverted":            {APIName: "IsConverted", Type: storage.FieldBoolean},
				"ConvertedAccountId":     {APIName: "ConvertedAccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
				"ConvertedContactId":     {APIName: "ConvertedContactId", Type: storage.FieldReference, ReferenceTo: []string{"Contact"}},
				"ConvertedOpportunityId": {APIName: "ConvertedOpportunityId", Type: storage.FieldReference, ReferenceTo: []string{"Opportunity"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Opportunity",
			KeyPrefix: "006",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString, Required: true},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
				"StageName": {APIName: "StageName", Type: storage.FieldPicklist, Required: true},
				"CloseDate": {APIName: "CloseDate", Type: storage.FieldDate, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func TestExecSOQLForLoopCanIterateInListChunks(t *testing.T) {
	program, err := CompileAnonymous(`
insert new List<Account>{
	new Account(Name = 'A'),
	new Account(Name = 'B'),
	new Account(Name = 'C')
};
Integer chunks = 0;
Integer rows = 0;
for (List<Account> accounts : [SELECT Id, Name FROM Account ORDER BY Name]) {
	chunks++;
	rows += accounts.size();
}
System.assertEquals(1, chunks);
System.assertEquals(3, rows);
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

func TestExecEnhancedForDoesNotChunkNestedLists(t *testing.T) {
	program, err := CompileAnonymous(`
List<List<Integer>> groups = new List<List<Integer>>{
	new List<Integer>{1, 2},
	new List<Integer>{3}
};
Integer chunks = 0;
Integer rows = 0;
for (List<Integer> groupValues : groups) {
	chunks++;
	rows += groupValues.size();
}
System.assertEquals(2, chunks);
System.assertEquals(3, rows);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
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
rows = Database.query('SELECT Id, Name FROM Account WHERE Name=:wanted', AccessLevel.USER_MODE);
System.assertEquals(1, rows.size());
List<String> names = new List<String>{'Acme', 'Beta'};
rows = Database.query('SELECT Id FROM Account WHERE Name IN :names ORDER BY Name');
System.assertEquals(2, rows.size());
rows = Database.query('SELECT Id FROM Account WHERE Name = :names ORDER BY Name');
System.assertEquals(2, rows.size());
Account probe = new Account(Name = 'Beta');
rows = Database.query('SELECT Id FROM Account WHERE Name = :probe.Name');
System.assertEquals(1, rows.size());
List<Account> betaRows = rows;
rows = Database.query('SELECT Id FROM Account WHERE Name = :probe.Name + \' Test\'');
System.assertEquals(0, rows.size());
Map<Id, Account> accountsById = new Map<Id, Account>();
accountsById.put(betaRows[0].Id, betaRows[0]);
rows = Database.query('SELECT Id FROM Account WHERE Id IN :accountsById.values()');
System.assertEquals(1, rows.size());
rows = Database.query('SELECT Id FROM Account WHERE RenewalDate__c = LAST_N_DAYS:2');
System.assertEquals(2, rows.size());
Date today = Date.today();
rows = Database.query('SELECT Id FROM Account WHERE RenewalDate__c >= :today');
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
caught = false;
try {
    Database.query('SELECT Id FROM Account WHERE Id = {!missing}');
} catch (QueryException qe) {
    caught = true;
}
System.assert(caught);
caught = false;
try {
    Database.query('SELECT Id FROM Account WHERE Id = \'bad data dot com\'');
} catch (QueryException qe) {
    caught = true;
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

func TestExecDatabaseQuerySingleSObjectNoRowsIsCatchableQueryException(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	SObject row = Database.query('SELECT Id, Name FROM Account WHERE Name = \'No Such Account\' LIMIT 1');
	System.assert(false, 'expected query exception');
} catch (QueryException qe) {
	caught = qe.getTypeName() + ':' + qe.getMessage();
}
System.assertEquals('System.QueryException:List has no rows for assignment to SObject', caught);
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

func TestExecMissingManagedListCustomSettingGetAllReturnsEmptyMap(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,npe4__Relationship_Auto_Create__c> settings = npe4__Relationship_Auto_Create__c.getAll();
System.assertEquals(0, settings.size());
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

func TestExecSummaryFieldDoesNotMutatePreviouslyQueriedParentAfterChildDelete(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Acme');
insert parent;
WidgetLine__c line = new WidgetLine__c(Account__c = parent.Id, Amount__c = 7, IsCoupon__c = false);
insert line;
Account queried = [SELECT SubTotal__c FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assert(queried.getPopulatedFieldsAsMap().containsKey('SubTotal__c'), 'queried subtotal should be populated');
System.assertEquals(7, queried.SubTotal__c, 'queried subtotal before child delete');
delete line;
System.assert(queried.getPopulatedFieldsAsMap().containsKey('SubTotal__c'), 'queried subtotal should remain populated');
System.assertEquals(7, queried.SubTotal__c, 'queried subtotal after child delete');
Account fresh = [SELECT SubTotal__c FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assertEquals(0, fresh.SubTotal__c, 'fresh subtotal after child delete');
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

func TestExecDMLAccessibleSummaryFieldDoesNotReadLiveRollup(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Acme');
insert parent;
insert new WidgetLine__c(Account__c = parent.Id, Amount__c = 7);
System.assertEquals(0, parent.SubTotal__c, 'inserted parent should keep stale summary value');
Account fresh = [SELECT SubTotal__c FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assertEquals(7, fresh.SubTotal__c, 'queried parent should read current summary value');
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
	}
	org.Objects["Account"] = account
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
				"Amount__c":  {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSummaryFieldCountsChildRecords(t *testing.T) {
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Acme');
insert parent;
System.assertEquals(0, parent.LineCount__c);
insert new WidgetLine__c(Account__c = parent.Id);
Account row = [SELECT LineCount__c FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assertEquals(1, row.LineCount__c);
WidgetLine__c line = [SELECT Id FROM WidgetLine__c WHERE Account__c = :parent.Id LIMIT 1];
delete line;
row = [SELECT LineCount__c FROM Account WHERE Id = :parent.Id LIMIT 1];
System.assertEquals(0, row.LineCount__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["LineCount__c"] = storage.Field{
		APIName:           "LineCount__c",
		Type:              storage.FieldSummary,
		DisplayType:       "INTEGER",
		SummaryForeignKey: "WidgetLine__c.Account__c",
		SummaryOperation:  "count",
	}
	org.Objects["Account"] = account
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSummaryFieldUsesExplicitJSONValue(t *testing.T) {
	program, err := CompileAnonymous(`
Widget__c row = (Widget__c)JSON.deserialize('{"Id":"a01000000000001","NU__SubTotal__c":10}', Widget__c.class);
System.assertEquals(10, row.SubTotal__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "NU"
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"SubTotal__c": {
					APIName:           "SubTotal__c",
					Type:              storage.FieldSummary,
					DisplayType:       "DECIMAL",
					SummarizedField:   "WidgetLine__c.Amount__c",
					SummaryForeignKey: "WidgetLine__c.Widget__c",
					SummaryOperation:  "sum",
				},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Widget__c": {APIName: "Widget__c", Type: storage.FieldReference, ReferenceTo: []string{"Widget__c"}, RelationshipName: "Widget__r"},
				"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSummaryFieldUpdateFiresParentUpdateTrigger(t *testing.T) {
	afterTrigger, err := CompileAnonymous(`
for (Account newer : Trigger.new) {
	insert new Probe__c(Account__c = newer.Id);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Acme');
insert parent;
WidgetLine__c line = new WidgetLine__c(Account__c = parent.Id, Amount__c = 4);
insert line;
line.Amount__c = 7;
update line;
List<Probe__c> probes = [SELECT Id FROM Probe__c WHERE Account__c = :parent.Id];
System.assertEquals(2, probes.size());
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
	}
	org.Objects["Account"] = account
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
				"Amount__c":  {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Probe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Probe__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountSummaryAfterUpdate",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "update",
		Program:   afterTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullSummaryFieldReevaluatesToZero(t *testing.T) {
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
	}
	org.Objects["Account"] = account
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
				"Amount__c":  {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	accountValue := Object("Account")
	accountValue.Fields["Id"] = String("001000000000001AAA")
	accountValue.Fields["SubTotal__c"] = Null
	value, err := machine.lookupPath(accountValue, []string{"SubTotal__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueDecimal || value.Decimal != 0 {
		t.Fatalf("summary value = %#v, want decimal 0", value)
	}
}

func TestExecMissingSummaryFieldOnNewSObjectReturnsEmptyAggregate(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
System.assertEquals(0, account.get('SubTotal__c'));
System.assertEquals(0, account.SubTotal__c);
System.assertEquals(0, account.get('LineCount__c'));
System.assertEquals(0, account.LineCount__c);
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
	}
	account.Definition.Fields["LineCount__c"] = storage.Field{
		APIName:           "LineCount__c",
		Type:              storage.FieldSummary,
		DisplayType:       "INTEGER",
		SummaryForeignKey: "WidgetLine__c.Account__c",
		SummaryOperation:  "count",
	}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStoredSummaryFieldReevaluatesOnRead(t *testing.T) {
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
	}
	org.Objects["Account"] = account
	org.Objects["WidgetLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "WidgetLine__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
				"Amount__c":  {APIName: "Amount__c", Type: storage.FieldDecimal},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001AAA": {
				Object: "WidgetLine__c",
				ID:     "a01000000000001AAA",
				Fields: map[string]storage.Value{
					"Account__c": storage.IDValue("001000000000001AAA"),
					"Amount__c":  storage.DecimalValue("7"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	accountValue := Object("Account")
	accountValue.Fields["Id"] = String("001000000000001AAA")
	accountValue.Fields["SubTotal__c"] = Decimal(99)
	value, err := machine.lookupPath(accountValue, []string{"SubTotal__c"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueDecimal || value.Decimal != 7 {
		t.Fatalf("summary value = %#v, want decimal 7", value)
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
binds.put('filter.name', 'Beta');
rows = Database.queryWithBinds('SELECT Id, Name FROM Account WHERE Name = :filter.name', binds, AccessLevel.USER_MODE);
System.assertEquals(1, rows.size());
System.assertEquals('Beta', rows.get(0).Name);
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

func TestExecDatabaseCountQueryWithBinds(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot');
insert new Account(Name = 'Beta', Rating = 'Warm');
Map<String,Object> binds = new Map<String,Object>();
binds.put('rating', 'Hot');
Integer hotCount = Database.countQueryWithBinds('SELECT COUNT() FROM Account WHERE Rating = :rating', binds, AccessLevel.USER_MODE);
System.assertEquals(1, hotCount);
binds.put('ratings', new List<String>{'Hot', 'Warm'});
Integer totalCount = Database.countQueryWithBinds('SELECT Id FROM Account WHERE Rating IN :ratings', binds, AccessLevel.SYSTEM_MODE);
System.assertEquals(2, totalCount);
binds.put('filter.rating', 'Warm');
Integer warmCount = Database.countQueryWithBinds('SELECT COUNT() FROM Account WHERE Rating = :filter.rating', binds, AccessLevel.USER_MODE);
System.assertEquals(1, warmCount);
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
Database.countQueryWithBinds('SELECT COUNT() FROM Account WHERE Name = :wanted', binds, 'USER_MODE');
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

func TestExecDatabaseCountQueryAccessLevel(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot');
insert new Account(Name = 'Beta', Rating = 'Warm');
Integer hotCount = Database.countQuery('SELECT Id FROM Account WHERE Rating = \'Hot\'', AccessLevel.USER_MODE);
System.assertEquals(1, hotCount);
Integer totalCount = Database.countQuery('SELECT COUNT() FROM Account', AccessLevel.SYSTEM_MODE);
System.assertEquals(2, totalCount);
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
Database.countQuery('SELECT COUNT() FROM Account', 'USER_MODE');
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
	user.Records["005000000000001"] = storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{
		"Id":       storage.IDValue("005000000000001"),
		"LastName": storage.StringValue("System"),
	}}
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
System.assertEquals('/services/data/v` + storage.DefaultRESTAPIVersion + `/sobjects/Account/' + a.Id, attrs.get('url'));
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
System.assertEquals('0', String.valueOf(a.Balance__c));
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

func TestExecDMLAccessibleTextFormulaEvaluatesFromFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
a.Street__c = 'Line1';
a.City__c = 'Austin';
insert a;
System.assertEquals('Line1<br />Austin', a.Address__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Street__c"] = storage.Field{APIName: "Street__c", Type: storage.FieldString}
	account.Definition.Fields["City__c"] = storage.Field{APIName: "City__c", Type: storage.FieldString}
	account.Definition.Fields["Address__c"] = storage.Field{APIName: "Address__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "Street__c & BR() & City__c"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectNumberFieldIntegerConstructorValueStringifiesAsDecimal(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme', Amount__c = 0);
System.assertEquals('0.0', String.valueOf(a.Amount__c));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
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

func TestExecApprovalLockUnlockResultShapes(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
System.assert(!Approval.isLocked(a));
Approval.LockResult locked = Approval.lock(a, false);
System.assert(locked.isSuccess());
System.assertEquals(a.Id, locked.getId());
System.assertEquals(0, locked.getErrors().size());
System.assert(Approval.isLocked(a.Id));
Map<Id, Boolean> lockStates = Approval.isLocked(new List<Id>{a.Id});
System.assertEquals(true, lockStates.get(a.Id));
Approval.UnlockResult unlocked = Approval.unlock(a.Id, false);
System.assert(unlocked.isSuccess());
System.assertEquals(a.Id, unlocked.getId());
System.assertEquals(0, unlocked.getErrors().size());
System.assert(!Approval.isLocked(a));
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

func TestExecUnsupportedDatabaseAndApprovalSurfacesReturnUnsupportedFeature(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
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
				"Name":      {APIName: "Name", Type: storage.FieldString},
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
	System.assertEquals(3, row.get('expr0'));
	System.assertEquals(3, [SELECT COUNT() FROM Account]);
	System.assertEquals(3, [SELECT COUNT(Id) FROM Account]);
	integer lowerCaseCount = [SELECT COUNT() FROM Account WHERE Rating = 'Hot'];
	System.assertEquals(2, lowerCaseCount);
	System.assertEquals(0, [SELECT COUNT() FROM Widget__c]);
	System.assertEquals(3, [SELECT COUNT(Id) FROM Account][0].get('expr0'));
	System.assertEquals(3, row.expr0);
System.assertEquals(2, row.expr1);
System.assertEquals(650.0, row.expr2);
System.assertEquals(100.0, row.expr3);
System.assertEquals(300.0, row.expr4);
System.assertEquals(216.6666666667, row.expr5);
System.assertEquals(3, row.namedCount);
System.assertEquals(650.0, row.totalRevenue);
System.assertEquals(216.6666666667, row.averageRevenue);
Integer iteratedGroups = 0;
for (AggregateResult ar : [SELECT COUNT(Id) cnt, Rating FROM Account WHERE Rating = 'Hot' GROUP BY Rating]) {
  iteratedGroups++;
  System.assertEquals('Hot', ar.get('Rating'));
  System.assertEquals(2, ar.get('cnt'));
}
System.assertEquals(1, iteratedGroups);
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
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__c", KeyPrefix: "a00", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: make(map[storage.ID]storage.Record)}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEmptyDynamicSOQLAggregateResultKeepsAggregateListType(t *testing.T) {
	program, err := CompileAnonymous(`
List<AggregateResult> rows = (List<AggregateResult>) Database.query('SELECT Rating FROM Account WHERE Rating = \'Missing\' GROUP BY Rating');
System.assertEquals(0, rows.size());
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

func TestExecEmptyDynamicSOQLNamespacedCustomMetadataKeepsObjectListType(t *testing.T) {
	program, err := CompileAnonymous(`
List<Logger__mdt> rows = Database.query('SELECT Id, pkg__IsActive__c FROM Logger__mdt WHERE pkg__IsActive__c = true');
System.assertEquals(0, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Logger__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "pkg__Logger__mdt",
			Metadata: map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"pkg__IsActive__c": {APIName: "pkg__IsActive__c", Type: storage.FieldBoolean},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEmptyDynamicSOQLFromSObjectListMethodCanAssignToConcreteList(t *testing.T) {
	runProgram, err := CompileAnonymous(`
return Database.query('SELECT Id, pkg__IsActive__c FROM Logger__mdt WHERE pkg__IsActive__c = true');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Logger__mdt> rows = QueryRunner.run();
System.assertEquals(0, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Logger__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "pkg__Logger__mdt",
			Metadata: map[string]string{"kind": "customMetadata"},
			Fields: map[string]storage.Field{
				"pkg__IsActive__c": {APIName: "pkg__IsActive__c", Type: storage.FieldBoolean},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "QueryRunner",
		Methods: map[string]Method{
			"run": {Name: "QueryRunner.run", ClassName: "QueryRunner", ReturnType: "List<SObject>", IsStatic: true, Program: runProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEmptyAggregateResultListAssignedThroughSObjectListLosesAggregateRuntime(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> rows = new List<AggregateResult>();
List<Account> accounts = rows;
System.assertEquals(0, accounts.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedUnitOfWorkNewInstanceHonorsMockInstance(t *testing.T) {
	newInstanceProgram, err := CompileAnonymous(`
if (mockInstance != null && Test.isRunningTest()) {
	return mockInstance;
}
return new fflib_SObjectUnitOfWork(sObjectTypes);
`)
	if err != nil {
		t.Fatal(err)
	}
	runProgram, err := CompileAnonymous(`
fflib_SObjectUnitOfWork.mockInstance = (fflib_SObjectUnitOfWork)Test.createStub(fflib_SObjectUnitOfWork.class, new Provider());
fflib_SObjectUnitOfWork actual = fflib_SObjectUnitOfWork.newInstance(new List<Schema.SObjectType>{ Contact.SObjectType });
actual.registerDirty(new Contact(Id = '003000000000001', LastName = 'Trail'));
actual.commitWork();
System.assertEquals(2, Provider.calls);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name:       "fflib_SObjectUnitOfWork",
		Namespace:  "dep",
		Dependency: true,
		StaticFields: map[string]Field{
			"mockInstance": {Name: "mockInstance", Type: "fflib_SObjectUnitOfWork", Static: true},
		},
		Methods: map[string]Method{
			"newInstance": {
				Name:       "fflib_SObjectUnitOfWork.newInstance",
				ClassName:  "fflib_SObjectUnitOfWork",
				ReturnType: "fflib_SObjectUnitOfWork",
				Params:     []Param{{Name: "sObjectTypes", Type: "List<Schema.SObjectType>"}},
				IsStatic:   true,
				Access:     "global",
				Program:    newInstanceProgram,
			},
			"commitWork": {
				Name:       "fflib_SObjectUnitOfWork.commitWork",
				ClassName:  "fflib_SObjectUnitOfWork",
				ReturnType: "void",
				Access:     "global",
			},
			"registerDirty": {
				Name:       "fflib_SObjectUnitOfWork.registerDirty",
				ClassName:  "fflib_SObjectUnitOfWork",
				ReturnType: "void",
				Params:     []Param{{Name: "record", Type: "SObject"}},
				Access:     "global",
			},
		},
		Access: "global",
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "fflib_SObjectUnitOfWork",
		Namespace: "pkg",
		StaticFields: map[string]Field{
			"mockInstance": {Name: "mockInstance", Type: "fflib_SObjectUnitOfWork", Static: true},
		},
		Methods: map[string]Method{
			"newInstance": {
				Name:       "fflib_SObjectUnitOfWork.newInstance",
				ClassName:  "fflib_SObjectUnitOfWork",
				ReturnType: "fflib_SObjectUnitOfWork",
				Params:     []Param{{Name: "sObjectTypes", Type: "List<Schema.SObjectType>"}},
				IsStatic:   true,
				Access:     "global",
				Program:    newInstanceProgram,
			},
			"commitWork": {
				Name:       "fflib_SObjectUnitOfWork.commitWork",
				ClassName:  "fflib_SObjectUnitOfWork",
				ReturnType: "void",
				Access:     "global",
			},
			"registerDirty": {
				Name:       "fflib_SObjectUnitOfWork.registerDirty",
				ClassName:  "fflib_SObjectUnitOfWork",
				ReturnType: "void",
				Params:     []Param{{Name: "record", Type: "SObject"}},
				Access:     "global",
			},
		},
		Access: "global",
	}); err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`
if (calls == null) {
	calls = 0;
}
calls = calls + 1;
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		StaticFields: map[string]Field{
			"calls": {Name: "calls", Type: "Integer", Static: true},
		},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Runner",
		Namespace: "pkg",
		Methods: map[string]Method{
			"run": {Name: "Runner.run", ClassName: "Runner", ReturnType: "void", IsStatic: true, Access: "global", Program: runProgram},
		},
		Access: "global",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.call("Runner.run", nil, nil, &Result{}); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLAggregateResultForEachWithMapKeySetBind(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot');
insert new Account(Name = 'Beta', Rating = 'Hot');
Map<String, List<Account>> byRating = new Map<String, List<Account>>();
byRating.put('Hot', new List<Account>());
Integer groups = 0;
for (AggregateResult ar : [
	SELECT COUNT(Id) cnt, Rating rating
	FROM Account
	WHERE Rating IN :byRating.keySet()
	GROUP BY Rating
	HAVING COUNT(Id) > 1
]) {
	groups++;
	System.assertEquals('Hot', ar.get('rating'));
	System.assertEquals(2, ar.get('cnt'));
}
System.assertEquals(1, groups);
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
List<Account> weekRows = [SELECT Id FROM Account WHERE RenewalDate__c = THIS_WEEK];
System.assertEquals(2, weekRows.size(), 'THIS_WEEK should cover the current Sunday-start week');
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
System.assertEquals('5/2/2026', DateWorker.echo(rows[0].RenewalDate__c));
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
				"Name":      {APIName: "Name", Type: storage.FieldString},
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
System.assertEquals('2026-05-02 12:00:00', a.CreatedDate.formatGmt('yyyy-MM-dd HH:mm:ss'));
a.Name = 'System Fields Updated';
update a;
System.assert(a.LastModifiedDate != null);
Account row = [SELECT Id, CreatedDate, CreatedById, LastModifiedDate, LastModifiedById, SystemModstamp, OwnerId, IsDeleted FROM Account WHERE Id = :a.Id];
System.assertEquals('2026-05-02 12:00:00', row.CreatedDate.formatGmt('yyyy-MM-dd HH:mm:ss'));
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

func TestExecSOQLQueriesSeededCurrentUserWithTightInBind(t *testing.T) {
	program, err := CompileAnonymous(`
Set<Id> ids = new Set<Id>{ UserInfo.getUserId() };
List<User> users = Database.query('SELECT Id, FirstName FROM User WHERE Id IN:ids');
System.assertEquals(1, users.size());
System.assertEquals(UserInfo.getUserId(), users[0].Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLQueriesSeededCurrentUserCreatedDate(t *testing.T) {
	program, err := CompileAnonymous(`
User user = [SELECT Id, CreatedDate FROM User WHERE Id = :UserInfo.getUserId()];
System.assert(user.CreatedDate != null);
System.assertEquals('2000-01-01 00:00:00', user.CreatedDate.formatGmt('yyyy-MM-dd HH:mm:ss'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLForViewCreatesRecentlyViewedRow(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Viewed');
insert account;
List<Account> viewed = [SELECT Id, Name FROM Account WHERE Id = :account.Id FOR VIEW];
System.assertEquals(1, viewed.size());
List<RecentlyViewed> recent = [SELECT Id, Name, Type, LastViewedDate FROM RecentlyViewed WHERE Type = 'Account'];
System.assertEquals(1, recent.size());
System.assertEquals(account.Id, recent[0].Id);
System.assertEquals('Viewed', recent[0].Name);
System.assertEquals('Account', recent[0].Type);
System.assertEquals('2026-05-02 12:00:00', recent[0].LastViewedDate.formatGmt('yyyy-MM-dd HH:mm:ss'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "RecentlyViewed")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAfterInsertTriggerCanQueryCreatedByUser(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
  Set<Id> ids = new Set<Id>{ a.CreatedById };
  List<User> users = Database.query('SELECT Id, FirstName FROM User WHERE Id IN:ids');
  System.assertEquals(1, users.size());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`insert new Account(Name = 'Trigger User');`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{Name: "AccountAfterInsert", Object: "Account", Timing: "after", Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOpportunityStageFlagsAreDerivedForReadsAndTriggers(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Opportunity opp : Trigger.new) {
  System.assertEquals(true, opp.IsClosed);
  System.assertEquals(true, opp.IsWon);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
insert new Opportunity(Name = 'Closed Gift', Amount = 10, CloseDate = Date.today(), StageName = 'Closed Won');
Opportunity opp = [SELECT Id, IsClosed, IsWon, StageName FROM Opportunity WHERE Name = 'Closed Gift' LIMIT 1];
System.assertEquals(true, opp.IsClosed);
System.assertEquals(true, opp.IsWon);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{Name: "OpportunityBeforeInsert", Object: "Opportunity", Timing: "before", Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "OpportunityAfterInsert", Object: "Opportunity", Timing: "after", Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMissingManagedNumericFieldOnStandardSObjectReadsNull(t *testing.T) {
	program, err := CompileAnonymous(`
Opportunity opp = new Opportunity(Name = 'Open Gift', Amount = 10, CloseDate = Date.today(), StageName = 'Prospecting');
System.assertEquals(null, opp.pkg__Number_of_Payments__c);
System.assertEquals(null, opp.pkg__Payments_Made__c);
insert opp;
Opportunity queried = [SELECT Id, pkg__Payments_Made__c, pkg__Number_of_Payments__c FROM Opportunity WHERE Id = :opp.Id LIMIT 1];
System.assertEquals(null, queried.pkg__payments_made__c);
System.assertEquals(null, queried.pkg__number_of_payments__c);
System.assert(queried.pkg__payments_made__c == null);
System.assert(queried.pkg__number_of_payments__c == null);
System.assertEquals(null, queried.get('pkg__Payments_Made__c'));
System.assertEquals(null, queried.get('pkg__Number_of_Payments__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	storage.EnsureDeterministicPlatformData(&org)
	opportunity := org.Objects["Opportunity"]
	opportunity.Definition.Fields["pkg__Payments_Made__c"] = storage.Field{APIName: "pkg__Payments_Made__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"}
	opportunity.Definition.Fields["pkg__Number_of_Payments__c"] = storage.Field{APIName: "pkg__Number_of_Payments__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"}
	org.Objects["Opportunity"] = opportunity
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestLookupPathManagedNumericProjectedNullThroughRegisteredFieldReadsNull(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	opportunity := org.Objects["Opportunity"]
	opportunity.Definition.Fields["pkg__Payments_Made__c"] = storage.Field{APIName: "pkg__Payments_Made__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"}
	org.Objects["Opportunity"] = opportunity
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Opportunity",
		Fields: map[string]Field{
			"pkg__Payments_Made__c": {Name: "pkg__Payments_Made__c", Type: "Decimal"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	row := Object("Opportunity")
	row.Fields["pkg__Payments_Made__c"] = typedNull("Decimal")
	row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Opportunity", map[string]bool{
		"pkg__payments_made__c": true,
	})

	got, err := machine.lookupPath(row, []string{"pkg__payments_made__c"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueNull {
		t.Fatalf("managed numeric null = %s (%s), want null", got.String(), got.Kind)
	}
}

func TestSyntheticSchemaFieldKeepsCustomFieldShapeUnknown(t *testing.T) {
	for _, fieldName := range []string{
		"pkg__EcfmgNumber__c",
		"pkg__NUCCTaxonomy__c",
		"pkg__Number_of_Payments__c",
		"pkg__Has_Result__c",
		"pkg__StartedDate__c",
		"pkg__OwnerId__c",
	} {
		field := syntheticSchemaField(fieldName)
		if field.Type != storage.FieldAny || field.DisplayType != "" || len(field.ReferenceTo) != 0 || field.RelationshipName != "" {
			t.Fatalf("%s inferred as %#v, want unknown metadata fallback", fieldName, field)
		}
	}
}

func TestExecUnknownCustomFieldIntegerIsNotCoercedToDecimalWithoutMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Line__c unknown = new Line__c(Count__c = 1);
Object unknownValue = unknown.get('Count__c');
Line__c known = new Line__c(Amount__c = 1);
Object knownValue = known.get('Amount__c');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Line__c",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.Org = &org
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Vars["unknownValue"]; got.Kind != ValueInt || got.Int != 1 {
		t.Fatalf("unknownValue = %#v, want integer 1", got)
	}
	if got := result.Vars["knownValue"]; got.Kind != ValueDecimal || got.Decimal != 1 {
		t.Fatalf("knownValue = %#v, want decimal 1", got)
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
				"Name":      {APIName: "Name", Type: storage.FieldString},
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

func TestExecMissingChildRelationshipFieldDefaultsToEmptyList(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
System.assertEquals(true, a.Contacts.isEmpty());
System.assertEquals(0, a.Contacts.size());
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

func TestExecConstructedSObjectChildRelationshipDoesNotLazyLoadFromOrg(t *testing.T) {
	program, err := CompileAnonymous(`
Account stored = new Account(Name = 'Acme');
insert stored;
insert new Contact(AccountId = stored.Id, LastName = 'Child');
Account fabricated = new Account(Id = stored.Id);
System.assertEquals(0, fabricated.Contacts.size());
Account queried = [SELECT Id FROM Account WHERE Id = :stored.Id];
System.assertEquals(1, queried.Contacts.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectEqualityUsesTypeAndFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account first = new Account(Name = 'Acme');
Account clone = first.clone(false, true, true, true);
System.assertEquals(first, clone);
clone.Name = 'Beta';
System.assertNotEquals(first, clone);
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

func TestSObjectEqualityTreatsCustomRelationshipShellsAsSObjects(t *testing.T) {
	left := Object("Line__c")
	left.Fields["Product__r"] = Object("Product__r")
	right := Object("Line__c")
	right.Fields["Product__r"] = Object("Product__r")
	if !left.Equal(right) {
		t.Fatalf("expected matching custom relationship shells to be equal")
	}
	right.Fields["Product__r"].Fields["Name"] = String("Changed")
	if left.Equal(right) {
		t.Fatalf("expected relationship shell field differences to be unequal")
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

func TestExecDatabaseInsertGenericListPropagatesIDs(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
List<SObject> records = new List<SObject>{account};
Database.insert(records);
System.assertNotEquals(null, account.Id);
Account stored = [SELECT Id FROM Account WHERE Id = :account.Id];
System.assertEquals(account.Id, stored.Id);
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

func TestExecDatabaseInsertSingleSObjectPropagatesID(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
Database.insert(account);
System.assertNotEquals(null, account.Id);
Account stored = [SELECT Id FROM Account WHERE Id = :account.Id];
System.assertEquals(account.Id, stored.Id);
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

func TestRecordFromValuePrefersPackageLocalFieldAliasOverCanonicalDefault(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "pkg__Product__c",
			KeyPrefix: "a12",
			Fields: map[string]storage.Field{
				"pkg__RevenueGLAccount__c": {APIName: "pkg__RevenueGLAccount__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__GLAccount__c"}},
			},
		},
	}
	machine.SetOrg(&org)
	product := Object("pkg__Product__c")
	setExplicitSObjectField(&product, "pkg__RevenueGLAccount__c", platformScalar("Id", "aNQ000000000001"))
	setExplicitSObjectField(&product, "RevenueGLAccount__c", platformScalar("Id", "aNQ000000000002"))
	record, err := machine.recordFromValue(&product)
	if err != nil {
		t.Fatal(err)
	}
	got := record.Fields["pkg__RevenueGLAccount__c"]
	if got.ID != "aNQ000000000002" {
		t.Fatalf("RevenueGLAccount = %#v, want package-local alias value", got)
	}
}

func TestExecDatabaseInsertGenericListAfterClearingPlaceholderIDPropagatesID(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
account.Id = '001000000000999AAA';
List<SObject> records = new List<SObject>{account};
for (SObject record : records) {
	record.Id = null;
}
Database.insert(records);
System.assertNotEquals(null, account.Id);
System.assertNotEquals('001000000000999AAA', account.Id);
Account stored = [SELECT Id FROM Account WHERE Id = :account.Id];
System.assertEquals(account.Id, stored.Id);
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

func TestExecSObjectPutFieldTokenPersistsThroughGenericListInsert(t *testing.T) {
	program, err := CompileAnonymous(`
SObject record = Account.SObjectType.newSObject(null, true);
record.put(Account.Name, 'Test Org');
List<SObject> records = new List<SObject>{record};
Database.insert(records);
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Test Org'];
System.assertEquals(1, rows.size());
System.assertEquals('Test Org', rows[0].Name);
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

func TestExecMapHeldSObjectPutFieldTokenPersistsThroughGenericListInsert(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> order = new List<String>();
Map<String, List<SObject>> byType = new Map<String, List<SObject>>();
SObject record = Account.SObjectType.newSObject(null, true);
record.put(Account.Name, 'Test Org');
String typeName = record.getSObjectType().getDescribe().getName();
List<SObject> bucket = byType.get(typeName);
if (bucket == null) {
    bucket = new List<SObject>();
    byType.put(typeName, bucket);
    order.add(typeName);
}
bucket.add(record);
for (String orderedType : order) {
    List<SObject> inserts = byType.get(orderedType);
    for (SObject inserted : inserts) {
        inserted.Id = null;
    }
    Database.insert(inserts);
}
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Test Org'];
System.assertEquals(1, rows.size());
System.assertEquals('Test Org', rows[0].Name);
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

func TestExecGenericListInsertKeepsRecordTypeIdWhenExplicitMarkerIsLost(t *testing.T) {
	program, err := CompileAnonymous(`
Id businessRecordTypeId = '012RL00000CgP6aYAF';
SObject record = Account.SObjectType.newSObject(null, true);
record.put(Account.Name, 'Test Org');
record.put(Account.RecordTypeId, businessRecordTypeId);
List<SObject> records = new List<SObject>{record};
Database.insert(records);
Account stored = [SELECT Id, RecordTypeId FROM Account WHERE Name = 'Test Org'];
System.assertEquals(businessRecordTypeId, stored.RecordTypeId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRecordsFromValueMergesSObjectAliasesFromCallerScope(t *testing.T) {
	rich := Object("Account")
	rich.Ref = 42
	rich.Fields["Name"] = String("Test Org")
	thin := Object("Account")
	thin.Ref = 42
	thin.Fields["Id"] = platformScalar("Id", "001000000000999AAA")
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.Globals["records"] = List(thin)
	machine.scopeStack = []map[string]Value{{"record": rich}}
	records, _, err := machine.recordsFromValue(machine.Globals["records"])
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d", len(records))
	}
	if got, ok := records[0].GetField("Name"); !ok || got.String != "Test Org" {
		t.Fatalf("merged Name = %#v ok=%v", got, ok)
	}
}

func TestExecSObjectFieldAssignmentPropagatesToSameIDAliasBeforeInsert(t *testing.T) {
	program, err := CompileAnonymous(`
SObject cached = new Account(Id = '001000000000999AAA', Name = 'Old');
Account account = (Account)cached;
account.Name = 'Castro';
cached.Id = null;
Database.insert(new List<SObject>{cached});
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Castro'];
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

func TestExecDMLMergesSObjectAliasesFromStaticCache(t *testing.T) {
	loadProgram, err := CompileAnonymous(`
Cached = new Account(Id = '001000000000999AAA', Name = 'Old');
return (Account)Cached;
`)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := CompileAnonymous(`
Cached.Id = null;
insert new List<SObject>{ Cached };
`)
	if err != nil {
		t.Fatal(err)
	}
	runProgram, err := CompileAnonymous(`
Account account = AliasCache.load();
account.Name = 'New';
AliasCache.save();
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'New'];
System.assertEquals(1, rows.size());
System.assertEquals(rows[0].Id, account.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	testProgram, err := CompileAnonymous(`
AliasHarness.run();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "AliasCache",
		StaticFields: map[string]Field{
			"Cached": {Name: "Cached", Type: "SObject", Static: true},
		},
		Methods: map[string]Method{
			"load#": {Name: "AliasCache.load", ClassName: "AliasCache", ReturnType: "Account", Program: loadProgram, IsStatic: true},
			"save#": {Name: "AliasCache.save", ClassName: "AliasCache", ReturnType: "void", Program: saveProgram, IsStatic: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "AliasHarness",
		Methods: map[string]Method{
			"run#": {Name: "AliasHarness.run", ClassName: "AliasHarness", ReturnType: "void", Program: runProgram, IsStatic: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(testProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecListIndexedSObjectAliasPersistsFieldMutation(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = new List<SObject>{ new Account(Id = '001000000000999AAA', Name = 'Old') };
Account first = (Account)records[0];
Account second = (Account)records[0];
first.Name = 'New';
records[0].Id = null;
insert records;
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'New'];
System.assertEquals(1, rows.size());
System.assertEquals(rows[0].Id, first.Id);
System.assertEquals(rows[0].Id, second.Id);
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

func TestExecObjectHeldConcreteSObjectListCastPreservesAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Account original = new Account(Id = '001000000000999AAA', Name = 'Old');
Object cachedData = new List<Account>{ original };
Account account = (Account)((List<SObject>)cachedData)[0];
account.Name = 'New';
List<SObject> records = (List<SObject>)cachedData;
records[0].Id = null;
insert records;
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'New'];
System.assertEquals(1, rows.size());
System.assertEquals(rows[0].Id, account.Id);
System.assertEquals(rows[0].Id, original.Id);
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

func TestExecMapHeldConcreteSObjectListCastPreservesAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Account original = new Account(Id = '001000000000999AAA', Name = 'Old');
Map<String,Object> recordsByDataSource = new Map<String,Object>();
recordsByDataSource.put('DataSource$NewAccount', new List<Account>{ original });
Account account = (Account)((List<SObject>)recordsByDataSource.get('DataSource$NewAccount'))[0];
account.Name = 'New';
Object cachedData = recordsByDataSource.get('DataSource$NewAccount');
List<SObject> records = (List<SObject>)cachedData;
records[0].Id = null;
insert records;
List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'New'];
System.assertEquals(1, rows.size());
System.assertEquals(rows[0].Id, account.Id);
System.assertEquals(rows[0].Id, original.Id);
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

func TestExecQueryServiceSaveRejectsMissingAndNonListDataSource(t *testing.T) {
	program, err := CompileAnonymous(`
QueryService service = new QueryService();
Boolean missingCaught = false;
try {
	service.save('MissingDataSource', new UnitOfWork());
} catch (InvalidOperationException e) {
	missingCaught = true;
}
System.assert(missingCaught, 'missing cached data should fail');
service.recordsByDataSource.put('BadDataSource', 'not records');
Boolean caught = false;
try {
	service.save('BadDataSource', new UnitOfWork());
} catch (InvalidOperationException e) {
	caught = true;
}
System.assert(caught, 'cached non-list data should still fail');
`)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := CompileAnonymous(`
Object cachedData = recordsByDataSource.get(dataSourceTypeName);
if (!(cachedData instanceof List<SObject>)) {
	throw new InvalidOperationException('Cannot save without a save operation implemented: ' + dataSourceTypeName);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	ctorProgram, err := CompileAnonymous(`
recordsByDataSource = new Map<String,Object>();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "UnitOfWork",
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "InvalidOperationException",
		SuperClass: "Exception",
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "QueryService",
		Constructors: []Method{{
			Name:          "QueryService.<init>",
			ClassName:     "QueryService",
			IsConstructor: true,
			Program:       ctorProgram,
		}},
		Fields: map[string]Field{
			"recordsByDataSource": {Name: "recordsByDataSource", Type: "Map<String,Object>"},
		},
		Methods: map[string]Method{
			"save#String#UnitOfWork": {
				Name:       "QueryService.save",
				ClassName:  "QueryService",
				ReturnType: "void",
				Params: []Param{
					{Name: "dataSourceTypeName", Type: "String"},
					{Name: "unitOfWork", Type: "UnitOfWork"},
				},
				Program: saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPersonAccountSignalUsesPersonRecordTypeDefault(t *testing.T) {
	program, err := CompileAnonymous(`
Account company = new Account(Name = 'Company');
insert company;
Account person = new Account(LastName = 'Castro');
insert person;
System.assertEquals('Business', [SELECT RecordType.DeveloperName FROM Account WHERE Id = :company.Id].RecordType.DeveloperName);
System.assertEquals('Individual', [SELECT RecordType.DeveloperName FROM Account WHERE Id = :person.Id].RecordType.DeveloperName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{ID: "012000000000001", DeveloperName: "Business", Name: "Business", Active: true, Available: true, Default: true},
		{ID: "012000000000002", DeveloperName: "Individual", Name: "Individual", Active: true, Available: true},
	}
	org.Objects["Account"] = account
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestApplyDMLTestNameDefaultsWithPartialInsert(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Widget__c",
		KeyPrefix: "a00",
		Label:     "Widget",
		Fields: map[string]storage.Field{
			"Id":   {APIName: "Id", Type: storage.FieldID},
			"Name": {APIName: "Name", Label: "Widget Name", Type: storage.FieldString, Required: true},
		},
	}}
	machine.SetOrg(&org)
	machine.testContext = &TestContext{}

	widget := Object("Widget__c")
	results, err := machine.applyDML("insert", widget, false, "", dml.Options{}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("insert results = %#v", results)
	}
	if rows := machine.Org.Objects["Widget__c"].Records; len(rows) != 1 {
		t.Fatalf("widget rows = %#v", rows)
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

func TestExecCustomObjectFieldTokenSynthesizesWithoutMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectField field = pkg__Invoice__c.pkg__Amount__c;
System.assertEquals('pkg__Amount__c', field.getDescribe().getName());
System.assertEquals('pkg__Invoice__c', field.getDescribe().getSObjectName());
System.assertEquals('pkg__Invoice__c', pkg__Invoice__c.SObjectType.getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectPutWrongFieldTokenThrowsSObjectException(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"Email": {APIName: "Email", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	record := Object("Account")
	token := sObjectFieldToken("Contact", "Email")
	_, handled, err := machine.callSObjectMember(record, "put", []Value{token, String("ada@example.com")})
	if err == nil {
		t.Fatal("expected SObjectException")
	}
	if !handled {
		t.Fatal("put was not handled")
	}
	var thrown *apexThrowError
	if !errors.As(err, &thrown) || thrown.value.Type != "SObjectException" {
		t.Fatalf("err = %#v, want SObjectException", err)
	}
}

func TestExecSObjectGetUnknownFieldThrowsSObjectException(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
try {
  new Account(Name = 'Acme').get('THIS_SHOULDNT_EXIST');
  System.assert(false, 'expected SObjectException');
} catch (SObjectException e) {
  System.assert(e.getMessage().contains('THIS_SHOULDNT_EXIST'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectGetWrongFieldTokenCatchCanUseFallbackRecord(t *testing.T) {
	program, err := CompileAnonymous(`
Contact person = new Contact(Email = 'ada@example.invalid');
Account account = new Account(Name = 'Acme');
Object value;
try {
  value = account.get(Contact.Email);
} catch (SObjectException e) {
  value = person.get(Contact.Email);
}
System.assertEquals('ada@example.invalid', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSObjectGetCoercesNumericStringByFieldDefinition(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	record := Object("Account")
	setExplicitSObjectField(&record, "AnnualRevenue", String("12.5"))

	value, handled, err := machine.callSObjectMember(record, "get", []Value{String("AnnualRevenue")})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("get was not handled")
	}
	if value.Kind != ValueDecimal || value.Decimal != 12.5 {
		t.Fatalf("AnnualRevenue value = %#v", value)
	}
}

func TestExecCatchNestedExceptionByQualifiedName(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	throw new InnerException('blocked');
} catch (Outer.InnerException e) {
	caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer.InnerException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "InnerException", SuperClass: "Outer.InnerException"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExceptionGetInaccessibleFieldsDefaultsEmpty(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	throw new NoAccessException('blocked');
} catch (Exception e) {
	Map<String, Set<String>> fields = e.getInaccessibleFields();
	System.assert(fields != null);
	System.assertEquals(0, fields.size());
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

func TestExecDMLRejectsNonNullCalculatedFieldWritesAsSaveResult(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
a.put('Score__c', 7);
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
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated, DisplayType: "INTEGER"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUpdateQueriedCalculatedFieldDoesNotWriteIt(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Account loaded = [SELECT Id, Name, Score__c FROM Account WHERE Id = :a.Id];
System.assertEquals(1, loaded.Score__c);
loaded.Name = 'Changed';
update loaded;
Account saved = [SELECT Name FROM Account WHERE Id = :a.Id];
System.assertEquals('Changed', saved.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated, DisplayType: "INTEGER", Formula: "1"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFromValueSkipsImplicitReadOnlyStandardFields(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	account := Object("Account")
	account.Fields["Name"] = String("Acme")
	account.Fields["IsDeleted"] = Bool(false)
	account.Fields["IsCustomerPortal"] = Bool(false)
	record, err := machine.recordFromValue(&account)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := record.Fields["IsDeleted"]; ok {
		t.Fatalf("implicit IsDeleted should not be DML-visible: %#v", record.Fields)
	}
	if _, ok := record.Fields["IsCustomerPortal"]; ok {
		t.Fatalf("implicit IsCustomerPortal should not be DML-visible: %#v", record.Fields)
	}

	setExplicitSObjectField(&account, "IsDeleted", Bool(true))
	record, err = machine.recordFromValue(&account)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := record.Fields["IsDeleted"]; !ok {
		t.Fatalf("explicit non-default IsDeleted should remain VM-visible: %#v", record.Fields)
	}

	account = Object("Account")
	account.Fields["Name"] = String("Acme")
	account.Fields["IsDeleted"] = Bool(false)
	account.Fields["IsCustomerPortal"] = Bool(false)
	if _, err := machine.applyDML("insert", account, true, "", dml.Options{}, &Result{}); err != nil {
		t.Fatalf("implicit read-only fields should not reach DML: %v", err)
	}

	attachment := Object("Attachment")
	attachment.Fields["Name"] = String("note.txt")
	attachment.Fields["ParentId"] = String("00100000000LFLTAA4")
	record, err = machine.recordFromValue(&attachment)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := record.Fields["ParentId"]; !ok || value.String == "" {
		t.Fatalf("scalar ParentId should remain DML-visible: %#v", record.Fields)
	}
}

func TestRecordFromValueSkipsImplicitCalculatedFieldsOnUpdate(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "Name"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)

	value := Object("Account")
	value.Fields["Id"] = platformScalar("Id", "001000000000001AAA")
	value.Fields["Name"] = String("Changed")
	value.Fields["Score__c"] = String("Acme")
	setExplicitSObjectField(&value, "Name", value.Fields["Name"])
	setExplicitSObjectField(&value, "Score__c", value.Fields["Score__c"])

	record, err := machine.recordFromValue(&value)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := record.Fields["Score__c"]; ok {
		t.Fatalf("implicit calculated field should not be DML-visible: %#v", record.Fields)
	}
	if got := record.Fields["Name"].String; got != "Changed" {
		t.Fatalf("Name = %q, want Changed", got)
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
		System.assert(e.getMessage().contains('REQUIRED_FIELD_MISSING,'));
		System.assertEquals(3, e.getNumDml());
	System.assertEquals(0, e.getDmlIndex(0));
	System.assertEquals('REQUIRED_FIELD_MISSING', e.getDmlStatusCode(0));
	System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING, e.getDmlType(0));
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

func TestExecManualParentRelationshipDoesNotPopulateLookupField(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Id = '001B000001DVM9tIAH');
Contact contact = new Contact(Account = account);
System.assertEquals(null, contact.AccountId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
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

func TestExecBeforeInsertTriggerSeesPersonAccountShape(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.IsPersonAccount) {
		a.PersonEmail__c = a.PersonEmail;
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account account = new Account(FirstName = 'Ada', LastName = 'Lovelace', PersonEmail = 'ada@example.invalid');
insert account;
Account stored = [SELECT IsPersonAccount, PersonEmail__c FROM Account WHERE Id = :account.Id];
System.assertEquals(true, stored.IsPersonAccount);
System.assertEquals('ada@example.invalid', stored.PersonEmail__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["PersonEmail__c"] = storage.Field{APIName: "PersonEmail__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertPersonShape",
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

func TestExecParentRelationshipAccessWinsOverChildRelationshipNameCollision(t *testing.T) {
	program, err := CompileAnonymous(`
Product__c product = new Product__c(Name = 'Cap');
insert product;
Merchandise__c merchandise = new Merchandise__c(Name = 'Hat', Product2__c = product.Id);
insert merchandise;
OrderLine__c line = new OrderLine__c(Name = 'Line', Merchandise__c = merchandise.Id);
insert line;
OrderLine__c stored = [SELECT Id, Merchandise__c, Merchandise__r.Product2__c FROM OrderLine__c WHERE Id = :line.Id];
System.assertEquals(product.Id, stored.Merchandise__r.Product2__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Product__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Merchandise__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Merchandise__c",
			KeyPrefix: "a02",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"Product2__c":  {APIName: "Product2__c", Type: storage.FieldReference, ReferenceTo: []string{"Product__c"}},
				"OrderLine__c": {APIName: "OrderLine__c", Type: storage.FieldReference, ReferenceTo: []string{"OrderLine__c"}},
			},
			Relations: []storage.Relationship{
				{Field: "Product2__c", ParentObjects: []string{"Product__c"}, ParentRelationship: "Product2__r"},
				{Field: "OrderLine__c", ParentObjects: []string{"OrderLine__c"}, ParentRelationship: "OrderLine__r", ChildRelationship: "Merchandise__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["OrderLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "OrderLine__c",
			KeyPrefix: "a03",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"Merchandise__c": {APIName: "Merchandise__c", Type: storage.FieldReference, ReferenceTo: []string{"Merchandise__c"}},
			},
			Relations: []storage.Relationship{
				{Field: "Merchandise__c", ParentObjects: []string{"Merchandise__c"}, ParentRelationship: "Merchandise__r", ChildRelationship: "OrderLines__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDynamicSOQLLocalBindBuildsIdSObjectMapWithCustomField(t *testing.T) {
	selectByID, err := CompileAnonymous("return Database.query('SELECT Id, Name, ExternalId__c FROM Account WHERE id in :idSet');")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account acct = new Account(Name = 'Acme', ExternalId__c = 'v1');
insert acct;
Set<Id> ids = new Set<Id>{ acct.Id };
Map<Id, Account> accounts = new Map<Id, Account>((List<Account>) LocalAccountSelector.selectSObjectsById(ids));
System.assertEquals('v1', accounts.get(acct.Id)?.ExternalId__c);
Account loaded = accounts.get(acct.Id);
SObject record = loaded;
System.assertNotEquals(null, Account.ExternalId__c);
Schema.SObjectField externalIdField = Account.ExternalId__c;
System.assertEquals('v1', record.get(externalIdField));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ExternalId__c"] = storage.Field{APIName: "ExternalId__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "LocalAccountSelector",
		Methods: map[string]Method{
			"selectSObjectsById": {Name: "LocalAccountSelector.selectSObjectsById", ClassName: "LocalAccountSelector", ReturnType: "List<SObject>", Params: []Param{{Name: "idSet", Type: "Set<Id>"}}, IsStatic: true, Access: "public", Program: selectByID},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLUsesCurrentNamespaceObjectForDependencyClass(t *testing.T) {
	countProducts, err := CompileAnonymous(`return [SELECT Id FROM Product__c].size();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`System.assertEquals(1, pkg.InventoryManager.countProducts());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "namz"
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Product__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	org.Objects["pkg__Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "pkg__Product__c", Fields: map[string]storage.Field{"pkg__TrackInventory__c": {APIName: "pkg__TrackInventory__c", Type: storage.FieldBoolean}}},
		Records: map[storage.ID]storage.Record{
			"a0P000000000001": {ID: "a0P000000000001", Object: "pkg__Product__c", Fields: map[string]storage.Value{"pkg__TrackInventory__c": storage.BooleanValue(true)}},
		},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name:      "InventoryManager",
		Namespace: "pkg",
		Access:    "global",
		Methods: map[string]Method{
			"countProducts": {Name: "InventoryManager.countProducts", ClassName: "InventoryManager", ReturnType: "Integer", IsStatic: true, Access: "global", Program: countProducts},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedCustomObjectRunsUnqualifiedTrigger(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Thing__c thing : Trigger.new) {
	thing.Name = 'triggered';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Thing__c thing = new Thing__c();
insert thing;
Thing__c loaded = [SELECT Name FROM Thing__c WHERE Id = :thing.Id][0];
System.assertEquals('triggered', loaded.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "verifiable"
	org.Objects["verifiable__Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "verifiable__Thing__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "ThingBeforeInsert",
		Object:    "Thing__c",
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

func TestExecDMLWrapsCustomTriggerExceptionAsDmlException(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
throw new AccountTriggerHandlerException('Failed to insert tasks');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account row = new Account(Name = 'Acme');
insert row;
row.Name = 'Changed';
Boolean caught = false;
try {
	update row;
} catch (DmlException e) {
	caught = true;
	System.assert(e.getMessage().containsIgnoreCase('failed to insert tasks'), e.getMessage());
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.Classes["AccountTriggerHandlerException"] = Class{
		Name:       "AccountTriggerHandlerException",
		SuperClass: "Exception",
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterUpdateThrowsCustom",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
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

func TestExecTriggerAddErrorThroughHelperHeldRecordsProducesDMLResults(t *testing.T) {
	constructor, err := CompileAnonymous(`this.records = records;`)
	if err != nil {
		t.Fatal(err)
	}
	validate, err := CompileAnonymous(`
for (Account a : this.records) {
	if (a.Name == 'Block') {
		a.Name.addError('blocked by helper');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`
AccountValidator validator = new AccountValidator(Trigger.new);
validator.validate();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> records = new List<Account>{new Account(Name = 'Block')};
List<Object> results = Database.insert(records, false);
System.assertEquals(1, results.size());
Object first = results.get(0);
System.assert(!first.isSuccess());
System.assertEquals(1, first.getErrors().size());
System.assertEquals('blocked by helper', first.getErrors().get(0).getMessage());
List<Account> survivors = [SELECT Id FROM Account];
System.assertEquals(0, survivors.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	class := Class{
		Name: "AccountValidator",
		Fields: map[string]Field{
			"records": {Name: "records", Type: "List<Account>"},
		},
		Constructors: []Method{{
			Name:          "AccountValidator.<init>",
			ClassName:     "AccountValidator",
			ReturnType:    "void",
			IsConstructor: true,
			Params:        []Param{{Name: "records", Type: "List<Account>"}},
			Program:       constructor,
		}},
		Methods: map[string]Method{
			"validate": {Name: "AccountValidator.validate", ClassName: "AccountValidator", ReturnType: "void", Program: validate},
		},
	}
	if err := machine.RegisterClass(class); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertHelperAddError",
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

func TestExecAfterTriggerAddErrorProducesPartialDMLResults(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.Name == 'Block After') {
		a.Name.addError('blocked after trigger');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account ok = new Account(Name = 'Keep After');
Account blocked = new Account(Name = 'Block After');
List<Object> results = Database.insert(new List<Account>{ok, blocked}, false);
System.assertEquals(2, results.size());
System.assert(results.get(0).isSuccess());
System.assert(!results.get(1).isSuccess());
System.assertEquals('blocked after trigger', results.get(1).getErrors().get(0).getMessage());
System.assertEquals('Name', results.get(1).getErrors().get(0).getFields().get(0));
List<Account> survivors = [SELECT Name FROM Account];
System.assertEquals(1, survivors.size());
System.assertEquals('Keep After', survivors.get(0).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterInsertAddError",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestNeedsEarlyDMLRollbackSnapshotSkipsSinglePlainInsert(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)

	if machine.needsEarlyDMLRollbackSnapshot("insert", []storage.Record{{Object: "Account"}}, true) {
		t.Fatalf("single plain insert should not need an early full-org rollback snapshot")
	}
}

func TestNeedsEarlyDMLRollbackSnapshotKeepsRiskyCases(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)

	if !machine.needsEarlyDMLRollbackSnapshot("insert", []storage.Record{{Object: "Account"}, {Object: "Account"}}, true) {
		t.Fatalf("batch all-or-none insert needs an early rollback snapshot")
	}

	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsert",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "insert",
		Program:   ir.Program{},
	}); err != nil {
		t.Fatal(err)
	}
	if !machine.needsEarlyDMLRollbackSnapshot("insert", []storage.Record{{Object: "Account"}}, true) {
		t.Fatalf("triggered insert needs an early rollback snapshot")
	}

	machine = New(nil)
	child := storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Line__c",
		Fields:  map[string]storage.Field{},
	}}
	parent := storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Total__c": {
				APIName:           "Total__c",
				Type:              storage.FieldSummary,
				SummarizedField:   "Line__c.Amount__c",
				SummaryForeignKey: "Line__c.Parent__c",
			},
		},
	}}
	org = storage.NewOrgState()
	org.Objects["Line__c"] = child
	org.Objects["Parent__c"] = parent
	machine.SetOrg(&org)
	if !machine.needsEarlyDMLRollbackSnapshot("insert", []storage.Record{{Object: "Line__c"}}, true) {
		t.Fatalf("summary side-effect insert needs an early rollback snapshot")
	}
}

func TestExecDMLUpdateIgnoresUnmodifiedReadonlyQueriedField(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = [SELECT Name, ReadonlyText__c FROM Account WHERE Id = '001000000000001' LIMIT 1];
account.Name = 'Updated';
update account;
Account stored = [SELECT Name FROM Account WHERE Id = '001000000000001' LIMIT 1];
System.assertEquals('Updated', stored.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := singleSOQLAssignmentOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ReadonlyText__c"] = storage.Field{
		APIName:    "ReadonlyText__c",
		Type:       storage.FieldString,
		Formula:    `"selected"`,
		Updateable: storage.BoolFlag(false),
	}
	record := account.Records["001000000000001"]
	record.Fields["ReadonlyText__c"] = storage.StringValue("selected")
	account.Records["001000000000001"] = record
	org.Objects["Account"] = account
	machine.SetOrg(&org)

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLInsertKeepsCreateableReadonlyReferenceField(t *testing.T) {
	program, err := CompileAnonymous(`
Parent__c parent = new Parent__c(Name = 'Parent');
insert parent;

SObject child = Schema.getGlobalDescribe().get('Line__c').newSObject(null, true);
Map<Schema.SObjectField, Object> valuesByField = new Map<Schema.SObjectField, Object>{
	Line__c.Name => 'Line',
	Line__c.Parent__c => parent.Id
};
for (Schema.SObjectField field : valuesByField.keySet()) {
	child.put(field, valuesByField.get(field));
}
insert child;

Line__c stored = [SELECT Parent__c FROM Line__c LIMIT 1];
System.assertEquals(parent.Id, stored.Parent__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Parent__c",
		KeyPrefix: "a00",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
		},
	}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Line__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Line__c",
		KeyPrefix: "a01",
		Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)},
		},
		Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Parent__c"}, ParentRelationship: "Parent__r", ChildRelationship: "Lines__r"}},
	}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFromValueKeepsCreateableReadonlyFieldOnInsert(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Line__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Line__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)},
		},
	}}
	machine.SetOrg(&org)

	record, err := machine.recordFromValue(&Value{
		Kind: ValueObject,
		Type: "Line__c",
		Fields: map[string]Value{
			"Parent__c": String("a00000000000001AAA"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := record.GetField("Parent__c"); !ok || value.Kind != storage.ValueID || value.ID != "a00000000000001AAA" {
		t.Fatalf("Parent__c = %#v ok=%t", value, ok)
	}
}

func TestExecSummaryUpdateTriggerPreservesReadonlyParentFields(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Mid__c row : Trigger.new) {
	row.Name = row.Name;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Root__c root = new Root__c(Name = 'Root');
insert root;

Mid__c mid = new Mid__c(Name = 'Mid', Root__c = root.Id);
insert mid;

insert new Leaf__c(Name = 'Leaf', Mid__c = mid.Id, Amount__c = 3);

List<Mid__c> rows = [SELECT Root__c, Total__c FROM Mid__c WHERE Root__c IN :new Set<Id>{root.Id}];
System.assertEquals(1, rows.size());
System.assertEquals(root.Id, rows[0].Root__c);
System.assertEquals(3, rows[0].Total__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Root__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Root__c",
		KeyPrefix: "a00",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
		},
	}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Mid__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Mid__c",
		KeyPrefix: "a01",
		Fields: map[string]storage.Field{
			"Name":     {APIName: "Name", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
			"Root__c":  {APIName: "Root__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Root__c"}, RelationshipName: "Root__r", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)},
			"Total__c": {APIName: "Total__c", Type: storage.FieldSummary, DisplayType: "DOUBLE", SummarizedField: "Leaf__c.Amount__c", SummaryForeignKey: "Leaf__c.Mid__c", SummaryOperation: "SUM"},
		},
		Relations: []storage.Relationship{{Field: "Root__c", ParentObjects: []string{"Root__c"}, ParentRelationship: "Root__r", ChildRelationship: "Mids__r"}},
	}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Leaf__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Leaf__c",
		KeyPrefix: "a02",
		Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString, Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
			"Mid__c":    {APIName: "Mid__c", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"Mid__c"}, RelationshipName: "Mid__r", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(false)},
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
		},
		Relations: []storage.Relationship{{Field: "Mid__c", ParentObjects: []string{"Mid__c"}, ParentRelationship: "Mid__r", ChildRelationship: "Leaves__r"}},
	}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{Name: "MidBeforeUpdate", Object: "Mid__c", Timing: triggerTimingBefore, Operation: "update", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAfterTriggerAddErrorRepeatingUpdateDoesNotPersistFailedStatus(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	Account oldRecord = Trigger.oldMap.get(a.Id);
	if (a.Rating != oldRecord.Rating && (a.Rating == 'BlockedOne' || a.Rating == 'BlockedTwo')) {
		a.addError('blocked status transition');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Rollback Probe', Rating = 'Open');
insert account;

account.Rating = 'BlockedOne';
Database.SaveResult first = Database.update(account, false);
System.assert(!first.isSuccess());

account.Rating = 'BlockedTwo';
Database.SaveResult second = Database.update(account, false);
System.assert(!second.isSuccess());

account.Rating = 'BlockedTwo';
Database.SaveResult third = Database.update(account, false);
System.assert(!third.isSuccess());

Account stored = [SELECT Rating FROM Account WHERE Id = :account.Id LIMIT 1];
System.assertEquals('Open', stored.Rating);
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
		Name:      "AccountAfterUpdateBlockedStatus",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecValidationRuleIsChangedSurvivesBeforeTriggerSideEffect(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Batch__c b : Trigger.new) {
	Batch__c oldRecord = (Batch__c) Trigger.oldMap.get(b.Id);
	if (b.Status__c != oldRecord.Status__c && b.Status__c == 'Posted') {
		b.PostedOn__c = Date.today();
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Batch__c batch = new Batch__c(Name = 'Manual', Status__c = 'Open');
insert batch;

Batch__c loaded = [SELECT Id, Name, Status__c FROM Batch__c WHERE Id = :batch.Id LIMIT 1];
loaded.Status__c = 'Posted';
List<SObject> recordsToSave = new List<SObject>{loaded};
upsert recordsToSave;

Batch__c posted = [SELECT Id, Status__c, PostedOn__c FROM Batch__c WHERE Id = :batch.Id LIMIT 1];
System.assertEquals('Posted', posted.Status__c);
System.assertEquals(Date.today(), posted.PostedOn__c);

posted.Name = 'Edited';
try {
	upsert posted;
	System.assert(false, 'expected posted batch edit to fail');
} catch (DmlException ex) {
	System.assert(ex.getDmlMessage(0).contains('cannot be edited'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Batch__c",
			KeyPrefix: "a17",
			Fields: map[string]storage.Field{
				"Name":        {APIName: "Name", Type: storage.FieldString},
				"Status__c":   {APIName: "Status__c", Type: storage.FieldString},
				"PostedOn__c": {APIName: "PostedOn__c", Type: storage.FieldDate},
			},
			ValidationRules: []storage.ValidationRule{{
				Name:                  "Cannot_Edit_Posted_Or_Exported",
				Active:                true,
				ErrorConditionFormula: `AND(!ISChanged(Status__c), OR(Text(Status__c) = 'Posted', Text(Status__c) = 'Exported'))`,
				ErrorMessage:          "Batches with a status of Posted or Exported cannot be edited.",
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "BatchBeforeUpdatePostDate",
		Object:    "Batch__c",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTriggerCustomFieldAddErrorProducesDMLResults(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Widget__c w : Trigger.new) {
	if (String.isBlank(w.Code__c)) {
		w.Code__c.addError('Code required');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Widget__c> records = new List<Widget__c>{new Widget__c(Name = 'Blocked')};
List<Object> results = Database.insert(records, false);
System.assertEquals(1, results.size());
Object first = results.get(0);
System.assert(!first.isSuccess());
System.assertEquals('Code required', first.getErrors().get(0).getMessage());
System.assertEquals('Code__c', first.getErrors().get(0).getFields().get(0));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Widget__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Type: storage.FieldString},
				"Code__c": {APIName: "Code__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "WidgetBeforeInsertAddError",
		Object:    "Widget__c",
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

func TestBeforeInsertTriggerCanQueryExistingRowsAndAddOverlapError(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
Set<Id> parentIds = new Set<Id>();
for (Account candidate : Trigger.new) {
	Id parentId = (Id)candidate.get('ParentId');
	if (parentId != null) {
		parentIds.add(parentId);
	}
}
List<Account> existingRows = [SELECT Id, ParentId, StartDate__c, EndDate__c FROM Account WHERE ParentId IN :parentIds];
for (Account candidate : Trigger.new) {
	for (Account existing : existingRows) {
		if (candidate.ParentId == existing.ParentId &&
			candidate.Id != existing.Id &&
			candidate.StartDate__c >= existing.StartDate__c &&
			candidate.StartDate__c <= existing.EndDate__c) {
			candidate.addError('overlap');
		}
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account parent = new Account(Name = 'Parent');
insert parent;
Account first = new Account(Name = 'First', ParentId = parent.Id, StartDate__c = Date.newInstance(2026, 1, 1), EndDate__c = Date.newInstance(2026, 12, 31));
insert first;
Account second = new Account(Name = 'Second', ParentId = parent.Id, StartDate__c = first.StartDate__c.addMonths(1), EndDate__c = first.EndDate__c);
try {
	upsert second;
	System.assert(false, 'overlapping insert should fail');
} catch (DmlException ex) {
	System.assertEquals(1, ex.getNumDml());
	System.assertEquals('overlap', ex.getDmlMessage(0));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ParentId"] = storage.Field{APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
	account.Definition.Fields["StartDate__c"] = storage.Field{APIName: "StartDate__c", Type: storage.FieldDate}
	account.Definition.Fields["EndDate__c"] = storage.Field{APIName: "EndDate__c", Type: storage.FieldDate}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertOverlap",
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

func TestBeforeInsertTriggerAddErrorThroughClonedObjectListBlocksDML(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
List<Object> copied = ((List<Object>) Trigger.new).clone();
List<SObject> records = (List<SObject>) copied;
for (Account candidate : (List<Account>) records) {
	candidate.addError('blocked through object list');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	insert new Account(Name = 'Blocked');
	System.assert(false, 'insert should fail');
} catch (DmlException ex) {
	System.assertEquals(1, ex.getNumDml());
	System.assertEquals('blocked through object list', ex.getDmlMessage(0));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertObjectListAddError",
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

func TestFrameworkSObjectDomainAddErrorThroughRecordsBlocksDML(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`this.objects = records.clone();`)
	if err != nil {
		t.Fatal(err)
	}
	getRecordsProgram, err := CompileAnonymous(`return (List<SObject>) this.objects;`)
	if err != nil {
		t.Fatal(err)
	}
	handleProgram, err := CompileAnonymous(`
for (Account candidate : (List<Account>) getRecords()) {
	candidate.addError('blocked through domain');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`framework_SObjectDomain.triggerHandler(AccountDomain.class);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	insert new Account(Name = 'Blocked');
	System.assert(false, 'insert should fail');
} catch (DmlException ex) {
	System.assertEquals(1, ex.getNumDml());
	System.assertEquals('blocked through domain', ex.getDmlMessage(0));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "AccountDomain",
		Fields: map[string]Field{
			"objects": {Name: "objects", Type: "List<Object>"},
		},
		Constructors: []Method{{
			Name:          "AccountDomain.<init>",
			ClassName:     "AccountDomain",
			ReturnType:    "void",
			IsConstructor: true,
			Params:        []Param{{Name: "records", Type: "List<SObject>"}},
			Program:       constructorProgram,
		}},
		Methods: map[string]Method{
			"getRecords":         {Name: "AccountDomain.getRecords", ClassName: "AccountDomain", ReturnType: "List<SObject>", Program: getRecordsProgram},
			"handleBeforeInsert": {Name: "AccountDomain.handleBeforeInsert", ClassName: "AccountDomain", ReturnType: "void", Program: handleProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountDomainBeforeInsert",
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

func TestFrameworkSObjectDomainAfterInsertAddErrorThroughRecordsBlocksDML(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`this.objects = records.clone();`)
	if err != nil {
		t.Fatal(err)
	}
	getRecordsProgram, err := CompileAnonymous(`return (List<SObject>) this.objects;`)
	if err != nil {
		t.Fatal(err)
	}
	handleProgram, err := CompileAnonymous(`
for (Account candidate : (List<Account>) getRecords()) {
	candidate.addError('blocked through after domain');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`framework_SObjectDomain.triggerHandler(AccountDomain.class);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	insert new Account(Name = 'Blocked');
	System.assert(false, 'insert should fail');
} catch (DmlException ex) {
	System.assertEquals(1, ex.getNumDml());
	System.assertEquals('blocked through after domain', ex.getDmlMessage(0));
}
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'Blocked']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "AccountDomain",
		Fields: map[string]Field{
			"objects": {Name: "objects", Type: "List<Object>"},
		},
		Constructors: []Method{{
			Name:          "AccountDomain.<init>",
			ClassName:     "AccountDomain",
			ReturnType:    "void",
			IsConstructor: true,
			Params:        []Param{{Name: "records", Type: "List<SObject>"}},
			Program:       constructorProgram,
		}},
		Methods: map[string]Method{
			"getRecords":        {Name: "AccountDomain.getRecords", ClassName: "AccountDomain", ReturnType: "List<SObject>", Program: getRecordsProgram},
			"handleAfterInsert": {Name: "AccountDomain.handleAfterInsert", ClassName: "AccountDomain", ReturnType: "void", Program: handleProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountDomainAfterInsert",
		Object:    "Account",
		Timing:    triggerTimingAfter,
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
		a.addError(Account.Name, 'token field overload', false);
		a.addError('Rating', 'string field overload');
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
System.assertEquals(4, errors.size());
Object objectError = errors.get(0);
System.assertEquals('object overload', objectError.getMessage());
List<Object> objectFields = objectError.getFields();
System.assertEquals(0, objectFields.size());
Object fieldError = errors.get(1);
System.assertEquals('unset field overload', fieldError.getMessage());
List<Object> fieldFields = fieldError.getFields();
System.assertEquals(1, fieldFields.size());
System.assertEquals('Rating', fieldFields.get(0));
Object tokenFieldError = errors.get(2);
System.assertEquals('token field overload', tokenFieldError.getMessage());
System.assertEquals('Name', tokenFieldError.getFields().get(0));
Object stringFieldError = errors.get(3);
System.assertEquals('string field overload', stringFieldError.getMessage());
System.assertEquals('Rating', stringFieldError.getFields().get(0));
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

func TestExecPrimitiveFieldAddErrorOverloadsProduceFieldErrors(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	if (a.Name == 'Primitive Block') {
		a.Flag__c.addError('boolean field');
		a.Date__c.addError('date field');
		a.Stamp__c.addError('datetime field');
		a.Amount__c.addError('decimal field');
		a.Double__c.addError('double field');
		a.Lookup__c.addError('id field');
		a.Count__c.addError('integer field');
		a.Long__c.addError('long field');
		a.Text__c.addError('string field');
		a.Clock__c.addError('time field');
	}
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account blocked = new Account(Name = 'Primitive Block');
Object result = Database.insert(blocked, false);
System.assert(!result.isSuccess());
List<Object> errors = result.getErrors();
System.assertEquals(10, errors.size());
System.assertEquals('Flag__c', errors.get(0).getFields().get(0));
System.assertEquals('Date__c', errors.get(1).getFields().get(0));
System.assertEquals('Stamp__c', errors.get(2).getFields().get(0));
System.assertEquals('Amount__c', errors.get(3).getFields().get(0));
System.assertEquals('Double__c', errors.get(4).getFields().get(0));
System.assertEquals('Lookup__c', errors.get(5).getFields().get(0));
System.assertEquals('Count__c', errors.get(6).getFields().get(0));
System.assertEquals('Long__c', errors.get(7).getFields().get(0));
System.assertEquals('Text__c', errors.get(8).getFields().get(0));
System.assertEquals('Clock__c', errors.get(9).getFields().get(0));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Flag__c"] = storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean}
	account.Definition.Fields["Date__c"] = storage.Field{APIName: "Date__c", Type: storage.FieldDate}
	account.Definition.Fields["Stamp__c"] = storage.Field{APIName: "Stamp__c", Type: storage.FieldDateTime}
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	account.Definition.Fields["Double__c"] = storage.Field{APIName: "Double__c", Type: storage.FieldDecimal}
	account.Definition.Fields["Lookup__c"] = storage.Field{APIName: "Lookup__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}
	account.Definition.Fields["Count__c"] = storage.Field{APIName: "Count__c", Type: storage.FieldInteger}
	account.Definition.Fields["Long__c"] = storage.Field{APIName: "Long__c", Type: storage.FieldInteger}
	account.Definition.Fields["Text__c"] = storage.Field{APIName: "Text__c", Type: storage.FieldString}
	account.Definition.Fields["Clock__c"] = storage.Field{APIName: "Clock__c", Type: storage.FieldAny}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeInsertPrimitiveAddError",
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

func TestExecStandalonePrimitiveAddErrorStaysUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
String localValue = 'not a field';
localValue.addError('bad');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), `unsupported call "localValue.addError"`) {
		t.Fatalf("expected standalone primitive addError to stay unsupported, got %v", err)
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

func TestExecFailedAfterDeletePreservesExistingSObjectID(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account oldAccount : Trigger.old) {
	oldAccount.addError('blocked after delete');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account blocked = new Account(Name = 'Blocked');
insert blocked;
String beforeId = blocked.Id;
try {
	delete blocked;
	System.assert(false, 'delete should fail');
} catch (Exception e) {
}
System.assertEquals(beforeId, blocked.Id);
insert new Contact(LastName = 'Uses preserved id', AccountId = blocked.Id);
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
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterDeleteAddError",
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
System.assertEquals(Account.SObjectType, Trigger.oldMap.getSObjectType());
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

func TestTriggerContextListsCarryConcreteSObjectType(t *testing.T) {
	trigger := Trigger{Object: "ActionEvent__e", Timing: triggerTimingAfter, Operation: "insert"}
	record := storage.Record{Object: "ActionEvent__e", Fields: map[string]storage.Value{"Payload__c": storage.StringValue("{}")}}
	ctx := triggerContext(trigger, []storage.Record{record}, nil)
	newList := ctx["Trigger.new"]
	if newList.Type != "List<ActionEvent__e>" {
		t.Fatalf("Trigger.new type = %q, want concrete SObject list", newList.Type)
	}
	if newList.Runtime != "List<SObject>" {
		t.Fatalf("Trigger.new runtime = %q, want List<SObject>", newList.Runtime)
	}
	machine := New(nil)
	if score := machine.conversionScore("List<SObject>", newList); score < 0 {
		t.Fatalf("Trigger.new should be assignable to List<SObject>, score=%d", score)
	}
	if score := machine.conversionScore("List<ActionEvent__e>", newList); score < 0 {
		t.Fatalf("Trigger.new should be assignable to concrete List<ActionEvent__e>, score=%d", score)
	}
}

func TestNamespacedCustomMetadataListAssignsToLocalType(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	machine.Org = &org

	records := List(Object("pkg__EventHandler__mdt"))
	records.Type = "List<pkg__EventHandler__mdt>"

	coerced, err := machine.coerceAssignable("List<EventHandler__mdt>", records)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Type != "List<EventHandler__mdt>" {
		t.Fatalf("coerced type = %q, want List<EventHandler__mdt>", coerced.Type)
	}
}

func TestNamespacedCustomMetadataListAssignsUsingCurrentClassNamespace(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "EventHandlerService", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "EventHandlerService"

	records := List(Object("pkg__EventHandler__mdt"))
	records.Type = "List<pkg__EventHandler__mdt>"

	if _, err := machine.coerceAssignable("List<EventHandler__mdt>", records); err != nil {
		t.Fatal(err)
	}
}

func TestExecAfterUpdateTriggerOldMapKeepsPreUpdateValues(t *testing.T) {
	afterTrigger, err := CompileAnonymous(`
for (Account newer : Trigger.new) {
	Account older = Trigger.oldMap.get(newer.Id);
	System.assertEquals('Before', older.Name);
	System.assertEquals('After', newer.Name);
	System.assertEquals(null, older.Subscription__c);
	System.assertEquals('aCh000000000001AAA', newer.Subscription__c);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Before');
insert a;
a.Name = 'After';
a.Subscription__c = 'aCh000000000001AAA';
update a;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Subscription__c"] = storage.Field{APIName: "Subscription__c", Type: storage.FieldReference, ReferenceTo: []string{"Subscription__c"}}
	org.Objects["Account"] = account
	org.Objects["Subscription__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Subscription__c",
			KeyPrefix: "aCh",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{
			"aCh000000000001AAA": {Object: "Subscription__c", ID: "aCh000000000001AAA"},
		},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterUpdateOldMapSnapshot",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "update",
		Program:   afterTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAfterUpdateTriggerRunsForEquivalentEighteenCharacterID(t *testing.T) {
	afterTrigger, err := CompileAnonymous(`
for (Account newer : Trigger.new) {
	insert new Contact(LastName = 'after ran', AccountId = newer.Id);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Before');
insert a;
Account updateRecord = new Account(Id = (Id)a.Id.to18(), Name = 'After');
update updateRecord;
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
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountAfterUpdateEquivalentID",
		Object:    "Account",
		Timing:    triggerTimingAfter,
		Operation: "update",
		Program:   afterTrigger,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	contacts := org.Objects["Contact"].Records
	if len(contacts) != 1 {
		t.Fatalf("contacts = %d, want 1", len(contacts))
	}
	for _, record := range contacts {
		if got := record.Fields["LastName"]; got.Kind != storage.ValueString || got.String != "after ran" {
			t.Fatalf("contact last name = %#v", got)
		}
	}
}

func TestExecBeforeUpdateTriggerSeesMergedRecordAndFormulas(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account a : Trigger.new) {
	System.assertEquals('Before', a.Name);
	System.assertEquals(7, a.Score__c);
	a.Rating = 'Warm';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Before');
a.Amount__c = 5;
insert a;
Account patch = new Account(Id = a.Id);
patch.Rating = 'Cold';
update patch;
Account updated = [SELECT Id, Name, Rating FROM Account WHERE Id = :a.Id];
System.assertEquals('Before', updated.Name);
System.assertEquals('Warm', updated.Rating);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldInteger}
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated, DisplayType: "Integer", Formula: "Amount__c + 2"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "AccountBeforeUpdateMergedRecord",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBeforeUpdateSObjectPutThroughTriggerNewMapValuesPersists(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Account row : Trigger.new) {
	row.Rating = 'Warm';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Before');
insert a;
Account patch = new Account(Id = a.Id);
patch.Name = 'After';
update patch;
Account updated = [SELECT Rating FROM Account WHERE Id = :a.Id];
System.assertEquals('Warm', updated.Rating);
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
		Name:      "AccountBeforeUpdateSObjectPut",
		Object:    "Account",
		Timing:    triggerTimingBefore,
		Operation: "update",
		Program:   triggerProgram,
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

func TestExecAfterUndeleteCanUpdateRelatedDeletedRecord(t *testing.T) {
	afterChildUndelete, err := CompileAnonymous(`
Child__c child = (Child__c)Trigger.new.get(0);
update new Parent__c(Id = child.Parent__c, Count__c = 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Parent__c parent = new Parent__c(Name = 'Parent');
insert parent;
Child__c child = new Child__c(Name = 'Child', Parent__c = parent.Id);
insert child;
delete parent;
delete child;
undelete child;
undelete parent;
Parent__c restored = [SELECT Count__c FROM Parent__c WHERE Id = :parent.Id];
System.assertEquals(1, restored.Count__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["Parent__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Parent__c",
			KeyPrefix: "a10",
			Fields: map[string]storage.Field{
				"Name":     {APIName: "Name", Type: storage.FieldString},
				"Count__c": {APIName: "Count__c", Type: storage.FieldDecimal},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Child__c",
			KeyPrefix: "a11",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "ChildAfterUndelete",
		Object:    "Child__c",
		Timing:    triggerTimingAfter,
		Operation: "undelete",
		Program:   afterChildUndelete,
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
PlanType__c membershipType = new PlanType__c();
insert membershipType;
PlanType__c stored = [SELECT Name FROM PlanType__c WHERE Id = :membershipType.Id];
System.assertEquals('Subscription Type', stored.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	org.Objects["PlanType__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "PlanType__c",
			Label:     "Subscription Type",
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

func TestExecDMLNullAndMissingIDFailuresAreCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
String nullDelete = '';
try {
	Contact contact = null;
	delete contact;
} catch (Exception e) {
	nullDelete = e.getMessage();
}
System.assert(nullDelete.contains('Attempt to de-reference a null object'), nullDelete);

String missingID = '';
try {
	update new Account(Name = 'No Id');
} catch (DmlException e) {
	missingID = e.getMessage();
}
System.assert(missingID.contains('Id not specified in an update call:'), missingID);

Account existing = new Account(Name = 'Existing');
insert existing;
List<Account> queried = [SELECT Id, Name FROM Account WHERE Id = :existing.Id];
List<Account> accounts = (List<Account>)JSON.deserialize(JSON.serialize(queried), List<Account>.class);
for (Account account : accounts) {
	account.Id = account.ExternalSalesforceId__c;
}
missingID = '';
try {
	update accounts;
} catch (DmlException e) {
	missingID = e.getMessage();
}
System.assert(missingID.contains('Id not specified in an update call:'), missingID);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["ExternalSalesforceId__c"] = storage.Field{APIName: "ExternalSalesforceId__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLUpdateMissingRecordDoesNotRunTriggers(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	update new Account(Id = '001000000000001AAA', Name = 'Missing');
} catch (DmlException e) {
	caught = true;
}
System.assert(caught);
System.assertEquals(0, TriggerProbe.calls);
`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`TriggerProbe.touch();`)
	if err != nil {
		t.Fatal(err)
	}
	touchProgram, err := CompileAnonymous(`
if (calls == null) {
	calls = 0;
}
calls = calls + 1;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "TriggerProbe",
		StaticFields: map[string]Field{
			"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0)},
		},
		Methods: map[string]Method{
			"touch": {Name: "TriggerProbe.touch", ClassName: "TriggerProbe", ReturnType: "void", IsStatic: true, Program: touchProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "AccountBeforeUpdate", Object: "Account", Timing: triggerTimingBefore, Operation: "update", Program: triggerProgram}); err != nil {
		t.Fatal(err)
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
System.assertNotEquals(null, oldRow.MasterRecordId);
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

func TestExecMergeAfterDeleteRunsBeforeAfterUpdate(t *testing.T) {
	afterDelete, err := CompileAnonymous(`
insert new Contact(LastName = 'merge-delete-before-update');
`)
	if err != nil {
		t.Fatal(err)
	}
	afterUpdate, err := CompileAnonymous(`
List<Contact> markers = [SELECT Id FROM Contact WHERE LastName = 'merge-delete-before-update'];
System.assertEquals(1, markers.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account master = new Account(Name = 'Merge Master');
insert master;
Account duplicate = new Account(Name = 'Merge Duplicate');
insert duplicate;
Object merged = Database.merge(master, duplicate, false);
System.assert(merged.isSuccess());
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
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{Name: "AccountMergeOrderAfterDelete", Object: "Account", Timing: triggerTimingAfter, Operation: "delete", Program: afterDelete}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "AccountMergeOrderAfterUpdate", Object: "Account", Timing: triggerTimingAfter, Operation: "update", Program: afterUpdate}); err != nil {
		t.Fatal(err)
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
Object created = Database.upsert(first, Account.External_Key__c, false);
System.assert(created.isSuccess(), 'external id upsert should succeed');
System.assert(created.isCreated(), 'external id upsert should create');

Account second = new Account(External_Key__c = 'EXT-1', Name = 'Changed');
Object updated = Database.upsert(second, Account.External_Key__c, false);
System.assert(updated.isSuccess(), 'external id update should succeed');
System.assert(!updated.isCreated(), 'external id update should update');
System.assertEquals(created.getId(), updated.getId(), 'external id update id');

Account explicit = new Account(Other_Key__c = 'other-1', Name = 'Explicit');
Object explicitCreated = Database.upsert(explicit, Account.Other_Key__c, false);
System.assert(explicitCreated.isSuccess(), 'explicit external id upsert should create');
System.assert(explicitCreated.isCreated(), 'explicit external id upsert should create flag');
Account explicitUpdate = new Account(Other_Key__c = 'OTHER-1', Name = 'Explicit Changed');
Object explicitUpdated = Database.upsert(explicitUpdate, Account.Fields.Other_Key__c, false);
System.assert(explicitUpdated.isSuccess(), 'explicit field token upsert should succeed');
System.assert(!explicitUpdated.isCreated(), 'explicit field token upsert should update');
System.assertEquals(explicitCreated.getId(), explicitUpdated.getId(), 'explicit external id update id');
Account statementCreate = new Account(Other_Key__c = 'other-2', Name = 'Statement');
upsert statementCreate Other_Key__c;
Account statementUpdate = new Account(Other_Key__c = 'OTHER-2', Name = 'Statement Changed');
upsert statementUpdate Other_Key__c;
System.assertEquals(statementCreate.Id, statementUpdate.Id, 'statement external id update id');

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
System.assert(!duplicateResult.isSuccess(), 'duplicate unique insert should fail');
List<Object> duplicateErrors = duplicateResult.getErrors();
Object duplicateError = duplicateErrors.get(0);
System.assertEquals('DUPLICATE_VALUE', duplicateError.getStatusCode());

Contact bad = new Contact(LastName = 'Smith', AccountId = '001999999999999');
Object badResult = Database.insert(bad, false);
System.assert(!badResult.isSuccess(), 'bad lookup insert should fail');
List<Object> badErrors = badResult.getErrors();
Object badError = badErrors.get(0);
System.assertEquals('FIELD_INTEGRITY_EXCEPTION', badError.getStatusCode());

Contact good = new Contact(LastName = 'Jones', AccountId = created.getId());
insert good;
delete good;
List<Contact> deleted = [SELECT Id FROM Contact WHERE Id = :good.Id];
System.assertEquals(0, deleted.size());
Object undeleteResult = Database.undelete(good, false);
System.assert(undeleteResult.isSuccess(), 'undelete should succeed');
List<Contact> restored = [SELECT Id FROM Contact WHERE Id = :good.Id];
System.assertEquals(1, restored.size(), 'undelete should restore query visibility');

Account mergeDuplicate = new Account(Name = 'Merge Duplicate');
insert mergeDuplicate;
Contact mergeChild = new Contact(LastName = 'Merge Child', AccountId = mergeDuplicate.Id);
insert mergeChild;
Account mergeMaster = new Account(Id = created.getId());
Object mergeResult = Database.merge(mergeMaster, mergeDuplicate, false);
System.assert(mergeResult.isSuccess(), 'merge should succeed');
System.assertEquals(created.getId(), mergeResult.getId(), 'merge result id');
String mergedMasterId = created.getId();
List<Object> mergedIds = mergeResult.getMergedRecordIds();
System.assertEquals(1, mergedIds.size());
List<Object> updatedRelatedIds = mergeResult.getUpdatedRelatedIds();
System.assertEquals(1, updatedRelatedIds.size());
System.assertEquals(mergeChild.Id, updatedRelatedIds.get(0));
List<Account> activeDuplicate = [SELECT Id FROM Account WHERE Id = :mergeDuplicate.Id];
System.assertEquals(0, activeDuplicate.size(), 'merge duplicate should be hidden from default SOQL');
List<Account> deletedDuplicate = [SELECT Id, IsDeleted, MasterRecordId FROM Account WHERE Id = :mergeDuplicate.Id ALL ROWS];
System.assertEquals(1, deletedDuplicate.size(), 'merge duplicate should remain available with ALL ROWS');
Account deletedDuplicateRow = deletedDuplicate.get(0);
System.assert(deletedDuplicateRow.IsDeleted, 'merge duplicate should be marked deleted');
System.assertEquals(created.getId(), deletedDuplicateRow.MasterRecordId, 'merge duplicate master id');
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

func TestExecUpsertRejectsCustomObjectOwnerIdSetViaFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
Contact provider = new Contact(LastName = 'Owner Target');
insert provider;

SObject workflow = Credentialing_Workflow__c.SObjectType.newSObject();
workflow.put(Credentialing_Workflow__c.CVORequestId__c, 'request-1');
workflow.put(Credentialing_Workflow__c.OwnerId, provider.Id);
List<Object> results = Database.upsert(new List<SObject>{workflow}, Credentialing_Workflow__c.CVORequestId__c, false);
System.assertEquals(1, results.size());
Object result = results.get(0);
System.assert(!result.isSuccess());
System.assertEquals('FIELD_INTEGRITY_EXCEPTION', result.getErrors().get(0).getStatusCode());
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
	org.Objects["Credentialing_Workflow__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Credentialing_Workflow__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"CVORequestId__c": {APIName: "CVORequestId__c", Type: storage.FieldString, ExternalID: true, Unique: true},
			},
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
				"Description":                          {APIName: "Description", Type: storage.FieldString},
				"ParentId":                             {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent"},
				"Phone":                                {APIName: "Phone", Type: storage.FieldString},
				"Website":                              {APIName: "Website", Type: storage.FieldString},
				"MasterRecordId":                       {APIName: "MasterRecordId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "MasterRecord"},
				"CopyFromPrimaryAffiliationBilling__c": {APIName: "CopyFromPrimaryAffiliationBilling__c", Type: storage.FieldBoolean, DefaultValue: "false"},
				"Total__c":                             {APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "DECIMAL"},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func stripInaccessibleTestOrg() storage.OrgState {
	org := testDataOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Id":        {APIName: "Id", Type: storage.FieldID},
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Secret__c": {APIName: "Secret__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":      storage.StringValue("Acme"),
					"Secret__c": storage.StringValue("Hidden"),
				},
			},
		},
	}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"00e000000000002": {
			ID:     "00e000000000002",
			Object: "Profile",
			Fields: map[string]storage.Value{
				"Name": storage.StringValue("Minimum Access - Salesforce"),
			},
		},
	}}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"110000000000001": {
			ID:     "110000000000001",
			Object: "ObjectPermissions",
			Fields: map[string]storage.Value{
				"ParentId":          storage.IDValue("00e000000000002"),
				"SObjectType":       storage.StringValue("Account"),
				"PermissionsRead":   storage.BooleanValue(true),
				"PermissionsCreate": storage.BooleanValue(true),
				"PermissionsEdit":   storage.BooleanValue(true),
			},
		},
	}}
	org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0FP000000000001": {
			ID:     "0FP000000000001",
			Object: "FieldPermissions",
			Fields: map[string]storage.Value{
				"ParentId":        storage.IDValue("00e000000000002"),
				"SObjectType":     storage.StringValue("Account"),
				"Field":           storage.StringValue("Account.Name"),
				"PermissionsRead": storage.BooleanValue(true),
				"PermissionsEdit": storage.BooleanValue(true),
			},
		},
	}}
	return org
}

func stripInaccessibleTestUser() Value {
	user := Object("User")
	user.Fields["Id"] = String("005000000000002")
	user.Fields["ProfileId"] = String("00e000000000002")
	return user
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
Map<String,Local_Setting__c> lowerSettings = Local_Setting__c.getall();
System.assertEquals(1, lowerSettings.size());
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

func TestExecSOQLCustomMetadataSelectedAuditFieldIsQueryable(t *testing.T) {
	program, err := CompileAnonymous(`
Feature__mdt cfg = [SELECT Id, CreatedDate FROM Feature__mdt LIMIT 1];
Schema.SObjectField createdDateField = Feature__mdt.SObjectType.getDescribe().fields.getMap().get('CreatedDate');
cfg.get(createdDateField);
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

func TestExecCustomSettingGetInstanceMissingReturnsTypedNull(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, Local_Setting__c.getInstance(null));
System.assertEquals(null, Local_Setting__c.getInstance('Missing'));
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

func TestExecSyntheticNamespacedCustomSettingGetInstanceMissingReturnsTypedNull(t *testing.T) {
	program, err := CompileAnonymous(`
SObject setting = pkg__Local_Setting__c.getInstance();
System.assertEquals(null, setting);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMetadataLightCustomSettingGetInstanceMissingReturnsDefaultRecord(t *testing.T) {
	program, err := CompileAnonymous(`
pkg__Managed_Setting__c setting = pkg__Managed_Setting__c.getInstance();
System.assertEquals(null, setting.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["pkg__Managed_Setting__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Managed_Setting__c",
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

func TestExecCustomDataAccessorsThroughSObjectSurface(t *testing.T) {
	program, err := CompileAnonymous(`
SObject setting = Local_Setting__c.getInstance('Default');
System.assertEquals(1, setting.getAll().size());
System.assertEquals('Default', setting.getValues('Default').get('Name'));
SObject hierarchy = Hierarchy_Setting__c.getInstance();
System.assertEquals(true, hierarchy.getOrgDefaults().get('Enabled__c'));
System.assertEquals(true, hierarchy.getInstance('005000000000001').get('Enabled__c'));
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

func TestExecDescribeSObjectResultIsCustomSetting(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(Local_Setting__c.SObjectType.getDescribe().isCustomSetting());
System.assert(!Feature__mdt.SObjectType.getDescribe().isCustomSetting());
System.assert(!Account.SObjectType.getDescribe().isCustomSetting());
System.assert(Local_Setting__c.SObjectType.isCustomSetting());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields:    map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLCustomMetadataRelationshipProjection(t *testing.T) {
	program, err := CompileAnonymous(`
List<Binding__mdt> rows = [SELECT DeveloperName, Target__r.QualifiedApiName FROM Binding__mdt WHERE Target__r.QualifiedApiName = 'Target'];
List<Binding__mdt> baseRows = [SELECT DeveloperName FROM Binding__mdt];
System.assert(baseRows.size() == 1, 'base rows ' + String.valueOf(baseRows.size()));
List<Binding__mdt> allRows = [SELECT DeveloperName, Target__r.QualifiedApiName FROM Binding__mdt];
System.assert(allRows.size() == 1, 'all rows ' + String.valueOf(allRows.size()));
System.assertEquals('Target', allRows[0].Target__r.QualifiedApiName, 'relationship projection without where');
System.assertEquals(1, rows.size());
System.assertEquals('Default', rows[0].DeveloperName);
System.assertEquals('Target', rows[0].Target__r.QualifiedApiName);
List<Binding__mdt> fieldRows = [SELECT DeveloperName, Field__r.QualifiedApiName FROM Binding__mdt WHERE Field__r.QualifiedApiName = 'Status__c'];
System.assert(fieldRows.size() == 1, 'field where rows ' + String.valueOf(fieldRows.size()));
System.assertEquals('Status__c', fieldRows[0].Field__r.QualifiedApiName);
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
		{"dml", "Feature__mdt cfg = Feature__mdt.getInstance('Default'); update cfg;", "DML cannot modify read-only custom metadata"},
		{"field assignment without org resolution", "Ghost__mdt cfg = new Ghost__mdt(); cfg.__glade_readonly = 'custom metadata'; cfg.Enabled__c = false;", "cannot modify read-only custom metadata"},
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

func TestExecCustomSettingStaticRecordsAreMutable(t *testing.T) {
	program, err := CompileAnonymous(`
Local_Setting__c setting = Local_Setting__c.getInstance('Default');
setting.put('Enabled__c', true);
System.assertEquals(true, Local_Setting__c.getInstance('Default').Enabled__c);
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

func TestExecSparseHierarchyCustomSettingBooleanDefaultIsUsableInCondition(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c defaults = Hierarchy_Setting__c.getOrgDefaults();
if (defaults.Defaulted__c) {
    System.assert(false);
}
System.assertEquals(false, defaults.Defaulted__c);
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

func TestExecConstructedSObjectBooleanDefaultIsUsableInCondition(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c settings = new Hierarchy_Setting__c();
if (settings.Defaulted__c) {
    System.assert(false);
}
System.assertEquals(false, settings.Defaulted__c);
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

func TestExecHierarchyCustomSettingAbsentOrgDefaultsEqualsFreshEmptySObject(t *testing.T) {
	program, err := CompileAnonymous(`
Hierarchy_Setting__c defaults = Hierarchy_Setting__c.getOrgDefaults();
System.assertEquals(false, defaults.Defaulted__c);
System.assertEquals(new Hierarchy_Setting__c(), defaults);
System.assert(defaults == new Hierarchy_Setting__c());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	hierarchy := org.Objects["Hierarchy_Setting__c"]
	hierarchy.Records = map[storage.ID]storage.Record{}
	org.Objects["Hierarchy_Setting__c"] = hierarchy
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListCustomSettingGetAllRefreshesAfterUpsert(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(1, Local_Setting__c.getAll().size());
System.assertEquals(false, Local_Setting__c.getAll().get('Default').Enabled__c);
Local_Setting__c existing = Local_Setting__c.getInstance('Default');
existing.Enabled__c = true;
update existing;
System.assertEquals(true, Local_Setting__c.getAll().get('Default').Enabled__c);
Local_Setting__c setting = new Local_Setting__c(Name = 'Second', Enabled__c = true);
upsert setting;
System.assertEquals(2, Local_Setting__c.getAll().size());
String serialized = JSON.serialize(Local_Setting__c.getAll().get('Default'));
System.assert(serialized.contains('"Optional__c":null'), serialized);
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

func TestExecListCustomSettingGetValuesByName(t *testing.T) {
	program, err := CompileAnonymous(`
Local_Setting__c setting = Local_Setting__c.getValues('Default');
System.assertNotEquals(null, setting);
System.assertEquals(false, setting.Enabled__c);
System.assertEquals(null, Local_Setting__c.getValues('Missing'));
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
				"Name":        {APIName: "Name", Type: storage.FieldString},
				"Enabled__c":  {APIName: "Enabled__c", Type: storage.FieldBoolean},
				"Optional__c": {APIName: "Optional__c", Type: storage.FieldString},
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
			"Field__c":  {APIName: "Field__c", Type: storage.FieldReference, ReferenceTo: []string{"FieldDefinition"}, RelationshipName: "Field__r"},
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
				"Field__c":      storage.StringValue("Status__c"),
			}},
		},
	}
	return org
}
