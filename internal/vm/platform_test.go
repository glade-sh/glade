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

func TestExecAssertIsInstanceOfType(t *testing.T) {
	program, err := CompileAnonymous(`
Object account = new Account(Name = 'Acme');
Assert.isInstanceOfType(account, Account.class);
Assert.isInstanceOfType(account, SObject.class, 'account is sobject');
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
	failProgram, err := CompileAnonymous(`
Object account = new Account(Name = 'Acme');
Assert.isInstanceOfType(account, Contact.class, 'wrong type');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	machine.SetOrg(&org)
	_, err = machine.Execute(failProgram)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" || !strings.Contains(runtimeErr.Message, "wrong type") {
		t.Fatalf("expected AssertException wrong type, got %v", err)
	}
}

func TestExecBuiltinConstructorsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
HttpRequest req = new httprequest();
req.setEndpoint('https://example.test');
req.setMethod('GET');
System.assertEquals('GET', req.getMethod());

PageReference page = new pagereference('/apex/Home');
System.assertEquals('/apex/Home', page.getUrl());
ApexPages.PageReference apexPage = new ApexPages.PageReference('/apex/Alias');
System.assertEquals('/apex/Alias', apexPage.getUrl());

Messaging.SingleEmailMessage email = new messaging.singleemailmessage();
email.setSubject('Hello');
System.assertEquals('Hello', email.getSubject());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
			src:  `Search.unavailable('FIND {Acme} IN ALL FIELDS RETURNING Account(Id)');`,
			want: `unsupported call "Search.unavailable local search/SOSL surface"`,
		},
		{
			name: "approval process api",
			src:  `Approval.ProcessWorkitemRequest.setAction('Approve');`,
			want: `unsupported call "Approval.ProcessWorkitemRequest.setAction local approval process and lock surface"`,
		},
		{
			name: "approval process api",
			src:  `Approval.process(null);`,
			want: `unsupported call "Approval.process local approval process and lock surface"`,
		},
		{
			name: "auth oauth api",
			src:  `Auth.JWTUtil.validateJWTWithKeysEndpoint('token', 'https://example.invalid/keys');`,
			want: `unsupported call "Auth.JWTUtil.validateJWTWithKeysEndpoint local authentication token/cloud API surface"`,
		},
		{
			name: "event bus publish after commit",
			src:  `EventBus.publishAfterCommit(new List<Account>{new Account(Name = 'Acme')});`,
			want: `unsupported call "EventBus.publishAfterCommit local platform event after-commit delivery surface"`,
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
			name: "crypto key wrapper api",
			src:  `Crypto.getKeyStore('LocalKeys');`,
			want: `unsupported call "Crypto.getKeyStore local key, certificate, encryption, and random surfaces"`,
		},
		{
			name: "crypto key wrapper api second path",
			src:  `Crypto.generateSelfSignedCertificate('LocalKeys');`,
			want: `unsupported call "Crypto.generateSelfSignedCertificate local key, certificate, encryption, and random surfaces"`,
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

func TestExecSearchQueryUsesFixedSearchResults(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Nook Inc');
insert account;
Contact contact = new Contact(LastName = 'Nook');
insert contact;
Test.setFixedSearchResults(new List<Id>{account.Id, contact.Id});
List<List<SObject>> rows = Search.query('FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name) LIMIT 10');
System.assertEquals(2, rows.size());
System.assertEquals(1, rows[0].size());
System.assertEquals(account.Id, rows[0][0].Id);
System.assertEquals('Nook Inc', rows[0][0].get('Name'));
System.assertEquals(1, rows[1].size());
System.assertEquals(contact.Id, rows[1][0].Id);
List<Account> inlineRows = (List<Account>)([FIND 'Nook*' IN NAME FIELDS RETURNING Account(Id, Name)][0]);
System.assertEquals(1, inlineRows.size());
System.assertEquals(account.Id, inlineRows[0].Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}, "Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSearchQueryReturnsDeterministicEmptyRows(t *testing.T) {
	program, err := CompileAnonymous(`
List<List<SObject>> rows = Search.query('FIND {Missing*} IN ALL FIELDS RETURNING Account(Id), Contact(Id)', null);
System.assertEquals(2, rows.size());
System.assertEquals(0, rows[0].size());
System.assertEquals(0, rows[1].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMetadataDeployContainerLocalModel(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomMetadata item = new Metadata.CustomMetadata();
item.fullName = 'Feature.Default';
item.label = 'Default';
Metadata.CustomMetadataValue value = new Metadata.CustomMetadataValue();
value.field = 'Enabled__c';
value.value = false;
item.values.add(value);
container.addMetadata(item);
Metadata.CustomMetadata created = new Metadata.CustomMetadata();
created.fullName = 'Feature.Created';
created.label = 'Created';
Metadata.CustomMetadataValue createdValue = new Metadata.CustomMetadataValue();
createdValue.field = 'Enabled__c';
createdValue.value = true;
created.values.add(createdValue);
container.addMetadata(created);
Id deploymentId = Metadata.Operations.enqueueDeployment(container, null);
System.assertEquals('0Af000000000001', (String)deploymentId);
Metadata.DeployResult deployStatus = Metadata.Operations.checkDeployStatus(deploymentId, true);
System.assert(deployStatus.done);
System.assert(deployStatus.success);
System.assertEquals('SUCCEEDED', deployStatus.status.name());
System.assertEquals((String)deploymentId, (String)deployStatus.id);
System.assertEquals(2, deployStatus.numberComponentsTotal);
System.assertEquals(2, deployStatus.numberComponentsDeployed);
System.assertEquals(0, deployStatus.numberComponentErrors);
System.assertEquals(2, deployStatus.details.componentSuccesses.size());
System.assertEquals('Feature.Default', deployStatus.details.componentSuccesses[0].fullName);
System.assertEquals('CustomMetadata', deployStatus.details.componentSuccesses[0].componentType);
Metadata.DeployResult deployStatusWithoutDetails = Metadata.Operations.checkDeployStatus(deploymentId, false);
System.assertEquals(null, deployStatusWithoutDetails.details);
Metadata.AsyncResult asyncResult = new Metadata.AsyncResult();
asyncResult.id = deploymentId;
asyncResult.done = true;
asyncResult.state = 'Succeeded';
System.assertEquals((String)deploymentId, (String)asyncResult.id);
System.assertEquals(2, container.metadata.size());
System.assertEquals(1, item.values.size());
System.assertEquals('Enabled__c', ((Metadata.CustomMetadataValue)item.values[0]).field);
Feature__mdt cfg = Feature__mdt.getInstance('Default');
System.assertEquals('Default', cfg.MasterLabel);
System.assertEquals(false, cfg.Enabled__c);
Feature__mdt createdCfg = Feature__mdt.getInstance('Created');
System.assertEquals('Created', createdCfg.MasterLabel);
System.assertEquals(true, createdCfg.Enabled__c);
Metadata.DeployResult result = new Metadata.DeployResult();
result.status = Metadata.DeployStatus.SUCCEEDED;
System.assertEquals('SUCCEEDED', result.status.name());
System.assertEquals('SUCCEEDED', String.valueOf(result.status));
result.details = new Metadata.DeployDetails();
result.details.componentFailures.add(new Metadata.DeployMessage());
System.assertEquals(1, result.details.componentFailures.size());
List<Metadata.CustomMetadata> records = Metadata.Operations.retrieve(Metadata.MetadataType.CustomMetadata, new List<String>{'Feature.Default', 'Feature.Created'});
System.assertEquals(2, records.size());
Metadata.CustomMetadata retrieved = records[0];
System.assertEquals('Feature.Default', retrieved.fullName);
System.assertEquals(1, retrieved.values.size());
System.assertEquals('Enabled__c', retrieved.values[0].field);
System.assertEquals(false, retrieved.values[0].value);
System.assertEquals('Feature.Created', records[1].fullName);
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

func TestExecReportsReportManagerLocalHarness(t *testing.T) {
	program, err := CompileAnonymous(`
Id reportId = '00O000000000001';
reports.ReportResults results = reports.ReportManager.runReport(reportId, true);
System.assertEquals(false, results.getAllData());
System.assertEquals(true, results.getHasDetailRows());
System.assertEquals(0, results.getFactMap().size());
System.assertEquals(reportId, results.getReportMetadata().getId());
System.assertEquals('Local Report', results.getReportMetadata().getName());
reports.ReportMetadata metadata = new reports.ReportMetadata();
metadata.setName('Override');
reports.ReportInstance instance = reports.ReportManager.runAsyncReport(reportId, metadata, false);
System.assertEquals('Success', instance.getStatus());
System.assertEquals(reportId, instance.getReportId());
System.assertEquals(false, instance.getReportResults().getHasDetailRows());
System.assertEquals('Override', instance.getReportResults().getReportMetadata().getName());
System.assertEquals(instance.getId(), reports.ReportManager.getReportInstance(instance.getId()).getId());
System.assertEquals(1, reports.ReportManager.getReportInstances(reportId).size());
System.assertEquals(0, reports.ReportManager.getDatatypeFilterOperatorMap().size());
reports.ReportDescribeResult describe = reports.ReportManager.describeReport(reportId);
System.assertEquals(reportId, describe.getReportMetadata().getId());
System.assertEquals(0, describe.getReportExtendedMetadata().getDetailColumnInfo().size());
System.assertEquals(0, describe.getReportTypeMetadata().getStandardFilterInfos().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocalTelemetryProvisioningAndPrefCenterHarnesses(t *testing.T) {
	program, err := CompileAnonymous(`
IsvPartners.AppAnalytics.logCustomInteraction('clicked');
UserProvisioning.UserProvisioningLog.log('0PR-local', 'Created');
String token = pref_center.TokenUtility.generateToken('subscriber-1');
System.assert(token.startsWith('local-token-'));
Map<String,String> tokens = pref_center.TokenUtility.generateTokens(new List<String>{'a', 'b'});
System.assertEquals(2, tokens.size());
System.assertEquals(pref_center.TokenUtility.generateToken('a'), tokens.get('a'));
System.assert(tokens.get('a') != tokens.get('b'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecVisualEditorDataRowAndSearchSuggestionOption(t *testing.T) {
	program, err := CompileAnonymous(`
VisualEditor.DataRow selected = new VisualEditor.DataRow('A', 42, true);
VisualEditor.DataRow other = new VisualEditor.DataRow('B', 'value');
System.assertEquals('A', selected.getLabel());
System.assertEquals(42, (Integer)selected.getValue());
System.assertEquals(true, selected.isSelected());
System.assert(selected.compareTo(other) < 0);
selected.setValue('changed');
System.assertEquals('changed', selected.getValue());
Search.SuggestionOption option = new Search.SuggestionOption();
option.setFilter(new Search.KnowledgeSuggestionFilter());
option.setLimit(5);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocalMockHarnessSurfaces(t *testing.T) {
	program, err := CompileAnonymous(`
Test.startTest();
Test.getEventBus().deliver();
eventbus.TestEventService.publishEvent('Event__e', new Map<String,Object>{ 'Name' => 'local' });
HttpResponse callback = Test.getExternalService().sendCallback(new HttpRequest());
System.assertEquals(200, callback.getStatusCode());
Component.apex.page page = Test.invokePage(new PageReference('/apex/TestPage'));
System.assert(page != null);
HttpResponse asyncResponse = new TestAsyncHttp().executeHttpRequest(new HttpRequest());
System.assertEquals(200, asyncResponse.getStatusCode());
functions.FunctionInvocation ok = functions.MockFunctionInvocationFactory.createSuccessResponse('fn-1', 'done');
System.assertEquals('fn-1', ok.getInvocationId());
System.assertEquals('done', ok.getResponse());
System.assertEquals('SUCCESS', ok.getStatus().name());
functions.FunctionInvocation failed = functions.MockFunctionInvocationFactory.createErrorResponse('fn-2', functions.FunctionErrorType.RUNTIME_EXCEPTION, 'bad');
System.assertEquals('ERROR', failed.getStatus().name());
System.assertEquals('bad', failed.getError().getMessage());
functions.FunctionInvocation mockResult = new functions.FunctionInvokeMock().respond('fn-3', 'payload');
System.assertEquals('fn-3', mockResult.getInvocationId());
System.assertEquals('payload', mockResult.getResponse());
String created = SubMgmt.Test.create('Subscription', new Map<String,Object>{ 'Name' => 'local' });
System.assert(created.startsWith('local-subscription-'));
SubMgmt.Test.modify('001000000000001', new Map<String,Object>{ 'Name' => 'changed' });
SubMgmt.Test.remove('001000000000001');
ConnectedApplication app = UserProvisioning.ConnectorTestUtil.createConnectedApp('Local App');
System.assertEquals('Local App', app.Name);
new CartExtension.CartCalculateExecutorMock().calculate(new CartExtension.CartCalculateOrchestratorRequest(null, null, null));
new CartExtension.SplitShipmentServiceMock().arrangeItems(null);
ConnectApi.BaseEndpointExtension endpoint = new ConnectApi.BaseEndpointExtension();
ConnectApi.EndpointExtensionRequest endpointRequest = new ConnectApi.EndpointExtensionRequest();
ConnectApi.EndpointExtensionResponse endpointResponse = new ConnectApi.EndpointExtensionResponse('body', 1, 'etag');
System.assertEquals(endpointRequest, endpoint.beforeGet(endpointRequest));
System.assertEquals(endpointResponse, endpoint.afterGet(endpointResponse, endpointRequest));
sfsqlquery.QueryHandle handle = sfsqlquery.QueryHandle.create('query-1', 'default');
sfsqlquery.SqlQueueable queued = new sfsqlquery.SqlQueueable(handle);
System.assertEquals('query-1', queued.getQueryId());
System.assert(queued.getRows() != null);
queued.chainNextJob(handle);
queued.processDataChunk();
queued.cancel();
System.assertEquals(true, (Boolean)wave.Templates.getTemplates().get('local'));
System.assertEquals(true, (Boolean)wave.Templates.getTemplate('Template').get('local'));
Flow.Interview interview = Flow.Interview.createInterview('LocalFlow', new Map<String,Object>{ 'answer' => 42 });
interview.start();
System.assertEquals(42, (Integer)interview.getVariableValue('answer'));
Continuation continuation = new Continuation(60);
String requestLabel = continuation.addHttpRequest(new HttpRequest());
System.assertEquals(1, continuation.getRequests().size());
System.assert(continuation.getRequests().containsKey(requestLabel));
Iterator<String> iter = new List<String>{'a'}.iterator();
System.assert(iter.hasNext());
System.assertEquals('a', (String)iter.next());
FormulaRecalcFieldError recalcError = new FormulaRecalcFieldError();
System.assertEquals('', recalcError.getFieldName());
System.assertEquals('', recalcError.getFieldError());
CartExtension.PricingCartCalculator pricing = new CartExtension.PricingCartCalculator(new CartExtension.CartCalculateExecutorMock());
pricing.calculate(new CartExtension.CartCalculateCalculatorRequest(null, null));
CartExtension.SplitShipmentService split = new CartExtension.SplitShipmentService(new CartExtension.SplitShipmentServiceMock());
split.arrangeItems(null);
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.testContext = &TestContext{}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecQuickActionDescribeAndTemplateDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
List<QuickAction.DescribeAvailableQuickActionResult> available =
	QuickAction.describeAvailableQuickActions('Account');
System.assertEquals(1, available.size());
System.assertEquals('Account.NewTask', available[0].getName());
System.assertEquals('New Task', available[0].getLabel());
System.assertEquals('Create', available[0].getType());

List<QuickAction.DescribeQuickActionResult> described =
	QuickAction.describeQuickActions(new List<String>{'Account.NewTask', 'Account.Unknown'});
System.assertEquals(2, described.size());
System.assertEquals('Account', described[0].getTargetSobjectType());
System.assertEquals(0, described[0].getDefaultValues().size());
System.assertEquals('Account.Unknown', described[1].getName());

QuickAction.QuickActionTemplateResult template =
	QuickAction.retrieveQuickActionTemplate('Account.NewTask', '001000000000001');
System.assertEquals(true, template.isSuccess());
System.assertEquals('001000000000001', template.getContextId());
System.assertEquals('Account.NewTask', template.getDefaultValues().getQuickActionName());

List<QuickAction.QuickActionTemplateResult> templates =
	QuickAction.retrieveQuickActionTemplates(new List<String>{'Account.NewTask'}, '001000000000001');
System.assertEquals(1, templates.size());
System.assertEquals(true, templates[0].isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Metadata.QuickActions = []storage.QuickActionMetadata{{
		Name:         "Account.NewTask",
		Label:        "New Task",
		Type:         "Create",
		TargetObject: "Account",
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestNewSendEmailQuickActionDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
QuickAction.SendEmailQuickActionDefaults defaults =
	Test.newSendEmailQuickActionDefaults('001000000000001', '00T000000000001');
System.assertEquals('SendEmail', defaults.getActionName());
System.assertEquals('SendEmail', defaults.getActionType());
System.assertEquals('001000000000001', defaults.getContextId());
System.assertEquals('00T000000000001', defaults.getInReplyToId());
System.assertEquals(0, defaults.getFromAddressList().size());
defaults.setTemplateId('00X000000000001');
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

func TestExecMetadataDeployContainerCustomObjectAndField(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomObject objectDef = new Metadata.CustomObject();
objectDef.fullName = 'Invoice__c';
objectDef.label = 'Invoice';
objectDef.pluralLabel = 'Invoices';
container.addMetadata(objectDef);

Metadata.CustomField amount = new Metadata.CustomField();
amount.fullName = 'Invoice__c.Amount__c';
amount.label = 'Amount';
amount.type = 'Number';
amount.required = true;
container.addMetadata(amount);

Metadata.CustomField paid = new Metadata.CustomField();
paid.fullName = 'Invoice__c.Paid__c';
paid.label = 'Paid';
paid.type = 'Checkbox';
container.addMetadata(paid);

Id deploymentId = Metadata.Operations.enqueueDeployment(container, null);
Metadata.DeployResult result = Metadata.Operations.checkDeployStatus(deploymentId, true);
System.assert(result.success);
System.assertEquals(3, result.numberComponentsTotal);
System.assertEquals(3, result.numberComponentsDeployed);
System.assertEquals(0, result.numberComponentErrors);
System.assertEquals('CustomObject', result.details.componentSuccesses[0].componentType);
System.assertEquals('Invoice__c', result.details.componentSuccesses[0].fullName);
System.assertEquals('CustomField', result.details.componentSuccesses[1].componentType);
System.assertEquals('Invoice__c.Amount__c', result.details.componentSuccesses[1].fullName);

Map<String,Object> describes = Schema.getGlobalDescribe();
System.assert(describes.containsKey('Invoice__c'));
Object invoiceType = describes.get('Invoice__c');
Object invoiceDescribe = invoiceType.getDescribe();
System.assertEquals('Invoice__c', invoiceDescribe.getName());
Map<String,Object> fields = invoiceDescribe.fields.getMap();
System.assert(fields.containsKey('Amount__c'));
System.assert(fields.containsKey('Paid__c'));
System.assertEquals('Amount', fields.get('Amount__c').getDescribe().getLabel());

SObject invoice = invoiceType.newSObject();
invoice.put('Name', 'INV-1');
invoice.put('Amount__c', 42);
invoice.put('Paid__c', true);
insert invoice;
List<SObject> rows = Database.query('SELECT Name, Amount__c, Paid__c FROM Invoice__c WHERE Amount__c = 42');
System.assertEquals(1, rows.size());
System.assertEquals('INV-1', (String)rows[0].get('Name'));
System.assertEquals(42, (Integer)rows[0].get('Amount__c'));
System.assertEquals(true, (Boolean)rows[0].get('Paid__c'));
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

func TestExecMetadataDeploymentUnsupportedItemType(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
container.addMetadata(new Metadata.Metadata());
Metadata.Operations.enqueueDeployment(container, null);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := customDataOrg()
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	if err == nil {
		t.Fatal("expected unsupported metadata deploy item")
	}
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Metadata.Operations.enqueueDeployment Metadata.Metadata metadata deploy"` {
		t.Fatalf("err = %#v, want UnsupportedFeature Metadata.Metadata deploy", err)
	}
}

func TestExecMetadataDeploymentInvalidSupportedItemReturnsFailureResult(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomObject objectDef = new Metadata.CustomObject();
objectDef.fullName = 'Invoice__c';
container.addMetadata(objectDef);
Metadata.CustomField missingObject = new Metadata.CustomField();
missingObject.fullName = 'Missing__c.Amount__c';
missingObject.label = 'Amount';
missingObject.type = 'Number';
container.addMetadata(missingObject);
Id deploymentId = Metadata.Operations.enqueueDeployment(container, null);
Metadata.DeployResult result = Metadata.Operations.checkDeployStatus(deploymentId, true);
System.assert(result.done);
System.assert(!result.success);
System.assertEquals('FAILED', result.status.name());
System.assertEquals(2, result.numberComponentsTotal);
System.assertEquals(0, result.numberComponentsDeployed);
System.assertEquals(1, result.numberComponentErrors);
System.assertEquals(0, result.details.componentSuccesses.size());
System.assertEquals(1, result.details.componentFailures.size());
System.assertEquals('CustomField', result.details.componentFailures[0].componentType);
System.assertEquals('Missing__c.Amount__c', result.details.componentFailures[0].fullName);
System.assert(result.details.componentFailures[0].problem.contains('unknown object Missing__c'));
System.assert(!Schema.getGlobalDescribe().containsKey('Invoice__c'));
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

func TestExecEventBusPublishReturnsLocalSuccessResults(t *testing.T) {
	program, err := CompileAnonymous(`
Database.SaveResult single = EventBus.publish(new Account(Name = 'Acme'));
System.assert(single.isSuccess());
System.assertEquals(null, single.getId());
System.assertEquals(0, single.getErrors().size());
List<Database.SaveResult> many = EventBus.publish(new List<Account>{new Account(Name = 'One'), new Account(Name = 'Two')});
System.assertEquals(2, many.size());
System.assert(many.get(0).isSuccess());
System.assert(many.get(1).isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEventBusPublishReturnsFailureForMissingRequiredPlatformEventField(t *testing.T) {
	program, err := CompileAnonymous(`
Database.SaveResult result = EventBus.publish(new Local_Event__e(Name__c = 'Trail'));
System.assert(!result.isSuccess());
System.assertEquals(1, result.getErrors().size());
System.assertEquals('REQUIRED_FIELD_MISSING', String.valueOf(result.getErrors()[0].getStatusCode()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Local_Event__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Local_Event__e",
			KeyPrefix: "e00",
			Fields: map[string]storage.Field{
				"Name__c":    {APIName: "Name__c", Type: storage.FieldString},
				"Account__c": {APIName: "Account__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEventBusPublishRequiredTextFieldAcceptsIdScalar(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Event Account');
insert account;
Database.SaveResult result = EventBus.publish(new Local_Event__e(AccountId__c = account.Id, Name__c = 'Trail'));
System.assert(result.isSuccess());
System.assertEquals(0, result.getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Local_Event__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Local_Event__e",
			KeyPrefix: "e00",
			Fields: map[string]storage.Field{
				"AccountId__c": {APIName: "AccountId__c", Type: storage.FieldString, Required: true},
				"Name__c":      {APIName: "Name__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEventBusPublishAfterInsertTriggerUpdatesRelatedRecordByTextId(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
Set<Id> accountIds = new Set<Id>();
for (Local_Event__e evt : Trigger.new) {
	accountIds.add(evt.AccountId__c);
}
Map<Id, Account> accounts = new Map<Id, Account>([SELECT Name FROM Account WHERE Id IN :accountIds]);
for (Local_Event__e evt : Trigger.new) {
	accounts.get(evt.AccountId__c).Website = evt.Url__c;
}
update accounts.values();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Event Account');
insert account;
Database.SaveResult result = EventBus.publish(new Local_Event__e(AccountId__c = account.Id, Name__c = 'Trail', Url__c = 'https://example.test'));
System.assert(result.isSuccess());
Account updated = [SELECT Website FROM Account WHERE Id = :account.Id LIMIT 1];
System.assertEquals('https://example.test', updated.Website);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Local_Event__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Local_Event__e",
			KeyPrefix: "e00",
			Fields: map[string]storage.Field{
				"AccountId__c": {APIName: "AccountId__c", Type: storage.FieldString, Required: true},
				"Name__c":      {APIName: "Name__c", Type: storage.FieldString, Required: true},
				"Url__c":       {APIName: "Url__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{
		Name:      "LocalEventTrigger",
		Object:    "Local_Event__e",
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

func TestExecPlatformEventSObjectTypeNewSObjectSeedsEventUuid(t *testing.T) {
	program, err := CompileAnonymous(`
Event_Recipes_Demo__e directEvent = new Event_Recipes_Demo__e();
System.assertEquals(null, directEvent.EventUuid);
Event_Recipes_Demo__e tokenEvent = (Event_Recipes_Demo__e) Event_Recipes_Demo__e.SObjectType.newSObject(null, true);
System.assertNotEquals(null, tokenEvent.EventUuid);
System.assertEquals(36, tokenEvent.EventUuid.length());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Event_Recipes_Demo__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Event_Recipes_Demo__e",
			Fields: map[string]storage.Field{
				"EventUuid":    {APIName: "EventUuid", Type: storage.FieldString},
				"AccountId__c": {APIName: "AccountId__c", Type: storage.FieldReference},
				"Title__c":     {APIName: "Title__c", Type: storage.FieldString},
				"Url__c":       {APIName: "Url__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEventBusPublishCallbackDeliveredAtStopTest(t *testing.T) {
	callbackProgram, err := CompileAnonymous(`
List<String> eventUuids = result.getEventUuids();
insert new Task(Subject = 'success ' + eventUuids[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Event_Recipes_Demo__e event = (Event_Recipes_Demo__e) Event_Recipes_Demo__e.SObjectType.newSObject(null, true);
String uuid = event.EventUuid;
String expected = 'success ' + uuid;
Test.startTest();
Database.SaveResult result = EventBus.publish(event, new PublishCallback());
System.assert(result.isSuccess());
Test.stopTest();
List<Task> tasks = [SELECT Subject FROM Task WHERE Subject = :expected];
System.assertEquals(1, tasks.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	org.Objects["Event_Recipes_Demo__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Event_Recipes_Demo__e", Fields: map[string]storage.Field{"EventUuid": {APIName: "EventUuid", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	storage.EnsureStandardObject(&org, "Task")
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "PublishCallback",
		Methods: map[string]Method{
			"onSuccess": {
				Name:       "PublishCallback.onSuccess",
				ClassName:  "PublishCallback",
				ReturnType: "void",
				Params:     []Param{{Name: "result", Type: "eventbus.SuccessResult"}},
				Program:    callbackProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEventBusPublishCallbackFailureCanBeForced(t *testing.T) {
	callbackProgram, err := CompileAnonymous(`
List<String> eventUuids = result.getEventUuids();
insert new Task(Subject = 'failure ' + eventUuids[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Event_Recipes_Demo__e event = (Event_Recipes_Demo__e) Event_Recipes_Demo__e.SObjectType.newSObject(null, true);
String uuid = event.EventUuid;
String expected = 'failure ' + uuid;
Test.startTest();
EventBus.publish(event, new PublishCallback());
Test.getEventBus().fail();
Test.stopTest();
List<Task> tasks = [SELECT Subject FROM Task WHERE Subject = :expected];
System.assertEquals(1, tasks.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	org.Objects["Event_Recipes_Demo__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Event_Recipes_Demo__e", Fields: map[string]storage.Field{"EventUuid": {APIName: "EventUuid", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	storage.EnsureStandardObject(&org, "Task")
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "PublishCallback",
		Methods: map[string]Method{
			"onFailure": {
				Name:       "PublishCallback.onFailure",
				ClassName:  "PublishCallback",
				ReturnType: "void",
				Params:     []Param{{Name: "result", Type: "eventbus.FailureResult"}},
				Program:    callbackProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConnectApiOrganizationSettingsStub(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.OrganizationSettings settings = ConnectApi.Organization.getSettings();
System.assertEquals('00DLOCAL00000001', settings.orgId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.OrgID = "00DLOCAL00000001"
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConnectApiChatterUsersFollowingsHonorsSeeAllData(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	ConnectApi.ChatterUsers.getFollowings(null, UserInfo.getUserId());
} catch (UnsupportedOperationException e) {
	caught = true;
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "ConnectApi.ChatterUsers",
		Methods: map[string]Method{
			"getFollowings": {
				Name:       "ConnectApi.ChatterUsers.getFollowings",
				ClassName:  "ConnectApi.ChatterUsers",
				ReturnType: "ConnectApi.FollowingPage",
				Params:     []Param{{Name: "communityId", Type: "String"}, {Name: "userId", Type: "String"}},
				IsStatic:   true,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	seeAllDataProgram, err := CompileAnonymous(`
ConnectApi.FollowingPage page = ConnectApi.ChatterUsers.getFollowings(null, UserInfo.getUserId());
System.assertNotEquals(null, page);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	if err := machine.RegisterClass(Class{
		Name: "ConnectApi.ChatterUsers",
		Methods: map[string]Method{
			"getFollowings": {
				Name:       "ConnectApi.ChatterUsers.getFollowings",
				ClassName:  "ConnectApi.ChatterUsers",
				ReturnType: "ConnectApi.FollowingPage",
				Params:     []Param{{Name: "communityId", Type: "String"}, {Name: "userId", Type: "String"}},
				IsStatic:   true,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	machine.SetTestSeeAllData(true)
	if _, err := machine.Execute(seeAllDataProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformCachePartitions(t *testing.T) {
	program, err := CompileAnonymous(`
Cache.OrgPartition orgCache = Cache.Org.getPartition('local');
System.assertEquals(null, orgCache.get('missing'));
orgCache.put('name', 'Acme');
orgCache.put('visible', 'Trail', 60, Cache.Visibility.ALL, false);
System.assert(orgCache.contains('name'));
System.assertEquals('Acme', (String) orgCache.get('name'));
System.assertEquals('Trail', (String) orgCache.get('visible'));
Set<String> orgKeys = orgCache.getKeys();
System.assertEquals(2, orgKeys.size());
System.assert(orgKeys.contains('name'));
System.assert(orgKeys.contains('visible'));
System.assertEquals(2, orgCache.getNumKeys());
System.assertEquals('Acme', (String) orgCache.remove('name'));
System.assert(!orgCache.contains('name'));
System.assertEquals(1, orgCache.getNumKeys());

Cache.SessionPartition sessionCache = Cache.Session.getPartition('local');
Cache.Partition generalSession = sessionCache;
Cache.Partition generalOrg = orgCache;
sessionCache.put('count', 7, 60);
System.assertEquals(7, (Integer) sessionCache.get('count'));
System.assert(sessionCache.getKeys().contains('count'));
System.assertEquals(1, sessionCache.getNumKeys());
generalSession.put('general', 'session');
generalOrg.put('general', 'org');
System.assertEquals('session', (String) generalSession.get('general'));
System.assertEquals('org', (String) generalOrg.get('general'));
System.assertEquals(null, sessionCache.get(String.class, 'missing'));
sessionCache.remove(String.class, 'missing');

System.assert(Cache.Org.isAvailable());
System.assert(Cache.Org.getCapacity() > 0);
System.assertEquals('local.default', Cache.Org.getName());
System.assertEquals(0, Cache.Org.getAvgGetSize());
System.assertEquals(0, Cache.Org.getAvgGetTime());
System.assertEquals(0, Cache.Org.getAvgValueSize());
System.assertEquals(0, Cache.Org.getMaxGetSize());
System.assertEquals(0, Cache.Org.getMaxGetTime());
System.assertEquals(0, Cache.Org.getMaxValueSize());
System.assertEquals(0, Cache.Org.getMissRate());
System.assertEquals('local.default.account', orgCache.createFullyQualifiedKey('local', 'default', 'account'));
System.assertEquals('local.default', orgCache.createFullyQualifiedPartition('local', 'default'));
orgCache.validatePartitionName('default');
orgCache.validateKey(false, 'account');
orgCache.validateKeyValue(false, 'account', 'value');
Cache.Org.put('defaulted', 'org-default');
System.assert(Cache.Org.contains('defaulted'));
System.assertEquals('org-default', (String) Cache.Org.get('defaulted'));
System.assert(Cache.Org.getKeys().contains('defaulted'));
System.assertEquals(1, Cache.Org.getNumKeys());
System.assertEquals('org-default', (String) Cache.Org.getPartition('default').get('defaulted'));
System.assertEquals('org-default', (String) Cache.Org.remove('defaulted'));
System.assert(!Cache.Org.contains('defaulted'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformCacheBuilderLoadsAndMemoizesDefaultPartition(t *testing.T) {
	loadProgram, err := CompileAnonymous(`return 'loaded:' + requiredButNotUsed;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Cache.OrgPartition named = Cache.Org.getPartition('local.default');
System.assertEquals(0, named.getNumKeys());
System.assertEquals('loaded:shape', (String) named.get(CacheLoader.class, 'shape'));
System.assertEquals(1, named.getNumKeys());
System.assertEquals('loaded:shape', (String) Cache.Org.get(CacheLoader.class, 'shape'));
System.assertEquals(1, Cache.Org.getNumKeys());
Set<String> keys = named.getKeys();
System.assertEquals(1, keys.size());
System.assert(keys.toString().contains('CacheLoader'));
System.assertEquals('loaded:shape', (String) Cache.Org.remove(CacheLoader.class, 'shape'));
System.assertEquals(0, named.getNumKeys());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "CacheLoader",
		Interfaces: []string{"Cache.CacheBuilder"},
		Methods: map[string]Method{
			"doLoad": {
				Name:       "CacheLoader.doLoad",
				ClassName:  "CacheLoader",
				ReturnType: "Object",
				Params:     []Param{{Name: "requiredButNotUsed", Type: "String"}},
				Program:    loadProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformCachePartitionMetadataSeedsDefaultPartition(t *testing.T) {
	program, err := CompileAnonymous(`
PlatformCachePartition partition = [
	SELECT developerName, NamespacePrefix
	FROM PlatformCachePartition
	WHERE NamespacePrefix = ''
	LIMIT 1
];
System.assertEquals('default', partition.DeveloperName);
Cache.OrgPartition orgCache = Cache.Org.getPartition('local.' + partition.DeveloperName);
System.assert(orgCache.isAvailable());
System.assertEquals('local.default', orgCache.getName());
System.assert(Cache.Org.getPartition('local.default') != null);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PlatformCache"})
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatedConversionRateRequiresMultiCurrencyOrgShape(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	Database.query('SELECT Id FROM DatedConversionRate LIMIT 1');
} catch (QueryException e) {
	caught = true;
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

	enabledProgram, err := CompileAnonymous(`
Database.query('SELECT Id FROM DatedConversionRate LIMIT 1');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	storage.EnsureStandardObject(&org, "DatedConversionRate")
	storage.ApplyOrgShape(&org, []string{"MultiCurrency"})
	machine.SetOrg(&org)
	if _, err := machine.Execute(enabledProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecCommunityAuthValueObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,String> attributes = new Map<String,String>{ 'display_name' => 'Ada' };
Auth.UserData data = new Auth.UserData('003000000000001', 'Ada', 'Lovelace', 'Ada Lovelace', 'ada@example.invalid', null, 'ada@example.invalid', 'en_US', 'local', null, attributes);
System.assertEquals('003000000000001', data.identifier);
System.assertEquals('ada@example.invalid', data.email);
System.assertEquals('local-self-registration', UserManagement.initSelfRegistration(Auth.VerificationMethod.EMAIL, new User(LastName='Lovelace', Email='ada@example.invalid')));
System.assertEquals('+1 4155550100', UserManagement.formatPhoneNumber('1', '4155550100'));
System.assertEquals('+441234', UserManagement.formatPhoneNumber('44', '+441234'));
Auth.VerificationResult result = UserManagement.verifySelfRegistration(Auth.VerificationMethod.EMAIL, 'local-self-registration', '12345', '/welcome');
System.assert(result.success);
System.assertEquals('/welcome', result.redirect.getUrl());
System.assertEquals(true, Auth.AuthToken.revokeAccess('provider', 'user', 'token'));
System.assert(Auth.SessionManagement.getCurrentSession().get('SessionId').contains('session'));
Auth.AuthConfiguration config = new Auth.AuthConfiguration('https://local.example', '/start');
System.assertEquals(0, config.getAuthProviders().size());
System.assertEquals('https://local.example', config.getAuthConfig().Url);
System.assertEquals('/start', config.getStartUrl());
System.assertEquals('https://local.example/services/auth/sso/local?startURL=/start', Auth.AuthConfiguration.getAuthProviderSsoUrl('https://local.example', '/start', 'local'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocalAsyncDrainRunsQueuedJobsOutsideTestContext(t *testing.T) {
	jobProgram, err := CompileAnonymous(`insert new Account(Name = 'async ran');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String jobId = System.enqueueJob(new MarkJob());
System.assertEquals('707000000000001', jobId);
Integer beforeDrain = [SELECT COUNT() FROM Account WHERE Name = 'async ran'];
System.assertEquals(0, beforeDrain);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "MarkJob",
		Methods: map[string]Method{
			"execute": {Name: "MarkJob.execute", ClassName: "MarkJob", ReturnType: "void", Program: jobProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 0 {
		t.Fatalf("async job ran before drain: %#v", org.Objects["Account"].Records)
	}
	if err := machine.DrainAsync(&result); err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Account"].Records) != 1 {
		t.Fatalf("async drain records = %#v", org.Objects["Account"].Records)
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
PageReference aliasReplacement = new PageReference('Page.AccountView?mode=alias');
Test.setCurrentPageReference(aliasReplacement);
System.assertEquals('/apex/AccountView?mode=alias', System.currentPageReference().getUrl());
System.assertEquals('/apex/AccountView?mode=alias', ApexPages.currentPage().getUrl());
System.assertEquals('Page.Missing', new PageReference('Page.Missing').getUrl());
ApexPages.Severity severity = ApexPages.Severity.ERROR;
System.assertEquals('ERROR', severity.name());
System.assertEquals('ERROR', severity.toString());
System.assertEquals(3, severity.ordinal());
System.assertEquals(5, ApexPages.Severity.values().size());
ApexPages.Message message = new ApexPages.Message(severity, 'Summary', 'Detail');
System.assertEquals(ApexPages.Severity.ERROR, message.getSeverity());
System.assertEquals(ApexPages.Severity.ERROR, ApexPages.severity.ERROR);
System.assertEquals('Summary', message.getSummary());
System.assertEquals('Detail', message.getDetail());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.RegisterPageReference("AccountView")
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

func TestExecBusinessHoursLocalTwentyFourSevenModel(t *testing.T) {
	program, err := CompileAnonymous(`
Id businessHoursId = '01m000000000001AAA';
Datetime start = Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0);
System.assert(BusinessHours.isWithin(businessHoursId, start));
System.assertEquals(start.addMilliseconds(1250), BusinessHours.add(businessHoursId, start, 1250));
System.assertEquals(start.addMilliseconds(1250), BusinessHours.addGmt(businessHoursId, start, 1250));
System.assertEquals(1250, BusinessHours.diff(businessHoursId, start, start.addMilliseconds(1250)));
System.assertEquals(start, BusinessHours.nextStartDate(businessHoursId, start));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCasesThreadingHelpers(t *testing.T) {
	program, err := CompileAnonymous(`
Id caseId = '500000000000001AAA';
String threadId = Cases.generateThreadingMessageId(caseId);
System.assertEquals(caseId, Cases.getCaseIdFromEmailThreadId(threadId));
Messaging.InboundEmail.Header header = new Messaging.InboundEmail.Header();
header.name = 'References';
header.value = 'previous ' + threadId;
System.assertEquals(caseId, Cases.getCaseIdFromEmailHeaders(new List<Messaging.InboundEmail.Header>{header}));
System.assertEquals(null, Cases.getCaseIdFromEmailThreadId('missing'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemDeterministicLocalHelpers(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, System.isFunctionCallback());
System.assertEquals(false, System.isRunningElasticCompute());
System.assertEquals('SY', System.getQuiddityShortCode(System.Request.getCurrent().getQuiddity()));
System.assertEquals('65.0.0', System.requestVersion().toString());
System.assertEquals('READ_WRITE', String.valueOf(System.getApplicationReadWriteMode()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAppExchangePassiveValueObjects(t *testing.T) {
	program, err := CompileAnonymous(`
AppExchangeTrialTemplate template = new AppExchangeTrialTemplate();
System.assertEquals(null, template.getCreatedDate());
System.assertEquals(null, template.getLastModifiedDate());
System.assertEquals(null, template.getId());
System.assertEquals(null, template.getName());
System.assertEquals(null, template.getStatus());
AppExchangeUserPerms perms = new AppExchangeUserPerms();
System.assertEquals(false, perms.getCanEditBilling());
System.assertEquals(false, perms.getCanInstall());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOrgLimitsLocalSnapshot(t *testing.T) {
	program, err := CompileAnonymous(`
List<OrgLimit> limits = OrgLimits.getAll();
System.assert(limits.size() > 0);
Map<String, OrgLimit> byName = OrgLimits.getMap();
System.assert(byName.containsKey('DailyApiRequests'));
OrgLimit api = byName.get('DailyApiRequests');
System.assertEquals('DailyApiRequests', api.getName());
System.assertEquals(100, api.getLimit());
System.assertEquals(0, api.getValue());
System.assertEquals('DailyApiRequests', api.toString());
OrgLimit cloned = (OrgLimit)api.clone();
System.assertEquals(api.getName(), cloned.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSlackPassiveDTOBuildersAndMocks(t *testing.T) {
	program, err := CompileAnonymous(`
Slack.ChatPostMessageRequest request = Slack.ChatPostMessageRequest.builder().
	channel('C123').
	text('hello').
	build();
System.assertEquals('C123', request.getChannel());
System.assertEquals('hello', request.getText());
Slack.Message message = new Slack.Message();
message.setText('local');
System.assertEquals('local', message.getText());
Slack.BotClientMock mock = new Slack.BotClientMock();
Slack.AuthTestResponse response = mock.authTest(Slack.AuthTestRequest.builder().build());
System.assertNotEquals(null, response);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedCommerceDTOCollectionMutators(t *testing.T) {
	program, err := CompileAnonymous(`
commercestorepricing.PricingRequest request = new commercestorepricing.PricingRequest();
commercestorepricing.PricingRequestItem item = new commercestorepricing.PricingRequestItem('01t000000000001AAA');
request.addPricingRequestItem(item);
System.assertEquals(1, request.getPricingRequestItems().size());
System.assertEquals(item, request.getPricingRequestItems().get(0));
System.assertEquals(0, request.getPricingRequestItems().indexOf(item));
System.assert(request.getPricingRequestItems().iterator().hasNext());
request.removePricingRequestItem(item);
System.assertEquals(0, request.getPricingRequestItems().size());
commercestorepricing.PsmIDCollection ids = new commercestorepricing.PsmIDCollection();
System.assert(ids.isEmpty());
System.assertEquals(-1, ids.getIndexOf('missing'));

commercestoretax.GetStoreTaxesInfoResponse taxResponse = new commercestoretax.GetStoreTaxesInfoResponse(commercestoretax.TaxLocaleType.Net);
commercestoretax.StoreTaxesInfoContainer taxInfo = new commercestoretax.StoreTaxesInfoContainer();
taxResponse.addTaxesInfo('01t000000000001AAA', taxInfo);
System.assertEquals(taxInfo, taxResponse.getTaxesInfo().get('01t000000000001AAA'));
taxResponse.removeTaxesInfo('01t000000000001AAA');
System.assert(taxResponse.getTaxesInfo().isEmpty());
taxResponse.setError('bad', 'localized');
System.assertEquals('bad', taxResponse.getErrorMessage());
System.assertEquals('localized', taxResponse.getLocalizedErrorMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
ApexPages.Action action = new ApexPages.Action('{!save}');
System.assertEquals('{!save}', action.getExpression());
ApexPages.Action nullAction = new ApexPages.Action('{!null}');
System.assertEquals(null, nullAction.invoke());
ApexPages.Component component = new ApexPages.Component();
System.assertEquals(null, component.getComponentById('missing'));
ApexPages.addMessage(withDetail);
ApexPages.addMessage(new ApexPages.message(ApexPages.Severity.Error, 'Lowercase constructor'));
ApexPages.addMessage(withoutDetail);
ApexPages.addMessages(new List<ApexPages.Message>{new ApexPages.Message('WARNING', 'List message')});
try {
	throw new VisualforceException('Thrown message');
} catch (Exception e) {
	ApexPages.addMessages(e);
}
System.assert(ApexPages.hasMessages());
System.assert(ApexPages.hasMessages(ApexPages.Severity.ERROR));
System.assert(ApexPages.hasMessages(ApexPages.Severity.Info));
System.assert(ApexPages.hasMessages(ApexPages.Severity.WARNING));
System.assertEquals(5, ApexPages.getMessages().size());
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
System.assertEquals('?id=001B000001DVM9t', blank.getUrl());
System.assertEquals('text/html', blank.getHeaders().get('Accept'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFormulaBuilderEvaluateAndRecalculateLocalFields(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme', Amount__c = 40, Paid__c = 12);
formulaeval.FormulaInstance formulaInstance = Formula.builder()
	.withFormula('Amount__c - Paid__c')
	.withReturnType(formulaeval.FormulaReturnType.DECIMAL)
	.build();
System.assertEquals(28, formulaInstance.evaluate(account));
Set<String> fields = formulaInstance.getReferencedFields();
System.assert(fields.contains('Amount__c'));
System.assert(fields.contains('Paid__c'));

List<FormulaRecalcResult> results = Formula.recalculateFormulas(new List<SObject>{account});
System.assertEquals(1, results.size());
System.assert(results[0].isSuccess());
System.assertEquals(0, results[0].getErrors().size());
System.assertEquals(28, account.get('Balance__c'));

FormulaRecalcResult single = account.recalculateFormulas();
System.assert(single.isSuccess());
System.assertEquals(account, single.getSObject());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY"}
	account.Definition.Fields["Paid__c"] = storage.Field{APIName: "Paid__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY"}
	account.Definition.Fields["Balance__c"] = storage.Field{APIName: "Balance__c", Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: "Amount__c - Paid__c"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFormulaRecalculateReportsUnsupportedLocalExpression(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
List<FormulaRecalcResult> results = Formula.recalculateFormulas(new List<SObject>{account});
System.assertEquals(1, results.size());
System.assert(!results[0].isSuccess());
System.assertEquals(1, results[0].getErrors().size());
System.assertEquals('Unsupported__c', results[0].getErrors()[0].getFieldName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Unsupported__c"] = storage.Field{APIName: "Unsupported__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "HYPERLINK('/x', Name)"}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSearchFindUsesFixedSearchResults(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Nook Inc');
insert account;
Test.setFixedSearchResults(new List<Id>{account.Id});
Search.SearchResults results = Search.find('FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name)');
List<Search.SearchResult> accounts = results.get('Account');
System.assertEquals(1, accounts.size());
System.assertEquals(account.Id, accounts[0].getSObject().Id);
System.assertEquals('', accounts[0].getSnippet());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPageReferenceParametersPreserveIDText(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordId = Id.valueOf('001000000000001');
String assigned = recordId;
System.assertEquals('001000000000001AAA', assigned);
PageReference page = new PageReference('/apex/Order');
page.getParameters().put('recordId', recordId);
System.assertEquals('001000000000001AAA', page.getParameters().get('recordId'));
System.assertEquals('/apex/Order?recordId=001000000000001AAA', page.getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecVisualforceControllerAndSelectOptionSlice(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'VF');
ApexPages.StandardController controller = new ApexPages.StandardController(account);
System.assertEquals(null, controller.getId());
PageReference saved = controller.save();
System.assertEquals(account.Id, controller.getId());
System.assertEquals('/' + account.Id, saved.getUrl());
PageReference token = Page.MyPage;
System.assertEquals('/apex/MyPage', token.getUrl());
System.assertEquals('/apex/MyPage', page.MyPage.getUrl());
SelectOption option = new SelectOption('1', 'One', true, false);
System.assertEquals('1', option.getValue());
System.assertEquals('One', option.getLabel());
System.assert(option.getDisabled());
System.assertEquals(false, option.getEscapeItem());
System.assertEquals(new SelectOption('1', 'One', true, false), option);
option.setLabel('Changed');
option.setDisabled(false);
System.assertEquals('Changed', option.getLabel());
System.assertEquals(false, option.getDisabled());
ApexPages.StandardSetController setController = new ApexPages.StandardSetController(new List<Account>{account, new Account(Name = 'Second')});
System.assertEquals(2, setController.getResultSize());
System.assertEquals(account, setController.getRecord());
System.assertEquals(0, setController.getListViewOptions().size());
setController.setFilterId('00B000000000001');
System.assertEquals('00B000000000001', setController.getFilterId());
setController.setPageSize(1);
System.assertEquals(1, setController.getRecords().size());
System.assert(setController.getHasNext());
setController.setPageNumber(2);
System.assertEquals(2, setController.getPageNumber());
System.assert(setController.getHasPrevious());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSelectOptionAcceptsIdValues(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001B000001DVM9t';
SelectOption option = new SelectOption(accountId, accountId);
System.assertEquals('001B000001DVM9tIAH', option.getValue());
System.assertEquals('001B000001DVM9tIAH', option.getLabel());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredVisualforcePageReferences(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('/apex/AccountView', Page.AccountView.getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.RegisterPageReference("AccountView")
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	missing, err := CompileAnonymous(`PageReference page = Page.Missing;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(missing); err == nil || err.Error() != "unknown Visualforce page Page.Missing" {
		t.Fatalf("err = %v, want unknown Visualforce page", err)
	}
}

func TestExecVisualEditorDynamicPickListRows(t *testing.T) {
	program, err := CompileAnonymous(`
VisualEditor.DataRow row = new VisualEditor.DataRow('Template', 'a01000000000001');
System.assertEquals('Template', row.getLabel());
System.assertEquals('a01000000000001', row.getValue());
row.setLabel('Updated');
row.setValue('next');
VisualEditor.DynamicPickListRows rows = new VisualEditor.DynamicPickListRows();
rows.addRow(row);
System.assertEquals(1, rows.size());
System.assertEquals('Updated', rows.get(0).getLabel());
System.assertEquals('next', rows.getRows().get(0).getValue());
System.assertEquals('next', rows.getDataRows().get(0).getValue());
System.assertEquals(false, rows.containsAllRows());
rows.setContainsAllRows(true);
System.assertEquals(true, rows.containsAllRows());
VisualEditor.DynamicPickListRows copy = new VisualEditor.DynamicPickListRows(rows.getDataRows(), true);
copy.addAllRows(rows.getDataRows());
copy.sort();
System.assertEquals(2, copy.size());
System.assertEquals(true, copy.containsAllRows());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserInfoPackageLicenseUsesOrgAssignments(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, UserInfo.hasPackageLicense('050000000000001'));
System.assertEquals(false, UserInfo.isCurrentUserLicensed('pkg'));
System.assertEquals(false, UserInfo.isCurrentUserLicensedForPackage('050000000000001'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	licensedOrg := testDataOrg()
	licensedOrg.Objects["PackageLicense"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"050000000000001": {
			ID:     "050000000000001",
			Object: "PackageLicense",
			Fields: map[string]storage.Value{
				"NamespacePrefix": storage.StringValue("pkg"),
				"Status":          storage.StringValue("Active"),
			},
		},
	}}
	licensedOrg.Objects["UserPackageLicense"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0PL000000000001": {
			ID:     "0PL000000000001",
			Object: "UserPackageLicense",
			Fields: map[string]storage.Value{
				"PackageLicenseId": storage.IDValue("050000000000001"),
				"UserId":           storage.IDValue("005-local-user"),
			},
		},
	}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	program, err = CompileAnonymous(`
System.assertEquals(true, UserInfo.hasPackageLicense('050000000000001'));
System.assertEquals(true, UserInfo.isCurrentUserLicensed('pkg'));
System.assertEquals(true, UserInfo.isCurrentUserLicensedForPackage('050000000000001'));
System.assertEquals(false, UserInfo.isCurrentUserLicensed('missing'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	machine.SetOrg(&licensedOrg)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String("005-local-user")
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecLabelsFromLocalMetadataRegistry(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('Hello', Label.Greeting);
System.assertEquals('Hello', System.Label.Greeting);
System.assertEquals('Bonjour', Label.pkg.Greeting);
System.assertEquals('Dependency_Message', Label.ext.Dependency_Message);
System.assertEquals('invalid_email', Label.Site.invalid_email);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Namespace = "pkg"
	org.Metadata.Labels = []storage.LabelMetadata{
		{Name: "Greeting", Namespace: "pkg", Language: "en_US", Value: "Hello"},
		{Name: "Greeting", Namespace: "pkg", Language: "fr", Value: "Bonjour"},
	}
	org.Metadata.ManagedLabelNamespaces = []string{"ext"}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMessagingResultAndUnsupportedEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.EmailFileAttachment attachment = new Messaging.EmailFileAttachment();
System.assertEquals(null, attachment.getBody());
System.assertEquals(null, attachment.getContentType());
System.assertEquals(null, attachment.getFileName());
System.assertEquals(null, attachment.getId());
System.assertEquals(null, attachment.GETID());
System.assertEquals(false, attachment.getInline());
attachment.setBody(Blob.valueOf('file-body'));
attachment.setContentType('text/plain');
attachment.setFileName('trail.txt');
attachment.setInline(true);
System.assertEquals('file-body', attachment.getBody().toString());
System.assertEquals('text/plain', attachment.getContentType());
System.assertEquals('trail.txt', attachment.getFileName());
System.assertEquals('trail.txt', attachment.GETFILENAME());
System.assertEquals(true, attachment.getInline());
Messaging.SingleEmailMessage msg = new Messaging.SingleEmailMessage();
System.assertEquals(0, msg.getToAddresses().size());
System.assertEquals(0, msg.getCcAddresses().size());
System.assertEquals(0, msg.getBccAddresses().size());
System.assertEquals(0, msg.getFileAttachments().size());
System.assertEquals(0, msg.getEntityAttachments().size());
System.assertEquals(0, msg.getDocumentAttachments().size());
System.assertEquals(0, msg.getTargetObjectIds().size());
System.assertEquals(null, msg.getSubject());
System.assertEquals(null, msg.getHtmlBody());
System.assertEquals(null, msg.getPlainTextBody());
System.assertEquals(null, msg.getTemplateId());
System.assertEquals(null, msg.getTemplateName());
System.assertEquals(null, msg.getTargetObjectId());
System.assertEquals(null, msg.getWhatId());
System.assertEquals(null, msg.getUnsubscribeComment());
System.assertEquals(0, msg.getUnsubscribeUrls().size());
System.assertEquals(false, msg.getSaveAsActivity());
System.assertEquals(false, msg.getTreatBodiesAsTemplate());
System.assertEquals(false, msg.isTreatBodiesAsTemplate());
System.assertEquals(false, msg.getTreatTargetObjectAsRecipient());
System.assertEquals(false, msg.isTreatTargetObjectAsRecipient());
System.assertEquals(false, msg.getUseSignature());
System.assertEquals(false, msg.getBccSender());
System.assertEquals(false, msg.getOneClickPost());
System.assertEquals(false, msg.isUserMail());
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
msg.setUnsubscribeComment('unsubscribe comment');
msg.setUnsubscribeUrls(new List<String>{'https://example.test/unsubscribe'});
msg.setOneClickPost(true);
msg.setOptOutPolicy('FILTER');
msg.setEmailPriority('High');
msg.setBccSender(true);
msg.setFileAttachments(new List<Messaging.EmailFileAttachment>{attachment});
System.assertEquals('trail@example.test', msg.getToAddresses().get(0));
System.assertEquals('copy@example.test', msg.getCcAddresses().get(0));
System.assertEquals('blind@example.test', msg.getBccAddresses().get(0));
System.assertEquals('Trail', msg.getSubject());
System.assertEquals('Body', msg.getPlainTextBody());
System.assertEquals('<p>Body</p>', msg.getHtmlBody());
System.assertEquals('reply@example.test', msg.getReplyTo());
System.assertEquals('Trail Sender', msg.getSenderDisplayName());
System.assertEquals('UTF-8', msg.getCharset());
System.assertEquals('<message@example.test>', msg.getInReplyTo());
System.assertEquals('<root@example.test>', msg.getReferences());
System.assertEquals('0D2000000000001', msg.getOrgWideEmailAddressId());
System.assertEquals('003000000000001', msg.getTargetObjectId());
System.assertEquals('00X000000000001', msg.getTemplateId());
System.assertEquals('001000000000001', msg.getWhatId());
System.assertEquals(false, msg.getSaveAsActivity());
System.assertEquals(false, msg.getTreatBodiesAsTemplate());
System.assertEquals(false, msg.getTreatTargetObjectAsRecipient());
System.assertEquals(false, msg.getUseSignature());
System.assertEquals('015000000000001', msg.getEntityAttachments().get(0));
System.assertEquals('015000000000002', msg.getDocumentAttachments().get(0));
System.assertEquals('003000000000002', msg.getTargetObjectIds().get(0));
System.assertEquals('unsubscribe comment', msg.getUnsubscribeComment());
System.assertEquals('https://example.test/unsubscribe', msg.getUnsubscribeUrls().get(0));
System.assertEquals('FILTER', msg.getOptOutPolicy());
System.assertEquals('High', msg.getEmailPriority());
System.assertEquals(true, msg.getBccSender());
System.assertEquals(true, msg.getOneClickPost());
System.assertEquals('trail.txt', msg.getFileAttachments().get(0).getFileName());
Messaging.SingleEmailMessage second = new Messaging.SingleEmailMessage();
second.setToAddresses(new List<String>{'second@example.test'});
second.setPlainTextBody('Second body');
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
			name: "reserve capacity surface",
			src:  `Messaging.reserveMassEmailCapacity(1);`,
			want: `unsupported call "Messaging.reserveMassEmailCapacity local messaging transport/template surface"`,
		},
		{
			name: "push notification surface",
			src:  `Messaging.sendPushNotification(new List<String>{'005000000000001'}, 'payload');`,
			want: `unsupported call "Messaging.sendPushNotification local messaging transport/template surface"`,
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
		{
			name: "getter-arity",
			src:  `Messaging.SingleEmailMessage msg = new Messaging.SingleEmailMessage(); msg.getSubject('extra');`,
			want: `Messaging.SingleEmailMessage.getSubject expects 0 arguments`,
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

func TestExecPassiveGeneratedPlatformDTOAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
commercepromotions.PromotionRequest request = new commercepromotions.PromotionRequest();
System.assertEquals(null, request.getBuyerAccountId());
request.buyerAccountId = 'buyer-one';
request.webStoreId = 'store-one';
System.assertEquals('buyer-one', request.getBuyerAccountId());
System.assertEquals('store-one', request.GETWEBSTOREID());
System.assertEquals(false, request.isActive());
Map<String,Object> values = request.getAsMap();
System.assertEquals('buyer-one', (String)values.get('buyerAccountId'));
System.assertEquals('store-one', (String)values.get('webStoreId'));
commercepromotions.PromotionRequest cloned = (commercepromotions.PromotionRequest)request.clone();
System.assertEquals('buyer-one', cloned.getBuyerAccountId());
System.assert(request.equals(request));
System.assert(!request.equals(new commercepromotions.PromotionRequest()));
commercepromotions.PromotionRequest named = new commercepromotions.PromotionRequest(
	salesTransaction = new Account(Name = 'Buyer'),
	buyerAccountId = 'buyer-two',
	webStoreId = 'store-two',
	couponCodes = new List<String>{'SAVE'}
);
System.assertEquals('buyer-two', named.getBuyerAccountId());
System.assertEquals('store-two', named.GETWEBSTOREID());
System.assertEquals(1, named.getCouponCodes().size());
commercepromotions.PromotionRequest mixedCaseNamed = new commercepromotions.PromotionRequest(
	SALESTRANSACTION = new Account(Name = 'Buyer'),
	BUYERACCOUNTID = 'buyer-three',
	WEBSTOREID = 'store-three',
	COUPONCODES = new List<String>{'SHIP'}
);
System.assertEquals('buyer-three', mixedCaseNamed.getBuyerAccountId());
System.assertEquals('store-three', mixedCaseNamed.GETWEBSTOREID());
CartExtension.Cart cart = new CartExtension.Cart();
cart.setName('local cart');
cart.setCustomField('ExternalKey__c', 'external-one');
System.assertEquals('local cart', cart.getName());
System.assertEquals('external-one', (String)cart.getCustomField('ExternalKey__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestConstructPassiveGeneratedPlatformDTONamedArgsBindProperties(t *testing.T) {
	vm := New(nil)
	if err := vm.RegisterClass(Class{
		Name:       "commercepromotions.PromotionRequest",
		Namespace:  "commercepromotions",
		SuperClass: "Object",
		Access:     "global",
		Fields: map[string]Field{
			"buyerAccountId": {Name: "buyerAccountId", Type: "String", Access: "global", Property: true},
			"webStoreId":     {Name: "webStoreId", Type: "String", Access: "global", Property: true},
		},
		FieldOrder: []string{"buyerAccountId", "webStoreId"},
		Constructors: []Method{{
			Name:          "commercepromotions.PromotionRequest.<init>",
			ClassName:     "commercepromotions.PromotionRequest",
			IsConstructor: true,
			Access:        "global",
			Params:        []Param{{Name: "param1", Type: "String"}, {Name: "param2", Type: "String"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	namedArgs := map[string]Value{
		"BUYERACCOUNTID": String("buyer-one"),
		"webStoreId":     String("store-one"),
	}
	class, ok := vm.lookupClass("commercepromotions.PromotionRequest")
	if !ok {
		t.Fatal("PromotionRequest class not registered")
	}
	_, orderedArgs, ok, ambiguous := vm.matchConstructorWithNamedArgs(class, nil, namedArgs)
	if !ok || ambiguous || len(orderedArgs) != 2 {
		t.Fatalf("placeholder constructor match ok=%v ambiguous=%v args=%#v", ok, ambiguous, orderedArgs)
	}
	value, err := vm.constructValue("commercepromotions.PromotionRequest", nil, namedArgs, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got := value.Fields["buyerAccountId"]; got.Kind != ValueString || got.Text != "buyer-one" {
		t.Fatalf("buyerAccountId = %#v", got)
	}
	if got := value.Fields["webStoreId"]; got.Kind != ValueString || got.Text != "store-one" {
		t.Fatalf("webStoreId = %#v", got)
	}
	if _, ok := value.Fields["BUYERACCOUNTID"]; ok {
		t.Fatalf("unexpected case-sensitive duplicate field: %#v", value.Fields)
	}
	if _, ok := value.Fields["param1"]; ok {
		t.Fatalf("unexpected placeholder constructor field: %#v", value.Fields)
	}
}

func TestExecRenderStoredEmailTemplate(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage rendered = Messaging.renderStoredEmailTemplate('00X000000000001AAA', '003000000000001AAA', '001000000000001AAA');
System.assertEquals('00X000000000001AAA', rendered.getTemplateId());
System.assertEquals('003000000000001AAA', rendered.getTargetObjectId());
System.assertEquals('001000000000001AAA', rendered.getWhatId());
System.assertEquals('Verify subject', rendered.getSubject());
System.assertEquals('<p>Verify body</p>', rendered.getHtmlBody());
System.assertEquals('Verify body', rendered.getPlainTextBody());
Messaging.SingleEmailMessage fallback = Messaging.renderStoredEmailTemplate('00X000000000002AAA', null, null);
System.assertEquals('00X000000000002AAA', fallback.getTemplateId());
System.assertEquals(null, fallback.getTargetObjectId());
System.assertEquals(null, fallback.getWhatId());
System.assertEquals('', fallback.getSubject());
System.assertEquals('', fallback.getHtmlBody());
System.assertEquals('', fallback.getPlainTextBody());
Messaging.SingleEmailMessage merged = Messaging.renderStoredEmailTemplate('00X000000000003AAA', '003000000000001AAA', '001000000000001AAA');
System.assertEquals('Hello Ada at Acme', merged.getSubject());
System.assertEquals('<p>Ada Trail / Acme</p>', merged.getHtmlBody());
System.assertEquals('Ada Trail / Acme', merged.getPlainTextBody());
Id whoId = '003000000000001AAA';
Id whatId = '001000000000001AAA';
Messaging.SingleEmailMessage mergedFromIds = Messaging.renderStoredEmailTemplate('00X000000000003AAA', whoId, whatId);
System.assertEquals('<p>Ada Trail / Acme</p>', mergedFromIds.getHtmlBody());
try {
	Messaging.renderStoredEmailTemplate('00X000000000099AAA', null, null);
	System.assert(false);
} catch (Exception e) {
	System.assert(e.getMessage().contains('Email template not found'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "EmailTemplate")
	accountObject := org.Objects["Account"]
	accountObject.Records["001000000000001AAA"] = storage.Record{
		ID:     "001000000000001AAA",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}
	org.Objects["Account"] = accountObject
	contactObject := org.Objects["Contact"]
	contactObject.Records["003000000000001AAA"] = storage.Record{
		ID:     "003000000000001AAA",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Ada"),
			"LastName":  storage.StringValue("Trail"),
			"Name":      storage.StringValue("Ada Trail"),
		},
	}
	org.Objects["Contact"] = contactObject
	templateObject := org.Objects["EmailTemplate"]
	templateObject.Records["00X000000000001AAA"] = storage.Record{
		ID:     "00X000000000001AAA",
		Object: "EmailTemplate",
		Fields: map[string]storage.Value{
			"Subject":   storage.StringValue("Verify subject"),
			"HtmlValue": storage.StringValue("<p>Verify body</p>"),
			"Body":      storage.StringValue("Verify body"),
		},
	}
	templateObject.Records["00X000000000002AAA"] = storage.Record{
		ID:     "00X000000000002AAA",
		Object: "EmailTemplate",
		Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue("MissingBodies"),
		},
	}
	templateObject.Records["00X000000000003AAA"] = storage.Record{
		ID:     "00X000000000003AAA",
		Object: "EmailTemplate",
		Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue("Welcome"),
			"Name":          storage.StringValue("Welcome"),
			"Subject":       storage.StringValue("Hello {!Recipient.FirstName} at {!RelatedTo.Name}"),
			"HtmlValue":     storage.StringValue("<p>{!Contact.Name} / {!Account.Name}</p>"),
			"Body":          storage.StringValue("{!Contact.Name} / {!Account.Name}"),
		},
	}
	org.Objects["EmailTemplate"] = templateObject

	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "missing-template",
			src:  `Messaging.renderStoredEmailTemplate('00X000000000099AAA', null, null);`,
			want: `EmailException: Email template not found: 00X000000000099AAA`,
		},
		{
			name: "template-id-type",
			src:  `Messaging.renderStoredEmailTemplate(7, null, null);`,
			want: `Messaging.renderStoredEmailTemplate expects templateId String or Id`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			_, err = machine.Execute(program)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecSendEmailCapturesSideEffectsAndTemplates(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage direct = new Messaging.SingleEmailMessage();
direct.setToAddresses(new List<String>{'trail@example.test'});
direct.setCcAddresses(new List<String>{'copy@example.test'});
direct.setBccAddresses(new List<String>{'blind@example.test'});
direct.setSubject('Direct subject');
direct.setPlainTextBody('Direct text');
direct.setHtmlBody('<p>Direct</p>');
direct.setTargetObjectId('003000000000001AAA');
direct.setWhatId('001000000000001AAA');
direct.setSaveAsActivity(true);
direct.setEntityAttachments(new List<String>{'015000000000001AAA'});
direct.setDocumentAttachments(new List<String>{'015000000000002AAA'});

	Messaging.SingleEmailMessage templated = new Messaging.SingleEmailMessage();
	templated.setToAddresses(new List<String>{'template@example.test'});
	templated.setTemplateId('00X000000000003AAA');
	templated.setTargetObjectId('003000000000001AAA');
	templated.setWhatId('001000000000001AAA');
	Messaging.SingleEmailMessage templatedWithProperties = new Messaging.SingleEmailMessage();
	templatedWithProperties.ToAddresses = new List<String>{'property-template@example.test'};
	templatedWithProperties.TemplateId = (Id)'00X000000000003AAA';
	templatedWithProperties.TargetObjectId = (Id)'003000000000001AAA';
	templatedWithProperties.WhatId = (Id)'001000000000001AAA';

	List<Messaging.SendEmailResult> results = Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{direct, templated, templatedWithProperties});
	System.assertEquals(3, results.size());
	System.assertEquals('Hello Ada at Acme', templatedWithProperties.Subject);
	System.assertEquals('<p>Ada Trail / Acme</p>', templatedWithProperties.HtmlBody);
	System.assertEquals('Ada Trail / Acme', templatedWithProperties.PlainTextBody);
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := emailTemplateTestOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CapturedEmails) != 3 {
		t.Fatalf("captured emails = %#v", result.CapturedEmails)
	}
	first := result.CapturedEmails[0]
	if first.ToAddresses[0] != "trail@example.test" || first.CcAddresses[0] != "copy@example.test" || first.BccAddresses[0] != "blind@example.test" {
		t.Fatalf("direct recipients = %#v", first)
	}
	if first.Subject != "Direct subject" || first.PlainTextBody != "Direct text" || first.HTMLBody != "<p>Direct</p>" || !first.SaveAsActivity {
		t.Fatalf("direct body capture = %#v", first)
	}
	if first.TargetObjectID != "003000000000001AAA" || first.WhatID != "001000000000001AAA" || first.EntityAttachments[0] != "015000000000001AAA" || first.DocumentAttachments[0] != "015000000000002AAA" {
		t.Fatalf("direct metadata capture = %#v", first)
	}
	second := result.CapturedEmails[1]
	if second.TemplateID != "00X000000000003AAA" || second.Subject != "Hello Ada at Acme" || second.HTMLBody != "<p>Ada Trail / Acme</p>" || second.PlainTextBody != "Ada Trail / Acme" {
		t.Fatalf("templated capture = %#v", second)
	}
	third := result.CapturedEmails[2]
	if third.TemplateID != "00X000000000003AAA" || third.TargetObjectID != "003000000000001AAA" || third.WhatID != "001000000000001AAA" {
		t.Fatalf("property templated capture = %#v", third)
	}
}

func TestExecSendEmailCaptureRollback(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage before = new Messaging.SingleEmailMessage();
before.setToAddresses(new List<String>{'before@example.test'});
before.setPlainTextBody('before');
Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{before});
System.Savepoint sp = Database.setSavepoint();
Messaging.SingleEmailMessage after = new Messaging.SingleEmailMessage();
after.setToAddresses(new List<String>{'after@example.test'});
after.setPlainTextBody('after');
Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{after});
Database.rollback(sp);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CapturedEmails) != 1 || result.CapturedEmails[0].ToAddresses[0] != "before@example.test" {
		t.Fatalf("captured emails after rollback = %#v", result.CapturedEmails)
	}
}

func TestExecWorkflowEmailCaptureRollsBackWithFailedAutomation(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> rows = new List<Account>();
rows.add(new Account(Name = 'Acme'));
rows.add(new Account(Name = 'Bad'));
insert rows;
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.WorkflowRules = []storage.WorkflowRule{
		{
			Name:   "NotifyAcme",
			Active: true,
			Criteria: []storage.WorkflowCriteriaItem{{
				Field:     "Name",
				Operation: "equals",
				Value:     "Acme",
			}},
			EmailAlerts: []storage.WorkflowEmailAlert{{
				Name:     "Notify",
				Template: "welcome",
				Recipients: []storage.WorkflowEmailRecipient{{
					Type:      "email",
					Recipient: "workflow@example.test",
				}},
			}},
		},
		{
			Name:   "BreakBad",
			Active: true,
			Criteria: []storage.WorkflowCriteriaItem{{
				Field:     "Name",
				Operation: "equals",
				Value:     "Bad",
			}},
			FieldUpdates: []storage.WorkflowFieldUpdate{{
				Name:         "BadUpdate",
				Field:        "Missing__c",
				LiteralValue: "broken",
			}},
		},
	}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err == nil {
		t.Fatal("expected failed automation")
	}
	if len(result.CapturedEmails) != 0 {
		t.Fatalf("captured emails should roll back with failed all-or-none DML: %#v", result.CapturedEmails)
	}
	if got := len(org.Objects["Account"].Records); got != 0 {
		t.Fatalf("records after automation rollback = %d, want 0", got)
	}
	if !traceHas(result.Trace, "apex.automation.rollback", "apex.automation") {
		t.Fatalf("trace missing automation rollback: %#v", result.Trace)
	}
}

func TestExecFlowRecordCreateTraceEvents(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
insert account;
account.Name = 'Acme Updated';
update account;
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.FlowRules = []storage.FlowRule{{
		Name:   "CreateActionRequest",
		Active: true,
		RecordLookups: []storage.FlowRecordLookup{{
			Name:               "ExistingRequest",
			ObjectName:         "ActionRequest__c",
			GetFirstRecordOnly: true,
			Criteria: []storage.WorkflowCriteriaItem{
				{Field: "SourceRecordId__c", Operation: "equals", SourceField: "Id"},
				{Field: "ActionName__c", Operation: "equals", Value: "Notify"},
			},
		}},
		RecordCreates: []storage.FlowRecordCreate{{
			Name:       "CreateRequest",
			ObjectName: "ActionRequest__c",
			InputAssignments: []storage.WorkflowFieldUpdate{
				{Name: "ActionName__c", Field: "ActionName__c", LiteralValue: "Notify"},
				{Name: "SourceRecordId__c", Field: "SourceRecordId__c", SourceField: "Id"},
			},
		}},
	}}
	org.Objects["Account"] = account
	org.Objects["ActionRequest__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ActionRequest__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"ActionName__c":     {APIName: "ActionName__c", Type: storage.FieldString, Required: true},
				"SourceRecordId__c": {APIName: "SourceRecordId__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, Required: true},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"apex.flow.rule",
		"apex.flow.record_lookup",
		"apex.flow.record_create",
		"apex.flow.record_create_suppressed",
	} {
		if !traceHas(result.Trace, name, "apex.flow") {
			t.Fatalf("trace missing %s: %#v", name, result.Trace)
		}
	}
}

func TestExecWorkflowEmailRecipientTargetsRenderTemplates(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', OwnerId = '003000000000001AAA');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := emailTemplateTestOrg()
	account := org.Objects["Account"]
	if account.Definition.Fields == nil {
		account.Definition.Fields = make(map[string]storage.Field)
	}
	account.Definition.Fields["OwnerId"] = storage.Field{APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User", "Contact"}}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{
		Name:   "NotifyOwner",
		Active: true,
		Criteria: []storage.WorkflowCriteriaItem{{
			Field:     "Name",
			Operation: "equals",
			Value:     "Acme",
		}},
		EmailAlerts: []storage.WorkflowEmailAlert{{
			Name:     "Notify",
			Template: "Welcome",
			Recipients: []storage.WorkflowEmailRecipient{{
				Type: "owner",
			}},
		}},
	}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CapturedEmails) != 1 {
		t.Fatalf("captured emails = %#v", result.CapturedEmails)
	}
	email := result.CapturedEmails[0]
	if email.TargetObjectID != "003000000000001AAA" || len(email.TargetObjectIDs) != 1 {
		t.Fatalf("target capture = %#v", email)
	}
	if email.Subject != "Hello Ada at Acme" || email.PlainTextBody != "Ada Trail / Acme" {
		t.Fatalf("rendered template = %#v", email)
	}
}

func emailTemplateTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "EmailTemplate")
	accountObject := org.Objects["Account"]
	accountObject.Records["001000000000001AAA"] = storage.Record{
		ID:     "001000000000001AAA",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Acme"),
		},
	}
	org.Objects["Account"] = accountObject
	contactObject := org.Objects["Contact"]
	contactObject.Records["003000000000001AAA"] = storage.Record{
		ID:     "003000000000001AAA",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"FirstName": storage.StringValue("Ada"),
			"LastName":  storage.StringValue("Trail"),
			"Name":      storage.StringValue("Ada Trail"),
		},
	}
	org.Objects["Contact"] = contactObject
	templateObject := org.Objects["EmailTemplate"]
	templateObject.Records["00X000000000003AAA"] = storage.Record{
		ID:     "00X000000000003AAA",
		Object: "EmailTemplate",
		Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue("Welcome"),
			"Name":          storage.StringValue("Welcome"),
			"Subject":       storage.StringValue("Hello {!Recipient.FirstName} at {!RelatedTo.Name}"),
			"HtmlValue":     storage.StringValue("<p>{!Contact.Name} / {!Account.Name}</p>"),
			"Body":          storage.StringValue("{!Contact.Name} / {!Account.Name}"),
		},
	}
	org.Objects["EmailTemplate"] = templateObject
	return org
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
System.assertEquals(0, mass.getTargetObjectIds().size());
System.assertEquals(0, mass.getWhatIds().size());
System.assertEquals(null, mass.getTemplateId());
System.assertEquals(null, mass.getDescription());
System.assertEquals(null, mass.getOptOutPolicy());
System.assertEquals(null, mass.getEmailPriority());
System.assertEquals(null, mass.getReplyTo());
System.assertEquals(null, mass.getSenderDisplayName());
System.assertEquals(null, mass.getSubject());
System.assertEquals(false, mass.getSaveAsActivity());
System.assertEquals(false, mass.getBccSender());
System.assertEquals(false, mass.getUseSignature());
mass.setTargetObjectIds(new List<String>{'003000000000001', '003000000000002'});
mass.setWhatIds(new List<String>{'001000000000001'});
mass.setTemplateId('00X000000000001');
mass.setDescription('Trail mass email');
mass.setOptOutPolicy('FILTER');
mass.setEmailPriority('High');
mass.setReplyTo('reply@example.test');
mass.setSenderDisplayName('Trail Sender');
mass.setSubject('Mass subject');
mass.setSaveAsActivity(false);
mass.setBccSender(true);
mass.setUseSignature(true);
System.assertEquals('003000000000001', mass.getTargetObjectIds().get(0));
System.assertEquals('003000000000002', mass.getTargetObjectIds().get(1));
System.assertEquals('001000000000001', mass.getWhatIds().get(0));
System.assertEquals('00X000000000001', mass.getTemplateId());
System.assertEquals('Trail mass email', mass.getDescription());
System.assertEquals('FILTER', mass.getOptOutPolicy());
System.assertEquals('High', mass.getEmailPriority());
System.assertEquals('reply@example.test', mass.getReplyTo());
System.assertEquals('Trail Sender', mass.getSenderDisplayName());
System.assertEquals('Mass subject', mass.getSubject());
System.assertEquals(false, mass.getSaveAsActivity());
System.assertEquals(true, mass.getBccSender());
System.assertEquals(true, mass.getUseSignature());
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

	program, err = CompileAnonymous(`Messaging.MassEmailMessage mass = new Messaging.MassEmailMessage(); mass.getTemplateId('extra');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err == nil || err.Error() != "Messaging.MassEmailMessage.getTemplateId expects 0 arguments" {
		t.Fatalf("err = %v", err)
	}
}

func TestExecMessagingSendEmailLocalOverloadsAndLimitAccounting(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, Limits.getEmailInvocations());
List<Messaging.SendEmailResult> emptyResults = Messaging.sendEmail(new List<Messaging.SingleEmailMessage>());
System.assertEquals(1, Limits.getEmailInvocations());
System.assertEquals(0, emptyResults.size());

Messaging.SingleEmailMessage single = new Messaging.SingleEmailMessage();
single.setToAddresses(new List<String>{'single@example.test'});
single.setPlainTextBody('Single body');
Messaging.MassEmailMessage mass = new Messaging.MassEmailMessage();
mass.setTargetObjectIds(new List<String>{'003000000000001'});
mass.setTemplateId('00X000000000001');
List<Messaging.SendEmailResult> mixedResults = Messaging.sendEmail(new List<Object>{single, mass}, true);
System.assertEquals(2, Limits.getEmailInvocations());
System.assertEquals(2, mixedResults.size());
System.assert(mixedResults.get(0).isSuccess());
System.assert(mixedResults.get(1).isSuccess());
System.assertEquals(0, mixedResults.get(0).getErrors().size());
System.assertEquals(0, mixedResults.get(1).getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "invalid second argument",
			src:  `Messaging.sendEmail(new List<Messaging.SingleEmailMessage>(), 'not boolean');`,
			want: `unsupported call "Messaging.sendEmail send options overloads"`,
		},
		{
			name: "non-message item",
			src:  `Messaging.sendEmail(new List<Object>{new Messaging.SingleEmailMessage(), 'hello'});`,
			want: `Messaging.sendEmail expects SingleEmailMessage or MassEmailMessage list items`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			result, err := New(nil).Execute(program)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if result.Limits.EmailInvokes != 0 {
				t.Fatalf("email invocations = %d, want 0", result.Limits.EmailInvokes)
			}
		})
	}
}

func TestExecMessagingSendEmailInvalidSingleEmailHonorsAllOrNothing(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage invalid = new Messaging.SingleEmailMessage();
invalid.setToAddresses(new List<String>{'missing-body@example.test'});
try {
	Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{invalid});
	System.assert(false, 'Expected EmailException');
} catch (System.EmailException e) {
}
System.assertEquals(0, Limits.getEmailInvocations());
List<Messaging.SendEmailResult> results = Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{invalid}, false);
System.assertEquals(1, Limits.getEmailInvocations());
System.assertEquals(false, results[0].isSuccess());
System.assertEquals(1, results[0].getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecHttpResponseAndSendEmailResultDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
HttpResponse response = new HttpResponse();
System.assertEquals(200, response.getStatusCode());
System.assertEquals('OK', response.getStatus());
System.assertEquals('', response.getBody());
response.setBody('<response><status>ok</status></response>');
System.assertEquals('response', response.getBodyDocument().getRootElement().getName());
System.assertEquals(0, response.getHeaderKeys().size());
System.assertEquals(null, response.getHeader('missing'));

Messaging.SendEmailResult result = new Messaging.SendEmailResult();
System.assert(result.isSuccess());
System.assertEquals(0, result.getErrors().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMessagingSendEmailRejectsNonMessageItems(t *testing.T) {
	program, err := CompileAnonymous(`Messaging.sendEmail(new List<String>{'hello'});`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err == nil || err.Error() != "Messaging.sendEmail expects SingleEmailMessage or MassEmailMessage list items" {
		t.Fatalf("err = %v", err)
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

func TestExecTestInstallInvokesInstallHandler(t *testing.T) {
	program, err := CompileAnonymous(`
Test.testInstall(new InstallScript(), null);
Account account = [SELECT Id, Name FROM Account WHERE Name = 'Installed'];
System.assertEquals('Installed', account.Name);
Test.testInstall(new InstallScript(), new Version(1, 47, 0), false);
`)
	if err != nil {
		t.Fatal(err)
	}
	onInstall, err := CompileAnonymous(`
if (context.previousVersion() == null) {
	insert new Account(Name = 'Installed');
} else {
	System.assert(context.previousVersion().compareTo(new Version(1, 47, 1)) < 0);
	System.assertEquals('1.47.0', context.previousVersion().toString());
	System.assert(!context.isPush());
	System.assertEquals(null, context.installerId);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "InstallScript",
		Interfaces: []string{"InstallHandler"},
		Methods: map[string]Method{
			"onInstall": {
				Name:       "InstallScript.onInstall",
				ClassName:  "InstallScript",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "InstallContext"}},
				Program:    onInstall,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecWebServiceCalloutMockInvokesDoInvoke(t *testing.T) {
	program, err := CompileAnonymous(`
Test.setMock('WebServiceMock', new MockResponse());
Map<String, Object> response = new Map<String, Object>();
WebServiceCallout.invoke(
  new Object(),
  'request',
  response,
  new String[]{'https://example.test', 'soapAction', 'requestNS', 'requestName', 'responseNS', 'responseName', 'ResponseType'}
);
System.assertEquals('ResponseType', response.get('response_x'));
`)
	if err != nil {
		t.Fatal(err)
	}
	doInvoke, err := CompileAnonymous(`response.put('response_x', responseType);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "MockResponse",
		Interfaces: []string{"WebServiceMock"},
		Methods: map[string]Method{
			"doInvoke": {
				Name:       "MockResponse.doInvoke",
				ClassName:  "MockResponse",
				ReturnType: "void",
				Params: []Param{
					{Name: "stub", Type: "Object"},
					{Name: "request", Type: "Object"},
					{Name: "response", Type: "Map<String,Object>"},
					{Name: "endpoint", Type: "String"},
					{Name: "soapAction", Type: "String"},
					{Name: "requestName", Type: "String"},
					{Name: "responseNS", Type: "String"},
					{Name: "responseName", Type: "String"},
					{Name: "responseType", Type: "String"},
				},
				Program: doInvoke,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatalf("err = %v", err)
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

func TestExecSafeGeneratedTestHelpers(t *testing.T) {
	program, err := CompileAnonymous(`
List<Id> flexQueueOrder = Test.getFlexQueueOrder();
System.assertEquals(0, flexQueueOrder.size());
List<Id> batchJobIds = Test.enqueueBatchJobs(2);
System.assertEquals(2, batchJobIds.size());
System.assertEquals('707000000000001', String.valueOf(batchJobIds.get(0)));
System.assertEquals('707000000000002', String.valueOf(batchJobIds.get(1)));
Test.calculatePermissionSetGroup('0PG000000000001');
Id permissionSetGroupId = '0PG000000000003';
Test.calculatePermissionSetGroup(permissionSetGroupId);
Test.calculatePermissionSetGroup(new List<String>{'0PG000000000001', '0PG000000000002'});
Test.enableChangeDataCapture();
Test.setReadOnlyApplicationMode(true);
System.assertEquals(false, Test.isSoqlStubDefined(Account.SObjectType));
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

func TestExecSafeGeneratedTestHelpersRequireTestContext(t *testing.T) {
	program, err := CompileAnonymous(`Test.getFlexQueueOrder();`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err == nil || !strings.Contains(err.Error(), "only available in test context") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecTestNotificationActionHandlerInvokesExecuteAction(t *testing.T) {
	handlerProgram, err := CompileAnonymous(`
System.assertEquals(null, notification.getActionIdentifier());
return new Messaging.ActionResult();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Messaging.ActionResult result = Test.testNotificationActionHandler(new Handler(), new Messaging.ActionableNotification());
System.assertEquals(false, result.isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "Handler",
		Interfaces: []string{"Messaging.NotificationActionHandler"},
		Methods: map[string]Method{
			"executeAction": {
				Name:       "Handler.executeAction",
				ClassName:  "Handler",
				ReturnType: "Messaging.ActionResult",
				Params:     []Param{{Name: "notification", Type: "Messaging.ActionableNotification"}},
				Program:    handlerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestSandboxPostCopyScriptInvokesRunApexClass(t *testing.T) {
	copyProgram, err := CompileAnonymous(`
System.assertEquals('00D000000000001', String.valueOf(context.organizationId()));
System.assertEquals('0GR000000000001', String.valueOf(context.sandboxId()));
System.assertEquals('preview', context.sandboxName());
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.testSandboxPostCopyScript(new Copier(), '00D000000000001', '0GR000000000001', 'preview', true);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "Copier",
		Interfaces: []string{"SandboxPostCopy"},
		Methods: map[string]Method{
			"runApexClass": {
				Name:       "Copier.runApexClass",
				ClassName:  "Copier",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "SandboxContext"}},
				Program:    copyProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
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
System.assertEquals('SELECT Id, Name FROM Account', locator.getQuery());
Object iterator = locator.iterator();
System.assert(iterator.hasNext());
Account row = iterator.next();
System.assertEquals('Acme', row.Name);
System.assert(!iterator.hasNext());
Iterable<Account> typedIterable = (Iterable<Account>)locator;
Database.QueryLocator typedLocator = (Database.QueryLocator)typedIterable;
System.assertEquals('SELECT Id, Name FROM Account', typedLocator.getQuery());
Integer typedCount = 0;
for (Account typedRow : typedIterable) {
  typedCount++;
  System.assertEquals('Acme', typedRow.Name);
}
System.assertEquals(1, typedCount);
Iterable<Object> objectIterable = (Iterable<Object>)locator;
Integer objectCount = 0;
for (Object objectRow : objectIterable) {
  objectCount++;
  System.assertNotEquals(null, objectRow);
}
System.assertEquals(1, objectCount);

Object inlineLocator = Database.getQueryLocator([SELECT Id, Name FROM Account]);
Object inlineIterator = inlineLocator.iterator();
System.assert(inlineIterator.hasNext());
Account inlineRow = inlineIterator.next();
System.assertEquals('Acme', inlineRow.Name);
System.assert(!inlineIterator.hasNext());
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

func TestExecDatabaseGetQueryLocatorWithBinds(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
insert new Account(Name = 'Beta');
Map<String,Object> binds = new Map<String,Object>();
binds.put('wanted', 'Beta');
Database.QueryLocator locator = Database.getQueryLocatorWithBinds('SELECT Id, Name FROM Account WHERE Name = :wanted', binds, AccessLevel.USER_MODE);
System.assertEquals('SELECT Id, Name FROM Account WHERE Name = :wanted', locator.getQuery());
Object iterator = locator.iterator();
System.assert(iterator.hasNext());
Account row = iterator.next();
System.assertEquals('Beta', row.Name);
System.assert(!iterator.hasNext());
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

	badProgram, err := CompileAnonymous(`
Map<String,Object> binds = new Map<String,Object>();
Database.getQueryLocatorWithBinds('SELECT Id FROM Account', binds, 'USER_MODE');
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

func TestExecDatabaseGetAsyncLocatorLocalValues(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Database.QueryLocator locator = Database.getQueryLocator('SELECT Id, Name FROM Account');
String queryLocator = Database.getAsyncLocator(locator);
System.assertEquals(true, queryLocator.startsWith('local-query:SELECT Id, Name FROM Account'));

Database.SaveResult saved = Database.insert(new Account(Name = 'Beta'));
String resultLocator = Database.getAsyncLocator(saved);
System.assertEquals(true, resultLocator.startsWith('local-result:'));
System.assertEquals('already-local', Database.getAsyncLocator('already-local'));
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

func TestExecDatabaseUnitOfWorkCommitAndDiscard(t *testing.T) {
	program, err := CompileAnonymous(`
Database.UnitOfWork discarded = new Database.UnitOfWork();
discarded.insertRecord(new Account(Name = 'Discarded'));
discarded.discardWork();
discarded.commitWork();
System.assertEquals(0, ((List<Account>)Database.query('SELECT Id FROM Account WHERE Name = ''Discarded''')).size());

Database.UnitOfWork work = new Database.UnitOfWork();
Database.SaveResult insertResult = work.insertRecord(new Account(Name = 'Queued'));
System.assertEquals(false, insertResult.isSuccess());
System.assertEquals(0, ((List<Account>)Database.query('SELECT Id FROM Account WHERE Name = ''Queued''')).size());
work.commitWork();
System.assertEquals(true, insertResult.isSuccess());
System.assertNotEquals(null, insertResult.getId());
System.assertEquals(1, ((List<Account>)Database.query('SELECT Id FROM Account WHERE Name = ''Queued''')).size());

List<Account> accounts = new List<Account>{new Account(Name = 'One'), new Account(Name = 'Two')};
Database.UnitOfWork bulk = new Database.UnitOfWork();
List<Database.SaveResult> results = bulk.insertRecords(accounts);
bulk.commitWork();
System.assertEquals(2, results.size());
System.assertEquals(true, results.get(0).isSuccess());
System.assertEquals(true, results.get(1).isSuccess());
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

func TestExecDatabaseDMLAsyncAndImmediateAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Account asyncAccount = new Account(Name = 'Async');
Object asyncResult = Database.insertAsync(asyncAccount);
Object saved = Database.getAsyncSaveResult(asyncResult);
System.assert(saved.isSuccess());
System.assertNotEquals(null, saved.getId());

Account immediateUpdate = new Account(Id = saved.getId(), Name = 'Immediate');
Object updateResult = Database.updateImmediate(immediateUpdate, false);
System.assert(updateResult.isSuccess());

Object deleteResult = Database.deleteAsync(immediateUpdate, false);
Object deleted = Database.getAsyncDeleteResult(deleteResult);
System.assert(deleted.isSuccess());

List<Account> rows = Database.query('SELECT Id, Name, IsDeleted FROM Account WHERE Name = ''Immediate'' ALL ROWS');
System.assertEquals(1, rows.size());
System.assertEquals(true, rows.get(0).get('IsDeleted'));
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

func TestExecDatabaseCursorAndReplicationPlaceholders(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
insert new Account(Name = 'Beta');

Database.Cursor cursor = Database.getCursor('SELECT Id, Name FROM Account ORDER BY Name');
System.assertEquals(2, cursor.getNumRecords());
List<SObject> first = cursor.fetch(0, 1);
System.assertEquals(1, first.size());

Database.PaginationCursor page = Database.getPaginationCursor('SELECT Id, Name FROM Account ORDER BY Name');
Database.CursorFetchResult pageResult = page.fetchPage(1, 10);
System.assertEquals(1, pageResult.getRecords().size());
System.assertEquals(true, pageResult.isDone());
System.assertEquals(0, page.fetchDeleted(0, 10));

Datetime start = Datetime.newInstanceGmt(2026, 5, 15, 0, 0, 0);
Datetime finish = Datetime.newInstanceGmt(2026, 5, 15, 1, 0, 0);
Database.GetDeletedResult deleted = Database.getDeleted('Account', start, finish);
System.assertEquals(0, deleted.getDeletedRecords().size());
System.assertEquals(start, deleted.getEarliestDateAvailable());
System.assertEquals(finish, deleted.getLatestDateCovered());

Database.GetUpdatedResult updated = Database.getUpdated('Account', start, finish);
System.assertEquals(0, updated.getIds().size());
System.assertEquals(finish, updated.getLatestDateCovered());
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

func TestExecDatabaseSavepointRollback(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'before');
System.Savepoint sp = Database.setSavePoint();
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

func TestExecDatabaseRollbackPreservesAutoNumberSequence(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Ticket__c();
System.Savepoint sp = Database.setSavepoint();
insert new Ticket__c();
Database.rollback(sp);
insert new Ticket__c();
List<Ticket__c> rows = [SELECT Name FROM Ticket__c ORDER BY Name];
System.assertEquals('T-0001', rows[0].Name);
System.assertEquals('T-0003', rows[1].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Ticket__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Ticket__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString, AutoNumber: true, DisplayFormat: "T-{0000}"},
			},
		},
	}
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

func TestExecDatabaseReleaseSavepointInvalidatesReleasedAndNestedSavepoints(t *testing.T) {
	program, err := CompileAnonymous(`
System.Savepoint first = Database.setSavepoint();
insert new Account(Name = 'one');
System.Savepoint second = Database.setSavepoint();
insert new Account(Name = 'two');
Database.releaseSavepoint(second);
Database.rollback(first);
Integer afterRollback = [SELECT COUNT() FROM Account];
System.assertEquals(0, afterRollback);
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

	badProgram, err := CompileAnonymous(`
System.Savepoint first = Database.setSavepoint();
System.Savepoint second = Database.setSavepoint();
Database.releaseSavepoint(first);
Database.rollback(second);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(badProgram); err == nil || !strings.Contains(err.Error(), "invalid Savepoint") {
		t.Fatalf("expected invalid Savepoint error, got %v", err)
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

func TestExecTestSetCreatedDateUpdatesStoredSystemField(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Backdated');
insert account;
Test.setCreatedDate(account.Id, Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5));
Account row = [SELECT Id, CreatedDate FROM Account WHERE Id = :account.Id];
System.assertEquals('2026-01-02T03:04:05Z', row.CreatedDate.format());
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

func TestExecStopTestDoesNotDrainChainedAsyncJobs(t *testing.T) {
	queueProgram, err := CompileAnonymous(`Database.executeBatch(new BatchWorker(), 200);`)
	if err != nil {
		t.Fatal(err)
	}
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'scope') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`insert new Account(Name = 'batch execute');`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`insert new Account(Name = 'batch finish');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
System.enqueueJob(new QueueWorker());
Test.stopTest();
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'batch execute'].size());
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'batch finish'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "QueueWorker",
		Methods: map[string]Method{
			"execute": {Name: "QueueWorker.execute", ClassName: "QueueWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "BatchWorker",
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
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

func TestExecPublishImmediateDMLLimitsGetters(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, Limits.getPublishImmediateDML());
System.assertEquals(150, Limits.getLimitPublishImmediateDML());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPassiveLimitsGettersHaveStableValues(t *testing.T) {
	for _, getter := range []string{
		"getAggregateQueries",
		"getLimitAggregateQueries",
		"getFindSimilarCalls",
		"getLimitFindSimilarCalls",
		"getMobilePushApexCalls",
		"getLimitMobilePushApexCalls",
		"getQueryLocatorRows",
		"getLimitQueryLocatorRows",
		"getSavepointRollbacks",
		"getLimitSavepointRollbacks",
		"getSoslQueries",
		"getLimitSoslQueries",
	} {
		t.Run(getter, func(t *testing.T) {
			program, err := CompileAnonymous("Integer value = Limits." + getter + "(); System.assert(value >= 0);")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
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
Messaging.SingleEmailMessage email = new Messaging.SingleEmailMessage();
email.setToAddresses(new List<String>{'hello@example.test'});
email.setPlainTextBody('hello');
Messaging.sendEmail(new List<Messaging.SingleEmailMessage>{email});
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
System.scheduleBatch(new BatchWorker(), 'batch later', 1, 200);
System.assertEquals(5, Limits.getAsyncJobs());
System.assertEquals(2, Limits.getBatchJobs());
System.assertEquals(2, Limits.getScheduledJobs());
List<AsyncApexJob> scheduledBatches = [
	SELECT Id
	FROM AsyncApexJob
	WHERE CompletedDate = null
	AND JobType = 'BatchApex'
	AND ApexClass.Name = 'BatchWorker'
	AND ApexClass.NamespacePrefix = ''
	LIMIT 1
];
System.assertEquals(1, scheduledBatches.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
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
	if result.Limits.FutureCalls != 1 || result.Limits.QueueableJobs != 1 || result.Limits.BatchJobs != 2 || result.Limits.ScheduledJobs != 2 || result.Limits.EmailInvokes != 1 {
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

func TestExecScheduledApexResolvesExecuteBySchedulableContext(t *testing.T) {
	scheduledProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	batchProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
System.schedule('nightly', '0 0 0 * * ?', new MultiWorker());
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "MultiWorker",
		Interfaces: []string{"Schedulable", "Database.Batchable<SObject>"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "MultiWorker.execute",
		ClassName: "MultiWorker",
		Params:    []Param{{Name: "context", Type: "SchedulableContext"}},
		Program:   scheduledProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "MultiWorker.execute",
		ClassName: "MultiWorker",
		Params: []Param{
			{Name: "context", Type: "Database.BatchableContext"},
			{Name: "scope", Type: "List<SObject>"},
		},
		Program: batchProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
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

func TestExecAsyncInfoHasMaxStackDepthInQueueable(t *testing.T) {
	program, err := CompileAnonymous(`
Test.startTest();
System.enqueueJob(new QueueWorker());
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	queueProgram, err := CompileAnonymous(`
System.assertEquals(false, System.AsyncInfo.hasMaxStackDepth());
System.assertEquals(1, System.AsyncInfo.getCurrentQueueableStackDepth());
System.assertEquals(0, System.AsyncInfo.getMaximumQueueableStackDepth());
System.assertEquals(0, System.AsyncInfo.getMinimumQueueableDelayInMinutes());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "QueueWorker",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {
				Name:       "QueueWorker.execute",
				ClassName:  "QueueWorker",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "QueueableContext"}},
				Program:    queueProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAbortJobCompletedRecordsAreIdempotent(t *testing.T) {
	program, err := CompileAnonymous(`Test.startTest(); String id = System.enqueueJob(new QueueWorker()); Test.stopTest(); System.abortJob(id);`)
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
	if err := machine.RegisterMethod(Method{Name: "QueueWorker.execute", ClassName: "QueueWorker", Params: []Param{{Name: "context", Type: "QueueableContext"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	for id, record := range org.Objects["AsyncApexJob"].Records {
		status := record.Fields["Status"]
		if status.Kind != storage.ValueString || status.String != "Aborted" {
			t.Fatalf("job %s status = %#v, want Aborted", id, status)
		}
	}
}

func TestExecBatchWithEmptyStartSkipsExecuteAndRunsFinish(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`insert new Account(Name = 'executed');`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`insert new Account(Name = 'finished');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new BatchWorker(), 200);
Test.stopTest();
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'executed'].size());
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'finished'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "BatchWorker",
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFailedBatchPublishesBatchApexErrorEventTrigger(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Id = '001000000000001', Name = 'failed') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`throw new Exception('batch failed');`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`
for (BatchApexErrorEvent eventRecord : (List<BatchApexErrorEvent>)Trigger.new) {
	insert new Account(
		Name = 'batch error',
		External_Key__c = eventRecord.JobScope,
		Description = eventRecord.Phase
	);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	Test.startTest();
	Database.executeBatch(new BatchWorker(), 200);
	Test.stopTest();
} catch (Exception e) {
}
Test.getEventBus().deliver();
Account logged = [SELECT External_Key__c, Description FROM Account WHERE Name = 'batch error'];
System.assertEquals('001000000000001', logged.External_Key__c);
System.assertEquals('EXECUTE', logged.Description);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "BatchWorker",
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name:      "BatchApexErrorEventTrigger",
		Object:    "BatchApexErrorEvent",
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

func TestExecAsyncApexJobRecordsIncludeSystemTimestamps(t *testing.T) {
	program, err := CompileAnonymous(`
Test.startTest();
String id = System.enqueueJob(new QueueWorker());
Test.stopTest();
AsyncApexJob job = [SELECT Id, CreatedDate FROM AsyncApexJob WHERE Id = :id];
System.assertNotEquals(null, job.CreatedDate);
System.assertNotEquals('', job.CreatedDate.format());
`)
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
	if err := machine.RegisterMethod(Method{Name: "QueueWorker.execute", ClassName: "QueueWorker", Params: []Param{{Name: "context", Type: "QueueableContext"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestEventBusDeliverIsLocalNoop(t *testing.T) {
	program, err := CompileAnonymous(`
Test.getEventBus().deliver();
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

func TestExecBatchFinishSeesCompletedAsyncApexJob(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`
AsyncApexJob job = [SELECT Id, Status, CompletedDate FROM AsyncApexJob WHERE Id = :context.getJobId()];
System.assertEquals('Completed', job.Status);
System.assertNotEquals(null, job.CompletedDate);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new BatchWorker(), 200);
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "BatchWorker",
		Methods: map[string]Method{
			"start":  {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"finish": {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAbortJobUnknownRecordsAreTypedUnsupported(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
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
			if err := machine.RegisterMethod(Method{Name: "QueueWorker.execute", ClassName: "QueueWorker", Params: []Param{{Name: "context", Type: "QueueableContext"}}}); err != nil {
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
			name: "finalizer context job id",
			src:  `FinalizerContext fc = new FinalizerContext(); fc.getAsyncApexJobId();`,
			want: `unsupported call "FinalizerContext.getAsyncApexJobId local queueable finalizers"`,
		},
		{
			name: "finalizer context result",
			src:  `System.FinalizerContext fc = new System.FinalizerContext(); fc.getResult();`,
			want: `unsupported call "System.FinalizerContext.getResult local queueable finalizers"`,
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

func TestExecSystemAttachFinalizerIsAcceptedInTests(t *testing.T) {
	program, err := CompileAnonymous(`System.attachFinalizer(new QueueWorker());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "QueueWorker"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
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
  System.assertEquals(false, UserInfo.isMultiCurrencyOrganization());
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

func TestExecStaticUserInfoCallsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, Userinfo.isMultiCurrencyOrganization());
System.assertEquals('system', USERINFO.getuserid());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
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
String nullTimeZone;
System.assertEquals('2024-07-01', fromDateTime.format('yyyy-MM-dd', nullTimeZone));
System.assertEquals(Date.newInstance(2024, 7, 1), fromDateTime.dateGMT());
Datetime fromDateTimeGmt = Datetime.newInstanceGmt(Date.newInstance(2024, 7, 1), Time.newInstance(5, 30, 0, 250));
System.assertEquals('2024-07-01T05:30:00.25Z', fromDateTimeGmt.formatGmt());

Datetime fromMillis = Datetime.newInstance(winterLocal.getTime());
System.assertEquals(winterLocal, fromMillis);
System.assertEquals(0, Datetime.newInstance(0).getTime());

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
System.runAs(new User(Id = '005-panama-user', TimeZoneSidKey = 'America/Panama')) {
    Datetime stamp = Datetime.newInstance(Date.newInstance(2014, 11, 4), Time.newInstance(0, 0, 0, 0));
    System.assertEquals('2014-11-04T05:00:00Z', stamp.formatGmt());
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

func TestExecSiteGetSiteId(t *testing.T) {
	program, err := CompileAnonymous(`
String siteId = Site.getSiteId();
System.assertEquals('local-site', siteId);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOrgShapeBackedSiteNetworkAndCurrencyCalls(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(UserInfo.isMultiCurrencyOrganization());
System.assertEquals('0DM000000000001', Site.getSiteId());
System.assertEquals('https://local.oaer.example/local', Site.getBaseUrl());
System.assertEquals('', Site.getBaseRequestUrl());
System.assertEquals('', Site.getBaseSecureUrl());
System.assertEquals('', Site.getBaseCustomUrl());
System.assertEquals(null, Site.getDomain());
System.assertEquals(null, Site.getName());
System.assertEquals('/site/SiteTemplate.apexp', Site.getTemplate().getUrl());
System.assertEquals(null, Site.getSiteType());
System.assertEquals(null, Site.getSiteTypeLabel());
System.assertEquals('local', Site.getPathPrefix());
System.assertEquals('system@example.invalid', Site.getAdminEmail());
System.assertEquals('005000000000001', Site.getAdminId());
System.assertEquals('Local Site', Site.getMasterLabel());
System.assertEquals(true, Site.isRegistrationEnabled());
System.assertEquals(true, Site.isLoginEnabled());
System.assertEquals(true, Site.isValidUsername('user@example.invalid'));
System.assertEquals(false, Site.isValidUsername('not-an-email'));
Site.setExperienceId(Network.getNetworkId());
System.assertEquals('', Site.getErrorMessage());
System.assertEquals('', Site.getErrorDescription());
Site.forgotPassword('user@example.invalid');
User externalUser = new User(Username='external@example.invalid', LastName='External', Email='external@example.invalid', Alias='ext');
System.assertEquals('005000000000E01', Site.createExternalUser(externalUser, '001000000000001', 'secret', false));
System.assertEquals('005000000000E01', externalUser.Id);
User portalUser = new User(Username='portal@example.invalid', LastName='Portal', Email='portal@example.invalid', Alias='port');
System.assertEquals('005000000000E01', Site.createPortalUser(portalUser, '001000000000001', 'secret'));
System.assertEquals('/next', Site.login('external@example.invalid', 'secret', '/next').getUrl());
Site.validatePassword(externalUser, 'secret', 'secret');
System.assertEquals('0DB000000000001', Network.getNetworkId());
System.assertEquals('https://local.oaer.example/local/login', Network.getLoginUrl(Network.getNetworkId()));
System.assertEquals('/local', Network.communitiesLanding().getUrl());
System.assertEquals('https://local.oaer.example/local', ConnectApi.Communities.getCommunity(Network.getNetworkId()).siteUrl);
ConnectApi.UserProfiles.setPhoto(Network.getNetworkId(), UserInfo.getUserId(), '069000000000001', null);
ConnectApi.UserProfiles.deletePhoto(Network.getNetworkId(), UserInfo.getUserId());
ConnectApi.UserSettings userSettings = ConnectApi.Organization.getSettings().userSettings;
System.assertEquals('005-local-user', userSettings.userId);
ConnectApi.TimeZone zone = userSettings.timeZone;
System.assertEquals('UTC', zone.name);
System.assertEquals(false, Auth.CommunitiesUtil.isGuestUser());
Auth.JWT jwt = new Auth.JWT();
jwt.setIss('local-issuer');
System.assert(jwt.toJSONString().contains('local-issuer'));
System.setPassword(UserInfo.getUserId(), 'local-secret');
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, []string{"MultiCurrency", "Sites", "Communities"})
	machine := New(nil)
	machine.SetOrg(&org)
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

func TestSystemURLSalesforceBaseURLUsesRequestContext(t *testing.T) {
	program, err := CompileAnonymous(`
URL base = System.URL.getSalesforceBaseURL();
System.assertEquals('https://trail.example.test:8443', base.toExternalForm());
System.assertEquals('trail.example.test', base.getHost());
URL orgUrl = System.Url.getOrgDomainUrl();
System.assertEquals('https://trail.example.test:8443', orgUrl.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetServerBaseURL("https://trail.example.test:8443/")
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsScopesSupportedMixedDMLMode(t *testing.T) {
	fails, err := CompileAnonymous(`
	insert new PermissionSet(Name = 'LocalPermissions');
	insert new Account(Name = 'Acme');
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(fails); err == nil || !strings.Contains(err.Error(), "Mixed DML") {
		t.Fatalf("err = %v", err)
	}

	passes, err := CompileAnonymous(`
	insert new PermissionSet(Name = 'LocalPermissions');
	System.runAs(new User(Id = '005-user-a', ProfileId = '00e-profile-a', Username = 'user-a@example.test')) {
	  insert new Account(Name = 'Acme');
	}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(passes); err != nil {
		t.Fatal(err)
	}
}

func TestExecMixedDMLTreatsPermissionSetGroupsAsSetupObjects(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSetGroup groupRecord = new PermissionSetGroup(DeveloperName = 'LocalGroup', MasterLabel = 'Local Group');
insert groupRecord;
PermissionSet perm = new PermissionSet(Name = 'LocalPerm');
insert perm;
insert new PermissionSetGroupComponent(PermissionSetGroupId = groupRecord.Id, PermissionSetId = perm.Id);
insert new PermissionSetAssignment(PermissionSetGroupId = groupRecord.Id, AssigneeId = UserInfo.getUserId());
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

func TestExecRolelessUserDMLDoesNotTripMixedDML(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
insert new User(
	Username = 'setup@example.test',
	Alias = 'setup',
	CommunityNickname = 'setup',
	Email = 'setup@example.test',
	LastName = 'Setup',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	TimeZoneSidKey = 'UTC',
	EmailEncodingKey = 'UTF-8',
	ProfileId = '00e000000000001',
	UserRoleId = null
);
insert new User(
	Username = 'default-roleless@example.test',
	Alias = 'defrole',
	CommunityNickname = 'defrole',
	Email = 'default-roleless@example.test',
	LastName = 'Default',
	LocaleSidKey = 'en_US',
	LanguageLocaleKey = 'en_US',
	TimeZoneSidKey = 'UTC',
	EmailEncodingKey = 'UTF-8',
	ProfileId = '00e000000000001'
);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMixedDMLIsCatchableAndFailedAttemptDoesNotPoisonTransaction(t *testing.T) {
	program, err := CompileAnonymous(`
	insert new Account(Name = 'Before');
	Boolean caught = false;
	try {
	  insert new PermissionSet(Name = 'LocalPermissions');
	} catch (DmlException e) {
  caught = true;
  System.assert(e.getMessage().contains('Mixed DML'));
}
System.assert(caught);
insert new Account(Name = 'After');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecQueueGroupDMLDoesNotTripMixedDML(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Queue Target');
insert new Group(Name = 'Local Queue', DeveloperName = 'LocalQueue', Type = 'Queue');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Group")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPermissionMetadataObjectsAreSetupDML(t *testing.T) {
	setupOnly, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'LocalPermissions');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Account', PermissionsRead = true);
insert new FieldPermissions(ParentId = ps.Id, SObjectType = 'Account', Field = 'Account.Name', PermissionsRead = true);
insert new SetupEntityAccess(ParentId = ps.Id, SetupEntityId = '01p000000000001', SetupEntityType = 'ApexClass');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(setupOnly); err != nil {
		t.Fatal(err)
	}

	mixed, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'LocalPermissions');
insert ps;
insert new ObjectPermissions(ParentId = ps.Id, SObjectType = 'Account', PermissionsRead = true);
insert new Account(Name = 'Acme');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	org = testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(mixed); err == nil || !strings.Contains(err.Error(), "Mixed DML") {
		t.Fatalf("err = %v, want Mixed DML", err)
	}
}

func TestExecPermissionSetInsertDefaultsGeneratedRequiredPermissionFields(t *testing.T) {
	program, err := CompileAnonymous(`
PermissionSet ps = new PermissionSet(Name = 'LocalPermissions', Label = 'Local Permissions');
insert ps;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "PermissionSet")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	var inserted storage.Record
	for _, record := range org.Objects["PermissionSet"].Records {
		if record.Fields["Name"].String == "LocalPermissions" {
			inserted = record
			break
		}
	}
	if inserted.ID == "" {
		t.Fatalf("inserted permission set not found: %#v", org.Objects["PermissionSet"].Records)
	}
	if value := inserted.Fields["PermissionsApiEnabled"]; value.Kind != storage.ValueBoolean || value.Boolean {
		t.Fatalf("PermissionsApiEnabled = %#v, want false default", value)
	}
}

func TestExecUserInsertDefaultsGeneratedRequiredPreferenceFields(t *testing.T) {
	program, err := CompileAnonymous(`
User usr = new User(
  Username = 'local-user@example.test',
  Alias = 'local',
  Email = 'local-user@example.test',
  EmailEncodingKey = 'UTF-8',
  LastName = 'Testing',
  LanguageLocaleKey = 'en_US',
  LocaleSidKey = 'en_US',
  ProfileId = '00e000000000001',
  TimeZoneSidKey = 'America/Los_Angeles'
);
insert usr;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Profile")
	org.Objects["Profile"].Records["00e000000000001"] = storage.Record{
		ID:     "00e000000000001",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("System Administrator"),
		},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	var inserted storage.Record
	for _, record := range org.Objects["User"].Records {
		if record.Fields["Username"].String == "local-user@example.test" {
			inserted = record
			break
		}
	}
	if inserted.ID == "" {
		t.Fatalf("inserted user not found: %#v", org.Objects["User"].Records)
	}
	if value := inserted.Fields["UserPreferencesEnableLwrLexPilot"]; value.Kind != storage.ValueBoolean || value.Boolean {
		t.Fatalf("UserPreferencesEnableLwrLexPilot = %#v, want false default", value)
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
System.assertEquals('Alpha|Beta', String.join(new Set<String>{'Alpha', 'Beta'}, '|'));
System.assert(String.isBlank('   '));
System.assert(String.isNotBlank('x'));
System.assert(trimmed.equalsIgnoreCase('alpha,beta,alpha'));
System.assertEquals(false, trimmed.equalsIgnoreCase(null));
System.assertEquals(false, trimmed.equals(null));
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
System.assertEquals(d.addDays(3), d + 3);
System.assertEquals(d.addDays(-2), d - 2);
Date nextMonth = d.addMonths(1);
System.assertEquals('2026-06-02', nextMonth.format());
Date nextYear = d.addYears(1);
System.assertEquals('2027-05-02', nextYear.format());
Date parsedDate = Date.valueOf('2026-05-04');
System.assertEquals(2, d.daysBetween(parsedDate));
Object parsedDateObjectText = '2026-05-04';
System.assertEquals(parsedDate, Date.valueOf(parsedDateObjectText));
Object parsedDateObject = parsedDate;
System.assertEquals(parsedDate, Date.valueOf(parsedDateObject));
Object nullDateObject = null;
System.assertEquals(null, Date.valueOf(nullDateObject));
Date parsedDateTime = Date.valueOf('2026-05-04 23:59:58');
System.assertEquals(parsedDate, parsedDateTime);
System.assertEquals(parsedDate, Date.valueOf('2026-05-04T23:59:58Z'));
System.assertEquals(parsedDate, Date.valueOf('2026-5-4'));
System.assertEquals(parsedDate, Date.valueOf('2026-5-4 23:59:58'));
System.assertEquals(2026, Date.parse('01/01/2026').year());
System.assertEquals(2026, Date.parse('01/01/26').year());
Datetime dt = Datetime.now();
String dtText = dt.format();
System.assert(dtText.startsWith('2026-05-02T12:00:00'));
Date dtDate = dt.date();
System.assertEquals('2026-05-02', dtDate.format());
Datetime made = Datetime.newInstance(2026, 5, 2, 1, 2, 3);
System.assertEquals('2026-05-02T01:02:03Z', made.format());
System.assertEquals(1777683723000, made.getTime());
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
System.assertEquals(29, Date.daysInMonth(2024, 2));
System.assertEquals(28, Date.daysInMonth(2025, 2));
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
System.assertEquals(nowStamp, DateTime.Now());
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

func TestExecDatetimeParsesSpaceSeparatedUtcOffsetText(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime unixEpoch = Datetime.valueOfGmt('1970-01-01 00:00:00Z');
System.assertEquals('1970-01-01T00:00:00Z', unixEpoch.formatGmt());
Datetime leapDay = Datetime.valueOfGmt('2024-02-29 23:59:58Z');
System.assertEquals('2024-02-29T23:59:58Z', leapDay.formatGmt());
Datetime fractional = Datetime.valueOfGmt('2024-02-29 23:59:58.250Z');
System.assertEquals('2024-02-29T23:59:58.25Z', fractional.formatGmt());
Datetime offset = Datetime.valueOfGmt('2024-02-29 18:29:58-05:30');
System.assertEquals('2024-02-29T23:59:58Z', offset.formatGmt());
Datetime assigned = '2024-02-29 23:59:58+0000';
System.assertEquals('2024-02-29T23:59:58Z', assigned.formatGmt());
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

func TestExecDateValueOfInvalidTextCanBeCaught(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	Date.valueOf('Test0');
} catch (Exception e) {
	caught = e.getMessage().startsWith('Invalid date');
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDateTimeStringAssignmentPreservesFractionalSeconds(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime dt = '2024-01-15T10:30:45.123Z';
System.assertEquals('2024-01-15T10:30:45.123Z', dt.formatGmt());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
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
System.assertEquals(first.getId(), first.GETID());
List<Object> errors = second.getErrors();
System.assertEquals(errors.size(), second.GETERRORS().size());
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

func TestExecDatabaseDMLOptionsHeaderRuntimeBreadth(t *testing.T) {
	program, err := CompileAnonymous(`
Database.DMLOptions opts = new Database.DMLOptions();
opts.OptAllOrNone = false;
opts.AllowFieldTruncation = true;
opts.LocalizeErrors = true;
opts.EmailHeader.TriggerUserEmail = true;
opts.EmailHeader.triggerOtherEmail = false;
opts.DuplicateRuleHeader.AllowSave = true;
opts.DuplicateRuleHeader.RunAsCurrentUser = true;
opts.AssignmentRuleHeader.UseDefaultRule = true;
opts.AssignmentRuleHeader.AssignmentRuleId = '01Q000000000001';
Object locale = opts.LocaleOptions;
System.assertNotEquals(null, locale);

Object copied = opts.clone();
System.assertEquals(false, copied.OptAllOrNone);
System.assertEquals(true, copied.AllowFieldTruncation);
System.assertEquals(true, copied.EmailHeader.TriggerUserEmail);
System.assertEquals(false, copied.EmailHeader.triggerOtherEmail);
System.assertEquals(true, copied.DuplicateRuleHeader.AllowSave);
System.assertEquals(true, copied.AssignmentRuleHeader.UseDefaultRule);
System.assertEquals('01Q000000000001', copied.AssignmentRuleHeader.AssignmentRuleId);

Database.EmailHeader emailHeader = new Database.EmailHeader();
emailHeader.TriggerAutoResponseEmail = true;
Object emailHeaderCopy = emailHeader.clone();
System.assertEquals(true, emailHeaderCopy.TriggerAutoResponseEmail);

Database.AssignmentRuleHeader assignmentHeader = new Database.AssignmentRuleHeader();
assignmentHeader.UseDefaultRule = true;
System.assertEquals(true, assignmentHeader.clone().UseDefaultRule);

Database.DuplicateRuleHeader duplicateHeader = new Database.DuplicateRuleHeader();
duplicateHeader.AllowSave = true;
System.assertEquals(true, duplicateHeader.clone().AllowSave);

Database.LocaleOptions localeOptions = new Database.LocaleOptions();
System.assertNotEquals(null, localeOptions.clone());

List<Account> records = new List<Account>{new Account(Name = 'Acme'), new Account(Name = 'Beta')};
List<Database.SaveResult> results = Database.insert(records, opts);
System.assertEquals(2, results.size());
System.assert(results.get(0).isSuccess());
System.assert(results.get(1).isSuccess());
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
	lockText, _ := idValueText(machine.Globals["lockRollback"].Fields["Id"])
	lockID := storage.ID(lockText)
	if org.Objects["Account"].Records[lockID].System.Locked {
		t.Fatalf("Database.lock allOrNone rollback left %s locked", lockID)
	}
	unlockText, _ := idValueText(machine.Globals["unlockRollback"].Fields["Id"])
	unlockID := storage.ID(unlockText)
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

func TestExecDatabaseIdOverloadsForRecordActionsAndMerge(t *testing.T) {
	program, err := CompileAnonymous(`
Account restore = new Account(Name = 'Restore');
insert restore;
Id restoreId = restore.Id;
delete restore;
Database.UndeleteResult restored = Database.undelete(restoreId, false);
System.assert(restored.isSuccess());

Account purge = new Account(Name = 'Purge');
insert purge;
Id purgeId = purge.Id;
delete purge;
Database.EmptyRecycleBinResult emptied = Database.emptyRecycleBin(purgeId);
System.assert(emptied.isSuccess());

Account master = new Account(Name = 'Master');
insert master;
Account duplicate = new Account(Name = 'Duplicate');
insert duplicate;
Id duplicateId = duplicate.Id;
Database.MergeResult merged = Database.merge(master, duplicateId, false, AccessLevel.USER_MODE);
System.assert(merged.isSuccess());
System.assertEquals(duplicateId, merged.getMergedRecordIds().get(0));
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

func TestExecDatabaseAccessLevelOverloadRunsLocalDML(t *testing.T) {
	program, err := CompileAnonymous(`
Database.SaveResult inserted = Database.insert(new Account(Name = 'Acme'), true, AccessLevel.USER_MODE);
System.assert(inserted.isSuccess());
Database.SaveResult insertedSystem = Database.insert(new Account(Name = 'System Acme'), AccessLevel.SYSTEM_MODE);
System.assert(insertedSystem.isSuccess());
Database.SaveResult updatedUser = Database.update(new Account(Id = insertedSystem.getId(), Name = 'System Changed'), AccessLevel.USER_MODE);
System.assert(updatedUser.isSuccess());
Account upserted = new Account(Name = 'Upserted', Other_Key__c = 'ext-access');
Database.UpsertResult upsertResult = Database.upsert(upserted, Account.Other_Key__c, false, AccessLevel.SYSTEM_MODE);
System.assert(upsertResult.isSuccess());
Account upsertedModeOnly = new Account(Name = 'Upserted Mode Only', Other_Key__c = 'ext-access-mode-only');
Database.UpsertResult upsertModeOnly = Database.upsert(upsertedModeOnly, AccessLevel.SYSTEM_MODE);
System.assert(upsertModeOnly.isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Other_Key__c"] = storage.Field{APIName: "Other_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLKeywordsAcceptUserAndSystemMode(t *testing.T) {
	program, err := CompileAnonymous(`
Account userAccount = new Account(Name = 'User Keyword');
insert as user userAccount;
userAccount.Name = 'User Updated';
update as user userAccount;

Account systemAccount = new Account(Name = 'System Keyword');
insert as system systemAccount;
systemAccount.Name = 'System Updated';
update as system systemAccount;

upsert as user new Account(Name = 'Upsert User');
delete as system systemAccount;
undelete as system systemAccount;
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

func TestExecRequestGetCurrentBasics(t *testing.T) {
	program, err := CompileAnonymous(`
Request request = Request.getCurrent();
System.assertEquals('oaer-request-000000000001', request.getRequestId());
System.assertEquals(Quiddity.SYNCHRONOUS, request.getQuiddity());
System.assertEquals('oaer-request-000000000001', System.Request.getCurrent().getRequestId());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}

	testProgram, err := CompileAnonymous(`
System.assertEquals(Quiddity.RUNTEST_SYNC, Request.getCurrent().getQuiddity());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(testProgram); err != nil {
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
System.assertEquals('', req.getBodyAsBlob().toString());
System.assertEquals(0, req.getHeaderKeys().size());
System.assertEquals(false, req.getCompressed());
System.assertEquals(10000, req.getTimeout());
req.setEndpoint('callout:NamedCredential/path');
req.setMethod('post');
System.assertEquals('POST', req.getMethod());
req.setHeader('X-Test', 'first');
req.setHeader('x-test', 'second');
req.setHeader('Accept', 'application/json');
System.assertEquals('second', req.getHeader('X-TEST'));
System.assertEquals(null, req.getHeader('Missing'));
System.assertEquals(2, req.getHeaderKeys().size());
System.assertEquals('accept', req.getHeaderKeys().get(0));
System.assertEquals('x-test', req.getHeaderKeys().get(1));
req.setBody('');
System.assertEquals('', req.getBody());
System.assertEquals('', req.getBodyAsBlob().toString());
req.setBody('text-body');
System.assertEquals('746578742d626f6479', EncodingUtil.convertToHex(req.getBodyAsBlob()));
req.setBodyAsBlob(Blob.valueOf('blob-body'));
System.assertEquals('blob-body', req.getBody());
System.assertEquals('blob-body', req.getBodyAsBlob().toString());
req.setBodyAsBlob(Blob.valueOf(''));
System.assertEquals('', req.getBody());
System.assertEquals('', EncodingUtil.convertToHex(req.getBodyAsBlob()));
req.setBody('<request><name>local</name></request>');
System.assertEquals('request', req.GETBODYDOCUMENT().getRootElement().getName());
Dom.Document doc = new Dom.Document();
doc.load('<payload><value>42</value></payload>');
req.SETBODYDOCUMENT(doc);
System.assert(req.getBody().contains('<payload>'));
System.assertEquals('42', req.getBodyDocument().getRootElement().getChildElement('value', null).getText());
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
res.setBody('<response><status>ok</status></response>');
System.assertEquals('response', res.getBodyDocument().getRootElement().getName());
XmlStreamReader reader = res.GETXMLSTREAMREADER();
System.assertEquals(1, reader.next());
System.assertEquals('response', reader.getLocalName());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticResourceCalloutMocks(t *testing.T) {
	program, err := CompileAnonymous(`
StaticResourceCalloutMock singleMock = new StaticResourceCalloutMock();
singleMock.setStaticResource('Single_Response');
singleMock.setStatusCode(203);
singleMock.setHeader('Content-Type', 'application/json');
Test.setMock('HttpCalloutMock', singleMock);
HttpRequest firstReq = new HttpRequest();
firstReq.setEndpoint('https://example.test/single');
firstReq.setMethod('GET');
HttpResponse first = new Http().send(firstReq);
System.assertEquals(203, first.getStatusCode());
System.assertEquals('{"single":true}', first.getBody());
System.assertEquals('application/json', first.getHeader('content-type'));

MultiStaticResourceCalloutMock multiMock = new MultiStaticResourceCalloutMock();
multiMock.setStaticResource('https://example.test/a', 'Response_A');
multiMock.setStaticResource('https://example.test/b', 'Response_B');
multiMock.setStatusCode(204);
Test.setMock('HttpCalloutMock', multiMock);
HttpRequest reqA = new HttpRequest();
reqA.setEndpoint('https://example.test/a');
reqA.setMethod('GET');
System.assertEquals('A-body', new Http().send(reqA).getBody());
HttpRequest reqB = new HttpRequest();
reqB.setEndpoint('https://example.test/b');
reqB.setMethod('GET');
HttpResponse second = new Http().send(reqB);
System.assertEquals(204, second.getStatusCode());
System.assertEquals('B-body', second.getBody());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["StaticResource"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "StaticResource",
			KeyPrefix: "081",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
				"Body": {APIName: "Body", Type: storage.FieldBlob},
			},
		},
		Records: map[storage.ID]storage.Record{
			"081000000000001": {ID: "081000000000001", Object: "StaticResource", Fields: map[string]storage.Value{"Name": storage.StringValue("Single_Response"), "Body": storage.BlobValue(`{"single":true}`)}},
			"081000000000002": {ID: "081000000000002", Object: "StaticResource", Fields: map[string]storage.Value{"Name": storage.StringValue("Response_A"), "Body": storage.BlobValue("A-body")}},
			"081000000000003": {ID: "081000000000003", Object: "StaticResource", Fields: map[string]storage.Value{"Name": storage.StringValue("Response_B"), "Body": storage.BlobValue("B-body")}},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticResourceCalloutMocksUseRegistryAndNamedEndpoint(t *testing.T) {
	program, err := CompileAnonymous(`
MultiStaticResourceCalloutMock multiMock = new MultiStaticResourceCalloutMock();
multiMock.setStaticResource('https://billing.example.test/v1/accounts', 'Account_Response');
Test.setMock('HttpCalloutMock', multiMock);
HttpRequest req = new HttpRequest();
req.setEndpoint('callout:Billing/v1/accounts');
req.setMethod('GET');
System.assertEquals('account-body', new Http().send(req).getBody());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "Account_Response", Content: "account-body"}}
	org.Metadata.Endpoints = []storage.EndpointMetadata{{Kind: "NamedCredential", Name: "Billing", URL: "https://billing.example.test", Active: true}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
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
			name: "endpoint-empty",
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint('');`,
			want: "HttpRequest endpoint is required",
		},
		{
			name: "callout-empty",
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint('callout:');`,
			want: "HttpRequest endpoint named credential is required",
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

func TestExecHttpSendWithoutMockInTestContextIsUnsupportedTransport(t *testing.T) {
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
	machine := New(nil)
	machine.EnableTestContext()
	result, err := machine.Execute(program)
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

func TestExecContinuationLocalTestHarnessContainers(t *testing.T) {
	program, err := CompileAnonymous(`
Continuation cont = new Continuation(60);
System.assertEquals(60, cont.Timeout);
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test');
req.setMethod('GET');
String label = cont.addHttpRequest(req);
System.assertEquals('request-1', label);
System.assertEquals(req, cont.getRequests().get(label));
HttpResponse response = new HttpResponse();
response.setStatusCode(202);
response.setBody('accepted');
Test.setContinuationResponse(label, response);
HttpResponse observed = (HttpResponse)Continuation.getResponse(label);
System.assertEquals(202, observed.getStatusCode());
System.assertEquals('accepted', observed.getBody());
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

func TestExecCanvasTestMockRenderContextAndLifecycle(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,String> app = new Map<String,String>();
app.put('canvasUrl', 'https://canvas.example');
app.put('name', 'Local Canvas');
Map<String,String> env = new Map<String,String>();
env.put('displayLocation', 'Visualforce');
env.put('locationUrl', '/apex/Host');
Canvas.RenderContext ctx = Canvas.Test.mockRenderContext(app, env);
System.assertEquals('https://canvas.example', ctx.getApplicationContext().getCanvasUrl());
System.assertEquals('Local Canvas', ctx.getApplicationContext().getName());
System.assertEquals('Visualforce', ctx.getEnvironmentContext().getDisplayLocation());
System.assertEquals('/apex/Host', ctx.getEnvironmentContext().getLocationUrl());
ctx.getEnvironmentContext().addEntityField('Account.Name');
System.assertEquals('Account.Name', ctx.getEnvironmentContext().getEntityFields().get(0));
Canvas.CanvasLifecycleHandler handler = new Canvas.CanvasLifecycleHandler();
System.assertEquals(0, handler.excludeContextTypes().size());
Canvas.Test.testCanvasLifecycle(handler, ctx);
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

func TestExecHttpCalloutMockRespondMustReturnHttpResponse(t *testing.T) {
	respondProgram, err := CompileAnonymous(`return 'not-response';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new BadMock());
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test');
req.setMethod('GET');
Http h = new Http();
h.send(req);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterMethod(Method{
		Name:       "BadMock.respond",
		ClassName:  "BadMock",
		ReturnType: "String",
		Params:     []Param{{Name: "req", Type: "HttpRequest"}},
		Program:    respondProgram,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "HttpCalloutMock.respond must return HttpResponse") {
		t.Fatalf("err = %v, want HttpResponse return validation", err)
	}
}
