package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestAPI67ResidualRejectsTimeZoneDisplayNameBooleanByCall(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `TimeZone zone = TimeZone.getTimeZone('UTC'); zone.getDisplayName(false);`)
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "getDisplayName") {
			return
		}
	}
	t.Fatalf("accepted TimeZone.getDisplayName(Boolean): %#v", result.Diagnostics)
}

func TestAPI67CompileShapeRejectsAbstractAndVoidAssignmentBatch(t *testing.T) {
	tests := map[string]string{
		"Auth.AuthConfiguration no-arg constructor":                  `Auth.AuthConfiguration value = new Auth.AuthConfiguration();`,
		"Auth.UserData no-arg constructor":                           `Auth.UserData value = new Auth.UserData();`,
		"Auth.VerificationResult no-arg constructor":                 `Auth.VerificationResult value = new Auth.VerificationResult();`,
		"Invocable.Action no-arg constructor":                        `Invocable.Action value = new Invocable.Action();`,
		"WebServiceCalloutFuture abstract constructor":               `WebServiceCalloutFuture value = new WebServiceCalloutFuture();`,
		"VisualEditor.DynamicPickList abstract constructor":          `VisualEditor.DynamicPickList value = new VisualEditor.DynamicPickList();`,
		"ConnectedAppPlugin.refresh two args returns void":           `Auth.ConnectedAppPlugin plugin = null; Object value = plugin.refresh(null, null);`,
		"ConnectedAppPlugin.refresh invocation context returns void": `Auth.ConnectedAppPlugin plugin = null; Object value = plugin.refresh(null, null, null);`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("accepted Salesforce-rejected compile-shape probe: %s", source)
			}
		})
	}
}

func TestAPI67CompileShapePreservesAllowedConstructorNeighbors(t *testing.T) {
	for name, source := range map[string]string{
		"Auth.AuthConfiguration two args":                `Auth.AuthConfiguration value = new Auth.AuthConfiguration('client', 'secret');`,
		"Auth.UserData documented constructor":           `Auth.UserData value = new Auth.UserData('id', 'first', 'last', 'full', 'email', 'link', 'user', 'locale', 'provider', 'site', new Map<String,String>());`,
		"Auth.VerificationResult documented constructor": `Auth.VerificationResult value = new Auth.VerificationResult(null, true, 'ok');`,
	} {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if result.HasErrors() {
				t.Fatalf("rejected allowed Salesforce constructor: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAPI67CompileShapeRejectsInternalNoArgConstructors(t *testing.T) {
	tests := map[string]string{
		"Approval.LockResult":             `Approval.LockResult value = new Approval.LockResult();`,
		"Approval.ProcessRequest":         `Approval.ProcessRequest value = new Approval.ProcessRequest();`,
		"Approval.ProcessResult":          `Approval.ProcessResult value = new Approval.ProcessResult();`,
		"Approval.UnlockResult":           `Approval.UnlockResult value = new Approval.UnlockResult();`,
		"System.AppExchangeTrialTemplate": `System.AppExchangeTrialTemplate value = new System.AppExchangeTrialTemplate();`,
		"System.Domain":                   `System.Domain value = new System.Domain();`,
		"System.FormulaRecalcFieldError":  `System.FormulaRecalcFieldError value = new System.FormulaRecalcFieldError();`,
		"System.FormulaRecalcResult":      `System.FormulaRecalcResult value = new System.FormulaRecalcResult();`,
		"System.OrgLimit":                 `System.OrgLimit value = new System.OrgLimit();`,
		"System.QueueableContextImpl":     `System.QueueableContextImpl value = new System.QueueableContextImpl();`,
		"System.SchedulableContextImpl":   `System.SchedulableContextImpl value = new System.SchedulableContextImpl();`,
		"System.UUID":                     `System.UUID value = new System.UUID();`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("accepted Salesforce-rejected internal constructor: %s", source)
			}
		})
	}
}

