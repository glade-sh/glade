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
FutureWorker.mark();
System.enqueueJob(new QueueWorker());
System.assertEquals(2, Limits.getAsyncJobs());
System.assertEquals(1, Limits.getFutureCalls());
System.assertEquals(1, Limits.getQueueableJobs());

Database.executeBatch(new BatchWorker(), 1);
System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
System.assertEquals(4, Limits.getAsyncJobs());
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
System.assertEquals(5, Math.abs(-5));
System.assertEquals(7, Math.max(3, 7));
Date d = Date.today();
System.assertEquals('2026-05-02', d.format());
System.assertEquals(2026, d.year());
System.assertEquals(5, d.month());
System.assertEquals(2, d.day());
Date later = d.addDays(3);
System.assertEquals(3, d.daysBetween(later));
Datetime dt = Datetime.now();
String dtText = dt.format();
System.assert(dtText.startsWith('2026-05-02T12:00:00'));
Date dtDate = dt.date();
System.assertEquals('2026-05-02', dtDate.format());
String encoded = EncodingUtil.base64Encode(Blob.valueOf('abc'));
System.assertEquals('YWJj', encoded);
Blob decoded = EncodingUtil.base64Decode(encoded);
System.assertEquals('616263', EncodingUtil.convertToHex(decoded));
Blob digest = Crypto.generateDigest('SHA-256', Blob.valueOf('abc'));
String digestHex = EncodingUtil.convertToHex(digest);
System.assertEquals(64, digestHex.length());
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
