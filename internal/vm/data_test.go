package vm

import (
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

func TestExecSOQLDateLiterals(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Today', RenewalDate__c = Date.today());
Date oldDate = Date.today();
oldDate = oldDate.addDays(-2);
insert new Account(Name = 'Old', RenewalDate__c = oldDate);
List<Account> todayRows = [SELECT Id FROM Account WHERE RenewalDate__c = TODAY];
System.assertEquals(1, todayRows.size(), 'TODAY should match Date.today row');
List<Account> recentRows = [SELECT Id FROM Account WHERE RenewalDate__c = LAST_N_DAYS:2];
System.assertEquals(2, recentRows.size(), 'LAST_N_DAYS should include today and prior rows');
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
Contact c = new Contact(AccountId = a.Id, LastName = 'Smith');
insert c;
Contact row = [SELECT Id, Account.Name FROM Contact WHERE Id = :c.Id];
System.assertEquals('Acme', row.Account.Name);
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
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
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