func TestAPI67CompileShapeRejectsNonConstructibleAndHiddenConstructors(t *testing.T) {
	tests := map[string]string{
		"System.Aura":                 `System.Aura value = new System.Aura();`,
		"System.Collator":             `System.Collator value = new System.Collator();`,
		"System.FinalizerContextImpl": `System.FinalizerContextImpl value = new System.FinalizerContextImpl();`,
		"System.FlexQueue":            `System.FlexQueue value = new System.FlexQueue();`,
		"System.ResetPasswordResult":  `System.ResetPasswordResult value = new System.ResetPasswordResult();`,
		"System.System":               `System.System value = new System.System();`,
		"System.UIRequest":            `System.UIRequest value = new System.UIRequest();`,
		"System.WebServiceCallout":    `System.WebServiceCallout value = new System.WebServiceCallout();`,
		"TxnSecurity.EventCondition":  `TxnSecurity.EventCondition value = new TxnSecurity.EventCondition();`,
		"TxnSecurity.PolicyCondition": `TxnSecurity.PolicyCondition value = new TxnSecurity.PolicyCondition();`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("accepted Salesforce-rejected nonconstructible constructor: %s", source)
			}
		})
	}
}

func TestAPI67CompileShapeRejectsHiddenAuthCallsAndContinuationState(t *testing.T) {
	tests := map[string]string{
		"Auth.AuthConfiguration.getRightFrameUrl":            `Auth.AuthConfiguration value = null; Object result = value.getRightFrameUrl();`,
		"Auth.AuthProviderPluginClass.getCustomMetadataType": `Auth.AuthProviderPluginClass value = null; Object result = value.getCustomMetadataType();`,
		"Auth.AuthProviderPluginClass.getUserInfo":           `Auth.AuthProviderPluginClass value = null; Object result = value.getUserInfo(null, null);`,
		"Auth.AuthProviderPluginClass.initiate":              `Auth.AuthProviderPluginClass value = null; Object result = value.initiate(null, null);`,
		"Auth.ConnectedAppPlugin.customAttributes":           `Auth.ConnectedAppPlugin value = null; Object result = value.customAttributes(null, null, null);`,
		"System.Continuation.state":                          `System.Continuation value = null; Object result = value.state;`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("accepted Salesforce-rejected hidden member: %s", source)
			}
		})
	}
}

