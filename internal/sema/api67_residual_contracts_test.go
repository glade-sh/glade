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
