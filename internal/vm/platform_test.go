package vm

import (
	"errors"
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

func TestExecUnsupportedStdlibErrorsHaveStableShape(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "static",
			src:  `System.nope();`,
			want: `unsupported call "System.nope"`,
		},
		{
			name: "instance",
			src:  `String s = 'abc'; s.nope();`,
			want: `unsupported call "s.nope"`,
		},
		{
			name: "search api",
			src:  `Search.find('FIND {Acme} IN ALL FIELDS RETURNING Account(Id)');`,
			want: `unsupported call "Search.find local search/SOSL surface"`,
		},
		{
			name: "inline sosl find",
			src:  `Object rows = [FIND 'Acme' IN ALL FIELDS RETURNING Account(Id)];`,
			want: `unsupported call "SOSL/FIND local search surface"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			if err == nil {
				t.Fatal("expected unsupported feature error")
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("error type = %T, want *RuntimeError", err)
			}
			if runtimeErr.Type != "UnsupportedFeature" {
				t.Fatalf("runtime error type = %q, want UnsupportedFeature", runtimeErr.Type)
			}
			if runtimeErr.Message != tc.want || err.Error() != tc.want {
				t.Fatalf("error = (%q, %q), want %q", runtimeErr.Message, err.Error(), tc.want)
			}
		})
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
System.assert(mapJSON.contains('"dropped":null'));
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
	if !traceHas(result.Trace, "apex.email.send", "apex.email") {
		t.Fatalf("trace missing email event: %#v", result.Trace)
	}
	if !traceHas(result.Trace, "apex.async.enqueue", "apex.async") {
		t.Fatalf("trace missing async enqueue event: %#v", result.Trace)
	}
	if !traceHas(result.Trace, "apex.limits", "apex.limits") {
		t.Fatalf("trace missing limits event: %#v", result.Trace)
	}
}

func TestExecRunAsUserObjectScopesUserInfo(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-user-a', ProfileId = '00e-profile-a', Username = 'user-a@example.test')) {
  System.assertEquals('005-user-a', UserInfo.getUserId());
  System.assertEquals('00e-profile-a', UserInfo.getProfileId());
  System.assertEquals('user-a@example.test', UserInfo.getUserName());
  System.assertEquals('00D000000000001', UserInfo.getOrganizationId());
  System.assertEquals('', UserInfo.getSessionId());
  System.assertEquals('en_US', UserInfo.getLocale());
  System.assertEquals('UTC', UserInfo.getTimeZone());
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

func TestExecMessagingApexPagesAndURLBasics(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage email = new Messaging.SingleEmailMessage();
email.setToAddresses(new List<String>{'user@example.test'});
email.setSubject('Hello');
email.setPlainTextBody('Body');
List<Object> results = Messaging.sendEmail(new List<Object>{email});
Object sendResult = results.get(0);
System.assert(sendResult.isSuccess());
List<Object> sendErrors = sendResult.getErrors();
System.assertEquals(0, sendErrors.size());
System.assert(!ApexPages.hasMessages());
ApexPages.Message message = new ApexPages.Message('ERROR', 'Summary', 'Detail');
ApexPages.addMessage(message);
System.assert(ApexPages.hasMessages());
List<Object> messages = ApexPages.getMessages();
Object firstMessage = messages.get(0);
System.assertEquals('Summary', firstMessage.getSummary());
System.assertEquals('Detail', firstMessage.getDetail());
PageReference page = new PageReference('/apex/TestPage');
System.assertEquals('/apex/TestPage', page.getUrl());
page.setRedirect(true);
System.assert(page.getRedirect());
Map<String,Object> params = page.getParameters();
params.put('id', '001');
Map<String,Object> paramsAgain = page.getParameters();
System.assertEquals('001', paramsAgain.get('id'));
PageReference current = ApexPages.currentPage();
System.assertEquals('/apex/current', current.getUrl());
URL base = URL.getSalesforceBaseUrl();
System.assertEquals('https://local.oaer.example', base.toExternalForm());
URL orgUrl = URL.getOrgDomainUrl();
System.assertEquals('https://local.oaer.example', orgUrl.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
Date parsedDateTime = Date.valueOf('2026-05-04 23:59:58');
System.assertEquals(parsedDate, parsedDateTime);
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
	Object decodedLargeJson = JSON.deserializeUntyped('{"n":9223372036854775808}');
	System.assertEquals(9223372036854775808.0, decodedLargeJson.get('n'));
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

func TestExecDateDatetimeDeterministicInstanceMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Date leap = Date.newInstance(2024, 1, 31);
Date nextMonth = leap.addMonths(1);
System.assertEquals('2024-02-29', nextMonth.format());
Date leapDay = Date.newInstance(2024, 2, 29);
Date nextYear = leapDay.addYears(1);
System.assertEquals('2025-02-28', nextYear.format());
System.assertEquals(31, leap.day());
System.assertEquals(1, leap.month());
System.assertEquals(2024, leap.year());
Date monthStart = leap.toStartOfMonth();
Date monthEnd = leap.toEndOfMonth();
System.assertEquals('2024-01-01', monthStart.format());
System.assertEquals('2024-01-31', monthEnd.format());
Date due = leap.addDays(10);
System.assertEquals(10, leap.daysBetween(due));
System.assertEquals(-10, due.daysBetween(leap));
Date nextDay = leap.addDays(1);
Date expectedNextDay = Date.newInstance(2024, 2, 1);
System.assertEquals(expectedNextDay, nextDay);

Datetime stamp = Datetime.newInstance(2024, 1, 31, 23, 58, 59);
Datetime stampNextMonth = stamp.addMonths(1);
System.assertEquals('2024-02-29T23:58:59Z', stampNextMonth.format());
Datetime plusHour = stamp.addHours(1);
Datetime plusMinutes = plusHour.addMinutes(2);
Datetime plusSeconds = plusMinutes.addSeconds(3);
System.assertEquals('2024-02-01T01:01:02Z', plusSeconds.format());
Datetime tomorrowStamp = stamp.addDays(1);
Date tomorrowDate = tomorrowStamp.date();
System.assertEquals('2024-02-01', tomorrowDate.format());
System.assertEquals(2024, stamp.year());
System.assertEquals(1, stamp.month());
System.assertEquals(31, stamp.day());
System.assertEquals(23, stamp.hour());
System.assertEquals(58, stamp.minute());
System.assertEquals(59, stamp.second());
Datetime midnight = Datetime.newInstance(2024, 1, 31);
System.assertEquals('2024-01-31T00:00:00Z', midnight.format());
Datetime sameStamp = Datetime.newInstance(2024, 1, 31, 23, 58, 59);
System.assertEquals(sameStamp, stamp);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTimeDatetimeGmtAndTimeZoneMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Date today = Date.today();
System.assertEquals('2026-05-02', today.format());

Datetime nowStamp = Datetime.now();
System.assertEquals('2026-05-02T12:00:00Z', nowStamp.formatGmt());
Datetime gmt = Datetime.newInstanceGmt(2024, 2, 29, 23, 59, 58);
Date gmtDate = gmt.dateGmt();
System.assertEquals('2024-02-29', gmtDate.format());
System.assertEquals(Time.newInstance(23, 59, 58, 0), gmt.timeGmt());
Datetime parsedGmt = Datetime.valueOfGmt('2024-02-29 23:59:58');
System.assertEquals('2024-02-29T23:59:58Z', parsedGmt.formatGmt());
Datetime fractionalGmt = Datetime.valueOfGmt('2024-02-29T23:59:58.250Z');
Datetime plusMillis = fractionalGmt.addMilliseconds(750);
System.assertEquals('2024-02-29T23:59:59Z', plusMillis.formatGmt());
System.assertEquals(0, plusMillis.millisecond());

Time clock = Time.newInstance(23, 59, 58, 250);
System.assertEquals(23, clock.hour());
System.assertEquals(59, clock.minute());
System.assertEquals(58, clock.second());
System.assertEquals(250, clock.millisecond());
Time plusSeconds = clock.addSeconds(2);
System.assertEquals('00:00:00.250', plusSeconds.format());
Time plusMilliseconds = clock.addMilliseconds(750);
System.assertEquals('23:59:59', plusMilliseconds.format());
Time plusHours = clock.addHours(1);
System.assertEquals('00:59:58.250', plusHours.format());
Time plusMinutes = clock.addMinutes(-1);
System.assertEquals('23:58:58.250', plusMinutes.format());
System.assertEquals(Time.newInstance(12, 34, 56, 789), Time.valueOf('12:34:56.789'));

TimeZone utc = TimeZone.getTimeZone('UTC');
System.assertEquals('UTC', utc.getID());
System.assertEquals('UTC', utc.getDisplayName());
System.assertEquals(0, utc.getOffset(gmt));
TimeZone offset = TimeZone.getTimeZone('GMT+05:30');
System.assertEquals('GMT+05:30', offset.getID());
System.assertEquals(19800000, offset.getOffset(gmt));
TimeZone west = TimeZone.getTimeZone('UTC-02:00');
System.assertEquals('GMT-02:00', west.getID());
System.assertEquals(-7200000, west.getOffset(gmt));
TimeZone edge = TimeZone.getTimeZone('GMT+14:00');
System.assertEquals('GMT+14:00', edge.getDisplayName());
System.assertEquals(50400000, edge.getOffset(gmt));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatetimePatternFormatting(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime stamp = Datetime.valueOfGmt('2024-02-29T23:05:06.250Z');
System.assertEquals('2024-02-29 23:05:06.250 +0000 UTC', stamp.formatGmt('yyyy-MM-dd HH:mm:ss.SSS Z z'));
System.assertEquals('Thu, Feb 29 2024 11:05 PM', stamp.formatGmt('EEE, MMM d yyyy h:mm a'));
System.assertEquals('2024-03-01 04:35:06.250 +0530 GMT+05:30', stamp.format('yyyy-MM-dd HH:mm:ss.SSS Z z', 'GMT+05:30'));
System.assertEquals('2024-02-29T21:05:06', stamp.format('yyyy-MM-dd''T''HH:mm:ss', 'UTC-02:00'));
System.assertEquals('2024-03-01 13:05:06 +1400 GMT+14:00', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'GMT+14:00'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatetimePatternFormattingRejectsUnsupportedEdges(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "named timezone",
			src:  `Datetime stamp = Datetime.now(); stamp.format('yyyy-MM-dd', 'America/Los_Angeles');`,
			want: "unsupported call",
		},
		{
			name: "unsupported token",
			src:  `Datetime stamp = Datetime.now(); stamp.formatGmt('yyyy-QQ-dd');`,
			want: "unsupported pattern token",
		},
		{
			name: "unsupported millisecond token width",
			src:  `Datetime stamp = Datetime.now(); stamp.formatGmt('yyyy-MM-dd SSSS');`,
			want: "unsupported pattern token",
		},
		{
			name: "unterminated literal",
			src:  `Datetime stamp = Datetime.now(); stamp.formatGmt('yyyy-MM-dd''T');`,
			want: "unterminated quoted literal",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecDateTimeParsingRejectsInvalidText(t *testing.T) {
	cases := []string{
		`Date bad = Date.valueOf('2024-02-30');`,
		`Date bad = Date.valueOf('0000-01-01');`,
		`Datetime bad = Datetime.valueOfGmt('2024-02-29 25:00:00');`,
		`Datetime bad = Datetime.valueOfGmt('0000-01-01 00:00:00');`,
		`Time bad = Time.valueOf('24:00:00');`,
	}
	for _, source := range cases {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := New(nil).Execute(program); err == nil {
			t.Fatalf("source %q expected parse error", source)
		}
	}
}

func TestExecTimeZoneRejectsUnsupportedZones(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "named zone",
			src:  `TimeZone tz = TimeZone.getTimeZone('America/Los_Angeles');`,
			want: `unsupported call "TimeZone.getTimeZone America/Los_Angeles"`,
		},
		{
			name: "trimmed ID",
			src:  `TimeZone tz = TimeZone.getTimeZone(' UTC');`,
			want: `unsupported call "TimeZone.getTimeZone  UTC"`,
		},
		{
			name: "bad minute width",
			src:  `TimeZone tz = TimeZone.getTimeZone('GMT+05:3');`,
			want: `unsupported call "TimeZone.getTimeZone GMT+05:3"`,
		},
		{
			name: "offset outside deterministic slice",
			src:  `TimeZone tz = TimeZone.getTimeZone('GMT+14:01');`,
			want: `unsupported call "TimeZone.getTimeZone GMT+14:01"`,
		},
		{
			name: "display overload",
			src:  `TimeZone tz = TimeZone.getTimeZone('UTC'); tz.getDisplayName(true);`,
			want: `unsupported call "TimeZone.getDisplayName DST/locale overloads"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
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
req.setBodyAsBlob(Blob.valueOf('request-body'));
Blob requestBlob = req.getBodyAsBlob();
System.assertEquals('request-body', req.getBody());
System.assertEquals('726571756573742d626f6479', EncodingUtil.convertToHex(requestBlob));
req.setHeader('X-Test', 'yes');
req.setTimeout(5000);
System.assertEquals('https://example.test', req.getEndpoint());
System.assertEquals('GET', req.getMethod());
System.assertEquals('yes', req.getHeader('x-test'));
System.assertEquals(5000, req.getTimeout());
Http h = new Http();
HttpResponse res = h.send(req);
System.assertEquals(201, res.getStatusCode());
System.assertEquals('ok', res.getBody());
res.setStatus('Created');
res.setHeader('Content-Type', 'text/plain');
System.assertEquals('Created', res.getStatus());
System.assertEquals('text/plain', res.getHeader('content-type'));
Blob bodyBlob = res.getBodyAsBlob();
System.assertEquals('6f6b', EncodingUtil.convertToHex(bodyBlob));
System.assertEquals(1, Limits.getCallouts());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if !traceHas(result.Trace, "apex.callout.http", "apex.callout") {
		t.Fatalf("trace missing callout event: %#v", result.Trace)
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
