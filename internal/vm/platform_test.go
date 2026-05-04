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

func TestExecLimitsDMLDocumentedCasingAliases(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, Limits.getDMLStatements());
System.assertEquals(150, Limits.getLimitDMLStatements());
System.assertEquals(0, Limits.getDMLRows());
System.assertEquals(10000, Limits.getLimitDMLRows());
insert new Account(Name = 'Acme');
System.assertEquals(1, Limits.getDMLStatements());
System.assertEquals(1, Limits.getDMLRows());
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
		{
			name: "approval process api",
			src:  `Approval.ProcessWorkitemRequest.setAction('Approve');`,
			want: `unsupported call "Approval.ProcessWorkitemRequest.setAction local approval process and lock surface"`,
		},
		{
			name: "approval lock api",
			src:  `Approval.lock(new Account(Name = 'Acme'));`,
			want: `unsupported call "Approval.lock local approval process and lock surface"`,
		},
		{
			name: "auth token api",
			src:  `Auth.SessionManagement.getCurrentSession();`,
			want: `unsupported call "Auth.SessionManagement.getCurrentSession local authentication token/cloud API surface"`,
		},
		{
			name: "auth oauth api",
			src:  `Auth.JWTUtil.validateJWTWithKeysEndpoint('token', 'https://example.invalid/keys');`,
			want: `unsupported call "Auth.JWTUtil.validateJWTWithKeysEndpoint local authentication token/cloud API surface"`,
		},
		{
			name: "event bus publish",
			src:  `EventBus.publish(new Account(Name = 'Acme'));`,
			want: `unsupported call "EventBus.publish local platform event publish surface"`,
		},
		{
			name: "event bus publish after commit",
			src:  `EventBus.publishAfterCommit(new List<Account>{new Account(Name = 'Acme')});`,
			want: `unsupported call "EventBus.publishAfterCommit local platform event publish surface"`,
		},
		{
			name: "quick action ui",
			src:  `QuickAction.performQuickAction(null);`,
			want: `unsupported call "QuickAction.performQuickAction local quick action UI surface"`,
		},
		{
			name: "quick action describe",
			src:  `QuickAction.describeAvailableActions('Account');`,
			want: `unsupported call "QuickAction.describeAvailableActions local quick action UI surface"`,
		},
		{
			name: "canvas integration",
			src:  `Canvas.EnvironmentContext.getParameters();`,
			want: `unsupported call "Canvas.EnvironmentContext.getParameters local canvas app integration surface"`,
		},
		{
			name: "canvas lifecycle",
			src:  `Canvas.LifecycleHandler.onRender(null);`,
			want: `unsupported call "Canvas.LifecycleHandler.onRender local canvas app integration surface"`,
		},
		{
			name: "continuation static",
			src:  `Continuation.getResponse('request-one');`,
			want: `unsupported call "Continuation.getResponse local continuation callout surface"`,
		},
		{
			name: "continuation add request",
			src:  `Continuation.addHttpRequest(null, null);`,
			want: `unsupported call "Continuation.addHttpRequest local continuation callout surface"`,
		},
		{
			name: "crypto certificate api",
			src:  `Crypto.signWithCertificate('RSA-SHA256', Blob.valueOf('payload'), 'LocalCert');`,
			want: `unsupported call "Crypto.signWithCertificate local deterministic key, certificate, and encryption surfaces"`,
		},
		{
			name: "crypto key wrapper api",
			src:  `Crypto.getKeyStore('LocalKeys');`,
			want: `unsupported call "Crypto.getKeyStore local key, certificate, encryption, and random surfaces"`,
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

func TestExecApexPagesCurrentPageAndSeverityEdges(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference before = ApexPages.currentPage();
before.getParameters().put('before', 'yes');
System.assertEquals('yes', ApexPages.currentPage().getParameters().get('before'));
PageReference replacement = new PageReference('/apex/Replaced');
replacement.getHeaders().put('X-Local', 'true');
Test.setCurrentPage(replacement);
System.assertEquals('/apex/Replaced', ApexPages.currentPage().getUrl());
System.assertEquals('true', ApexPages.currentPage().getHeaders().get('X-Local'));
ApexPages.Severity severity = ApexPages.Severity.ERROR;
System.assertEquals('ERROR', severity.name());
System.assertEquals('ERROR', severity.toString());
System.assertEquals(3, severity.ordinal());
System.assertEquals(5, ApexPages.Severity.values().size());
ApexPages.Message message = new ApexPages.Message(severity, 'Summary', 'Detail');
System.assertEquals(ApexPages.Severity.ERROR, message.getSeverity());
System.assertEquals('Summary', message.getSummary());
System.assertEquals('Detail', message.getDetail());
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

func TestExecFeatureManagementUsesExecutionUserPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(FeatureManagement.checkPermission('CanRunLocal'));
System.assert(!FeatureManagement.checkPermission('OtherPermission'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{
		Object: "User",
		ID:     "005-user-b",
		Fields: map[string]storage.Value{
			"Permissions": {Kind: storage.ValueString, String: "CanRunLocal"},
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecApexPagesPageReferenceAndMessagesEdges(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Trail');
System.assertEquals('/apex/Trail', page.getUrl());
System.assertEquals('/apex/Trail', page.toString());
System.assertEquals('/apex/Trail', String.valueOf(page));
System.assertEquals(false, page.getRedirect());
page.setRedirect(true);
System.assertEquals(true, page.getRedirect());
page.getParameters().put('id', '001B000001DVM9t');
page.getHeaders().put('X-Test', 'yes');
System.assertEquals('001B000001DVM9t', page.getParameters().get('id'));
System.assertEquals('yes', page.getHeaders().get('X-Test'));
PageReference current = ApexPages.currentPage();
System.assertEquals('/apex/current', current.getUrl());
current.getHeaders().put('Accept', 'text/html');
System.assertEquals('text/html', current.getHeaders().get('Accept'));
ApexPages.Message withDetail = new ApexPages.Message('ERROR', 'Summary', 'Detail');
System.assertEquals('ERROR', withDetail.getSeverity());
System.assertEquals('Summary', withDetail.getSummary());
System.assertEquals('Detail', withDetail.getDetail());
ApexPages.Message withoutDetail = new ApexPages.Message('INFO', 'Only summary');
System.assertEquals('Only summary', withoutDetail.getDetail());
ApexPages.addMessage(withDetail);
System.assert(ApexPages.hasMessages());
System.assertEquals(1, ApexPages.getMessages().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPageReferenceLocalMapsStartTypedAndMutable(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference blank = new PageReference();
System.assertEquals('', blank.getUrl());
System.assertEquals('', blank.toString());
System.assertEquals('', String.valueOf(blank));
System.assertEquals(0, blank.getParameters().size());
System.assertEquals(0, blank.getHeaders().size());
blank.getParameters().put('id', '001B000001DVM9t');
blank.getHeaders().put('Accept', 'text/html');
System.assertEquals('001B000001DVM9t', blank.getParameters().get('id'));
System.assertEquals('text/html', blank.getHeaders().get('Accept'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMessagingResultAndUnsupportedEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage msg = new Messaging.SingleEmailMessage();
msg.setToAddresses(new List<String>{'trail@example.test'});
msg.setCcAddresses(new List<String>{'copy@example.test'});
msg.setBccAddresses(new List<String>{'blind@example.test'});
msg.setSubject('Trail');
msg.setPlainTextBody('Body');
msg.setHtmlBody('<p>Body</p>');
msg.setReplyTo('reply@example.test');
msg.setSenderDisplayName('Trail Sender');
msg.setCharset('UTF-8');
msg.setInReplyTo('<message@example.test>');
msg.setReferences('<root@example.test>');
msg.setOrgWideEmailAddressId('0D2000000000001');
msg.setTargetObjectId('003000000000001');
msg.setTemplateId('00X000000000001');
msg.setWhatId('001000000000001');
msg.setSaveAsActivity(false);
msg.setTreatBodiesAsTemplate(false);
msg.setTreatTargetObjectAsRecipient(false);
msg.setUseSignature(false);
msg.setEntityAttachments(new List<String>{'015000000000001'});
msg.setDocumentAttachments(new List<String>{'015000000000002'});
msg.setTargetObjectIds(new List<String>{'003000000000002'});
msg.setOptOutPolicy('FILTER');
msg.setEmailPriority('High');
msg.setBccSender(true);
msg.setFileAttachments(new List<Object>{});
Messaging.SingleEmailMessage second = new Messaging.SingleEmailMessage();
second.setToAddresses(new List<String>{'second@example.test'});
List<Messaging.SendEmailResult> results = Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{msg, second}, false);
System.assertEquals(1, Limits.getEmailInvocations());
System.assertEquals(2, results.size());
System.assert(results.get(0).isSuccess());
System.assert(results.get(1).isSuccess());
System.assertEquals(0, results.get(0).getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "non-list",
			src:  `Messaging.sendEmail('not a list');`,
			want: `Messaging.sendEmail expects List`,
		},
		{
			name: "send-options overload",
			src:  `Messaging.sendEmail(new List<Messaging.SingleEmailMessage>(), 'options');`,
			want: `unsupported call "Messaging.sendEmail send options overloads"`,
		},
		{
			name: "template surface",
			src:  `Messaging.renderStoredEmailTemplate('00X000000000001', '003000000000001', '001000000000001');`,
			want: `unsupported call "Messaging.renderStoredEmailTemplate local messaging transport/template surface"`,
		},
		{
			name: "send-options method",
			src:  `Messaging.SendEmailOptions opts = new Messaging.SendEmailOptions(); opts.setTriggerUserEmail(true);`,
			want: `unsupported call "Messaging.SendEmailOptions.setTriggerUserEmail local messaging send-options surface"`,
		},
		{
			name: "setter-type",
			src:  `Messaging.SingleEmailMessage msg = new Messaging.SingleEmailMessage(); msg.setHtmlBody(7);`,
			want: `Messaging.SingleEmailMessage.setHtmlBody expects String`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
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

func TestExecTestClearApexPageMessages(t *testing.T) {
	program, err := CompileAnonymous(`
ApexPages.addMessage(new ApexPages.Message('ERROR', 'Summary', 'Detail'));
System.assert(ApexPages.hasMessages());
System.assertEquals(1, ApexPages.getMessages().size());
Test.clearApexPageMessages();
System.assert(!ApexPages.hasMessages());
System.assertEquals(0, ApexPages.getMessages().size());
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

func TestExecMessagingMassEmailLocalShape(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.MassEmailMessage mass = new Messaging.MassEmailMessage();
mass.setTargetObjectIds(new List<String>{'003000000000001', '003000000000002'});
mass.setWhatIds(new List<String>{'001000000000001'});
mass.setTemplateId('00X000000000001');
mass.setDescription('Trail mass email');
mass.setOptOutPolicy('FILTER');
mass.setSaveAsActivity(false);
List<Messaging.SendEmailResult> results = Messaging.sendEmail(new List<Messaging.MassEmailMessage>{mass});
System.assertEquals(1, Limits.getEmailInvocations());
System.assertEquals(1, results.size());
System.assert(results.get(0).isSuccess());
System.assertEquals(0, results.get(0).getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestClearApexPageMessagesRequiresTestContext(t *testing.T) {
	program, err := CompileAnonymous(`Test.clearApexPageMessages();`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err == nil || !strings.Contains(err.Error(), "Test.clearApexPageMessages is only available in test context") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecTestSetMockRequiresTestContext(t *testing.T) {
	program, err := CompileAnonymous(`Test.setMock('HttpCalloutMock', new MockResponse());`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err == nil || !strings.Contains(err.Error(), "Test.setMock is only available in test context") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecTestSetMockAcceptsTypeTokenForHttpMock(t *testing.T) {
	program, err := CompileAnonymous(`Test.setMock(HttpCalloutMock.class, new MockResponse());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err = machine.Execute(program); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestExecTestSetMockRejectsUnsupportedMockType(t *testing.T) {
	program, err := CompileAnonymous(`Test.setMock('WebServiceMock', new MockResponse());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Test.setMock WebServiceMock mock surface"` {
		t.Fatalf("err = %#v, want UnsupportedFeature WebServiceMock", err)
	}
}

func TestExecUnsupportedTestHelperAPIsHaveStableShape(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "createStub",
			src:  `Test.createStub(Account.class, null);`,
			want: `unsupported call "Test.createStub local stub API"`,
		},
		{
			name: "createSoqlStub",
			src:  `Test.createSoqlStub(Account.class, 'SELECT Id FROM Account');`,
			want: `unsupported call "Test.createSoqlStub local stub API"`,
		},
		{
			name: "setFixedSearchResults",
			src:  `Test.setFixedSearchResults(new List<String>{'001000000000001'});`,
			want: `unsupported call "Test.setFixedSearchResults local SOSL fixed search results"`,
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
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want || err.Error() != tc.want {
				t.Fatalf("err = %#v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecDatabaseGetQueryLocator(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Object locator = Database.getQueryLocator('SELECT Id, Name FROM Account');
System.assertEquals(1, Limits.getQueries());
System.assertEquals(1, Limits.getQueryRows());
System.assertEquals('SELECT Id, Name FROM Account', locator.getQuery());
Object iterator = locator.iterator();
System.assert(iterator.hasNext());
Account row = iterator.next();
System.assertEquals('Acme', row.Name);
System.assert(!iterator.hasNext());
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

func TestExecStartStopRestoresOuterLimitViolations(t *testing.T) {
	program, err := CompileAnonymous(`
Account beforeStart = new Account(Name = 'Before');
insert beforeStart;
System.assertEquals(1, Limits.getDmlStatements());
Test.startTest();
insert new Account(Name = 'Inside');
System.assertEquals(1, Limits.getDmlStatements());
Test.stopTest();
System.assertEquals(1, Limits.getDmlStatements());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.SetLimitCaps(LimitCaps{
		Queries:       100,
		QueryRows:     50000,
		DMLStatements: 0,
		DMLRows:       10000,
		HeapSize:      6 * 1024 * 1024,
		CPUTimeMS:     10000,
		Callouts:      100,
		AsyncJobs:     50,
		FutureCalls:   50,
		QueueableJobs: 50,
		BatchJobs:     5,
		ScheduledJobs: 100,
		EmailInvokes:  10,
	})
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LimitViolations) != 1 {
		t.Fatalf("violations = %#v, want one restored parent violation", result.LimitViolations)
	}
	got := result.LimitViolations[0]
	if got.Name != "dmlStatements" || got.Used != 1 || got.Limit != 0 {
		t.Fatalf("violation = %#v, want restored parent dmlStatements 1/0", got)
	}
}

func TestExecPublishImmediateDMLLimitsGettersUnsupported(t *testing.T) {
	for _, getter := range []string{"getPublishImmediateDML", "getLimitPublishImmediateDML"} {
		t.Run(getter, func(t *testing.T) {
			program, err := CompileAnonymous("Integer used = Limits." + getter + "();")
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
			want := `unsupported call "Limits.` + getter + `"`
			if runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != want || err.Error() != want {
				t.Fatalf("error = (%q, %q, %q), want UnsupportedFeature %q", runtimeErr.Type, runtimeErr.Message, err.Error(), want)
			}
		})
	}
}

func TestExecUnsupportedLimitsGettersHaveStableShape(t *testing.T) {
	for _, getter := range []string{
		"getAggregateQueries",
		"getLimitAggregateQueries",
		"getFindSimilarCalls",
		"getLimitFindSimilarCalls",
		"getMobilePushApexCalls",
		"getLimitMobilePushApexCalls",
		"getPublishImmediateDML",
		"getLimitPublishImmediateDML",
		"getQueryLocatorRows",
		"getLimitQueryLocatorRows",
		"getSavepointRollbacks",
		"getLimitSavepointRollbacks",
		"getSoslQueries",
		"getLimitSoslQueries",
	} {
		t.Run(getter, func(t *testing.T) {
			program, err := CompileAnonymous("Integer used = Limits." + getter + "();")
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
			want := `unsupported call "Limits.` + getter + `"`
			if runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != want || err.Error() != want {
				t.Fatalf("error = (%q, %q, %q), want UnsupportedFeature %q", runtimeErr.Type, runtimeErr.Message, err.Error(), want)
			}
		})
	}
}

func TestAsyncAndEmailCapsHaveStableStrictAndPermissiveShape(t *testing.T) {
	cases := []struct {
		name string
		cap  func(*LimitCaps)
	}{
		{name: "futureCalls", cap: func(caps *LimitCaps) { caps.FutureCalls = 0 }},
		{name: "queueableJobs", cap: func(caps *LimitCaps) { caps.QueueableJobs = 0 }},
		{name: "batchJobs", cap: func(caps *LimitCaps) { caps.BatchJobs = 0 }},
		{name: "scheduledJobs", cap: func(caps *LimitCaps) { caps.ScheduledJobs = 0 }},
		{name: "emailInvocations", cap: func(caps *LimitCaps) { caps.EmailInvokes = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/strict", func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			caps := defaultLimitCaps()
			tc.cap(&caps)
			machine.SetLimitCaps(caps)
			err := machine.incrementLimit(tc.name, 1)
			if err == nil {
				t.Fatal("expected strict limit error")
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("error type = %T, want *RuntimeError", err)
			}
			if runtimeErr.Type != "System.LimitException" || runtimeErr.Message != "Too many "+tc.name+": 1 out of 0" {
				t.Fatalf("runtime error = %#v", runtimeErr)
			}
		})
		t.Run(tc.name+"/permissive", func(t *testing.T) {
			machine := New(nil)
			caps := defaultLimitCaps()
			tc.cap(&caps)
			machine.SetLimitCaps(caps)
			if err := machine.incrementLimit(tc.name, 1); err != nil {
				t.Fatal(err)
			}
			if len(machine.limitViolations) != 1 || machine.limitViolations[0].Name != tc.name {
				t.Fatalf("violations = %#v", machine.limitViolations)
			}
		})
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

func TestExecExecuteBatchRejectsScopeAbovePlatformMaximum(t *testing.T) {
	program, err := CompileAnonymous(`Database.executeBatch(new BatchWorker(), 2001);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "BatchWorker"}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "scope size must be at most 2000") {
		t.Fatalf("err = %v, want scope maximum", err)
	}
}

func TestExecAbortJobRemovesQueuedLocalJobs(t *testing.T) {
	program, err := CompileAnonymous(`
Test.startTest();
String queueId = System.enqueueJob(new QueueWorker());
String scheduleId = System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
System.abortJob(queueId);
System.abortJob(scheduleId);
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	for _, class := range []Class{{Name: "QueueWorker"}, {Name: "ScheduledWorker"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	jobs := org.Objects["AsyncApexJob"].Records
	if len(jobs) != 2 {
		t.Fatalf("AsyncApexJob records = %d, want 2", len(jobs))
	}
	for id, record := range jobs {
		status := record.Fields["Status"]
		if status.Kind != storage.ValueString || status.String != "Aborted" {
			t.Fatalf("job %s status = %#v, want Aborted", id, status)
		}
	}
	cron := org.Objects["CronTrigger"].Records[storage.ID("08e000000000002")]
	if state := cron.Fields["State"]; state.Kind != storage.ValueString || state.String != "Deleted" {
		t.Fatalf("cron state = %#v, want Deleted", state)
	}
}

func TestExecAbortJobCompletedAndUnknownRecordsAreTypedUnsupported(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "completed",
			src:  `Test.startTest(); String id = System.enqueueJob(new QueueWorker()); Test.stopTest(); System.abortJob(id);`,
			want: `unsupported call "System.abortJob completed local async records"`,
		},
		{
			name: "unknown",
			src:  `System.abortJob('707000000999999');`,
			want: `unsupported call "System.abortJob unknown local async records"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := storage.NewOrgState()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "QueueWorker"}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "QueueWorker.execute", ClassName: "QueueWorker"}); err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
	}
}

func TestExecAsyncUnsupportedEdgesAreTyped(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "enqueue async options",
			src:  `System.enqueueJob(new QueueWorker(), new AsyncOptions());`,
			want: `unsupported call "System.enqueueJob AsyncOptions overload"`,
		},
		{
			name: "async options getter",
			src:  `AsyncOptions opts = new AsyncOptions(); opts.getMaximumQueueableStackDepth();`,
			want: `unsupported call "AsyncOptions.getMaximumQueueableStackDepth local async options surface"`,
		},
		{
			name: "async options setter",
			src:  `AsyncOptions opts = new AsyncOptions(); opts.setMinimumQueueableDelayInMinutes(1);`,
			want: `unsupported call "AsyncOptions.setMinimumQueueableDelayInMinutes local async options surface"`,
		},
		{
			name: "async info",
			src:  `AsyncInfo.getCurrentQueueableStackDepth();`,
			want: `unsupported call "AsyncInfo.getCurrentQueueableStackDepth local async info surface"`,
		},
		{
			name: "finalizer",
			src:  `System.attachFinalizer(new QueueWorker());`,
			want: `unsupported call "System.attachFinalizer local queueable finalizers"`,
		},
		{
			name: "finalizer context job id",
			src:  `FinalizerContext fc = new FinalizerContext(); fc.getAsyncApexJobId();`,
			want: `unsupported call "FinalizerContext.getAsyncApexJobId local queueable finalizers"`,
		},
		{
			name: "finalizer context result",
			src:  `System.FinalizerContext fc = new System.FinalizerContext(); fc.getResult();`,
			want: `unsupported call "System.FinalizerContext.getResult local queueable finalizers"`,
		},
		{
			name: "schedule batch",
			src:  `Database.scheduleBatch(null, 'nightly', 1, 200);`,
			want: `unsupported call "Database.scheduleBatch local async scheduling surface"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "QueueWorker"}); err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
	}
}

func TestExecRunAsUserObjectScopesUserInfo(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-user-a', ProfileId = '00e-profile-a', Username = 'user-a@example.test', LocaleSidKey = 'de_DE', LanguageLocaleKey = 'fr')) {
  System.assertEquals('005-user-a', UserInfo.getUserId());
  System.assertEquals('00e-profile-a', UserInfo.getProfileId());
  System.assertEquals('user-a@example.test', UserInfo.getUserName());
  System.assertEquals('System User', UserInfo.getName());
  System.assertEquals('System', UserInfo.getFirstName());
  System.assertEquals('User', UserInfo.getLastName());
  System.assertEquals('system@example.invalid', UserInfo.getUserEmail());
  System.assertEquals('00D000000000001', UserInfo.getOrganizationId());
  System.assertEquals('', UserInfo.getSessionId());
  System.assertEquals('de_DE', UserInfo.getLocale());
  System.assertEquals('fr', UserInfo.getLanguage());
  TimeZone tz = UserInfo.getTimeZone();
  System.assertEquals('UTC', tz.getID());
  System.assertEquals('UTC', tz.getDisplayName());
}
System.assertEquals('system', UserInfo.getUserId());
System.runAs(new User(Id = '005-user-b', FirstName = 'Ada', LastName = 'Trail', Name = 'Ada Trail', Email = 'ada@example.test', Permissions = new List<String>{'CanRunLocal'})) {
  System.assertEquals('Ada Trail', UserInfo.getName());
  System.assertEquals('Ada', UserInfo.getFirstName());
  System.assertEquals('Trail', UserInfo.getLastName());
  System.assertEquals('ada@example.test', UserInfo.getUserEmail());
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

func TestExecCurrentUserTimeZoneScopesUserInfoAndDatetimeFormat(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime winter = Datetime.valueOfGmt('2024-02-29T23:05:06Z');
Datetime summer = Datetime.valueOfGmt('2024-07-01T12:00:00Z');
TimeZone tz = UserInfo.getTimeZone();
System.assertEquals('America/Los_Angeles', tz.getID());
System.assertEquals('America/Los_Angeles', tz.getDisplayName());
System.assertEquals(-28800000, tz.getOffset(winter));
System.assertEquals(-25200000, tz.getOffset(summer));
System.assertEquals('2024-02-29 15:05:06 -0800 PST', winter.format('yyyy-MM-dd HH:mm:ss Z z'));
System.assertEquals('2024-07-01 05:00:00 -0700 PDT', summer.format('yyyy-MM-dd HH:mm:ss Z z'));
System.assertEquals('2024-02-29T15:05:06-08:00', winter.format());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{
		ID:     "005-local-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	easternProgram, err := CompileAnonymous(`
Datetime winter = Datetime.valueOfGmt('2024-02-29T23:05:06Z');
Datetime summer = Datetime.valueOfGmt('2024-07-01T12:00:00Z');
TimeZone eastern = UserInfo.getTimeZone();
System.assertEquals('America/New_York', eastern.getID());
System.assertEquals(-18000000, eastern.getOffset(winter));
System.assertEquals(-14400000, eastern.getOffset(summer));
System.assertEquals('2024-02-29 18:05:06 -0500 EST', winter.format('yyyy-MM-dd HH:mm:ss Z z'));
System.assertEquals('2024-07-01T08:00:00-04:00', summer.format());
`)
	if err != nil {
		t.Fatal(err)
	}
	easternMachine := New(nil)
	easternMachine.SetCurrentUser(storage.Record{
		ID:     "005-eastern-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/New_York"),
		},
	})
	if _, err := easternMachine.Execute(easternProgram); err != nil {
		t.Fatal(err)
	}

	denverProgram, err := CompileAnonymous(`
Datetime winter = Datetime.valueOfGmt('2024-02-29T23:05:06Z');
Datetime summer = Datetime.valueOfGmt('2024-07-01T12:00:00Z');
TimeZone mountain = UserInfo.getTimeZone();
System.assertEquals('America/Denver', mountain.getID());
System.assertEquals(-25200000, mountain.getOffset(winter));
System.assertEquals(-21600000, mountain.getOffset(summer));
System.assertEquals('2024-02-29 16:05:06 -0700 MST', winter.format('yyyy-MM-dd HH:mm:ss Z z'));
System.assertEquals('2024-07-01T06:00:00-06:00', summer.format());
`)
	if err != nil {
		t.Fatal(err)
	}
	denverMachine := New(nil)
	denverMachine.SetCurrentUser(storage.Record{
		ID:     "005-denver-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/Denver"),
		},
	})
	if _, err := denverMachine.Execute(denverProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatetimeLocalConstructionAndComponentsUseCurrentUserTimeZone(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime winterLocal = Datetime.newInstance(2024, 2, 29, 23, 5, 6);
System.assertEquals('2024-03-01T07:05:06Z', winterLocal.formatGmt());
System.assertEquals('2024-02-29T23:05:06-08:00', winterLocal.format());
System.assertEquals('2024-02-29', winterLocal.date().format());
System.assertEquals('2024-03-01', winterLocal.dateGmt().format());
System.assertEquals(Time.newInstance(23, 5, 6, 0), winterLocal.time());
System.assertEquals(Time.newInstance(7, 5, 6, 0), winterLocal.timeGmt());
System.assertEquals(2024, winterLocal.year());
System.assertEquals(2, winterLocal.month());
System.assertEquals(29, winterLocal.day());
System.assertEquals(23, winterLocal.hour());
System.assertEquals(5, winterLocal.minute());
System.assertEquals(6, winterLocal.second());
System.assertEquals(2024, winterLocal.yearGmt());
System.assertEquals(3, winterLocal.monthGmt());
System.assertEquals(1, winterLocal.dayGmt());
System.assertEquals(7, winterLocal.hourGmt());
System.assertEquals(5, winterLocal.minuteGmt());
System.assertEquals(6, winterLocal.secondGmt());

Datetime fromDateTime = Datetime.newInstance(Date.newInstance(2024, 7, 1), Time.newInstance(5, 30, 0, 250));
System.assertEquals('2024-07-01T12:30:00.25Z', fromDateTime.formatGmt());
System.assertEquals('2024-07-01T05:30:00.25-07:00', fromDateTime.format());
Datetime fromDateTimeGmt = Datetime.newInstanceGmt(Date.newInstance(2024, 7, 1), Time.newInstance(5, 30, 0, 250));
System.assertEquals('2024-07-01T05:30:00.25Z', fromDateTimeGmt.formatGmt());

Datetime gap = Datetime.newInstance(2024, 3, 10, 2, 30, 0);
System.assertEquals('2024-03-10T10:30:00Z', gap.formatGmt());
System.assertEquals('2024-03-10T03:30:00-07:00', gap.format());
System.assertEquals(3, gap.hour());
System.assertEquals(10, gap.hourGmt());

Datetime overlap = Datetime.newInstance(2024, 11, 3, 1, 30, 0);
System.assertEquals('2024-11-03T08:30:00Z', overlap.formatGmt());
System.assertEquals('2024-11-03T01:30:00-07:00', overlap.format());
System.assertEquals(1, overlap.hour());
System.assertEquals(8, overlap.hourGmt());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{
		ID:     "005-pacific-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatetimeLocalConstructionUsesRunAsTimeZone(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-ny-user', TimeZoneSidKey = 'America/New_York')) {
    Datetime stamp = Datetime.newInstance(Date.newInstance(2024, 7, 1), Time.newInstance(8, 0, 0, 0));
    System.assertEquals('2024-07-01T12:00:00Z', stamp.formatGmt());
    System.assertEquals(8, stamp.hour());
    System.assertEquals(12, stamp.hourGmt());
    System.assertEquals('2024-07-01', stamp.date().format());
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

func TestExecLocalCurrentUserContextDoesNotEnableRunAs(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('005-local-user', UserInfo.getUserId());
System.assertEquals('local@example.test', UserInfo.getUserName());
System.assertEquals('local-email@example.test', UserInfo.getUserEmail());
System.assertEquals('es_MX', UserInfo.getLocale());
System.assertEquals('es', UserInfo.getLanguage());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{
		ID:     "005-local-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"Username":          storage.StringValue("local@example.test"),
			"Email":             storage.StringValue("local-email@example.test"),
			"LocaleSidKey":      storage.StringValue("es_MX"),
			"LanguageLocaleKey": storage.StringValue("es"),
		},
	})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	runAsProgram, err := CompileAnonymous(`System.runAs(new User(Id = '005-other')) { System.assert(true); }`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(runAsProgram)
	if err == nil || !strings.Contains(err.Error(), "System.runAs is only available in test context") {
		t.Fatalf("runAs err = %v", err)
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
Object accountTypeToken = describe.get('Account');
Object accountDescribe = accountTypeToken.getDescribe();
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
Date marchEnd = Date.newInstance(2024, 3, 31);
Date previousMonth = marchEnd.addMonths(-1);
System.assertEquals('2024-02-29', previousMonth.format());
Date leapDay = Date.newInstance(2024, 2, 29);
Date nextYear = leapDay.addYears(1);
System.assertEquals('2025-02-28', nextYear.format());
Date previousYear = leapDay.addYears(-1);
System.assertEquals('2023-02-28', previousYear.format());
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
Datetime stampPreviousMonth = stamp.addMonths(-1);
System.assertEquals('2023-12-31T23:58:59Z', stampPreviousMonth.format());
Datetime leapStamp = Datetime.newInstance(2024, 2, 29, 1, 2, 3);
Datetime leapStampNextYear = leapStamp.addYears(1);
System.assertEquals('2025-02-28T01:02:03Z', leapStampNextYear.format());
Datetime leapStampPreviousYear = leapStamp.addYears(-1);
System.assertEquals('2023-02-28T01:02:03Z', leapStampPreviousYear.format());
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

func TestExecUserInfoGetTimeZoneRejectsUnsupportedCurrentUserZone(t *testing.T) {
	program, err := CompileAnonymous(`TimeZone zone = UserInfo.getTimeZone();`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetCurrentUser(storage.Record{
		ID:     "005-phoenix-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"TimeZoneSidKey": storage.StringValue("America/Phoenix"),
		},
	})
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "TimeZone.getTimeZone America/Phoenix"` {
		t.Fatalf("err = %#v, want UnsupportedFeature for unsupported current user timezone", err)
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
TimeZone pacific = TimeZone.getTimeZone('America/Los_Angeles');
System.assertEquals('America/Los_Angeles', pacific.getID());
System.assertEquals('America/Los_Angeles', pacific.getDisplayName());
System.assertEquals('PST', pacific.getDisplayName(false));
System.assertEquals('PDT', pacific.getDisplayName(true));
System.assertEquals(-28800000, pacific.getOffset(gmt));
Datetime summerNoon = Datetime.valueOfGmt('2024-07-01T12:00:00Z');
System.assertEquals(-25200000, pacific.getOffset(summerNoon));
TimeZone eastern = TimeZone.getTimeZone('America/New_York');
System.assertEquals('America/New_York', eastern.getID());
System.assertEquals('America/New_York', eastern.getDisplayName());
System.assertEquals(-18000000, eastern.getOffset(gmt));
System.assertEquals(-14400000, eastern.getOffset(summerNoon));
TimeZone central = TimeZone.getTimeZone('America/Chicago');
System.assertEquals('America/Chicago', central.getID());
System.assertEquals('America/Chicago', central.getDisplayName());
System.assertEquals(-21600000, central.getOffset(gmt));
System.assertEquals(-18000000, central.getOffset(summerNoon));
TimeZone mountain = TimeZone.getTimeZone('America/Denver');
System.assertEquals('America/Denver', mountain.getID());
System.assertEquals('America/Denver', mountain.getDisplayName());
System.assertEquals(-25200000, mountain.getOffset(gmt));
System.assertEquals(-21600000, mountain.getOffset(summerNoon));
TimeZone london = TimeZone.getTimeZone('Europe/London');
System.assertEquals('Europe/London', london.getID());
System.assertEquals('Europe/London', london.getDisplayName());
System.assertEquals(0, london.getOffset(gmt));
System.assertEquals(3600000, london.getOffset(summerNoon));
TimeZone berlin = TimeZone.getTimeZone('Europe/Berlin');
System.assertEquals('Europe/Berlin', berlin.getID());
System.assertEquals('Europe/Berlin', berlin.getDisplayName());
System.assertEquals(3600000, berlin.getOffset(gmt));
System.assertEquals(7200000, berlin.getOffset(summerNoon));
TimeZone tokyo = TimeZone.getTimeZone('Asia/Tokyo');
System.assertEquals('Asia/Tokyo', tokyo.getID());
System.assertEquals('Asia/Tokyo', tokyo.getDisplayName());
System.assertEquals('JST', tokyo.getDisplayName(false));
System.assertEquals('JST', tokyo.getDisplayName(true));
System.assertEquals(32400000, tokyo.getOffset(gmt));
System.assertEquals(32400000, tokyo.getOffset(summerNoon));
TimeZone sydney = TimeZone.getTimeZone('Australia/Sydney');
System.assertEquals('Australia/Sydney', sydney.getID());
System.assertEquals('Australia/Sydney', sydney.getDisplayName());
System.assertEquals(39600000, sydney.getOffset(gmt));
System.assertEquals(36000000, sydney.getOffset(summerNoon));
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
System.assertEquals('2024-02-29 15:05:06 -0800 PST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'America/Los_Angeles'));
Datetime summer = Datetime.valueOfGmt('2024-07-01T12:00:00Z');
System.assertEquals('2024-07-01 05:00:00 -0700 PDT', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'America/Los_Angeles'));
System.assertEquals('2024-02-29 18:05:06 -0500 EST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'America/New_York'));
System.assertEquals('2024-07-01 08:00:00 -0400 EDT', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'America/New_York'));
System.assertEquals('2024-02-29 17:05:06 -0600 CST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'America/Chicago'));
System.assertEquals('2024-07-01 07:00:00 -0500 CDT', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'America/Chicago'));
System.assertEquals('2024-02-29 16:05:06 -0700 MST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'America/Denver'));
System.assertEquals('2024-07-01 06:00:00 -0600 MDT', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'America/Denver'));
System.assertEquals('2024-02-29 23:05:06 +0000 GMT', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Europe/London'));
System.assertEquals('2024-07-01 13:00:00 +0100 BST', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'Europe/London'));
System.assertEquals('2024-03-01 00:05:06 +0100 CET', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Europe/Berlin'));
System.assertEquals('2024-07-01 14:00:00 +0200 CEST', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'Europe/Berlin'));
System.assertEquals('2024-03-01 08:05:06 +0900 JST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Asia/Tokyo'));
System.assertEquals('2024-07-01 21:00:00 +0900 JST', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'Asia/Tokyo'));
System.assertEquals('2024-03-01 10:05:06 +1100 AEDT', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Australia/Sydney'));
System.assertEquals('2024-07-01 22:00:00 +1000 AEST', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'Australia/Sydney'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamedTimeZoneDSTBoundaries(t *testing.T) {
	program, err := CompileAnonymous(`
TimeZone central = TimeZone.getTimeZone('America/Chicago');
System.assertEquals(-21600000, central.getOffset(Datetime.valueOfGmt('2024-03-10T07:59:59Z')));
System.assertEquals(-18000000, central.getOffset(Datetime.valueOfGmt('2024-03-10T08:00:00Z')));
System.assertEquals(-18000000, central.getOffset(Datetime.valueOfGmt('2024-11-03T06:59:59Z')));
System.assertEquals(-21600000, central.getOffset(Datetime.valueOfGmt('2024-11-03T07:00:00Z')));

TimeZone london = TimeZone.getTimeZone('Europe/London');
System.assertEquals(0, london.getOffset(Datetime.valueOfGmt('2024-03-31T00:59:59Z')));
System.assertEquals(3600000, london.getOffset(Datetime.valueOfGmt('2024-03-31T01:00:00Z')));
System.assertEquals(3600000, london.getOffset(Datetime.valueOfGmt('2024-10-27T00:59:59Z')));
System.assertEquals(0, london.getOffset(Datetime.valueOfGmt('2024-10-27T01:00:00Z')));

TimeZone sydney = TimeZone.getTimeZone('Australia/Sydney');
System.assertEquals(39600000, sydney.getOffset(Datetime.valueOfGmt('2024-04-06T15:59:59Z')));
System.assertEquals(36000000, sydney.getOffset(Datetime.valueOfGmt('2024-04-06T16:00:00Z')));
System.assertEquals(36000000, sydney.getOffset(Datetime.valueOfGmt('2024-10-05T15:59:59Z')));
System.assertEquals(39600000, sydney.getOffset(Datetime.valueOfGmt('2024-10-05T16:00:00Z')));

Datetime stamp = Datetime.valueOfGmt('2024-03-31T01:00:00Z');
System.assertEquals('2024-03-31 02:00:00 +0100 BST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Europe/London'));
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
			name: "unknown named timezone",
			src:  `Datetime stamp = Datetime.now(); stamp.format('yyyy-MM-dd', 'America/Phoenix');`,
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
		{
			name: "locale dependent pattern token",
			src:  `Datetime stamp = Datetime.now(); stamp.formatGmt('LLLL');`,
			want: `unsupported call "Datetime.format locale-dependent pattern token \"LLLL\""`,
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
			name: "display locale overload",
			src:  `TimeZone tz = TimeZone.getTimeZone('UTC'); tz.getDisplayName(true, 0);`,
			want: `unsupported call "TimeZone.getDisplayName locale/style overloads"`,
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

func TestExecDatabaseResultAccessorsAcrossLocalDML(t *testing.T) {
	program, err := CompileAnonymous(`
Account base = new Account(Name = 'Base');
Database.SaveResult inserted = Database.insert(base, false);
System.assert(inserted.isSuccess());
System.assertNotEquals(null, inserted.getId());
System.assertEquals(0, inserted.getErrors().size());

Database.SaveResult badInsert = Database.insert(new Account(Bogus__c = 'nope'), false);
System.assert(!badInsert.isSuccess());
System.assertEquals(null, badInsert.getId());
Object badInsertError = badInsert.getErrors().get(0);
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE', badInsertError.getStatusCode());
System.assert(badInsertError.getMessage().contains('unknown field'));
System.assertEquals('Bogus__c', badInsertError.getFields().get(0));
System.assertEquals(0, badInsertError.getExtendedErrorDetails().size());

Database.SaveResult updated = Database.update(new Account(Id = inserted.getId(), Name = 'Changed'), false);
System.assert(updated.isSuccess());
System.assertEquals(inserted.getId(), updated.getId());

Account formulaWrite = new Account(Id = inserted.getId());
formulaWrite.put('Score__c', null);
Database.SaveResult badUpdate = Database.update(formulaWrite, false);
System.assert(!badUpdate.isSuccess());
Object badUpdateError = badUpdate.getErrors().get(0);
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE', badUpdateError.getStatusCode());
System.assertEquals('Score__c', badUpdateError.getFields().get(0));

Account upsertNew = new Account(Name = 'Upsert New', External_Key__c = 'ext-1');
Database.UpsertResult upsertCreated = Database.upsert(upsertNew, false);
System.assert(upsertCreated.isSuccess());
System.assert(upsertCreated.isCreated());
Account upsertExisting = new Account(External_Key__c = 'EXT-1', Name = 'Upsert Changed');
Database.UpsertResult upsertUpdated = Database.upsert(upsertExisting, false);
System.assert(upsertUpdated.isSuccess());
System.assert(!upsertUpdated.isCreated());
System.assertEquals(upsertCreated.getId(), upsertUpdated.getId());
System.assertEquals(0, upsertUpdated.getErrors().size());

Account master = new Account(Name = 'Master');
insert master;
Account duplicate = new Account(Name = 'Duplicate');
insert duplicate;
Contact child = new Contact(LastName = 'Child', AccountId = duplicate.Id);
insert child;
Database.MergeResult merged = Database.merge(master, duplicate, false);
System.assert(merged.isSuccess());
System.assertEquals(master.Id, merged.getId());
System.assertEquals(duplicate.Id, merged.getMergedRecordIds().get(0));
System.assertEquals(child.Id, merged.getUpdatedRelatedIds().get(0));
System.assertEquals(0, merged.getErrors().size());

Database.UndeleteResult activeUndelete = Database.undelete(base, false);
System.assert(!activeUndelete.isSuccess());
System.assertEquals(inserted.getId(), activeUndelete.getId());
System.assertEquals('ENTITY_IS_NOT_DELETED', activeUndelete.getErrors().get(0).getStatusCode());

Account recycle = new Account(Name = 'Recycle');
insert recycle;
Database.DeleteResult deleted = Database.delete(recycle, false);
System.assert(deleted.isSuccess());
System.assertEquals(recycle.Id, deleted.getId());
System.assertEquals(0, deleted.getErrors().size());
Database.UndeleteResult restored = Database.undelete(recycle, false);
System.assert(restored.isSuccess());
System.assertEquals(recycle.Id, restored.getId());
Database.delete(recycle, false);
Database.EmptyRecycleBinResult emptied = Database.emptyRecycleBin(recycle, false);
System.assert(emptied.isSuccess());
System.assertEquals(recycle.Id, emptied.getId());
System.assertEquals(0, [SELECT Id FROM Account WHERE Id = :recycle.Id ALL ROWS].size());

Account lockRow = new Account(Name = 'Lock');
insert lockRow;
Database.LockResult locked = Database.lock(lockRow, false);
System.assert(locked.isSuccess());
System.assertEquals(lockRow.Id, locked.getId());
System.assertEquals(0, locked.getErrors().size());
Database.UnlockResult unlocked = Database.unlock(lockRow, false);
System.assert(unlocked.isSuccess());
System.assertEquals(lockRow.Id, unlocked.getId());
System.assertEquals(0, unlocked.getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.Fields["Score__c"] = storage.Field{APIName: "Score__c", Type: storage.FieldCalculated}
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
			}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseRecordActionAllOrNoneRollsBackResults(t *testing.T) {
	program, err := CompileAnonymous(`
Account lockRollback = new Account(Name = 'Lock Rollback');
insert lockRollback;
Account missing = new Account(Id = '001999999999999');
Boolean lockCaught = false;
try {
	Database.lock(new List<Account>{lockRollback, missing}, true);
} catch (DmlException e) {
	lockCaught = true;
	System.assert(e.getMessage().contains('Database.lock failed'));
}
System.assert(lockCaught);

Account unlockRollback = new Account(Name = 'Unlock Rollback');
insert unlockRollback;
Database.lock(unlockRollback, false);
Boolean unlockCaught = false;
try {
	Database.unlock(new List<Account>{unlockRollback, missing}, true);
} catch (DmlException e) {
	unlockCaught = true;
	System.assert(e.getMessage().contains('Database.unlock failed'));
}
System.assert(unlockCaught);

Account recycleRollback = new Account(Name = 'Recycle Rollback');
insert recycleRollback;
delete recycleRollback;
Boolean emptyCaught = false;
try {
	Database.emptyRecycleBin(new List<Account>{recycleRollback, missing}, true);
} catch (DmlException e) {
	emptyCaught = true;
	System.assert(e.getMessage().contains('Database.emptyRecycleBin failed'));
}
System.assert(emptyCaught);
List<Account> recycleRows = [SELECT Id, IsDeleted FROM Account WHERE Id = :recycleRollback.Id ALL ROWS];
System.assertEquals(1, recycleRows.size());
Account recycleRow = recycleRows.get(0);
System.assert(recycleRow.IsDeleted);
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
	lockID := storage.ID(machine.Globals["lockRollback"].Fields["Id"].Text)
	if org.Objects["Account"].Records[lockID].System.Locked {
		t.Fatalf("Database.lock allOrNone rollback left %s locked", lockID)
	}
	unlockID := storage.ID(machine.Globals["unlockRollback"].Fields["Id"].Text)
	if !org.Objects["Account"].Records[unlockID].System.Locked {
		t.Fatalf("Database.unlock allOrNone rollback left %s unlocked", unlockID)
	}
}

func TestExecDatabaseDeleteAndUndeleteResultTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'Acme');
insert a;
Database.UndeleteResult active = Database.undelete(a, false);
System.assert(!active.isSuccess());
System.assertEquals(a.Id, active.getId());
System.assertEquals('ENTITY_IS_NOT_DELETED', active.getErrors().get(0).getStatusCode());
Database.DeleteResult deleted = Database.delete(a, false);
System.assert(deleted.isSuccess());
System.assertEquals(a.Id, deleted.getId());
System.assertEquals(0, deleted.getErrors().size());
Database.UndeleteResult restored = Database.undelete(a, false);
System.assert(restored.isSuccess());
System.assertEquals(a.Id, restored.getId());
System.assertEquals(0, restored.getErrors().size());
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
	if got := machine.Globals["deleted"].Type; got != "Database.DeleteResult" {
		t.Fatalf("deleted result type = %q, want Database.DeleteResult", got)
	}
	if got := machine.Globals["active"].Type; got != "Database.UndeleteResult" {
		t.Fatalf("active result type = %q, want Database.UndeleteResult", got)
	}
	if got := machine.Globals["restored"].Type; got != "Database.UndeleteResult" {
		t.Fatalf("undelete result type = %q, want Database.UndeleteResult", got)
	}
}

func TestExecDatabaseUndeleteMixedRowsAndRollback(t *testing.T) {
	program, err := CompileAnonymous(`
Account deleted = new Account(Name = 'Deleted');
Account active = new Account(Name = 'Active');
insert new List<Account>{deleted, active};
delete deleted;

Account missing = new Account(Id = '001999999999999');
Account wrongType = new Account(Id = '003000000000001');
List<Account> mixed = new List<Account>{deleted, active, missing, wrongType};
List<Object> results = Database.undelete(mixed, false);
System.assertEquals(4, results.size());

Object restored = results.get(0);
Object activeResult = results.get(1);
Object missingResult = results.get(2);
Object wrongTypeResult = results.get(3);

System.assert(restored.isSuccess());
System.assertEquals(deleted.Id, restored.getId());
System.assert(!activeResult.isSuccess());
System.assertEquals(active.Id, activeResult.getId());
System.assertEquals('ENTITY_IS_NOT_DELETED', activeResult.getErrors().get(0).getStatusCode());
System.assert(!missingResult.isSuccess());
System.assertEquals('001999999999999', missingResult.getId());
System.assertEquals('ENTITY_IS_DELETED', missingResult.getErrors().get(0).getStatusCode());
System.assert(!wrongTypeResult.isSuccess());
System.assertEquals('003000000000001', wrongTypeResult.getId());
System.assertEquals('INVALID_FIELD', wrongTypeResult.getErrors().get(0).getStatusCode());

List<Account> visible = [SELECT Id FROM Account WHERE Id = :deleted.Id];
System.assertEquals(1, visible.size());

delete deleted;
Boolean caught = false;
try {
	Database.undelete(new List<Account>{deleted, active}, true);
} catch (DmlException e) {
	caught = true;
	System.assert(e.getMessage().contains('Database.undelete failed'));
	System.assert(e.getMessage().contains('not deleted'));
}
System.assert(caught);
List<Account> rolledBack = [SELECT Id, IsDeleted FROM Account WHERE Id = :deleted.Id ALL ROWS];
System.assertEquals(1, rolledBack.size());
Account rolledBackRow = rolledBack.get(0);
System.assert(rolledBackRow.IsDeleted);
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

func TestExecDatabaseAllOrNoneTrueThrowsAndRollsBack(t *testing.T) {
	program, err := CompileAnonymous(`
Account good = new Account(Name = 'Acme');
Account bad = new Account(Bogus__c = 'nope');
List<Account> records = new List<Account>{good, bad};
Boolean caught = false;
try {
	Database.insert(records, true);
} catch (DmlException e) {
	caught = true;
	System.assert(e.getMessage().contains('Database.insert failed'));
	System.assert(e.getMessage().contains('unknown field'));
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
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatabaseAccessLevelOverloadUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`Database.insert(new Account(Name = 'Acme'), true, AccessLevel.USER_MODE);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Database.insert AccessLevel overload"` {
		t.Fatalf("err = %#v, want UnsupportedFeature AccessLevel overload", err)
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
System.assertEquals(false, req.getCompressed());
req.setCompressed(true);
System.assertEquals('https://example.test', req.getEndpoint());
System.assertEquals('GET', req.getMethod());
System.assertEquals('yes', req.getHeader('x-test'));
System.assertEquals(true, req.getCompressed());
System.assertEquals(1, req.getHeaderKeys().size());
System.assert(req.getHeaderKeys().contains('x-test'));
System.assertEquals(5000, req.getTimeout());
Http h = new Http();
HttpResponse res = h.send(req);
System.assertEquals(201, res.getStatusCode());
System.assertEquals('ok', res.getBody());
res.setStatus('Created');
res.setHeader('Content-Type', 'text/plain');
System.assertEquals('Created', res.getStatus());
System.assertEquals('text/plain', res.getHeader('content-type'));
System.assertEquals(1, res.getHeaderKeys().size());
System.assert(res.getHeaderKeys().contains('content-type'));
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

func TestExecHttpRequestValidationAndHeaderEdges(t *testing.T) {
	program, err := CompileAnonymous(`
HttpRequest req = new HttpRequest();
System.assertEquals('', req.getEndpoint());
System.assertEquals('', req.getMethod());
System.assertEquals('', req.getBody());
System.assertEquals(0, req.getHeaderKeys().size());
System.assertEquals(false, req.getCompressed());
System.assertEquals(10000, req.getTimeout());
req.setEndpoint('callout:NamedCredential/path');
req.setMethod('post');
System.assertEquals('POST', req.getMethod());
req.setHeader('X-Test', 'first');
req.setHeader('x-test', 'second');
System.assertEquals('second', req.getHeader('X-TEST'));
System.assertEquals(null, req.getHeader('Missing'));
req.setBodyAsBlob(Blob.valueOf('blob-body'));
System.assertEquals('blob-body', req.getBodyAsBlob().toString());
req.setTimeout(120000);
System.assertEquals(120000, req.getTimeout());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHttpResponseConstructorDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
HttpResponse res = new HttpResponse();
System.assertEquals(200, res.getStatusCode());
System.assertEquals('OK', res.getStatus());
System.assertEquals('', res.getBody());
System.assertEquals(0, res.getHeaderKeys().size());
res.setBodyAsBlob(Blob.valueOf('response-body'));
System.assertEquals('response-body', res.getBodyAsBlob().toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHttpRequestRejectsInvalidEdges(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "endpoint-relative",
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint('/relative');`,
			want: "HttpRequest endpoint must be an absolute http, https, or callout URL",
		},
		{
			name: "method",
			src:  `HttpRequest req = new HttpRequest(); req.setMethod('CONNECT');`,
			want: `HttpRequest method "CONNECT" is not supported`,
		},
		{
			name: "timeout-low",
			src:  `HttpRequest req = new HttpRequest(); req.setTimeout(0);`,
			want: "HttpRequest timeout must be between 1 and 120000 milliseconds",
		},
		{
			name: "timeout-high",
			src:  `HttpRequest req = new HttpRequest(); req.setTimeout(120001);`,
			want: "HttpRequest timeout must be between 1 and 120000 milliseconds",
		},
		{
			name: "send-missing-method",
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint('https://example.test'); Http h = new Http(); h.send(req);`,
			want: "HttpRequest method is required before Http.send",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecHttpSendWithoutMockIsUnsupportedTransport(t *testing.T) {
	program, err := CompileAnonymous(`
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test');
req.setMethod('GET');
Http h = new Http();
h.send(req);
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(nil).Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Http.send real network transport"` {
		t.Fatalf("err = %#v, want UnsupportedFeature real transport", err)
	}
	if result.Limits.Callouts != 1 {
		t.Fatalf("callouts = %d, want 1", result.Limits.Callouts)
	}
}

func TestExecUnsupportedHttpCalloutSurfacesHaveStableShape(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "static-resource-mock-constructor",
			src:  `StaticResourceCalloutMock mock = new StaticResourceCalloutMock();`,
			want: `unsupported call "StaticResourceCalloutMock local static resource callout mock surface"`,
		},
		{
			name: "multi-static-resource-mock-constructor",
			src:  `MultiStaticResourceCalloutMock mock = new MultiStaticResourceCalloutMock();`,
			want: `unsupported call "MultiStaticResourceCalloutMock local static resource callout mock surface"`,
		},
		{
			name: "continuation-constructor",
			src:  `Continuation cont = new Continuation(60);`,
			want: `unsupported call "Continuation constructor local continuation callout surface"`,
		},
		{
			name: "client-certificate-name",
			src:  `HttpRequest req = new HttpRequest(); req.setClientCertificateName('LocalCert');`,
			want: `unsupported call "HttpRequest.setClientCertificateName local client certificate callout surface"`,
		},
		{
			name: "client-certificate-material",
			src:  `HttpRequest req = new HttpRequest(); req.setClientCertificate('cert', 'password');`,
			want: `unsupported call "HttpRequest.setClientCertificate local client certificate callout surface"`,
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
				t.Fatalf("err = %#v, want %s", err, tc.want)
			}
		})
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
