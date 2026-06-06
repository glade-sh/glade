package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestGeneratedFamilyUnsupportedTypePrefixIsCaseInsensitive(t *testing.T) {
	cases := []struct {
		typeName string
		want     bool
	}{
		{typeName: "Messaging.ActionResult.Builder", want: true},
		{typeName: "metadata.CustomMetadata", want: true},
		{typeName: "Cache.OrgPartition", want: true},
		{typeName: "ConnectApi.ChatterFeeds", want: false},
		{typeName: "Database.QueryLocator", want: false},
	}
	for _, tc := range cases {
		if got := generatedFamilyUnsupportedTypePrefix(tc.typeName); got != tc.want {
			t.Fatalf("generatedFamilyUnsupportedTypePrefix(%q)=%v want %v", tc.typeName, got, tc.want)
		}
	}
}

func TestGeneratedPassiveUnsupportedStaticFamilyMethodThrowsUnsupportedOperation(t *testing.T) {
	machine := New(nil)
	callee, ok := findPassiveGeneratedStaticFamilyCalleeForTest(machine)
	if !ok {
		t.Skip("no passive-generated static family callee with zero arguments found")
	}
	_, err := machine.call(callee, nil, nil, &Result{})
	if err == nil {
		t.Fatalf("expected %s to throw UnsupportedOperationException", callee)
	}
	if !strings.Contains(err.Error(), "UnsupportedOperationException") {
		t.Fatalf("expected UnsupportedOperationException, got %v", err)
	}
	if !strings.Contains(err.Error(), callee+" local stub surface") {
		t.Fatalf("expected local stub surface message for %s, got %v", callee, err)
	}
}

func TestExecWebStoreContextGetCommerceContextUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`WebStoreContext.getCommerceContext();`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), `unsupported call "WebStoreContext.getCommerceContext local commerce context service"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func findPassiveGeneratedStaticFamilyCalleeForTest(vm *VM) (string, bool) {
	for className, methodsByName := range generatedPlatformMethodIndex {
		if !generatedFamilyUnsupportedTypePrefix(className) {
			continue
		}
		for _, methods := range methodsByName {
			for _, method := range methods {
				if !method.IsStatic || len(method.Params) != 0 || !passiveGeneratedMethod(method) {
					continue
				}
				if method.ClassName == "" {
					method.ClassName = className
				}
				if !vm.generatedPlatformMethodAllowsDefault(method) {
					continue
				}
				return method.ClassName + "." + apexMethodMemberName(method.Name), true
			}
		}
	}
	return "", false
}

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
	account := org.Objects["Account"]
	nameField := account.Definition.Fields["Name"]
	nameField.Required = true
	account.Definition.Fields["Name"] = nameField
	org.Objects["Account"] = account
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

func TestExecCustomMetadataSOQLDoesNotSpendQueryLimits(t *testing.T) {
	program, err := CompileAnonymous(`