func TestAPI67ResidualRejectsAcquiredNonSalesforceSurfaces(t *testing.T) {
	tests := map[string]string{
		"Messaging.SendEmailOptions":                    `Messaging.SendEmailOptions options = new Messaging.SendEmailOptions();`,
		"System.Messaging.SendEmailOptions":             `System.Messaging.SendEmailOptions options = new System.Messaging.SendEmailOptions();`,
		"System.Messaging.SingleEmailMessage":           `System.Messaging.SingleEmailMessage message = new System.Messaging.SingleEmailMessage();`,
		"Iterator.remove":                               `Iterator<Integer> values = new List<Integer>{1}.iterator(); values.remove();`,
		"Matcher.appendReplacement":                     `Matcher value = Pattern.compile('a').matcher('a'); value.appendReplacement('', 'x');`,
		"Matcher.appendTail":                            `Matcher value = Pattern.compile('a').matcher('a'); value.appendTail('');`,
		"Canvas.EnvironmentContext.getParametersAsJSON": `Canvas.EnvironmentContext.getParametersAsJSON();`,
		"Canvas.EnvironmentContext.getParameters":       `Canvas.EnvironmentContext.getParameters();`,
		"Canvas.LifecycleHandler":                       `Canvas.LifecycleHandler.onRender(null);`,
		"Database.BatchableContext constructor":         `new Database.BatchableContext();`,
		"AsyncOptions minimum delay getter":             `AsyncOptions options = new AsyncOptions(); options.getMinimumQueueableDelayInMinutes();`,
		"Database.LockResult":                           `Database.LockResult result = Database.lock(new Account(Name = 'lock'), false);`,
		"Database.UnlockResult":                         `Database.UnlockResult result = Database.unlock(new Account(Name = 'unlock'), false);`,
		"Database.lock":                                 `Database.lock(new Account(Name = 'lock'), false);`,
		"Database.unlock":                               `Database.unlock(new Account(Name = 'unlock'), false);`,
		"QuickAction.describeAvailableActions":          `QuickAction.describeAvailableActions('Account');`,
		"Pattern.compile flags overload":                `Pattern.compile('a', 2);`,
		"Pattern.CASE_INSENSITIVE":                      `Integer flag = Pattern.CASE_INSENSITIVE;`,
		"Pattern.COMMENTS":                              `Integer flag = Pattern.COMMENTS;`,
		"Pattern.MULTILINE":                             `Integer flag = Pattern.MULTILINE;`,
		"Pattern.LITERAL":                               `Integer flag = Pattern.LITERAL;`,
		"Pattern.DOTALL":                                `Integer flag = Pattern.DOTALL;`,
		"Pattern.UNICODE_CASE":                          `Integer flag = Pattern.UNICODE_CASE;`,
		"Pattern.UNIX_LINES":                            `Integer flag = Pattern.UNIX_LINES;`,
		"Pattern.CANON_EQ":                              `Integer flag = Pattern.CANON_EQ;`,
		"Pattern.UNICODE_CHARACTER_CLASS":               `Integer flag = Pattern.UNICODE_CHARACTER_CLASS;`,
		"TimeZone.getDisplayName(Boolean)":              `TimeZone zone = TimeZone.getTimeZone('UTC'); zone.getDisplayName(false);`,
		"PushUpgrade create four arguments":             `System.PushUpgradeCustomizationRepository.create('package', 'subscriber', true, 7);`,
		"PushUpgrade block date by id":                  `System.PushUpgradeCustomizationRepository.getPushUpgradeBlockInitiatedDateForId('id');`,
		"PushUpgrade block date by index":               `System.PushUpgradeCustomizationRepository.getPushUpgradeBlockInitiatedDateForIndex('package', 'index');`,
		"PushUpgrade customization summary by id":       `System.PushUpgradeCustomizationRepository.getCustomizationSummaryById('id');`,
		"PushUpgrade customization summary by index":    `System.PushUpgradeCustomizationRepository.getCustomizationSummaryByIndex('package', 'index');`,
		"PushUpgrade expiration days by id":             `System.PushUpgradeCustomizationRepository.getExpirationDaysForId('id');`,
		"PushUpgrade expiration days by index":          `System.PushUpgradeCustomizationRepository.getExpirationDaysForIndex('package', 'index');`,
		"PushUpgrade expired by id":                     `System.PushUpgradeCustomizationRepository.isBlockingCapabilityExpiredForId('id');`,
		"PushUpgrade expired by index":                  `System.PushUpgradeCustomizationRepository.isBlockingCapabilityExpiredForIndex('package', 'index');`,
		"PushUpgrade list summaries":                    `System.PushUpgradeCustomizationRepository.listAllCustomizationSummaries();`,
		"PushUpgrade custom allowed by id":              `System.PushUpgradeCustomizationRepository.setCustomUpgradeAllowedForId('id', true, 7);`,
		"PushUpgrade custom allowed by index":           `System.PushUpgradeCustomizationRepository.setCustomUpgradeAllowedForIndex('package', 'index', true, 7);`,
		"PushUpgrade expiration by id":                  `System.PushUpgradeCustomizationRepository.setExpirationDaysForId('id', 7);`,
		"PushUpgrade expiration by index":               `System.PushUpgradeCustomizationRepository.setExpirationDaysForIndex('package', 'index', 7);`,
		"System.QuickAction.describeAvailableActions":   `System.QuickAction.describeAvailableActions('Account');`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("accepted API 67-rejected source: %s", source)
			}
		})
	}
}

