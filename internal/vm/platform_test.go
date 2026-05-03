package vm

import (
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestExecLimitsCountersAndPermissiveViolations(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, Limits.getQueries());
Account a = new Account(Name = 'Acme');
insert a;
List<Account> rows = [SELECT Id, Name FROM Account];
System.assertEquals(1, Limits.getQueries());
System.assertEquals(1, Limits.getQueryRows());
System.assertEquals(1, Limits.getDmlStatements());
System.assertEquals(1, Limits.getDmlRows());
System.assert(Limits.getHeapSize() > 0);
System.assert(Limits.getCpuTime() > 0);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.SetLimitCaps(LimitCaps{
		Queries:       0,
		QueryRows:     100,
		DMLStatements: 100,
		DMLRows:       100,
		HeapSize:      1 << 20,
		CPUTimeMS:     100,
		Callouts:      100,
		AsyncJobs:     50,
	})
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LimitViolations) == 0 || result.LimitViolations[0].Name != "queries" {
		t.Fatalf("violations = %#v", result.LimitViolations)
	}
}

func TestExecSOQLLimitRowsCountsChildSubqueryRows(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
insert new Contact(LastName = 'One', AccountId = a.Id);
insert new Contact(LastName = 'Two', AccountId = a.Id);
List<Account> rows = [SELECT Id, (SELECT Id FROM Contacts) FROM Account WHERE Id = :a.Id];
System.assertEquals(1, Limits.getQueries());
System.assertEquals(3, Limits.getQueryRows());
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

func TestExecDMLLimitRowsCountsCascadeDeletes(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
insert new Contact(LastName = 'One', AccountId = a.Id);
insert new Contact(LastName = 'Two', AccountId = a.Id);
System.assertEquals(3, Limits.getDmlRows());
delete a;
System.assertEquals(6, Limits.getDmlRows());
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
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHeapLimitTracksMutatedCollections(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>();
Integer before = Limits.getHeapSize();
values.add('abcdefghijklmnopqrstuvwxyz');
Integer after = Limits.getHeapSize();
System.assert(after > before);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCPULimitAccountsForSOQLAndDMLRows(t *testing.T) {
	program, err := CompileAnonymous(`
Integer start = Limits.getCpuTime();
List<Account> rows = new List<Account>{
	new Account(Name = 'A'),
	new Account(Name = 'B'),
	new Account(Name = 'C')
};
insert rows;
Integer afterDml = Limits.getCpuTime();
System.assert(afterDml >= start + 3);
List<Account> queried = [SELECT Id FROM Account];
Integer afterQuery = Limits.getCpuTime();
System.assert(afterQuery >= afterDml + 3);
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

func TestExecStrictLimitModeFails(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> rows = [SELECT Id FROM Account];
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.SetLimitMode(LimitModeStrict)
	machine.SetLimitCaps(LimitCaps{
		Queries:       0,
		QueryRows:     100,
		DMLStatements: 100,
		DMLRows:       100,
		HeapSize:      1 << 20,
		CPUTimeMS:     100,
		Callouts:      100,
		AsyncJobs:     50,
	})
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "System.LimitException") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecJSONCommonSerializeOverloads(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme', Phone = null);
String compact = JSON.serialize(a, true);
System.assert(compact.contains('"Name":"Acme"'));
System.assert(!compact.contains('Phone'));
Map<String,Object> values = new Map<String,Object>();
values.put('kept', 'yes');
values.put('dropped', null);
String mapJSON = JSON.serialize(values);
System.assert(mapJSON.contains('kept'));
System.assert(!mapJSON.contains('dropped'));
String pretty = JSON.serializePretty(a);
System.assert(pretty.contains('  "Name"'));
Account decoded = JSON.deserializeStrict('{"Name":"Acme"}', Account.class);
System.assertEquals('Acme', decoded.Name);
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

func TestExecJSONDeserializeStrictRejectsUnknownFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account decoded = JSON.deserializeStrict('{"Name":"Acme","NoSuchField__c":"x"}', Account.class);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecCommonTestPlatformAPIs(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(Test.isRunningTest());
System.assertEquals('01s000000000001', Test.getStandardPricebookId());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseGetQueryLocator(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Object locator = Database.getQueryLocator('SELECT Id, Name FROM Account');
System.assertEquals(1, Limits.getQueries());
System.assertEquals(1, Limits.getQueryRows());
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

func TestExecTypeForNameAndNewInstance(t *testing.T) {
	program, err := CompileAnonymous(`
Type accountType = Type.forName('Account');
System.assertEquals('Account', accountType.getName());
Account account = accountType.newInstance();
account.Name = 'Acme';
System.assertEquals('Acme', account.Name);
Type namespaced = Type.forName('pkg', 'Thing');
System.assertEquals('pkg.Thing', namespaced.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseSavepointRollback(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'before');
System.Savepoint sp = Database.setSavepoint();
insert new Account(Name = 'after');
Integer beforeRollback = [SELECT COUNT() FROM Account];
System.assertEquals(2, beforeRollback);
Database.rollback(sp);
Integer afterRollback = [SELECT COUNT() FROM Account];
System.assertEquals(1, afterRollback);
System.assertEquals(4, Limits.getDmlStatements());
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

func TestExecRollbackInvalidatesLaterSavepoints(t *testing.T) {
	program, err := CompileAnonymous(`
System.Savepoint first = Database.setSavepoint();
insert new Account(Name = 'one');
System.Savepoint second = Database.setSavepoint();
insert new Account(Name = 'two');
Database.rollback(first);
Database.rollback(second);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "invalid Savepoint") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecStandardPricebookIdRequiresTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
String pricebookId = Test.getStandardPricebookId();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "only available in test context") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecStartStopRestoresOuterLimitWindow(t *testing.T) {
	program, err := CompileAnonymous(`
Account beforeStart = new Account(Name = 'Before');
insert beforeStart;
System.assertEquals(1, Limits.getDmlStatements());
Test.startTest();
Account insideWindow = new Account(Name = 'Inside');
insert insideWindow;
System.assertEquals(1, Limits.getDmlStatements());
Test.stopTest();
System.assertEquals(1, Limits.getDmlStatements());
Account afterStop = new Account(Name = 'After');
insert afterStop;
System.assertEquals(2, Limits.getDmlStatements());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformLimitCounters(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, Limits.getEmailInvocations());
System.assertEquals(10, Limits.getLimitEmailInvocations());
Messaging.sendEmail(new List<String>{'hello'});
System.assertEquals(1, Limits.getEmailInvocations());

System.assertEquals(0, Limits.getAsyncJobs());
System.assertEquals(50, Limits.getLimitAsyncJobs());
System.assertEquals(0, Limits.getFutureCalls());
System.assertEquals(0, Limits.getQueueableJobs());
System.assertEquals(0, Limits.getBatchJobs());
System.assertEquals(5, Limits.getLimitBatchJobs());
System.assertEquals(0, Limits.getScheduledJobs());
System.assertEquals(100, Limits.getLimitScheduledJobs());
FutureWorker.mark();
System.enqueueJob(new QueueWorker());
System.assertEquals(2, Limits.getAsyncJobs());
System.assertEquals(1, Limits.getFutureCalls());
System.assertEquals(1, Limits.getQueueableJobs());

Database.executeBatch(new BatchWorker(), 1);
System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
System.assertEquals(4, Limits.getAsyncJobs());
System.assertEquals(1, Limits.getBatchJobs());
System.assertEquals(1, Limits.getScheduledJobs());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	for _, class := range []Class{{Name: "FutureWorker"}, {Name: "QueueWorker"}, {Name: "BatchWorker"}, {Name: "ScheduledWorker"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{Name: "FutureWorker.mark", ClassName: "FutureWorker", IsStatic: true, Modifiers: []string{"future"}}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if result.Limits.FutureCalls != 1 || result.Limits.QueueableJobs != 1 || result.Limits.BatchJobs != 1 || result.Limits.ScheduledJobs != 1 || result.Limits.EmailInvokes != 1 {
		t.Fatalf("limits = %#v", result.Limits)
	}
}

func TestExecRunAsUserObjectScopesUserInfo(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-user-a', ProfileId = '00e-profile-a', Username = 'user-a@example.test')) {
  System.assertEquals('005-user-a', UserInfo.getUserId());
  System.assertEquals('00e-profile-a', UserInfo.getProfileId());
  System.assertEquals('user-a@example.test', UserInfo.getUserName());
}
System.assertEquals('system', UserInfo.getUserId());
System.runAs(new User(Id = '005-user-b', Permissions = new List<String>{'CanRunLocal'})) {
  System.assert(FeatureManagement.checkPermission('CanRunLocal'));
  System.assert(!FeatureManagement.checkPermission('OtherPermission'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsScopesSupportedMixedDMLMode(t *testing.T) {
	fails, err := CompileAnonymous(`
insert new User(Username = 'setup@example.test');
insert new Account(Name = 'Acme');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(fails); err == nil || !strings.Contains(err.Error(), "Mixed DML") {
		t.Fatalf("err = %v", err)
	}

	passes, err := CompileAnonymous(`
insert new User(Username = 'setup@example.test');
System.runAs(new User(Id = '005-user-a', ProfileId = '00e-profile-a', Username = 'user-a@example.test')) {
  insert new Account(Name = 'Acme');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(passes); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformAPIs(t *testing.T) {
	program, err := CompileAnonymous(`
String padded = '  Alpha,Beta,Alpha  ';
String trimmed = padded.trim();
System.assertEquals('Alpha,Beta,Alpha', trimmed);
System.assertEquals(0, trimmed.indexOf('Alpha'));
System.assertEquals(11, trimmed.lastIndexOf('Alpha'));
System.assertEquals('Omega,Beta,Omega', trimmed.replace('Alpha', 'Omega'));
List<String> pieces = trimmed.split(',');
System.assertEquals(3, pieces.size());
System.assertEquals('Alpha|Beta|Alpha', String.join(pieces, '|'));
System.assert(String.isBlank('   '));
System.assert(String.isNotBlank('x'));
System.assert(trimmed.equalsIgnoreCase('alpha,beta,alpha'));
Pattern pattern = Pattern.compile('[A-Z]+');
Matcher matcher = pattern.matcher('abc DEF ghi');
System.assert(matcher.find());
System.assertEquals('DEF', matcher.group());
System.assert(Pattern.matches('[0-9]+', '12345'));
System.assertEquals(5, Math.abs(-5));
System.assertEquals(7, Math.max(3, 7));
System.assertEquals(8.0, Math.pow(2, 3));
System.assertEquals(3.0, Math.sqrt(9));
System.assertEquals(2.0, Math.floor(2.9));
System.assertEquals(3.0, Math.ceil(2.1));
Decimal amount = 12.345;
System.assertEquals(12.35, amount.setScale(2));
System.assertEquals(12, amount.intValue());
System.assertEquals(12, amount.round());
Date d = Date.today();
System.assertEquals('2026-05-02', d.format());
System.assertEquals(2026, d.year());
System.assertEquals(5, d.month());
System.assertEquals(2, d.day());
Date later = d.addDays(3);
System.assertEquals(3, d.daysBetween(later));
Date nextMonth = d.addMonths(1);
System.assertEquals('2026-06-02', nextMonth.format());
Date nextYear = d.addYears(1);
System.assertEquals('2027-05-02', nextYear.format());
Date parsedDate = Date.valueOf('2026-05-04');
System.assertEquals(2, d.daysBetween(parsedDate));
Datetime dt = Datetime.now();
String dtText = dt.format();
System.assert(dtText.startsWith('2026-05-02T12:00:00'));
Date dtDate = dt.date();
System.assertEquals('2026-05-02', dtDate.format());
Datetime made = Datetime.newInstance(2026, 5, 2, 1, 2, 3);
System.assertEquals('2026-05-02T01:02:03Z', made.format());
Datetime madePlusHour = made.addHours(1);
System.assertEquals('2026-05-02T02:02:03Z', madePlusHour.format());
Datetime madePlusMinutes = made.addMinutes(2);
System.assertEquals('2026-05-02T01:04:03Z', madePlusMinutes.format());
Datetime madePlusSeconds = made.addSeconds(3);
System.assertEquals('2026-05-02T01:02:06Z', madePlusSeconds.format());
Datetime madePlusDay = made.addDays(1);
System.assertEquals('2026-05-03T01:02:03Z', madePlusDay.format());
Datetime parsedDt = Datetime.valueOf('2026-05-02 01:02:03');
String madeText = made.format();
String parsedDtText = parsedDt.format();
System.assertEquals(madeText, parsedDtText);
Time tm = Time.valueOf('01:02:03');
System.assertEquals(1, tm.hour());
System.assertEquals(2, tm.minute());
System.assertEquals(3, tm.second());
String encoded = EncodingUtil.base64Encode(Blob.valueOf('abc'));
System.assertEquals('YWJj', encoded);
Blob decoded = EncodingUtil.base64Decode(encoded);
System.assertEquals('616263', EncodingUtil.convertToHex(decoded));
String escaped = EncodingUtil.urlEncode('a b+c', 'UTF-8');
System.assertEquals('a+b%2Bc', escaped);
System.assertEquals('a b+c', EncodingUtil.urlDecode(escaped, 'UTF-8'));
Blob digest = Crypto.generateDigest('SHA-256', Blob.valueOf('abc'));
String digestHex = EncodingUtil.convertToHex(digest);
System.assertEquals(64, digestHex.length());
Blob sha1Digest = Crypto.generateDigest('SHA1', Blob.valueOf('abc'));
String sha1Hex = EncodingUtil.convertToHex(sha1Digest);
System.assertEquals(40, sha1Hex.length());
Blob md5Digest = Crypto.generateDigest('MD5', Blob.valueOf('abc'));
String md5Hex = EncodingUtil.convertToHex(md5Digest);
System.assertEquals(32, md5Hex.length());
String jsonText = JSON.serialize(new Account(Name = 'Acme'));
Object decodedJson = JSON.deserializeUntyped('{"name":"Acme","ok":true}');
System.assertEquals('Acme', decodedJson.get('name'));
Account decodedAccount = JSON.deserialize('{"Name":"Typed"}', Account.class);
System.assertEquals('Typed', decodedAccount.Name);
Type accountType = Account.class;
System.assertEquals('Account', accountType.getName());
Map<String,Object> describe = Schema.getGlobalDescribe();
System.assert(describe.containsKey('Account'));
Object accountDescribe = describe.get('Account');
System.assertEquals('Account', accountDescribe.getName());
System.assert(!FeatureManagement.checkPermission('Example'));
System.assert(!ApexPages.hasMessages());
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

func TestExecDateTimeConstructorsRejectInvalidParts(t *testing.T) {
	for _, source := range []string{
		`Date bad = Date.newInstance(2026, 13, 2);`,
		`Datetime bad = Datetime.newInstance(2026, 5, 32, 1, 2, 3);`,
		`Time bad = Time.newInstance(25, 0, 0);`,
	} {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		machine := New(nil)
		if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("source %q err = %v", source, err)
		}
	}
}

func TestExecDatabaseSaveResultMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Account good = new Account(Name = 'Acme');
Account bad = new Account(Bogus__c = 'nope');
List<Account> records = new List<Account>{good, bad};
List<Object> results = Database.insert(records, false);
Object first = results.get(0);
Object second = results.get(1);
System.assert(first.isSuccess(), 'first row should save');
System.assert(!second.isSuccess(), 'second row should fail');
System.assertNotEquals('', first.getId());
List<Object> errors = second.getErrors();
System.assertEquals(1, errors.size());
Object err = errors.get(0);
String message = err.getMessage();
System.assert(message.contains('unknown field'));
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

func TestExecDatabaseErrorShapeAndUpsertResult(t *testing.T) {
	program, err := CompileAnonymous(`
Account good = new Account(Name = 'Acme');
Account missing = new Account();
Account bad = new Account(Bogus__c = 'nope');
List<Account> records = new List<Account>{good, missing, bad};
List<Object> results = Database.insert(records, false);

Object first = results.get(0);
Object second = results.get(1);
Object third = results.get(2);

System.assert(first.isSuccess());
System.assert(!second.isSuccess());
System.assert(!third.isSuccess());

List<Object> errors2 = second.getErrors();
Object err2 = errors2.get(0);
System.assertEquals('REQUIRED_FIELD_MISSING', err2.getStatusCode());
List<Object> fields2 = err2.getFields();
System.assertEquals(1, fields2.size());
System.assertEquals('Name', fields2.get(0));

List<Object> errors3 = third.getErrors();
Object err3 = errors3.get(0);
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE', err3.getStatusCode());
List<Object> fields3 = err3.getFields();
System.assertEquals(1, fields3.size());
System.assertEquals('Bogus__c', fields3.get(0));

Account upsertNew = new Account(Name = 'NewCo');
Object upsertCreate = Database.upsert(upsertNew, false);
System.assert(upsertCreate.isSuccess());
System.assert(upsertCreate.isCreated());

Account upsertExisting = new Account(Id = upsertCreate.getId(), Name = 'OldCo');
Object upsertUpdate = Database.upsert(upsertExisting, false);
System.assert(upsertUpdate.isSuccess());
System.assert(!upsertUpdate.isCreated());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
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

func TestExecHttpCalloutMockBasics(t *testing.T) {
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new MockResponse(body = 'ok', statusCode = 201));
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test');
req.setMethod('GET');
Http h = new Http();
HttpResponse res = h.send(req);
System.assertEquals(201, res.getStatusCode());
System.assertEquals('ok', res.getBody());
System.assertEquals(1, Limits.getCallouts());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHttpCalloutMockRespondMethod(t *testing.T) {
	respondProgram, err := CompileAnonymous(`
HttpResponse res = new HttpResponse();
res.setStatusCode(202);
res.setBody(req.getBody() + ':mocked');
return res;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new EchoMock());
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test');
req.setMethod('POST');
req.setBody('payload');
Http h = new Http();
HttpResponse res = h.send(req);
System.assertEquals(202, res.getStatusCode());
System.assertEquals('payload:mocked', res.getBody());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterMethod(Method{
		Name:       "EchoMock.respond",
		ClassName:  "EchoMock",
		ReturnType: "HttpResponse",
		Params:     []Param{{Name: "req", Type: "HttpRequest"}},
		Program:    respondProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