List<Feature__mdt> rows = [SELECT Id, DeveloperName FROM Feature__mdt];
System.assertEquals(1, rows.size());
System.assertEquals(0, Limits.getQueries());
System.assertEquals(0, Limits.getQueryRows());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Feature__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Feature__mdt",
			Fields: map[string]storage.Field{
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"m01000000000001": {
				ID:     "m01000000000001",
				Object: "Feature__mdt",
				Fields: map[string]storage.Value{
					"DeveloperName": storage.StringValue("Default"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
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
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureDeterministicPlatformData(&org)
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

dom.Document doc = new dom.Document();
dom.XmlNode root = doc.createrootelement('root', null, null);
root.setattribute('name', 'local');
System.assertEquals('local', root.getattribute('name', null));
System.assert(doc.toxmlstring().contains('name="local"'));

Messaging.SingleEmailMessage email = new messaging.singleemailmessage();
email.setSubject('Hello');
System.assertEquals('Hello', email.getSubject());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAppLauncherControllerLocalHelpers(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('/ForgotPassword', applauncher.LoginFormController.getForgotPasswordUrl());
System.assertEquals('/SelfRegister', applauncher.LoginFormController.getSelfRegistrationUrl());
System.assertEquals('', applauncher.LoginFormController.getLoginRightFrameUrl());
System.assert(applauncher.LoginFormController.getIsSelfRegistrationEnabled());
System.assert(applauncher.LoginFormController.getIsUsernamePasswordEnabled());
Map<String, Boolean> enabled = applauncher.LoginFormController.getUsernamePasswordSelfRegEnabled();
System.assertEquals(true, enabled.get('usernamePasswordEnabled'));
System.assertEquals(true, enabled.get('selfRegistrationEnabled'));
System.assertEquals('/home', applauncher.LoginFormController.login('local@example.test', 'secret', '/home'));
System.assertEquals('/', applauncher.LoginFormController.loginGetPageRefUrl('local@example.test', 'secret', ''));
System.assertEquals('', applauncher.LoginFormController.setExperienceId('0DB-local-network'));

System.assert(applauncher.SelfRegisterController.isValidPassword('secret', 'secret'));
System.assert(!applauncher.SelfRegisterController.isValidPassword('secret', 'different'));
System.assertEquals(0, applauncher.SelfRegisterController.getExtraFields('Extra_Fields').size());
System.assertEquals('/confirm', applauncher.SelfRegisterController.selfRegisterGetRedirectUrl('First', 'Last', 'local@example.test', 'secret', 'secret', null, '/confirm', '{}', '/start', true, true).toString());
System.assertEquals('/start', applauncher.SelfRegisterController.commonSelfRegisterGetRedirectUrl('First', 'Last', 'local@example.test', 'secret', 'secret', null, '', '{}', '/start', true, true, false, '{}'));
System.assertEquals('', applauncher.SelfRegisterController.setExperienceId('0DB-local-network'));

System.assertEquals(0, applauncher.SocialLoginController.getAuthProviders().size());
System.assertEquals(0, applauncher.SocialLoginController.getSamlProviders().size());
System.assertEquals('https://local.glade.example/services/auth/sso/LocalProvider?startURL=%2Fstart', applauncher.SocialLoginController.getSsoUrl('/start', 'LocalProvider'));
System.assertEquals('https://local.glade.example/services/auth/sso/LocalProvider?startURL=%2Fstart', applauncher.SocialLoginController.getCommunityDomainSsoUrl('/start', 'LocalProvider'));
System.assertEquals('https://local.glade.example/services/auth/saml/SamlProvider?startURL=%2Fstart', applauncher.SocialLoginController.getSamlSsoUrl('/start', 'SamlProvider'));
System.assertEquals('https://local.glade.example/services/auth/saml/SamlProvider?startURL=%2Fstart', applauncher.SocialLoginController.getSamlSsoUrlNoCache('/start', 'SamlProvider'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAppLauncherControllerServiceFlowsStayUnsupported(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "registration service",
			src:  `applauncher.SelfRegisterController.selfRegister('First', 'Last', 'local@example.test', 'secret', 'secret', null, '/confirm', '{}', '/start', true);`,
			want: `unsupported call "applauncher.SelfRegisterController.selfRegister local user registration service flow"`,
		},
		{
			name: "identity provider callback",
			src:  `applauncher.SocialLoginController.handleIdp();`,
			want: `unsupported call "applauncher.SocialLoginController.handleIdp local identity provider callback flow"`,
		},
		{
			name: "forgot password flow",
			src:  `applauncher.ForgotPasswordController.forgotPassword('user@example.test', '/home');`,
			want: `unsupported call "applauncher.ForgotPasswordController.forgotPassword local password reset flow"`,
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
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want || err.Error() != tc.want {
				t.Fatalf("error = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
	}
}

func TestExecPackagedControllerLocalDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, mapslite.MapsLiteUtils.userHasMaps());
System.assertEquals('', mapslite.MapsLiteUtils.accessCheck());
System.assertEquals('', regrelloapex.LoginFormController.getForgotPasswordUrl());
System.assertEquals('', setup_service_livemessage.MessagingChannelAppleDomainController.getApplePayDomain('local.example'));
System.assertEquals(0, wave.Dags.getDags(new wave.DagsSearchOptions()).size());
System.assertEquals(0, wave.NodeType.valueOf('source').ordinal());
System.assertEquals(0, wave.ProjectionType.valueOf('dim').ordinal());

ime_mrm.EventManagementBudgetApi budget = new ime_mrm.EventManagementBudgetApi();
Map<String,Object> budgetResult = budget.invokeMethod('get', new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>());
System.assertEquals(true, budgetResult.get('success'));

wavetemplate.Answers answers = new wavetemplate.Answers();
answers.put('key', 'value');
System.assertEquals('value', answers.get('key'));
System.assertEquals(false, wavetemplate.Access.integUserHasAccessToSObjectField('Account', 'Name'));

aiaccelerator.SampleCustomFeatureExtractor extractor = new aiaccelerator.SampleCustomFeatureExtractor();
System.assertEquals(0, extractor.extractFeatures(new List<Map<String,Object>>(), new Map<String,Object>()).size());
System.assertEquals('', omnichannel.RouteWorkApexController.search('work', 'Account', 'Name'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPackagedControllerServiceFlowsStayUnsupported(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "archive mutation",
			src:  `SF_Archive.ArchiverAccessor.maskArchivedRecords(new List<SF_Archive.Criteria>());`,
			want: `unsupported call "SF_Archive.ArchiverAccessor.maskArchivedRecords"`,
		},
		{
			name: "domain upload",
			src:  `setup_service_livemessage.MessagingChannelAppleDomainController.uploadDomainVerificationCertificate('local.example', 'certificate');`,
			want: `unsupported call "setup_service_livemessage.MessagingChannelAppleDomainController.uploadDomainVerificationCertificate"`,
		},
		{
			name: "maps geocode",
			src:  `mapslite.MapsLiteUtils.falconGeocodeRecords('Account');`,
			want: `unsupported call "mapslite.MapsLiteUtils.falconGeocodeRecords local maps geocode service flow"`,
		},
		{
			name: "quote execution",
			src:  `placequote.PlaceQuoteExecutor.execute(placequote.PricingPreferenceEnum.System, new placequote.GraphRequest('graph', new List<placequote.RecordWithReferenceRequest>()));`,
			want: `unsupported call "placequote.PlaceQuoteExecutor.execute"`,
		},
		{
			name: "session handler",
			src:  `embeddedMessaging.EmbeddedMessagingSessionHandler.handleRequestWithSfdcSession(new embeddedMessaging.EmbeddedMessagingAccessTokenRequest('channel', UserInfo.getUserId(), new List<String>()));`,
			want: `unsupported call "embeddedMessaging.EmbeddedMessagingSessionHandler.handleRequestWithSfdcSession"`,
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
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want || err.Error() != tc.want {
				t.Fatalf("error = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
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
			want: `unsupported call "Approval.ProcessWorkitemRequest.setAction local approval process metadata"`,
		},
		{
			name: "approval process api",
			src:  `Approval.process(new Approval.ProcessSubmitRequest());`,
			want: `unsupported call "Approval.process local approval process metadata"`,
		},
		{
			name: "approval process api all or none",
			src:  `Approval.process(new Approval.ProcessSubmitRequest(), false);`,
			want: `unsupported call "Approval.process local approval process metadata"`,
		},
		{
			name: "business hours add api",
			src:  `BusinessHours.add('01m000000000001AAA', Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0), 1250);`,
			want: `unsupported call "BusinessHours.add local business hours metadata"`,
		},
		{
			name: "business hours add gmt api",
			src:  `BusinessHours.addGmt('01m000000000001AAA', Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0), 1250);`,
			want: `unsupported call "BusinessHours.addGmt local business hours metadata"`,
		},
		{
			name: "business hours diff api",
			src:  `BusinessHours.diff('01m000000000001AAA', Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0), Datetime.newInstanceGmt(2026, 5, 15, 11, 0, 0));`,
			want: `unsupported call "BusinessHours.diff local business hours metadata"`,
		},
		{
			name: "business hours is within api",
			src:  `BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0));`,
			want: `unsupported call "BusinessHours.isWithin local business hours metadata"`,
		},
		{
			name: "business hours next start date api",
			src:  `BusinessHours.nextStartDate('01m000000000001AAA', Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0));`,
			want: `unsupported call "BusinessHours.nextStartDate local business hours metadata"`,
		},
		{
			name: "answers find similar api",
			src:  `Answers.findSimilar(new Question(Title = 'Acme'));`,
			want: `unsupported call "Answers.findSimilar local Answers zone search surface"`,
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
			name: "canvas integration exact parameters api",
			src:  `Canvas.EnvironmentContext.getParametersAsJSON();`,
			want: `unsupported call "Canvas.EnvironmentContext.getParametersAsJSON local canvas app integration surface"`,
		},
		{
			name: "canvas application context",
			src:  `Canvas.ApplicationContext.getCanvasUrl();`,
			want: `unsupported call "Canvas.ApplicationContext.getCanvasUrl local canvas app integration surface"`,
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
		{
			name: "crypto managed iv authenticated data encrypt",
			src:  `Crypto.encryptWithManagedIV('AES256', Blob.valueOf('0123456789abcdef0123456789abcdef'), Blob.valueOf('hello'), Blob.valueOf('aad'));`,
			want: `unsupported call "Crypto.encryptWithManagedIV local authenticated-data managed-IV AES surface"`,
		},
		{
			name: "crypto managed iv authenticated data decrypt",
			src:  `Crypto.decryptWithManagedIV('AES256', Blob.valueOf('0123456789abcdef0123456789abcdef'), Blob.valueOf('cipher'), Blob.valueOf('aad'));`,
			want: `unsupported call "Crypto.decryptWithManagedIV local authenticated-data managed-IV AES surface"`,
		},
		{
			name: "system password reset",
			src:  `System.resetPassword('005000000000001', true);`,
			want: `unsupported call "System.resetPassword local password/admin mutation surface"`,
		},
		{
			name: "system password move",
			src:  `System.movePassword('005000000000001', '005000000000002');`,
			want: `unsupported call "System.movePassword local password/admin mutation surface"`,
		},
		{
			name: "system approval process",
			src:  `System.process(new List<SObject>(), 'Approve', 'comment', 'next');`,
			want: `unsupported call "System.process local approval submit/process surface"`,
		},
		{
			name: "system approval submit",
			src:  `System.submit(new List<SObject>(), 'comment', 'next');`,
			want: `unsupported call "System.submit local approval submit/process surface"`,
		},
		{
			name: "data mask run job",
			src:  `data_mask.DataMaskIntegrationUtil.runMask('{}');`,
			want: `unsupported call "data_mask.DataMaskIntegrationUtil.runMask local data mask job surface"`,
		},
		{
			name: "data mask cancel job",
			src:  `data_mask.DataMaskIntegrationUtil.cancelJob('job-local');`,
			want: `unsupported call "data_mask.DataMaskIntegrationUtil.cancelJob local data mask job surface"`,
		},
		{
			name: "commerce inventory delete reservation",
			src:  `new commerce_inventory.CommerceInventoryService().deleteReservation('res-local', new commerce_inventory.InventoryReservation());`,
			want: `unsupported call "commerce_inventory.CommerceInventoryService.deleteReservation local commerce inventory mutation surface"`,
		},
		{
			name: "commerce inventory upsert reservation",
			src:  `new commerce_inventory.CommerceInventoryService().upsertReservation(new commerce_inventory.UpsertReservationRequest(0, 'res-local', '001000000000001AAA', new List<commerce_inventory.UpsertItemReservationRequest>()), new commerce_inventory.InventoryReservation(), 'ALL');`,
			want: `unsupported call "commerce_inventory.CommerceInventoryService.upsertReservation local commerce inventory mutation surface"`,
		},
		{
			name: "knowledge delete draft",
			src:  `KbManagement.PublishingService.deleteDraftArticle('ka0000000000001');`,
			want: `unsupported call "KbManagement.PublishingService.deleteDraftArticle local Knowledge delete surface"`,
		},
		{
			name: "knowledge delete archived version",
			src:  `KbManagement.PublishingService.deleteArchivedArticleVersion('ka0000000000001', 1);`,
			want: `unsupported call "KbManagement.PublishingService.deleteArchivedArticleVersion local Knowledge delete surface"`,
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

func TestExecSearchQueryOrdersFixedSearchResultsByReturningField(t *testing.T) {
	program, err := CompileAnonymous(`
Account testAccount = new Account(Name = 'Test Account');
Account anotherAccount = new Account(Name = 'Another Account');
insert new List<Account>{testAccount, anotherAccount};
Test.setFixedSearchResults(new List<Id>{testAccount.Id, anotherAccount.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Account*} IN ALL FIELDS RETURNING Account(Id, Name ORDER BY Name)')[0];
System.assertEquals(2, rows.size());
System.assertEquals(anotherAccount.Id, rows[0].Id);
System.assertEquals(testAccount.Id, rows[1].Id);
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

func TestExecSearchQueryAppliesReturningLimitAfterOrder(t *testing.T) {
	program, err := CompileAnonymous(`
Account beta = new Account(Name = 'Beta Account');
Account alpha = new Account(Name = 'Alpha Account');
Account gamma = new Account(Name = 'Gamma Account');
insert new List<Account>{beta, alpha, gamma};
Test.setFixedSearchResults(new List<Id>{beta.Id, alpha.Id, gamma.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Account*} IN ALL FIELDS RETURNING Account(Id, Name ORDER BY Name LIMIT 1)')[0];
System.assertEquals(1, rows.size());
System.assertEquals(alpha.Id, rows[0].Id);
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

func TestExecSearchQueryAppliesReturningWhere(t *testing.T) {
	program, err := CompileAnonymous(`
Account beta = new Account(Name = 'Beta Account');
Account alpha = new Account(Name = 'Alpha Account');
insert new List<Account>{beta, alpha};
Test.setFixedSearchResults(new List<Id>{beta.Id, alpha.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Account*} IN ALL FIELDS RETURNING Account(Id, Name WHERE Name = ''Alpha Account'')')[0];
System.assertEquals(1, rows.size());
System.assertEquals(alpha.Id, rows[0].Id);
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

func TestExecSearchQueryReturningWhereMatchesTextCaseInsensitively(t *testing.T) {
	program, err := CompileAnonymous(`
Account alpha = new Account(Name = 'Alpha Account');
insert alpha;
Test.setFixedSearchResults(new List<Id>{alpha.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Account*} IN ALL FIELDS RETURNING Account(Id, Name WHERE Name = ''alpha account'')')[0];
System.assertEquals(1, rows.size());
System.assertEquals(alpha.Id, rows[0].Id);
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

func TestExecSearchQueryProjectsReturningFormatAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme', AnnualRevenue = 100);
insert account;
Test.setFixedSearchResults(new List<Id>{account.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Acme} IN ALL FIELDS RETURNING Account(Id, AnnualRevenue, FORMAT(AnnualRevenue) formattedRevenue)')[0];
System.assertEquals(1, rows.size());
System.assertEquals('100.0', rows[0].get('formattedRevenue'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "AnnualRevenue": {APIName: "AnnualRevenue", Type: storage.FieldDecimal}}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSearchQueryProjectsReturningConvertCurrencyAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme', AnnualRevenue = 100);
insert account;
Test.setFixedSearchResults(new List<Id>{account.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Acme} IN ALL FIELDS RETURNING Account(Id, AnnualRevenue, convertCurrency(AnnualRevenue) convertedRevenue)')[0];
System.assertEquals(1, rows.size());
System.assertEquals(100.0, rows[0].get('convertedRevenue'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "AnnualRevenue": {APIName: "AnnualRevenue", Type: storage.FieldDecimal}}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSearchQueryProjectsReturningToLabelAlias(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme', Rating = 'Hot');
insert account;
Test.setFixedSearchResults(new List<Id>{account.Id});
List<Account> rows = (List<Account>)Search.query('FIND {Acme} IN ALL FIELDS RETURNING Account(Id, Rating, toLabel(Rating) ratingLabel)')[0];
System.assertEquals(1, rows.size());
System.assertEquals('Hot Label', rows[0].get('ratingLabel'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":   {APIName: "Name", Type: storage.FieldString},
			"Rating": {APIName: "Rating", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Hot", Label: "Hot Label"}}},
		}},
		Records: map[storage.ID]storage.Record{},
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
	machine.EnableTestContext()
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

func TestExecMetadataEnumStaticAndInstanceBehavior(t *testing.T) {
	program, err := CompileAnonymous(`
List<Metadata.DeployStatus> statuses = Metadata.DeployStatus.values();
System.assertEquals(8, statuses.size());
System.assert(statuses.contains(Metadata.DeployStatus.SUCCEEDED));
Metadata.DeployStatus succeeded = Metadata.DeployStatus.valueOf('SUCCEEDED');
System.assertEquals(Metadata.DeployStatus.SUCCEEDED, succeeded);
System.assert(succeeded.equals(Metadata.DeployStatus.SUCCEEDED));
System.assert(!succeeded.equals(Metadata.DeployStatus.FAILED));
System.assertEquals('SUCCEEDED', succeeded.name());
System.assertEquals('SUCCEEDED', succeeded.toString());
System.assertEquals('SUCCEEDED', String.valueOf(succeeded));
Metadata.MetadataType metadataType = Metadata.MetadataType.valueOf('CustomMetadata');
System.assertEquals(Metadata.MetadataType.CustomMetadata, metadataType);
System.assertEquals('CustomMetadata', metadataType.name());
System.assertEquals('CustomMetadata', Metadata.MetadataType.values()[0].toString());
try {
	Metadata.DeployStatus.valueOf('missing');
	System.assert(false);
} catch (Exception e) {
	System.assertEquals('System.NoSuchElementException', e.getTypeName());
	System.assert(e.getMessage().contains('No enum value found called missing'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSearchAccessLevelOverloadsUseLocalModel(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Nook Inc');
insert account;
Test.setFixedSearchResults(new List<Id>{account.Id});
List<List<SObject>> rows = Search.query('FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name)', AccessLevel.USER_MODE);
System.assertEquals(1, rows[0].size());
Search.SearchResults found = Search.find('FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name)', AccessLevel.SYSTEM_MODE);
System.assertEquals(1, found.get('Account').size());
Search.SuggestionOption option = new Search.SuggestionOption();
Search.SuggestionResults suggestions = Search.suggest('Nook', 'Account', option);
System.assertEquals(0, suggestions.getSuggestionResults().size());
Search.SuggestionResults userSuggestions = Search.suggest('Nook', 'Account', option, AccessLevel.USER_MODE);
System.assertEquals(false, userSuggestions.hasMoreResults());
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
BcpProvisionService.enableC2C();
DistributedLedgerService.enableC2C();
System.assertEquals(0, BusRuleDtMig.DecisionTableMigrationService.migrateDecisionTables(new List<String>{'dt-1'}, 'local').size());
System.assertEquals(0, BusinessRule.CalculationMatrixMigrationService.migrate(new List<String>{'cm-1'}, 'ns').size());
System.assertEquals(0, BusinessRule.CalculationMatrixMigrationService.migrate('cm-1', 'ns').size());
System.assertEquals(0, BusinessRule.CalculationProcedureMigrationService.migrate(new List<String>{'cp-1'}, 'ns').size());
System.assertEquals(0, BusinessRule.CalculationProcedureMigrationService.migrate('cp-1', 'ns').size());
System.assertEquals(0, BusinessRule.DecisionMatrixRowMigratorService.migrate('dmv-1').size());
ApptBooking.WaitlistController waitlist = new ApptBooking.WaitlistController();
System.assertEquals(0, waitlist.call('local', new Map<String,Object>()).size());
System.assertEquals(0, waitlist.invokeMethod('local', new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).size());
System.assertEquals(false, data_mask.DataMaskIntegrationUtil.isCoreAllowed());
System.assertEquals(false, data_mask.DataMaskIntegrationUtil.isLibraryInUse('lib-local'));
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

func TestExecVisualEditorDataRowConstructorPrecedesRegisteredStubClass(t *testing.T) {
	program, err := CompileAnonymous(`
VisualEditor.DataRow row = new VisualEditor.DataRow('None', '');
System.assertEquals('', row.getValue());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VisualEditor.DataRow"}); err != nil {
		t.Fatal(err)
	}
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
System.assert(created.startsWith('a6S'));
SubMgmt.Test.modify(created, new Map<String,Object>{ 'Name' => 'changed' });
SubMgmt.Test.remove(created);
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

func TestExecQuickActionPerformReturnsCapturedLocalResult(t *testing.T) {
	program, err := CompileAnonymous(`
QuickAction.QuickActionRequest request = new QuickAction.QuickActionRequest();
request.setQuickActionName('Account.NewTask');
request.setContextId('001000000000001');
QuickAction.QuickActionResult result = QuickAction.performQuickAction(request);
System.assert(result.isSuccess());
System.assert(!result.isCreated());
System.assertEquals('001000000000001', String.valueOf(result.getContextId()));
System.assertEquals(0, result.getErrors().size());
System.assertEquals(0, result.getIds().size());
List<QuickAction.QuickActionResult> results =
	QuickAction.performQuickActions(new List<QuickAction.QuickActionRequest>{ request }, true);
System.assertEquals(1, results.size());
System.assert(results.get(0).isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
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

func TestExecMetadataDTOConstructorsDefaultsAndMaps(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
System.assertEquals(0, container.getMetadata().size());

Metadata.CustomObject objectDef = new Metadata.CustomObject();
System.assertEquals(false, objectDef.enableActivities);
System.assertEquals(false, objectDef.enableReports);
System.assertEquals(null, objectDef.fullName);
Map<String,Object> objectValues = objectDef.getAsMap();
System.assert(objectValues.containsKey('enableActivities'));
System.assertEquals(false, (Boolean)objectValues.get('enableActivities'));

objectDef.fullName = 'Invoice__c';
container.addMetadata(objectDef);
System.assertEquals(1, container.getMetadata().size());
System.assertEquals(false, container.removeMetadataByFullName('Missing__c'));
System.assertEquals(true, container.removeMetadataByFullName('Invoice__c'));
System.assertEquals(0, container.getMetadata().size());

Metadata.CustomMetadata item = new Metadata.CustomMetadata();
System.assertEquals(false, item.protected_x);
System.assertEquals(0, item.values.size());
Metadata.CustomMetadataValue value = new Metadata.CustomMetadataValue();
System.assertEquals(null, value.field);
System.assertEquals(null, value.value);
item.fullName = 'Feature.Default';
container.addMetadata(item);
System.assertEquals(true, container.removeMetadata(item));

Metadata.CustomField field = new Metadata.CustomField();
System.assertEquals(false, field.required);
System.assertEquals(false, field.unique);
System.assertEquals(false, field.externalId);

Metadata.DeployResult result = new Metadata.DeployResult();
System.assert(result.done);
System.assert(result.success);
System.assertEquals(0, result.numberComponentsTotal);
System.assertEquals(0, result.numberComponentErrors);
System.assertEquals(0, result.details.componentFailures.size());
System.assertEquals(0, result.details.componentSuccesses.size());

Metadata.DeployMessage message = new Metadata.DeployMessage();
System.assertEquals(false, message.success);
System.assertEquals(0, message.lineNumber);
System.assertEquals(null, message.problem);
Map<String,Object> messageValues = message.getAsMap();
System.assert(messageValues.keySet().contains('success'));
System.assertEquals(false, (Boolean)messageValues.get('success'));

Metadata.AsyncResult asyncResult = new Metadata.AsyncResult();
System.assert(asyncResult.done);
System.assertEquals('Succeeded', asyncResult.state);

Metadata.Metadata base = new Metadata.Metadata();
System.assertEquals(null, base.fullName);
System.assert(base.getAsMap().containsKey('fullName'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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

func TestExecMetadataDeploymentUnsupportedBoundaries(t *testing.T) {
	tests := []struct {
		name string
		apex string
		org  bool
		want string
	}{
		{
			name: "custom metadata deploy without org storage",
			apex: `
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomMetadata item = new Metadata.CustomMetadata();
item.fullName = 'Feature.Default';
container.addMetadata(item);
Metadata.Operations.enqueueDeployment(container, null);
`,
			want: `unsupported call "Metadata.Operations.enqueueDeployment requires org storage for local metadata mutation"`,
		},
		{
			name: "unknown deploy status id",
			apex: `
Metadata.Operations.checkDeployStatus('0Af000000000999', true);
`,
			org:  true,
			want: `unsupported call "Metadata.Operations.checkDeployStatus unknown local deployment 0Af000000000999"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.apex)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if tc.org {
				org := customDataOrg()
				machine.SetOrg(&org)
			}
			_, err = machine.Execute(program)
			if err == nil {
				t.Fatal("expected unsupported metadata boundary")
			}
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
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
Database.SaveResult userMode = EventBus.publishWithAccessLevel(new Account(Name = 'User'), AccessLevel.USER_MODE);
System.assert(userMode.isSuccess());
List<Database.SaveResult> callbackResults = EventBus.publishWithAccessLevel(new List<Account>{new Account(Name = 'Callback')}, null, AccessLevel.SYSTEM_MODE);
System.assertEquals(1, callbackResults.size());
System.assert(callbackResults.get(0).isSuccess());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEventBusPublishWithAccessLevelRejectsNonAccessLevel(t *testing.T) {
	machine := New(nil)
	record := Object("Account")
	record.Fields["Name"] = String("Acme")

	_, err := machine.eventBusPublishWithAccessLevel([]Value{record, String("USER_MODE")}, &Result{})
	if err == nil || !strings.Contains(err.Error(), "expects AccessLevel") {
		t.Fatalf("expected AccessLevel error, got %v", err)
	}
	_, err = machine.eventBusPublishWithAccessLevel([]Value{record, Null, String("USER_MODE")}, &Result{})
	if err == nil || !strings.Contains(err.Error(), "expects AccessLevel") {
		t.Fatalf("expected AccessLevel error, got %v", err)
	}
}

func TestExecEventBusTriggerContextCurrentContextReturnsLocalObject(t *testing.T) {
	program, err := CompileAnonymous(`
	eventbus.TriggerContext ctx = eventbus.TriggerContext.currentContext();
System.assertNotEquals(null, ctx);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
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
	storage.EnsureDeterministicPlatformData(&org)
	profile := org.Objects["Profile"]
	if profile.Records == nil {
		profile.Records = make(map[storage.ID]storage.Record)
	}
	profile.Records["00e000000000099"] = storage.Record{
		ID:     "00e000000000099",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Partner Login User"),
		},
	}
	org.Objects["Profile"] = profile
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
	profile := org.Objects["Profile"]
	if profile.Records == nil {
		profile.Records = make(map[storage.ID]storage.Record)
	}
	profile.Records["00e000000000099"] = storage.Record{
		ID:     "00e000000000099",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Partner Login User"),
		},
	}
	org.Objects["Profile"] = profile
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

func TestExecEventBusPublishInTestContextRequiresExplicitDeliver(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
for (Local_Event__e evt : Trigger.new) {
	Account account = [SELECT Id, Website FROM Account WHERE Id = :evt.AccountId__c LIMIT 1];
	account.Website = evt.Url__c;
	update account;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Event Account');
insert account;
Test.startTest();
EventBus.publish(new Local_Event__e(AccountId__c = account.Id, Name__c = 'Trail', Url__c = 'https://example.test'));
Account beforeDeliver = [SELECT Website FROM Account WHERE Id = :account.Id LIMIT 1];
System.assertEquals(null, beforeDeliver.Website);
Test.getEventBus().deliver();
Account afterDeliver = [SELECT Website FROM Account WHERE Id = :account.Id LIMIT 1];
System.assertEquals('https://example.test', afterDeliver.Website);
Test.stopTest();
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
	machine.EnableTestContext()
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

func TestExecEventBusPublishBeforeStartTestRequiresExplicitDeliver(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`insert new Account(Name = 'pre-start event delivered');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
EventBus.publish(new Local_Event__e(Name__c = 'Trail'));
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'pre-start event delivered'].size());
Test.startTest();
Test.stopTest();
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'pre-start event delivered'].size());
Test.getEventBus().deliver();
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'pre-start event delivered'].size());
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
				"Name__c": {APIName: "Name__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
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

func TestExecStopTestDefersQueueableEnqueuedByPlatformEventTrigger(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`System.enqueueJob(new QueueWorker());`)
	if err != nil {
		t.Fatal(err)
	}
	queueProgram, err := CompileAnonymous(`insert new Account(Name = 'platform event queueable ran');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
EventBus.publish(new Local_Event__e(Name__c = 'Trail'));
Test.stopTest();
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'platform event queueable ran'].size());
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
				"Name__c": {APIName: "Name__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterTrigger(Trigger{
		Name:      "LocalEventTrigger",
		Object:    "Local_Event__e",
		Timing:    triggerTimingAfter,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "QueueWorker",
		Methods: map[string]Method{
			"execute": {Name: "QueueWorker.execute", ClassName: "QueueWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStopTestPlatformEventDeliveryUsesFreshStatics(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
if (StaticBox.Value == null) {
    insert new Account(Name = 'fresh statics');
} else {
    insert new Account(Name = 'stale statics');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
StaticBox.Value = 'parent test value';
Test.startTest();
EventBus.publish(new Local_Event__e(Name__c = 'Trail'));
Test.stopTest();
System.assertEquals('parent test value', StaticBox.Value);
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'fresh statics'].size());
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'stale statics'].size());
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
				"Name__c": {APIName: "Name__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "StaticBox",
		StaticFields: map[string]Field{
			"Value": {Name: "Value", Type: "String", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
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

func TestExecExplicitEventBusDeliverAllowsStopTestToDrainQueueable(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`System.enqueueJob(new QueueWorker());`)
	if err != nil {
		t.Fatal(err)
	}
	queueProgram, err := CompileAnonymous(`insert new Account(Name = 'explicit platform event queueable ran');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
EventBus.publish(new Local_Event__e(Name__c = 'Trail'));
Test.getEventBus().deliver();
Test.stopTest();
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'explicit platform event queueable ran'].size());
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
				"Name__c": {APIName: "Name__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterTrigger(Trigger{
		Name:      "LocalEventTrigger",
		Object:    "Local_Event__e",
		Timing:    triggerTimingAfter,
		Operation: "insert",
		Program:   triggerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "QueueWorker",
		Methods: map[string]Method{
			"execute": {Name: "QueueWorker.execute", ClassName: "QueueWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStopTestPlatformEventDeliveryUsesAutomatedProcessUser(t *testing.T) {
	triggerProgram, err := CompileAnonymous(`
System.assertEquals('AutomatedProcess', UserInfo.getUserType());
insert new Account(Name = UserInfo.getUserType());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'Partner Login User' LIMIT 1];
System.runAs(new User(Id = '005-community-user', ProfileId = p.Id, Username = 'community@example.test')) {
	Test.startTest();
	EventBus.publish(new Local_Event__e(Name__c = 'Trail'));
	Test.stopTest();
}
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'AutomatedProcess'].size());
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'PowerCustomerSuccess'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	profile := org.Objects["Profile"]
	if profile.Records == nil {
		profile.Records = make(map[storage.ID]storage.Record)
	}
	profile.Records["00e000000000099"] = storage.Record{
		ID:     "00e000000000099",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Partner Login User"),
		},
	}
	org.Objects["Profile"] = profile
	org.Objects["Local_Event__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Local_Event__e",
			KeyPrefix: "e00",
			Fields: map[string]storage.Field{
				"Name__c": {APIName: "Name__c", Type: storage.FieldString, Required: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterTrigger(Trigger{
		Name:      "LocalEventUserTrigger",
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
System.assertEquals('00DLOCAL00000001', settings.id);
System.assertEquals('Local Organization', settings.name);
System.assertEquals('en_US', settings.defaultLanguage);
System.assertEquals('en_US', settings.defaultLocale);
System.assertEquals('UTC', settings.defaultTimeZone.id);
System.assertEquals('UTC', settings.defaultTimeZone.name);
System.assertEquals('UTC', settings.defaultTimeZone.displayName);
System.assertEquals(0, settings.defaultTimeZone.offset);
System.assertEquals(0, settings.defaultTimeZone.gmtOffset);
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

func TestExecConnectApiChatterUsersFollowingsReturnsLocalReadPage(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.FollowingPage page = ConnectApi.ChatterUsers.getFollowings(null, UserInfo.getUserId());
System.assertNotEquals(null, page);
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
}

func TestExecConnectApiNamedCredentialsPrimaryFlow(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.ExternalCredentialInput externalInput = new ConnectApi.ExternalCredentialInput();
externalInput.developerName = 'googleBooksAPIApex';
externalInput.principals = new List<ConnectApi.ExternalCredentialPrincipalInput>();
ConnectApi.ExternalCredential external = ConnectApi.NamedCredentials.createExternalCredential(externalInput);
System.assertNotEquals(null, external);

ConnectApi.NamedCredentialInput namedInput = new ConnectApi.NamedCredentialInput();
namedInput.developerName = 'googleBooks';
namedInput.calloutUrl = 'https://www.googleapis.com/books/v1';
namedInput.externalCredentials = new List<ConnectApi.ExternalCredentialInput>{ externalInput };
ConnectApi.NamedCredential named = ConnectApi.NamedCredentials.createNamedCredential(namedInput);
System.assertNotEquals(null, named);

ConnectApi.ExternalCredential fetched = ConnectApi.NamedCredentials.getExternalCredential('googleBooksAPIApex');
System.assertNotEquals(null, fetched);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConnectApiPrimaryUsageFallbackThrowsConnectApiException(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "named credentials unsupported method",
			src:  `ConnectApi.NamedCredentials.deleteNamedCredential('devName');`,
		},
		{
			name: "user profiles unsupported method",
			src:  `ConnectApi.UserProfiles.getBannerPhoto(Network.getNetworkId(), UserInfo.getUserId());`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = New(nil).Execute(program)
			if err == nil || !strings.Contains(err.Error(), "ConnectApi.ConnectApiException") {
				t.Fatalf("expected ConnectApi.ConnectApiException, got %v", err)
			}
		})
	}
}

func TestExecConnectApiNextBestActionReadDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.Recommendation recommendation = ConnectApi.NextBestAction.getRecommendation('rec');
ConnectApi.RecommendationReaction reaction = ConnectApi.NextBestAction.getRecommendationReaction('rec');
ConnectApi.RecommendationReactions reactions = ConnectApi.NextBestAction.getRecommendationReactions('user', 'target', 'type', 'action', 0, 10);
System.assertNotEquals(null, recommendation);
System.assertNotEquals(null, reaction);
System.assertNotEquals(null, reactions);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestConnectApiReadOnlyHarnessDispatchIsCaseInsensitive(t *testing.T) {
	if !connectAPIReadOnlyHarnessType("connectapi.nextbestaction") {
		t.Fatalf("lower-case ConnectApi type was not accepted")
	}
	if !connectAPIReadOnlyHarnessMethodAllowed("connectapi.nextbestaction", "getrecommendation") {
		t.Fatalf("lower-case ConnectApi method was not accepted")
	}
	if connectAPIReadOnlyHarnessMethodAllowed("connectapi.nextbestaction", "executeStrategy") {
		t.Fatalf("executeStrategy should remain unsupported")
	}
}

func TestExecPlatformCachePartitions(t *testing.T) {
	program, err := CompileAnonymous(`
Cache.OrgPartition orgCache = Cache.Org.getPartition('local');
System.assertEquals(null, orgCache.get('missing'));
orgCache.put('name', 'Acme');
orgCache.put('namespaceVisible', 'Scoped', Cache.Visibility.NAMESPACE);
orgCache.put('visible', 'Trail', 60, Cache.Visibility.ALL, false);
System.assert(orgCache.contains('name'));
System.assertEquals('Acme', (String) orgCache.get('name'));
System.assertEquals('Scoped', (String) orgCache.get('namespaceVisible'));
System.assertEquals('Trail', (String) orgCache.get('visible'));
Set<String> orgKeys = orgCache.getKeys();
System.assertEquals(3, orgKeys.size());
System.assert(orgKeys.contains('name'));
System.assert(orgKeys.contains('namespaceVisible'));
System.assert(orgKeys.contains('visible'));
System.assertEquals(3, orgCache.getNumKeys());
System.assertEquals('Acme', (String) orgCache.remove('name'));
System.assert(!orgCache.contains('name'));
System.assertEquals(2, orgCache.getNumKeys());

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
Cache.Org.put('visible-default', 'org-visible', Cache.Visibility.ALL);
System.assert(Cache.Org.contains('defaulted'));
System.assertEquals('org-default', (String) Cache.Org.get('defaulted'));
System.assertEquals('org-visible', (String) Cache.Org.get('visible-default'));
System.assert(Cache.Org.getKeys().contains('defaulted'));
System.assertEquals(2, Cache.Org.getNumKeys());
System.assertEquals('org-default', (String) Cache.Org.getPartition('default').get('defaulted'));
System.assertEquals('org-default', (String) Cache.Org.remove('defaulted'));
System.assert(!Cache.Org.contains('defaulted'));
System.assertEquals('org-visible', (String) Cache.Org.remove('visible-default'));

Cache.SecondaryKeyApi secondary = Cache.SecondaryKeyApi.get('localFeature');
secondary.putImmediate('alpha', 'A', 'group-1');
secondary.putImmediate('beta', 'B', 'group-1');
secondary.putImmediate('gamma', 'C', 'group-2');
System.assertEquals(2, secondary.scanForCount('group-1', 'group-1'));
Cache.ScanResult firstScan = secondary.scanForKeyValues('group-1', 'group-2', 2);
System.assertEquals(false, firstScan.isDone);
System.assertEquals(2, firstScan.result.size());
System.assertEquals('A', (String) firstScan.result.get('alpha'));
Cache.ScanResult secondScan = secondary.scanForMoreKeyValues(firstScan.scanLocator, 10);
System.assertEquals(true, secondScan.isDone);
System.assertEquals(1, secondScan.result.size());
System.assertEquals('C', (String) secondScan.result.get('gamma'));
System.assertEquals(true, secondary.remove('beta'));
System.assertEquals(false, secondary.remove('missing'));
System.assertEquals(2, secondary.scanForCount('', ''));
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

func TestExecPlatformCacheBuilderAcceptsGeneratedLowercaseInterface(t *testing.T) {
	loadProgram, err := CompileAnonymous(`return 'loaded:' + key;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('loaded:shape', (String) Cache.Org.get(CacheLoader.class, 'shape'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "CacheLoader",
		Interfaces: []string{"cache.CacheBuilder"},
		Methods: map[string]Method{
			"doLoad": {
				Name:       "CacheLoader.doLoad",
				ClassName:  "CacheLoader",
				ReturnType: "Object",
				Params:     []Param{{Name: "key", Type: "String"}},
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

func TestExecPlatformHelperTailSafeDefaults(t *testing.T) {
	cases := []string{`
CartExtension.PlaceOrderResponse placeOrder =
	new CartExtension.CheckoutPlaceOrder().validate(
		new CartExtension.PlaceOrderRequest(CartExtension.CartTestUtil.createCart()),
		new List<String>()
	);
System.assertNotEquals(null, placeOrder);
`, `
System.assertEquals(false, YubiAuthForAloha.validateYubiKeyLogin('user', 'password'));
`, `
ConnectApi.LiteralJson waveResult = wave.QueryBuilder.load('dataset', 'v1').execute('q');
System.assertNotEquals(null, waveResult);
List<Object> rows = (List<Object>) waveResult.json;
System.assertEquals(0, rows.size());
`}
	for _, source := range cases {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err != nil {
			t.Fatalf("%s\n%v", source, err)
		}
	}
}

func TestExecCartExtensionUnsupportedFamilyExplicitDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
CartExtension.CartDeliveryGroup deliveryGroup = new CartExtension.CartDeliveryGroup();
System.assertEquals(false, deliveryGroup.getIsDefault());
System.assertEquals(false, deliveryGroup.getIsGift());
System.assertEquals('Shipment 1', deliveryGroup.getName());

CartExtension.OrderGraph graph = new CartExtension.OrderGraph();
Order orderRecord = graph.getOrder();
System.assertNotEquals(null, orderRecord);
System.assertEquals('@{ref_Order_1.id}', (String)orderRecord.get('id'));
System.assertEquals(0, graph.getOrderAdjustmentGroups().size());
System.assertEquals(0, graph.getOrderDeliveryGroups().size());
System.assertEquals(0, graph.getOrderDeliveryMethods().size());
System.assertEquals(0, graph.getOrderItemAdjustmentLineItems().size());
System.assertEquals(0, graph.getOrderItems().size());
System.assertEquals(0, graph.getOrderItemTaxLineItems().size());

CartExtension.PlaceOrderResponse response = CartExtension.PlaceOrderResponse.success();
System.assertNotEquals(null, response);
System.assertEquals('Success', (String)response.status);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformHelperTailUnsupportedFences(t *testing.T) {
	cases := []string{
		`System.changeOwnPassword('old', 'new', 'new');`,
		`ConnectApi.Payments.authorize(null);`,
		`new ApexPages.Action('{!save}').invoke();`,
		`functions.Function.get('fn').invoke(null);`,
	}
	for _, source := range cases {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("%s error = %v, want unsupported", source, err)
		}
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
UserManagement.deregisterVerificationMethod(UserInfo.getUserId(), Auth.VerificationMethod.EMAIL);
System.assertEquals('local-passwordless-login', UserManagement.initPasswordlessLogin(UserInfo.getUserId(), Auth.VerificationMethod.EMAIL));
System.assertEquals('local-verification', UserManagement.initRegisterVerificationMethod(Auth.VerificationMethod.EMAIL));
System.assertEquals('local-verification', UserManagement.initVerificationMethod(Auth.VerificationMethod.EMAIL));
System.assertEquals('local-verification', UserManagement.initVerificationMethod(Auth.VerificationMethod.EMAIL, 'login', new Map<String,String>()));
UserManagement.obfuscateUser(UserInfo.getUserId());
UserManagement.obfuscateUser(UserInfo.getUserId(), 'masked@example.invalid');
System.assertEquals('/register', UserManagement.registerVerificationMethod(Auth.VerificationMethod.EMAIL, '/register').getUrl());
System.assertEquals(false, UserManagement.sendAsyncEmailConfirmation(UserInfo.getUserId(), '00X000000000001', Network.getNetworkId(), '/start'));
Auth.VerificationResult result = UserManagement.verifySelfRegistration(Auth.VerificationMethod.EMAIL, 'local-self-registration', '12345', '/welcome');
System.assert(result.success);
System.assertEquals('/welcome', result.redirect.getUrl());
Auth.VerificationResult passwordless = UserManagement.verifyPasswordlessLogin(UserInfo.getUserId(), Auth.VerificationMethod.EMAIL, 'ada@example.invalid', '12345', '/passwordless');
System.assert(passwordless.success);
System.assertEquals('/passwordless', passwordless.redirect.getUrl());
System.assertEquals('local-verification', UserManagement.verifyRegisterVerificationMethod('12345', Auth.VerificationMethod.EMAIL));
System.assert(UserManagement.verifyVerificationMethod('ada@example.invalid', '12345', Auth.VerificationMethod.EMAIL).success);
System.assertEquals(true, Auth.AuthToken.revokeAccess('provider', 'user', 'token'));
System.assert(Auth.SessionManagement.getCurrentSession().get('SessionId').contains('session'));
Auth.AuthConfiguration config = new Auth.AuthConfiguration('https://local.example', '/start');
System.assertEquals(0, config.getAuthProviders().size());
List<AuthProvider> providers = config.getAuthProviders();
System.assertEquals(0, providers.size());
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

func TestAuthConfigurationCommunityUsingSiteAsContainerDefaultsFalse(t *testing.T) {
	program, err := CompileAnonymous(`
auth.AuthConfiguration config = new auth.AuthConfiguration('https://local.example', '');
System.assertEquals(false, config.isCommunityUsingSiteAsContainer());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestAuthConfigurationLoginDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
Auth.AuthConfiguration config = new Auth.AuthConfiguration(Network.getNetworkId(), '');
System.assertEquals(true, config.getUsernamePasswordEnabled());
System.assertEquals(false, config.getSelfRegistrationEnabled());
System.assertEquals(null, config.getSelfRegistrationUrl());
System.assertEquals('/ForgotPassword', config.getForgotPasswordUrl());
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

func TestPageReferenceNullConstructorThrowsCatchableException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
  new PageReference(null);
  System.assert(false, 'expected exception');
} catch (NullPointerException e) {
  System.assertEquals('Argument 1 cannot be null', e.getMessage());
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

func TestPageReferenceApexPageRecordConstructorUsesPageName(t *testing.T) {
	program, err := CompileAnonymous(`
SObject pageRecord = ApexPage.SObjectType.newSObject();
pageRecord.put('Name', 'LocalPage');
PageReference page = new PageReference(pageRecord);
System.assertEquals('/apex/LocalPage', page.getUrl());
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

func TestPageReferenceAnchorAccessorsUpdateURL(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Local?id=1#top');
System.assertEquals('top', page.getAnchor());
PageReference returned = page.setAnchor('bottom');
System.assertEquals(page, returned);
System.assertEquals('bottom', page.getAnchor());
System.assertEquals('/apex/Local?id=1#bottom', page.getUrl());
System.assertEquals(null, new PageReference('/apex/Local').getAnchor());
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

func TestPageReferenceRedirectAccessorReturnsReceiver(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Local');
System.assertEquals(false, page.getRedirect());
PageReference returned = page.setRedirect(true);
System.assertEquals(page, returned);
System.assertEquals(true, page.getRedirect());
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

func TestPageReferenceRedirectCodeAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Local');
System.assertEquals(0, page.getRedirectCode());
PageReference returned = page.setRedirectCode(301);
System.assertEquals(page, returned);
System.assertEquals(301, page.getRedirectCode());
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

func TestGeneratedAuthConfigurationConstructorAcceptsCommunityUrlAndStartUrl(t *testing.T) {
	machine := New(nil)
	value, handled, err := machine.constructGeneratedPlatformValue("Auth.AuthConfiguration", []Value{String("https://local.example"), String("")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("generated Auth.AuthConfiguration constructor was not handled")
	}
	result, _, _, handled, err := machine.callPlatformObjectMember(value, "isCommunityUsingSiteAsContainer", nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("isCommunityUsingSiteAsContainer was not handled")
	}
	if result.Kind != ValueBool || result.Bool {
		t.Fatalf("isCommunityUsingSiteAsContainer = %#v, want false", result)
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
PageReference defaultPage = ApexPages.currentPage();
System.assertEquals(null, defaultPage);
System.assertEquals(null, System.currentPageReference());
PageReference before = new PageReference('/apex/Before');
Test.setCurrentPage(before);
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

func TestExecImplicitCurrentPageMaterializesOnMemberAccess(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, ApexPages.currentPage());
System.assertEquals(null, System.currentPageReference());
System.assertEquals('', ApexPages.currentPage().getUrl());
System.assertEquals('', System.currentPageReference().getUrl());
Test.startTest();
System.assertNotEquals(null, ApexPages.currentPage());
System.assertEquals('', ApexPages.currentPage().getUrl());
Test.stopTest();
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

func TestExecTestStartTestDoesNotInitializeDefaultCurrentPage(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, ApexPages.currentPage());
Test.startTest();
System.assertEquals(null, ApexPages.currentPage());
Test.stopTest();
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

func TestExecFutureContextSurvivesTriggerDML(t *testing.T) {
	futureProgram, err := CompileAnonymous(`
System.assertEquals(true, System.isFuture());
insert new Contact(LastName = 'Future');
	`)
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymous(`
for (SObject record : Trigger.new) {
	if (!System.isFuture()) {
		record.addError('expected future context');
	}
}
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
FutureWorker.mark();
Test.stopTest();
System.assertEquals(1, [SELECT COUNT() FROM Contact WHERE LastName = 'Future']);
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
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "FutureWorker",
		Methods: map[string]Method{
			"mark": {Name: "FutureWorker.mark", ClassName: "FutureWorker", ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, Program: futureProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "ContactBeforeInsert", Object: "Contact", Timing: triggerTimingBefore, Operation: "insert", Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPrivateFutureCalledFromSameClassIsEnqueued(t *testing.T) {
	callProgram, err := CompileAnonymous(`mark();`)
	if err != nil {
		t.Fatal(err)
	}
	futureProgram, err := CompileAnonymous(`
System.assertEquals(true, System.isFuture());
insert new Contact(LastName = 'Private Future');
	`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
FutureWorker.callMark();
System.assertEquals(0, [SELECT COUNT() FROM Contact WHERE LastName = 'Private Future']);
Test.stopTest();
System.assertEquals(1, [SELECT COUNT() FROM Contact WHERE LastName = 'Private Future']);
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
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "FutureWorker",
		Methods: map[string]Method{
			"callMark": {Name: "FutureWorker.callMark", ClassName: "FutureWorker", ReturnType: "void", IsStatic: true, Program: callProgram},
			"mark":     {Name: "FutureWorker.mark", ClassName: "FutureWorker", ReturnType: "void", IsStatic: true, Access: "private", Modifiers: []string{"future", "private"}, Program: futureProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestStartTestAdvancesNowPastSetupTime(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime before = Datetime.now();
Test.startTest();
Datetime afterStart = Datetime.now();
Test.stopTest();
System.assert(afterStart > before);
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

func TestExecDatetimeNowAdvancesBetweenCalls(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime before = Datetime.now();
Datetime after = Datetime.now();
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

func TestExecImplicitCurrentPageMaterializesForPageReferenceArgument(t *testing.T) {
	program, err := CompileAnonymous(`
PageReferenceProbe.accept(ApexPages.currentPage());
System.assertEquals('?seen=yes', ApexPages.currentPage().getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	acceptProgram, err := CompileAnonymous(`
System.assertNotEquals(null, page);
page.getParameters().put('seen', 'yes');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "PageReferenceProbe",
		Methods: map[string]Method{
			"accept": {Name: "PageReferenceProbe.accept", ClassName: "PageReferenceProbe", IsStatic: true, Params: []Param{{Name: "page", Type: "PageReference"}}, Program: acceptProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestAssertEqualsTreatsCurrentNamespaceApexStubMessagesAsEquivalent(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(
    'Wanted but not invoked: fflib_MyList__sfdc_ApexStub.add(String).',
    'Wanted but not invoked: NU.fflib_MyList__sfdc_ApexStub.add(String).'
);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "NU"
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFeatureManagementUsesExecutionUserPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(FeatureManagement.checkPermission('CanRunLocal'));
System.assert(!FeatureManagement.checkPermission('OtherPermission'));
FeatureManagement.changeProtection('LocalFeature', 'CustomPermission', 'Protected');
System.assertEquals(null, Packaging.getCurrentPackageId());
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

func TestExecFeatureManagementPackageValuesRoundTrip(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, System.FeatureManagement.checkPackageBooleanValue('LocalBooleanFeature'));
System.FeatureManagement.setPackageBooleanValue('LocalBooleanFeature', true);
System.assertEquals(true, System.FeatureManagement.checkPackageBooleanValue('LocalBooleanFeature'));
System.FeatureManagement.setPackageIntegerValue('LocalIntegerFeature', 7);
System.assertEquals(7, System.FeatureManagement.checkPackageIntegerValue('LocalIntegerFeature'));
System.FeatureManagement.setPackageDateValue('LocalDateFeature', Date.newInstance(2026, 6, 3));
System.assertEquals(Date.newInstance(2026, 6, 3), System.FeatureManagement.checkPackageDateValue('LocalDateFeature'));
System.assertEquals(null, System.FeatureManagement.checkPackageDateValue('MissingLocalDateFeature'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformDescribeConstructorsAreUnsupported(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{source: `new FeatureManagement();`, want: `unsupported call "FeatureManagement constructor"`},
		{source: `new Schema.DescribeFieldResult();`, want: `unsupported call "Schema.DescribeFieldResult constructor"`},
		{source: `new Schema.SObjectType();`, want: `unsupported call "Schema.SObjectType constructor"`},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want UnsupportedFeature %q", err, tc.want)
			}
		})
	}
}

func TestExecFeatureManagementUsesAssignedPermissionSetCustomPermissions(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-user-a')) {
    System.assert(FeatureManagement.checkPermission('ManageFeature'));
    System.assert(!FeatureManagement.checkPermission('OtherPermission'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "PermissionSet")
	storage.EnsureStandardObject(&org, "PermissionSetAssignment")
	org.Objects["PermissionSet"].Records["0PS000000000101"] = storage.Record{
		ID:     "0PS000000000101",
		Object: "PermissionSet",
		Fields: map[string]storage.Value{
			"Name":              storage.StringValue("LocalPermissions"),
			"CustomPermissions": storage.ListValue(storage.StringValue("ManageFeature")),
		},
	}
	org.Objects["PermissionSetAssignment"].Records["0Pa000000000101"] = storage.Record{
		ID:     "0Pa000000000101",
		Object: "PermissionSetAssignment",
		Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue("005-user-a"),
			"PermissionSetId": storage.IDValue("0PS000000000101"),
		},
	}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatacloudFindDuplicatesLocalEmptyResults(t *testing.T) {
	program, err := CompileAnonymous(`
List<Datacloud.FindDuplicatesResult> byRecord = Datacloud.FindDuplicates.findDuplicates(new List<SObject>{new Account(Name = 'Acme')});
System.assertEquals(1, byRecord.size());
System.assert(byRecord.get(0).isSuccess());
System.assertEquals(0, byRecord.get(0).getDuplicateResults().size());
System.assertEquals(0, byRecord.get(0).getErrors().size());
List<Datacloud.FindDuplicatesResult> byId = Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{'001000000000001AAA'});
System.assertEquals(1, byId.size());
System.assert(byId.get(0).isSuccess());
System.assertEquals(0, byId.get(0).getDuplicateResults().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNLPPredictionsLocalDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
NLPPredictions.FAQPredictionInput input = new NLPPredictions.FAQPredictionInput('How do I reset my password?', 'local-model');
NLPPredictions.FAQPredictionResult result = NLPPredictions.FAQPrediction.predict(input);
System.assertEquals(0, result.getMatches().size());
NLPPredictions.PredictionHandler handler = new NLPPredictions.PredictionHandler();
handler.handlePredictionRequest(new NLPPredictions.PredictionRequestContextImpl());
handler.handlePredictionResponse(new NLPPredictions.PredictionResponseContextImpl());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecBusinessHoursMetadataBackedMethodsAreUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
Id businessHoursId = '01m000000000001AAA';
Datetime start = Datetime.newInstanceGmt(2026, 5, 15, 10, 0, 0);
BusinessHours.add(businessHoursId, start, 1250);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), `unsupported call "BusinessHours.add local business hours metadata"`) {
		t.Fatalf("BusinessHours.add error = %v", err)
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

func TestExecEmailMessagesThreadingHelpers(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001000000000001AAA';
String token = EmailMessages.getFormattedThreadingToken(accountId);
System.assert(token.contains('001000000000001AAA'));
System.assertEquals(accountId, EmailMessages.getRecordIdFromEmail('reply ' + token, null, null));
System.assertEquals(accountId, EmailMessages.getRecordIdFromEmail('reply', 'body ' + token, null));
System.assertEquals(accountId, EmailMessages.getRecordIdFromEmail('reply', null, '<span>' + token + '</span>'));
System.assertEquals(null, EmailMessages.getRecordIdFromEmail('reply', 'missing token', null));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSafePlatformServiceHarnesses(t *testing.T) {
	program, err := CompileAnonymous(`
List<Datacloud.FindDuplicatesResult> duplicateRows =
	Datacloud.FindDuplicates.findDuplicates(new List<SObject>{ new Account(Name = 'Acme') });
System.assertEquals(1, duplicateRows.size());
System.assertEquals(1, Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{ '001000000000001AAA' }).size());

FeatureManagement.changeProtection('pkg', 'Feature', 'Protected');
System.assertEquals(null, Packaging.getCurrentPackageId());
System.assertEquals(0, SupportPredictiveService.findSimilarCases('500000000000001AAA').size());

String articleId = 'ka0000000000001';
KbManagement.PublishingService.publishArticle(articleId, true);
KbManagement.PublishingService.archiveOnlineArticle(articleId, Datetime.now());
KbManagement.PublishingService.scheduleForPublication(articleId, Datetime.now());
KbManagement.PublishingService.cancelScheduledPublicationOfArticle(articleId);
KbManagement.PublishingService.cancelScheduledArchivingOfArticle(articleId);
KbManagement.PublishingService.completeTranslation(articleId);
KbManagement.PublishingService.setTranslationToIncomplete(articleId);
System.assertEquals(articleId, KbManagement.PublishingService.editOnlineArticle(articleId, false));
System.assertEquals(articleId, KbManagement.PublishingService.editArchivedArticle(articleId));
System.assertEquals(articleId, KbManagement.PublishingService.restoreOldVersion(articleId, 1));
System.assertEquals(articleId, KbManagement.PublishingService.submitForTranslation(articleId, 'fr', 'French', Datetime.now()));
System.assertEquals(articleId, KbManagement.PublishingService.editPublishedTranslation(articleId, 'fr', false));
KbManagement.PublishingService.assignDraftArticleTask(articleId, '005000000000001AAA', 'Review', Datetime.now(), true);
KbManagement.PublishingService.assignDraftTranslationTask(articleId, 'fr', 'Review', Datetime.now(), true);

Map<String,Object> retrieved = RemoteObjectController.retrieve('Account', new List<String>{'Id'}, new Map<String,Object>());
System.assertEquals(true, retrieved.get('success'));
System.assertEquals(true, RemoteObjectController.create('Account', new Map<String,Object>{ 'Name' => 'Acme' }).get('success'));
System.assertEquals(true, RemoteObjectController.updat('Account', new Map<String,Object>{ 'Id' => '001000000000001AAA' }).get('success'));
System.assertEquals(true, RemoteObjectController.update('Account', new List<String>{ '001000000000001AAA' }, new Map<String,Object>{ 'Name' => 'Acme' }).get('success'));
System.assertEquals(true, RemoteObjectController.del('Account', new List<String>{ '001000000000001AAA' }).get('success'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIndustryControllerLocalDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(0, healthcloudext.AppointmentBookingSelfService.findProviders(null).size());
System.assertEquals(0, healthcloudext.AppointmentBookingSelfService.findAssets(null).size());
System.assertEquals(0, healthcloudext.AppointmentBookingSelfService.findAvailableAppointmentSlots('', '', '', new List<Map<String,Object>>(), false).size());
System.assertEquals(0, healthcloudext.AppointmentBookingSelfService.findAvailableAssetSlots('', '', '', new List<Map<String,Object>>(), false, '', '').size());
System.assertEquals(null, healthcloudext.AppointmentBookingSelfService.logSelfServiceInstrumentation(1, 'local'));
System.assertNotEquals(null, healthcloudext.AppointmentBookingSelfService.getGeoLocationCoordinates(null));
System.assertNotEquals(null, healthcloudext.AppointmentBookingSelfService.validateSlotStatusSelfService(null));

System.assertEquals(false, healthcloudext.IntegratedCareManagementApexHelper.checkObjectCreationAccess('Account'));
System.assertEquals(0, healthcloudext.IntegratedCareManagementApexHelper.checkEntity('Account').size());
System.assertEquals('a<br/>b', healthcloudext.IntegratedCareManagementApexHelper.convertMultiLineToHtml('a\nb'));
System.assertEquals(0, healthcloudext.IntegratedCareManagementApexHelper.fetchSuggestedAssessmentsForPatient('001', 'a', 'b', 'c', 'd').size());
System.assertEquals(0, healthcloudext.IntegratedCareManagementApexHelper.getPicklist('Account', 'Name').size());

System.assertEquals(0, LoyaltyManagement.LoyaltyResources.getPointsBalance(new List<LoyaltyManagement.MemberPointBalanceInput>()).size());
System.assertEquals(0, LoyaltyManagement.LoyaltyResources.getTier(new List<LoyaltyManagement.MemberTierInput>()).size());
System.assertEquals(0, LoyaltyManagement.LoyaltyResources.getLoyaltyPromotions(new List<LoyaltyManagement.LoyaltyPromotionInput>()).size());
System.assertEquals(0, LoyaltyManagement.LoyaltyResources.getLoyaltyPromotionBasedOnSalesforceCDP(new List<LoyaltyManagement.CdpBasedLoyaltyPromotionInput>()).size());
System.assertEquals(false, LoyaltyManagement.WidgetVisibility.checkVisibility('member', new Map<String,Object>()));
System.assertEquals(true, ((Map<String,Object>)LoyaltyManagement.WidgetCumulativePromotions.call('load', new Map<String,Object>())).get('success'));
System.assertEquals(true, ((Map<String,Object>)LoyaltyManagement.WidgetMemberBadges.call('load', new Map<String,Object>())).get('success'));
System.assertEquals(true, ((Map<String,Object>)LoyaltyManagement.WidgetReferMember.call('load', new Map<String,Object>())).get('success'));

System.assertEquals(false, industries_docgen.DocGenPermsAndAccessChecksService.hasDocGenOrgPerm('u', 'p'));
System.assertEquals(false, industries_docgen.DocGenPermsAndAccessChecksService.hasDocGenMetadataSetting('u', 'p'));
System.assertEquals(false, industries_docgen.DocGenPermsAndAccessChecksService.isRuntimeUser('u', 'p', 'c'));
System.assertEquals(0, new fschousehold.FSCFinancialAccountService().call('load', new Map<String,Object>()).size());
System.assertEquals(0, new fscwmgen.RecordAlertProvider().getAlertsByWhatId('001').size());
System.assertEquals(0, new fscwmgen.RecordAlertBatchProvider().getAlertsByWhatIdBatch(new List<String>{'001'}).size());
System.assertEquals(0, new healthcloudext.AppointmentBookingInterop().findSlots(null).size());
System.assertNotEquals(null, new healthcloudext.AppointmentBookingInterop().getSlotStatus(null));
System.assertEquals(false, new healthcloudext.IQuotasAndAllocation().validateSlotChain(null));
System.assertEquals(0, healthcloudext.IntegratedCareManagementApexUtil.checkCreateAccess(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).size());
System.assertEquals(false, new healthcloudext.ProviderSearchCardUtil().invokeMethod('load', new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()));
System.assertNotEquals(null, new id_verification.IdentityVerificationExt().search(null));
System.assertEquals(false, new ind_docgen_api.OpenInterface().invokeMethod('load', new Map<String,Object>(), new Map<String,Object>()));
System.assertEquals(null, new ind_docgen_api.EnvelopeStatusScheduler().execute(null));
System.assertEquals(null, new industries_docgen.DocumentTemplate().Call('load', new Map<String,Object>()));
System.assertNotEquals(null, new service_cloud_voice.GroupSetup().listGroups(null));
System.assertNotEquals(null, new service_cloud_voice.PhoneNumberProvider().listPhoneNumbers(null));
System.assertNotEquals(null, new service_cloud_voice.QueueSetup().listQueues(null));
System.assertNotEquals(null, new service_cloud_voice.QueueManager().supportsQueueUserGrouping(null));

inventorypricing.GetInventoryPricing inventory = new inventorypricing.GetInventoryPricing();
inventorypricing.InventoryPricingData data = new inventorypricing.InventoryPricingData();
System.assertNotEquals(null, inventory.processInput(null));
System.assertEquals(data, inventory.getInventory(data));
System.assertEquals(data, inventory.getPricing(data));
System.assertEquals(data, inventory.getInventoryAndPricing(data));
System.assertEquals(data, inventory.handleInventoryPricingServiceException(new Exception('local'), data));
System.assertNotEquals(null, inventory.createResponse(data));

System.assertEquals(0, new CommerceDxSampleapp.CommerceDx_Inventory().calculateInventoryLevel('webstore', 'event'));
new CommerceDxSampleapp.CommerceDx_Inventory().executeNonExistentMethod('webstore');
System.assertEquals(false, new CommerceDxSampleapp.CommerceDx_Inventory_Tutorial_115().isAvailable('webstore', 'product'));
	System.assertEquals('[]', data_mask.DataMaskIntegrationUtil.getJobs());
System.assertEquals('{}', data_mask.DataMaskIntegrationUtil.getRunLogResponse('job-local'));
commerce_inventory.CommerceInventoryService inventoryService = new commerce_inventory.CommerceInventoryService();
commerce_inventory.InventoryLevelsResponse levels = inventoryService.getInventoryLevel(
	new commerce_inventory.InventoryLevelsRequest('LOCATION', new Set<commerce_inventory.InventoryLevelsItemRequest>()));
System.assertNotEquals(null, levels);
System.assertEquals(0, levels.getItemsInventoryLevels().size());
commerce_inventory.InventoryReservation reservation = inventoryService.getReservation('0aB000000000001AAA');
System.assertNotEquals(null, reservation);
System.assertEquals(0, reservation.getDurationInSeconds());
System.assertEquals(0, reservation.getItems().size());
commerce_inventory.InventoryCheckAvailability availability = inventoryService.checkInventory(
	new commerce_inventory.InventoryCheckAvailability(new Set<commerce_inventory.InventoryCheckItemAvailability>()));
System.assertNotEquals(null, availability);
System.assertEquals(0, availability.getInventoryCheckItemAvailability().size());
commercepayments.ClientSidePaymentAdapter paymentAdapter = new commercepayments.ClientSidePaymentAdapter();
System.assertEquals(null, paymentAdapter.getClientComponentName());
System.assertEquals(0, paymentAdapter.getClientConfiguration().size());
commerce_ordermanagement.ProductExpandResponse expandResponse = new commerce_ordermanagement.ProductExpandService().returnReasons(new commerce_ordermanagement.ProductExpandRequest());
System.assertNotEquals(null, expandResponse);
System.assertEquals(null, expandResponse.getSucceed());

System.assertEquals(true, new ime_mrm.EventManagementBudgetApi().getMngEventBudgets(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).get('success'));
System.assertEquals(true, new ime_mrm.EventManagementManagedEventApi().getMngEvent(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).get('success'));
System.assertEquals(true, new ime_mrm.EventManagementParticipantApi().getMngEventParticipantsByEvent(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).get('success'));
System.assertEquals(true, new ime_mrm.EventManagementProductApi().getMngEventProducts(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).get('success'));
System.assertEquals(true, new ime_mrm.EventManagementSubjectApi().getSubjectAssignments(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>()).get('success'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedFamilyUnsupportedTypePrefixTargetsOnlyLowercaseStubFamilies(t *testing.T) {
	for _, typeName := range []string{
		"cartextension.Any",
		"commercepayments.Any",
		"metadata.Any",
		"limits.Any",
		"cache.Any",
		"lxscheduler.Any",
		"messaging.Any",
	} {
		if !generatedFamilyUnsupportedTypePrefix(typeName) {
			t.Fatalf("expected %s to match", typeName)
		}
	}
	for _, typeName := range []string{
		"ConnectApi.ChatterFeeds",
		"Database.QueryLocator",
		"System.HttpRequest",
	} {
		if generatedFamilyUnsupportedTypePrefix(typeName) {
			t.Fatalf("did not expect %s to match", typeName)
		}
	}
}

func TestExecIndustryControllerMutationsStayUnsupported(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "health booking",
			src:  `healthcloudext.AppointmentBookingSelfService.bookSelfServiceAppointment(null);`,
			want: `unsupported call "healthcloudext.AppointmentBookingSelfService.bookSelfServiceAppointment local industry service mutation surface"`,
		},
		{
			name: "loyalty points",
			src:  `LoyaltyManagement.LoyaltyResources.creditPoints(new List<LoyaltyManagement.PointsInput>());`,
			want: `unsupported call "LoyaltyManagement.LoyaltyResources.creditPoints local industry service mutation surface"`,
		},
		{
			name: "event create",
			src:  `new ime_mrm.EventManagementBudgetApi().createMngEventBudget(new Map<String,Object>(), new Map<String,Object>(), new Map<String,Object>());`,
			want: `unsupported call "ime_mrm.EventManagementBudgetApi.createMngEventBudget local industry service mutation surface"`,
		},
		{
			name: "fsc alert mutation",
			src:  `new fscwmgen.RecordAlertProvider().dismissAlert('a');`,
			want: `unsupported call "fscwmgen.RecordAlertProvider.dismissAlert local industry service mutation surface"`,
		},
		{
			name: "health interop booking",
			src:  `new healthcloudext.AppointmentBookingInterop().bookAppointment(null);`,
			want: `unsupported call "healthcloudext.AppointmentBookingInterop.bookAppointment local industry service mutation surface"`,
		},
		{
			name: "service voice create",
			src:  `new service_cloud_voice.GroupSetup().createGroup(null);`,
			want: `unsupported call "service_cloud_voice.GroupSetup.createGroup local industry service mutation surface"`,
		},
		{
			name: "sales transaction",
			src:  `RevSalesTrxn.PlaceSalesTransactionExecutor.execute(null, null, null, null, 'local');`,
			want: `unsupported call "RevSalesTrxn.PlaceSalesTransactionExecutor.execute local industry service mutation surface"`,
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

func TestExecSlackLocalClientReadHarness(t *testing.T) {
	program, err := CompileAnonymous(`
Slack.BotClient bot = new Slack.BotClient();
Slack.AuthTestResponse auth = bot.authTest(new Slack.AuthTestRequest());
System.assertNotEquals(null, auth);
System.assertNotEquals(null, bot.chatPostMessage(new Slack.ChatPostMessageRequest()));
System.assertNotEquals(null, bot.chatUpdate(new Slack.ChatUpdateRequest()));
System.assertNotEquals(null, bot.viewsOpen(new Slack.ViewsOpenRequest()));
System.assertNotEquals(null, bot.viewsPublish(new Slack.ViewsPublishRequest()));
System.assertNotEquals(null, bot.workflowsStepCompleted(new Slack.WorkflowsStepCompletedRequest()));
System.assertNotEquals(null, bot.workflowsStepFailed(new Slack.WorkflowsStepFailedRequest()));
System.assertNotEquals(null, bot.workflowsUpdateStep(new Slack.WorkflowsUpdateStepRequest()));
Slack.UsersInfoResponse userInfo = bot.usersInfo(new Slack.UsersInfoRequest());
System.assertNotEquals(null, userInfo);
System.assertNotEquals(null, bot.bookmarksList(new Slack.BookmarksListRequest()));
System.assertNotEquals(null, bot.reactionsGet(new Slack.ReactionsGetRequest()));
System.assertNotEquals(null, bot.conversationsListConnectInvites(new Slack.ConversationsListConnectInvitesRequest()));
System.assertNotEquals(null, bot.conversationsOpen(new Slack.ConversationsOpenRequest()));
System.assertNotEquals(null, bot.conversationsClose(new Slack.ConversationsCloseRequest()));
System.assertNotEquals(null, bot.conversationsMark(new Slack.ConversationsMarkRequest()));
System.assertNotEquals(null, bot.bookmarksEdit(new Slack.BookmarksEditRequest()));
System.assertNotEquals(null, bot.filesRemoteShare(new Slack.FilesRemoteShareRequest()));
System.assertNotEquals(null, bot.migrationExchange(new Slack.MigrationExchangeRequest()));

Slack.UserClient user = new Slack.UserClient();
Slack.ApiTestResponse api = user.apiTest(new Slack.ApiTestRequest());
System.assertNotEquals(null, api);
System.assertNotEquals(null, user.chatPostEphemeral(new Slack.ChatPostEphemeralRequest()));
System.assertNotEquals(null, user.chatScheduleMessage(new Slack.ChatScheduleMessageRequest()));
System.assertNotEquals(null, user.viewsPush(new Slack.ViewsPushRequest()));
System.assertNotEquals(null, user.viewsUpdate(new Slack.ViewsUpdateRequest()));
Slack.TeamInfoResponse team = user.teamInfo(new Slack.TeamInfoRequest());
System.assertNotEquals(null, team);
System.assertNotEquals(null, user.searchAll(new Slack.SearchAllRequest()));
System.assertNotEquals(null, user.teamAccessLogs(new Slack.TeamAccessLogsRequest()));
System.assertNotEquals(null, user.usersIdentity(new Slack.UsersIdentityRequest()));
System.assertNotEquals(null, user.conversationsOpen(new Slack.ConversationsOpenRequest()));
System.assertNotEquals(null, user.conversationsClose(new Slack.ConversationsCloseRequest()));
System.assertNotEquals(null, user.conversationsMark(new Slack.ConversationsMarkRequest()));
System.assertNotEquals(null, user.bookmarksEdit(new Slack.BookmarksEditRequest()));
System.assertNotEquals(null, user.filesRemoteShare(new Slack.FilesRemoteShareRequest()));
System.assertNotEquals(null, user.filesSharedPublicURL(new Slack.FilesSharedPublicURLRequest()));
System.assertNotEquals(null, user.migrationExchange(new Slack.MigrationExchangeRequest()));

Slack.AppClient app = new Slack.AppClient();
Slack.AuthTestResponse appAuth = app.AUTHTEST(new Slack.AuthTestRequest());
System.assertNotEquals(null, appAuth);
		`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSlackLocalDispatcherAndRunnableHarnessDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
Slack.ActionDispatcher actionDispatcher = new Slack.ActionDispatcher();
System.assertEquals(false, actionDispatcher.allowUnauthenticatedUsers());
System.assertNotEquals(null, actionDispatcher.invoke(new Map<String, Object>(), new Slack.RequestContext()));
Slack.EventDispatcher eventDispatcher = new Slack.EventDispatcher();
System.assertEquals(false, eventDispatcher.allowUnauthenticatedUsers());
System.assertNotEquals(null, eventDispatcher.invoke(new Slack.EventParameters(new Slack.Event(), 'team', 1), new Slack.RequestContext()));
Slack.ShortcutDispatcher shortcutDispatcher = new Slack.ShortcutDispatcher();
System.assertEquals(false, shortcutDispatcher.allowUnauthenticatedUsers());
System.assertNotEquals(null, shortcutDispatcher.invoke(new Slack.ShortcutParameters('shortcut'), new Slack.RequestContext()));
Slack.SlashCommandDispatcher slashCommandDispatcher = new Slack.SlashCommandDispatcher();
System.assertEquals(false, slashCommandDispatcher.allowUnauthenticatedUsers());
System.assertNotEquals(null, slashCommandDispatcher.invoke(new Slack.SlashCommandParameters('/local', 'payload'), new Slack.RequestContext()));
Slack.RunnableHandler handler = new Slack.RunnableHandler();
handler.run();
System.assert(JSON.serialize(handler).contains('"ran":true'));
Slack.UserMappingUrlServiceProvider urls = new Slack.UserMappingUrlServiceProvider();
System.assert(urls.generateSlackAuthorizationUrl('state').contains('state=state'));
System.assert(urls.generatePartnerAuthorizationUrl('partner', 'team').contains('partner=partner'));
Slack.UserProvisioningProvider provisioning = new Slack.UserProvisioningProvider();
System.assertNotEquals(null, provisioning.importUsers(new List<Slack.UserMapping>(), 'team'));
System.assertNotEquals(null, provisioning.revokeUsersBySalesforceId(new List<String>(), 'team'));
System.assertNotEquals(null, provisioning.revokeUsersBySlackId(new List<String>()));
	`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestExecSlackLocalComponentHarnessMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Slack.Checkbox checkbox = new Slack.Checkbox();
checkbox.toggleValue();
System.assertEquals(true, checkbox.getValue());
checkbox.TOGGLEVALUE();
System.assertEquals(false, checkbox.getValue());

Slack.CheckboxGroup groupValue = new Slack.CheckboxGroup();
groupValue.toggleValue('opt-a');
System.assertEquals(1, groupValue.getValue().size());
groupValue.TOGGLEVALUE('opt-a');
System.assertEquals(0, groupValue.getValue().size());

Slack.ExternalSelect selectValue = new Slack.ExternalSelect();
selectValue.QUERY('abc');

Slack.Modal modal = new Slack.Modal();
System.assertEquals(false, modal.hasInputErrors());
System.assertEquals(true, modal.SUBMIT());
modal.close();

Slack.Overflow overflow = new Slack.Overflow();
overflow.clickOption('next');

Slack.Message message = new Slack.Message();
System.assertEquals(true, message.canBeSeenByUser(new Slack.TestHarness.User()));

Slack.Button button = new Slack.Button();
button.CLICK();
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSlackLocalChannelHarnessSendsSessionMessage(t *testing.T) {
	program, err := CompileAnonymous(`
Slack.State state = new Slack.State();
Slack.TestHarness.Enterprise enterprise = state.createEnterprise('E1', 'Example');
Slack.TestHarness.Team team = state.createTeam('example', enterprise);
Slack.TestHarness.User user = state.createUser('U1', 'M User', team, 'en_US');
Slack.TestHarness.Channel channel = state.createPublicChannel(team, 'general', 'en_US');
Slack.TestHarness.UserSession session = state.createUserSession(user, channel);

channel.addUser(user);
System.assertEquals(true, channel.canBeOpenedByUser(user));
Slack.TestHarness.Message message = channel.SENDMESSAGE(session, 'hello');
System.assertEquals('hello', message.getText());
System.assertEquals(1, session.getMessages().size());
session.executeSlashCommand('/local', new Slack.App());
session.executeSlashCommand('/local', 'payload', new Slack.App());
session.executeGlobalShortcut('shortcut', new Slack.App());
session.executeMessageShortcut('shortcut', message, new Slack.App());
session.executeEvent(new Slack.Event(), new Slack.App());
channel.removeUser(user);
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
Test.setCurrentPage(new PageReference('/apex/current'));
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
ApexPages.Action listAction = new ApexPages.Action('{!List}');
System.assertEquals('/list', listAction.invoke().getUrl());
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
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecVisualforceComponentNamespaceAssignableToApexPagesComponent(t *testing.T) {
	program, err := CompileAnonymous(`
ApexPages.Component component = new Component.AccountDetail();
System.assertNotEquals(null, component);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
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

func TestExecPageReferenceForResourceBuildsStaticResourceURL(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference root = PageReference.forResource('MyStaticResource');
System.assertEquals('/resource/MyStaticResource', root.getUrl());
PageReference nested = PageReference.forResource('MyStaticResource', 'images/logo.svg');
System.assertEquals('/resource/MyStaticResource/images/logo.svg', nested.getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPageReferenceForResourceRejectsMissingStaticResource(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	PageReference.forResource('MissingStaticResource');
} catch (Exception e) {
	caught = true;
	System.assertEquals('System.VisualforceException', e.getTypeName());
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "KnownStaticResource"}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPageReferenceParametersAreCaseInsensitive(t *testing.T) {
	directPage := newPageReference("")
	directParams := directPage.Fields["parameters"]
	directVM := New(nil)
	if _, handled, err := directVM.callValueMember("", directParams, "put", []Value{String("Id"), String("value")}, &Result{}); err != nil || !handled {
		t.Fatalf("direct put handled=%t err=%v", handled, err)
	}
	if got, handled, err := directVM.callValueMember("", directParams, "get", []Value{String("Id")}, &Result{}); err != nil || !handled || got.Kind != ValueString || got.Text != "value" {
		t.Fatalf("direct get got=%#v handled=%t err=%v params=%#v", got, handled, err, directParams)
	}

	program, err := CompileAnonymous(`
Map<String,String> plain = new Map<String,String>();
plain.put('Id', 'plain');
System.assertEquals('plain', plain.get('Id'));
PageReference blank = new PageReference();
blank.getParameters().put('Foo', 'bar');
System.assertEquals('bar', blank.getParameters().get('Foo'));
blank.getParameters().put('Id', '001000000000001');
System.assertEquals(2, blank.getParameters().size());
System.assertEquals('001000000000001', blank.getParameters().get('Id'));
System.assertEquals('001000000000001', blank.getParameters().get('id'));
System.assert(blank.getParameters().containsKey('ID'));
System.assertEquals('001000000000001', blank.getParameters().remove('iD'));
System.assertEquals(null, blank.getParameters().get('Id'));
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

func TestExecPageReferenceGetUrlEncodesRawQueryValues(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/ss/apex/pkg__Login?startUrl=productdetails?id=aO9000000000001CAA');
System.assertEquals('/ss/apex/pkg__Login?startUrl=productdetails%3Fid%3DaO9000000000001CAA', page.getUrl());
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

func TestExecKnowledgeArticleVersionStandardControllerSetDataCategory(t *testing.T) {
	program, err := CompileAnonymous(`
Knowledge__kav article = new Knowledge__kav(Title = 'Local article');
ApexPages.KnowledgeArticleVersionStandardController controller = new ApexPages.KnowledgeArticleVersionStandardController(article);
controller.setDataCategory('Products', 'Hardware');
System.assertEquals(null, controller.getSourceId());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardControllerViewUsesTypedIDField(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = Id.valueOf('001000000000001');
Account account = new Account(Id = accountId, Name = 'Existing');
ApexPages.StandardController controller = new ApexPages.StandardController(account);
System.assertEquals('/001000000000001', controller.view().getUrl());
System.assertEquals(accountId, Id.valueOf(controller.view().getUrl().replace('/', '')));
System.assertEquals('/001000000000001', controller.cancel().getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardControllerIdBindsIntoSOQLSet(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Existing');
insert account;
insert new Contact(LastName = 'Line', AccountId = account.Id);
Account queried = [SELECT Id, Name, (SELECT Id, AccountId FROM Contacts) FROM Account WHERE Id = :account.Id LIMIT 1];
ApexPages.StandardController controller = new ApexPages.StandardController(queried);
Set<Id> ids = new Set<Id>{controller.getId()};
List<Account> rows = [SELECT Id, Name, (SELECT Id, AccountId FROM Contacts) FROM Account WHERE Id IN :ids FOR UPDATE];
System.assertEquals(1, rows.size());
System.assertEquals(account.Id, rows[0].Id);
System.assertEquals(1, rows[0].Contacts.size());
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

func TestExecComponentApexDefaultsSupportExpressionsAndChildren(t *testing.T) {
	program, err := CompileAnonymous(`
Component.Apex.Column column = new Component.Apex.Column();
column.expressions.value = '{!row.value}';
System.assertEquals('{!row.value}', column.expressions.value);
Component.Apex.OutputText output = new Component.Apex.OutputText();
output.Expressions.Value = '{!$Label.Done}';
System.assertEquals('{!$Label.Done}', output.Value);
Component.Apex.PageBlockTable table = new Component.Apex.PageBlockTable();
table.childComponents.add(column);
System.assertEquals(1, table.childComponents.size());
System.assertEquals(0, table.rows);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
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
try {
    UserInfo.hasPackageLicense('Dummy Id');
    System.assert(false, 'expected invalid package id to throw');
} catch (System.StringException e) {
    System.assertEquals('Invalid id: Dummy Id', e.getMessage());
}
try {
    UserInfo.hasPackageLicense('050000000000001');
    System.assert(false, 'expected missing package to throw');
} catch (System.TypeException e) {
    System.assertEquals('Package Not Found', e.getMessage());
}
System.assertEquals(false, UserInfo.isCurrentUserLicensed('pkg'));
try {
    UserInfo.isCurrentUserLicensedForPackage('050000000000001');
    System.assert(false, 'expected missing package to throw');
} catch (System.TypeException e) {
    System.assertEquals('Package Not Found', e.getMessage());
}
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
System.assertEquals(false, UserInfo.hasPackageLicense('050000000000002'));
System.assertEquals(false, UserInfo.isCurrentUserLicensed('missing'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine = New(nil)
	packageLicenses := licensedOrg.Objects["PackageLicense"]
	packageLicenses.Records["050000000000002"] = storage.Record{
		ID:     "050000000000002",
		Object: "PackageLicense",
		Fields: map[string]storage.Value{
			"NamespacePrefix": storage.StringValue("pkg2"),
			"Status":          storage.StringValue("Active"),
		},
	}
	licensedOrg.Objects["PackageLicense"] = packageLicenses
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

func TestExecSystemLabelMethodsLimitsAsyncAndTargetExceptionConstructors(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(1.2, Decimal.valueOf('1.25').divide(Decimal.valueOf('1'), 1, RoundingMode.valueOf('HALF_DOWN')));
System.assertEquals('Hello', System.Label.get('', 'Greeting'));
System.assertEquals('Bonjour', System.Label.get('pkg', 'Greeting', 'fr'));
System.assert(System.Label.translationExists('pkg', 'Greeting', 'fr'));
System.assert(!System.Label.translationExists('pkg', 'Greeting', 'es'));
System.assertEquals(0, Limits.getAsyncCalls());
System.assert(Limits.getLimitAsyncCalls() > 0);

Exception invalidNoMessage = new InvalidParameterValueException();
Exception invalidCause = new InvalidParameterValueException(new Exception('cause'));
Exception invalidMessage = new InvalidParameterValueException('bad value');
System.assertEquals('System.InvalidParameterValueException', invalidNoMessage.getTypeName());
System.assertEquals('cause', invalidCause.getCause().getMessage());
System.assertEquals('bad value', invalidMessage.getMessage());

Exception noAccess = new NoAccessException('blocked', new Exception('root'));
Exception noData = new NoDataFoundException('missing', new Exception('root'));
Exception npe = new NullPointerException('null value', new Exception('root'));
System.assertEquals('blocked', noAccess.getMessage());
System.assertEquals('missing', noData.getMessage());
System.assertEquals('null value', npe.getMessage());
System.assertEquals('root', noAccess.getCause().getMessage());
System.assertEquals('root', noData.getCause().getMessage());
System.assertEquals('root', npe.getCause().getMessage());
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
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringFormatResolvesVisualforceLabelMergeExpression(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('Log In', String.format('{!$Label.LogIn}', new List<String>()));
System.assertEquals('Upload limit 5 MB', String.format('{!$Label.UploadPhotoTooBig}', new List<String>{ '5 MB' }));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Metadata.Labels = []storage.LabelMetadata{
		{Name: "LogIn", Language: "en_US", Value: "Log In"},
		{Name: "UploadPhotoTooBig", Language: "en_US", Value: "Upload limit {0}"},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfRendersComponentLabelExpression(t *testing.T) {
	program, err := CompileAnonymous(`
Component.Apex.OutputText output = new Component.Apex.OutputText();
output.Expressions.Value = '{!$Label.LogIn}';
System.assertEquals('{!$Label.LogIn}', output.Value);
System.assertEquals('Log In', String.valueOf(output.Value));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Metadata.Labels = []storage.LabelMetadata{
		{Name: "LogIn", Language: "en_US", Value: "Log In"},
	}
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
Messaging.reserveSingleEmailCapacity(2);
Messaging.reserveMassEmailCapacity(1);
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

func TestExecVisualEditorDynamicPickListRowsPrecedesRegisteredStubClass(t *testing.T) {
	program, err := CompileAnonymous(`
List<VisualEditor.DataRow> dataRows = new List<VisualEditor.DataRow>();
dataRows.add(new VisualEditor.DataRow('A', 'a'));
VisualEditor.DynamicPickListRows rows = new VisualEditor.DynamicPickListRows(dataRows);
System.assertEquals(1, rows.getDataRows().size());
System.assertEquals('A', rows.getDataRows().get(0).getLabel());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "VisualEditor.DynamicPickListRows",
		Methods: map[string]Method{
			"getDataRows": {
				Name:       "VisualEditor.DynamicPickListRows.getDataRows",
				ClassName:  "VisualEditor.DynamicPickListRows",
				ReturnType: "List<VisualEditor.DataRow>",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "VisualEditor.DataRow",
		Methods: map[string]Method{
			"getLabel": {
				Name:       "VisualEditor.DataRow.getLabel",
				ClassName:  "VisualEditor.DataRow",
				ReturnType: "String",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformDTOGetterReadsMatchingField(t *testing.T) {
	program, err := CompileAnonymous(`
commercepromotions.PromotionRequest request = new commercepromotions.PromotionRequest();
Object direct = request.buyerAccountId;
Object getter = request.getBuyerAccountId();
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(nil).Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Vars["direct"]; got.Kind != ValueNull {
		t.Fatalf("direct buyerAccountId = %#v vars=%#v", got, result.Vars)
	}
	if got := result.Vars["getter"]; got.Kind != ValueNull {
		t.Fatalf("getter buyerAccountId = %#v vars=%#v", got, result.Vars)
	}
}

func TestGeneratedPlatformDTOCallValueMemberReadsMatchingField(t *testing.T) {
	machine := New(nil)
	receiver, err := machine.constructValue("commercepromotions.PromotionRequest", nil, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	machine.Globals["request"] = receiver
	machine.VarTypes["request"] = "commercepromotions.PromotionRequest"
	got, handled, err := machine.callValueMember("request", receiver, "getBuyerAccountId", nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("getBuyerAccountId was not handled")
	}
	if got.Kind != ValueNull {
		t.Fatalf("getBuyerAccountId = %#v receiver=%#v", got, receiver)
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

func TestConstructPassiveGeneratedPlatformDTOPositionalArgsBindParameters(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "CommerceBuyGrp.BuyerGroupRequest",
		Namespace:  "CommerceBuyGrp",
		SuperClass: "Object",
		Access:     "global",
		Constructors: []Method{{
			Name:          "CommerceBuyGrp.BuyerGroupRequest.<init>",
			ClassName:     "CommerceBuyGrp.BuyerGroupRequest",
			IsConstructor: true,
			Access:        "global",
			Params: []Param{
				{Name: "storeId", Type: "String"},
				{Name: "accountId", Type: "String"},
				{Name: "requestContextParameters", Type: "Map<String,Object>"},
			},
		}},
		Methods: map[string]Method{
			"getStoreId": {
				Name:       "CommerceBuyGrp.BuyerGroupRequest.getStoreId",
				ClassName:  "CommerceBuyGrp.BuyerGroupRequest",
				ReturnType: "String",
				Access:     "global",
				Modifiers:  []string{"passive-generated"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.constructValue("CommerceBuyGrp.BuyerGroupRequest", []Value{String("store-one"), String("account-one"), Map()}, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got := value.Fields["storeId"]; got.Kind != ValueString || got.Text != "store-one" {
		t.Fatalf("storeId = %#v fields=%#v", got, value.Fields)
	}
	machine.Globals["request"] = value
	machine.VarTypes["request"] = "CommerceBuyGrp.BuyerGroupRequest"
	got, handled, err := machine.callValueMember("request", value, "getStoreId", nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("getStoreId was not handled")
	}
	if got.Kind != ValueString || got.Text != "store-one" {
		t.Fatalf("getStoreId = %#v fields=%#v", got, value.Fields)
	}
}

func TestConstructGeneratedPlatformDTOPositionalArgsBindParameters(t *testing.T) {
	machine := New(nil)
	value, err := machine.constructValue("CommerceBuyGrp.BuyerGroupRequest", []Value{String("store-one"), String("account-one"), Map()}, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if got := value.Fields["storeId"]; got.Kind != ValueString || got.Text != "store-one" {
		t.Fatalf("storeId = %#v fields=%#v", got, value.Fields)
	}
}

func TestExecMessagingNotificationAndTemplateDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
List<Messaging.RenderEmailTemplateBodyResult> rendered =
	Messaging.renderEmailTemplate('003000000000001AAA', '001000000000001AAA', new List<String>{'Hello {!Contact.Name} / {!Account.Name}'});
System.assertEquals(1, rendered.size());
System.assert(rendered.get(0).getSuccess());
System.assertEquals('Hello Ada Trail / Acme', rendered.get(0).getMergedBody());
System.assertEquals(0, rendered.get(0).getErrors().size());

Messaging.CustomNotification custom = new Messaging.CustomNotification();
custom.setNotificationTypeId('0ML000000000001AAA');
custom.setSenderId('005000000000001AAA');
custom.setTitle('Title');
custom.setBody('Body');
custom.setTargetId('001000000000001AAA');
custom.setTargetPageRef('/lightning/r/Account/001000000000001AAA/view');
custom.send(new Set<String>{'005000000000001AAA'});

Map<String,Object> payload = Messaging.PushNotificationPayload.apple('Alert', 'default', 1, new Map<String,Object>{'recordId' => '001000000000001AAA'});
System.assert(payload.containsKey('aps'));
System.assertEquals('001000000000001AAA', payload.get('recordId'));
Messaging.PushNotification push = new Messaging.PushNotification(payload);
push.setTtl(60);
push.send('ConnectedApp', new Set<String>{'005000000000001AAA'});

List<Messaging.SendEmailResult> messageResults = Messaging.sendEmailMessage(new List<Id>{'02s000000000001AAA'}, false);
System.assertEquals(1, messageResults.size());
System.assert(messageResults.get(0).isSuccess());

Messaging.InboundEmail inbound = Messaging.extractInboundEmail('raw message not parsed locally', true);
System.assertEquals(0, inbound.headers.size());
System.assertEquals(false, inbound.plainTextBodyIsTruncated);

Messaging.ActionResult actionResult = new Messaging.ActionResult.Builder().withSuccess(true).withMessage('ok').build();
System.assert(actionResult.isSuccess());
System.assertEquals('ok', actionResult.getMessage());
Messaging.ActionableNotification actionable = new Messaging.ActionableNotification.Builder().withActionIdentifier('open').build();
System.assertEquals('open', actionable.getActionIdentifier());
`)
	if err != nil {
		t.Fatal(err)
	}
	vm := New(nil)
	org := emailTemplateTestOrg()
	vm.SetOrg(&org)
	if _, err := vm.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMessagingSettersAcceptNullOptionalArguments(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage email = new Messaging.SingleEmailMessage();
email.setToAddresses(null);
email.setCcAddresses(null);

Messaging.CustomNotification custom = new Messaging.CustomNotification();
custom.setSenderId(null);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
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
Messaging.SingleEmailMessage withAttachmentOption = Messaging.renderStoredEmailTemplate('00X000000000001AAA', '003000000000001AAA', '001000000000001AAA', Messaging.AttachmentRetrievalOption.METADATA_ONLY);
System.assertEquals('Verify body', withAttachmentOption.getPlainTextBody());
Messaging.SingleEmailMessage withUpdateUsage = Messaging.renderStoredEmailTemplate('00X000000000001AAA', '003000000000001AAA', '001000000000001AAA', Messaging.AttachmentRetrievalOption.METADATA_WITH_BODY, false);
System.assertEquals('Verify subject', withUpdateUsage.getSubject());
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

func TestExecRenderStoredVisualforceEmailTemplateContent(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage rendered = Messaging.renderStoredEmailTemplate('00X000000000004AAA', null, null);
System.assert(rendered.getHtmlBody().contains('src="http://example.com/logo.png"'));
System.assert(rendered.getHtmlBody().contains('href="http://example.com/reset"'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "EmailTemplate")
	templateObject := org.Objects["EmailTemplate"]
	templateObject.Records["00X000000000004AAA"] = storage.Record{
		ID:     "00X000000000004AAA",
		Object: "EmailTemplate",
		Fields: map[string]storage.Value{
			"Subject":      storage.StringValue("Reset"),
			"TemplateType": storage.StringValue("visualforce"),
			"Body": storage.StringValue(`<messaging:emailTemplate subject="Reset" recipientType="Contact">
<messaging:htmlEmailBody>
<apex:outputText escape="false" value="{!'<img src=\"'}"/><c:EmailContent key="Logo"/><apex:outputText escape="false" value="{!'\"/>'}"/>
<apex:outputText escape="false" value="{!'<a href=\"'}"/>
<nu:EmailContent key="Link"/>
<apex:outputText escape="false" value="{!'\">Reset</a>'}"/>
</messaging:htmlEmailBody>
</messaging:emailTemplate>`),
		},
	}
	org.Objects["EmailTemplate"] = templateObject
	machine := New(nil)
	machine.SetOrg(&org)
	content := Map()
	content.Type = "Map<String,String>"
	content.Map[mapKey(String("Logo"))] = String("http://example.com/logo.png")
	content.MapKeys[mapKey(String("Logo"))] = String("Logo")
	content.MapOrder = append(content.MapOrder, mapKey(String("Logo")))
	content.Map[mapKey(String("Link"))] = String("http://example.com/reset")
	content.MapKeys[mapKey(String("Link"))] = String("Link")
	content.MapOrder = append(content.MapOrder, mapKey(String("Link")))
	machine.Classes["EmailContent"] = Class{
		Name: "EmailContent",
		StaticFields: map[string]Field{
			"contentMap": {Name: "contentMap", Type: "Map<String,String>", Static: true, Value: content},
		},
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
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

func TestExecApexPagesAddMessageDeduplicatesEquivalentMessages(t *testing.T) {
	program, err := CompileAnonymous(`
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Same summary'));
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Same summary'));
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Same summary', 'Same detail'));
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Same summary', 'Same detail'));
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.ERROR, 'Same summary'));
System.assertEquals(3, ApexPages.getMessages().size());
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
Account account = [SELECT Id, Name, Description FROM Account WHERE Name = 'Installed'];
System.assertEquals('Installed', account.Name);
Test.testInstall(new InstallScript(), new Version(1, 47, 0), false);
`)
	if err != nil {
		t.Fatal(err)
	}
	onInstall, err := CompileAnonymous(`
if (context.previousVersion() == null) {
	System.assert(!context.isUpgrade());
	insert new Account(Name = 'Installed');
} else {
	System.assert(context.isUpgrade());
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

func TestExecTestUninstallInvokesUninstallHandler(t *testing.T) {
	program, err := CompileAnonymous(`
Test.testUninstall(new UninstallScript());
System.assertEquals(1, UninstallScript.count);
`)
	if err != nil {
		t.Fatal(err)
	}
	onUninstall, err := CompileAnonymous(`
System.assertNotEquals(null, context);
UninstallScript.count++;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "UninstallScript",
		Interfaces: []string{"UninstallHandler"},
		StaticFields: map[string]Field{
			"count": {Name: "count", Type: "Integer", InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"onUninstall": {
				Name:       "UninstallScript.onUninstall",
				ClassName:  "UninstallScript",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "UninstallContext"}},
				Program:    onUninstall,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestInstallComparesPreviousVersionWithStaticFinalVersion(t *testing.T) {
	program, err := CompileAnonymous(`
Test.testInstall(new InstallScript(), new Version(1, 24), true);
System.assertEquals(0, InstallScript.lessThanCount);
`)
	if err != nil {
		t.Fatal(err)
	}
	onInstall, err := CompileAnonymous(`
if (context.previousVersion().compareTo(VERSION_1_24) < 0) {
	lessThanCount++;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "InstallScript",
		StaticFields: map[string]Field{
			"VERSION_1_24":  {Name: "VERSION_1_24", Type: "Version", Value: versionTestValue(1, 24, 0)},
			"lessThanCount": {Name: "lessThanCount", Type: "Integer", Value: Int(0)},
		},
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

func versionTestValue(major, minor, patch int64) Value {
	version := Object("Version")
	version.Fields["major"] = Int(major)
	version.Fields["minor"] = Int(minor)
	version.Fields["patch"] = Int(patch)
	return version
}

func TestExecTestInstallSuppressesAfterTriggerSideEffects(t *testing.T) {
	program, err := CompileAnonymous(`
Test.testInstall(new InstallScript(), new Version(1, 0), true);
System.assertEquals(1, InstallScript.beforeCount);
System.assertEquals(0, InstallScript.afterCount);
Account account = [SELECT Id, Name, Description FROM Account WHERE Name = 'Installed'];
System.assertEquals('Touched', account.Description);
`)
	if err != nil {
		t.Fatal(err)
	}
	onInstall, err := CompileAnonymous(`
insert new Account(Name = 'Installed');
`)
	if err != nil {
		t.Fatal(err)
	}
	triggerBefore, err := CompileAnonymous(`
InstallScript.beforeCount++;
for (Account account : (List<Account>) Trigger.new) {
	account.Description = 'Touched';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	triggerAfter, err := CompileAnonymous(`
InstallScript.afterCount++;
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
		StaticFields: map[string]Field{
			"beforeCount": {Name: "beforeCount", Type: "Integer", InitialValue: Int(0)},
			"afterCount":  {Name: "afterCount", Type: "Integer", InitialValue: Int(0)},
		},
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
	if err := machine.RegisterTrigger(Trigger{Object: "Account", Timing: triggerTimingBefore, Operation: "insert", Program: triggerBefore}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Object: "Account", Timing: triggerTimingAfter, Operation: "insert", Program: triggerAfter}); err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatalf("err = %v", err)
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

func TestExecWebServiceCalloutWithoutMockCreatesEmptyResponseShell(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> response = new Map<String, Object>();
WebServiceCallout.invoke(
  new Object(),
  'request',
  response,
  new String[]{'https://example.test', 'soapAction', 'requestNS', 'requestName', 'responseNS', 'responseName', 'ResponseType'}
);
System.assertNotEquals(null, response.get('response_x'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatalf("err = %#v", err)
	}
	if result.Limits.Callouts != 1 {
		t.Fatalf("callouts = %d, want 1", result.Limits.Callouts)
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
System.assertEquals(false, FlexQueue.moveJobToFront('707000000000001'));
System.assertEquals(false, FlexQueue.moveJobToEnd('707000000000001'));
System.assertEquals(false, FlexQueue.moveBeforeJob('707000000000001', '707000000000002'));
System.assertEquals(false, FlexQueue.moveAfterJob('707000000000001', '707000000000002'));
System.pauseJobById('08e000000000001');
System.pauseJobByName('local job');
System.resumeJobById('08e000000000001');
System.resumeJobByName('local job');
System.assertEquals(0, System.purgeOldAsyncJobs(Date.today()));
System.assertEquals(0, System.purgeOldAsyncJobs(Date.today(), 10));
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
System.assertEquals('SELECT Id , Name FROM Account', inlineLocator.getQuery());
List<Account> inlineQueried = Database.query(inlineLocator.getQuery());
System.assertEquals(1, inlineQueried.size());
Object inlineIterator = inlineLocator.iterator();
System.assert(inlineIterator.hasNext());
Account inlineRow = inlineIterator.next();
System.assertEquals('Acme', inlineRow.Name);
System.assert(!inlineIterator.hasNext());

Database.QueryLocator accessLocator = Database.getQueryLocator('SELECT Id, Name FROM Account', AccessLevel.USER_MODE);
System.assertEquals('SELECT Id, Name FROM Account', accessLocator.getQuery());
Object accessIterator = accessLocator.iterator();
System.assert(accessIterator.hasNext());
Account accessRow = accessIterator.next();
System.assertEquals('Acme', accessRow.Name);
System.assert(!accessIterator.hasNext());

Database.QueryLocator emptyInlineLocator = Database.getQueryLocator([
  SELECT Id, Name
  FROM Account
  WHERE Name = 'Missing'
  ORDER BY Name NULLS LAST
]);
System.assertEquals('SELECT Id , Name FROM Account WHERE Name = ''Missing'' ORDER BY Name NULLS LAST', emptyInlineLocator.getQuery());
List<Account> emptyInlineQueried = Database.query(emptyInlineLocator.getQuery());
System.assertEquals(0, emptyInlineQueried.size());
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

func TestBatchScopeValuesRefreshesQueryLocatorQueriedFields(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"pkg__SelectedDate__c": {APIName: "pkg__SelectedDate__c", Type: storage.FieldDate},
		}},
	}
	machine.Org = &org

	row := Object("Contact")
	row.Fields["Id"] = platformScalar("Id", "003000000000001AAA")
	row.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Contact", map[string]bool{"currencyisocode": true})
	locator := Object("Database.QueryLocator")
	locator.Fields["Query"] = String("SELECT Id, pkg__SelectedDate__c FROM Contact")
	locator.Fields["Records"] = List(row)

	scope, err := machine.batchScopeValues(locator, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope) != 1 || !machine.queriedSObjectFieldsIncludes(scope[0], "pkg__SelectedDate__c") {
		t.Fatalf("scope = %#v, want refreshed queried field", scope)
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

func TestUnitOfWorkCommitPersistsMultipleObjectBuckets(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a00",
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
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Name":      {APIName: "Name", Type: storage.FieldString},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Parent__c"},
					ParentRelationship: "Parent__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	uow, err := machine.constructFrameworkSObjectUnitOfWork([]Value{
		List(sObjectTypeToken("Parent__c"), sObjectTypeToken("Child__c")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := Object("Parent__c")
	parent.Fields["Name"] = String("Parent")
	child := Object("Child__c")
	child.Fields["Name"] = String("Child")
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{child}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "commitwork", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	if got := len(machine.Org.Objects["Parent__c"].Records); got != 1 {
		t.Fatalf("parent records = %d", got)
	}
	if got := len(machine.Org.Objects["Child__c"].Records); got != 1 {
		t.Fatalf("child records = %d", got)
	}
}

func TestUnitOfWorkCommitPersistsChildBucketWithDeferredRelationship(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Unused__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Unused__c",
				KeyPrefix: "a09",
				Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a00",
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
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Name":      {APIName: "Name", Type: storage.FieldString},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Parent__c"},
					ParentRelationship: "Parent__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	uow, err := machine.constructFrameworkSObjectUnitOfWork([]Value{
		List(sObjectTypeToken("Unused__c"), sObjectTypeToken("Parent__c"), sObjectTypeToken("Child__c")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := Object("Parent__c")
	parent.Fields["Name"] = String("Parent")
	child := Object("Child__c")
	child.Fields["Name"] = String("Child")
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{child}, nil); err != nil {
		t.Fatal(err)
	}
	if err := machine.addFrameworkSObjectUnitOfWorkRelationship(uow, "Child__c", child, sObjectFieldToken("Child__c", "Parent__c"), parent); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "commitwork", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	if got := len(machine.Org.Objects["Parent__c"].Records); got != 1 {
		t.Fatalf("parent records = %d", got)
	}
	if got := len(machine.Org.Objects["Child__c"].Records); got != 1 {
		t.Fatalf("child records = %d", got)
	}
	for _, row := range machine.Org.Objects["Child__c"].Records {
		if row.Fields["Parent__c"].Kind != storage.ValueID {
			t.Fatalf("child parent lookup = %#v", row.Fields["Parent__c"])
		}
	}
}

func TestUnitOfWorkCommitPersistsOutOfOrderRelationship(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a00",
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
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Name":      {APIName: "Name", Type: storage.FieldString},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Parent__c"},
					ParentRelationship: "Parent__r",
				}},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	uow, err := machine.constructFrameworkSObjectUnitOfWork([]Value{
		List(sObjectTypeToken("Child__c"), sObjectTypeToken("Parent__c")),
		frameworkSObjectUnitOfWorkRelationshipBehaviorValue("AttemptResolveOutOfOrder"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := Object("Parent__c")
	parent.Fields["Name"] = String("Parent")
	child := Object("Child__c")
	child.Fields["Name"] = String("Child")
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{child, sObjectFieldToken("Child__c", "Parent__c"), parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "commitwork", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range machine.Org.Objects["Child__c"].Records {
		if row.Fields["Parent__c"].Kind != storage.ValueID {
			t.Fatalf("child parent lookup = %#v", row.Fields["Parent__c"])
		}
	}
}

func TestUnitOfWorkOutOfOrderUpdateUsesPersistedRelationshipState(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a00",
				Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
				},
			},
			Records: map[storage.ID]storage.Record{
				"a00000000000001AAA": {
					ID:     "a00000000000001AAA",
					Object: "Parent__c",
					Fields: map[string]storage.Value{
						"Id": storage.IDValue("a00000000000001AAA"),
					},
				},
			},
		},
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Child__c",
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
				},
			},
			Records: map[storage.ID]storage.Record{
				"a01000000000001AAA": {
					ID:     "a01000000000001AAA",
					Object: "Child__c",
					Fields: map[string]storage.Value{
						"Id": storage.IDValue("a01000000000001AAA"),
					},
				},
			},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)

	parent := Object("Parent__c")
	parent.Fields["Id"] = platformScalar("Id", "a00000000000001AAA")
	child := Object("Child__c")
	child.Fields["Id"] = platformScalar("Id", "a01000000000001AAA")
	child.Fields["Parent__c"] = platformScalar("Id", "a00000000000001AAA")
	relationship := Object("framework_SObjectUnitOfWork.Relationship")
	relationship.Fields["Record"] = child
	relationship.Fields["RelatedToField"] = sObjectFieldToken("Child__c", "Parent__c")
	relationship.Fields["RelatedTo"] = parent

	updateRecord, ok, err := machine.frameworkSObjectUnitOfWorkOutOfOrderUpdate(relationship)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		persisted, hasField, hasRecord := machine.persistedSObjectFieldValue(child, "Parent__c")
		t.Fatalf("expected persisted missing lookup to produce an update; persisted=%#v hasField=%v hasRecord=%v recordID=%#v relatedID=%#v", persisted, hasField, hasRecord, sObjectIDValue(child), sObjectIDValue(parent))
	}
	if got, ok := comparableIDText(updateRecord.Fields["Parent__c"]); !ok || got != "a00000000000001AAA" {
		t.Fatalf("out-of-order update parent = %#v", got)
	}

	stored := machine.Org.Objects["Child__c"]
	row := stored.Records["a01000000000001AAA"]
	row.Fields["Parent__c"] = storage.IDValue("a00000000000001AAA")
	stored.Records["a01000000000001AAA"] = row
	machine.Org.Objects["Child__c"] = stored
	if _, ok, err := machine.frameworkSObjectUnitOfWorkOutOfOrderUpdate(relationship); err != nil || ok {
		t.Fatalf("stored relationship should not produce update: ok=%v err=%v", ok, err)
	}
}

func TestUnitOfWorkCustomDMLPropagatesInsertedIDsForOutOfOrderRelationship(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a00",
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
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Name":      {APIName: "Name", Type: storage.FieldString},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	insertProgram, err := CompileAnonymous(`Database.insert(records);`)
	if err != nil {
		t.Fatal(err)
	}
	updateProgram, err := CompileAnonymous(`Database.update(records);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "CustomDML",
		Methods: map[string]Method{
			"dmlInsert": {
				Name:      "CustomDML.dmlInsert",
				ClassName: "CustomDML",
				Params:    []Param{{Name: "records", Type: "List<SObject>"}},
				Program:   insertProgram,
			},
			"dmlUpdate": {
				Name:      "CustomDML.dmlUpdate",
				ClassName: "CustomDML",
				Params:    []Param{{Name: "records", Type: "List<SObject>"}},
				Program:   updateProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	uow, err := machine.constructFrameworkSObjectUnitOfWork([]Value{
		List(sObjectTypeToken("Child__c"), sObjectTypeToken("Parent__c")),
		frameworkSObjectUnitOfWorkRelationshipBehaviorValue("AttemptResolveOutOfOrder"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	uow.Fields["m_dml"] = Object("CustomDML")
	parent := Object("Parent__c")
	parent.Fields["Name"] = String("Parent")
	child := Object("Child__c")
	child.Fields["Name"] = String("Child")
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{child, sObjectFieldToken("Child__c", "Parent__c"), parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "commitwork", nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range machine.Org.Objects["Child__c"].Records {
		if row.Fields["Parent__c"].Kind != storage.ValueID {
			t.Fatalf("child parent lookup = %#v", row.Fields["Parent__c"])
		}
	}
}

func TestUnitOfWorkRelationshipResolutionUpdatesCopiedRecordAlias(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Parent__c": {
			Definition: storage.ObjectDefinition{
				APIName:   "Parent__c",
				KeyPrefix: "a00",
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
				KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Name":      {APIName: "Name", Type: storage.FieldString},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"},
				},
			},
			Records: map[storage.ID]storage.Record{},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	uow, err := machine.constructFrameworkSObjectUnitOfWork([]Value{
		List(sObjectTypeToken("Parent__c"), sObjectTypeToken("Child__c")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parent := Object("Parent__c")
	parent.Fields["Name"] = String("Parent")
	child := Object("Child__c")
	child.Fields["Name"] = String("Child")
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{parent}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "registernew", []Value{child}, nil); err != nil {
		t.Fatal(err)
	}
	copiedChild := cloneValuePreserveRefs(child)
	if err := machine.addFrameworkSObjectUnitOfWorkRelationship(uow, "Child__c", copiedChild, sObjectFieldToken("Child__c", "Parent__c"), parent); err != nil {
		t.Fatal(err)
	}
	if _, _, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "commitwork", nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, row := range machine.Org.Objects["Child__c"].Records {
		if row.Fields["Parent__c"].Kind != storage.ValueID {
			t.Fatalf("child parent lookup = %#v", row.Fields["Parent__c"])
		}
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
System.assertEquals(null, namespaced);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeDescribeUsesLocalNameWithoutTokenHint(t *testing.T) {
	program, err := CompileAnonymous(`
Setup_Data__c record = new Setup_Data__c();
Schema.DescribeSObjectResult localDescribe = record.getSObjectType().getDescribe();
System.assertEquals('Setup_Data__c', localDescribe.getName());
System.assertEquals('Setup_Data__c', localDescribe.getLocalName());

Map<Schema.SObjectType, Schema.DescribeSObjectResult> cache = new Map<Schema.SObjectType, Schema.DescribeSObjectResult>();
cache.put(record.getSObjectType(), localDescribe);

Schema.DescribeSObjectResult tokenDescribe = Setup_Data__c.SObjectType.getDescribe();
System.assertEquals('Setup_Data__c', tokenDescribe.getName());
System.assertEquals('Setup_Data__c', tokenDescribe.getLocalName());
System.assertEquals('Setup_Data__c', cache.get(Setup_Data__c.SObjectType).getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "verifiable"
	org.Objects["Setup_Data__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Setup_Data__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["verifiable__Setup_Data__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "verifiable__Setup_Data__c",
			Fields: map[string]storage.Field{
				"Name":                       {APIName: "Name", Type: storage.FieldString},
				"verifiable__Webhook_Url__c": {APIName: "verifiable__Webhook_Url__c", Type: storage.FieldString},
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

func TestExecCurrentPackageSObjectTypeStringMatchesDescribeName(t *testing.T) {
	program, err := CompileAnonymous(`
String describeName = Source__mdt.SObjectType.getDescribe().getName();
System.assertEquals('Source__mdt', describeName);
System.assertEquals(describeName, String.valueOf(Source__mdt.SObjectType));
System.assertEquals(describeName, String.valueOf(Source__mdt.class));
System.assertEquals(describeName, Source__mdt.class.getName());

List<Schema.SObjectType> references = Container__mdt.Source__c.getDescribe().getReferenceTo();
System.assertEquals(1, references.size());
System.assertEquals(describeName, references[0].getDescribe().getName());
System.assertEquals(describeName, String.valueOf(references[0]));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Source__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Source__mdt",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["pkg__Source__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Source__mdt",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["pkg__Container__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Container__mdt",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Type: storage.FieldString},
				"pkg__Source__c": {APIName: "pkg__Source__c", Type: storage.FieldReference, ReferenceTo: []string{"pkg__Source__mdt"}, RelationshipName: "pkg__Source__r"},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentNamespace = "pkg"
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeDescribeKeepsManagedNameWithoutLocalObjectAlias(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('pkg__Setting__c', Setting__c.SObjectType.getDescribe().getName());
System.assertEquals('Setting__c', Setting__c.SObjectType.getDescribe().getLocalName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Setting__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Setting__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedFieldSetMapMatchesLocalMetadataObjectName(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Thing__c.SObjectType.getDescribe();
System.assertNotEquals(null, describe.fieldSets.getMap().get('pkg__Details'));
System.assertNotEquals(null, describe.fieldSets.getMap().get('Details'));
System.assertEquals(1, SObjectType.Thing__c.FieldSets.pkg__Details.getFields().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["pkg__Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Thing__c",
			Fields: map[string]storage.Field{
				"pkg__Name__c": {APIName: "pkg__Name__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Metadata.FieldSets = []storage.FieldSetMetadata{
		{
			ObjectName: "Thing__c",
			Namespace:  "pkg",
			Name:       "Details",
			Fields: []storage.FieldSetMemberMetadata{
				{Field: "Name__c"},
			},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInsertedCustomIdsResolveToTheirOwnSObjectTypeWithDuplicatePrefixes(t *testing.T) {
	program, err := CompileAnonymous(`
Alpha__c alpha = new Alpha__c(Name = 'Alpha');
Beta__c beta = new Beta__c(Name = 'Beta');
insert alpha;
insert beta;
System.assertEquals(Alpha__c.SObjectType, alpha.Id.getSObjectType());
System.assertEquals(Beta__c.SObjectType, beta.Id.getSObjectType());
System.assertNotEquals(String.valueOf(alpha.Id).left(3), String.valueOf(beta.Id).left(3));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Alpha__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Alpha__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Beta__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Beta__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
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

func TestChildRelationshipListTypePrefersCurrentPackageObjectOverAliasRecordType(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["NU__Cart__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "NU__Cart__c"}}
	org.Objects["CartItem"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "CartItem",
			Relations: []storage.Relationship{{
				Field:             "Cart__c",
				ParentObjects:     []string{"NU__Cart__c"},
				ChildRelationship: "CartItems__r",
			}},
		},
	}
	org.Objects["NU__CartItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "NU__CartItem__c",
			Fields: map[string]storage.Field{
				"NU__Cart__c": {APIName: "NU__Cart__c", Type: storage.FieldReference, ReferenceTo: []string{"NU__Cart__c"}, ChildRelationshipName: "NU__CartItems__r"},
			},
			Relations: []storage.Relationship{{
				Field:             "NU__Cart__c",
				ParentObjects:     []string{"NU__Cart__c"},
				ChildRelationship: "NU__CartItems__r",
			}},
		},
	}
	machine.SetOrg(&org)

	got := machine.childRelationshipListType("NU__Cart__c", "CartItems__r", []storage.Record{{Object: "CartItem"}})
	if got != "NU__CartItem__c" {
		t.Fatalf("childRelationshipListType = %q, want NU__CartItem__c", got)
	}
}

func TestExecNetworkGetNetworkIdFallbackIsValidId(t *testing.T) {
	program, err := CompileAnonymous(`
String networkId = Network.getNetworkId();
System.assertEquals('0DB000000000001', networkId);
System.assertEquals(networkId, Id.valueOf(networkId).toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	delete(org.Objects, "Network")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameUsesBuiltinPrecedenceInsideClassMethod(t *testing.T) {
	resolveProgram, err := CompileAnonymous(`return Type.forName(Account.SObjectType.getDescribe().getName());`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
TypeForNameProbe probe = new TypeForNameProbe();
probe.Type = 'local';
Type resolved = probe.resolve();
System.assertEquals('Account', resolved.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "TypeForNameProbe",
		Fields: map[string]Field{
			"Type": {Name: "Type", Type: "String"},
		},
		Methods: map[string]Method{
			"resolve": {
				Name:       "TypeForNameProbe.resolve",
				ClassName:  "TypeForNameProbe",
				ReturnType: "Type",
				Program:    resolveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameNamespaceAcceptsQualifiedName(t *testing.T) {
	program, err := CompileAnonymous(`
Type qualified = Type.forName('pkg', 'pkg.Thing');
System.assertNotEquals(null, qualified);
System.assertEquals('pkg.Thing', qualified.getName());
Object value = Type.forName('pkg', 'Thing').newInstance();
System.Assert.isInstanceOfType(value, qualified);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Thing", Namespace: "pkg", Access: "global"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameNamespacedNestedClassPreservesNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
Type inner = Type.forName('NU', 'SystemUtilTest.TestInnerClass');
System.assertEquals('NU.SystemUtilTest.TestInnerClass', inner.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "SystemUtilTest.TestInnerClass", Namespace: "NU"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecClassLiteralGetNameUsesOrgNamespace(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('NU.LocalThing', LocalThing.class.getName());
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "NU"
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "LocalThing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedNestedTypeNewInstanceMatchesInheritedInterface(t *testing.T) {
	makeProgram, err := CompileAnonymous(`return Type.forName('NU', 'PaymentGatewayServiceTest.MockPaymentGatewayWithTwoPhase').newInstance();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Object gateway = PaymentGatewayFactory.make();
System.assert(gateway instanceof IPaymentGateway2);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "IPaymentGateway2", Namespace: "NU", IsInterface: true},
		{Name: "PaymentGatewayServiceTest.MockPaymentGateway", Namespace: "NU", Interfaces: []string{"IPaymentGateway2"}},
		{Name: "PaymentGatewayServiceTest.MockPaymentGatewayWithTwoPhase", Namespace: "NU", SuperClass: "MockPaymentGateway"},
		{Name: "PaymentGatewayFactory", Namespace: "NU", Access: "global", Methods: map[string]Method{
			"make": {Name: "PaymentGatewayFactory.make", ClassName: "PaymentGatewayFactory", Access: "global", IsStatic: true, ReturnType: "Object", Program: makeProgram},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNewInstanceAllowsPublicReflectedPackageClass(t *testing.T) {
	program, err := CompileAnonymous(`
Object value = Type.forName('pkg.PublicWorker').newInstance();
System.assertNotEquals(null, value);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.currentNamespace = "namz"
	if err := machine.RegisterClass(Class{Name: "PublicWorker", Namespace: "pkg", Access: "public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNewInstanceAllowsNestedTestClassAcrossNamespace(t *testing.T) {
	makeProgram, err := CompileAnonymous(`return Type.forName('HarnessTest.PrivateWorker').newInstance();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Object value = Factory.make();
System.assertNotEquals(null, value);
System.assert(value instanceof HarnessTest.PrivateWorker);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "HarnessTest", IsTest: true},
		{Name: "HarnessTest.PrivateWorker", Access: "private"},
		{Name: "Factory", Namespace: "pkg", Access: "global", Methods: map[string]Method{
			"make": {Name: "Factory.make", ClassName: "Factory", Access: "global", IsStatic: true, ReturnType: "Object", Program: makeProgram},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedNestedTypeNewInstanceMatchesInheritedInterfaceWithQualifiedNestedSuperclass(t *testing.T) {
	makeProgram, err := CompileAnonymous(`return Type.forName(null, 'PaymentGatewayServiceTest.MockPaymentGatewayWithTwoPhase').newInstance();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Object gateway = PaymentGatewayFactory.make();
System.assert(gateway instanceof IPaymentGateway2);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "IPaymentGateway2", Namespace: "NU", IsInterface: true},
		{Name: "PaymentGatewayServiceTest.MockPaymentGateway", Namespace: "NU", Interfaces: []string{"IPaymentGateway2"}},
		{Name: "PaymentGatewayServiceTest.MockPaymentGatewayWithTwoPhase", Namespace: "NU", SuperClass: "PaymentGatewayServiceTest.MockPaymentGateway"},
		{Name: "PaymentGatewayFactory", Namespace: "NU", Access: "global", Methods: map[string]Method{
			"make": {Name: "PaymentGatewayFactory.make", ClassName: "PaymentGatewayFactory", Access: "global", IsStatic: true, ReturnType: "Object", Program: makeProgram},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedSuperclassShadowsSameNamespaceTopLevelClass(t *testing.T) {
	makeProgram, err := CompileAnonymous(`return Type.forName(null, 'PaymentGatewayServiceTest.MockPaymentGatewayWithTwoPhase').newInstance();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Object gateway = PaymentGatewayFactory.make();
System.assert(gateway instanceof IPaymentGateway2);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "IPaymentGateway", Namespace: "NU", IsInterface: true},
		{Name: "IPaymentGateway2", Namespace: "NU", IsInterface: true},
		{Name: "MockPaymentGateway", Namespace: "NU", Interfaces: []string{"IPaymentGateway"}},
		{Name: "PaymentGatewayServiceTest.MockPaymentGateway", Namespace: "NU", Interfaces: []string{"IPaymentGateway2"}},
		{Name: "PaymentGatewayServiceTest.MockPaymentGatewayWithTwoPhase", Namespace: "NU", SuperClass: "MockPaymentGateway"},
		{Name: "PaymentGatewayFactory", Namespace: "NU", Access: "global", Methods: map[string]Method{
			"make": {Name: "PaymentGatewayFactory.make", ClassName: "PaymentGatewayFactory", Access: "global", IsStatic: true, ReturnType: "Object", Program: makeProgram},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListUpcastUsesUserClassWhenElementNameShadowsSObject(t *testing.T) {
	program, err := CompileAnonymous(`
List<AdjustmentOrder> adjustments = new List<AdjustmentOrder>();
List<Order> orders = (List<Order>) adjustments;
System.assertEquals(0, orders.size());
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Order", Namespace: "NU"},
		{Name: "AdjustmentOrder", Namespace: "NU", SuperClass: "Order"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapGetFindsNamespacedEnumKeyWithShortDeclaredType(t *testing.T) {
	program, err := CompileAnonymous(`
Map<ExprTokenType, Integer> precedences = new Map<ExprTokenType, Integer> {
    ExprTokenType.Equals => 3
};
System.assertEquals(3, precedences.get(ExprTokenType.Equals));
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ExprTokenType", Namespace: "NU", EnumValues: []string{"Equals"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNewInstanceAllowsNamespacedGenericCollection(t *testing.T) {
	program, err := CompileAnonymous(`
Type itemType = Type.forName('namz', 'PriceAdjustment');
List<IPersistenceSupport> items = (List<IPersistenceSupport>) Type.forName('List<' + itemType.getName() + '>').newInstance();
System.assertEquals(0, items.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "IPersistenceSupport", Namespace: "namz", IsInterface: true},
		{Name: "PriceAdjustment", Namespace: "namz", Interfaces: []string{"IPersistenceSupport"}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNewInstanceAbstractClassIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Type t = Type.forName('AbstractThing');
try {
    Object value = t.newInstance();
    System.assert(false, 'expected TypeException');
} catch (Exception e) {
    System.assert(e.getMessage().contains('cannot instantiate abstract class'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AbstractThing", IsAbstract: true}); err != nil {
		t.Fatal(err)
	}
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

func TestExecDatabaseAsyncDMLRejectsNonCallbackBeforeMutation(t *testing.T) {
	program, err := CompileAnonymous(`
Database.insertAsync(new Account(Name = 'Bad Callback'), new Account(Name = 'Not Callback'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	before := len(org.Objects["Account"].Records)
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	if err == nil {
		t.Fatal("expected async DML callback validation error")
	}
	if !strings.Contains(err.Error(), "DataSource.AsyncSaveCallback") {
		t.Fatalf("error = %v, want DataSource.AsyncSaveCallback", err)
	}
	if after := len(org.Objects["Account"].Records); after != before {
		t.Fatalf("Account rows after invalid callback = %d, want %d", after, before)
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

func TestExecDatabaseCursorWithBinds(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme', Rating = 'Hot');
insert new Account(Name = 'Beta', Rating = 'Warm');
Map<String,Object> binds = new Map<String,Object>();
binds.put('rating', 'Warm');

Database.Cursor cursor = Database.getCursorWithBinds('SELECT Id, Name FROM Account WHERE Rating = :rating ORDER BY Name', binds, null);
System.assertEquals(1, cursor.getNumRecords());
List<SObject> rows = cursor.fetch(0, 10);
System.assertEquals(1, rows.size());
System.assertEquals('Beta', rows.get(0).get('Name'));

Database.PaginationCursor page = Database.getPaginationCursorWithBinds('SELECT Id, Name FROM Account WHERE Rating = :rating ORDER BY Name', binds, null);
Database.CursorFetchResult pageResult = page.fetchPage(0, 10);
System.assertEquals(1, pageResult.getRecords().size());
System.assertEquals(true, pageResult.isDone());
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
System.assertEquals('2026-01-02 03:04:05', row.CreatedDate.formatGmt('yyyy-MM-dd HH:mm:ss'));
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
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
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

func TestExecBatchAllowsConcreteSObjectScopeForSObjectInterface(t *testing.T) {
	startProgram, err := CompileAnonymous(`return Database.getQueryLocator('SELECT Id, Name FROM Account');`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
for (Account account : scope) {
	account.Name = account.Name + ' edited';
}
update scope;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
insert new Account(Name = 'scope');
Test.startTest();
Database.executeBatch(new BatchWorker(), 200);
Test.stopTest();
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'scope edited']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Database.QueryLocator", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Account>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStopTestDrainsQueueableEnqueuedByBatch(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'scope') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`System.enqueueJob(new QueueWorker());`)
	if err != nil {
		t.Fatal(err)
	}
	queueProgram, err := CompileAnonymous(`insert new Account(Name = 'queueable from batch');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new BatchWorker(), 200);
Test.stopTest();
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'queueable from batch'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "QueueWorker",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {Name: "QueueWorker.execute", ClassName: "QueueWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBatchExecuteChunksResetGovernorLimits(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'one'), new Account(Name = 'two'), new Account(Name = 'three') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`insert new Account(Name = 'chunk');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new BatchWorker(), 1);
Test.stopTest();
System.assertEquals(3, [SELECT COUNT() FROM Account WHERE Name = 'chunk']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	caps := defaultLimitCaps()
	caps.DMLStatements = 1
	machine.SetLimitCaps(caps)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBatchRecordsWorkerJobsForChunks(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Id = '001000000000001', Name = 'one'), new Account(Id = '001000000000002', Name = 'two'), new Account(Id = '001000000000003', Name = 'three') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`insert new Account(Name = 'chunk');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
String jobId = Database.executeBatch(new BatchWorker(), 2);
Test.stopTest();
System.assertEquals(1, [SELECT COUNT() FROM AsyncApexJob WHERE Id = :jobId AND JobType = 'BatchApex' AND Status = 'Completed']);
System.assertEquals(2, [SELECT COUNT() FROM AsyncApexJob WHERE ParentJobId = :jobId AND JobType = 'BatchApexWorker' AND Status = 'Completed']);
System.assertEquals(1, [SELECT COUNT() FROM AsyncApexJob WHERE ParentJobId = :jobId AND LastProcessed = '001000000000003' AND LastProcessedOffset = 3]);
System.assertEquals(1, [SELECT COUNT() FROM AsyncApexJob WHERE ParentJobId = :jobId AND LastProcessed = '001000000000002' AND LastProcessedOffset = 2]);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFailedBatchChunkRollsBackChunkDML(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<Integer>{1, 2};`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
Integer value = scope.get(0);
insert new Account(Name = 'chunk-' + value);
if (value == 2) {
	throw new Exception('second chunk failed');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	Test.startTest();
	Database.executeBatch(new BatchWorker(), 1);
	Test.stopTest();
} catch (Exception e) {
}
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'chunk-1']);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'chunk-2']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<Integer>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNonStatefulBatchResetsInstanceFieldsBetweenTransactions(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<Integer>{1, 2};`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
Counter = Counter + 1;
insert new Account(Name = 'non-stateful-count-' + Counter);
`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`insert new Account(Name = 'non-stateful-finish-' + Counter);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new NonStatefulBatch(), 1);
Test.stopTest();
System.assertEquals(2, [SELECT COUNT() FROM Account WHERE Name = 'non-stateful-count-1']);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'non-stateful-count-2']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'non-stateful-finish-0']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "NonStatefulBatch",
		Interfaces: []string{"Database.Batchable<Integer>"},
		Fields: map[string]Field{
			"Counter": {Name: "Counter", Type: "Integer", Value: Int(0), InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"start":   {Name: "NonStatefulBatch.start", ClassName: "NonStatefulBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "NonStatefulBatch.execute", ClassName: "NonStatefulBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: executeProgram},
			"finish":  {Name: "NonStatefulBatch.finish", ClassName: "NonStatefulBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStatefulBatchPreservesInstanceFieldsBetweenTransactions(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<Integer>{1, 2};`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
Counter = Counter + 1;
insert new Account(Name = 'stateful-count-' + Counter);
`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`insert new Account(Name = 'stateful-finish-' + Counter);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new StatefulBatch(), 1);
Test.stopTest();
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'stateful-count-1']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'stateful-count-2']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'stateful-finish-2']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "StatefulBatch",
		Interfaces: []string{"Database.Batchable<Integer>", "Database.Stateful"},
		Fields: map[string]Field{
			"Counter": {Name: "Counter", Type: "Integer", Value: Int(0), InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"start":   {Name: "StatefulBatch.start", ClassName: "StatefulBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "StatefulBatch.execute", ClassName: "StatefulBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: executeProgram},
			"finish":  {Name: "StatefulBatch.finish", ClassName: "StatefulBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBatchFinishInheritedSchedulerCanEnqueueQueueable(t *testing.T) {
	parentStart, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	parentFinish, err := CompileAnonymous(`new ChildScheduler().ensureScheduled();`)
	if err != nil {
		t.Fatal(err)
	}
	ensureScheduled, err := CompileAnonymous(`
if (testShouldSchedule && !typesAlreadyScheduledFor.contains(ChildScheduler.class)) {
	typesAlreadyScheduledFor.add(ChildScheduler.class);
	System.enqueueJob(new QueueWorker());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	queueExecute, err := CompileAnonymous(`Database.executeBatch(new FollowupBatch(), 200);`)
	if err != nil {
		t.Fatal(err)
	}
	followupStart, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
BaseScheduler.testShouldSchedule = true;
new ChildScheduler().ensureScheduled();
Test.startTest();
Database.executeBatch(new ParentBatch(), 200);
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
		Name: "BaseScheduler",
		StaticFields: map[string]Field{
			"testShouldSchedule":       {Name: "testShouldSchedule", Type: "Boolean", Static: true, Value: Bool(false), InitialValue: Bool(false)},
			"typesAlreadyScheduledFor": {Name: "typesAlreadyScheduledFor", Type: "Set<Type>", Static: true, Value: Set(), InitialValue: Set()},
		},
		Methods: map[string]Method{
			"ensureScheduled": {Name: "BaseScheduler.ensureScheduled", ClassName: "BaseScheduler", ReturnType: "void", Program: ensureScheduled},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "ChildScheduler", SuperClass: "BaseScheduler"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ParentBatch",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "ParentBatch.start", ClassName: "ParentBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: parentStart},
			"execute": {Name: "ParentBatch.execute", ClassName: "ParentBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}},
			"finish":  {Name: "ParentBatch.finish", ClassName: "ParentBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: parentFinish},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "QueueWorker",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {Name: "QueueWorker.execute", ClassName: "QueueWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueExecute},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "FollowupBatch",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "FollowupBatch.start", ClassName: "FollowupBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: followupStart},
			"execute": {Name: "FollowupBatch.execute", ClassName: "FollowupBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}},
			"finish":  {Name: "FollowupBatch.finish", ClassName: "FollowupBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	jobs := org.Objects["AsyncApexJob"].Records
	if len(jobs) != 4 {
		t.Fatalf("AsyncApexJob records = %d, want 4: %#v", len(jobs), jobs)
	}
}

func TestExecAsyncJobsDoNotMutateCallerInstances(t *testing.T) {
	batchStart, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'scope') };`)
	if err != nil {
		t.Fatal(err)
	}
	batchExecute, err := CompileAnonymous(`
this.Marker = 'batch async';
insert new Account(Name = this.Marker);
`)
	if err != nil {
		t.Fatal(err)
	}
	queueExecute, err := CompileAnonymous(`
this.Marker = 'queue async';
insert new Account(Name = this.Marker);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
BatchWorker batch = new BatchWorker();
batch.Marker = 'caller batch';
QueueWorker queueable = new QueueWorker();
queueable.Marker = 'caller queue';
Test.startTest();
Database.executeBatch(batch, 1);
System.enqueueJob(queueable);
Test.stopTest();
System.assertEquals('caller batch', batch.Marker);
System.assertEquals('caller queue', queueable.Marker);
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'batch async'].size());
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'queue async'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Fields: map[string]Field{
			"Marker": {Name: "Marker", Type: "String"},
		},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: batchStart},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: batchExecute},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "QueueWorker",
		Fields: map[string]Field{
			"Marker": {Name: "Marker", Type: "String"},
		},
		Methods: map[string]Method{
			"execute": {Name: "QueueWorker.execute", ClassName: "QueueWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueExecute},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStopTestDrainsOnlyJobsEnqueuedAfterStartTest(t *testing.T) {
	preStartProgram, err := CompileAnonymous(`insert new Account(Name = 'pre-start async');`)
	if err != nil {
		t.Fatal(err)
	}
	insideProgram, err := CompileAnonymous(`insert new Account(Name = 'inside async');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.enqueueJob(new PreStartWorker());
Test.startTest();
System.enqueueJob(new InsideWorker());
Test.stopTest();
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'pre-start async'].size());
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'inside async'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "PreStartWorker",
		Methods: map[string]Method{
			"execute": {Name: "PreStartWorker.execute", ClassName: "PreStartWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: preStartProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "InsideWorker",
		Methods: map[string]Method{
			"execute": {Name: "InsideWorker.execute", ClassName: "InsideWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: insideProgram},
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
	AND ApexClass.NamespacePrefix = null
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
	for _, class := range []Class{{Name: "FutureWorker"}, {Name: "QueueWorker"}, {Name: "BatchWorker", Interfaces: []string{"Database.Batchable<SObject>"}}, {Name: "ScheduledWorker"}} {
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
	if err := machine.RegisterClass(Class{Name: "BatchWorker", Interfaces: []string{"Database.Batchable<SObject>"}}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "scope size must be at most 2000") {
		t.Fatalf("err = %v, want scope maximum", err)
	}
}

func TestExecExecuteBatchRejectsNonBatchableObject(t *testing.T) {
	program, err := CompileAnonymous(`Database.executeBatch(new NotBatch(), 1);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "NotBatch"}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "expects Batchable object") {
		t.Fatalf("err = %v, want Batchable object rejection", err)
	}
}

func TestExecExecuteBatchRejectsStructuralBatchShape(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`Database.executeBatch(new StructuralBatch(), 1);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "StructuralBatch",
		Methods: map[string]Method{
			"start":   {Name: "StructuralBatch.start", ClassName: "StructuralBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "StructuralBatch.execute", ClassName: "StructuralBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "expects Batchable object") {
		t.Fatalf("err = %v, want Batchable object rejection", err)
	}
}

func TestExecScheduleBatchRejectsNonBatchableObject(t *testing.T) {
	program, err := CompileAnonymous(`System.scheduleBatch(new NotBatch(), 'nightly', 1, 200);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "NotBatch"}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "expects Batchable object") {
		t.Fatalf("err = %v, want Batchable object rejection", err)
	}
}

func TestExecExecuteBatchRejectsIncompleteBatchableContract(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<Integer>{1};`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		methods map[string]Method
		want    string
	}{
		{
			name: "MissingStartBatch",
			methods: map[string]Method{
				"execute": {Name: "MissingStartBatch.execute", ClassName: "MissingStartBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: executeProgram},
				"finish":  {Name: "MissingStartBatch.finish", ClassName: "MissingStartBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: executeProgram},
			},
			want: "missing start",
		},
		{
			name: "MissingExecuteBatch",
			methods: map[string]Method{
				"start":  {Name: "MissingExecuteBatch.start", ClassName: "MissingExecuteBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
				"finish": {Name: "MissingExecuteBatch.finish", ClassName: "MissingExecuteBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: executeProgram},
			},
			want: "missing execute",
		},
		{
			name: "MissingFinishBatch",
			methods: map[string]Method{
				"start":   {Name: "MissingFinishBatch.start", ClassName: "MissingFinishBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
				"execute": {Name: "MissingFinishBatch.execute", ClassName: "MissingFinishBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: executeProgram},
			},
			want: "missing finish",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(`Database.executeBatch(new ` + tt.name + `(), 1);`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: tt.name, Interfaces: []string{"Database.Batchable<Integer>"}, Methods: tt.methods}); err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			err = machine.DrainAsync(&result)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestExecExecuteBatchRejectsInvalidBatchableSignatures(t *testing.T) {
	listStartProgram, err := CompileAnonymous(`return new List<Integer>{1};`)
	if err != nil {
		t.Fatal(err)
	}
	stringStartProgram, err := CompileAnonymous(`return 'not iterable';`)
	if err != nil {
		t.Fatal(err)
	}
	voidProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	stringFinishProgram, err := CompileAnonymous(`return 'done';`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		methods map[string]Method
		want    string
	}{
		{
			name: "WrongStartReturnBatch",
			methods: map[string]Method{
				"start":   {Name: "WrongStartReturnBatch.start", ClassName: "WrongStartReturnBatch", ReturnType: "String", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: stringStartProgram},
				"execute": {Name: "WrongStartReturnBatch.execute", ClassName: "WrongStartReturnBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: voidProgram},
				"finish":  {Name: "WrongStartReturnBatch.finish", ClassName: "WrongStartReturnBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: voidProgram},
			},
			want: "invalid start",
		},
		{
			name: "ScopeOnlyExecuteBatch",
			methods: map[string]Method{
				"start":   {Name: "ScopeOnlyExecuteBatch.start", ClassName: "ScopeOnlyExecuteBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: listStartProgram},
				"execute": {Name: "ScopeOnlyExecuteBatch.execute", ClassName: "ScopeOnlyExecuteBatch", ReturnType: "void", Params: []Param{{Name: "scope", Type: "List<Integer>"}}, Program: voidProgram},
				"finish":  {Name: "ScopeOnlyExecuteBatch.finish", ClassName: "ScopeOnlyExecuteBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: voidProgram},
			},
			want: "invalid execute",
		},
		{
			name: "NoArgExecuteBatch",
			methods: map[string]Method{
				"start":   {Name: "NoArgExecuteBatch.start", ClassName: "NoArgExecuteBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: listStartProgram},
				"execute": {Name: "NoArgExecuteBatch.execute", ClassName: "NoArgExecuteBatch", ReturnType: "void", Program: voidProgram},
				"finish":  {Name: "NoArgExecuteBatch.finish", ClassName: "NoArgExecuteBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: voidProgram},
			},
			want: "invalid execute",
		},
		{
			name: "WrongFinishReturnBatch",
			methods: map[string]Method{
				"start":   {Name: "WrongFinishReturnBatch.start", ClassName: "WrongFinishReturnBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: listStartProgram},
				"execute": {Name: "WrongFinishReturnBatch.execute", ClassName: "WrongFinishReturnBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: voidProgram},
				"finish":  {Name: "WrongFinishReturnBatch.finish", ClassName: "WrongFinishReturnBatch", ReturnType: "String", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: stringFinishProgram},
			},
			want: "invalid finish",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(`Database.executeBatch(new ` + tt.name + `(), 1);`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: tt.name, Interfaces: []string{"Database.Batchable<Integer>"}, Methods: tt.methods}); err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			err = machine.DrainAsync(&result)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
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

func TestExecScheduledApexCronJobDetailUsesScheduledApexType(t *testing.T) {
	program, err := CompileAnonymous(`
String scheduleId = System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
List<CronTrigger> rows = [
	SELECT Id, NextFireTime, CronJobDetail.Name, CronJobDetail.JobType
	FROM CronTrigger
	WHERE CronJobDetail.Name = 'nightly' AND CronJobDetail.JobType = '7'
];
System.assertEquals(1, rows.size());
System.assertEquals(scheduleId, rows[0].Id);
System.assertEquals(Date.today() + 1, rows[0].NextFireTime.date());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "ScheduledWorker", Interfaces: []string{"Schedulable"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAbortJobAcceptsCronTriggerIDValue(t *testing.T) {
	program, err := CompileAnonymous(`
Id scheduleId = System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
System.abortJob(scheduleId);
List<AsyncApexJob> rows = [
	SELECT Id
	FROM AsyncApexJob
	WHERE JobType = 'ScheduledApex' AND Status = 'Aborted'
];
System.assertEquals(1, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "ScheduledWorker", Interfaces: []string{"Schedulable"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStopTestAllowsScheduledApexToEnqueueMultipleQueueables(t *testing.T) {
	scheduledProgram, err := CompileAnonymous(`
System.enqueueJob(new FirstWorker());
System.enqueueJob(new SecondWorker());
`)
	if err != nil {
		t.Fatal(err)
	}
	firstProgram, err := CompileAnonymous(`insert new Account(Name = 'first queueable ran');`)
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous(`insert new Account(Name = 'second queueable ran');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
System.schedule('nightly', '0 0 0 * * ?', new ScheduledWorker());
Test.stopTest();
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'first queueable ran'].size());
System.assertEquals(0, [SELECT Id FROM Account WHERE Name = 'second queueable ran'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "ScheduledWorker",
		Interfaces: []string{"Schedulable"},
		Methods: map[string]Method{
			"execute": {Name: "ScheduledWorker.execute", ClassName: "ScheduledWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, Program: scheduledProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, worker := range []struct {
		name    string
		program ir.Program
	}{
		{name: "FirstWorker", program: firstProgram},
		{name: "SecondWorker", program: secondProgram},
	} {
		if err := machine.RegisterClass(Class{
			Name:       worker.name,
			Interfaces: []string{"Queueable"},
			Methods: map[string]Method{
				"execute": {Name: worker.name + ".execute", ClassName: worker.name, ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: worker.program},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStopTestRejectsMultipleQueueableChildrenFromQueueable(t *testing.T) {
	queueProgram, err := CompileAnonymous(`
System.enqueueJob(new FirstWorker());
System.enqueueJob(new SecondWorker());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
System.enqueueJob(new ParentWorker());
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "ParentWorker",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {Name: "ParentWorker.execute", ClassName: "ParentWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: queueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"FirstWorker", "SecondWorker"} {
		if err := machine.RegisterClass(Class{Name: name, Interfaces: []string{"Queueable"}}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Queueable chaining limit exceeded") {
		t.Fatalf("err = %v, want queueable chaining limit", err)
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
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
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

func TestExecBatchWithoutFinishRecordsCompletedJobCounts(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'one'), new Account(Name = 'two'), new Account(Name = 'three') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`insert new Account(Name = 'chunk ran');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
String jobId = Database.executeBatch(new BatchWorker(), 1);
Test.stopTest();
AsyncApexJob job = [
	SELECT Id, Status, TotalJobItems, JobItemsProcessed, NumberOfErrors, CompletedDate
	FROM AsyncApexJob
	WHERE Id = :jobId
];
System.assertEquals('Completed', job.Status);
System.assertEquals(3, job.TotalJobItems);
System.assertEquals(3, job.JobItemsProcessed);
System.assertEquals(0, job.NumberOfErrors);
System.assertNotEquals(null, job.CompletedDate);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecFailedBatchRecordsProcessedChunksBeforeError(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'one'), new Account(Name = 'two') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
if ([SELECT COUNT() FROM Account WHERE Name = 'chunk ran'] > 0) {
	throw new Exception('second chunk failed');
}
insert new Account(Name = 'chunk ran');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String jobId;
try {
	Test.startTest();
	jobId = Database.executeBatch(new BatchWorker(), 1);
	Test.stopTest();
} catch (Exception e) {
}
AsyncApexJob job = [
	SELECT Id, Status, TotalJobItems, JobItemsProcessed, NumberOfErrors, ExtendedStatus, CompletedDate
	FROM AsyncApexJob
	WHERE Id = :jobId
];
System.assertEquals('Failed', job.Status);
System.assertEquals(2, job.TotalJobItems);
System.assertEquals(1, job.JobItemsProcessed);
System.assertEquals(1, job.NumberOfErrors);
System.assert(job.ExtendedStatus.contains('second chunk failed'));
System.assertNotEquals(null, job.CompletedDate);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
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
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: executeProgram},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
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

func TestExecFailedBatchFinishRecordsFailedJobCounts(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<SObject>{ new Account(Name = 'one'), new Account(Name = 'two') };`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`insert new Account(Name = 'finish failure chunk');`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`
insert new Account(Name = 'finish before failure');
throw new Exception('finish failed');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String jobId;
try {
	Test.startTest();
	jobId = Database.executeBatch(new BatchWorker(), 1);
	Test.stopTest();
} catch (Exception e) {
}
AsyncApexJob job = [
	SELECT Id, Status, TotalJobItems, JobItemsProcessed, NumberOfErrors, ExtendedStatus, CompletedDate
	FROM AsyncApexJob
	WHERE Id = :jobId
];
System.assertEquals('Failed', job.Status);
System.assertEquals(2, job.TotalJobItems);
System.assertEquals(2, job.JobItemsProcessed);
System.assertEquals(1, job.NumberOfErrors);
System.assert(job.ExtendedStatus.contains('finish failed'));
System.assertNotEquals(null, job.CompletedDate);
System.assertEquals(2, [SELECT COUNT() FROM Account WHERE Name = 'finish failure chunk']);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'finish before failure']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
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

func TestExecBatchApexErrorEventListGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = new List<SObject>{
	new BatchApexErrorEvent(Phase = 'EXECUTE', JobScope = '001000000000001')
};
System.assertEquals('BatchApexErrorEvent', records.getSObjectType().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.ensureBatchApexErrorEventObject()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecQueryLocatorChunkIteratorDefaultConstructorIsEmptyIterator(t *testing.T) {
	program, err := CompileAnonymous(`
Database.QueryLocatorChunkIterator iterator = new Database.QueryLocatorChunkIterator();
System.assertEquals(false, iterator.hasNext());
try {
	iterator.next();
	System.assert(false, 'empty chunk iterator should throw on next');
} catch (System.NoSuchElementException e) {
	System.assert(e.getMessage().contains('Iterator has no more elements'));
}
Database.QueryLocatorChunkIterator copied = (Database.QueryLocatorChunkIterator)iterator.clone();
System.assertEquals(false, copied.hasNext());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
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

func TestRecordAsyncJobSplitsNamespacedApexClassRows(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	machine.SetOrg(&org)
	job := AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("verifiable.ActionProcessorQueueable")}

	machine.recordAsyncJob(job, "Queued", "")

	apexClass := machine.Org.Objects["ApexClass"].Records[storage.ID(asyncApexClassID("verifiable.ActionProcessorQueueable"))]
	if apexClass.Fields["Name"].String != "ActionProcessorQueueable" {
		t.Fatalf("ApexClass.Name = %#v", apexClass.Fields["Name"])
	}
	if apexClass.Fields["NamespacePrefix"].String != "verifiable" {
		t.Fatalf("ApexClass.NamespacePrefix = %#v", apexClass.Fields["NamespacePrefix"])
	}
	asyncJob := machine.Org.Objects["AsyncApexJob"].Records[storage.ID(job.ID)]
	if asyncJob.Fields["ApexClassId"].ID != apexClass.ID {
		t.Fatalf("AsyncApexJob.ApexClassId = %s, want %s", asyncJob.Fields["ApexClassId"].ID, apexClass.ID)
	}
}

func TestRecordAsyncJobUsesRegisteredClassNamespace(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "ActionProcessorQueueable", Namespace: "verifiable"}); err != nil {
		t.Fatal(err)
	}
	job := AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("ActionProcessorQueueable")}

	machine.recordAsyncJob(job, "Queued", "")

	apexClass := machine.Org.Objects["ApexClass"].Records[storage.ID(asyncApexClassID("ActionProcessorQueueable"))]
	if apexClass.Fields["Name"].String != "ActionProcessorQueueable" {
		t.Fatalf("ApexClass.Name = %#v", apexClass.Fields["Name"])
	}
	if apexClass.Fields["NamespacePrefix"].String != "verifiable" {
		t.Fatalf("ApexClass.NamespacePrefix = %#v", apexClass.Fields["NamespacePrefix"])
	}
}

func TestRecordAsyncJobReusesExistingApexClassRow(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "verifiable"
	org.Objects["ApexClass"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "ApexClass"},
		Records: map[storage.ID]storage.Record{
			"01pExistingAAA": {
				ID:     "01pExistingAAA",
				Object: "ApexClass",
				Fields: map[string]storage.Value{
					"Name":            storage.StringValue("ActionProcessorQueueable"),
					"NamespacePrefix": storage.StringValue("verifiable"),
				},
			},
		},
	}
	machine.SetOrg(&org)
	job := AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("ActionProcessorQueueable")}

	machine.recordAsyncJob(job, "Queued", "")

	asyncJob := machine.Org.Objects["AsyncApexJob"].Records[storage.ID(job.ID)]
	if asyncJob.Fields["ApexClassId"].ID != "01pExistingAAA" {
		t.Fatalf("AsyncApexJob.ApexClassId = %s, want existing ApexClass row", asyncJob.Fields["ApexClassId"].ID)
	}
	if got := len(machine.Org.Objects["ApexClass"].Records); got != 1 {
		t.Fatalf("ApexClass rows = %d, want no duplicate rows", got)
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
		Name:       "BatchWorker",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "BatchWorker.start", ClassName: "BatchWorker", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "BatchWorker.execute", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}},
			"finish":  {Name: "BatchWorker.finish", ClassName: "BatchWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBatchEnqueuedFromFinishDoesNotSeePreviousBatchAsRunning(t *testing.T) {
	firstStart, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	firstFinish, err := CompileAnonymous(`Database.executeBatch(new SecondBatch(), 200);`)
	if err != nil {
		t.Fatal(err)
	}
	secondStart, err := CompileAnonymous(`
List<AsyncApexJob> running = [
	SELECT Id
	FROM AsyncApexJob
	WHERE CompletedDate = null
	AND JobType = 'BatchApex'
	AND Id != :context.getJobId()
];
System.assertEquals(0, running.size());
return new List<SObject>{ new Account(Name = 'scope') };
`)
	if err != nil {
		t.Fatal(err)
	}
	secondExecute, err := CompileAnonymous(`insert new Account(Name = 'second batch ran');`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new FirstBatch(), 200);
Test.stopTest();
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'second batch ran'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "FirstBatch",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "FirstBatch.start", ClassName: "FirstBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: firstStart},
			"execute": {Name: "FirstBatch.execute", ClassName: "FirstBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}},
			"finish":  {Name: "FirstBatch.finish", ClassName: "FirstBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: firstFinish},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "SecondBatch",
		Interfaces: []string{"Database.Batchable<SObject>"},
		Methods: map[string]Method{
			"start":   {Name: "SecondBatch.start", ClassName: "SecondBatch", ReturnType: "Iterable<SObject>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: secondStart},
			"execute": {Name: "SecondBatch.execute", ClassName: "SecondBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}, Program: secondExecute},
			"finish":  {Name: "SecondBatch.finish", ClassName: "SecondBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBatchFinishDoesNotReenqueueAfterInstanceCursorCleared(t *testing.T) {
	startProgram, err := CompileAnonymous(`return new List<Integer>{1};`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
this.nextCursor = 'cursor';
this.nextCursor = null;
insert new Account(Name = 'cursor batch ran');
`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`
if (String.isNotBlank(this.nextCursor)) {
	Database.executeBatch(this, 1);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
Database.executeBatch(new CursorBatch(), 1);
Test.stopTest();
System.assertEquals(1, [SELECT Id FROM Account WHERE Name = 'cursor batch ran'].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "CursorBatch",
		Interfaces: []string{"Database.Batchable<Integer>"},
		Fields: map[string]Field{
			"nextCursor": {Name: "nextCursor", Type: "String"},
		},
		Methods: map[string]Method{
			"start":   {Name: "CursorBatch.start", ClassName: "CursorBatch", ReturnType: "Iterable<Integer>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: startProgram},
			"execute": {Name: "CursorBatch.execute", ClassName: "CursorBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Integer>"}}, Program: executeProgram},
			"finish":  {Name: "CursorBatch.finish", ClassName: "CursorBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: finishProgram},
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

func TestExecFinalizerContextMethods(t *testing.T) {
	program, err := CompileAnonymous(`
FinalizerContext fc = new FinalizerContext();
System.assertEquals('', fc.getAsyncApexJobId());
System.assertEquals('', fc.getRequestId());
System.assertEquals('SUCCESS', fc.getResult().name());
System.assertEquals(null, fc.getException());
System.FinalizerContext systemContext = new System.FinalizerContext();
System.assertEquals('SUCCESS', systemContext.getResult().name());
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
  System.assertEquals('00D000000000001EAA', UserInfo.getOrganizationId());
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

func TestExecNetworkCommunitiesLandingIsEmptyInTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = Network.communitiesLanding();
System.assertEquals('', page.getUrl());
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
System.assertEquals('Feb 29, 2024', winter.format('MMM dd, YYYY'));
System.assertEquals('2024-07-01 05:00:00 -0700 PDT', summer.format('yyyy-MM-dd HH:mm:ss Z z'));
System.assertEquals('2/29/2024, 3:05 PM', winter.format());
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
System.assertEquals('7/1/2024, 8:00 AM', summer.format());
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
System.assertEquals('7/1/2024, 6:00 AM', summer.format());
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
System.assertEquals('2024-03-01 07:05:06', winterLocal.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('2/29/2024, 11:05 PM', winterLocal.format());
System.assertEquals('2024-02-29', String.valueOf(winterLocal.date()));
System.assertEquals('2024-03-01', String.valueOf(winterLocal.dateGmt()));
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
	System.assertEquals('2024-07-01 12:30:00.250', fromDateTime.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
	System.assertEquals('7/1/2024, 5:30 AM', fromDateTime.format());
	Datetime parsedDefaultFormat = Datetime.parse('7/1/2024, 5:30 AM');
	System.assertEquals('2024-07-01 12:30:00', parsedDefaultFormat.formatGmt('yyyy-MM-dd HH:mm:ss'));
	String nullTimeZone;
	System.assertEquals('2024-07-01', fromDateTime.format('yyyy-MM-dd', nullTimeZone));
System.assertEquals(Date.newInstance(2024, 7, 1), fromDateTime.dateGMT());
Datetime fromDateTimeGmt = Datetime.newInstanceGmt(Date.newInstance(2024, 7, 1), Time.newInstance(5, 30, 0, 250));
System.assertEquals('2024-07-01 05:30:00.250', fromDateTimeGmt.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));

Datetime fromMillis = Datetime.newInstance(winterLocal.getTime());
System.assertEquals(winterLocal, fromMillis);
System.assertEquals(0, Datetime.newInstance(0).getTime());

Datetime gap = Datetime.newInstance(2024, 3, 10, 2, 30, 0);
System.assertEquals('2024-03-10 10:30:00', gap.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('3/10/2024, 3:30 AM', gap.format());
System.assertEquals(3, gap.hour());
System.assertEquals(10, gap.hourGmt());

Datetime overlap = Datetime.newInstance(2024, 11, 3, 1, 30, 0);
System.assertEquals('2024-11-03 08:30:00', overlap.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('11/3/2024, 1:30 AM', overlap.format());
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
    System.assertEquals('2024-07-01 12:00:00', stamp.formatGmt('yyyy-MM-dd HH:mm:ss'));
    System.assertEquals(8, stamp.hour());
    System.assertEquals(12, stamp.hourGmt());
    System.assertEquals('2024-07-01', String.valueOf(stamp.date()));
}
System.runAs(new User(Id = '005-panama-user', TimeZoneSidKey = 'America/Panama')) {
    Datetime stamp = Datetime.newInstance(Date.newInstance(2014, 11, 4), Time.newInstance(0, 0, 0, 0));
    System.assertEquals('2014-11-04 05:00:00', stamp.formatGmt('yyyy-MM-dd HH:mm:ss'));
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

func TestExecSitePasswordAndExperienceContracts(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('', Site.getExperienceId());
Site.setExperienceId('LocalExperience001');
System.assertEquals('LocalExperience001', Site.getExperienceId());
System.assertEquals(true, Site.forgotPassword('user@example.invalid'));
System.assertEquals(true, Site.forgotPassword('user@example.invalid', 'ResetTemplate'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if machine.siteExperienceID != "LocalExperience001" {
		t.Fatalf("siteExperienceID = %q, want LocalExperience001", machine.siteExperienceID)
	}
	machine.ResetApexPageState()
	if machine.siteExperienceID != "" {
		t.Fatalf("ResetApexPageState kept siteExperienceID = %q", machine.siteExperienceID)
	}
}

func TestExecOrgShapeBackedSiteNetworkAndCurrencyCalls(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(UserInfo.isMultiCurrencyOrganization());
System.assertEquals('0DM000000000001', Site.getSiteId());
System.assertEquals('https://local.glade.example/local', Site.getBaseUrl());
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
User personUser = new User(Username='person@example.invalid', LastName='Person', Email='person@example.invalid', Alias='pers');
System.assertEquals('005000000000E01', Site.createPersonAccountPortalUser(personUser, '005000000000001', 'secret'));
System.assertEquals('005000000000E01', personUser.Id);
System.assertEquals('/passwordless', Site.passwordlessLogin(UserInfo.getUserId(), new List<Auth.VerificationMethod>(), '/passwordless').getUrl());
Site.setPortalUserAsAuthProvider(personUser, '001000000000001');
User portalUser = new User(Username='portal@example.invalid', LastName='Portal', Email='portal@example.invalid', Alias='port');
System.assertEquals('005000000000E01', Site.createPortalUser(portalUser, '001000000000001', 'secret'));
System.assertEquals('/next', Site.login('external@example.invalid', 'secret', '/next').getUrl());
Site.validatePassword(externalUser, 'secret', 'secret');
System.assertEquals('0DB000000000001', Network.getNetworkId());
System.assertEquals('https://local.glade.example/local/login', Network.getLoginUrl(Network.getNetworkId()));
System.assertEquals('/local', Network.communitiesLanding().getUrl());
System.assertEquals('/start', Network.forwardToAuthPage('/start').getUrl());
System.assertEquals('/start', Network.forwardToAuthPage('/start', 'Site').getUrl());
System.assertEquals('https://local.glade.example/local/secur/logout.jsp', Network.getLogoutUrl(Network.getNetworkId()));
System.assertEquals('https://local.glade.example/local/SelfRegister', Network.getSelfRegUrl(Network.getNetworkId()));
System.assertEquals('707000000000001', Network.createExternalUserAsync(externalUser, new Contact(), new Account()));
System.assertEquals('707000000000001', Network.createRecordAsync('selfRegistration', externalUser));
System.assertEquals(0, Network.loadAllPackageDefaultNetworkDashboardSettings());
System.assertEquals(0, Network.loadAllPackageDefaultNetworkPulseSettings());
System.assertEquals(0, Network.loadAllPackageDefaultNetworkWorkspaceMetricSettings());
	System.assertEquals('https://local.glade.example/local', ConnectApi.Communities.getCommunity(Network.getNetworkId()).siteUrl);
	System.assertNotEquals(null, ConnectApi.UserProfiles.getUserProfile(Network.getNetworkId(), UserInfo.getUserId()));
	System.assertNotEquals(null, ConnectApi.UserProfiles.getPhoto(Network.getNetworkId(), UserInfo.getUserId()));
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

func TestExecSiteCreateUserReturnsNullInTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
User externalUser = new User(Username='external@example.invalid', LastName='External', Email='external@example.invalid', Alias='ext');
System.assertEquals(null, Site.createExternalUser(externalUser, '001000000000001', 'secret', false));
System.assertEquals(null, Site.createPortalUser(externalUser, '001000000000001', 'secret'));
System.assertEquals(null, Site.createPersonAccountPortalUser(externalUser, '005000000000001', 'secret'));
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

func TestExecNetworkGetNetworkIdFallbackIsApexIDShaped(t *testing.T) {
	program, err := CompileAnonymous(`
String networkId = Network.getNetworkId();
System.assertEquals('0DB000000000001', networkId);
Id.valueOf(networkId);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsHonorsExplicitGuestUserType(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-guest-user', ProfileId = '00e000000000007', Username = 'guest@example.test', UserType = 'Guest')) {
	System.assertEquals('Guest', UserInfo.getUserType());
	System.assertEquals(true, Auth.CommunitiesUtil.isGuestUser());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsHydratesStoredGuestUserType(t *testing.T) {
	program, err := CompileAnonymous(`
System.runAs(new User(Id = '005-guest-user')) {
	System.assertEquals('Guest', UserInfo.getUserType());
	System.assertEquals(true, Auth.CommunitiesUtil.isGuestUser());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	userState := org.Objects["User"]
	userState.Records["005-guest-user"] = storage.Record{
		ID:     "005-guest-user",
		Object: "User",
		Fields: map[string]storage.Value{
			"UserType": storage.StringValue("Guest"),
		},
	}
	org.Objects["User"] = userState
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsDefaultsProfileOnlyUserTypeToStandard(t *testing.T) {
	program, err := CompileAnonymous(`
	Profile p = [SELECT Id FROM Profile WHERE Name = 'Internal Login User' LIMIT 1];
	System.runAs(new User(Id = '005-community-user', ProfileId = p.Id, Username = 'community@example.test')) {
		System.assertEquals('Standard', UserInfo.getUserType());
	}
	`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	profile := org.Objects["Profile"]
	profile.Records["00e000000000099"] = storage.Record{
		ID:     "00e000000000099",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Internal Login User"),
		},
	}
	org.Objects["Profile"] = profile
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsInfersCustomerCommunityUserTypeFromProfile(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'Customer Community User' LIMIT 1];
System.runAs(new User(Id = '005-community-user', ProfileId = p.Id, Username = 'community@example.test')) {
	System.assertEquals('CspLitePortal', UserInfo.getUserType());
}
Profile login = [SELECT Id FROM Profile WHERE Name = 'Customer Community Login User' LIMIT 1];
System.runAs(new User(Id = '005-community-login-user', ProfileId = login.Id, Username = 'community-login@example.test')) {
	System.assertEquals('CspLitePortal', UserInfo.getUserType());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	profile := org.Objects["Profile"]
	profile.Records["00e000000000099"] = storage.Record{
		ID:     "00e000000000099",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Customer Community User"),
		},
	}
	profile.Records["00e000000000098"] = storage.Record{
		ID:     "00e000000000098",
		Object: "Profile",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Customer Community Login User"),
		},
	}
	org.Objects["Profile"] = profile
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCreatedByProfileUserLicenseUsesCommunityLicenseKey(t *testing.T) {
	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name = 'Chatter External User' LIMIT 1];
System.runAs(new User(Id = '005-community-user', ProfileId = p.Id, Username = 'community@example.test')) {
	insert new Account(Name = 'Acme');
}
Account row = [SELECT CreatedBy.Profile.UserLicense.LicenseDefinitionKey FROM Account WHERE Name = 'Acme' LIMIT 1];
System.assertEquals('PID_Customer_Community_Login', row.CreatedBy.Profile.UserLicense.LicenseDefinitionKey);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunAsPreservesStaticSingletonInstanceFields(t *testing.T) {
	program, err := CompileAnonymous(`
Cache.Instance.DefaultAccount;
System.runAs(new User(Id = '005-community-user', UserType = 'CspLitePortal')) {
	System.assertNotEquals(null, Cache.Instance.DefaultAccount);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	getterProgram, err := CompileAnonymous(`
if (DefaultAccount == null) {
	DefaultAccount = new Account(Name = 'cached');
}
return DefaultAccount;
`)
	if err != nil {
		t.Fatal(err)
	}
	instanceGetterProgram, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new Cache();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Cache",
		StaticFields: map[string]Field{
			"Instance": {Name: "Instance", Type: "Cache", Static: true, Property: true, Getter: &Method{Name: "Cache.Instance.get", ClassName: "Cache", IsStatic: true, ReturnType: "Cache", Program: instanceGetterProgram}},
		},
		Fields: map[string]Field{
			"DefaultAccount": {Name: "DefaultAccount", Type: "Account", Property: true, Getter: &Method{Name: "Cache.DefaultAccount.get", ClassName: "Cache", ReturnType: "Account", Program: getterProgram}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecServiceRoutingLocalHarnesses(t *testing.T) {
	program, err := CompileAnonymous(`
Aura.redirect(new PageReference('/redirect'));
Site.UrlRewriter rewriter = new Site.UrlRewriter();
List<PageReference> rewritten = rewriter.generateUrlFor(new List<PageReference>{new PageReference('/one')});
System.assertEquals(1, rewritten.size());
System.assertEquals('/one', rewritten[0].getUrl());
System.assertEquals('/two', rewriter.mapRequestUrl(new PageReference('/two')).getUrl());
System.assertEquals('001000000000001', new ChatterAnswers.AccountCreator().createAccount('Ada', 'Lovelace', UserInfo.getUserId()));
LiveAgent.LiveAgentRealTimeSystem.cancelChatRequests(new List<String>{'request-1'});
LiveAgent.LiveAgentRealTimeSystem.setButtonStatus('button-1', true);
System.assertEquals(0, LiveAgent.LiveAgentRealTimeSystem.routeChatRequests(new List<LiveAgent.LiveChatRoutingRoute>()).size());
new LiveAgent.LiveChatRouter().doRouting(new List<LiveAgent.LiveChatRoutingRequest>());
System.assertEquals('', new Support.EinsteinBots().sendMessageToBot('bot', 'version', 'hello'));
System.assertEquals(null, new Support.EmailTemplateSelector().getDefaultEmailTemplateId('001000000000001'));
System.assertEquals(0, new Support.MilestoneTriggerTimeCalculator().calculateMilestoneTriggerTime('001000000000001', 'First_Response'));
System.assertEquals(0, Support.LifeScienceAttendees.parse('{}').attendees.size());
Support.LifeScienceUpdateEmailTransactions.updateRecords('[]');
System.assertEquals(null, new RichMessaging.ProcessFormHandler().processFormRequest(new RichMessaging.ProcessFormResponse(new Map<String,String>(), '0Mw000000000001', '0Mc000000000001', 'local', 'reply-1')));
System.assert(new RichMessaging.ProcessPaymentHandler().processPaymentRequest(new RichMessaging.ProcessPaymentRequest('txn-1', null, null, null, null, null, '001000000000001')) != null);
System.assert(new RichMessaging.ProcessCatalogOrderHandler().processCatalogOrderRequest(new RichMessaging.ProcessCatalogOrderRequest()) != null);
System.assert(new RichMessaging.AuthRequestHandler().handleAuthRequest(new RichMessaging.AuthRequestResponse('token', '001000000000001', 'provider')) != null);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformCallbackDefaultHarnesses(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(Process.SparkPlugApi.describePlugin('LocalPlugin') != null);
System.assertEquals(0, Process.SparkPlugApi.describePlugins().size());
System.assertEquals('{}', Process.SparkPlugApi.invokePluginWithJson('LocalPlugin', '{}'));
System.assertEquals('local-email-verification-token', TrailblazerIdentity.generateUserEmailVerificationToken('00D000000000001', UserInfo.getUserId(), 'local@example.invalid'));
System.assertEquals(0, TrailblazerIdentity.getUserOrgInfo(new List<String>{'local@example.invalid'}).size());
TrailblazerIdentity.splunkLog('local', 'message');
System.assertEquals(false, new TxnSecurity.EventCondition().evaluate(new Account()));
System.assertEquals(false, new TxnSecurity.PolicyCondition().evaluate(null));
new eventbus.EventPublishFailureCallback().onFailure(null);
new eventbus.EventPublishSuccessCallback().onSuccess(null);
System.assertEquals(0, new workflow.Action().invoke(null).size());
new workflow.ActionDml().invoke();
Social.DefaultInboundSocialPostHandler defaultHandler = new Social.DefaultInboundSocialPostHandler();
System.assertEquals(null, defaultHandler.createPersonaParent(null));
System.assertEquals('', defaultHandler.getCaseSubject(null));
System.assertEquals('', defaultHandler.getDefaultAccountId());
System.assertEquals(0, defaultHandler.getMaxNumberOfDaysClosedToReopenCase());
System.assertEquals('', defaultHandler.getPersonaFirstName(null));
System.assertEquals('', defaultHandler.getPersonaLastName(null));
System.assertEquals(0, defaultHandler.getPostTagsThatCreateCase().size());
System.assertEquals(false, defaultHandler.getUsingCaseAssignmentRule());
System.assert(defaultHandler.handleInboundSocialPost(null, null, new Map<String,Object>()) != null);
System.assert(new Social.InboundSocialPostHandler().handleInboundSocialPost(null, null, new Map<String,Object>()) != null);
System.assert(new Social.InboundSocialPostHandlerImpl().handleInboundSocialPost(null, null, new Map<String,Object>()) != null);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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
Test.setCurrentPage(new PageReference('/apex/current'));
PageReference current = ApexPages.currentPage();
System.assertEquals('/apex/current', current.getUrl());
URL base = URL.getSalesforceBaseUrl();
System.assertEquals('https://local.glade.example', base.toExternalForm());
URL orgUrl = URL.getOrgDomainUrl();
System.assertEquals('https://local.glade.example', orgUrl.toString());
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
System.assertEquals('2026-05-02', String.valueOf(d));
System.assertEquals(2026, d.year());
System.assertEquals(5, d.month());
System.assertEquals(2, d.day());
Date later = d.addDays(3);
System.assertEquals(3, d.daysBetween(later));
System.assertEquals(d.addDays(1), d.AddDays(1));
System.assertEquals(d.addDays(3), d + 3);
System.assertEquals(d.addDays(-2), d - 2);
Date nextMonth = d.addMonths(1);
System.assertEquals('2026-06-02', String.valueOf(nextMonth));
Date nextYear = d.addYears(1);
System.assertEquals('2027-05-02', String.valueOf(nextYear));
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
	System.assertEquals(parsedDate, Date.valueOf('2026-05-04 23:59:58-7'));
	System.assertEquals(parsedDate, Date.valueOf('2026-05-04 23:59:580'));
	System.assertEquals(parsedDate, Date.valueOf('2026-5-4'));
	System.assertEquals(parsedDate, Date.valueOf('2026-5-4 23:59:58'));
	System.assertEquals(parsedDate, Date.valueOf('2026-05-04,not-a-date'));
System.assertEquals(2026, Date.parse('01/01/2026').year());
System.assertEquals(2026, Date.parse('01/01/26').year());
Datetime dt = Datetime.now();
String dtText = dt.formatGmt('yyyy-MM-dd HH:mm:ss');
System.assert(dtText.startsWith('2026-05-02 12:00:00'));
Date dtDate = dt.date();
System.assertEquals('2026-05-02', String.valueOf(dtDate));
Datetime made = Datetime.newInstance(2026, 5, 2, 1, 2, 3);
System.assertEquals('2026-05-02 01:02:03', made.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals(1777683723000, made.getTime());
Datetime madePlusHour = made.addHours(1);
System.assertEquals('2026-05-02 02:02:03', madePlusHour.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime madePlusMinutes = made.addMinutes(2);
System.assertEquals('2026-05-02 01:04:03', madePlusMinutes.formatGmt('yyyy-MM-dd HH:mm:ss'));
	Datetime madePlusSeconds = made.addSeconds(3);
	System.assertEquals('2026-05-02 01:02:06', madePlusSeconds.formatGmt('yyyy-MM-dd HH:mm:ss'));
	Datetime madePlusDay = made.addDays(1);
	System.assertEquals('2026-05-03 01:02:03', madePlusDay.formatGmt('yyyy-MM-dd HH:mm:ss'));
	Datetime madePlusFractionalDay = made + (100000.0 / 86400000.0);
	System.assertEquals('2026-05-02 01:03:43', madePlusFractionalDay.formatGmt('yyyy-MM-dd HH:mm:ss'));
		Datetime parsedDt = Datetime.valueOf('2026-05-02 01:02:03');
	String madeText = made.formatGmt('yyyy-MM-dd HH:mm:ss');
	String parsedDtText = parsedDt.formatGmt('yyyy-MM-dd HH:mm:ss');
	System.assertEquals(madeText, parsedDtText);
	Object parsedDtObjectText = '2026-05-02 01:02:03';
	System.assertEquals(madeText, Datetime.valueOf(parsedDtObjectText).formatGmt('yyyy-MM-dd HH:mm:ss'));
	System.assertEquals('2026-05-02 08:02:03', Datetime.valueOf('2026-05-02 01:02:03-7').formatGmt('yyyy-MM-dd HH:mm:ss'));
	System.assertEquals('2026-05-02 01:02:03', Datetime.valueOf('2026-05-02 01:02:030').formatGmt('yyyy-MM-dd HH:mm:ss'));
	Object parsedDtObject = parsedDt;
	System.assertEquals(madeText, Datetime.valueOf(parsedDtObject).formatGmt('yyyy-MM-dd HH:mm:ss'));
	Time tm = Time.valueOf('01:02:03');
System.assertEquals(1, tm.hour());
System.assertEquals(2, tm.minute());
System.assertEquals(3, tm.second());
Time midnightUtc = Time.valueOf('00:00:00.000Z');
System.assertEquals(0, midnightUtc.hour());
System.assertEquals(0, midnightUtc.minute());
System.assertEquals(0, midnightUtc.second());
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

func TestExecDateNewInstanceAcceptsMonthDayYearUIParts(t *testing.T) {
	program, err := CompileAnonymous(`
Date day = Date.newInstance(4, 20, 2020);
System.assertEquals(Date.newInstance(2020, 4, 20), day);
System.assertEquals('0000-01-01', String.valueOf(Date.newInstance(0, 1, 1)));
System.assertEquals('2027-01-02', String.valueOf(Date.newInstance(2026, 13, 2)));
System.assertEquals('2017-11-30', String.valueOf(Date.newInstance(2018, 0, 0)));
System.assertEquals('0000-01-01 01:02:03', Datetime.newInstanceGmt(Date.newInstance(0, 1, 1), Time.newInstance(1, 2, 3, 0)).formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('2024-01-01 00:00:00', Datetime.newInstanceGmt(1, 1, 2024).formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('2017-11-30 00:00:00', Datetime.newInstanceGmt(2018, 0, 0).formatGmt('yyyy-MM-dd HH:mm:ss'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDateDatetimeDeterministicInstanceMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Date leap = Date.newInstance(2024, 1, 31);
Date nextMonth = leap.addMonths(1);
System.assertEquals('2024-02-29', String.valueOf(nextMonth));
Date marchEnd = Date.newInstance(2024, 3, 31);
Date previousMonth = marchEnd.addMonths(-1);
System.assertEquals('2024-02-29', String.valueOf(previousMonth));
Date leapDay = Date.newInstance(2024, 2, 29);
Date nextYear = leapDay.addYears(1);
System.assertEquals('2025-02-28', String.valueOf(nextYear));
Date previousYear = leapDay.addYears(-1);
System.assertEquals('2023-02-28', String.valueOf(previousYear));
System.assertEquals(31, leap.day());
System.assertEquals(1, leap.month());
System.assertEquals(2024, leap.year());
System.assertEquals(29, Date.daysInMonth(2024, 2));
System.assertEquals(28, Date.daysInMonth(2025, 2));
Date monthStart = leap.toStartOfMonth();
Date monthEnd = leap.toEndOfMonth();
System.assertEquals('2024-01-01', String.valueOf(monthStart));
System.assertEquals('2024-01-31', String.valueOf(monthEnd));
Date due = leap.addDays(10);
System.assertEquals(10, leap.daysBetween(due));
System.assertEquals(-10, due.daysBetween(leap));
Date nextDay = leap.addDays(1);
Date expectedNextDay = Date.newInstance(2024, 2, 1);
System.assertEquals(expectedNextDay, nextDay);

Datetime stamp = Datetime.newInstance(2024, 1, 31, 23, 58, 59);
Datetime stampNextMonth = stamp.addMonths(1);
System.assertEquals('2024-02-29 23:58:59', stampNextMonth.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime stampPreviousMonth = stamp.addMonths(-1);
System.assertEquals('2023-12-31 23:58:59', stampPreviousMonth.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime leapStamp = Datetime.newInstance(2024, 2, 29, 1, 2, 3);
Datetime leapStampNextYear = leapStamp.addYears(1);
System.assertEquals('2025-02-28 01:02:03', leapStampNextYear.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime leapStampPreviousYear = leapStamp.addYears(-1);
System.assertEquals('2023-02-28 01:02:03', leapStampPreviousYear.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime plusHour = stamp.addHours(1);
Datetime plusMinutes = plusHour.addMinutes(2);
Datetime plusSeconds = plusMinutes.addSeconds(3);
System.assertEquals('2024-02-01 01:01:02', plusSeconds.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime tomorrowStamp = stamp.addDays(1);
Date tomorrowDate = tomorrowStamp.date();
System.assertEquals('2024-02-01', String.valueOf(tomorrowDate));
System.assertEquals(2024, stamp.year());
System.assertEquals(1, stamp.month());
System.assertEquals(31, stamp.day());
System.assertEquals(23, stamp.hour());
System.assertEquals(58, stamp.minute());
System.assertEquals(59, stamp.second());
Datetime midnight = Datetime.newInstance(2024, 1, 31);
System.assertEquals('2024-01-31 00:00:00', midnight.formatGmt('yyyy-MM-dd HH:mm:ss'));
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
System.assertEquals('2026-05-02', String.valueOf(today));

Datetime nowStamp = Datetime.now();
System.assertEquals('2026-05-02 12:00:00', nowStamp.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals(nowStamp, DateTime.Now());
Datetime gmt = Datetime.newInstanceGmt(2024, 2, 29, 23, 59, 58);
Date gmtDate = gmt.dateGmt();
System.assertEquals('2024-02-29', String.valueOf(gmtDate));
System.assertEquals(Time.newInstance(23, 59, 58, 0), gmt.timeGmt());
Datetime parsedGmt = Datetime.valueOfGmt('2024-02-29 23:59:58');
System.assertEquals('2024-02-29 23:59:58', parsedGmt.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime fractionalGmt = Datetime.valueOfGmt('2024-02-29T23:59:58.250Z');
System.assertEquals('2024-02-29 23:59:58', fractionalGmt.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals(0, fractionalGmt.millisecond());
Datetime plusMillis = fractionalGmt.addMilliseconds(750);
System.assertEquals('2024-02-29 23:59:58.750', plusMillis.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
System.assertEquals(750, plusMillis.millisecond());

Time clock = Time.newInstance(23, 59, 58, 250);
System.assertEquals(23, clock.hour());
System.assertEquals(59, clock.minute());
System.assertEquals(58, clock.second());
System.assertEquals(250, clock.millisecond());
Time plusSeconds = clock.addSeconds(2);
System.assertEquals('00:00:00.250Z', plusSeconds.toString());
Time plusMilliseconds = clock.addMilliseconds(750);
System.assertEquals('23:59:59.000Z', plusMilliseconds.toString());
Time plusHours = clock.addHours(1);
System.assertEquals('00:59:58.250Z', plusHours.toString());
Time plusMinutes = clock.addMinutes(-1);
System.assertEquals('23:58:58.250Z', plusMinutes.toString());
System.assertEquals(Time.newInstance(12, 34, 56, 789), Time.valueOf('12:34:56.789'));

TimeZone utc = TimeZone.getTimeZone('UTC');
System.assertEquals('UTC', utc.getID());
System.assertEquals('UTC', utc.toString());
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
System.assertEquals('America/Los_Angeles', pacific.toString());
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
System.assertEquals('1970-01-01 00:00:00', unixEpoch.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime leapDay = Datetime.valueOfGmt('2024-02-29 23:59:58Z');
System.assertEquals('2024-02-29 23:59:58', leapDay.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime fractional = Datetime.valueOfGmt('2024-02-29 23:59:58.250Z');
System.assertEquals('2024-02-29 23:59:58', fractional.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals(0, fractional.millisecond());
Datetime offset = Datetime.valueOfGmt('2024-02-29 18:29:58-05:30');
System.assertEquals('2024-02-29 23:59:58', offset.formatGmt('yyyy-MM-dd HH:mm:ss'));
Datetime assigned = '2024-02-29 23:59:58+0000';
System.assertEquals('2024-02-29 23:59:58', assigned.formatGmt('yyyy-MM-dd HH:mm:ss'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDatetimeValueOfTruncatesFractionalSeconds(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime spaceSeparated = Datetime.valueOfGmt('2024-02-29 23:59:58.250Z');
Datetime isoSeparated = Datetime.valueOfGmt('2024-02-29T23:59:58.250Z');
Datetime localValue = Datetime.valueOf('2024-02-29 23:59:58.250');
Datetime expectedGmt = Datetime.valueOfGmt('2024-02-29 23:59:58Z');
Datetime expectedLocal = Datetime.valueOf('2024-02-29 23:59:58');
System.assertEquals(expectedGmt, spaceSeparated);
System.assertEquals(expectedGmt, isoSeparated);
System.assertEquals(expectedLocal, localValue);
System.assertEquals(0, spaceSeparated.millisecond());
System.assertEquals(0, isoSeparated.millisecond());
System.assertEquals(0, localValue.millisecond());
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
Datetime stamp = Datetime.newInstanceGmt(2024, 2, 29, 23, 5, 6).addMilliseconds(250);
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
System.assertEquals('2024-03-01 06:05:06 +0700 ICT', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Asia/Ho_Chi_Minh'));
System.assertEquals('2024-03-01 08:05:06 +0900 JST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Asia/Tokyo'));
System.assertEquals('2024-07-01 21:00:00 +0900 JST', summer.format('yyyy-MM-dd HH:mm:ss Z z', 'Asia/Tokyo'));
System.assertEquals('2024-02-29 13:05:06 -1000 HST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Pacific/Honolulu'));
System.assertEquals('2024-02-29 12:05:06 -1100 SST', stamp.format('yyyy-MM-dd HH:mm:ss Z z', 'Pacific/Pago_Pago'));
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
			want: "Invalid timezone",
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
System.assertEquals('2024-01-15 10:30:45.123', dt.formatGmt('yyyy-MM-dd HH:mm:ss.SSS'));
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
System.assertEquals(Account.SObjectType, first.getId().getSObjectType());
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
System.assertEquals(false, opts.optAllOrNone, 'default optAllOrNone');
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
System.assertEquals(false, copied.OptAllOrNone, 'cloned OptAllOrNone');
System.assertEquals(true, copied.AllowFieldTruncation);
System.assertEquals(true, copied.EmailHeader.TriggerUserEmail);
System.assertEquals(false, copied.EmailHeader.triggerOtherEmail, 'cloned triggerOtherEmail');
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

Account recordWithOptions = new Account(Name = 'WithOptions');
recordWithOptions.setOptions(opts);
System.assertEquals(true, recordWithOptions.getOptions().AllowFieldTruncation);
insert recordWithOptions;
System.assertNotEquals(null, recordWithOptions.Id);

Database.DMLOptions lowerOpts = new Database.DMLOptions();
lowerOpts.optAllOrNone = false;
List<Database.SaveResult> lowerResults = Database.insert(new List<Account>{new Account(Bogus__c = 'nope')}, lowerOpts);
System.assertEquals(1, lowerResults.size());
System.assertEquals(false, lowerResults.get(0).isSuccess(), 'lower optAllOrNone failed result');

Database.DMLOptions upperOpts = new Database.DMLOptions();
upperOpts.OptAllOrNone = false;
List<Database.SaveResult> upperResults = Database.insert(new List<Account>{new Account(Bogus__c = 'nope')}, upperOpts);
System.assertEquals(1, upperResults.size());
System.assertEquals(false, upperResults.get(0).isSuccess(), 'upper OptAllOrNone failed result');
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
Database.UpsertResult upsertCreated = Database.upsert(upsertNew, Account.External_Key__c, false);
System.assert(upsertCreated.isSuccess());
System.assert(upsertCreated.isCreated());
Account upsertExisting = new Account(External_Key__c = 'EXT-1', Name = 'Upsert Changed');
Database.UpsertResult upsertUpdated = Database.upsert(upsertExisting, Account.External_Key__c, false);
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
Account upsertedExternalMode = new Account(Name = 'Upserted External Mode', Other_Key__c = 'ext-access-level');
Database.UpsertResult upsertExternalMode = Database.upsert(upsertedExternalMode, Account.Other_Key__c, AccessLevel.USER_MODE);
System.assert(upsertExternalMode.isSuccess());
Account upsertedModeOnly = new Account(Name = 'Upserted Mode Only', Other_Key__c = 'ext-access-mode-only');
Database.UpsertResult upsertModeOnly = Database.upsert(upsertedModeOnly, AccessLevel.SYSTEM_MODE);
System.assert(upsertModeOnly.isSuccess());
List<Account> upsertedList = new List<Account>{
	new Account(Name = 'Upserted List A', Other_Key__c = 'ext-list-a'),
	new Account(Name = 'Upserted List B', Other_Key__c = 'ext-list-b')
};
List<Database.UpsertResult> upsertListResults = Database.upsert(upsertedList, Account.Other_Key__c, false, AccessLevel.SYSTEM_MODE);
System.assertEquals(2, upsertListResults.size());
System.assert(upsertListResults.get(0).isSuccess());
System.assert(upsertListResults.get(1).isSuccess());
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

func TestExecAccessLevelWithPermissionSetIdIsExplicitUnsupported(t *testing.T) {
	program, err := CompileAnonymous("AccessLevel.withPermissionSetId('0PS000000000001');")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "AccessLevel.withPermissionSetId permission-set-scoped user mode"` {
		t.Fatalf("err = %#v, want UnsupportedFeature permission-set-scoped user mode", err)
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
System.assertEquals('glade-request-000000000001', request.getRequestId());
System.assertEquals(Quiddity.SYNCHRONOUS, request.getQuiddity());
System.assertEquals('glade-request-000000000001', System.Request.getCurrent().getRequestId());
System.assertEquals('glade-request-000000000001', RequestImpl.getCurrent().getRequestId());
UIRequest uiRequest = UIRequest.getCurrent();
System.assertEquals('local.glade.example', uiRequest.getRequestHeader('host'));
System.assertEquals('local.glade.example', uiRequest.getRequestHeader('Host'));
System.assertEquals(null, uiRequest.getRequestHeader('x-missing'));
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

func TestExecHttpCalloutMockAllowsBlankRequest(t *testing.T) {
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new MockResponse(body = 'ok', statusCode = 201));
HttpResponse res = new Http().send(new HttpRequest());
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

func TestExecHttpCalloutMockAllowsSchemalessEndpoint(t *testing.T) {
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new MockResponse(body = 'ok', statusCode = 201));
HttpRequest req = new HttpRequest();
req.setEndpoint('maps.googleapis.com/maps/api/geocode/json?address=ambiguous-address&key=there');
req.setMethod('GET');
HttpResponse res = new Http().send(req);
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
try {
	res.setBody(null);
	System.assert(false, 'null response body should throw');
} catch (NullPointerException e) {
	System.assertEquals('Argument 1 cannot be null', e.getMessage());
}
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
singleMock.setStatus('Single Status');
singleMock.setHeader('Content-Type', 'application/json');
Test.setMock('HttpCalloutMock', singleMock);
HttpRequest firstReq = new HttpRequest();
firstReq.setEndpoint('https://example.test/single');
firstReq.setMethod('GET');
HttpResponse first = new Http().send(firstReq);
System.assertEquals(203, first.getStatusCode());
System.assertEquals('Single Status', first.getStatus());
System.assertEquals('{"single":true}', first.getBody());
System.assertEquals('application/json', first.getHeader('content-type'));

MultiStaticResourceCalloutMock multiMock = new MultiStaticResourceCalloutMock();
multiMock.setStaticResource('https://example.test/a', 'Response_A');
multiMock.setStaticResource('https://example.test/b', 'Response_B');
multiMock.setStatusCode(204);
multiMock.setStatus('Multi Status');
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
System.assertEquals('Multi Status', second.getStatus());
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

func TestExecHttpCalloutMockStaticCounterPersistsAcrossRespondCalls(t *testing.T) {
	respondProgram, err := CompileAnonymous(`
HttpResponse res = new HttpResponse();
res.setStatusCode(200);
res.setBody(String.valueOf(idx));
idx++;
return res;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new CounterMock());
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test/counter');
req.setMethod('GET');
System.assertEquals('0', new Http().send(req).getBody());
System.assertEquals('1', new Http().send(req).getBody());
System.assertEquals(2, CounterMock.idx);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "CounterMock",
		StaticFields: map[string]Field{
			"idx": {Name: "idx", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "CounterMock.respond",
		ClassName:  "CounterMock",
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

func TestExecChainedBatchPreservesHttpMockStaticStateAcrossDrainPasses(t *testing.T) {
	respondProgram, err := CompileAnonymous(`
HttpResponse res = new HttpResponse();
res.setStatusCode(200);
res.setBody(String.valueOf(idx));
idx++;
return res;
`)
	if err != nil {
		t.Fatal(err)
	}
	startProgram, err := CompileAnonymous(`return new List<Integer>{1};`)
	if err != nil {
		t.Fatal(err)
	}
	executeProgram, err := CompileAnonymous(`
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test/page');
req.setMethod('GET');
String body = new Http().send(req).getBody();
insert new Account(Name = 'page-' + body);
Integer seen = [SELECT COUNT() FROM Account WHERE Name LIKE 'page-%'];
cursor = seen < 3 ? body : null;
`)
	if err != nil {
		t.Fatal(err)
	}
	finishProgram, err := CompileAnonymous(`
if (String.isNotBlank(cursor)) {
    Database.executeBatch(this, 1);
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.setMock('HttpCalloutMock', new CounterMock());
Test.startTest();
Database.executeBatch(new PagedBatch(), 1);
Test.stopTest();
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'page-0']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'page-1']);
System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'page-2']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "CounterMock",
		StaticFields: map[string]Field{
			"idx": {Name: "idx", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "CounterMock.respond",
		ClassName:  "CounterMock",
		ReturnType: "HttpResponse",
		Params:     []Param{{Name: "req", Type: "HttpRequest"}},
		Program:    respondProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "PagedBatch",
		Interfaces: []string{"Database.Batchable<Integer>", "Database.Stateful"},
		Fields: map[string]Field{
			"cursor": {Name: "cursor", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "PagedBatch.start",
		ClassName:  "PagedBatch",
		ReturnType: "Iterable<Integer>",
		Params:     []Param{{Name: "bc", Type: "Database.BatchableContext"}},
		Program:    startProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "PagedBatch.execute",
		ClassName:  "PagedBatch",
		ReturnType: "void",
		Params: []Param{
			{Name: "bc", Type: "Database.BatchableContext"},
			{Name: "scope", Type: "List<Integer>"},
		},
		Program: executeProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "PagedBatch.finish",
		ClassName:  "PagedBatch",
		ReturnType: "void",
		Params:     []Param{{Name: "bc", Type: "Database.BatchableContext"}},
		Program:    finishProgram,
	}); err != nil {
		t.Fatal(err)
	}
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
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint('/relative'); req.setMethod('GET'); new Http().send(req);`,
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
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint(''); req.setMethod('GET'); new Http().send(req);`,
			want: "HttpRequest endpoint is required",
		},
		{
			name: "callout-empty",
			src:  `HttpRequest req = new HttpRequest(); req.setEndpoint('callout:'); req.setMethod('GET'); new Http().send(req);`,
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

func TestExecHttpSendWithoutMockInTestContextReturnsDeterministicResponse(t *testing.T) {
	program, err := CompileAnonymous(`
HttpRequest req = new HttpRequest();
req.setEndpoint('https://example.test');
req.setMethod('GET');
Http h = new Http();
HttpResponse res = h.send(req);
System.assertEquals(200, res.getStatusCode());
System.assertEquals('{}', res.getBody());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatalf("Execute: %v", err)
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