func TestAPI67ResidualPreservesCanonicalSurfaces(t *testing.T) {
	for name, source := range map[string]string{
		"canonical single email message":    `Messaging.SingleEmailMessage message = new Messaging.SingleEmailMessage(); message.setSubject('subject');`,
		"inbound email nested constructors": `new Messaging.InboundEmail.AuthenticationResult(); new Messaging.InboundEmail.AuthenticationResultField(); new Messaging.InboundEmail.BinaryAttachment(); new Messaging.InboundEmail.TextAttachment();`,
		"canonical async info getter":       `AsyncInfo.getMinimumQueueableDelayInMinutes();`,
		"canonical approval lock":           `Approval.lock(new Account(Name = 'lock'));`,
		"canonical quick action":            `QuickAction.describeAvailableQuickActions('Account');`,
		"canonical pattern compile":         `Pattern.compile('a');`,
		"canonical push upgrade create":     `PushUpgradeCustomizationRepository.create('package', 'subscriber', true);`,
		"canonical push upgrade method":     `PushUpgradeCustomizationRepository.getCustomUpgradeAllowedForId('id');`,
		"canonical canvas instance method":  `Canvas.EnvironmentContext context = null; context.getParametersAsJSON();`,
		"canonical qualified schema":        `System.Schema.describeDataCategoryGroups(new List<String>()); System.Schema.describeDataCategoryGroupStructures(new List<Schema.DataCategoryGroupSobjectTypePair>(), false);`,
		"event bus list access level":       `List<Database.SaveResult> results = EventBus.publishWithAccessLevel(new List<Account>{new Account(Name = 'a')}, AccessLevel.USER_MODE);`,
		"event bus list callback access":    `List<Database.SaveResult> results = EventBus.publishWithAccessLevel(new List<Account>{new Account(Name = 'a')}, null, AccessLevel.USER_MODE);`,
		"event bus object access level":     `Database.SaveResult result = EventBus.publishWithAccessLevel(new Account(Name = 'a'), AccessLevel.USER_MODE);`,
		"event bus object callback access":  `Database.SaveResult result = EventBus.publishWithAccessLevel(new Account(Name = 'a'), null, AccessLevel.USER_MODE);`,
	} {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if result.HasErrors() {
				t.Fatalf("rejected canonical API source: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAPI67ResidualRejectsRemovedSiteURLHelpers(t *testing.T) {
	for _, method := range []string{"getCurrentSiteUrl", "getCustomWebAddress", "getPrefix"} {
		t.Run(method, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, "Site."+method+"();")
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA028") {
				t.Fatalf("accepted removed Site.%s: %#v", method, result.Diagnostics)
			}
		})
	}
}

func TestAPI67ResidualRejectsInvalidDatabaseAllowCalloutsType(t *testing.T) {
	for _, source := range []string{
		`Database.insertAsync(new Account(Name = 'callout'), Database.AllowCallouts.ALLOW, AccessLevel.SYSTEM_MODE);`,
		`Database.insertAsync(new List<Account>{new Account(Name = 'callout')}, Database.AllowCallouts.ALLOW, AccessLevel.SYSTEM_MODE);`,
		`Database.updateAsync(new Account(Id = '001000000000001AAA'), Database.AllowCallouts.ALLOW, AccessLevel.SYSTEM_MODE);`,
		`Database.updateAsync(new List<Account>{new Account(Id = '001000000000001AAA')}, Database.AllowCallouts.ALLOW, AccessLevel.SYSTEM_MODE);`,
		`Database.deleteAsync(new Account(Id = '001000000000001AAA'), Database.AllowCallouts.ALLOW, AccessLevel.SYSTEM_MODE);`,
		`Database.deleteAsync(new List<Account>{new Account(Id = '001000000000001AAA')}, Database.AllowCallouts.ALLOW, AccessLevel.SYSTEM_MODE);`,
	} {
		result := AnalyzeAnonymous(typesys.Index{}, source)
		if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA028") {
			t.Fatalf("accepted invalid Database.AllowCallouts type in %q: %#v", source, result.Diagnostics)
		}
	}
}

func TestCB68RejectsThreeAcquiredNonSurfaces(t *testing.T) {
	tests := map[string]string{
		"Approval constructor":                    `new Approval();`,
		"QueueableDuplicateSignature constructor": `new QueueableDuplicateSignature();`,
		"ConnectApi.getError":                     `ConnectApi.getError();`,
		"ConnectApi.getErrorMessage":              `ConnectApi.getErrorMessage();`,
		"ConnectApi.getErrorTypeName":             `ConnectApi.getErrorTypeName();`,
		"ConnectApi.getResult":                    `ConnectApi.getResult();`,
		"ConnectApi.isSuccess":                    `ConnectApi.isSuccess();`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA028") {
				t.Fatalf("accepted acquired non-Salesforce surface %q: %#v", source, result.Diagnostics)
			}
		})
	}
}

func TestCB68PreservesNeighboringValidPlatformSurfaces(t *testing.T) {
	for name, source := range map[string]string{
		"Approval.process":                    `Approval.ProcessResult result = Approval.process(new Approval.ProcessSubmitRequest());`,
		"QueueableDuplicateSignature builder": `QueueableDuplicateSignature signature = QueueableDuplicateSignature.builder().addString('job').build();`,
		"ConnectApi real type":                `ConnectApi.OrganizationSettings settings = ConnectApi.Organization.getSettings();`,
	} {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source)
			if result.HasErrors() {
				t.Fatalf("rejected valid neighboring platform surface: %#v", result.Diagnostics)
			}
		})
	}
}
