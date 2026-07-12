package sema

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeResolvesMemberTypes(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<Thing__c>"},
				},
			},
		},
		Objects: []schema.Object{{Name: "Thing__c"}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeResolvesNamespaceQualifiedSchemaAliases(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<pkg__Thing__c>"},
				},
			},
		},
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "pkg__Thing__c", File: "Thing.trigger"}},
		Objects: []schema.Object{{
			Name: "Thing__c",
			Fields: []schema.Field{
				{Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"pkg__Thing__c"}},
			},
		}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeResolvesProjectLocalNamesAgainstNamespacedSchema(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<Thing__c>"},
					{Kind: apexast.DeclarationMethod, Name: "save", Parameters: []apexast.Parameter{{Name: "row", Type: "Thing__c"}}},
				},
			},
		},
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "Thing__c", File: "Thing.trigger"}},
		Objects: []schema.Object{{
			Name: "pkg__Thing__c",
			Fields: []schema.Field{
				{Name: "pkg__Parent__c", Type: "Lookup", ReferenceTo: []string{"Thing__c"}},
			},
		}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectLocalSObjectFieldsAgainstNamespacedSchema(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesNamespacedFields.cls"), `
public class UsesNamespacedFields {
  public void run(Membership__c membership) {
    Date nextStart = membership.StartDate__c.addMonths(1);
    MembershipType__c membershipType = membership.MembershipType2__r;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesNamespacedFields.cls")}}, schema.Schema{
		Objects: []schema.Object{
			{
				Name: "pkg__Membership__c",
				Fields: []schema.Field{
					{Name: "pkg__StartDate__c", Type: "Date"},
					{Name: "pkg__MembershipType2__c", Type: "Lookup", ReferenceTo: []string{"pkg__MembershipType__c"}, RelationshipName: "pkg__MembershipType2__r"},
				},
			},
			{Name: "pkg__MembershipType__c"},
		},
	})
	index.Project.Namespace = "pkg"

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA018" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected namespaced SObject field diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCommonSObjectRelationshipsResolveStandardChains(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "UsesCommonRelationships.cls")
	writeSemaFile(t, classPath, `
public class UsesCommonRelationships {
  public String licenseKey(pkg__Cart__c cart) {
    return cart.CreatedBy.Profile.UserLicense.LicenseDefinitionKey;
  }
  public String modifierUsername(pkg__Cart__c cart) {
    return cart.LastModifiedBy.Username;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "pkg__Cart__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected common SObject relationship diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSOQLCountExpressionAssignsInteger(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CountsRows.cls"), `
public class CountsRows {
  public Integer run(String prefix) {
    Integer rows = [SELECT COUNT() FROM Thing__c WHERE Name LIKE :prefix];
    return rows;
  }
  public Integer returnCount() {
    return [SELECT COUNT() FROM Thing__c WHERE Name != null];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "CountsRows.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Thing__c", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" || diag.Code == "GLADESEMA019" {
			t.Fatalf("unexpected COUNT() type diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzePageReferenceMethodChainsUseReturnTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPageReference.cls"), `
public class UsesPageReference {
  public void run() {
    PageReference pageRef = Page.OrderPayment;
    String pageUrl = Page.OrderPayment.getUrl();
    String pageContent = Page.GetSessionId.getContent().toString();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesPageReference.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("unexpected PageReference chain type diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeVisualforceComponentTypesAssignableToApexPagesComponent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesVisualforceComponents.cls"), `
public class UsesVisualforceComponents {
  public ApexPages.Component customComponent() {
    return new Component.AccountDetail();
  }
  public ApexPages.Component standardComponent() {
    return new Component.Apex.OutputPanel();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesVisualforceComponents.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" || diag.Code == "GLADESEMA006" || diag.Code == "GLADESEMA011" {
			t.Fatalf("unexpected Visualforce component diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestSemaResolveFieldUsesShortCandidateIndex(t *testing.T) {
	model := make(map[string]typeMembers)
	for i := 0; i < 1000; i++ {
		name := "pkg.CandidateType" + strconv.Itoa(i)
		model[normalizeName(name)] = typeMembers{
			name:     name,
			shortKey: semaShortTypeKey(name),
			fields:   map[string]typesys.MemberSymbol{},
			methods:  map[string][]typesys.MemberSymbol{},
		}
	}
	targetName := "pkg.TargetInner"
	model[normalizeName(targetName)] = typeMembers{
		name:     targetName,
		shortKey: semaShortTypeKey(targetName),
		fields: map[string]typesys.MemberSymbol{
			"name": {Kind: apexast.DeclarationField, Name: "Name", Type: "String"},
		},
		methods: map[string][]typesys.MemberSymbol{},
	}
	view := semaTypeMemberViewFromMembers(model)

	field, ok := semaResolveField(view, "TargetInner", "Name", make(map[string]bool))
	if !ok {
		t.Fatal("short-name field lookup did not resolve")
	}
	if field.member.Type != "String" {
		t.Fatalf("field type = %q, want String", field.member.Type)
	}

	allocs := testing.AllocsPerRun(20, func() {
		if _, ok := semaResolveField(view, "TargetInner", "Name", make(map[string]bool)); !ok {
			t.Fatal("short-name field lookup did not resolve")
		}
	})
	if allocs > 20 {
		t.Fatalf("short-name field lookup allocated %.0f times, want <= 20", allocs)
	}
}

func TestSemaAnyKnownFieldNilModel(t *testing.T) {
	if semaAnyKnownField(nil, "Name") {
		t.Fatal("nil model reported a known field")
	}
}

func TestSemaAnyKnownFieldZeroView(t *testing.T) {
	if semaAnyKnownField(&semaTypeMemberView{}, "Name") {
		t.Fatal("zero view reported a known field")
	}
}

func TestSemaAnyKnownFieldCurrentOnlyView(t *testing.T) {
	model := &semaTypeMemberView{current: map[string]typeMembers{
		"current": {
			fields: map[string]typesys.MemberSymbol{
				normalizeName("OverlayField"): {Kind: apexast.DeclarationField, Name: "OverlayField", Type: "String"},
			},
		},
	}}
	if !semaAnyKnownField(model, "OverlayField") {
		t.Fatal("current-only view did not report its field")
	}
	if semaAnyKnownField(model, "MissingField") {
		t.Fatal("current-only view reported a missing field")
	}
}

func TestNormalizeNameMatchesTrimmedLowercase(t *testing.T) {
	cases := []string{
		" Account__c ",
		"already_lower__c",
		"pkg.Outer.Inner",
		"Straße",
	}
	for _, input := range cases {
		want := strings.ToLower(strings.TrimSpace(input))
		if got := normalizeName(input); got != want {
			t.Fatalf("normalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAnalyzeUnknownMemberType(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}}},
				},
			},
		},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "GLADESEMA002" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func TestAnalyzeMethodParameterTypes(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "run",
						Parameters: []apexast.Parameter{
							{Name: "accounts", Type: "List<Account>"},
							{Name: "missing", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 2, Column: 20}}},
						},
					},
				},
			},
		},
		Objects: []schema.Object{{Name: "Account"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA004" {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

func TestAnalyzeRecognizesCallableAndStubProviderTypes(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Provider",
				File:       "Provider.cls",
				Interfaces: []string{"System.StubProvider"},
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "handleMethodCall",
						Type: "Object",
						Parameters: []apexast.Parameter{
							{Name: "stubbedObject", Type: "Object"},
							{Name: "stubbedMethodName", Type: "String"},
							{Name: "returnType", Type: "Type"},
							{Name: "listOfParamTypes", Type: "List<Type>"},
							{Name: "listOfParamNames", Type: "List<String>"},
							{Name: "listOfArgs", Type: "List<Object>"},
						},
					},
				},
			},
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Action",
				File:       "Action.cls",
				Interfaces: []string{"System.Callable"},
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "call",
						Type: "Object",
						Parameters: []apexast.Parameter{
							{Name: "action", Type: "String"},
							{Name: "args", Type: "Map<String, Object>"},
						},
					},
				},
			},
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Mock",
				File:       "Mock.cls",
				Interfaces: []string{"HttpCalloutMock"},
				Members: []typesys.MemberSymbol{{
					Kind: apexast.DeclarationMethod,
					Name: "respond",
					Type: "HttpResponse",
					Parameters: []apexast.Parameter{
						{Name: "request", Type: "HttpRequest"},
					},
				}},
			},
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Queued",
				File:       "Queued.cls",
				Interfaces: []string{"Queueable"},
				Members: []typesys.MemberSymbol{{
					Kind: apexast.DeclarationMethod,
					Name: "execute",
					Type: "void",
					Parameters: []apexast.Parameter{
						{Name: "context", Type: "QueueableContext"},
					},
				}},
			},
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Scheduled",
				File:       "Scheduled.cls",
				Interfaces: []string{"Schedulable"},
				Members: []typesys.MemberSymbol{{
					Kind: apexast.DeclarationMethod,
					Name: "execute",
					Type: "void",
					Parameters: []apexast.Parameter{
						{Name: "context", Type: "SchedulableContext"},
					},
				}},
			},
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Batch",
				File:       "Batch.cls",
				Interfaces: []string{"Database.Batchable<Account>"},
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "start",
						Type: "Database.QueryLocator",
						Parameters: []apexast.Parameter{
							{Name: "context", Type: "Database.BatchableContext"},
						},
					},
					{
						Kind: apexast.DeclarationMethod,
						Name: "execute",
						Type: "void",
						Parameters: []apexast.Parameter{
							{Name: "context", Type: "Database.BatchableContext"},
							{Name: "records", Type: "List<Account>"},
						},
					},
					{
						Kind: apexast.DeclarationMethod,
						Name: "finish",
						Type: "void",
						Parameters: []apexast.Parameter{
							{Name: "context", Type: "Database.BatchableContext"},
						},
					},
				},
			},
		},
		Objects: []schema.Object{{Name: "Account"}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePlatformAPITestCreateStubAndSetMockBridge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Greeter.cls"), `
public interface Greeter {
  String greet(String name);
}
`)
	writeSemaFile(t, filepath.Join(root, "GreeterProvider.cls"), `
private class GreeterProvider implements System.StubProvider {
  public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType, List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
    return 'stubbed';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "MockResponse.cls"), `
private class MockResponse implements HttpCalloutMock {
  public HttpResponse respond(HttpRequest request) {
    return new HttpResponse();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "PlatformBridgeTest.cls"), `
@isTest
private class PlatformBridgeTest {
  @isTest static void bridgeCompiles() {
    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    Test.setMock('HttpCalloutMock', new MockResponse());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Greeter.cls"),
		filepath.Join(root, "GreeterProvider.cls"),
		filepath.Join(root, "MockResponse.cls"),
		filepath.Join(root, "PlatformBridgeTest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePlatformAPITestSetCurrentPageReferencePageToken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PageTokenTest.cls"), `
@isTest
private class PageTokenTest {
  @isTest static void pageTokenCompiles() {
    Test.setCurrentPage(Page.AccountView);
    Test.setCurrentPageReference(Page.OrderList);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PageTokenTest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePlatformAPIInterfacesRequireContracts(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind:       apexast.DeclarationClass,
				Name:       "Provider",
				File:       "Provider.cls",
				Interfaces: []string{"System.StubProvider"},
			},
		},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected missing StubProvider contract diagnostic")
	}
}

func TestAnalyzeProjectNamespaceQualifiedTypes(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Helper",
				File: "Helper.cls",
			},
			{
				Kind: apexast.DeclarationClass,
				Name: "UsesHelper",
				File: "UsesHelper.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "pkg.Helper"},
					{
						Kind: apexast.DeclarationMethod,
						Name: "withParam",
						Parameters: []apexast.Parameter{
							{Name: "helper", Type: "pkg.Helper"},
						},
					},
				},
			},
		},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTypeReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public class Inner {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesInner.cls"), `
public class UsesInner {
  public Outer.Inner build() {
    Outer.Inner value = new Outer.Inner();
    return value;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Outer.cls"),
		filepath.Join(root, "UsesInner.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestStaticClassFieldPathUnknownLongPathReturnsPromptly(t *testing.T) {
	fieldPath := strings.Join([]string{
		"one", "two", "three", "four", "five", "six", "seven", "eight",
		"nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen",
		"sixteen", "seventeen", "eighteen", "nineteen", "twenty", "twentyone",
		"twentytwo", "twentythree", "twentyfour",
	}, ".")
	done := make(chan bool, 1)
	go func() {
		_, ok := semaStaticClassFieldPathMember(semaTypeMemberViewFromMembers(map[string]typeMembers{}), "Missing", fieldPath)
		done <- ok
	}()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("unexpected static field path match")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("unknown static field path lookup did not return promptly")
	}
}

func TestExplicitPlatformStaticFieldPathMemberUsesReadOnlyPlatformView(t *testing.T) {
	typesys.StandardPlatformSymbolView()
	model := semaTypeMemberViewFromMembers(map[string]typeMembers{})
	allocs := testing.AllocsPerRun(10, func() {
		target, ok := semaStaticClassFieldPathMember(model, "System.RoundingMode", "HALF_UP")
		if !ok {
			t.Fatal("expected System.RoundingMode.HALF_UP")
		}
		if target.member.Type != "RoundingMode" {
			t.Fatalf("System.RoundingMode.HALF_UP type = %q, want RoundingMode", target.member.Type)
		}
	})
	if allocs > 8 {
		t.Fatalf("static platform field lookup allocated %.0f times, want at most 8", allocs)
	}
}

func TestChainedCallReceiverLongDottedFluentChainReturnsPromptly(t *testing.T) {
	var chain strings.Builder
	chain.WriteString("Mock.Stub.start()\n")
	for i := 0; i < 24; i++ {
		chain.WriteString("    .when(Mock.MI.MockService")
		chain.WriteString(strconv.Itoa(i))
		chain.WriteString(".generate((TransactionGenerator.TransactionGenerationRequest) match_anyObject()))\n")
		chain.WriteString("    .then(mockResult)\n")
	}
	chain.WriteString("    .when(Mock.MI.MockTransactionGenerator2.generate((TransactionGenerator.TransactionGenerationRequest) match_anyObject()))\n")
	chain.WriteString("    .then(mockTransactionResult)\n")
	chain.WriteString("    .stop();")
	body := chain.String()
	callStart := strings.LastIndex(body, ".then(mockTransactionResult)")
	if callStart < 0 {
		t.Fatal("test setup did not find target chained call")
	}
	callStart++
	done := make(chan bool, 1)
	go func() {
		_, _, _ = semaChainedCallReceiver(body, callStart, map[string]string{
			semaCurrentTypeScopeKey: "TestCartSubmitter",
		}, semaTypeMemberViewFromMembers(map[string]typeMembers{}), "TestCartSubmitter")
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("long dotted fluent-chain receiver lookup did not return promptly")
	}
}

func TestChainedCallReceiverExpandedLongReceiverReturnsPromptly(t *testing.T) {
	var chain strings.Builder
	chain.WriteString("Mock.Stub.start()\n")
	chain.WriteString("    .when(Mock.MI.MockCartManager.getCartByIdForUpdate(cartToSubmit.Id)).then(cartToSubmit)\n")
	chain.WriteString("    .when(Mock.MI.MockCartItemManager.getCartItemsWithoutCouponsByCartIds(match_anySetOfId())).then(cartToSubmit.CartItems__r)\n")
	chain.WriteString("    .when(Mock.MI.MockCartItemManager.getClonedBillingHistoryCartItemsByCartIds(new set<Id> { cartToSubmit.Id }, true)).then(cartToSubmit.CartItems__r)\n")
	for i := 0; i < 12; i++ {
		chain.WriteString("    .when(Mock.MI.MockService")
		chain.WriteString(strconv.Itoa(i))
		chain.WriteString(".generate((TransactionGenerator.TransactionGenerationRequest) match_anyObject()))\n")
		chain.WriteString("    .then(mockResult)\n")
	}
	chain.WriteString("    .when(Mock.MI.MockTransactionGenerator2.generate((TransactionGenerator.TransactionGenerationRequest) match_anyObject()))\n")
	chain.WriteString("    .then(mockTransactionResult)\n")
	chain.WriteString("    .stop();")
	body := chain.String()
	callStart := strings.LastIndex(body, ".then(mockTransactionResult)")
	if callStart < 0 {
		t.Fatal("test setup did not find target chained call")
	}
	callStart++
	done := make(chan bool, 1)
	go func() {
		_, _, _ = semaChainedCallReceiver(body, callStart, map[string]string{
			semaCurrentTypeScopeKey: "TestCartSubmitter",
		}, semaTypeMemberViewFromMembers(map[string]typeMembers{}), "TestCartSubmitter")
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expanded long fluent-chain receiver lookup did not return promptly")
	}
}

func TestAnalyzeNestedEnumShortQualifiedAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "Comparator.cls")
	writeSemaFile(t, classPath, `
public class Comparator {
  public class ASCComparatorException extends Exception {}
  public SortOrder order { get; set; }
  public enum SortOrder { ASCENDING, DESCENDING }
  public Comparator() {
    order = SortOrder.ASCENDING;
  }
  public Comparator(Comparator.SortOrder order) {
    if (order == null) {
      throw new ASCComparatorException('Sort order cannot be null');
    }
    this.order = order;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeForEachOverDynamicObjectDefersElementValidation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "DynamicIterable.cls")
	writeSemaFile(t, classPath, `
public class DynamicIterable {
  public static void run(Object values) {
    for (String value : values) {
      System.debug(value);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUserDefinedIterableAndAddAll(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "IterableClient.cls")
	writeSemaFile(t, classPath, `
public class IterableClient implements Iterable<RecordPage> {
  public Iterator<RecordPage> iterator() {
    return null;
  }
  public class RecordPage {
    public List<String> getRecords() {
      return new List<String>();
    }
  }
  public static List<String> run() {
    List<String> records = new List<String>();
    IterableClient client = new IterableClient();
    for (IterableClient.RecordPage page : client) {
      records.addAll(page.getRecords());
    }
    return records;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	for _, typ := range index.Types {
		if typ.Name == "IterableClient" && len(typ.Interfaces) == 0 {
			t.Fatalf("missing interfaces: %#v", typ)
		}
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSOSLFindLiteral(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "UsesSOSL.cls")
	writeSemaFile(t, classPath, `
public class UsesSOSL {
  public static void run(String keyword) {
    List<List<SObject>> searchResults = [
      FIND :keyword
      IN ALL FIELDS
      RETURNING Account(Name), Contact(LastName, Account.Name)
    ];
    Account[] accounts = searchResults[0];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "Account"}, {Name: "Contact"}},
	})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeWarnsForSOQLInLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "QueryInLoop.cls")
	writeSemaFile(t, classPath, `
public class QueryInLoop {
  public static void run(List<Account> accounts) {
    for (Account account : accounts) {
      List<Contact> contacts = [SELECT Id FROM Contact WHERE AccountId = :account.Id];
      System.debug(contacts.size());
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "Account"}, {Name: "Contact"}},
	})

	result := Analyze(index)
	if !hasDiagnosticCode(result.Diagnostics, "GLADEPERF001") {
		t.Fatalf("expected GLADEPERF001 diagnostic, got %#v", result.Diagnostics)
	}
	if result.HasErrors() {
		t.Fatalf("performance warning should not fail check: %#v", result.Diagnostics)
	}
}

func TestAnalyzeWarnsForDMLInLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "DMLInLoop.cls")
	writeSemaFile(t, classPath, `
public class DMLInLoop {
  public static void run(List<Account> accounts) {
    for (Account account : accounts) {
      update account;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "Account"}},
	})

	result := Analyze(index)
	if !hasDiagnosticCode(result.Diagnostics, "GLADEPERF001") {
		t.Fatalf("expected GLADEPERF001 diagnostic, got %#v", result.Diagnostics)
	}
	if result.HasErrors() {
		t.Fatalf("performance warning should not fail check: %#v", result.Diagnostics)
	}
}

func TestAnalyzeWarnsForDatabaseDMLCallInLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "DatabaseDMLInLoop.cls")
	writeSemaFile(t, classPath, `
public class DatabaseDMLInLoop {
  public static void run(List<Account> accounts) {
    for (Account account : accounts) {
      Database.update(account, false);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "Account"}},
	})

	result := Analyze(index)
	if !hasDiagnosticCode(result.Diagnostics, "GLADEPERF001") {
		t.Fatalf("expected GLADEPERF001 diagnostic, got %#v", result.Diagnostics)
	}
	if result.HasErrors() {
		t.Fatalf("performance warning should not fail check: %#v", result.Diagnostics)
	}
}

func TestAnalyzeWarnsForStaticFirstTouchMassMetadata(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "HeavyConstants.cls")
	writeSemaFile(t, classPath, `
public class HeavyConstants {
  public static Map<String, Schema.SObjectType> TOKENS = Schema.getGlobalDescribe();
  static {
    Map<String, FeatureFlag__mdt> flags = FeatureFlag__mdt.getAll();
  }
  public static final String LABEL = 'ok';
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "FeatureFlag__mdt"}},
	})

	result := Analyze(index)
	if !hasDiagnosticCode(result.Diagnostics, "GLADEPERF002") {
		t.Fatalf("expected GLADEPERF002 diagnostic, got %#v", result.Diagnostics)
	}
	if result.HasErrors() {
		t.Fatalf("performance warning should not fail check: %#v", result.Diagnostics)
	}
}

func TestAnalyzeShortNestedTypeMatchesAnyCompatibleCandidate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "One.cls"), `
public class One {
  public class Shared {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {}
`)
	writeSemaFile(t, filepath.Join(root, "Two.cls"), `
public class Two {
  public class Shared extends Base {
    public String Ensured;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesShared.cls"), `
public class UsesShared {
  public static void run(Shared candidate) {
    Base baseValue = candidate;
    String ensured = candidate.Ensured;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "One.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Two.cls"),
		filepath.Join(root, "UsesShared.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTestDoubleStaticMockAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseMock.cls"), `
public virtual class BaseMock {
  @TestVisible private static BaseMock mockInstance;
  public BaseMock() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "OneTest.cls"), `
@IsTest
private class OneTest {
  private class MockChild extends BaseMock {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesMockTest.cls"), `
@IsTest
private class UsesMockTest {
  @IsTest static void run() {
    MockChild mockChild = new MockChild();
    BaseMock.mockInstance = mockChild;
    System.assert(mockChild.Ensured);
  }
  private class MockChild extends BaseMock {
    public Boolean Ensured = false;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "BaseMock.cls"),
		filepath.Join(root, "OneTest.cls"),
		filepath.Join(root, "UsesMockTest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProductNamespaceGeneratedDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesProductNamespaces.cls"), `
public class UsesProductNamespaces {
  public static void run() {
    connectapi.organizationsettings settings = connectapi.organization.getsettings();
    ConnectApi.TimeZone zone = settings.userSettings.timeZone;
    Metadata.DeployContainer container = new Metadata.DeployContainer();
    Metadata.CustomMetadata item = new Metadata.CustomMetadata();
    Metadata.CustomMetadataValue value = new Metadata.CustomMetadataValue();
    value.field = 'Enabled__c';
    value.value = true;
    item.values.add(value);
    container.addMetadata(item);
    Id deploymentId = Metadata.Operations.enqueueDeployment(container, null);
    Metadata.DeployResult result = Metadata.Operations.checkDeployStatus(deploymentId, true);
    handleDeployResult(result.id, result.errorMessage, result.success);
    Cache.OrgPartition partition = cache.org.getpartition('local');
    partition.put('zone', zone.id, 60, cache.visibility.all, false);
  }
  private static void handleDeployResult(Id jobId, String message, Boolean success) {
  }
  public static Cache.Partition getPartition(Boolean session) {
    if (session) {
      return Cache.Session.getPartition('local');
    }
    return Cache.Org.getPartition('local');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesProductNamespaces.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUserInfoStandardDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesUserInfo.cls"), `
public class UsesUserInfo {
  public static void run() {
    Id userId = UserInfo.getUserId();
    Id profileId = USERINFO.getProfileId();
    String username = UserInfo.getUserName();
    String name = UserInfo.getName();
    String firstName = UserInfo.getFirstName();
    String lastName = UserInfo.getLastName();
    String email = UserInfo.getUserEmail();
    Id orgId = UserInfo.getOrganizationId();
    String userType = UserInfo.getUserType();
    String sessionId = UserInfo.getSessionId();
    String locale = UserInfo.getLocale();
    String language = UserInfo.getLanguage();
    TimeZone zone = UserInfo.getTimeZone();
    Boolean multiCurrency = UserInfo.isMultiCurrencyOrganization();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesUserInfo.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseDMLCollectionOverloads(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatabaseDML.cls"), `
public class UsesDatabaseDML {
  public static void insertAccountsViaDatabaseMethod(List<String> names, Boolean allOrNothing, System.AccessLevel accessLevel) {}
  public static void run(List<Account> accounts, Account account, Id recordId, List<Id> recordIds, Database.DMLOptions opts) {
    insertAccountsViaDatabaseMethod(new List<String>{'Texas'}, false, AccessLevel.SYSTEM_MODE);
    List<Database.SaveResult> insertResults = Database.insert(accounts);
    List<Database.SaveResult> partialInsertResults = Database.insert(accounts, false);
    List<Database.SaveResult> optionInsertResults = Database.insert(accounts, opts);
    Database.SaveResult singleInsert = Database.insert(account, false);
    List<Database.SaveResult> userModeInsertResults = Database.insert(accounts, false, AccessLevel.USER_MODE);
    Database.SaveResult systemModeInsert = Database.insert(account, AccessLevel.SYSTEM_MODE);
    List<Database.SaveResult> updateResults = Database.update(accounts);
    List<Database.SaveResult> userModeUpdateResults = Database.update(accounts, false, AccessLevel.USER_MODE);
    List<Database.DeleteResult> deleteResults = Database.delete(accounts, false);
    List<Database.DeleteResult> userModeDeleteResults = Database.delete(accounts, false, AccessLevel.USER_MODE);
    Database.DeleteResult idDelete = Database.delete(recordId);
    Database.DeleteResult systemModeIdDelete = Database.delete(recordId, AccessLevel.SYSTEM_MODE);
    List<Database.DeleteResult> idDeleteResults = Database.delete(recordIds, false);
    List<Database.UpsertResult> upsertResults = Database.upsert(accounts, Account.External_Id__c, false);
    List<Database.UpsertResult> userModeExternalIdUpsert = Database.upsert(accounts, Account.External_Id__c, AccessLevel.USER_MODE);
    Database.UpsertResult singleUpsert = Database.upsert(account, Account.External_Id__c, false);
    Database.UpsertResult singleUserModeExternalIdUpsert = Database.upsert(account, Account.External_Id__c, AccessLevel.USER_MODE);
    List<Database.UpsertResult> systemModeUpsertResults = Database.upsert(accounts, AccessLevel.SYSTEM_MODE);
    Database.UpsertResult singleSystemModeUpsert = Database.upsert(account, AccessLevel.SYSTEM_MODE);
    List<Database.UpsertResult> systemModeUpsertNoExternalId = Database.upsert(accounts, true, AccessLevel.SYSTEM_MODE);
    List<Database.UpsertResult> userModeUpsertResults = Database.upsert(accounts, Account.External_Id__c, false, AccessLevel.USER_MODE);
    Database.UpsertResult singleUserModeUpsertNoExternalId = Database.upsert(account, false, AccessLevel.USER_MODE);
    Database.UpsertResult systemModeSingleUpsert = Database.upsert(account, Account.External_Id__c, false, AccessLevel.SYSTEM_MODE);
    Database.UndeleteResult idUndelete = Database.undelete(recordId);
    List<Database.UndeleteResult> idUndeleteResults = Database.undelete(recordIds, false, AccessLevel.USER_MODE);
    Database.EmptyRecycleBinResult idEmptyRecycleBin = Database.emptyRecycleBin(recordId);
    List<Database.EmptyRecycleBinResult> idEmptyRecycleBinResults = Database.emptyRecycleBin(recordIds);
    Database.MergeResult idMerge = Database.merge(account, recordId, false, AccessLevel.USER_MODE);
    List<Database.MergeResult> idMergeResults = Database.merge(account, recordIds, AccessLevel.SYSTEM_MODE);
  }
  public static Database.UpsertResult singleResult(Account account) {
    return Database.upsert(account, Account.External_Id__c, false, AccessLevel.SYSTEM_MODE);
  }
  public static List<Database.UpsertResult> collectionResult(List<Account> accounts) {
    return Database.upsert(accounts, Account.External_Id__c, false, AccessLevel.USER_MODE);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDatabaseDML.cls"),
	}}, schema.Schema{Objects: []schema.Object{{
		Name: "Account",
		Fields: []schema.Field{
			{Name: "External_Id__c", Type: "Text"},
		},
	}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCaseInsensitivePlatformEnumAndDateStatics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCaseInsensitivePlatformStatics.cls"), `
public class UsesCaseInsensitivePlatformStatics {
  public static void run() {
    System.debug(logginglevel.INFO, 'info');
    System.debug(logginglevel.ERROR, 'error');
    Date parsed = Date.ValueOf((Object) '2026-05-07');
    Date today = date.TODAY();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesCaseInsensitivePlatformStatics.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeBroadSystemStubShapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesBroadSystemShapes.cls"), `
public class UsesBroadSystemShapes {
  public enum ParameterSet {
    SSN_PARAMETER_SET,
    EDUCATION_PARAMETER_SET
  }
  public static void run(HttpRequest request, Account account, EntityParticle particle, FieldDefinition fieldDefinition) {
    HttpRequest made = new HttpRequest();
    made.setEndpoint('callout:example');
    made.setMethod('GET');
    made.setHeader('X-Test', request.getHeader('X-Test'));
    String body = made.getBody();
    Integer namePos = account.Name.indexOf('School');
    Integer laterPos = account.Name.indexOf('School', namePos);
    Datetime gmtDate = Datetime.newInstanceGmt(2026, 5, 14);
    AsyncOptions asyncOptions = new AsyncOptions();
    asyncOptions.MaximumQueueableStackDepth = 5;
    Database.DMLOptions dmlOptions = new Database.DMLOptions();
    dmlOptions.OptAllOrNone = true;
    dmlOptions.EmailHeader.triggerUserEmail = false;
    dmlOptions.DuplicateRuleHeader.AllowSave = true;
    dmlOptions.AssignmentRuleHeader.UseDefaultRule = true;
    String street = account.BillingAddress.getStreet();
    Datetime parsed = Datetime.valueOf((Object) '2026-05-14 00:00:00');
    Schema.DescribeSObjectResult described = account.getSObjectType().getDescribe(SObjectDescribeOptions.DEFERRED);
    account.addError(Account.Name, 'Name is required');
    Iterable<SObjectField> iterableFields = new List<SObjectField>{ Account.Name };
    for (SObjectField field : iterableFields) {
      String fieldName = field.getDescribe().getName();
    }
    List<EntityDefinition> entityDefinitions = new List<SObject>();
    switch on 'equals' {
      when equals {
        body = 'equal';
      }
      when not_equals {
        body = 'not equal';
      }
    }
    ParameterSet parameterSet = ParameterSet.SSN_PARAMETER_SET;
    Boolean entityUpdateable = particle.FieldDefinition.EntityDefinition.RunningUserEntityAccess.IsUpdatable;
    Boolean fieldAccessible = fieldDefinition.RunningUserFieldAccess.IsAccessible;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesBroadSystemShapes.cls"),
	}}, schema.Schema{Objects: []schema.Object{{
		Name:   "Account",
		Fields: []schema.Field{{Name: "BillingAddress", Type: "Address"}},
	}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGeneratedSystemStubsAreCaseInsensitive(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesGeneratedStubCase.cls"), `
public class UsesGeneratedStubCase {
  public static void run(Account account) {
    httprequest request = new HTTPRequest();
    request.SETendpoint('callout:example');
    request.setMETHOD('GET');
    String header = request.GetHEADER('X-Test');

    apexpages.Message message = new APEXPages.Message(
      SEVERITY = apexpages.Severity.ERROR,
      SUMMARY = 'Summary',
      DETAIL = 'Detail'
    );

    Schema.DisplayType displayType = DisplayType.sTrInG;
    Schema.DisplayType schemaDisplayType = schema.DisplayType.PICKLIST;
    Schema.SObjectType token = Account.SObjectType;

    Database.QueryLocatorIterator locatorIterator = null;
    system.Iterator<SObject> systemIterator = locatorIterator;
    Iterator<SObject> shortIterator = locatorIterator;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesGeneratedStubCase.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectNamedConstructorsAndRunAsBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectConstructors.cls"), `
@IsTest
public class UsesSObjectConstructors {
  @IsTest
  static void run() {
    User user = new User(Id = '005000000000001AAA');
    System.runAs(user) {
      Account account = new Account(Name = 'Acme', NumberOfEmployees = 7);
      Contact contact = new Contact(LastName = 'Smith', AccountId = account.Id);
      List<SObject> records = new List<SObject>{ new Contact(LastName = 'Jones') };
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSObjectConstructors.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Contact"}, {Name: "User"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConcreteSObjectRelationshipAccessors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesConcreteSObjectAccessors.cls"), `
public class UsesConcreteSObjectAccessors {
  public static void run(Account record) {
    SObject parent = record.getSObject('Parent');
    List<SObject> children = record.getSObjects('Contacts');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesConcreteSObjectAccessors.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectRelationshipAssignmentAllowsReferencedRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AssignsRelationshipRecord.cls"), `
public class AssignsRelationshipRecord {
  public static void run(CartItem__c item) {
    RecordType recordType = [SELECT Id, Name FROM RecordType WHERE DeveloperName = 'Registration' LIMIT 1];
    item.RecordType = recordType;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "AssignsRelationshipRecord.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "CartItem__c"}}})

	model := buildSemaTypeMemberView(index)
	targetType := semaFieldScope(model, "CartItem__c", make(map[string]bool))[normalizeName("RecordType")]
	if targetType != "RecordType" {
		t.Fatalf("CartItem__c.RecordType type = %q", targetType)
	}
	if !semaAssignableToType(targetType, "RecordType", model) {
		t.Fatalf("%q should be assignable to RecordType", targetType)
	}
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "item.RecordType")
}

func TestAnalyzeStandardSObjectRelationshipAccessors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStandardSObjectAccessors.cls"), `
public class UsesStandardSObjectAccessors {
  public static void run(ContentDocumentLink documentLink) {
    Boolean matched = documentLink.ContentDocument.Title.startsWithIgnoreCase('Profile');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesStandardSObjectAccessors.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePolymorphicStandardSObjectRelationshipAccessors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPolymorphicSObjectAccessors.cls"), `
public class UsesPolymorphicSObjectAccessors {
  public static void run(Task task, ContentDocumentLink link) {
    Id whoId = task.Who.Id;
    Id whatId = task.What.Id;
    Id linkedEntityId = link.LinkedEntity.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesPolymorphicSObjectAccessors.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePolymorphicProjectSObjectRelationshipAccessors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPolymorphicProjectSObjectAccessors.cls"), `
public class UsesPolymorphicProjectSObjectAccessors {
  public static void run(Activity_Link__c link) {
    SObject explicitParent = link.Related_To__r;
    Id explicitId = link.Related_To__r.Id;
    SObject inferredParent = link.Subject__r;
    Id inferredId = link.Subject__r.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesPolymorphicProjectSObjectAccessors.cls"),
	}}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Contact"},
		{Name: "Activity_Link__c", Fields: []schema.Field{
			{Name: "Related_To__c", Type: "Lookup", ReferenceTo: []string{"Account", "Contact"}, RelationshipName: "Related_To__r"},
			{Name: "Subject__c", Type: "Lookup", ReferenceTo: []string{"Account", "Contact"}},
		}},
	}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaDescribeSObjectResultMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDescribe.cls"), `
public class UsesDescribe {
  public static void run(Schema.SObjectType token) {
    Schema.DescribeSObjectResult describe = token.getDescribe();
    String name = describe.getName();
    String label = describe.getLabel();
    String plural = describe.getLabelPlural();
    String prefix = describe.getKeyPrefix();
    Map<String, Schema.SObjectField> fields = describe.getFields();
    List<Schema.RecordTypeInfo> infos = describe.getRecordTypeInfos();
    Map<String, Schema.RecordTypeInfo> byName = describe.getRecordTypeInfosByName();
    Map<String, Schema.RecordTypeInfo> byDeveloperName = describe.getRecordTypeInfosByDeveloperName();
    List<Schema.ChildRelationship> children = describe.getChildRelationships();
    Schema.ChildRelationship child = children[0];
    String relationship = child.getRelationshipName();
    Schema.SObjectType childType = child.getChildSObject();
    Schema.SObjectField field = child.getField();
    Boolean cascade = child.isCascadeDelete();
    Boolean accessible = describe.isAccessible();
    Boolean creatable = describe.isCreateable();
    Boolean updateable = describe.isUpdateable();
    Boolean deletable = describe.isDeletable();
    Boolean queryable = describe.isQueryable();
    Boolean searchable = describe.isSearchable();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDescribe.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDateDatetimeStandardDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDates.cls"), `
public class UsesDates {
  public static void run() {
    Date today = Date.today();
    Date made = Date.newInstance(2026, 5, 7);
    Date parsed = Date.valueOf('2026-05-07');
    Date parsedObject = Date.valueOf((Object) '2026-05-07');
    Date due = System.today().addDays(30);
    Date nextMonth = due.addMonths(1);
    Date nextYear = due.addYears(1);
    Integer days = today.daysBetween(due);
    Integer day = due.day();
    Integer month = due.month();
    Integer year = due.year();
    String formattedDate = due.format();
    Datetime nowStamp = Datetime.now();
    Datetime stamp = Datetime.newInstance(2026, 5, 7, 1, 2, 3);
    Datetime stampFromMillis = Datetime.newInstance(stamp.getTime());
    Datetime stampFromParts = Datetime.newInstance(today, Time.newInstance(1, 2, 3, 0));
    Datetime gmtStamp = Datetime.newInstanceGmt(2026, 5, 7, 1, 2, 3);
    Datetime parsedStamp = Datetime.valueOfGmt('2026-05-07T01:02:03Z');
    Datetime later = stamp.addDays(1).addHours(2).addMinutes(3).addSeconds(4).addMilliseconds(5);
    Date localDate = later.date();
    Date gmtDate = later.dateGmt();
    Time localTime = later.time();
    Time gmtTime = later.timeGmt();
    String formatted = later.format('yyyy-MM-dd', 'UTC');
    String gmtFormatted = later.formatGmt('yyyy-MM-dd');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDates.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemLabelReferencesAsStrings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesLabels.cls"), `
public class UsesLabels {
  public class ResultData {
    public void addParameterField(String parameterType, String label, Object value) {}
  }
  public void run(ResultData resultData, Object value) {
    resultData.addParameterField('Name', System.Label.facilityResultsFacilityName, value);
    resultData.addParameterField('Name', Label.facilityResultsFacilityName, value);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesLabels.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeFieldsTokens(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectTypeFields.cls"), `
public class UsesSObjectTypeFields {
  public void run() {
    List<Schema.SObjectField> fields = new List<Schema.SObjectField>{
      Account.SObjectType.fields.name,
      Account.SObjectType.fields.ParentId,
      Account.SObjectType.fields.ownerId,
      Contact.SObjectType.fields.accountId,
      Lead.SObjectType.fields.FirstNAMe,
      Lead.SObjectType.fields.cOMPANY
    };
    Map<String, Schema.SObjectField> accountFields = Account.SObjectType.fields.getMap();
    Schema.SObjectField nameField = Account.SObjectType.fields.get('Name');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSObjectTypeFields.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA021" || diag.Code == "GLADESEMA008") && strings.Contains(diag.Message, "SObjectType.fields") {
			t.Fatalf("SObjectType.fields tokens should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeSObjectResultFieldsGetMapChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDescribeSObjectResultFields.cls"), `
public class UsesDescribeSObjectResultFields {
  public Schema.SObjectField run(Schema.DescribeSObjectResult describe) {
    Schema.SObjectField fieldToken = describe.fields.getMap().get('RecordTypeId');
    Schema.SObjectField methodToken = describe.getFields().get('Name');
    Schema.FieldSet fieldSetToken = describe.fieldSets.getMap().get('AccountSummary');
    Schema.FieldSet methodFieldSetToken = describe.getFieldSets().get('AccountSummary');
    return fieldToken == null ? methodToken : fieldToken;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDescribeSObjectResultFields.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedSObjectTypeToken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChainedSObjectType.cls"), `
public class UsesChainedSObjectType {
  public static void run() {
    Schema.SObjectType accountType = Account.SObjectType.SObjectType;
    Schema.SObjectType customMetadataType = Review_Object_Setup__mdt.SObjectType.SObjectType;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesChainedSObjectType.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Review_Object_Setup__mdt"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestExtractBodyForSemaSkipsCommentApostrophes(t *testing.T) {
	source := `public class Example {
  public static String run() {
    if (true) {
      // don't let this comment hide the nested block
      if (true) {
        return 'ok';
      }
    }
    return 'fallback';
  }
}`
	start := strings.Index(source, "public static String run")
	body, _, ok := extractBodyForSema(source, diagnostic.Range{
		Start: diagnostic.Position{Offset: start},
		End:   diagnostic.Position{Offset: len(source) - 1},
	})
	if !ok {
		t.Fatalf("expected body extraction to succeed")
	}
	if !strings.Contains(body, "return 'fallback';") {
		t.Fatalf("expected body to include final return, got %q", body)
	}
}

func TestBlockBoundsAtSkipsCommentApostrophes(t *testing.T) {
	body := `{
  if (true) {
    // don't let this comment hide the close brace
    String rec = 'first';
  }
  if (true) {
    String rec = 'second';
  }
}`
	pos := strings.LastIndex(body, "String rec")
	start, end := blockBoundsAt(body, pos)
	block := body[start:end]
	if strings.Contains(block, "first") {
		t.Fatalf("expected second block scope only, got %q", block)
	}
}

func TestAnalyzeMultilineSOQLDoesNotDeclareLocals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMultilineSOQL.cls"), `
public class UsesMultilineSOQL {
  public static void run(Id recordId) {
    Account updatedRec = [
      SELECT Id, Name
      FROM Account
      WHERE Id = :recordId
    ];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesMultilineSOQL.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" && (strings.Contains(diag.Message, "unknown type \"SELECT\"") || strings.Contains(diag.Message, "unknown type \"WHERE\"")) {
			t.Fatalf("SOQL query should not be treated as a local declaration: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSecurityStripInaccessibleDecision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSecurityStripInaccessible.cls"), `
public class UsesSecurityStripInaccessible {
  public static List<SObject> run(System.AccessType accessType, List<SObject> records) {
    System.SObjectAccessDecision decision = Security.stripInaccessible(accessType, records, false);
    Set<Integer> modified = decision.getModifiedIndexes();
    Map<String, Set<String>> removed = decision.getRemovedFields();
    return decision.getRecords();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSecurityStripInaccessible.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCoreSystemTypeAliases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCoreSystemTypes.cls"), `
public class UsesCoreSystemTypes implements System.Finalizer {
  public void execute(System.FinalizerContext context) {
    Id jobId = context.getAsyncApexJobId();
    String requestId = context.getRequestId();
  }
  public static void run(FieldSet shortFieldSet, Schema.FieldSet schemaFieldSet, System.AccessLevel accessLevel, System.AccessType accessType) {
    List<Schema.FieldSetMember> shortMembers = shortFieldSet.getFields();
    List<Schema.FieldSetMember> schemaMembers = schemaFieldSet.getFields();
    Schema.DisplayType shortType = shortMembers[0].getType();
    Schema.DisplayType schemaType = schemaMembers[0].getType();
    System.AccessLevel systemMode = System.AccessLevel.SYSTEM_MODE;
    AccessLevel userMode = AccessLevel.USER_MODE;
    System.AccessType readable = System.AccessType.READABLE;
    AccessType updatable = AccessType.UPDATABLE;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesCoreSystemTypes.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeExplicitSystemTypesBeatLocalShadows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Cookie.cls"), `
public class Cookie {
  public Cookie() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSystemCookie.cls"), `
public class UsesSystemCookie {
  public Integer run() {
    System.Cookie cookie = new System.Cookie('name', 'value', '/', 60, true);
    return cookie.getMaxAge();
  }
  public String path(System.Cookie cookie) {
    return cookie.getPath();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesSystemLocation.cls"), `
public class UsesSystemLocation {
  private class Location {
    public Location(System.Location location) {
      if (location == null) {
        return;
      }
      Latitude = location.getLatitude();
      Longitude = location.getLongitude();
    }
    public Double Latitude { get; private set; }
    public Double Longitude { get; private set; }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "GeoLocator.cls"), `
public class GeoLocator {
  public class Location {
    public Location(Decimal latitude, Decimal longitude) {}
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesDmlException.cls"), `
public class UsesDmlException {
  public Integer count(DmlException error) {
    return error.getNumDml();
  }
  public String message(DmlException error) {
    return error.getDmlMessage(0);
  }
  public Object kind(DmlException error) {
    return error.getDMLType(0);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "PKG",
		ApexFiles: []string{
			filepath.Join(root, "Cookie.cls"),
			filepath.Join(root, "UsesSystemCookie.cls"),
			filepath.Join(root, "UsesSystemLocation.cls"),
			filepath.Join(root, "GeoLocator.cls"),
			filepath.Join(root, "UsesDmlException.cls"),
		},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA011" || diag.Code == "GLADESEMA019" {
			t.Fatalf("unexpected explicit System type diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestSemaPlatformConstructorSignaturesUseStandardSymbols(t *testing.T) {
	for _, tc := range []struct {
		name string
		want [][]string
	}{
		{
			name: "System.Cookie",
			want: [][]string{
				{"String", "String", "String", "Integer", "Boolean"},
				{"String", "String", "String", "Integer", "Boolean", "String"},
				{"String", "String", "String", "Integer", "Boolean", "String", "Boolean"},
			},
		},
		{
			name: "System.HttpRequest",
			want: [][]string{{}},
		},
	} {
		got, ok := semaPlatformConstructorSignatures(tc.name)
		if !ok {
			t.Fatalf("semaPlatformConstructorSignatures(%q) not found", tc.name)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("semaPlatformConstructorSignatures(%q) = %#v, want %#v", tc.name, got, tc.want)
		}
	}
}

func TestAnalyzeCaseInsensitiveVariableBeatsSObjectToken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPermissionSetGroup.cls"), `
public class UsesPermissionSetGroup {
  private static void assignPermissionSetGroup(PermissionSetGroup permissionSetGroup) {
    if (PermissionSetGroup.Status != 'Updated') {
      Test.calculatePermissionSetGroup(PermissionSetGroup.Id);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPermissionSetGroup.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "PermissionSetGroup", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Status", Type: "String"}}}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeFieldsPathReturnsDescribeFieldResult(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSchemaSObjectTypeFields.cls"), `
public class UsesSchemaSObjectTypeFields {
  public void run() {
    Schema.DescribeFieldResult dfr = Schema.SObjectType.Account.fields.Name;
    Schema.SObjectField token = Account.Name;
    System.assert(dfr.getSObjectField() == token);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSchemaSObjectTypeFields.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "String"}}}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeFieldsStringProperties(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSchemaSObjectTypeFieldStrings.cls"), `
public class UsesSchemaSObjectTypeFieldStrings {
  public void run() {
    String label = Schema.SObjectType.Account.fields.Name.label;
    String name = Schema.SObjectType.Account.Fields.Name.Name;
    acceptString(Account.SObjectType.fields.Name.label);
  }

  private void acceptString(String value) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSchemaSObjectTypeFieldStrings.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "String"}}}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectTypeFieldSetsTokenPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSchemaSObjectTypeFieldSets.cls"), `
public class UsesSchemaSObjectTypeFieldSets {
  public void run() {
    List<Schema.FieldSetMember> direct = Schema.SObjectType.Account.fieldSets.AccountSummary.getFields();
    List<Schema.FieldSetMember> shortName = Account.SObjectType.FieldSets.AccountSummary.getFields();
    List<Schema.FieldSetMember> fromMap = Schema.SObjectType.Account.fieldSets.getMap().get('AccountSummary').getFields();
    List<Schema.FieldSetMember> fromGet = Schema.SObjectType.Account.fieldSets.get('AccountSummary').getFields();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSchemaSObjectTypeFieldSets.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectFieldsTokenGetDescribe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectFieldsToken.cls"), `
public class UsesSObjectFieldsToken {
  public String run() {
    return Settings__c.fields.Enabled__c.getDescribe().getName();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSObjectFieldsToken.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Settings__c", Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeRelationshipFieldTokenPathReturnsSObjectField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesRelationshipFieldToken.cls"), `
public class UsesRelationshipFieldToken {
  public Schema.SObjectField run() {
    return Thing__c.Account__c.Name;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesRelationshipFieldToken.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Thing__c", Fields: []schema.Field{{Name: "Account__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}},
		{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}},
	}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaEmailTemplateSObjectShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSchemaEmailTemplate.cls"), `
public class UsesSchemaEmailTemplate {
  public void run(Schema.EmailTemplate template) {
    Id idValue = template.Id;
    String name = template.Name;
    String namespacePrefix = template.NamespacePrefix;
    String developerName = template.DeveloperName;
    String subject = template.Subject;
    acceptTemplates(new List<Schema.EmailTemplate>{ template });
  }

  private void acceptTemplates(List<SObject> templates) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSchemaEmailTemplate.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNamespacedObjectAcceptsLocalFieldAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesNamespacedTemplate.cls"), `
public class UsesNamespacedTemplate {
  public String run(pkg__Template__c template) {
    List<pkg__Template__c> templates = [
      SELECT EmailRecipientFields__c
      FROM pkg__Template__c
    ];
    return template.EmailRecipientFields__c.split(',')[0];
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesNamespacedTemplate.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "pkg__Template__c",
		Fields: []schema.Field{
			{Name: "pkg__EmailRecipientFields__c", Type: "LongTextArea"},
		},
	}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedClassCanCallOuterStaticHelperOverObjectToString(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "OuterHelper.cls"), `
public class OuterHelper {
  private static String toString(Object value) {
    return String.valueOf(value);
  }

  private class Inner {
    public String run(Object value) {
      return toString(value);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "OuterHelper.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" && strings.Contains(diag.Message, "toString") {
			t.Fatalf("inner class should resolve outer static toString(Object): %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeImplicitHelperMethodsBeatPlatformObjectMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "HelperOverloads.cls"), `
public class HelperOverloads {
  private static String toString(Object value) {
    return String.valueOf(value);
  }

  private static Boolean equals(Object left, Object right) {
    return left == right;
  }

  public static Map<String, Object> withObj(Object key, Object value) {
    return new Map<String, Object>{ toString(key) => value };
  }

  public static Boolean same(Object left, Object right) {
    return equals(left, right);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "HelperOverloads.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFluentUserEqualsBeatsCollectionEquals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Query.cls"), `
public class Query {
  public enum LogicalOperator { OR_VALUE }
  public class Condition {
    public Condition() {}
    public Condition(LogicalOperator logicalOperator) {}
    public Condition equals(String fieldName, Object value) {
      return this;
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesQuery.cls"), `
public class UsesQuery {
  public Query.Condition run() {
    return new Query.Condition()
      .equals('Name', 'test')
      .equals('Reason', 'Other');
  }
  public Query.Condition runWithOperator() {
    return new Query.Condition(
        QUERY.LogicalOperator.OR_VALUE
      )
      .equals('Name', 'test')
      .equals('Reason', 'Other');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "acme",
		ApexFiles: []string{
			filepath.Join(root, "Query.cls"),
			filepath.Join(root, "UsesQuery.cls"),
		},
	}, schema.Schema{})
	sourceBytes, err := os.ReadFile(filepath.Join(root, "UsesQuery.cls"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	pos := strings.LastIndex(source, "equals")
	enrichedIndex := enrichIndexWithSchemaDerivedObjects(enrichIndexWithStandardSymbols(index))
	model := buildSemaTypeMemberView(enrichedIndex)
	receiverType, chainedMethod, ok := semaChainedCallReceiverNear(source, pos, "equals", map[string]string{semaCurrentTypeScopeKey: "UsesQuery"}, model, "UsesQuery")
	if !ok || chainedMethod != "equals" || receiverType != "Query.Condition" {
		t.Fatalf("chained receiver = (%q, %q, %v)", receiverType, chainedMethod, ok)
	}

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "equals") {
			t.Fatalf("user-defined fluent equals should beat collection equals: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStaticUserEqualsAcceptsSObjectFieldToken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Query.cls"), `
public class Query {
  public static Query equals(SObjectField field, Object predicate) {
    return new Query();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesQuery.cls"), `
public class UsesQuery {
  public Query run() {
    return Query.equals(Opportunity.IsWon, true);
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Query.cls"),
			filepath.Join(root, "UsesQuery.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "Opportunity", Fields: []schema.Field{{Name: "IsWon", Type: "Boolean"}}}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "equals") {
			t.Fatalf("static user-defined equals should beat platform equals: %#v", result.Diagnostics)
		}
	}
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeListSortComparator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesListSortComparator.cls"), `
public class UsesListSortComparator {
  public class AccountComparator implements Comparator<Account> {
    public Integer compare(Account left, Account right) {
      return 0;
    }
  }
  public static void run(List<Account> accounts) {
    accounts.sort(new AccountComparator());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesListSortComparator.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatacloudDuplicateResultTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatacloudDuplicateResult.cls"), `
public class UsesDatacloudDuplicateResult {
  public static List<Id> run(Database.DuplicateError duplicateError) {
    Datacloud.DuplicateResult duplicateResult = duplicateError.getDuplicateResult();
    List<Datacloud.MatchRecord> matchRecords = duplicateResult.getMatchResults()[0].getMatchRecords();
    List<Id> ids = new List<Id>();
    for (Datacloud.MatchRecord match : matchRecords) {
      ids.add(match.getRecord().Id);
    }
    return ids;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDatacloudDuplicateResult.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeURLStandardDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesURL.cls"), `
public class UsesURL {
  public String run() {
    return new URL(URL.getSalesforceBaseUrl(), '/apexrest/example').toExternalForm();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesURL.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeListSortSystemComparatorSObjectForConcreteSObjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesListSortComparator.cls"), `
public class UsesListSortComparator {
  public class AccountComparator implements System.Comparator<SObject> {
    public Integer compare(SObject left, SObject right) {
      return 0;
    }
  }
  public static void run(List<Account> accounts) {
    accounts.sort(new AccountComparator());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesListSortComparator.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectGetErrorsReturnsDatabaseErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectErrors.cls"), `
public class UsesSObjectErrors {
  public void run(Account account) {
    List<Database.Error> errors = account.getErrors();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSObjectErrors.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzePlatformExceptionSubtypeAssignableToException(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCacheException.cls"), `
public class UsesCacheException {
  private static Boolean log(Exception excp) {
    return true;
  }
  public void run() {
    try {
      return;
    } catch (Cache.Org.OrgCacheException oce) {
      log(oce);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesCacheException.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeHttpRequestGetters(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesHttpRequest.cls"), `
public class UsesHttpRequest {
  public class MockHttpCallout {
    public System.HttpRequest request;
  }
  public String run(HttpRequest request, MockHttpCallout mockCallout) {
    return request.getEndpoint() + ':' + request.getBody() + ':' + mockCallout.request.getEndpoint() + ':' + mockCallout.request.getBody();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesHttpRequest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNullCoalescingDoesNotReportSyntheticCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCoalesce.cls"), `
public class UsesCoalesce {
  public Integer run(Map<String, Integer> counts) {
    return (counts.get('spruce') ?? 0) + 1;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCoalesce.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "__coalesce") {
			t.Fatalf("null coalescing should not report synthetic call: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNullCoalescingKeepsConcreteReturnType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCoalesce.cls"), `
public class UsesCoalesce {
  public Config__mdt fallbackConfig;
  public Config__mdt pick(Config__mdt provided) {
    return provided ?? fallbackConfig;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCoalesce.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Config__mdt"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeQualifiedEnumFieldPathMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEnumMethods.cls"), `
public class UsesEnumMethods {
  private System.LoggingLevel entryLoggingLevel;
  private System.TriggerOperation context;
  public String run(LoggingContext loggingContext) {
    Integer current = this.entryLoggingLevel.ordinal();
    Integer user = loggingContext.userLoggingLevel.ordinal();
    return this.context.name() + ':' + loggingContext.userLoggingLevel.name() + ':' + current + ':' + user;
  }
  public class LoggingContext {
    public System.LoggingLevel userLoggingLevel;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEnumMethods.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemEnumMethodsWithProjectShadowClass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TriggerOperation.cls"), `
public class TriggerOperation {
  public static Date BEFORE_UPDATE;
}
`)
	writeSemaFile(t, filepath.Join(root, "LoggingLevel.cls"), `
public class LoggingLevel {
  public static Date INFO;
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesEnumMethods.cls"), `
public class UsesEnumMethods {
  private System.LoggingLevel entryLoggingLevel;
  public void run(LoggingContext loggingContext) {
    String name = loggingContext.userLoggingLevel.name();
    Integer ordinal = this.entryLoggingLevel.ordinal();
    System.TriggerOperation triggerOperationType = System.TriggerOperation.BEFORE_UPDATE;
  }
  public class LoggingContext {
    public System.LoggingLevel userLoggingLevel;
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "LoggingLevel.cls"),
			filepath.Join(root, "TriggerOperation.cls"),
			filepath.Join(root, "UsesEnumMethods.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringLiteralMethodAndSplitTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStringLiterals.cls"), `
public class UsesStringLiterals {
  public void run() {
    Integer code = 'Physical'.hashCode();
    List<String> parts = 'a,b,c'.split(',');
    List<String> noMatch = 'hello'.split(',');
    Boolean blank = String.isNotBlank('value');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStringLiterals.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStaticStringFieldTokensStayStrings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String StateFieldName = 'State__c';
  public static final String CartFieldName = 'Cart__c';
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesOrderFieldNames.cls"), `
public class UsesOrderFieldNames {
  public void run() {
    String state = Order.StateFieldName.toLowerCase();
    Boolean present = String.isNotBlank(Order.CartFieldName);
    Set<String> fieldsToQuery = new Set<String>();
    fieldsToQuery.add(Order.StateFieldName);
    fieldsToQuery.add(Order.CartFieldName.toLowerCase());
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Order.cls"),
			filepath.Join(root, "UsesOrderFieldNames.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTestVisibleStaticMapFieldChainedGet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EmailContent.cls"), `
public class EmailContent {
  @TestVisible private static Map<String, String> contentMap = new Map<String, String>();
  public static void addContent(String key, String value) {
    contentMap.put(key, value);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "EmailContentTest.cls"), `
@IsTest
private class EmailContentTest {
  private static final String TEST_KEY = 'key';
  private static final String TEST_VALUE = 'value';
  @IsTest
  private static void addContent_validKey_expectStored() {
    EmailContent.addContent(TEST_KEY, TEST_VALUE);
    System.assertEquals(TEST_VALUE, EmailContent.contentMap.get(TEST_KEY));
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "EmailContent.cls"),
			filepath.Join(root, "EmailContentTest.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFluentBuilderNestedStaticConditionCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Q.cls"), `
public class Q {
  public Q(Schema.SObjectType objectType) {}
  public Q selectFields(Set<String> fields) { return this; }
  public Q add(Condition condition) { return this; }
  public static Condition condition(String fieldName) { return new Condition(fieldName); }
  public class Condition {
    public Condition(String fieldName) {}
    public Condition isEqualTo(Object value) { return this; }
    public Condition isGreaterThan(Decimal value) { return this; }
    public Condition isNotIn(String bindExpression) { return this; }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String StateFieldName = 'State__c';
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesQ.cls"), `
public class UsesQ {
  public void run(Id entityId, Set<Id> recordIds) {
    Set<String> fieldsToQuery = new Set<String>();
    fieldsToQuery.add('CurrencyIsoCode');
    fieldsToQuery.add(Order.StateFieldName);
    Q query = new Q(Account.SObjectType)
      .selectFields(fieldsToQuery)
      .add(Q.condition('Entity__c').isEqualTo(entityId))
      .add(Q.condition('Balance__c').isGreaterThan(0))
      .add(Q.condition('Id').isNotIn(':recordIds'));
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Q.cls"),
			filepath.Join(root, "Order.cls"),
			filepath.Join(root, "UsesQ.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeExplicitCastsFromMapGetInitializeTypedLocals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMapCasts.cls"), `
public class UsesMapCasts {
  public void run(Map<String, Object> parsed, Map<Id, SObject> oldRecordMap, Map<Id, SObject> newRecordMap, Account account) {
    Map<String, Object> data = (Map<String, Object>) parsed.get('data');
    List<Object> currentParameters = (List<Object>) data.get('positiveParameters');
    Map<String, Object> record = (Map<String, Object>) data?.get('record');
    Account oldAccount = (Account) oldRecordMap.get(account.Id);
    Account newAccount = (Account) newRecordMap.get(account.Id);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMapCasts.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeExplicitCastsFromFluentBaseReturns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseFabricator.cls"), `
public virtual class BaseFabricator {
  protected BaseFabricator setDefaults() { return this; }
  public virtual BaseFabricator addChildren(String relationshipName, BaseFabricator child) { return this; }
}
`)
	writeSemaFile(t, filepath.Join(root, "AccountFabricator.cls"), `
public class AccountFabricator extends BaseFabricator {
  public AccountFabricator defaults() {
    return (AccountFabricator) super.setDefaults();
  }
  public AccountFabricator addChild(AccountFabricator child) {
    return (AccountFabricator) super.addChildren('Contacts', child);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "BaseBuilder.cls"), `
public virtual class BaseBuilder {
  protected BaseBuilder withData(String fieldName, Object value) { return this; }
}
`)
	writeSemaFile(t, filepath.Join(root, "AccountBuilder.cls"), `
public class AccountBuilder extends BaseBuilder {
  public AccountBuilder named(String name) {
    return (AccountBuilder) this.withData('Name', name);
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "BaseFabricator.cls"),
			filepath.Join(root, "AccountFabricator.cls"),
			filepath.Join(root, "BaseBuilder.cls"),
			filepath.Join(root, "AccountBuilder.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeQualifiedIterableEnhancedFor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesIterable.cls"), `
public class UsesIterable {
  public void run(System.Iterable<Id> recordIds) {
    for (Id recordId : recordIds) {
      String value = recordId;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesIterable.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUnbracedEnhancedForLocalVisibleInNestedLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesFieldSets.cls"), `
public class UsesFieldSets {
  public void run(List<Schema.FieldSet> fieldSets) {
    List<String> paths = new List<String>();
    for (Schema.FieldSet fieldSet : fieldSets)
      for (Schema.FieldSetMember fieldSetMember : fieldSet.getFields())
        paths.add(fieldSetMember.getFieldPath());
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFieldSets.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSchemaDerivedCustomMetadataAndShareFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMetadata.cls"), `
public class UsesMetadata {
  public String metadataField(Rollup__mdt meta) {
    return meta.LookupFieldOnLookupObject__c.toLowerCase();
  }
  public String setupOwner(LoggerSettings__c settingsRecord) {
    return settingsRecord.SetupOwner.Name + ':' + settingsRecord.SetupOwner.Type;
  }
  public void flowDefinitionView(Log__c log, Schema.FlowDefinitionView flowDefinition) {
    log.FlowLastModifiedByName__c = flowDefinition.LastModifiedBy;
    log.FlowLastModifiedByName__c = flowDefinition.OverriddenBy.LastModifiedBy;
    log.FlowLastModifiedByName__c = flowDefinition.OverriddenFlow.LastModifiedBy;
    log.FlowLastModifiedByName__c = flowDefinition.SourceTemplate.LastModifiedBy;
    log.FlowTriggerSObjectType__c = flowDefinition.TriggerObjectOrEvent.QualifiedApiName;
  }
  public Log__Share share(Log__c log, Id userId) {
    return new Log__Share(
      ParentId = log.Id,
      UserOrGroupId = userId,
      AccessLevel = 'Read',
      RowCause = Schema.Log__Share.RowCause.LoggedByUser__c
    );
  }
}
`)
	schemaDir := filepath.Join(root, "Schema")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(schemaDir, "FlowDefinitionView.cls"), `
public class FlowDefinitionView {
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(schemaDir, "FlowDefinitionView.cls"),
			filepath.Join(root, "UsesMetadata.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{
		{
			Name: "Rollup__mdt",
			Fields: []schema.Field{{
				Name: "LookupFieldOnLookupObject__c",
				Type: "MetadataRelationship",
			}},
		},
		{
			Name:               "LoggerSettings__c",
			CustomSettingsType: "Hierarchy",
		},
		{
			Name:         "Log__c",
			SharingModel: "Private",
			Fields: []schema.Field{
				{Name: "FlowLastModifiedByName__c", Type: "Text"},
				{Name: "FlowTriggerSObjectType__c", Type: "Text"},
			},
		},
	}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStandardShareAccessFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStandardShare.cls"), `
public class UsesStandardShare {
  public void run() {
    AccountShare share = new AccountShare(
      AccountId = UserInfo.getUserId(),
      UserOrGroupId = UserInfo.getUserId(),
      AccountAccessLevel = 'Read',
      CaseAccessLevel = 'Read',
      ContactAccessLevel = 'Read',
      OpportunityAccessLevel = 'Read',
      RowCause = 'Manual'
    );
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStandardShare.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeOverloadPrefersIterableIdOverSObjectList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesBuilder.cls"), `
public class UsesBuilder {
  public class Builder {
    public Builder setRecord(SObject record) { return this; }
    public Builder setRecord(List<SObject> records) { return this; }
    public Builder setRecord(System.Iterable<Id> recordIds) { return this; }
  }
  public void run(Builder builder, List<Id> ids, AggregateResult aggregateResult) {
    builder.setRecord(ids);
    builder.setRecord(aggregateResult);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesBuilder.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConnectApiOrganizationSettingsOrgIdIsId(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesConnectApi.cls"), `
public class UsesConnectApi {
  public void run(Config__c config) {
    config.SetupOwnerId = ConnectApi.Organization.getSettings().orgId;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesConnectApi.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name:               "Config__c",
		CustomSettingsType: "Hierarchy",
	}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNullCoalescingReturnKeepsConcreteType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCoalesceReturn.cls"), `
public class UsesCoalesceReturn {
  private static RollupPluginParameter__mdt parameterMock;
  public RollupPluginParameter__mdt getParameterInstance(String developerNameOrId) {
    return parameterMock ?? RollupPluginParameter__mdt.getInstance(developerNameOrId);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCoalesceReturn.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "RollupPluginParameter__mdt"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeObjectInitializerTernaryDoesNotUseNamedFieldAsCondition(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesInitializerTernary.cls"), `
public class UsesInitializerTernary {
  public void run(Boolean cond) {
    List<Rollup__mdt> records = new List<Rollup__mdt>{
      new Rollup__mdt(
        Name = 'row',
        CalcItemWhereClause__c = cond ? 'CurrencyIsoCode != null' : null
      )
    };
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesInitializerTernary.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Rollup__mdt",
		Fields: []schema.Field{
			{Name: "CalcItemWhereClause__c", Type: "LongTextArea"},
		},
	}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStaticCallWithListConstructedFromSetArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSetListArg.cls"), `
public class UsesSetListArg {
  public class Rollup {
    public enum InvocationPoint { FROM_FULL_RECALC_FLOW }
    public static String performBulkFullRecalcWithParentIds(String serializedMetadata, String invokePointName, List<String> parentIds) {
      return '';
    }
  }
  public void run(Set<String> optionalParentIds) {
    String enqueuedJobId = Rollup.performBulkFullRecalcWithParentIds(
      JSON.serialize(new List<String>()),
      Rollup.InvocationPoint.FROM_FULL_RECALC_FLOW.name(),
      new List<String>(optionalParentIds)
    );
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSetListArg.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCollectionConstructorsWithArrayElementGenericTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Address__c.cls"), `public class Address__c {}`)
	writeSemaFile(t, filepath.Join(root, "UsesArrayElementGenerics.cls"), `
public class UsesArrayElementGenerics {
  public void run() {
    Map<Id, Address__c[]> addressesById = new Map<Id, Address__c[]>();
    List<Address__c[]> addressGroups = new List<Address__c[]>();
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Address__c.cls"),
			filepath.Join(root, "UsesArrayElementGenerics.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "Address__c"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "with Boolean")
	assertNoDiagnosticContaining(t, result, "GLADESEMA025", "Map<Id, Address__c[]>")
}

func TestAnalyzeMultilineSOQLDoesNotReportClauseKeywordsAsCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMultilineSOQL.cls"), `
public class UsesMultilineSOQL {
  public void run(Set<Id> ids) {
    List<Account> rows = [SELECT Id FROM Account
      WHERE (Id IN :ids)
      AND (Name != null)];
    AggregateResult ag = [SELECT Count(Id) cnt FROM Opportunity
      WHERE (AccountId IN :ids)
      AND (IsWon = true)];
    for (Account account : [SELECT Id FROM Account
      WHERE (Id IN :ids)
      AND (Name != null)]) {
      System.debug(account.Id);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMultilineSOQL.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "WHERE")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "AND")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "IN")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "AggregateResult")
}

func TestAnalyzeEnhancedForSOQLWithRelationshipFieldsSkipsClauseKeywords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesRelationshipSOQL.cls"), `
public class UsesRelationshipSOQL {
  public void run(Set<Id> setParentId, Set<Id> setExistingAlloId) {
    for (Allocation__c allo : [SELECT Id, Payment__c, Payment__r.npe01__Payment_Amount__c, Payment__r.npe01__Paid__c, Payment__r.npe01__Written_Off__c, Opportunity__c, Opportunity__r.Amount, Amount__c, Percent__c, General_Accounting_Unit__c, Recurring_Donation__c, Campaign__c FROM Allocation__c
      WHERE (Payment__c IN :setParentId OR Opportunity__c IN :setParentId OR Recurring_Donation__c IN :setParentId OR Campaign__c IN :setParentId)
      AND Id NOT IN :setExistingAlloId]) {
      System.debug(allo.Id);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesRelationshipSOQL.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Allocation__c", Fields: []schema.Field{
			{Name: "Payment__c", Type: "Lookup", ReferenceTo: []string{"npe01__OppPayment__c"}, RelationshipName: "Payment__r"},
			{Name: "Opportunity__c", Type: "Lookup", ReferenceTo: []string{"Opportunity"}, RelationshipName: "Opportunity__r"},
			{Name: "Amount__c", Type: "Currency"},
			{Name: "Percent__c", Type: "Percent"},
			{Name: "General_Accounting_Unit__c", Type: "Lookup", ReferenceTo: []string{"Account"}},
			{Name: "Recurring_Donation__c", Type: "Lookup", ReferenceTo: []string{"Account"}},
			{Name: "Campaign__c", Type: "Lookup", ReferenceTo: []string{"Campaign"}},
		}},
		{Name: "npe01__OppPayment__c", Fields: []schema.Field{
			{Name: "npe01__Payment_Amount__c", Type: "Currency"},
			{Name: "npe01__Paid__c", Type: "Checkbox"},
			{Name: "npe01__Written_Off__c", Type: "Checkbox"},
		}},
	}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "WHERE")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "AND")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "IN")
}

func TestAnalyzeCastedStringComparisonIsBoolean(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCastedComparison.cls"), `
public class UsesCastedComparison {
  public void run(SObject left, SObject right) {
    Boolean isChanged = false;
    isChanged = (String) left.get('CurrencyIsoCode') != (String) right.get('CurrencyIsoCode');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCastedComparison.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "assigns String to Boolean")
}

func TestAnalyzeCustomOrderPropertyDoesNotCollapseToStandardOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "OrderItem.cls"), `
public class OrderItem {
  private SObject record;

  public SObject getRecord() {
    return record;
  }

  public Order__c Order {
    get { return ((OrderItem__c) getRecord()).Order__r; }
    private set;
  }

  public void run() {
    OrderItem wrapper = new OrderItem();
    Order__c receivedOrder = wrapper.Order;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "OrderItem.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Order__c"},
		{Name: "OrderItem__c", Fields: []schema.Field{{
			Name:             "Order__c",
			Type:             "Lookup",
			ReferenceTo:      []string{"Order__c"},
			RelationshipName: "Order",
		}}},
	}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Order__c")
}

func TestAnalyzeOverrideSkipsMissingExternalSuperclass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMissingBase.cls"), `
public class UsesMissingBase extends pkg.MissingBase {
  public override String getName() {
    return 'name';
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMissingBase.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA016", "override")
}

func TestAnalyzeFieldPathSkipsMissingExternalReceiverType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMissingDependencyField.cls"), `
public class UsesMissingDependencyField {
  private pkg.Request request;

  public void run() {
    if (String.isNotBlank(this.request.CreditCardNumber)) {
      withPaymentToken(this.request.PaymentToken);
    }
  }

  private void withPaymentToken(String token) {
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMissingDependencyField.cls")},
	}, schema.Schema{})
	index.Types = append(index.Types, typesys.TypeSymbol{
		Kind:       apexast.DeclarationClass,
		Name:       "Request",
		Namespace:  "pkg",
		Dependency: true,
	})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA021", "request.PaymentToken")
	assertNoDiagnosticContaining(t, result, "GLADESEMA021", "request.CreditCardNumber")
}

func TestAnalyzeSignedIntegerLiteralReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSignedLiteral.cls"), `
public class UsesSignedLiteral {
  public Integer compare(Boolean left, Boolean right) {
    if (!left && right) return -1;
    else if (left == right) return 0;
    else return 1;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSignedLiteral.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA019", "Integer")
}

func TestAnalyzeInlineSOQLDMLChoosesListOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesInlineSOQLDML.cls"), `
public class UsesInlineSOQLDML {
  public void run() {
    Database.delete([SELECT Id FROM Account WHERE Name = 'Acme']);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesInlineSOQLDML.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA022", "Database.delete")
}

func TestAnalyzeSOSLReturningFieldsAreNotCallArgs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSOSLReturning.cls"), `
public class UsesSOSLReturning {
  public void run(String queryValue) {
    if (queryValue != null) {
      // don't let comments before bracketed queries confuse text scanning
      List<List<SObject>> searchResults = [FIND :queryValue IN NAME FIELDS RETURNING Contact(Id, Name) LIMIT 100];
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSOSLReturning.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA013", "Id")
	assertNoDiagnosticContaining(t, result, "GLADESEMA013", "Name")
}

func TestAnalyzeMapGetAcceptsCurrentClassStringConstant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMapConstant.cls"), `
public class UsesMapConstant {
  private static final String RET_URL = 'retURL';
  public String run(Map<String, String> params) {
    return params.get(RET_URL);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMapConstant.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "get")
	assertNoDiagnosticContaining(t, result, "GLADESEMA013", "RET_URL")
}

func TestAnalyzeMapGetAndRemoveAcceptCurrentClassStringConstantInsideCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMapConstantCalls.cls"), `
public class UsesMapConstantCalls {
  private static final String RET_URL = 'retURL';
  public void run(Map<String, String> params, PageReference pageRef) {
    System.assertEquals(params.get(RET_URL), pageRef.getUrl(), 'message');
    params.remove(RET_URL);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMapConstantCalls.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "get")
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "remove")
}

func TestAnalyzeTestMethodMapGetAndRemoveAcceptPrivateStaticStringConstant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesTestMethodMapConstant.cls"), `
@isTest
private class UsesTestMethodMapConstant {
  private static final String PARAM_RETURL = 'retURL';
  private static final String PARAM_RETURL_FAIL = 'failRetURL';
  private static testMethod void testOnCancel() {
    Map<String, String> params = new Map<String, String>{
      PARAM_RETURL => '006/o',
      PARAM_RETURL_FAIL => '001/o'
    };
    PageReference cancelPage = new PageReference('/apex/test');
    System.assertEquals(params.get(PARAM_RETURL_FAIL), cancelPage.getUrl(), 'message');
    params.remove(PARAM_RETURL_FAIL);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesTestMethodMapConstant.cls")},
	}, schema.Schema{})
	model := buildSemaTypeMemberView(enrichIndexWithSchemaDerivedObjects(enrichIndexWithStandardSymbols(index)))
	fields := semaFieldScope(model, "UsesTestMethodMapConstant", make(map[string]bool))
	if got := fields[normalizeName("PARAM_RETURL_FAIL")]; got != "String" {
		t.Fatalf("PARAM_RETURL_FAIL field type = %q, want String", got)
	}
	sourceBytes, err := os.ReadFile(filepath.Join(root, "UsesTestMethodMapConstant.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range index.Types {
		if typ.Name != "UsesTestMethodMapConstant" {
			continue
		}
		for _, member := range typ.Members {
			if member.Name != "testOnCancel" {
				continue
			}
			body, bodyOffset, ok := extractBodyForSema(string(sourceBytes), member.Range)
			if !ok {
				t.Fatalf("method body not found")
			}
			baseScope := map[string]string{semaCurrentTypeScopeKey: typ.Name}
			for name, fieldType := range fields {
				baseScope[name] = fieldType
			}
			scopes, _ := (&Analyzer{}).collectBodyScopes(typ, member, body, bodyOffset, string(sourceBytes), baseScope, model)
			if got, ok := scopes.visibleAt("params", strings.Index(body, "params.get")); !ok || got != "Map<String,String>" {
				t.Fatalf("params scope at get = %q/%v, want Map<String,String>", got, ok)
			}
		}
	}
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "get")
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "remove")
}

func TestAnalyzeNestedUtilityToStringOverloadBeatsPlatformToString(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesNestedUtility.cls"), `
public class UsesNestedUtility {
  private static final Converter CONVERTER = new Converter();

  public class Holder {
    public String join() {
      StringBuilder builder = new StringBuilder();
      builder.append('a');
      return builder.toString(',');
    }
    public String convert(Object value) {
      return CONVERTER.toString(value);
    }
    public String convertViaType(Object value) {
      return UsesNestedUtility.Converter.toString(value);
    }
  }

  public class StringBuilder {
    private List<String> values = new List<String>();
    public void append(String value) {
      values.add(value);
    }
    public override String toString() {
      return String.join(values, '');
    }
    public String toString(String separator) {
      return String.join(values, separator);
    }
  }

  public class Converter {
    public String toString(Object input) {
      return String.valueOf(input);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesNestedUtility.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "toString")
}

func TestAnalyzeDecimalRoundWithRoundingMode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDecimalRound.cls"), `
public class UsesDecimalRound {
  public Decimal run(Decimal value) {
    return value.round(System.RoundingMode.HALF_UP);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDecimalRound.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "round")
}

func TestAnalyzeDecimalRoundOnParenthesizedNumericExpression(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDecimalRoundExpression.cls"), `
public class UsesDecimalRoundExpression {
  public Decimal run(Decimal rtStart, Decimal multiplier) {
    Decimal rt = ((rtStart + ((Math.random()/100) * multiplier))*1000000).round(System.RoundingMode.HALF_UP);
    return rt;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDecimalRoundExpression.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA023", "round")
}

func TestAnalyzeSOQLRollupGroupByDoesNotReportCallArgs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSOQLRollup.cls"), `
public class UsesSOQLRollup {
  public void run(Set<Id> ids) {
    // RD's rollup query should stay hidden from call-arg checks.
    for (SObject obj : [
      SELECT COUNT(Id) oppcount, npe03__Recurring_Donation__c rdid, IsWon
      FROM Opportunity
      WHERE npe03__Recurring_Donation__c IN :ids
      GROUP BY rollup(npe03__Recurring_Donation__c, IsWon)
    ]) {
      Id rdid = (Id)obj.get('rdid');
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSOQLRollup.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Opportunity", Fields: []schema.Field{{Name: "npe03__Recurring_Donation__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA013", "npe03__Recurring_Donation__c")
}

func TestAnalyzeNamespacedSelfLookupChildRelationshipAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesRefunds.cls"), `
public class UsesRefunds {
  public void run(npe01__OppPayment__c originalPayment) {
    for (npe01__OppPayment__c refund : originalPayment.Refunds__r) {
      Id refundId = refund.Id;
    }
  }
}
`)
	index := typesys.Build(
		project.Project{
			Root:      root,
			Namespace: "pkg",
			ApexFiles: []string{filepath.Join(root, "UsesRefunds.cls")},
		},
		schema.Schema{Objects: []schema.Object{{
			Name: "npe01__OppPayment__c",
			Fields: []schema.Field{{
				Name:                  "pkg__OriginalPayment__c",
				Type:                  "Lookup",
				ReferenceTo:           []string{"npe01__OppPayment__c"},
				RelationshipName:      "pkg__OriginalPayment__r",
				ChildRelationshipName: "pkg__Refunds__r",
			}},
		}}},
	)
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA024", "Refunds__r")
	assertNoDiagnosticContaining(t, result, "GLADESEMA024", "Refund__c")
}

func TestAnalyzeSObjectTypeTokenMatchesDescribeSObjectResult(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDescribeToken.cls"), `
public class UsesDescribeToken {
  public void accept(Schema.DescribeSObjectResult describe) {}
  public void run() {
    Schema.DescribeSObjectResult describe = Schema.SObjectType.Opportunity;
    accept(Schema.SObjectType.Opportunity);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDescribeToken.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Opportunity"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA009", "accept")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "describe")
}

func TestAnalyzeSchemaQualifiedCustomSObjectMatchesCustomSObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSchemaCustomObject.cls"), `
public class UsesSchemaCustomObject {
  public Schema.Funding_Request__c selected(Id reqId) {
    Schema.Funding_Request__c fundingRequest = new Schema.Funding_Request__c();
    for (Schema.Funding_Request__c nextRequest : load(reqId)) {
      fundingRequest = nextRequest;
    }
    final Schema.Funding_Request__c record = [
      SELECT Id
      FROM Funding_Request__c
      LIMIT 1
    ];
    return record;
  }
  public List<Funding_Request__c> load(Id reqId) {
    return new List<Funding_Request__c>();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSchemaCustomObject.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Funding_Request__c"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "record")
	assertNoDiagnosticContaining(t, result, "GLADESEMA024", "nextRequest")
}

func TestAnalyzeUnknownExternalFieldTypeStopsFieldPathCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesExternalField.cls"), `
public class UsesExternalField {
  private pkg.Request request;
  private pkg.Product wrappedProduct;
  private MissingRetriever retriever;
  public Object run() {
    return this.request.Amount;
  }
  public Object call() {
    return this.wrappedProduct.getName();
  }
  public Object localCall() {
    return this.retriever.get();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesExternalField.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA021", "request.Amount")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "wrappedProduct.getName")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "retriever.get")
}

func TestAnalyzeEventSObjectTokensPreferStandardSObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEventTokens.cls"), `
public class UsesEventTokens {
  public static void acceptType(SObjectType token) {}
  public static void acceptSchemaType(Schema.SObjectType token) {}
  public static void acceptField(SObjectField field) {}
  public static void acceptSchemaField(Schema.SObjectField field) {}

  public void run() {
    acceptType(Event.SObjectType);
    acceptSchemaType(Event.SObjectType);
    acceptField(Event.ActivityDateTime);
    acceptSchemaField(Event.ActivityDatetime);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEventTokens.cls")},
	}, schema.Schema{})
	model := buildSemaTypeMemberView(index)
	scope := map[string]string{semaCurrentTypeScopeKey: "UsesEventTokens"}
	if got := inferSemaArgTypeWithModel("Event.SObjectType", scope, model); got != "Schema.SObjectType" {
		t.Fatalf("Event.SObjectType type = %q, want Schema.SObjectType", got)
	}
	if got := inferSemaArgTypeWithModel("Event.ActivityDateTime", scope, model); got != "Schema.SObjectField" {
		t.Fatalf("Event.ActivityDateTime type = %q, want Schema.SObjectField", got)
	}
	irScope := newIRSemaScope(scope)
	eventExpr := ir.Expr{Kind: ir.ExprVariable, Name: "Event"}
	if got := NewAnalyzer().inferIRExprType(ir.Expr{Kind: ir.ExprVariable, Name: "Event.SObjectType"}, irScope, model, "UsesEventTokens"); got != "Schema.SObjectType" {
		t.Fatalf("IR variable Event.SObjectType type = %q, want Schema.SObjectType", got)
	}
	if got := NewAnalyzer().inferIRExprType(ir.Expr{Kind: ir.ExprVariable, Name: "Event.ActivityDateTime"}, irScope, model, "UsesEventTokens"); got != "Schema.SObjectField" {
		t.Fatalf("IR variable Event.ActivityDateTime type = %q, want Schema.SObjectField", got)
	}
	if got := NewAnalyzer().inferIRExprType(ir.Expr{Kind: ir.ExprCall, Callee: "__field:SObjectType", Left: &eventExpr}, irScope, model, "UsesEventTokens"); got != "Schema.SObjectType" {
		t.Fatalf("IR Event.SObjectType type = %q, want Schema.SObjectType", got)
	}
	if got := NewAnalyzer().inferIRExprType(ir.Expr{Kind: ir.ExprCall, Callee: "__field:ActivityDateTime", Left: &eventExpr}, irScope, model, "UsesEventTokens"); got != "Schema.SObjectField" {
		t.Fatalf("IR Event.ActivityDateTime type = %q, want Schema.SObjectField", got)
	}
	candidates := resolveImplicitMemberMethods(model, "UsesEventTokens", "acceptType")
	if _, ok, _ := bestResolvedMemberByArgTypes(candidates, []string{"Schema.SObjectType"}, model); !ok {
		t.Fatalf("acceptType candidates %#v do not accept Schema.SObjectType", candidates)
	}
	for _, tc := range []struct {
		method string
		arg    string
	}{
		{"acceptSchemaType", "Schema.SObjectType"},
		{"acceptField", "Schema.SObjectField"},
		{"acceptSchemaField", "Schema.SObjectField"},
	} {
		if _, ok, _ := bestResolvedMemberByArgTypes(resolveImplicitMemberMethods(model, "UsesEventTokens", tc.method), []string{tc.arg}, model); !ok {
			t.Fatalf("%s candidates do not accept %s", tc.method, tc.arg)
		}
	}
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStandardEventIncludesIsClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEvent.cls"), `
public class UsesEvent {
  public void run() {
    List<Event> events = [SELECT Id, IsClosed FROM Event WHERE IsClosed = false];
    Schema.SObjectField fieldToken = Event.IsClosed;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEvent.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA_QUERY_FIELD" || diag.Code == "GLADESEMA021" || diag.Code == "GLADESEMA018" {
			t.Fatalf("unexpected Event.IsClosed diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTestIsRunningTestStaticCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesTest.cls"), `
public class UsesTest {
  public Boolean run() {
    return Test.isRunningTest();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesTest.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTestIsRunningTestStaticCallInNamespacedProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesTest.cls"), `
public class UsesTest {
  public Boolean run() {
    return Test.isRunningTest();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "dlrs",
		ApexFiles: []string{filepath.Join(root, "UsesTest.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTestIsRunningTestPrefersPlatformClassOverInheritedField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {
  public static TestFactory Test { get; private set; }

  public class TestFactory {
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesTest.cls"), `
public class UsesTest extends Base {
  public Boolean run() {
    return Test.isRunningTest();
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Base.cls"),
			filepath.Join(root, "UsesTest.cls"),
		},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" {
			t.Fatalf("platform Test.isRunningTest should not be resolved through inherited field: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectClonePreservesConcreteReceiverType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectClone.cls"), `
public class UsesSObjectClone {
  public Contact cloneContact(Contact originalContact) {
    Contact newContact = originalContact.clone();
    return newContact;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSObjectClone.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Contact"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "Contact local")
}

func TestAnalyzeNamespacedContactCheckboxAndFinalClone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesContactFields.cls"), `
public inherited sharing class UsesContactFields {
  private Boolean isExcludedFromName;
  public UsesContactFields(Contact con) {
    this.isExcludedFromName = con.Exclude_from_Household_Name__c;
  }
  public Contact cloneContact() {
    final Contact originalContact = new Contact(MailingStreet = '\n');
    Contact newContact = originalContact.clone();
    return newContact;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "pkg",
		ApexFiles: []string{filepath.Join(root, "UsesContactFields.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Contact",
		Fields: []schema.Field{
			{Name: "Exclude_from_Household_Name__c", Type: "Checkbox"},
		},
	}}})
	enrichedIndex := enrichIndexWithSchemaDerivedObjects(enrichIndexWithStandardSymbols(index))
	model := buildSemaTypeMemberView(enrichedIndex)
	foundConParam := false
	for _, typ := range index.Types {
		if typ.Name != "UsesContactFields" {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationConstructor && len(member.Parameters) == 1 && member.Parameters[0].Name == "con" {
				foundConParam = true
			}
		}
	}
	if !foundConParam {
		t.Fatalf("constructor parameter con was not indexed")
	}
	scope := map[string]string{
		semaCurrentTypeScopeKey:          "UsesContactFields",
		normalizeName("con"):             "Contact",
		normalizeName("originalContact"): "Contact",
	}
	if got := inferSemaArgTypeWithModel("con.Exclude_from_Household_Name__c", scope, model); got != "Boolean" {
		t.Fatalf("contact checkbox type = %q, want Boolean", got)
	}
	if got := inferSemaArgTypeWithModel("originalContact.clone()", scope, model); got != "Contact" {
		t.Fatalf("contact clone type = %q, want Contact", got)
	}
	if got := resolveNestedTypeReference(model, "UsesContactFields", "Contact"); got != "Contact" {
		t.Fatalf("resolved Contact = %q, want Contact", got)
	}
	irScope := newIRSemaScope(scope)
	if got := (&Analyzer{}).inferIRExprType(ir.Expr{Kind: ir.ExprVariable, Name: "con.Exclude_from_Household_Name__c"}, irScope, model, "UsesContactFields"); got != "Boolean" {
		t.Fatalf("IR contact checkbox type = %q, want Boolean", got)
	}
	sourceBytes, err := os.ReadFile(filepath.Join(root, "UsesContactFields.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range enrichedIndex.Types {
		if typ.Name != "UsesContactFields" {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationConstructor {
				continue
			}
			body, bodyOffset, ok := extractBodyForSema(string(sourceBytes), member.Range)
			if !ok {
				t.Fatalf("constructor body not found")
			}
			normalizedMember := semaNormalizeMemberTypes(model, typ.Name, member)
			baseScope := map[string]string{semaCurrentTypeScopeKey: typ.Name}
			for name, fieldType := range semaFieldScope(model, typ.Name, make(map[string]bool)) {
				baseScope[name] = fieldType
			}
			for _, param := range normalizedMember.Parameters {
				baseScope[normalizeName(param.Name)] = param.Type
			}
			diags := (&Analyzer{}).checkBodyIR(typ, normalizedMember, body, bodyOffset, string(sourceBytes), baseScope, model, buildConstructability(enrichedIndex))
			assertNoDiagnosticContaining(t, Result{Diagnostics: diags}, "GLADESEMA018", "isExcludedFromName")
		}
	}
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "isExcludedFromName")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "newContact")
}

func TestAnalyzeMapStringSObjectConstructorFromList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMapFromList.cls"), `
public class UsesMapFromList {
  public Map<String, Contact> run(List<Contact> contacts) {
    return new Map<String, Contact>(contacts);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMapFromList.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Contact"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA025", "Map<String,Contact>")
}

func TestAnalyzeEnhancedForLocalDoesNotShadowClassAfterLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesLoopShadow.cls"), `
public class UsesLoopShadow {
  public class Rollup {
    public enum InvocationPoint { FROM_FULL_RECALC_FLOW }
    public static String performBulkFullRecalcWithParentIds(String serializedMetadata, String invokePointName, List<String> parentIds) {
      return '';
    }
  }
  public void run(List<String> values, Set<String> optionalParentIds) {
    for (String rollup : values) {
      String copy = rollup;
    }
    String enqueuedJobId = Rollup.performBulkFullRecalcWithParentIds(
      JSON.serialize(values),
      Rollup.InvocationPoint.FROM_FULL_RECALC_FLOW.name(),
      new List<String>(optionalParentIds)
    );
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesLoopShadow.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultilineLocalConstructorDeclarationStaysInScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMultilineLocal.cls"), `
public class UsesMultilineLocal {
  public class Processor {
    public Processor(String one, String two) {}
  }
  public void run(Boolean shouldClear) {
    Processor parentResetProcessor = new Processor(
      'one',
      'two'
    );
    if (shouldClear) {
      parentResetProcessor = null;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMultilineLocal.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultilineLocalConstructorDeclarationAcrossElseIf(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMultilineLocal.cls"), `
public class UsesMultilineLocal {
  public enum InvocationPoint { FROM_FULL_RECALC_FLOW, FROM_FULL_RECALC_LWC }
  public class Processor {
    public Processor(List<String> records, String parentType, String query, Set<String> ids, InvocationPoint invokePoint) {}
  }
  public Processor run(Boolean isFullRecalcApp, Boolean shouldSkip, List<String> records, Set<String> ids, InvocationPoint invokePoint) {
    String parentType = 'Account';
    String parentQuery = 'SELECT Id FROM Account';
    Processor parentResetProcessor = new Processor(
      records,
      parentType,
      parentQuery,
      // duplicate the record ids before full recalc also receives them
      new Set<String>(ids),
      invokePoint
    );
    if (records.isEmpty() == false && isFullRecalcApp && shouldSkip == false) {
      return parentResetProcessor;
    } else if (
      invokePoint != InvocationPoint.FROM_FULL_RECALC_LWC ||
      shouldSkip ||
      ids.isEmpty() == false
    ) {
      parentResetProcessor = null;
    }
    return parentResetProcessor;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMultilineLocal.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNumericSetScaleAndSObjectErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPlatformMethods.cls"), `
public class UsesPlatformMethods {
  public void run(Account accountRecord, Decimal price) {
    Decimal rounded = price.setScale(2);
    Long asLong = rounded.longValue();
    Boolean hasErrors = accountRecord.hasErrors();
    Boolean noErrors = accountRecord.getErrors().isEmpty();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPlatformMethods.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, "setScale") || strings.Contains(diag.Message, "getErrors") || strings.Contains(diag.Message, "hasErrors")) {
			t.Fatalf("platform methods should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDateArithmeticAndSObjectCloneOverloads(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TermInfo.cls"), `
public class TermInfo {
  public Date StartDate { get; set; }
  public List<TermInfo> Children { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesCloneAndDate.cls"), `
public class UsesCloneAndDate {
  public void run(Account accountRecord) {
    Date endDate = Date.today().addYears(1).addDays(-1);
    SObject shallow = accountRecord.clone(false, true);
    SObject deep = accountRecord.clone(false, true, false);
    TermInfo info = new TermInfo();
    info.StartDate = Date.today();
    Date nestedEnd = info.StartDate.addYears(1).addDays(-1);
    info.Children = new List<TermInfo>();
    info.Children.add(info);
  }
  private class Calculator {
    public TermInfo Calculate(List<Account> accounts) {
      TermInfo info = new TermInfo();
      info.StartDate = Date.today();
      info.Children = new List<TermInfo>();
      for (Account accountRecord : accounts) {
        info.Children.add(info);
        Date nestedEnd = info.StartDate.addYears(1).addDays(-1);
      }
      return info;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "TermInfo.cls"), filepath.Join(root, "UsesCloneAndDate.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA023") && (strings.Contains(diag.Message, "addYears") || strings.Contains(diag.Message, "addDays") || strings.Contains(diag.Message, "clone")) {
			t.Fatalf("date arithmetic and SObject clone overloads should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAdditionalPlatformSeams(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CustomException.cls"), `
public class CustomException extends Exception {
  public CustomException(String message, Boolean display) {
    this(message);
  }
  public String getUserMessage() {
    return getMessage();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesPlatformSeams.cls"), `
public class UsesPlatformSeams {
  public PageReference run(Folder folderRecord, Note noteRecord, PermissionSetAssignment assignment, User userRecord, Date startDate, String label, SalesTaxRequest request) {
    Integer compared = label.compareTo('next');
    Integer months = startDate.monthsBetween(Date.today());
    Id folderId = folderRecord.Id;
    Id noteId = noteRecord.Id;
    String assigneeEmail = assignment.Assignee.Email;
    Account portalAccount = userRecord.Contact.Account;
    Address addressCopy = request.Address.clone();
    return ApexPages.currentPage();
  }
  public void safeMap(Map<String, Object> values) {
    Object value = values?.get('key');
  }
  public Boolean outside(Date startDate, Date joinDate, Date endDate) {
    return joinDate < startDate ||
           joinDate > endDate;
  }
  public void addFieldError(SObject record) {
    record.addError(Account.Name, 'bad');
  }
  public List<Schema.SObjectField> fields() {
    return new List<Schema.SObjectField> {
      RecentlyViewed.LastViewedDate,
      RecentlyViewed.Type,
      RecentlyViewed.Name,
      RecentlyViewed.Id
    };
  }
  public Boolean hasValidConfigurations(String typeName) {
    switch on typeName {
      when 'A' {
        return true;
      }
      when 'B', 'C' {
        return false;
      }
    }
    return false;
  }
}
public class SalesTaxRequest {
  public Address Address { get; set; }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "CustomException.cls"), filepath.Join(root, "UsesPlatformSeams.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "PermissionSetAssignment"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		switch diag.Code {
		case "GLADESEMA006", "GLADESEMA008", "GLADESEMA011", "GLADESEMA019", "GLADESEMA021":
			t.Fatalf("platform seams should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFallbackPropertyNamesForDateAndList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesFallbackProperties.cls"), `
public class UsesFallbackProperties {
  public void run(Object info, Object response) {
    Date endDate = info.StartDate.addYears(1).addDays(-1);
    response.MembershipTermInfos.add(info);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFallbackProperties.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA023") && (strings.Contains(diag.Message, "addYears") || strings.Contains(diag.Message, "MembershipTermInfos")) {
			t.Fatalf("fallback property names should provide Date/List types: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeUnknownExternalPackageFieldDoesNotUseFallbackPropertyType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesExternalPackageResponse.cls"), `
public class UsesExternalPackageResponse {
  public void run(CBBP.TransactionServiceResponse serviceResponse) {
    if (serviceResponse.SettlementDate != null) {
      serviceResponse.SettlementDate.date();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesExternalPackageResponse.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "SettlementDate.date") {
			t.Fatalf("unknown external package fields should not use fallback Date type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCommaSeparatedLocalDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCommaLocals.cls"), `
public class UsesCommaLocals {
  public void run() {
    CartItem__c membershipItem, primaryDonationItem, secondaryDonationItem;
    membershipItem = new CartItem__c();
    primaryDonationItem = new CartItem__c();
    secondaryDonationItem = new CartItem__c();
    System.assertEquals(null, membershipItem);
    System.assertEquals(null, primaryDonationItem);
    System.assertEquals(null, secondaryDonationItem);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCommaLocals.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "CartItem__c"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" && (strings.Contains(diag.Message, "membershipItem") || strings.Contains(diag.Message, "primaryDonationItem") || strings.Contains(diag.Message, "secondaryDonationItem")) {
			t.Fatalf("comma-separated locals should be visible: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeApexClassBodyFieldAsString(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesApexClassBody.cls"), `
public class UsesApexClassBody {
  private List<ApexClass> queryClasses() {
    return new List<ApexClass>();
  }
  private void initTestData() {
    for (ApexClass testDataApexClass : queryClasses()) {
      if (classImplementsITestData(testDataApexClass.Body)) {
        System.assertEquals(true, true);
      }
    }
  }
  private Boolean classImplementsITestData(String body) {
    return body != null;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesApexClassBody.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" && strings.Contains(diag.Message, "classImplementsITestData") {
			t.Fatalf("ApexClass.Body should match String overloads: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedWrapperRecordCollectionAdd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "WrapperController.cls"), `
public class WrapperController {
  public class Setting {
    public BulkBillingSetting__c Record { get; private set; }
  }
  public List<Setting> Settings { get; private set; }
  public void save() {
    List<BulkBillingSetting__c> records = new List<BulkBillingSetting__c>();
    for (Setting wrapper : Settings) {
      validateRecord(wrapper.Record);
      records.add(wrapper.Record);
    }
  }
  private void validateRecord(BulkBillingSetting__c record) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "WrapperController.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "BulkBillingSetting__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMetadataSObjectFieldsAndStringEquals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMetadataObjects.cls"), `
public class UsesMetadataObjects {
  public void run(List<ApexPage> pages, String value) {
    Set<String> pageIds = new Set<String>();
    for (ApexPage page : pages) {
      pageIds.add(page.Id);
      if (!pageIds.contains(page.Id)) {
        pageIds.add(page.Name);
      }
    }
    Boolean same = value.equals(0);
    Schema.SObjectType pageType = ApexPage.SObjectType;
    Schema.SObjectType reportType = Report.SObjectType;
    Schema.SObjectType credentialType = NamedCredential.SObjectType;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesMetadataObjects.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStaticStandardSObjectFieldTokensInListInitializer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStandardFieldTokens.cls"), `
public class UsesStandardFieldTokens {
  public List<Schema.SObjectField> namedCredentialFields() {
    return new List<Schema.SObjectField> {
      NamedCredential.DeveloperName,
      NamedCredential.Endpoint,
      NamedCredential.MasterLabel,
      NamedCredential.NamespacePrefix
    };
  }
  public List<Schema.SObjectField> recentlyViewedFields() {
    return new List<Schema.SObjectField> {
      RecentlyViewed.LastViewedDate,
      RecentlyViewed.Type,
      RecentlyViewed.Name,
      RecentlyViewed.Id
    };
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStandardFieldTokens.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectFieldTokenDoesNotShadowClassStaticField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String EXTERNAL_TAX_STATUS_TRANSACTION_LOCKED = 'Locked';
  public String TaxTransactionStatus { get; set; }
  public void run() {
    this.TaxTransactionStatus = Order.EXTERNAL_TAX_STATUS_TRANSACTION_LOCKED;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Order.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" && strings.Contains(diag.Message, "Schema.SObjectField") {
			t.Fatalf("unexpected field token diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChildRelationshipAddAllSpecificType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChildRelationships.cls"), `
public class UsesChildRelationships {
  public void run(List<Account> accounts) {
    List<Affiliation__c> affiliations = new List<Affiliation__c>();
    List<Merchandise__c> merchandise = new List<Merchandise__c>();
    List<Application2__c> registrations = new List<Application2__c>();
    for (Account account : accounts) {
      affiliations.addAll(account.Affiliates__r);
      merchandise.addAll(account.Merchandise2__r);
      registrations.addAll(account.Registrations3__r);
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesChildRelationships.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Affiliation__c"}, {Name: "Merchandise__c"}, {Name: "Application2__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChildRelationshipAddAllToSObjectList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChildRelationships.cls"), `
public class UsesChildRelationships {
  public void run(List<Account> accounts, List<SampleParent__c> providers) {
    List<SObject> records = new List<SObject>();
    for (Account account : accounts) {
      if (account.Affiliates__r?.isEmpty() == false) {
        records.addAll(account.Affiliates__r);
      }
    }
    for (SampleParent__c provider : providers) {
      if (provider.SampleChild__r?.isEmpty() == false) {
        records.addAll(provider.SampleChild__r);
      }
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesChildRelationships.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Affiliation__c"},
		{Name: "SampleParent__c"},
		{Name: "SampleChild__c", Fields: []schema.Field{{
			Name:                  "SampleParent__c",
			Type:                  "Lookup",
			ReferenceTo:           []string{"SampleParent__c"},
			RelationshipName:      "SampleParent__r",
			ChildRelationshipName: "SampleChild__r",
		}}},
	}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMapConstructorAcceptsChildRelationshipList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChildRelationshipMapConstructor.cls"), `
public class UsesChildRelationshipMapConstructor {
  public Contact run(Address__c address, Id contactId) {
    Map<ID, Contact> contacts = new Map<ID, Contact>(address.Contacts1__r);
    return contacts.get(contactId);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "hed",
		ApexFiles: []string{filepath.Join(root, "UsesChildRelationshipMapConstructor.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "hed__Address__c"},
		{Name: "Contact", Fields: []schema.Field{{
			Name:                  "hed__Current_Address__c",
			Type:                  "Lookup",
			ReferenceTo:           []string{"hed__Address__c"},
			RelationshipName:      "hed__Current_Address__r",
			ChildRelationshipName: "hed__Contacts1__r",
		}}},
	}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStandardChildRelationshipListMemberCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStandardChildRelationships.cls"), `
public class UsesStandardChildRelationships {
  public void run(Opportunity opportunity) {
    Integer countRoles = opportunity.OpportunityContactRoles.size();
    OpportunityContactRole firstRole = opportunity.OpportunityContactRoles.get(0);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStandardChildRelationships.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCollectionAddAllIterableAndSObjectFieldLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesIterableCollections.cls"), `
public class UsesIterableCollections {
  public void run(Iterable<Account> accounts, Iterable<SObjectField> fields) {
    List<SObject> records = new List<SObject>();
    records.addAll(accounts);
    for (SObjectField field : fields) {
      String fieldName = field.getDescribe().getName();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesIterableCollections.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGenericInterfaceImplementationAssignableToInterface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MapBatch.cls"), `
public class MapBatch implements Database.Batchable<Map<String, Object>> {
  public MapBatch(Id configId, Iterable<Map<String, Object>> records) {
  }
  public Iterable<Map<String, Object>> start(Database.BatchableContext context) {
    return null;
  }
  public void execute(Database.BatchableContext context, List<Map<String, Object>> records) {
  }
  public void finish(Database.BatchableContext context) {
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesMapBatch.cls"), `
public class UsesMapBatch {
  public void run(Id configId, Iterable<Map<String, Object>> records) {
    Database.executeBatch(new MapBatch(configId, records), 200);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "MapBatch.cls"),
		filepath.Join(root, "UsesMapBatch.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" || diag.Code == "GLADESEMA011" {
			t.Fatalf("generic implemented interfaces should be assignable: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedIteratorImplementationAssignableToIteratorReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "IterableApiClient.cls"), `
public class IterableApiClient implements Iterable<RecordPage> {
  public Iterator<RecordPage> iterator() {
    return new RecordPageIterator(this);
  }
  public class RecordPage {}
  public class RecordPageIterator implements Iterator<RecordPage> {
    public RecordPageIterator(IterableApiClient client) {}
    public Boolean hasNext() { return true; }
    public RecordPage next() { return null; }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "IterableApiClient.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "Iterator<IterableApiClient.RecordPage>") {
			t.Fatalf("nested iterator implementation should satisfy Iterator return: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeInstallHandlerAssignableToTestInstall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PostInstall.cls"), `
global class PostInstall implements InstallHandler {
  global void onInstall(InstallContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "PostInstallTest.cls"), `
@IsTest
private class PostInstallTest {
  @IsTest
  static void installScript() {
    PostInstall postinstall = new PostInstall();
    Test.testInstall(postinstall, null);
    Test.testInstall(postinstall, new Version(4, 0), true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PostInstall.cls"),
		filepath.Join(root, "PostInstallTest.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "testInstall") {
			t.Fatalf("InstallHandler implementation should satisfy Test.testInstall: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeInterfaceEqualsMethodDoesNotCollideWithObjectEquals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Filter.cls"), `
public interface Filter {
  String equals(Object value);
}
`)
	writeSemaFile(t, filepath.Join(root, "FilterImpl.cls"), `
public class FilterImpl implements Filter {
  public String equals(Object value) {
    return 'ok';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Filter.cls"),
		filepath.Join(root, "FilterImpl.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA017" {
			t.Fatalf("explicit interface equals method should not collide with Object.equals: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSOQLForLoopAcceptsListChunkVariable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AggregateChunks.cls"), `
public class AggregateChunks {
  public void run() {
    for (List<AggregateResult> results : [
      SELECT COUNT(Id) logCount
      FROM Account
      GROUP BY Name
    ]) {
      Integer countRows = results.size();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "AggregateChunks.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA024" {
			t.Fatalf("SOQL for-loop list chunks should be assignable: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeRollupSummaryFieldUsesSummarizedFieldType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSummary.cls"), `
public class UsesSummary {
  public Date run(Job__c job) {
    return job.First_Shift__c.date();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSummary.cls")},
	}, schema.Schema{Objects: []schema.Object{
		{
			Name: "Job__c",
			Fields: []schema.Field{{
				Name:            "First_Shift__c",
				Type:            "Summary",
				SummarizedField: "Shift__c.Start_Date_Time__c",
			}},
		},
		{
			Name: "Shift__c",
			Fields: []schema.Field{{
				Name: "Start_Date_Time__c",
				Type: "Datetime",
			}},
		},
	}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA023") && strings.Contains(diag.Message, "date") {
			t.Fatalf("roll-up summary field should inherit summarized field type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSOQLLiteralSelectsTypedListOverload(t *testing.T) {
	t.Parallel()
	if got := semaSOQLLiteralListType("[SELECT Id FROM Account]"); got != "List<Account>" {
		t.Fatalf("SOQL literal type = %q", got)
	}
	if got := semaSOQLLiteralListType("[SELECT Id FROM User_Field_Map_Entry__mdt WHERE Map_Name__c IN (null, :mapName)]"); got != "List<User_Field_Map_Entry__mdt>" {
		t.Fatalf("custom metadata SOQL literal type = %q", got)
	}
	if got := semaSOQLLiteralListType("[SELECT Id, (SELECT Id FROM Contacts) FROM Account WHERE Id IN :ids]"); got != "List<Account>" {
		t.Fatalf("child subquery SOQL literal type = %q", got)
	}
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesOverloads.cls"), `
public class UsesOverloads {
  private static void addAll(List<Account> target, String name, List<Account> records) {}
  private static void addAll(List<Account> target, String name, List<Contact> records) {}
  public void run(List<Account> target) {
    addAll(target, 'accounts', [SELECT Id FROM Account]);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesOverloads.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Contact"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" && strings.Contains(diag.Message, "addAll") {
			t.Fatalf("SOQL literal should select overload by FROM object: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSOQLSingletonAcceptsSObjectTargets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSOQLReturn.cls"), `
public class UsesSOQLReturn {
  public class Wrapper {
    public Wrapper(Account account) {}
  }
  public Account get() {
    return [SELECT Id FROM Account LIMIT 1];
  }
  public void assign() {
    SObject record = [SELECT Id FROM Account LIMIT 1];
  }
  public void construct() {
    Wrapper wrapper = new Wrapper([SELECT Id FROM Account LIMIT 1]);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSOQLReturn.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA019" || diag.Code == "GLADESEMA018") && strings.Contains(diag.Message, "List<Account>") {
			t.Fatalf("SOQL singleton should satisfy SObject targets: %#v", result.Diagnostics)
		}
		if diag.Code == "GLADESEMA011" && strings.Contains(diag.Message, "Wrapper") {
			t.Fatalf("SOQL singleton should satisfy constructor argument: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStaticMethodNamedMatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringUtils.cls"), `
public class StringUtils {
  public static Boolean matches(String input, String regex) {
    return true;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesStringUtils.cls"), `
public class UsesStringUtils {
  public Boolean run(String value, String regex) {
    return StringUtils.matches(value, regex);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StringUtils.cls"),
		filepath.Join(root, "UsesStringUtils.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" || diag.Code == "GLADESEMA023" {
			t.Fatalf("static method named matches should resolve normally: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAccessLevelWithPermissionSetIdOnEnumValue(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesAccessLevel.cls"), `
public class UsesAccessLevel {
  public AccessLevel run(Id permissionSetId) {
    return AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesAccessLevel.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" || diag.Code == "GLADESEMA009" {
			t.Fatalf("AccessLevel enum value instance method should resolve: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeThrowTerminatesNonVoidMethod(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ThrowsOnly.cls"), `
public class ThrowsOnly {
  public Object run(Exception ex) {
    throw ex;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "ThrowsOnly.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" {
			t.Fatalf("throw should terminate non-void method: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFinalThrowAfterBlockTerminatesNonVoidMethod(t *testing.T) {
	t.Parallel()
	if !semaBodyEndsWithThrow(`
    if (blocked) {
      ex.setMessage('blocked');
    } else {
      ex.setMessage('open');
    }
    throw ex;
  `) {
		t.Fatalf("final throw after a block should terminate the method")
	}
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ThrowsAfterBlock.cls"), `
public class ThrowsAfterBlock {
  public Object run(Boolean blocked, Exception ex) {
    if (blocked) {
      ex.setMessage('blocked');
    } else {
      ex.setMessage('open');
    }
    throw ex;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "ThrowsAfterBlock.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" {
			t.Fatalf("final throw after block should terminate non-void method: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConditionalThrowDoesNotTerminateNonVoidMethod(t *testing.T) {
	t.Parallel()
	if semaBodyEndsWithThrow(`
    if (blocked) {
      throw ex;
    }
  `) {
		t.Fatalf("conditional throw body should not be treated as terminating")
	}
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ConditionalThrow.cls"), `
public class ConditionalThrow {
  public Object run(Boolean blocked, Exception ex) {
    if (blocked) {
      throw ex;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "ConditionalThrow.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "method must return Object on all paths") {
			found = true
		}
	}
	if !found {
		t.Fatalf("conditional throw should not terminate every path: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedCallOnNestedInterfaceReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Conditions.cls"), `
public class Conditions {
  public interface Condition {
    String toSOQL(Bindings bindings);
  }
  public class Bindings {
  }
  private class NullCondition implements Condition {
    public String toSOQL(Bindings bindings) {
      return '';
    }
  }
  private Condition compile() {
    return new NullCondition();
  }
  public String run(Bindings bindings) {
    return compile().toSOQL(bindings);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Conditions.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" || diag.Code == "GLADESEMA008" {
			t.Fatalf("chained call on nested interface return should resolve: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAmbiguousNullOverloadsAccepted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesNullOverloads.cls"), `
public class UsesNullOverloads {
  public class Response {
    public Response(String message) {}
    public Response(Account record) {}
  }
  public void run() {
    Response response = new Response(null);
    addTransition(null);
  }
  public void addTransition(String name) {}
  public void addTransition(Account record) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesNullOverloads.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemTypeAliasAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSystemType.cls"), `
public class UsesSystemType {
  public void run() {
    System.Type classType = Type.forName('Example');
    Boolean assignable = classType.isAssignableFrom(Type.forName('Other'));
    Object made = classType.newInstance();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSystemType.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFlowInterviewProjectTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	flowPath := filepath.Join(root, "force-app/main/default/flows/Calculate_discounts.flow-meta.xml")
	if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, flowPath, "<Flow/>")
	writeSemaFile(t, filepath.Join(root, "UsesFlow.cls"), `
public class UsesFlow {
  public void run(Map<String, Object> inputs) {
    Flow.Interview.Calculate_discounts interview = new Flow.Interview.Calculate_discounts(inputs);
    interview.start();
    Object value = interview.getVariableValue('totalDiscount');
    Type flowType = Flow.Interview.Calculate_discounts.class;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFlow.cls")},
		FlowFiles: []string{flowPath},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeShortNestedEnumValueOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CanTheUser.cls"), `
public class CanTheUser {
  public enum CrudType { CREATEABLE }
  private static Boolean crud(SObject obj, CrudType permission) {
    return true;
  }
  public static Boolean create(SObject obj) {
    return crud(obj, CrudType.CREATEABLE);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "CanTheUser.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFinalizerContextInterfaceArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesFinalizer.cls"), `
public class UsesFinalizer {
  class MockFinalizerContext implements System.FinalizerContext {
    public Id getAsyncApexJobId() { return null; }
    public String getRequestId() { return null; }
    public System.ParentJobResult getResult() { return System.ParentJobResult.SUCCESS; }
    public Exception getException() { return null; }
  }
  class Worker implements System.Finalizer {
    public void execute(FinalizerContext context) {}
  }
  public void run() {
    Worker worker = new Worker();
    worker.execute(new MockFinalizerContext());
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesFinalizer.cls")},
	}, schema.Schema{})
	model := buildSemaTypeMemberView(index)
	argType := inferSemaArgTypeWithModel("new MockFinalizerContext()", map[string]string{semaCurrentTypeScopeKey: "UsesFinalizer"}, model)
	if argType != "UsesFinalizer.MockFinalizerContext" {
		t.Fatalf("arg type = %q", argType)
	}
	if !semaAssignableToType("FinalizerContext", argType, model) {
		t.Fatalf("%s should assign to FinalizerContext", argType)
	}
	if score := semaConversionScore("FinalizerContext", argType, model); score < 0 {
		t.Fatalf("conversion score = %d", score)
	}
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeBitwiseIntegerExpressionAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Point.cls"), `
public class Point {
  final Integer hashCode;
  Point(Integer x, Integer y) {
    hashCode = x << 16 | y;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Point.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeListCapacityConstructors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesListCapacity.cls"), `
public class UsesListCapacity {
  class Row {}
  public void run(Integer size1, Integer size2) {
    List<List<Row>> rows = new List<List<Row>>(size1);
    rows[0] = new List<Row>(size2);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesListCapacity.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGeneratedStubMethodStaticAccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesGeneratedStubStaticAccess.cls"), `
public class UsesGeneratedStubStaticAccess {
  public void run() {
    Boolean invalidStatic = Database.SaveResult.isSuccess();
    Type classType = Type.forName('Example');
    Type invalidInstance = classType.forName('Other');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesGeneratedStubStaticAccess.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	var staticAccessDiagnostics int
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" {
			staticAccessDiagnostics++
		}
	}
	if staticAccessDiagnostics != 2 {
		t.Fatalf("expected 2 generated-stub static access diagnostics, got %d: %#v", staticAccessDiagnostics, result.Diagnostics)
	}
}

func TestAnalyzeSystemDateTodayStaticCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSystemDate.cls"), `
public class UsesSystemDate {
  public Date run() {
    return System.Date.today();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSystemDate.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" {
			t.Fatalf("System.Date.today should be treated as a static platform call: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStaticFieldReceiverBeatsCaseFoldedNestedType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStaticFieldReceiver.cls"), `
public class UsesStaticFieldReceiver {
  public static Runner pair = new Runner();
  public class Pair {}
  public class Runner {
    public Object run(Object first, Object second) {
      return null;
    }
  }
  public Object check() {
    return UsesStaticFieldReceiver.pair.run(1, 2);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStaticFieldReceiver.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" {
			t.Fatalf("static field receiver should not be treated as a nested type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeObjectEqualsAndFluentPageReferenceRedirect(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesPlatformSeams.cls"), `
public class UsesPlatformSeams {
  public Boolean compare(UsesPlatformSeams other) {
    return this.equals(other);
  }
  public PageReference redirect() {
    return (new PageReference('/home')).setRedirect(true);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesPlatformSeams.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedEnumValuesStaticCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Logger.cls"), `
public class Logger {
  public enum Level { INFO, WARN }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesLogger.cls"), `
public class UsesLogger {
  public List<Logger.Level> getLoggerLevels() {
    return Logger.Level.values();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Logger.cls"), filepath.Join(root, "UsesLogger.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGeneratedNestedEnumStaticMember(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesProcessParameterType.cls"), `
public class UsesProcessParameterType {
  public Process.InputParameter makeParam() {
    return new Process.InputParameter('name', Process.PluginDescribeResult.ParameterType.STRING, true);
  }
  public Process.OutputParameter makeLowercaseParam() {
    return new Process.OutputParameter('name', Process.PluginDescribeResult.ParameterType.string);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesProcessParameterType.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeArrayStyleAndWrappedLocalDeclarations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesWrappedLocals.cls"), `
public class UsesWrappedLocals {
  public void run(Account testAccount, Integer resultCount) {
    Id[] fixedSearchResults = new Id[1];
    Id[] dynamicSearchResults = new Id[resultCount];
    fixedSearchResults[0] = testAccount.Id;
    dynamicSearchResults[0] = testAccount.Id;
    Iterable<UsesWrappedLocals.Context>
      iterable = start(null);
    iterable.iterator();
  }
  public Iterable<Context> start(Object value) {
    return new List<Context>();
  }
  public class Context {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesWrappedLocals.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsSearchQueryCompileOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSearchQuery.cls"), `
public class UsesSearchQuery {
  public List<List<SObject>> run(String formattedQuery) {
    return Search.query(formattedQuery);
  }
  public void assign(String formattedQuery) {
    List<List<SObject>> results = Search.query(formattedQuery);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSearchQuery.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Search.query diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseQueryAssignsSingleAndListSObjectContexts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatabaseQuery.cls"), `
public class UsesDatabaseQuery {
  public void run(String query, Map<String, Object> binds) {
    SObject single = Database.query(query);
    Account account = Database.query(query);
    List<SObject> records = Database.query(query);
    List<Object> objects = Database.query(query);
    List<Account> accounts = Database.query(query);
    List<AggregateResult> grouped = Database.query(query);
    SObject singleWithBinds = Database.queryWithBinds(query, binds);
    List<Object> objectsWithBinds = Database.queryWithBinds(query, binds);
    List<Account> accountsWithBinds = Database.queryWithBinds(query, binds);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDatabaseQuery.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Database.query diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseQueryIsIterableAsSObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatabaseQueryForEach.cls"), `
public class UsesDatabaseQueryForEach {
  public void run(String query) {
    for (SObject row : Database.query(query)) {
      row.getSObjectType();
    }
    for (Account account : Database.query(query)) {
      account.getSObjectType();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDatabaseQueryForEach.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Database.query enhanced-for diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseQueryResultCanBeIndexedAsSObjectList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatabaseQueryIndex.cls"), `
public class UsesDatabaseQueryIndex {
  public Id run() {
    SObject row = Database.query(
      'SELECT Id FROM Account LIMIT 1'
    )[0];
    return (Id)row.get('Id');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesDatabaseQueryIndex.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "get") {
			t.Fatalf("unexpected Database.query index diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestDatabaseQueryResultCollectionSignature(t *testing.T) {
	sig, ok := semaCollectionMethodSignature("Database.QueryResult", "get")
	if !ok {
		t.Fatalf("Database.QueryResult.get signature not found")
	}
	if sig.returnType != "SObject" {
		t.Fatalf("Database.QueryResult.get return type = %q, want SObject", sig.returnType)
	}
	if !reflect.DeepEqual(sig.params, [][]string{{"Integer"}}) {
		t.Fatalf("Database.QueryResult.get params = %#v, want Integer", sig.params)
	}
}

func TestAnalyzeEncodingUtilAndBlobStaticMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEncodingUtil.cls"), `
public class UsesEncodingUtil {
  public void run(Blob body) {
    String encoded = EncodingUtil.base64Encode(body);
    Blob decoded = EncodingUtil.base64Decode(encoded);
    String hexed = EncodingUtil.convertToHex(decoded);
    Blob fromHex = EncodingUtil.convertFromHex(hexed);
    String escaped = EncodingUtil.urlEncode('a b', 'UTF-8');
    String plain = EncodingUtil.urlDecode(escaped, 'UTF-8');
    Blob literal = Blob.valueOf(plain);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEncodingUtil.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected EncodingUtil diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemStatusCodeAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesStatusCode.cls"), `
public class UsesStatusCode {
  public void run() {
    System.StatusCode statusCode = StatusCode.REQUIRED_FIELD_MISSING;
    StatusCode other = statusCode;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesStatusCode.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected StatusCode diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConcreteSObjectCloneReturnsConcreteType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectClone.cls"), `
public class UsesSObjectClone {
  public void run() {
    Contact oldRec = new Contact(LastName = 'Old');
    Contact newRec = oldRec.clone(true);
    newRec.LastName = 'New';
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSObjectClone.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Contact"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected SObject clone diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAssertClassMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesAssert.cls"), `
public class UsesAssert {
  public void run(Object value) {
    Assert.isFalse(false, 'message');
    Assert.isTrue(true);
    Assert.areEqual(value, value, 'same');
    Assert.areNotEqual(value, null);
    Assert.isNull(null, 'null');
    Assert.isNotNull(value);
    System.Assert.isInstanceOfType(value, Object.class, 'type');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesAssert.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected Assert diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAssertBooleanThroughNestedMapListFieldChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesNestedBooleanAssert.cls"), `
public class UsesNestedBooleanAssert {
  public class Root {
    public Map<String, Bucket> buckets;
  }
  public class Bucket {
    public List<Item> items;
  }
  public class Item {
    public Boolean enabled;
  }
  public void run(Root result) {
    Assert.isFalse(result.buckets.get('key').items[0].enabled, 'enabled');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesNestedBooleanAssert.cls")},
	}, schema.Schema{})
	model := buildSemaTypeMemberView(enrichIndexWithStandardSymbols(index))
	scope := map[string]string{
		normalizeName("result"): "UsesNestedBooleanAssert.Root",
		semaCurrentTypeScopeKey: "UsesNestedBooleanAssert",
	}
	for _, tc := range []struct {
		expr string
		want string
	}{
		{"result.buckets", "Map<String,UsesNestedBooleanAssert.Bucket>"},
		{"result.buckets.get('key')", "UsesNestedBooleanAssert.Bucket"},
		{"result.buckets.get('key').items", "List<UsesNestedBooleanAssert.Item>"},
		{"result.buckets.get('key').items[0]", "UsesNestedBooleanAssert.Item"},
		{"result.buckets.get('key').items[0].enabled", "Boolean"},
	} {
		if got := inferSemaArgTypeWithModel(tc.expr, scope, model); !strings.EqualFold(got, tc.want) {
			t.Fatalf("%s type = %q, want %q", tc.expr, got, tc.want)
		}
	}
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected nested Boolean Assert diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAssertBooleanThroughSingletonMethodResultChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSingletonMethodResultAssert.cls"), `
public class UsesSingletonMethodResultAssert {
  public class Root {
    public Map<String, Bucket> buckets;
  }
  public class Bucket {
    public List<Item> items;
  }
  public class Item {
    public Boolean enabled;
  }
  public class Cache {
    public static Cache Instance { get; private set; }
    public Root getRoot() {
      return null;
    }
  }
  public void run() {
    Root result = Cache.Instance.getRoot();
    Assert.isFalse(result.buckets.get('key').items[0].enabled, 'enabled');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSingletonMethodResultAssert.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected singleton chain Assert diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeClassLiteralTypeMethodsAreInstanceCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesClassLiteralType.cls"), `
public class UsesClassLiteralType {
  interface Runnable {}
  class Impl implements Runnable {}
  public Boolean run() {
    return Runnable.class.isAssignableFrom(Impl.class);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesClassLiteralType.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected class literal Type diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeRestContextStaticFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesRestContext.cls"), `
public class UsesRestContext {
  public void run() {
    RestRequest req = RestContext.request;
    RestResponse res = RestContext.response;
    String uri = req.requestURI;
    Blob body = req.requestBody;
    Integer status = res.statusCode;
    Blob responseBody = res.responseBody;
    route(req);
  }

  private static void route(System.RestRequest req) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesRestContext.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected RestContext diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsSameClassHelperWithSystemRestRequest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AccountsResource.cls"), `
@RestResource(urlMapping='/accounts/*')
global class AccountsResource {
  public static String list_all(System.RestRequest req) {
    return req.requestURI;
  }
  @HttpGet
  global static void doGet() {
    RestRequest req = RestContext.request;
    String uri = list_all(req);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "AccountsResource.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected same-class RestRequest diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeResolvesUnqualifiedNestedCastToCurrentOuter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Alpha.cls"), `
public class Alpha {
  public class Book {
    public String bookId;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Library.cls"), `
public class Library {
  private class Book {
    public Integer bookId;
  }
  public void run(Object obj) {
    Integer bid = ((Book) obj).bookId;
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "Alpha.cls"),
			filepath.Join(root, "Library.cls"),
		},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected current-outer nested cast diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCustomSObjectSetOptions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectOptions.cls"), `
public class UsesSObjectOptions {
  public void run() {
    Database.DMLOptions opts = new Database.DMLOptions();
    Thing__c record = new Thing__c();
    record.setOptions(opts);
    Database.DMLOptions current = record.getOptions();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSObjectOptions.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Thing__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected SObject options diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeAllowsSearchFindSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSearchFind.cls"), `
public class UsesSearchFind {
  public void run(String queryText) {
    search.FIND(queryText);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesSearchFind.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUserRecordAccessAndAddressComparison(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MergeLike.cls"), `
public class MergeLike {
  public class Address {
    public String Street { get; private set; }
  }
  public Boolean run(Object other, UserRecordAccess access) {
    Address otherAddress = (Address)other;
    return otherAddress != null && access.HasReadAccess;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "MergeLike.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerOperationEnumValueArgs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesTriggerOperation.cls"), `
public class UsesTriggerOperation {
  private static void createTriggeringContext(TriggerOperation operation, List<Account> newRecords, List<Account> oldRecords) {}
  public static void run() {
    List<Account> newRecords = new List<Account>();
    List<Account> oldRecords = new List<Account>();
    createTriggeringContext(TriggerOperation.AFTER_UPDATE, newRecords, oldRecords);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesTriggerOperation.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestCollectionSignaturesStripTypeModifiers(t *testing.T) {
	sig, ok := semaCollectionMethodSignature("global List<MembershipTermInfo>", "add")
	if !ok {
		t.Fatal("global List<T> should be recognized as a collection")
	}
	if len(sig.params) == 0 || len(sig.params[0]) != 1 || sig.params[0][0] != "MembershipTermInfo" {
		t.Fatalf("unexpected add signature: %#v", sig)
	}
}

func TestAnalyzeNestedCollectionAddAfterForEachShadow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MembershipTermInfo.cls"), `
global class MembershipTermInfo {
  public Id MembershipLinkId;
}
`)
	writeSemaFile(t, filepath.Join(root, "MembershipTermResponse.cls"), `
global class MembershipTermResponse {
  global List<MembershipTermInfo> MembershipTermInfos { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "MembershipTermRequestInfo.cls"), `
global class MembershipTermRequestInfo {
  public Id MembershipLinkId;
}
`)
	writeSemaFile(t, filepath.Join(root, "MembershipTermRequest.cls"), `
global class MembershipTermRequest {
  public List<MembershipTermRequestInfo> MembershipTermRequestInfos;
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesNestedCollectionAdd.cls"), `
public class UsesNestedCollectionAdd {
  private class CustomCalculator {
    public MembershipTermResponse Calculate(MembershipTermRequest request) {
      MembershipTermResponse response = new MembershipTermResponse();
      response.MembershipTermInfos = new List<MembershipTermInfo>();
      Set<Id> membershipLinkIds = new Set<Id>();
      for (MembershipTermRequestInfo info : request.MembershipTermRequestInfos) {
        membershipLinkIds.add(info.MembershipLinkId);
      }
      List<PlanTypeProductLink__c> membershipLinks = new List<PlanTypeProductLink__c>();
      for (PlanTypeProductLink__c link : membershipLinks) {
        MembershipTermInfo info = new MembershipTermInfo();
        info.MembershipLinkId = link.Id;
        response.MembershipTermInfos.add(info);
      }
      return response;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "MembershipTermInfo.cls"),
			filepath.Join(root, "MembershipTermResponse.cls"),
			filepath.Join(root, "MembershipTermRequestInfo.cls"),
			filepath.Join(root, "MembershipTermRequest.cls"),
			filepath.Join(root, "UsesNestedCollectionAdd.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "PlanTypeProductLink__c"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "add") {
			t.Fatalf("nested collection add should use the local element type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNewListHelperAllowsUntypedExpressionArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseTest.cls"), `
public class BaseTest {
  public static List<SObject> newList(SObject record) {
    return new List<SObject>{ record };
  }
  public static List<String> newList(String value) {
    return new List<String>{ value };
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesNewList.cls"), `
public class UsesNewList extends BaseTest {
  public void run(Account accountRecord) {
    Object holder = accountRecord;
    List<SObject> records = newList(holder.getSObject('Parent'));
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "BaseTest.cls"), filepath.Join(root, "UsesNewList.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" && strings.Contains(diag.Message, "newList") {
			t.Fatalf("newList helper should tolerate untyped expression arguments: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeEnumStaticMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEnum.cls"), `
public class UsesEnum {
  public enum Status { Ready, Done }
  public void run() {
    List<Status> allStatuses = Status.values();
    Status selected = Status.valueOf('Ready');
    Integer count = Status.values().size();
    for (Status status : Status.values()) {
      String name = status.name();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEnum.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, "values") || strings.Contains(diag.Message, "valueOf")) {
			t.Fatalf("enum static methods should be recognized: %#v", diag)
		}
	}
}

func TestAnalyzeSchemaSoapTypeAliases(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "UsesSoapType",
				File: "UsesSoapType.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationField, Name: "byAlias", Type: "Map<SoapType, Type>"},
					{Kind: apexast.DeclarationField, Name: "byQualifiedName", Type: "Map<Schema.SoapType, Type>"},
					{
						Kind: apexast.DeclarationMethod,
						Name: "accept",
						Parameters: []apexast.Parameter{
							{Name: "fieldType", Type: "SoapType"},
							{Name: "qualifiedFieldType", Type: "Schema.SoapType"},
						},
					},
				},
			},
		},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedTypeRelativeReferencesInsideOwner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public interface Named {
    String name();
  }
  public class Inner {
    public Inner(Integer value) {}
  }
  public class NamedImpl implements Named {
    public String name() {
      return 'named';
    }
  }
  public static Inner build(Integer value) {
    Inner made = new Inner(value);
    return made;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Outer.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedStaticPropertyNestedFieldCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public static Factory Test { get; private set; }
  public class Factory {
    public MockDatabase Database = new MockDatabase();
  }
  public class MockDatabase {
    public void onUpdate(List<SObject> records, Map<Id, SObject> oldRecords) {}
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesOuter.cls"), `
public class UsesOuter {
  public void run(Opportunity opp) {
    Outer.Test.Database.onUpdate(new List<Opportunity> { opp }, new Map<Id, Opportunity> { opp.Id => opp });
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Outer.cls"),
		filepath.Join(root, "UsesOuter.cls"),
	}}, schema.Schema{
		Objects: []schema.Object{{Name: "Opportunity"}},
	})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestCallArgumentsAtKeepsMapInitializerFatArrowInCall(t *testing.T) {
	body := "Outer.Test.Database.onUpdate(new Opportunity[] { newOpp }, new Map<Id, SObject> { newOpp.Id => oldOpp } );\nSystem.assertEquals(true, Outer.Test.Database.hasRecords());"
	calleeEnd := strings.Index(body, "(")
	if calleeEnd < 0 {
		t.Fatal("missing call")
	}

	args, ok := callArgumentsAt(body, calleeEnd)
	if !ok {
		t.Fatal("expected arguments")
	}
	if len(args) != 2 {
		t.Fatalf("args = %#v", args)
	}
	if strings.Contains(args[1].text, "assertEquals") {
		t.Fatalf("second argument consumed following call: %q", args[1].text)
	}
}

func TestCallArgumentsAtKeepsNestedFormatAndListInitializer(t *testing.T) {
	body := `keyValuePairs.add(String.format('{0}{1}{2}', new List<String> {
                key,
                PARAM_SPLITTER,
                get(key)
            }));`
	calleeEnd := strings.Index(body, "(")
	if calleeEnd < 0 {
		t.Fatal("missing call")
	}

	args, ok := callArgumentsAt(body, calleeEnd)
	if !ok {
		t.Fatal("expected arguments")
	}
	if len(args) != 1 {
		t.Fatalf("args = %#v", args)
	}
	if !strings.Contains(args[0].text, "String.format") || !strings.Contains(args[0].text, "get(key)") {
		t.Fatalf("argument = %q", args[0].text)
	}
}

func TestAnalyzeDatetimeFormatCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesDatetime.cls"), `
public class UsesDatetime {
  public String run() {
    return System.now().format('yyyyMMdd');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesDatetime.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeImplicitMethodArgumentSelectsSpecificOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DescribeHelper.cls"), `
public class DescribeHelper {
  public static DescribeHelper getDescribe(String name) { return null; }
  public static DescribeHelper getDescribe(Schema.SObjectType token) { return null; }
  public static DescribeHelper getDescribe(SObject record) { return null; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Selector.cls"), `
public abstract class Selector {
  public abstract Schema.SObjectType getSObjectType();
  public DescribeHelper get() {
    return DescribeHelper.getDescribe(getSObjectType());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DescribeHelper.cls"),
		filepath.Join(root, "Selector.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeThisFieldAccessIgnoresShadowingParameter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ShadowedField.cls"), `
public class ShadowedField {
  private Set<String> fields;
  public String path(Schema.SObjectField token) {
    return 'Name';
  }
  public void selectFields(List<Schema.SObjectField> fields) {
    for (Schema.SObjectField token : fields) {
      this.fields.add(path(token));
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ShadowedField.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProtectedInheritedMethodAcceptsSchemaSObjectFieldToken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DomainBase.cls"), `
public virtual class DomainBase {
  protected Set<Id> getIdFieldValues(Schema.SObjectField field) {
    return new Set<Id>();
  }
  protected virtual void addError(Schema.SObjectField field, String message) {
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "AccountDomain.cls"), `
public class AccountDomain extends DomainBase {
  public Set<Id> ids() {
    return getIdFieldValues(Schema.Account.Id);
  }
  public void addNameError(String message) {
    addError(Schema.Account.Name, message);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DomainBase.cls"),
		filepath.Join(root, "AccountDomain.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "String"}}}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseBatchableSObjectAllowsConcreteExecuteScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AccountBatch.cls"), `
public class AccountBatch implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext context) {
    return Database.getQueryLocator('SELECT Id FROM Account');
  }
  public void execute(Database.BatchableContext context, List<Account> scope) {
  }
  public void finish(Database.BatchableContext context) {
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "AccountBatch.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}}}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseBatchableSObjectAllowsObjectExecuteScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseBatch.cls"), `
public abstract class BaseBatch {
  public void execute(Database.BatchableContext context, List<Object> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "QueryLocatorBatch.cls"), `
public class QueryLocatorBatch extends BaseBatch implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext context) {
    return Database.getQueryLocator('SELECT Id FROM Account');
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "DirectBatch.cls"), `
public class DirectBatch implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext context) {
    return Database.getQueryLocator('SELECT Id FROM Account');
  }
  public void execute(Database.BatchableContext context, List<Object> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "BaseBatch.cls"),
		filepath.Join(root, "QueryLocatorBatch.cls"),
		filepath.Join(root, "DirectBatch.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}}}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA017" {
			t.Fatalf("unexpected Database.Batchable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardPermissionSetAssignmentsRelationship(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PermissionSetStatus.cls"), `
public class PermissionSetStatus {
  public Boolean assigned(PermissionSet permissionSet) {
    return permissionSet.Assignments.size() > 0;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PermissionSetStatus.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" || diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected PermissionSet Assignments diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCustomMetadataLabelAsString(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MetadataLabelSorter.cls"), `
public class MetadataLabelSorter {
  public Integer compare(DynamicGridConfiguration__mdt left, DynamicGridConfiguration__mdt right) {
    return left.Label.compareTo(right.Label);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "MetadataLabelSorter.cls"),
	}}, schema.Schema{Objects: []schema.Object{{
		Name: "DynamicGridConfiguration__mdt",
		Fields: []schema.Field{
			{Name: "IsActive__c", Type: "Checkbox"},
		},
	}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected custom metadata Label diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeInheritedOverloadRemainsVisible(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BaseBuilder.cls"), `
public virtual class BaseBuilder {
  public virtual String getStringValue() {
    return '';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChildBuilder.cls"), `
public class ChildBuilder extends BaseBuilder {
  public String getStringValue(String prefix) {
    return prefix;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesChildBuilder.cls"), `
public class UsesChildBuilder {
  public String run(ChildBuilder builder) {
    return builder.getStringValue();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "BaseBuilder.cls"),
		filepath.Join(root, "ChildBuilder.cls"),
		filepath.Join(root, "UsesChildBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUppercaseBooleanLiteral(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesBoolean.cls"), `
public class UsesBoolean {
  public Boolean run() {
    return TRUE;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesBoolean.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDatabaseSavepoint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSavepoint.cls"), `
public class UsesSavepoint {
  public void run() {
    Savepoint sp = Database.setSavepoint();
    Database.rollback(sp);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSavepoint.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSystemSavepointAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSystemSavepoint.cls"), `
public class UsesSystemSavepoint {
  public void run() {
    System.Savepoint sp = Database.setSavepoint();
    Database.rollback(sp);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSystemSavepoint.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSObjectIdFieldPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSObjectPath.cls"), `
public class UsesSObjectPath {
  public SObject relatedTo;
  public Id run() {
    return this.RelatedTo.Id;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSObjectPath.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSingleEmailMessageSetWhatId(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSingleEmail.cls"), `
public class UsesSingleEmail {
  public Messaging.SingleEmailMessage email;
  public SObject relatedTo;
  public void run() {
    this.email.setWhatId(this.relatedTo.Id);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSingleEmail.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSingleEmailMessageAssignableToMessagingEmail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesMessagingEmail.cls"), `
public class UsesMessagingEmail {
  public void registerEmail(Messaging.Email email) {}
  public void run() {
    Messaging.SingleEmailMessage email = new Messaging.SingleEmailMessage();
    registerEmail(email);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesMessagingEmail.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCommonSalesSObjectTypes(t *testing.T) {
	t.Parallel()
	index := typesys.Index{Types: []typesys.TypeSymbol{{
		Kind: apexast.DeclarationClass,
		Name: "UsesSalesObjects",
		File: "UsesSalesObjects.cls",
		Members: []typesys.MemberSymbol{{
			Kind: apexast.DeclarationMethod,
			Name: "run",
			Parameters: []apexast.Parameter{
				{Name: "pbe", Type: "PricebookEntry"},
				{Name: "line", Type: "OpportunityLineItem"},
			},
		}},
	}}}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSalesSObjectAssignableToSObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSalesRelationship.cls"), `
public class UsesSalesRelationship {
  public void registerRelationship(SObject record, Schema.SObjectField field, SObject parent) {}
  public void run(OpportunityLineItem line, PricebookEntry entry) {
    registerRelationship(line, OpportunityLineItem.PricebookEntryId, entry);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesSalesRelationship.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedCallArgumentSelectsSObjectFieldOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DescribeWrapper.cls"), `
public class DescribeWrapper {
  public static DescribeWrapper getDescribe(SObjectType objType) {
    return null;
  }
  public Schema.SObjectField getField(String name) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Security.cls"), `
public class Security {
  public static void checkFieldIsInsertable(SObjectType objType, String fieldName) {
    checkFieldIsInsertable(objType, DescribeWrapper.getDescribe(objType).getField(fieldName));
  }
  public static void checkFieldIsInsertable(SObjectType objType, SObjectField fieldToken) {}
  public static void checkFieldIsInsertable(SObjectType objType, DescribeFieldResult fieldDescribe) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "DescribeWrapper.cls"),
		filepath.Join(root, "Security.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringValueOfSelectsStringOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringValueBuilder.cls"), `
public class StringValueBuilder {
  public void add(List<String> values) {}
  public void add(String value) {}
  public void run(Schema.SObjectField fieldToken) {
    add(String.valueOf(fieldToken));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StringValueBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeStringFormatAcceptsObjectListArguments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringFormatBuilder.cls"), `
public class StringFormatBuilder {
  public String withStringList(String key, String value) {
    return String.format('{0}{1}', new List<String>{ key, value });
  }
  public String withObjectList(String key, Integer count) {
    List<Object> args = new List<Object>{ key, count };
    return String.format('{0}{1}', args);
  }
  public String withMixedObjectLiteral(String key, Integer count) {
    return String.format('{0}{1}', new List<Object>{ key, count });
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StringFormatBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeFieldSetMemberPathSelectsStringOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FieldSetBuilder.cls"), `
public class FieldSetBuilder {
  public void add(List<String> values) {}
  public void add(String value) {}
  public void run(Schema.FieldSetMember member) {
    add(member.getFieldPath());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "FieldSetBuilder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeVisibilityBaseline(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind:      apexast.DeclarationClass,
				Name:      "Both",
				File:      "Both.cls",
				Modifiers: []string{"public", "global"},
			},
			{
				Kind: apexast.DeclarationInterface,
				Name: "IWorker",
				File: "IWorker.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "hidden", Modifiers: []string{"private"}},
				},
			},
		},
	}

	result := Analyze(index)
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, diag := range result.Diagnostics {
		if diag.Code != "GLADESEMA005" {
			t.Fatalf("diagnostic = %#v", diag)
		}
	}
}

func TestAnalyzeMethodBodyBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void work() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  Integer field;
  public void run(String input) {
    Integer count = 1;
    Helper h = new Helper();
    h.work();
    field = count;
  }
  public void callRun() {
    run('x');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodBodyDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    MissingType item;
    MissingCtor built = new MissingCtor();
    missingValue = 1;
    missingCall();
  }
}
	`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	codes := map[string]bool{}
	for _, diag := range result.Diagnostics {
		codes[diag.Code] = true
	}
	for _, code := range []string{"GLADESEMA006", "GLADESEMA013", "GLADESEMA008"} {
		if !codes[code] {
			t.Fatalf("missing %s in diagnostics: %#v", code, result.Diagnostics)
		}
	}
}

func TestAnalyzeNonConstructableTypeDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `public abstract class Base {}`)
	writeSemaFile(t, filepath.Join(root, "IThing.cls"), `public interface IThing {}`)
	writeSemaFile(t, filepath.Join(root, "Mood.cls"), `public enum Mood { Happy }`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run() {
    Base base = new Base();
    IThing thing = new IThing();
    Mood mood = new Mood();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "IThing.cls"),
		filepath.Join(root, "Mood.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA015" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("expected 3 non-constructable diagnostics, got %d: %#v", count, result.Diagnostics)
	}
}

func TestAnalyzeCommaSeparatedLocalDeclaration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run() {
    String stepNames = '', delimiter = ', ';
    return stepNames.removeEnd(delimiter);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" && strings.Contains(diag.Message, "delimiter") {
			t.Fatalf("unexpected delimiter diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCustomExceptionConstructorInheritance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MyException.cls"), `
public class MyException extends Exception {}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run(Exception cause) {
    MyException empty = new MyException();
    MyException message = new MyException('blocked');
    MyException nested = new MyException(cause);
    throw new MyException('blocked', cause);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "MyException.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCallArgumentSeesBareSObjectTypeLocal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SelectorFactory.cls"), `
public class SelectorFactory {
  public interface ISelector {
    List<SObject> selectSObjectsById(Set<Id> recordIds);
  }
  public class SelectorFactoryInner {
    public ISelector newInstance(SObjectType objectType) {
      return null;
    }
    public List<SObject> selectById(Set<Id> recordIds) {
      throw new DeveloperException('Invalid record Id\'s set');
      SObjectType domainSObjectType = new List<Id>(recordIds)[0].getSObjectType();
      throw new DeveloperException('Unable to determine SObjectType');
      return newInstance(domainSObjectType).selectSObjectsById(recordIds);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "SelectorFactory.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" && strings.Contains(diag.Message, "domainSObjectType") {
			t.Fatalf("unexpected SObjectType local diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCustomRelationshipFieldInfersReferencedObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Product.cls"), `
public class Product {
  public static Product newInstance(Product__c record) {
    return new Product();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "OrderLine.cls"), `
public class OrderLine {
  public void run(OrderItemLine__c line) {
    Product product = Product.newInstance(line.Product2__r);
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Product.cls"), filepath.Join(root, "OrderLine.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "OrderItemLine__c",
			Fields: []schema.Field{{
				Name:        "Product2__c",
				Type:        "Lookup",
				ReferenceTo: []string{"Product__c"},
			}},
		}, {Name: "Product__c"}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" && strings.Contains(diag.Message, "Product.newInstance") {
			t.Fatalf("unexpected relationship field overload diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSelfLookupRelationshipNameStaysScalar(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSelfLookup.cls"), `
public class UsesSelfLookup {
  public void run(Account account) {
    Account current = account;
    current = current.PrimaryAffiliation__r;
    Id parentId = current.PrimaryAffiliation__r.Id;
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesSelfLookup.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "Account",
			Fields: []schema.Field{{
				Name:             "PrimaryAffiliation__c",
				Type:             "Lookup",
				ReferenceTo:      []string{"Account"},
				RelationshipName: "PrimaryAffiliation__r",
			}},
		}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected self lookup relationship diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMapLiteralValueChainedCallAfterArrow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ITestData.cls"), `
public interface ITestData {
  SObject insertRecord();
}
`)
	writeSemaFile(t, filepath.Join(root, "TestContext.cls"), `
public class TestContext {
  public static TestContext Instance { get; }
  public ITestData build(Schema.SObjectType objectType) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "AffiliationTestData.cls"), `
public class AffiliationTestData {
  protected Map<Schema.SObjectField, Object> getDefaultValueMap() {
    return new Map<Schema.SObjectField, Object> {
      Account.Name => TestContext.Instance.build(Account.SObjectType).insertRecord().Id
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ITestData.cls"),
		filepath.Join(root, "TestContext.cls"),
		filepath.Join(root, "AffiliationTestData.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "insertRecord") {
			t.Fatalf("unexpected map literal chained-call diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectAddErrorAndTriggerStaticFlags(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public void run(List<Affiliation__c> affiliations) {
    for (Affiliation__c affiliation : affiliations) {
      affiliation.addError('bad');
      affiliation.IsPrimaryContact__c.addError('bad');
      if (trigger.isInsert) {
        affiliation.addError(new Exception('bad'), false);
      }
    }
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Handler.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "Affiliation__c",
			Fields: []schema.Field{{
				Name: "IsPrimaryContact__c",
				Type: "Checkbox",
			}},
		}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA008" && strings.Contains(strings.ToLower(diag.Message), "adderror")) ||
			(diag.Code == "GLADESEMA013" && strings.Contains(strings.ToLower(diag.Message), "trigger.isinsert")) {
			t.Fatalf("unexpected SObject addError/Trigger diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeFieldGetNameSelectsStringOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CollectionUtil.cls"), `
public class CollectionUtil {
  public static Map<Id, List<SObject>> groupSObjectsByIdField(List<SObject> records, String field) {
    return new Map<Id, List<SObject>>();
  }
  public static Map<Id, List<SObject>> groupSObjectsByIdField(List<SObject> records, Schema.SObjectField field) {
    return new Map<Id, List<SObject>>();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public Map<Id, List<SObject>> run(List<Affiliation__c> affiliations) {
    return CollectionUtil.groupSObjectsByIdField(affiliations, Affiliation__c.Account__c.getDescribe().getName());
  }
}
`)
	index := typesys.Build(
		project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "CollectionUtil.cls"), filepath.Join(root, "Handler.cls")}},
		schema.Schema{Objects: []schema.Object{{
			Name: "Affiliation__c",
			Fields: []schema.Field{{
				Name:        "Account__c",
				Type:        "Lookup",
				ReferenceTo: []string{"Account"},
			}},
		}, {Name: "Account"}}},
	)

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" && strings.Contains(diag.Message, "groupSObjectsByIdField") {
			t.Fatalf("unexpected describe getName overload ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectMapGetSObjectType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Context.cls"), `
public class Context {
  public Map<Id, SObject> NewRecordMap { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public void run() {
    Context context = new Context();
    System.assertEquals(Account.SObjectType, context.NewRecordMap.getSObjectType());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Context.cls"),
		filepath.Join(root, "Handler.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getSObjectType") {
			t.Fatalf("unexpected SObject map getSObjectType diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardApexClassSelectorPatterns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "InvalidOperationException.cls"), `
public class InvalidOperationException extends Exception {}
`)
	writeSemaFile(t, filepath.Join(root, "ApexClassSelector.cls"), `
public class ApexClassSelector {
  public List<SObject> selectById(Set<Id> ids) {
    throw new InvalidOperationException();
  }
  protected Schema.SObjectType getSObjectType() {
    return ApexClass.SObjectType;
  }
  private List<Schema.SObjectField> getSObjectFieldList() {
    return new List<Schema.SObjectField> {
      ApexClass.Name,
      ApexClass.NamespacePrefix
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "InvalidOperationException.cls"),
		filepath.Join(root, "ApexClassSelector.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "selectById")) ||
			(diag.Code == "GLADESEMA021" && strings.Contains(diag.Message, "ApexClass")) ||
			(diag.Code == "GLADESEMA025" && strings.Contains(diag.Message, "Schema.SObjectField")) {
			t.Fatalf("unexpected ApexClass selector diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConcreteSObjectGetByFieldName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public void run(RecordType recordType) {
    Boolean value = (Boolean)recordType.get('IsPersonType');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Handler.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "recordType.get") {
			t.Fatalf("unexpected concrete SObject get diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectGetPopulatedFieldsAsMap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Map<String, Object> run(Account account) {
    return account.getPopulatedFieldsAsMap();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getPopulatedFieldsAsMap") {
			t.Fatalf("unexpected getPopulatedFieldsAsMap diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCronJobDetailAndCronTriggerStandardObjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    List<CronJobDetail> jobs = [SELECT Name, Id FROM CronJobDetail];
    CronTrigger trigger = [SELECT CronExpression, TimesTriggered, NextFireTime FROM CronTrigger];
    return !jobs.isEmpty() && trigger.TimesTriggered > 0;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" && (strings.Contains(diag.Message, "CronJobDetail") || strings.Contains(diag.Message, "CronTrigger")) {
			t.Fatalf("unexpected cron standard object diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzePlatformEventDomainListDowncast(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Domain.cls"), `
public class Domain {
  public Domain(List<SObject> records) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "EventDomain.cls"), `
public class EventDomain extends Domain {
  public EventDomain(List<ActionEvent__e> records) {
    super(records);
  }
  public class Constructor {
    public Domain construct(List<SObject> records) {
      return new EventDomain(records);
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ActionEvent__e.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Action Event</label>
</CustomObject>
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Domain.cls"), filepath.Join(root, "EventDomain.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "ActionEvent__e", Label: "Action Event"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA003" || diag.Code == "GLADESEMA004" {
			t.Fatalf("unexpected platform-event list downcast diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardChangeEventSObjectFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesChangeEvent.cls"), `
public class UsesChangeEvent {
  public void run() {
    EventBus.ChangeEventHeader header = new EventBus.ChangeEventHeader();
    header.changeType = 'UPDATE';
    header.recordIds = new List<Id>();
    ContactPointAddressChangeEvent event = new ContactPointAddressChangeEvent(
      ChangeEventHeader = header,
      PreferenceRank = 500
    );
    Integer rank = event.PreferenceRank;
    EventBus.ChangeEventHeader readHeader = event.ChangeEventHeader;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesChangeEvent.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestSemaStandardSObjectMembersIncludeChangeEvent(t *testing.T) {
	changeEvent := semaBuildStandardSObjectMembers("ContactPointAddressChangeEvent")
	if field, ok := changeEvent.fields[normalizeName("ChangeEventHeader")]; !ok || field.Type != "EventBus.ChangeEventHeader" {
		t.Fatalf("ChangeEventHeader = %#v", field)
	}
	if field, ok := changeEvent.fields[normalizeName("PreferenceRank")]; !ok || field.Type != "Integer" {
		t.Fatalf("PreferenceRank = %#v", field)
	}
}

func TestAnalyzeSObjectCloneAndAddError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Account run(Account account) {
    Account cloned = account.clone(false, true, false, false);
    account.addError('bad');
    return cloned;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, "clone") || strings.Contains(diag.Message, "addError")) {
			t.Fatalf("unexpected SObject instance method diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConstructorPrefersEnumOverObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EventHandlerResponse.cls"), `
public class EventHandlerResponse {
  public enum Status { NO_HANDLER, SUCCESS, ERROR }
  public EventHandlerResponse(Object data) {}
  public EventHandlerResponse(Status status) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Handler.cls"), `
public class Handler {
  public EventHandlerResponse run() {
    return new EventHandlerResponse(EventHandlerResponse.Status.NO_HANDLER);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "EventHandlerResponse.cls"),
		filepath.Join(root, "Handler.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" && strings.Contains(diag.Message, "EventHandlerResponse") {
			t.Fatalf("unexpected enum/Object constructor ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeThisConstructorCallWithNestedGenericList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AttachmentService.cls"), `
public class AttachmentService {
  public class Request {}
  public AttachmentService(Request request) {
    this(new List<Request>{ request });
  }
  public AttachmentService(List<Request> requests) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "AttachmentService.cls")}}, schema.Schema{Objects: []schema.Object{{Name: "Request"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" && strings.Contains(diag.Message, "AttachmentService") {
			t.Fatalf("unexpected this constructor diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStaticFactoryOverloadWithSObjectInitializer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Schedule.cls"), `
public class Schedule {
  public static Schedule newInstance(Schedule__c record) { return new Schedule(); }
  public static Schedule newInstance(List<ScheduleLine> lines) { return new Schedule(); }
}
`)
	writeSemaFile(t, filepath.Join(root, "ScheduleLine.cls"), `public class ScheduleLine {}`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Schedule run() {
    return Schedule.newInstance(new Schedule__c(Id = 'a00000000000001'));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Schedule.cls"),
		filepath.Join(root, "ScheduleLine.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Schedule__c"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" && strings.Contains(diag.Message, "Schedule.newInstance") {
			t.Fatalf("unexpected Schedule.newInstance ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedConstructorCallReturnType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AccountEvaluator.cls"), `
public class AccountEvaluator {
  public Boolean evaluate(Map<String, Object> data) { return true; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public Boolean run(Map<String, Object> data) {
    Boolean result = new AccountEvaluator().evaluate(data);
    return result;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "AccountEvaluator.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" && strings.Contains(diag.Message, "AccountEvaluator") {
			t.Fatalf("unexpected chained constructor return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedGenericReturnShortName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BillingHelper.cls"), `
public class BillingHelper {
  public class Status {}
  public List<Status> run() {
    List<Status> statuses = new List<Status>();
    return statuses;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "BillingHelper.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "Status") {
			t.Fatalf("unexpected nested generic return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCollectionCallAfterComparison(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Map<Id, List<SObject>> itemsByCartId, List<Object> failures) {
    return failures.size() != itemsByCartId.keySet().size();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "size") {
			t.Fatalf("unexpected chained size diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedMapConstructorKeySet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Set<Id> run(List<Account> accounts) {
    return new Map<Id, Account>(accounts).keySet();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "keySet") {
			t.Fatalf("unexpected chained keySet diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSOQLMapConstructorAndChunkLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesSOQL.cls"), `
public class UsesSOQL {
  public static void run(Set<Id> accountIds) {
    Map<Id, Account> accountsById = new Map<Id, Account>([
      SELECT Name FROM Account WHERE Id IN :accountIds
    ]);
    for (List<Account> accounts : [
      SELECT Name FROM Account
    ]) {
      System.debug(accounts.size());
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesSOQL.cls")}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeGeneratedStandardSObjectShapeAndSOQL(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesGeneratedSObjectShape.cls"), `
public class UsesGeneratedSObjectShape {
  public static void run(AIInsightAction action, AsyncApexJob job) {
    Id insightId = action.AiRecordInsightId;
    Decimal confidence = action.Confidence;
    Id apexClassId = job.ApexClassId;
    Schema.SObjectField token = AIInsightAction.SObjectType.fields.AiRecordInsightId;
    List<AIInsightAction> actions = [
      SELECT AiRecordInsightId, Confidence
      FROM AIInsightAction
    ];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UsesGeneratedSObjectShape.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedStaticFactoryCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Selector.cls"), `
public class Selector {
  public static Selector newInstance() {
    return new Selector();
  }
  public List<Account> selectById(Set<Id> ids) {
    return new List<Account>();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public List<Account> run(Set<Id> ids) {
    return Selector.newInstance().selectById(ids);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Selector.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "selectById") {
			t.Fatalf("unexpected chained factory diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardSetControllerAllowsUnresolvedQueryLocatorArg(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public ApexPages.StandardSetController run() {
    return new ApexPages.StandardSetController(Manager.Instance.getQueryLocator());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" && strings.Contains(diag.Message, "StandardSetController") {
			t.Fatalf("unexpected StandardSetController diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeApexPagesHasMessagesSeverity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    return ApexPages.hasMessages(ApexPages.Severity.Error);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "hasMessages") {
			t.Fatalf("unexpected ApexPages.hasMessages diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFinalLocalVariable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    final String TEST_FAILURE = 'Test Failure';
    System.assertEquals(TEST_FAILURE, TEST_FAILURE);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" && strings.Contains(diag.Message, "TEST_FAILURE") {
			t.Fatalf("unexpected final local variable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeIRStaticPropertyChainCallArgType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UnitOfWork.cls"), `
public class UnitOfWork {
  public void registerDeleted(List<SObject> records) {}
  public void registerDeleted(SObject record) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "TestContext.cls"), `
public class TestContext {
  public static TestContext Instance;
  public SObject get(Schema.SObjectType type) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    UnitOfWork unit = new UnitOfWork();
    unit.registerDeleted(TestContext.Instance.get(Account.SObjectType));
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "UnitOfWork.cls"),
		filepath.Join(root, "TestContext.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" && strings.Contains(diag.Message, "registerDeleted") {
			t.Fatalf("unexpected static property chain overload diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectGetSObjectFieldToken(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public SObject run(SObject record, Schema.SObjectField fieldToken) {
    return record.getSObject(fieldToken);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "getSObject") {
			t.Fatalf("unexpected SObject.getSObject diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeReturnAfterCommentStartingWithReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Boolean value) {
    // Return value from the method.
    return value;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "must return Boolean") {
			t.Fatalf("unexpected return diagnostic after comment: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeListDeepClonePreserveFlags(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public List<Account> run(List<Account> accounts) {
    return accounts.deepClone(true, true, true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "deepClone") {
			t.Fatalf("unexpected List.deepClone diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCollectionCallAfterLessThan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(List<Account> records) {
    for (Integer i = 1; i < records.size(); i++) {
      records.remove(i);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "size") {
			t.Fatalf("unexpected chained size diagnostic after less-than: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSummaryFieldReturnType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private Cart__c record;
  public Decimal run() {
    return this.record.SubTotal__c;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{
			Name: "Cart__c",
			Fields: []schema.Field{{
				Name: "SubTotal__c",
				Type: "Summary",
			}},
		}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "SubTotal__c") {
			t.Fatalf("unexpected summary field return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectPutSObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(SObject record, SObject related) {
    record.putSObject('Parent__r', related);
  }
  public void runCustom(CartItemLine__c record, SObject related) {
    record.putSObject('Product__r', related);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "CartItemLine__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "putSObject") {
			t.Fatalf("unexpected SObject.putSObject diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMapInitializerValueChainedCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  protected Schema.RecordTypeInfo getRecordType(String name) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello extends Base {
  public Map<Schema.SObjectField, Object> run() {
    return new Map<Schema.SObjectField, Object> {
      Account.RecordTypeId => getRecordType('Default').getRecordTypeId()
    };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getRecordTypeId") {
			t.Fatalf("unexpected map initializer chained call diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeKnownFluentWithCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Hello run() {
    return with(Account.RecordTypeId, '012000000000000AAA')
      .with(Account.Name, 'Acme')
      .withFirstName('Ada');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(strings.ToLower(diag.Message), "with") {
			t.Fatalf("unexpected fluent with diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCallAfterNestedStaticFactoryArg(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Generator.cls"), `
public class Generator {
  public static Id generate(Schema.SObjectType typeToken) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Converter.cls"), `
public class Converter {
  public static Converter newInstance(Id idValue) {
    return new Converter();
  }
  public OrderItem__c convertRecord(SObject record) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public OrderItem__c run(CartItem__c cartItem) {
    return Converter.newInstance(
      Generator.generate(Order__c.SObjectType))
      .convertRecord(cartItem);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Generator.cls"),
		filepath.Join(root, "Converter.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{
		Objects: []schema.Object{{Name: "CartItem__c"}, {Name: "Order__c"}, {Name: "OrderItem__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "convertRecord") {
			t.Fatalf("unexpected nested static factory chained call diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeGetClassGetName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run() {
    return getClass().getName();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getName") {
			t.Fatalf("unexpected getClass().getName diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeGetClassGetNameInsideChainedArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Selector.cls"), `
public class Selector {
  public static Selector newInstance() {
    return new Selector();
  }
  public Account selectByClassName(String className) {
    return new Account();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    Account record = Selector.newInstance().selectByClassName(getClass().getName());
    return record != null;
  }
  protected Type getClass() {
    return Hello.class;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Selector.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getName") {
			t.Fatalf("unexpected nested getClass().getName diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeBooleanReturnWithCastComparison(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  private static final String ERROR_CODE = 'E';
  public Boolean run() {
    return (String)this.getValueFromField(Account.Name) != ERROR_CODE;
  }
  private Object getValueFromField(Schema.SObjectField fieldToken) {
    return null;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "String from Boolean") {
			t.Fatalf("unexpected cast comparison return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDateTimeFieldDateCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Payment__c record) {
    return record.CreatedDate.Date() == Date.today();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Payment__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "CreatedDate.Date") {
			t.Fatalf("unexpected datetime date diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAmbiguousOverloadSameReturnType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Map<Id, Decimal> run(Object records) {
    return getPrices(records);
  }
  private Map<Id, Decimal> getPrices(List<CartItem__c> records) {
    return new Map<Id, Decimal>();
  }
  private Map<Id, Decimal> getPrices(List<CartItemLine__c> records) {
    return new Map<Id, Decimal>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "CartItem__c"}, {Name: "CartItemLine__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" && strings.Contains(diag.Message, "getPrices") {
			t.Fatalf("unexpected same-return overload ambiguity: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeRejectsSetArgumentWhenOnlyListObjectOverloadExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Q.cls"), `
public class Q {
  public Q() {}
  public Q(Schema.SObjectType objectType) {}
  public static QCondition condition(String fieldName) {
    return new QCondition();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "QCondition.cls"), `
public class QCondition {
  public QCondition isIn(List<Object> values) {
    return this;
  }
  public QCondition isIn(String value) {
    return this;
  }
  public QCondition isIn(Q subquery) {
    return this;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesQ.cls"), `
public class UsesQ {
  public void run(Set<Id> productIds) {
    Q.condition('Parent__c').isIn(productIds);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Q.cls"),
		filepath.Join(root, "QCondition.cls"),
		filepath.Join(root, "UsesQ.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" && strings.Contains(diag.Message, "isIn") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected no matching overload diagnostic for Set<Id> isIn argument, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeFallbackCustomStringFieldContains(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(SpecialPrice__c specialPrice, String priceClass) {
    return specialPrice.PriceClasses__c.contains(priceClass);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "SpecialPrice__c"}},
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "PriceClasses__c.contains") {
			t.Fatalf("unexpected fallback string field contains diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeLocalDeclarationShadowsForEachVariable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CartItem.cls"), `
public class CartItem {}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void save(List<CartItem__c> records, Map<String, CartItem> itemsByIdentifier) {
    for (CartItem__c item : records) {
      String key = 'existing';
    }
    CartItem item = itemsByIdentifier.get('missing');
    if (item == null) {
      item = new CartItem();
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "CartItem.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "CartItem__c"}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" && strings.Contains(diag.Message, `variable "item"`) {
			t.Fatalf("unexpected shadowed item assignment diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAuraHandledExceptionConstructable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    throw new AuraHandledException('blocked');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" && strings.Contains(diag.Message, "AuraHandledException") {
			t.Fatalf("unexpected AuraHandledException diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeFieldDefaultValueFormula(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer run() {
    return Integer.valueOf(Account.Name.getDescribe().getDefaultValueFormula());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getDefaultValueFormula") {
			t.Fatalf("unexpected getDefaultValueFormula diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeFieldReferenceRows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    Schema.DescribeFieldResult describe = Account.Name.getDescribe();
    String formula = describe.getCalculatedFormula();
    String target = describe.getReferenceTargetField();
    Integer order = describe.getRelationshipOrder();
    Schema.SObjectField token = describe.getSObjectField();
    return describe.isCaseSensitive() || formula == target || order == 0 || token != null;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, "getCalculatedFormula") ||
			strings.Contains(diag.Message, "getReferenceTargetField") ||
			strings.Contains(diag.Message, "getRelationshipOrder") ||
			strings.Contains(diag.Message, "getSObjectField") ||
			strings.Contains(diag.Message, "isCaseSensitive")) {
			t.Fatalf("unexpected describe field row diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeBooleanRows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run() {
    Schema.DescribeFieldResult fieldDescribe = Account.Name.getDescribe();
    Schema.DescribeSObjectResult objectDescribe = Account.SObjectType.getDescribe();
    return fieldDescribe.isFilterable() ||
      fieldDescribe.isGroupable() ||
      fieldDescribe.isIdLookup() ||
      fieldDescribe.isNamePointing() ||
      fieldDescribe.isPermissionable() ||
      fieldDescribe.isRestrictedPicklist() ||
      objectDescribe.isMergeable();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, "isFilterable") ||
			strings.Contains(diag.Message, "isGroupable") ||
			strings.Contains(diag.Message, "isIdLookup") ||
			strings.Contains(diag.Message, "isNamePointing") ||
			strings.Contains(diag.Message, "isPermissionable") ||
			strings.Contains(diag.Message, "isRestrictedPicklist") ||
			strings.Contains(diag.Message, "isMergeable")) {
			t.Fatalf("unexpected describe boolean row diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDescribeSobjectTypeAlias(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Schema.SObjectType run() {
    Schema.DescribeSObjectResult describe = Account.SObjectType.getDescribe();
    return describe.getSobjectType();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getSobjectType") {
			t.Fatalf("unexpected getSobjectType diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeObjectToStringCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AddressUtil.cls"), `
public class AddressUtil {
  public static Object getAddress(Account account, String fieldName) {
    return account.get(fieldName);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Boolean run(Account account) {
    return AddressUtil.getAddress(account, 'BillingStreet').toString() != '';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "AddressUtil.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "toString") {
			t.Fatalf("unexpected toString diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeScalarValueAndStringSliceMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run(Decimal amount, Integer count, String name) {
    Integer sortOrder = amount.intValue();
    String formatted = amount.toPlainString();
    String lower = name.toLowerCase(UserInfo.getLocale());
    String upper = name.toUpperCase(UserInfo.getLocale());
    Map<Object, Object> values = new Map<Object, Object>();
    values.put('limit', count <= 0 ? 1 : count);
    return name.left(sortOrder) + formatted.leftPad(4, '0') + lower + upper;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA023" {
			t.Fatalf("unexpected unknown method diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTypedListLiteralDoesNotReportUnknownNewlitCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PaymentLine__c.cls"), `public class PaymentLine__c {}`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(PaymentLine__c line) {
    List<PaymentLine__c> lines = new List<PaymentLine__c>{ line };
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PaymentLine__c.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "newlit:List<PaymentLine__c>") {
			t.Fatalf("unexpected newlit diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDomXmlNodeGetChildElement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Dom.XmlNode run(Dom.XmlNode node) {
    return node.getChildElement('name', null);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getChildElement") {
			t.Fatalf("unexpected getChildElement diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTypeNewInstance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Object run(Type typ) {
    return typ.newInstance();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "newInstance") {
			t.Fatalf("unexpected newInstance diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeUserInfoOrganizationName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public String run() {
    return UserInfo.getOrganizationName();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "getOrganizationName") {
			t.Fatalf("unexpected UserInfo diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMessagingSendEmailAllOrNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(List<Messaging.SingleEmailMessage> messages) {
    Messaging.sendEmail(messages, false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" && strings.Contains(diag.Message, "Messaging.sendEmail") {
			t.Fatalf("unexpected Messaging.sendEmail diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSystemHashCodeAcceptsObjectArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer run(Object value) {
    return System.hashCode(value);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "hashCode") {
			t.Fatalf("unexpected System.hashCode diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConnectApiNamedCredentialInputFieldTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public static ConnectApi.NamedCredential staticNamedCredential = new ConnectApi.NamedCredential();

  public ConnectApi.NamedCredential run(ConnectApi.NamedCredentialInput input) {
    ConnectApi.NamedCredential namedCredential = new ConnectApi.NamedCredential();
    namedCredential.developerName = input.developerName;
    namedCredential.masterLabel = input.masterLabel;
    namedCredential.type = input.type;
    namedCredential.calloutUrl = input.calloutUrl;
    return namedCredential;
  }

  public Object handle(ConnectApi.NamedCredentialInput input) {
    staticNamedCredential.type = input.type;
    return staticNamedCredential;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" && strings.Contains(diag.Message, "namedCredential.") {
			t.Fatalf("ConnectApi NamedCredential DTO fields should carry concrete types: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStandardExceptionSubtypeAssignable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Logger.cls"), `
public class Logger {
  public static void log(Exception e) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    try {
      throw new AsyncException('blocked');
    } catch (AsyncException e) {
      Logger.log(e);
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Logger.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" && strings.Contains(diag.Message, "Logger.log") {
			t.Fatalf("unexpected standard exception subtype diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeListReturnAssignableToIterable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Iterable<Object> run() {
    return new List<Object>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "Iterable<Object>") {
			t.Fatalf("unexpected Iterable return diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseBatchableMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Database.Batchable<Object> batchable, Database.BatchableContext context) {
    Iterable<Object> records = batchable.start(context);
    batchable.execute(context, new List<Object>());
    batchable.finish(context);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, ".start") || strings.Contains(diag.Message, ".execute") || strings.Contains(diag.Message, ".finish")) {
			t.Fatalf("unexpected Database.Batchable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseStatefulBatchCompiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StatefulBatch.cls"), `
public class StatefulBatch implements Database.Batchable<Integer>, Database.Stateful {
  public Integer counter = 0;
  public Iterable<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext context, List<Integer> scope) {
    counter++;
  }
  public void finish(Database.BatchableContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UseBatch.cls"), `
public class UseBatch {
  public void run() {
    Database.executeBatch(new StatefulBatch(), 1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StatefulBatch.cls"),
		filepath.Join(root, "UseBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "Stateful") || strings.Contains(diag.Message, "executeBatch") {
			t.Fatalf("unexpected Stateful batch diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseBatchableStartAcceptsListReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ListReturnBatch.cls"), `
public class ListReturnBatch implements Database.Batchable<Integer> {
  public List<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext context, List<Integer> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UseBatch.cls"), `
public class UseBatch {
  public void run() {
    Database.executeBatch(new ListReturnBatch(), 1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ListReturnBatch.cls"),
		filepath.Join(root, "UseBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "ListReturnBatch") || strings.Contains(diag.Message, "executeBatch") {
			t.Fatalf("unexpected List-return batch diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseBatchableStartAcceptsNamespacedSObjectReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EntityBatch.cls"), `
public class EntityBatch implements Database.Batchable<Entity__c> {
  public Iterable<Entity__c> start(Database.BatchableContext context) {
    return new List<Entity__c>();
  }
  public void execute(Database.BatchableContext context, List<Entity__c> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "PKG",
		ApexFiles: []string{filepath.Join(root, "EntityBatch.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "PKG__Entity__c", Fields: []schema.Field{{Name: "Id", Type: "Id"}}}}})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA017" {
			t.Fatalf("unexpected namespaced batch contract diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseBatchableRequiresAllMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "MissingExecuteBatch.cls"), `
public class MissingExecuteBatch implements Database.Batchable<Integer> {
  public Iterable<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1};
  }
  public void finish(Database.BatchableContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "MissingFinishBatch.cls"), `
public class MissingFinishBatch implements Database.Batchable<Integer> {
  public Iterable<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext context, List<Integer> scope) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "MissingExecuteBatch.cls"),
		filepath.Join(root, "MissingFinishBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	missingExecute := false
	missingFinish := false
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "MissingExecuteBatch") && strings.Contains(diag.Message, "execute") {
			missingExecute = true
		}
		if strings.Contains(diag.Message, "MissingFinishBatch") && strings.Contains(diag.Message, "finish") {
			missingFinish = true
		}
	}
	if !missingExecute || !missingFinish {
		t.Fatalf("batch contract diagnostics execute=%v finish=%v all=%#v", missingExecute, missingFinish, result.Diagnostics)
	}
}

func TestAnalyzeDatabaseBatchableRejectsWrongReturnTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "WrongStartBatch.cls"), `
public class WrongStartBatch implements Database.Batchable<Integer> {
  public String start(Database.BatchableContext context) {
    return 'nope';
  }
  public void execute(Database.BatchableContext context, List<Integer> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "WrongFinishBatch.cls"), `
public class WrongFinishBatch implements Database.Batchable<Integer> {
  public Iterable<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext context, List<Integer> scope) {}
  public String finish(Database.BatchableContext context) {
    return 'nope';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "WrongStartBatch.cls"),
		filepath.Join(root, "WrongFinishBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	wrongStart := false
	wrongFinish := false
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "WrongStartBatch") && strings.Contains(diag.Message, "start") {
			wrongStart = true
		}
		if strings.Contains(diag.Message, "WrongFinishBatch") && strings.Contains(diag.Message, "finish") {
			wrongFinish = true
		}
	}
	if !wrongStart || !wrongFinish {
		t.Fatalf("batch return diagnostics start=%v finish=%v all=%#v", wrongStart, wrongFinish, result.Diagnostics)
	}
}

func TestAnalyzeDatabaseBatchableGenericContractUsesItemType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "WrongScopeBatch.cls"), `
public class WrongScopeBatch implements Database.Batchable<Integer> {
  public Iterable<Integer> start(Database.BatchableContext context) {
    return new List<Integer>{1};
  }
  public void execute(Database.BatchableContext context, List<String> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "WrongStartItemBatch.cls"), `
public class WrongStartItemBatch implements Database.Batchable<Integer> {
  public Iterable<String> start(Database.BatchableContext context) {
    return new List<String>{'nope'};
  }
  public void execute(Database.BatchableContext context, List<Integer> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "WrongScopeBatch.cls"),
		filepath.Join(root, "WrongStartItemBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	wrongScope := false
	wrongStart := false
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "WrongScopeBatch") && strings.Contains(diag.Message, "execute") {
			wrongScope = true
		}
		if strings.Contains(diag.Message, "WrongStartItemBatch") && strings.Contains(diag.Message, "start") {
			wrongStart = true
		}
	}
	if !wrongScope || !wrongStart {
		t.Fatalf("batch generic diagnostics scope=%v start=%v all=%#v", wrongScope, wrongStart, result.Diagnostics)
	}
}

func TestAnalyzeDatabaseExecuteBatchRequiresBatchable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "NotBatch.cls"), `
public class NotBatch {}
`)
	writeSemaFile(t, filepath.Join(root, "UseBatch.cls"), `
public class UseBatch {
  public void run() {
    Database.executeBatch(new NotBatch(), 1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "NotBatch.cls"),
		filepath.Join(root, "UseBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "executeBatch") {
			return
		}
	}
	t.Fatalf("expected executeBatch batchable diagnostic: %#v", result.Diagnostics)
}

func TestAnalyzeDatabaseExecuteBatchRejectsStructuralBatchShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StructuralBatch.cls"), `
public class StructuralBatch {
  public Iterable<Object> start(Database.BatchableContext context) {
    return new List<Object>();
  }
  public void execute(Database.BatchableContext context, List<Object> scope) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UseBatch.cls"), `
public class UseBatch {
  public void run() {
    Database.executeBatch(new StructuralBatch(), 1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StructuralBatch.cls"),
		filepath.Join(root, "UseBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "executeBatch") {
			return
		}
	}
	t.Fatalf("expected structural executeBatch diagnostic: %#v", result.Diagnostics)
}

func TestAnalyzeSystemScheduleBatchRequiresBatchable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "NotBatch.cls"), `
public class NotBatch {}
`)
	writeSemaFile(t, filepath.Join(root, "UseBatch.cls"), `
public class UseBatch {
  public void run() {
    System.scheduleBatch(new NotBatch(), 'nightly', 1, 200);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "NotBatch.cls"),
		filepath.Join(root, "UseBatch.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "scheduleBatch") {
			return
		}
	}
	t.Fatalf("expected scheduleBatch batchable diagnostic: %#v", result.Diagnostics)
}

func TestAnalyzeQueryLocatorAndIterableMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Database.Batchable<Object> batch) {
    Database.QueryLocator locator = (Database.QueryLocator)batch.start(null);
    String query = locator.getQuery();
    Database.QueryLocatorIterator queryIterator = locator.iterator();
    queryIterator.hasNext();
    Object record = queryIterator.next();
    Iterator<SObject> sobjectIterator = locator.iterator();
    sobjectIterator.hasNext();
    SObject sobjectRecord = sobjectIterator.next();
    System.Iterator<SObject> qualifiedIterator = locator.iterator();
    qualifiedIterator.hasNext();
    Iterable<Object> iterable = batch.start(null);
    Iterator<Object> iterator = iterable.iterator();
    iterator.hasNext();
    Object value = iterator.next();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && (strings.Contains(diag.Message, "getQuery") || strings.Contains(diag.Message, "iterator") || strings.Contains(diag.Message, "hasNext") || strings.Contains(diag.Message, "next")) {
			t.Fatalf("unexpected query locator/iterable diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeIgnoresSObjectConstructorNamedArgumentsAsAssignments(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Account account = new Account(
      Name = 'Acme',
      Phone = '555'
    );
    Contact contact = new Contact(LastName = 'Smith', AccountId = account.Id);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" {
			t.Fatalf("unexpected named-argument assignment diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTernaryExpressionTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Account choose(Boolean pick, Account left, Account right) {
    Account selected = pick ? left : right;
    Object broader = pick ? left : 'fallback';
    Account nullable = pick ? left : null;
    String badLocal = pick ? left : right;
    return pick ? left : right;
  }
  public String badReturn(Boolean pick, Account account) {
    return pick ? account : null;
  }
  public void badConditionStillInfers(Integer pick, Account left, Account right) {
    String bad = pick ? left : right;
    Account okComparison = pick < 3 ? left : right;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	assignments := 0
	returns := 0
	conditions := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			assignments++
		}
		if diag.Code == "GLADESEMA019" {
			returns++
		}
		if diag.Code == "GLADESEMA020" {
			conditions++
		}
	}
	if assignments != 2 || returns != 1 || conditions != 1 {
		t.Fatalf("ternary diagnostics assignments=%d returns=%d conditions=%d diagnostics=%#v", assignments, returns, conditions, result.Diagnostics)
	}
}

func TestSemaBodyExpressionScanReusesExpressions(t *testing.T) {
	var body strings.Builder
	body.WriteString("Boolean pick = true;\n")
	body.WriteString("Account left = new Account();\n")
	body.WriteString("Account right = new Account();\n")
	for i := 0; i < 40; i++ {
		n := strconv.Itoa(i)
		body.WriteString("Account selected")
		body.WriteString(n)
		body.WriteString(" = pick ? left : right;\n")
		body.WriteString("selected")
		body.WriteString(n)
		body.WriteString(" = pick ? right : left;\n")
	}
	body.WriteString("return pick ? left : right;\n")

	scan := newSemaBodyExpressionScan(body.String())
	expressions := scan.expressions()
	if len(expressions) != 84 {
		t.Fatalf("scan expressions = %d, want 84", len(expressions))
	}
	allocs := testing.AllocsPerRun(10, func() {
		if len(scan.expressions()) != len(expressions) {
			t.Fatal("cached scan expression count changed")
		}
	})
	if allocs != 0 {
		t.Fatalf("cached scan expressions allocated %.0f times, want 0", allocs)
	}
}

func TestAnalyzeCastAndInstanceOfExpressionTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run(Object raw, Account fallback) {
    Boolean isPrimary = true;
    Account castAccount = (Account) raw;
    Boolean accountLike = raw instanceof Account;
    Account selected = raw instanceof Account ? (Account) raw : fallback;
    Boolean groupedLocal = (isPrimary) && accountLike;
    String badCast = (Account) raw;
    Integer badInstanceof = raw instanceof Account;
    String parenthesized = ('a') + 'b';
    Integer parenthesizedMinus = (1) - 2;
    Object badUnknownCast = (MissingType) raw;
    Boolean badUnknownCheck = raw instanceof MissingType;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	unknownTypes := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			count++
		}
		if diag.Code == "GLADESEMA006" {
			unknownTypes++
		}
	}
	if count != 2 || unknownTypes != 2 {
		t.Fatalf("cast diagnostics GLADESEMA018=%d GLADESEMA006=%d diagnostics=%#v", count, unknownTypes, result.Diagnostics)
	}
}

func TestCheckBodyIRReportsUnknownCastAndInstanceOfTypes(t *testing.T) {
	body := `
Object raw = null;
Object casted = (MissingCastType) raw;
Boolean checked = raw instanceof MissingInstanceType;
`
	typ := typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "Hello", File: "Hello.cls"}
	member := typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "run", Type: "void"}
	baseScope := map[string]string{semaCurrentTypeScopeKey: "Hello"}
	model := buildSemaTypeMemberView(typesys.Index{Types: []typesys.TypeSymbol{typ}})

	diagnostics := NewAnalyzer().checkBodyIR(typ, member, body, 0, body, baseScope, model, nil)
	var unknownTypes []string
	for _, diag := range diagnostics {
		if diag.Code == "GLADESEMA006" {
			unknownTypes = append(unknownTypes, diag.Message)
		}
	}
	if len(unknownTypes) != 2 {
		t.Fatalf("unknown expression type diagnostics = %d, want 2; diagnostics=%#v", len(unknownTypes), diagnostics)
	}
}

func TestAnalyzeReturnCastWithoutWhitespaceAfterReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Pluggable.cls"), `public interface Pluggable {}`)
	writeSemaFile(t, filepath.Join(root, "Fields.cls"), `public class Fields implements Pluggable {}`)
	writeSemaFile(t, filepath.Join(root, "UsesCastReturn.cls"), `
public class UsesCastReturn {
  public Pluggable run() {
    return(Pluggable)new Fields();
  }
  public void done(Boolean skip) {
    if (skip) {
      return ;
    }
    System.debug('done');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Pluggable.cls"),
		filepath.Join(root, "Fields.cls"),
		filepath.Join(root, "UsesCastReturn.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected cast return diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSimpleReturnTypeDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer ok(Integer value) {
    return value;
  }
  public Decimal widened(Integer value) {
    return value;
  }
  public Integer badString() {
    return 'bad';
  }
  public void badVoid() {
    return 1;
  }
  public String missingReturn() {
    Integer value = 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("GLADESEMA019 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeReturnUsesBlockScopedLocalType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "BlockReturn.cls"), `
public class BlockReturn {
  public static BlockReturn choose(Boolean done) {
    if (done) {
      BlockReturn value = new BlockReturn();
      return value;
    }
    String value = 'later';
    return new BlockReturn();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "BlockReturn.cls")}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeIRBodyAllPathsReturnDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer bothBranches(Boolean flag) {
    if (flag) {
      return 1;
    } else {
      return 2;
    }
  }
  public Integer switchAll(Integer value) {
    switch on value {
      when 1 { return 1; }
      when else { return 2; }
    }
  }
  public Integer tryCatchAll(Boolean flag) {
    try {
      return 1;
    } catch (Exception e) {
      return 2;
    }
  }
  public Integer tryFinallyReturn() {
    try {
      return 1;
    } finally {
      System.debug('cleanup');
    }
  }
  public Integer missingElse(Boolean flag) {
    if (flag) {
      return 1;
    }
  }
  public Integer missingSwitchElse(Integer value) {
    switch on value {
      when 1 { return 1; }
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" && strings.Contains(diag.Message, "on all paths") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("all-path return diagnostic count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeSimpleExpressionTypeDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void acceptDecimal(Decimal value) {}
  public void acceptBoolean(Boolean value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public Integer add(Integer left, Integer right) {
    return left + right;
  }
  public void run(Integer count, String name, Boolean ready) {
    Helper h = new Helper();
    Decimal total = count + 1.5;
    Boolean ok = ready && true;
    h.acceptDecimal(count + 2);
    h.acceptBoolean(count > 0);
    count = name + 'x';
    ready = count + 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["GLADESEMA018"] != 2 || counts["GLADESEMA009"] != 0 || counts["GLADESEMA019"] != 0 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeMethodCallOverloadBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void pick(Integer value) {}
  public void pick(String value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    h.pick(1);
    h.pick('one');
    h.pick(true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	var got bool
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" {
			got = true
		}
	}
	if !got {
		t.Fatalf("expected GLADESEMA009: %#v", result.Diagnostics)
	}
}

func TestAnalyzeNestedEnumOverloadDeclaredLater(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ProductFabricator.cls"), `
public class ProductFabricator {
  public enum RecordType { MERCHANDISE }
  public static List<ProductFabricator> createProducts(Integer count) {
    return createProducts(count, RecordType.MERCHANDISE);
  }
  public static List<ProductFabricator> createProducts(Integer count, RecordType recordType) {
    return new List<ProductFabricator>();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "ProductFabricator.cls")}}, schema.Schema{})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected nested enum overload to resolve: %#v", result.Diagnostics)
	}
}

func TestAnalyzeCallArgumentsIgnoreCommentedArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TestOrderItemLineManager.cls"), `
public class TestOrderItemLineManager {
  public static OrderItemLine__c insertNonDuesMembershipOLI(Id orderItemId, Id memberAcctId, Id membershipEnrollmentId) {
    return null;
  }
  public static void run(OrderItemLine__c cartMembershipOIL) {
    OrderItemLine__c nonDuesMembershipOIL =
      TestOrderItemLineManager.insertNonDuesMembershipOLI
        (cartMembershipOIL.OrderItem__c,
         //cartMembershipOIL.ShipTo__c,
         null,
         cartMembershipOIL.Subscription__c);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "TestOrderItemLineManager.cls")}}, schema.Schema{
		Objects: []schema.Object{
			{Name: "OrderItemLine__c", Fields: []schema.Field{
				{Name: "OrderItem__c", Type: "Lookup", ReferenceTo: []string{"OrderItem__c"}},
				{Name: "Subscription__c", Type: "Lookup", ReferenceTo: []string{"Subscription__c"}},
			}},
			{Name: "OrderItem__c"},
			{Name: "Subscription__c"},
		},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected commented argument to be ignored: %#v", result.Diagnostics)
	}
}

func TestAnalyzeChainedMapGetFieldIndexReturnType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "OrderConfirmation.cls"), `
public class OrderConfirmation {
  private class OrderItemCollection {
    public List<OrderItem__c> Items { get; set; }
  }
  public Map<String, OrderItemCollection> OrderItemMap { get; set; }
  public OrderItem__c get() {
    return OrderItemMap.get('Merchandise').Items[0];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "OrderConfirmation.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "OrderItem__c"}},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected chained Map.get field index to infer OrderItem__c: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMultipleLocalDeclaratorsKeepDeclaredType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "TotalAffiliatedAccounts.cls"), `
public class TotalAffiliatedAccounts {
  private static Map<Id, Affiliation__c> cache = new Map<Id, Affiliation__c>();
  private static void getAffiliatedAccountDeltas(Id affiliationId, Map<Id, Affiliation__c> oldRecords) {
    Id parentId;
    Affiliation__c newRecord = null, oldRecord = null;
    oldRecord = cache.get(affiliationId);
    if (oldRecord == null) {
      oldRecord = oldRecords.get(affiliationId);
    }
    parentId = oldRecord.ParentAccount__c;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "TotalAffiliatedAccounts.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Affiliation__c", Fields: []schema.Field{{Name: "ParentAccount__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}}},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected comma-declared local to keep Affiliation__c type: %#v", result.Diagnostics)
	}
}

func TestAnalyzeReturnEqualityDoesNotLookLikeAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Environment.cls"), `
public class Environment {
  private SObject context;
  public Boolean equals(Object obj) {
    Environment other = (Environment)obj;
    return context == other.context;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Environment.cls")}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" && strings.Contains(diag.Message, `variable "context"`) {
			t.Fatalf("unexpected equality assignment diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedCastFieldAccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Expr.cls"), `
public class Expr {
  public abstract class Base {}
  public class Variable extends Base {
    public Boolean isContext(Set<String> names) {
      return true;
    }
  }
  public class GetExpr extends Base {
    public Base objectExpr;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ContextResolver.cls"), `
public class ContextResolver {
  public Boolean run(Expr.GetExpr expr, Set<String> names) {
    if (expr.objectExpr instanceof Expr.Variable) {
      return ((Expr.Variable)expr.objectExpr).isContext(names);
    } else if (expr.objectExpr instanceof Expr.GetExpr) {
      return run((Expr.GetExpr)expr.objectExpr, names);
    }
    return false;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Expr.cls"),
		filepath.Join(root, "ContextResolver.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA021" && (strings.Contains(diag.Message, "Variable") || strings.Contains(diag.Message, "GetExpr")) {
			t.Fatalf("unexpected nested cast field diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMultipleUninitializedLocalDeclarators(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StandardRegistrationValidator.cls"), `
public class StandardRegistrationValidator {
  public void validate() {
    Application2__c existing, cancelled;
    String link, linkText;
    if (existing != null) {
      link = '/' + existing.Id;
      linkText = existing.Name;
    }
    if (cancelled != null) {
      link = '/' + cancelled.Id;
      linkText = cancelled.Name;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "StandardRegistrationValidator.cls")}}, schema.Schema{
		Objects: []schema.Object{{Name: "Application2__c", Fields: []schema.Field{
			{Name: "Name", Type: "String"},
		}}},
	})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" {
			t.Fatalf("unexpected unknown variable diagnostic for comma declarator: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeVisibleOverrideWinsOverProtectedBaseMethod(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SObjectWrapper.cls"), `
public virtual class SObjectWrapper {
  protected virtual Object getValueFromField(Schema.SObjectField field) {
    return null;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order extends SObjectWrapper {
  global override virtual Object getValueFromField(SObjectField field) {
    return super.getValueFromField(field);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Service.cls"), `
public class Service {
  private Order orderInstance;
  protected String getInvoiceNumber() {
    return (String)this.orderInstance.getValueFromField(Account.Name);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SObjectWrapper.cls"),
		filepath.Join(root, "Order.cls"),
		filepath.Join(root, "Service.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "String"}}}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA010" {
			t.Fatalf("unexpected GLADESEMA010 diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMultilineLocalDeclarationDoesNotRedeclareLaterUse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "PaymentSchedule.cls"), `
public class PaymentSchedule {
  public PaymentSchedule(Object record) {}
  public Integer getIntervalAmount() { return 1; }
  public String getIntervalUnit() { return 'Month'; }
}
`)
	writeSemaFile(t, filepath.Join(root, "PaymentScheduleLink__c.cls"), `
public class PaymentScheduleLink__c {
  public Object PaymentSchedule__r;
  public Integer ScheduleStartDayOverride__c;
}
`)
	writeSemaFile(t, filepath.Join(root, "Calculator.cls"), `
public class Calculator {
  public static void run(Integer amount, String unit, Integer overrideDay, Boolean flag) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Processor.cls"), `
public class Processor {
  private void calculateInstallment(PaymentScheduleLink__c paymentScheduleLink) {
    PaymentSchedule paymentSchedule = new PaymentSchedule(paymentScheduleLink.PaymentSchedule__r);

    Calculator.run(paymentSchedule.getIntervalAmount(),
        paymentSchedule.getIntervalUnit(),
        cartOrderData.StartDate,
        cartOrderData.EndDate,
        paymentSchedule == null ? null : paymentScheduleLink.ScheduleStartDayOverride__c,
        false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "PaymentSchedule.cls"),
		filepath.Join(root, "PaymentScheduleLink__c.cls"),
		filepath.Join(root, "Calculator.cls"),
		filepath.Join(root, "Processor.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" {
			t.Fatalf("unexpected GLADESEMA014 diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCommentedConstructorDoesNotTriggerConstructability(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SubscriptionManager.cls"), `
public abstract class SubscriptionManager {
}
`)
	writeSemaFile(t, filepath.Join(root, "TestSubscriptions.cls"), `
@isTest
public class TestSubscriptions {
  private static testMethod void SubscriptionDateValidationTest() {
    //SubscriptionManager sManager = new SubscriptionManager();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SubscriptionManager.cls"),
		filepath.Join(root, "TestSubscriptions.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA015" {
			t.Fatalf("unexpected GLADESEMA015 diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSObjectInstanceFieldsUseValueTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesInstanceFields.cls"), `
public class UsesInstanceFields {
  public void run(List<OrderItem__c> orderItems, List<Event__c> events) {
    for (OrderItem__c orderItem : orderItems) {
      Id entityId = orderItem.Entity__c;
    }
    for (Event__c event : events) {
      String label = event.Name;
    }
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "UsesInstanceFields.cls")}}, schema.Schema{
		Objects: []schema.Object{
			{Name: "OrderItem__c", Fields: []schema.Field{{Name: "Entity__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}},
			{Name: "Event__c", Fields: []schema.Field{{Name: "Name", Type: "Text"}}},
		},
	})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallNumericWideningBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void acceptInteger(Integer value) {}
  public void acceptLong(Long value) {}
  public void acceptDecimal(Decimal value) {}
  public void acceptDouble(Double value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    Integer count = 1;
    h.acceptLong(count);
    h.acceptDecimal(1);
    h.acceptDouble(1.5);
    h.acceptInteger(1.5);
    h.acceptDouble(count);
    h.acceptDecimal(true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("GLADESEMA009 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeMethodCallNumericOverloadChoosesNarrowestWidening(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public String pick(Integer value) { return 'integer'; }
  public Boolean pick(Decimal value) { return true; }
  public String widen(Integer value) { return 'integer'; }
  public String widen(Long value) { return 'long'; }
  public Boolean widen(Decimal value) { return true; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    String exact = h.pick(1);
    String widened = h.widen(1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected no errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallObjectOverloadChoosesNearestAncestor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Root.cls"), `public virtual class Root {}`)
	writeSemaFile(t, filepath.Join(root, "Parent.cls"), `public virtual class Parent extends Root {}`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `public class Child extends Parent {}`)
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Boolean pick(Object value) { return true; }
  public Boolean pick(Root value) { return true; }
  public String pick(Parent value) { return 'parent'; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    String result = h.pick(new Child());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Root.cls"),
		filepath.Join(root, "Parent.cls"),
		filepath.Join(root, "Child.cls"),
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected no errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallOverloadUsesPairwiseSpecificity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Boolean pick(Integer count, Object label) { return true; }
  public Boolean pick(Long count, String label) { return true; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    h.pick(1, 'one');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA022" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected GLADESEMA022: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodCallNullUsesMostSpecificOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public Boolean pick(Object value) { return true; }
  public String pick(String value) { return 'string'; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    String value = h.pick(null);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("expected no errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInheritedAndSuperMethodCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public interface Worker {
  void work(Integer value);
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  public void inherited(String value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base implements Worker {
  public void work(Integer value) {}
  public void run() {
    inherited('x');
    super.inherited('y');
    work(1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeInheritedSuperReturnAndFieldTypes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {
  public String label;
  public String inheritedLabel() {
    return label;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base {
  public String okThisCall() {
    return this.inheritedLabel();
  }
  public String okSuperCall() {
    return super.inheritedLabel();
  }
  public Integer badSuperReturn() {
    return super.inheritedLabel();
  }
  public void badSuperFieldAssign() {
    super.label = 1;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["GLADESEMA018"] != 1 || counts["GLADESEMA019"] != 1 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeSuppressesUnknownCallsThatMayComeFromMissingSuperclass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends MissingBase {
  public void run() {
    inherited();
    super.configure();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" {
			t.Fatalf("missing superclass inherited calls should not report unknown method: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSuppressesUnknownFieldsThatMayComeFromMissingSuperclass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends MissingBase {
  public String localField;
}
`)
	writeSemaFile(t, filepath.Join(root, "UseChild.cls"), `
public class UseChild {
  public Object run(Child child) {
    child.missingInheritedField = 'x';
    return child.otherMissingInheritedField;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Child.cls"),
		filepath.Join(root, "UseChild.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA021" {
			t.Fatalf("missing superclass inherited fields should not report unknown field: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSuppressesUnknownCallsThroughNestedTypeMissingSuperclass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base extends MissingBase {
}
`)
	writeSemaFile(t, filepath.Join(root, "Outer.cls"), `
public class Outer {
  public class Child extends Base {
  }
  public void run() {
    Child child = new Child();
    child.inheritedFromMissingBase();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Outer.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" {
			t.Fatalf("nested type with missing superclass chain should not report unknown method: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSuppressesSuperConstructorCallsToMissingSuperclass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends MissingBase {
  public Child(String name) {
    super(name);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" {
			t.Fatalf("missing superclass constructor calls should not report constructor diagnostics: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeInterfaceAndOverrideReturnInference(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public interface Worker {
  String work();
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public virtual class Base {
  public virtual Object pick() {
    return new Object();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base implements Worker {
  public String work() {
    return 'work';
  }
  public override Object pick() {
    return 'child';
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Uses.cls"), `
public class Uses {
  public void run() {
    Worker worker = new Child();
    String label = worker.work();
    Base base = new Child();
    String bad = base.pick();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
		filepath.Join(root, "Uses.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GLADESEMA018 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeOverrideAndImplementationContracts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public interface Worker {
  void work(Integer value);
}
`)
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public abstract class Base {
  public abstract String label();
  public virtual Integer score() { return 1; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Good.cls"), `
public class Good extends Base implements Worker {
  public override String label() { return 'ok'; }
  public void work(Integer value) {}
  public override Integer score() { return 2; }
}
`)
	writeSemaFile(t, filepath.Join(root, "Bad.cls"), `
public class Bad extends Base implements Worker {
  public override void missing() {}
  public abstract void ownAbstract();
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Good.cls"),
		filepath.Join(root, "Bad.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
	}
	if counts["GLADESEMA016"] != 1 || counts["GLADESEMA017"] != 3 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeSkipsInheritanceContractsForPackageArtifacts(t *testing.T) {
	t.Parallel()
	result := Analyze(typesys.Index{Types: []typesys.TypeSymbol{{
		Kind:       apexast.DeclarationClass,
		Name:       "ScheduleLinesProcessorJob",
		Namespace:  "pkg",
		Dependency: true,
		Artifact:   true,
		Interfaces: []string{"Database.Batchable", "Schedulable"},
		Modifiers:  []string{"global"},
	}}})

	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" || diag.Code == "GLADESEMA017" {
			t.Fatalf("artifact contract should not be revalidated: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeManagedPackageAccessRules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "TrailService.cls"), `
global class TrailService {
  global static String ok() {
    return TrailHelper.internalValue();
  }
  public static String hidden() {
    return 'hidden';
  }
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "TrailHelper.cls"), `
public class TrailHelper {
  public static String internalValue() {
    return 'from dependency';
  }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "CabinConsumer.cls"), `
public class CabinConsumer {
  public static String allowed() {
    return pkgx.TrailService.ok();
  }
  public static String denied() {
    return pkgx.TrailService.hidden();
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{
			filepath.Join(depRoot, "TrailService.cls"),
			filepath.Join(depRoot, "TrailHelper.cls"),
		},
	}
	index := typesys.Build(project.Project{
		Root: consumerRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Version:    "1.0",
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{filepath.Join(consumerRoot, "CabinConsumer.cls")},
	}, schema.Schema{})
	for i := range index.Types {
		if index.Types[i].Dependency {
			index.Types[i].Artifact = true
		}
	}

	result := Analyze(index)
	counts := map[string]int{}
	for _, diag := range result.Diagnostics {
		counts[diag.Code]++
		if diag.Code == "GLADESEMA008" {
			t.Fatalf("dependency member access should not be reported as unknown method: %#v", result.Diagnostics)
		}
	}
	if counts["dependency_member_access_denied"] != 1 {
		t.Fatalf("diagnostic counts = %#v diagnostics=%#v", counts, result.Diagnostics)
	}
}

func TestAnalyzeManagedPackageInstancePropertyKeepsNamespace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "GatewayService.cls"), `
global class GatewayService {
  global Result save(Request request) {
    return new Result();
  }
  global static GatewayService Instance {
    get {
      return new GatewayService();
    }
    private set;
  }
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "Request.cls"), `
global class Request {}
`)
	writeSemaFile(t, filepath.Join(depRoot, "Result.cls"), `
global class Result {}
`)
	writeSemaFile(t, filepath.Join(depRoot, "Schedule.cls"), `
global class Schedule {
  global static Schedule newInstance(Schedule__c record) {
    return new Schedule();
  }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "Consumer.cls"), `
public class Consumer {
  public pkgx.Result run() {
    return pkgx.GatewayService.Instance.save(new pkgx.Request());
  }
  public pkgx.Schedule wrap(pkgx__Schedule__c record) {
    return pkgx.Schedule.newInstance(record);
  }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "GatewayService.cls"), `
public class GatewayService {
  public String save(String input) {
    return input;
  }
  public static String localOnly() {
    return 'local';
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{
			filepath.Join(depRoot, "GatewayService.cls"),
			filepath.Join(depRoot, "Request.cls"),
			filepath.Join(depRoot, "Result.cls"),
			filepath.Join(depRoot, "Schedule.cls"),
		},
	}
	index := typesys.Build(project.Project{
		Root: consumerRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Version:    "1.0",
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{
			filepath.Join(consumerRoot, "Consumer.cls"),
			filepath.Join(consumerRoot, "GatewayService.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "pkgx__Schedule__c"}}})

	model := buildSemaTypeMemberView(index)
	instance, ok := semaResolveField(model, "pkgx.GatewayService", "Instance", make(map[string]bool))
	if !ok {
		t.Fatalf("dependency Instance property was not indexed")
	}
	if got := resolveNestedTypeReference(model, instance.owner, instance.member.Type); got != "pkgx.GatewayService" {
		t.Fatalf("dependency Instance type = %q, want pkgx.GatewayService", got)
	}
	var scheduleParam string
	for _, candidate := range resolveMemberMethods(model, "pkgx.Schedule", "newInstance") {
		if len(candidate.member.Parameters) == 1 {
			scheduleParam = candidate.member.Parameters[0].Type
			break
		}
	}
	if scheduleParam != "pkgx__Schedule__c" {
		t.Fatalf("dependency SObject parameter type = %q, want pkgx__Schedule__c", scheduleParam)
	}

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if (diag.Code == "GLADESEMA008" || diag.Code == "GLADESEMA009") && strings.Contains(diag.Message, "GatewayService.Instance.save") {
			t.Fatalf("dependency instance property lost namespace: %#v", result.Diagnostics)
		}
		if diag.Code == "GLADESEMA023" && strings.Contains(diag.Message, "Schedule.newInstance") {
			t.Fatalf("dependency SObject overload reported as collection call: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSameNamespaceSourceDependencyInheritance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "AgreementSetter.cls"), `
global abstract class AgreementSetter {
  global abstract OrderLineAgreement setAgreementFieldsFromCartLine(OrderLine line, OrderLineAgreement agreement);
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "OrderLine.cls"), `
global class OrderLine {}
`)
	writeSemaFile(t, filepath.Join(depRoot, "OrderLineAgreement.cls"), `
global abstract class OrderLineAgreement {
  global abstract OrderLineAgreement setStartDate(Date startDate);
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "OrderLineAgreementImpl.cls"), `
global virtual class OrderLineAgreementImpl extends OrderLineAgreement {
  global virtual override OrderLineAgreement setStartDate(Date startDate) {
    return this;
  }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "ProgramAgreement.cls"), `
public class ProgramAgreement extends pkgx.OrderLineAgreementImpl {
  public override pkgx.OrderLineAgreement setStartDate(Date startDate) {
    return this;
  }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "ProgramAgreementSetter.cls"), `
global class ProgramAgreementSetter extends pkgx.AgreementSetter {
  global ProgramAgreementSetter() {
    super();
  }
  global override pkgx.OrderLineAgreement setAgreementFieldsFromCartLine(pkgx.OrderLine line, pkgx.OrderLineAgreement agreement) {
    return agreement;
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{
			filepath.Join(depRoot, "AgreementSetter.cls"),
			filepath.Join(depRoot, "OrderLine.cls"),
			filepath.Join(depRoot, "OrderLineAgreement.cls"),
			filepath.Join(depRoot, "OrderLineAgreementImpl.cls"),
		},
	}
	index := typesys.Build(project.Project{
		Root:      consumerRoot,
		Namespace: "pkgx",
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{
			filepath.Join(consumerRoot, "ProgramAgreement.cls"),
			filepath.Join(consumerRoot, "ProgramAgreementSetter.cls"),
		},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" || diag.Code == "GLADESEMA016" {
			t.Fatalf("same-namespace source dependency inheritance should resolve: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSameNamespaceSourceDependencyShortName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "Q.cls"), `
global class Q {}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "UsesQ.cls"), `
public class UsesQ {
  public Q query;
  public void run() {
    Q localQuery = new Q();
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{filepath.Join(depRoot, "Q.cls")},
	}
	index := typesys.Build(project.Project{
		Root:      consumerRoot,
		Namespace: "pkgx",
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{filepath.Join(consumerRoot, "UsesQ.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA002", "Q")
	assertNoDiagnosticContaining(t, result, "GLADESEMA006", "Q")
}

func TestAnalyzeSameNamespaceSourceDependencyProtectedFieldAccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "RestRoute.cls"), `
global abstract class RestRoute {
  protected String resourceId;
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "ProductRoute.cls"), `
public class ProductRoute extends RestRoute {
  protected Object doGet() {
    if (String.isNotEmpty(this.resourceId)) {
      return this.resourceId;
    }
    return null;
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{filepath.Join(depRoot, "RestRoute.cls")},
	}
	index := typesys.Build(project.Project{
		Root:      consumerRoot,
		Namespace: "pkgx",
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{filepath.Join(consumerRoot, "ProductRoute.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA010", "resourceId")
}

func TestAnalyzeAcceptsLowercaseExternalManagedPackageApexNamespace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "UsesVendor.cls")
	writeSemaFile(t, classPath, `
public class UsesVendor {
  public vendor.PaymentGatewayResponse run(vendor.PaymentGatewayRequest request) {
    return null;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "localpkg",
		ApexFiles: []string{classPath},
	}, schema.Schema{})

	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA002", "vendor.PaymentGatewayResponse")
	assertNoDiagnosticContaining(t, result, "GLADESEMA004", "vendor.PaymentGatewayRequest")
}

func TestAnalyzeLeavesExternalManagedPackageSObjectFieldsOpen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "UsesExternalFields.cls")
	writeSemaFile(t, classPath, `
public class UsesExternalFields {
  public void run(vend__Membership__c membership) {
    membership.vend__EndDate__c.addMonths(1);
    membership.vend__Amount__c = 10.5;
    vend__ProductFrequencyLink__c link;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "localpkg",
		ApexFiles: []string{classPath},
	}, schema.Schema{Objects: []schema.Object{{Name: "vend__Membership__c"}}})

	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA002", "vend__ProductFrequencyLink__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA004", "vend__Membership__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA006", "vend__ProductFrequencyLink__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "vend__EndDate__c.addMonths")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "vend__Amount__c")
}

func TestAnalyzeKeepsLocalManagedPackageSObjectFieldsStrict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "UsesLocalFields.cls")
	writeSemaFile(t, classPath, `
public class UsesLocalFields {
  public void run(localpkg__Membership__c membership) {
    membership.localpkg__EndDate__c.addMonths(1);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "localpkg",
		ApexFiles: []string{classPath},
	}, schema.Schema{Objects: []schema.Object{{Name: "localpkg__Membership__c"}}})

	result := Analyze(index)
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA008") {
		t.Fatalf("expected local missing field call diagnostic, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeSkipsSourceBackedDependencyDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "BadDependency.cls"), `
public class BadDependency {
  public MissingType__c record;
  public void broken(MissingParam__c param) {
    MissingLocal__c localRecord;
  }
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "BadDependencyTrigger.trigger"), `
trigger BadDependencyTrigger on MissingObject__c (before insert) {}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "Consumer.cls"), `
public class Consumer {
  public void run() {}
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{
			filepath.Join(depRoot, "BadDependency.cls"),
			filepath.Join(depRoot, "BadDependencyTrigger.trigger"),
		},
	}
	index := typesys.Build(project.Project{
		Root: consumerRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{filepath.Join(consumerRoot, "Consumer.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.File, depRoot) {
			t.Fatalf("dependency diagnostic leaked into consumer check: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeRemapsDependencySourceNamespaceReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "base-source")
	consumerRoot := filepath.Join(root, "consumer")
	for _, dir := range []string{
		filepath.Join(depRoot, "force-app/main/classes"),
		filepath.Join(depRoot, "force-app/main/objects/Billing__c/fields"),
		filepath.Join(consumerRoot, "force-app/main/classes"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSemaFile(t, filepath.Join(depRoot, "sfdx-project.json"), `{"namespace":"BasePkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeSemaFile(t, filepath.Join(depRoot, "force-app/main/objects/Billing__c/Billing__c.object-meta.xml"), `
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
  <label>Billing</label>
  <pluralLabel>Billings</pluralLabel>
</CustomObject>
`)
	writeSemaFile(t, filepath.Join(depRoot, "force-app/main/objects/Billing__c/fields/Amount__c.field-meta.xml"), `
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
  <fullName>Amount__c</fullName>
  <label>Amount</label>
  <type>Number</type>
  <precision>16</precision>
  <scale>0</scale>
</CustomField>
`)
	writeSemaFile(t, filepath.Join(depRoot, "force-app/main/classes/Helper.cls"), `
global class Helper {
  global static Integer amount(BasePkg__Billing__c row) {
    return Integer.valueOf(row.Amount__c);
  }
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "force-app/main/classes/Gateway.cls"), `
global class Gateway {
  global static Integer createAmount(Integer amount) {
    BasePkg__Billing__c row = new BasePkg__Billing__c(Amount__c = amount);
    return BasePkg.Helper.amount(row);
  }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeSemaFile(t, filepath.Join(consumerRoot, "glade.yml"), `project:
  managedPackageDependencies: ["stagepkg:../base-source"]
  namespaceRemaps: ["BasePkg:stagepkg"]
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "force-app/main/classes/Consumer.cls"), `
public class Consumer {
  public Integer run(stagepkg__Billing__c row) {
    return stagepkg.Gateway.createAmount(Integer.valueOf(row.Amount__c));
  }
}
`)

	p, err := project.Load(consumerRoot)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	result := Analyze(typesys.Build(p, s))
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" && (strings.Contains(diag.Message, "BasePkg.Helper") || strings.Contains(diag.Message, "BasePkg__Billing__c")) {
			t.Fatalf("dependency source namespace reference was not remapped: %#v", result.Diagnostics)
		}
	}
	if result.HasErrors() {
		t.Fatalf("remapped dependency source should analyze without errors: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSourceBackedDependencyDowngradesSemanticUncertainty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "Dependency.cls"), `
public class Dependency {}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "Consumer.cls"), `
public class Consumer {
  public void run() {
    MissingLocal localRecord;
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{filepath.Join(depRoot, "Dependency.cls")},
	}
	index := typesys.Build(project.Project{
		Root: consumerRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{filepath.Join(consumerRoot, "Consumer.cls")},
	}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" && diag.Severity != diagnostic.Warning {
			t.Fatalf("source dependency semantic uncertainty should be warning: %#v", result.Diagnostics)
		}
	}
	if result.HasErrors() {
		t.Fatalf("source dependency semantic uncertainty should not fail check: %#v", result.Diagnostics)
	}
}

func TestAnalyzeDoesNotRequirePlatformStubsToSatisfyInterfaces(t *testing.T) {
	t.Parallel()
	result := Analyze(typesys.Index{Types: []typesys.TypeSymbol{{
		Kind:       apexast.DeclarationClass,
		Name:       "OrderItem",
		Namespace:  "pkg",
		Dependency: true,
		Artifact:   true,
		Interfaces: []string{"Comparable"},
		Modifiers:  []string{"global"},
	}}})
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA017" && strings.Contains(diag.Message, "OrderItem") && strings.Contains(diag.Message, "compareTo") {
			t.Fatalf("platform stubs should not be revalidated as user Apex: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedSiblingOverrideSignatures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ExprNode.cls"), `
public abstract class ExprNode {
  public abstract Object evaluate(Context context);
  public abstract class BinaryExprNode extends ExprNode {
  }
  public class AddNode extends BinaryExprNode {
    public override Object evaluate(Context context) {
      return null;
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Context.cls"), `
public interface Context {
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "ExprNode.cls"),
		filepath.Join(root, "Context.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" || diag.Code == "GLADESEMA017" {
			t.Fatalf("nested sibling inheritance should satisfy override contracts: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedInterfaceResolutionPrefersEnclosingType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Filter.cls"), `
public interface Filter {
  Boolean getExcludeBotUsers();
}
`)
	writeSemaFile(t, filepath.Join(root, "SOQL.cls"), `
public class SOQL {
  public interface Filter {
    Filter isFalse();
  }
  private class SoqlFilter implements Filter {
    public Filter isFalse() {
      return this;
    }
  }
  private class MissingFilter implements Filter {
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Filter.cls"),
		filepath.Join(root, "SOQL.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	getExcludeCount := 0
	missingNestedCount := 0
	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.Message, "getExcludeBotUsers") {
			getExcludeCount++
		}
		if strings.Contains(diag.Message, "isFalse") {
			missingNestedCount++
		}
	}
	if getExcludeCount != 0 || missingNestedCount != 1 {
		t.Fatalf("nested interface diagnostics getExclude=%d missingNested=%d all=%#v", getExcludeCount, missingNestedCount, result.Diagnostics)
	}
}

func TestAnalyzePlatformOverrideSignatures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Picklist.cls"), `
global class Picklist extends VisualEditor.DynamicPickList {
  global override VisualEditor.DataRow getDefaultValue() {
    return new VisualEditor.DataRow('None', '');
  }
  global override VisualEditor.DynamicPickListRows getValues() {
    return new VisualEditor.DynamicPickListRows();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Callback.cls"), `
public class Callback extends Metadata.DeployCallbackContext {
  public override Id getCallbackJobId() {
    return '000000000000001';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Picklist.cls"),
		filepath.Join(root, "Callback.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" {
			t.Fatalf("platform base overrides should satisfy override contracts: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeConstructorChainingBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Base.cls"), `
public class Base {
  public Base(Integer value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Child.cls"), `
public class Child extends Base {
  public Child() {
    super(1);
  }
  public Child(String value) {
    this();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Base.cls"),
		filepath.Join(root, "Child.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeConstructorChainingDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Plain.cls"), `
public class Plain {
  public void run() {
    this();
  }
  public Plain() {
    super(1);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Plain.cls")}}, schema.Schema{})

	result := Analyze(index)
	var count int
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("GLADESEMA011 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeMethodCallVisibilityDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Secret.cls"), `
public class Secret {
  private void hidden() {}
  protected void guarded() {}
  public void ownAccess() {
    hidden();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChildSecret.cls"), `
public class ChildSecret extends Secret {
  public void run() {
    guarded();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "GrandChildSecret.cls"), `
public class GrandChildSecret extends ChildSecret {
  public void runAgain() {
    guarded();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Intruder.cls"), `
public class Intruder {
  public void run() {
    Secret s = new Secret();
    s.hidden();
    s.guarded();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Secret.cls"),
		filepath.Join(root, "ChildSecret.cls"),
		filepath.Join(root, "GrandChildSecret.cls"),
		filepath.Join(root, "Intruder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	var count int
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA010" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("GLADESEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeTestVisibleMethodAccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Secret.cls"), `
public class Secret {
  @TestVisible private static String token = 'ok';
  @TestVisible private static void visibleForTests() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "SecretTest.cls"), `
@IsTest
private class SecretTest {
  @IsTest static void run() {
    Secret.visibleForTests();
  }
  private class Helper {
    String read() {
      return Secret.token;
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Intruder.cls"), `
public class Intruder {
  public void run() {
    Secret.visibleForTests();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Secret.cls"),
		filepath.Join(root, "SecretTest.cls"),
		filepath.Join(root, "Intruder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA010" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GLADESEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzePropertyAccessUsesDeclaredDtoType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ProcessOrderCommand.cls"), `
public class ProcessOrderCommand {
  public CreditCardPaymentDetailsDto CreditCardPaymentDetails { get; private set; }
  public ProcessOrderCommand(CreditCardPaymentDetailsDto details) {
    this.CreditCardPaymentDetails = details;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "CreditCardPaymentDetailsDto.cls"), `
public class CreditCardPaymentDetailsDto {
  public String cardHolderName { get; set; }
}
`)
	writeSemaFile(t, filepath.Join(root, "CreditCardPaymentDetails.cls"), `
public class CreditCardPaymentDetails {
  private String cardHolderName;
}
`)
	writeSemaFile(t, filepath.Join(root, "ProcessOrderService.cls"), `
public class ProcessOrderService {
  public String run(ProcessOrderCommand command) {
    return command.CreditCardPaymentDetails.cardHolderName;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, Namespace: "demo", ApexFiles: []string{
		filepath.Join(root, "ProcessOrderCommand.cls"),
		filepath.Join(root, "CreditCardPaymentDetailsDto.cls"),
		filepath.Join(root, "CreditCardPaymentDetails.cls"),
		filepath.Join(root, "ProcessOrderService.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA010" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected field diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNestedEnumConstantFieldPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "di_Binding.cls"), `
public abstract class di_Binding {
  public enum BindingType { Apex, VisualforceComponent, LightningComponent, Flow, Module }
}
`)
	writeSemaFile(t, filepath.Join(root, "di_Module.cls"), `
public class di_Module {
  private di_Binding.BindingType bindingType;
  public void flow() {
    bindingType = di_Binding.BindingType.Flow;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, Namespace: "demo", ApexFiles: []string{
		filepath.Join(root, "di_Binding.cls"),
		filepath.Join(root, "di_Module.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" || diag.Code == "GLADESEMA021" {
			t.Fatalf("unexpected nested enum diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeLocalNestedEnumWinsOverDependencyType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(depRoot, "di_Binding.cls"), `
global abstract class di_Binding {
  global enum BindingType { Apex, Module }
}
`)
	writeSemaFile(t, filepath.Join(depRoot, "di_Module.cls"), `
global class di_Module {
  private di_Binding.BindingType bindingType;
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "di_Binding.cls"), `
public abstract class di_Binding {
  public enum BindingType { Apex, VisualforceComponent, LightningComponent, Flow, Module }
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "di_Module.cls"), `
public class di_Module {
  private di_Binding.BindingType bindingType;
  public void flow() {
    bindingType = di_Binding.BindingType.Flow;
  }
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "stagepkg",
		ApexFiles: []string{
			filepath.Join(depRoot, "di_Binding.cls"),
			filepath.Join(depRoot, "di_Module.cls"),
		},
	}
	index := typesys.Build(project.Project{
		Root:      consumerRoot,
		Namespace: "demo",
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "stagepkg",
			SourceRoot: depRoot,
			Project:    &depProject,
		}},
		ApexFiles: []string{
			filepath.Join(consumerRoot, "di_Binding.cls"),
			filepath.Join(consumerRoot, "di_Module.cls"),
		},
	}, schema.Schema{})
	sort.SliceStable(index.Types, func(i, j int) bool {
		if index.Types[i].Dependency == index.Types[j].Dependency {
			return false
		}
		return !index.Types[i].Dependency && index.Types[j].Dependency
	})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA021" || diag.Code == "GLADESEMA013" {
			t.Fatalf("unexpected dependency shadow diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeNamespacedNestedEnumConstantFieldPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "LmsEventService.cls"), `
public class LmsEventService {
  public enum RecordType { PRODUCT, PURCHASE }
  public enum OperationType { ON_INSERT, ON_UPDATE }
  public void publish(String recordId, RecordType recordType, OperationType operationType) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static final String STATE_PRO_FORMA = 'Pro forma';
}
`)
	writeSemaFile(t, filepath.Join(root, "LmsEventServiceTest.cls"), `
private class LmsEventServiceTest {
  private static void run() {
    LmsEventService service = new LmsEventService();
    service.publish('testId', demo.LmsEventService.RecordType.PURCHASE, demo.LmsEventService.OperationType.ON_INSERT);
    String state = demo.Order.STATE_PRO_FORMA;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, Namespace: "demo", ApexFiles: []string{
		filepath.Join(root, "LmsEventService.cls"),
		filepath.Join(root, "Order.cls"),
		filepath.Join(root, "LmsEventServiceTest.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA013" || diag.Code == "GLADESEMA021" || diag.Code == "GLADESEMA009" {
			t.Fatalf("unexpected namespaced nested enum diagnostic: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeTestVisibleConstructorAccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SecretCtor.cls"), `
public class SecretCtor {
  @TestVisible private SecretCtor(String value) {}
  public SecretCtor() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "SecretCtorTest.cls"), `
@IsTest
private class SecretCtorTest {
  @IsTest static void run() {
    SecretCtor value = new SecretCtor('test');
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "IntruderCtor.cls"), `
public class IntruderCtor {
  public void run() {
    SecretCtor value = new SecretCtor('test');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "SecretCtor.cls"),
		filepath.Join(root, "SecretCtorTest.cls"),
		filepath.Join(root, "IntruderCtor.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA010" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GLADESEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeAnnotationSemantics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "GoodRest.cls"), `
@RestResource(urlMapping='/good/*')
global class Good {
  @HttpGet global static void getIt() {}
  @future(callout=true) public static void later() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "GoodTest.cls"), `
@IsTest
private class GoodTest {
  @TestSetup static void seed() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "GoodInvocable.cls"), `
public class GoodInvocable {
  @InvocableMethod public static void run(List<String> names) {}
  @InvocableVariable public String name;
}
`)
	writeSemaFile(t, filepath.Join(root, "BadRest.cls"), `
@RestResource(urlMapping='/bad/*')
public interface BadRest {
}
`)
	writeSemaFile(t, filepath.Join(root, "BadAnnotations.cls"), `
public class BadAnnotations {
  @HttpPost public static void postIt() {}
  @TestSetup static void seed(String name) {}
  @future public static String later() { return 'x'; }
  @InvocableMethod public void run() {}
  @InvocableVariable public void notVariable() {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "GoodRest.cls"),
		filepath.Join(root, "GoodTest.cls"),
		filepath.Join(root, "GoodInvocable.cls"),
		filepath.Join(root, "BadRest.cls"),
		filepath.Join(root, "BadAnnotations.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA026" {
			count++
		}
	}
	if count != 6 {
		t.Fatalf("GLADESEMA026 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeStaticAndInstanceMethodAccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Worker.cls"), `
public class Worker {
  public static void stat() {}
  public void inst() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Caller.cls"), `
public class Caller {
  public void run() {
    Worker.stat();
    Worker w = new Worker();
    w.inst();
    Worker.inst();
    w.stat();
  }
  public static void runStatic() {
    helper();
  }
  public void helper() {}
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Worker.cls"),
		filepath.Join(root, "Caller.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("GLADESEMA027 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeLocalDoesNotShadowTypeInOwnInitializer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Factory.cls"), `
public class Factory {
  public static Factory newInstance() {
    return new Factory();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Caller.cls"), `
public class Caller {
  public void run() {
    Factory factory = Factory.newInstance();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Factory.cls"),
		filepath.Join(root, "Caller.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" && strings.Contains(diag.Message, "Factory.newInstance") {
			t.Fatalf("local should not shadow its type inside its own initializer: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDuplicateNamedClassUsesOwnMembersForBody(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := filepath.Join(root, "one", "ReviewDataProviderTest.cls")
	second := filepath.Join(root, "two", "ReviewDataProviderTest.cls")
	if err := os.MkdirAll(filepath.Dir(first), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(second), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, first, `
@IsTest
private class ReviewDataProviderTest {
  @IsTest
  static void firstOnly() {
	firstHelper();
    System.assert(true);
  }

	private static void firstHelper() {
	}
}
`)
	writeSemaFile(t, second, `
@IsTest
private class ReviewDataProviderTest {
  @IsTest
  static void callsLocalHelper() {
    stubSetupMapping();
    String value = getFixture();
    System.assertEquals('ok', value);
  }

  private static void stubSetupMapping() {
  }

  private static String getFixture() {
    return 'ok';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{first, second}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code != "GLADESEMA008" {
			continue
		}
		if strings.Contains(diag.Message, "firstHelper") && diag.File == first {
			t.Fatalf("first duplicate body should use first file members: %#v", result.Diagnostics)
		}
		if (strings.Contains(diag.Message, "stubSetupMapping") || strings.Contains(diag.Message, "getFixture")) && diag.File == second {
			t.Fatalf("second duplicate body should use second file members: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeBuildsTypeMemberModelOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "Once.cls")
	writeSemaFile(t, file, `
public class Once {
  public void run() {
    System.debug('once');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{})
	counters := PerfCounters{Enabled: true}
	AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:  true,
		ExportTypes:  true,
		PerfCounters: &counters,
	})
	if got := counters.TypeMemberModel.Calls; got != 1 {
		t.Fatalf("type-member model calls = %d, want 1", got)
	}
}

func TestAnalyzeWithArtifactsSurvivesSourceRemovalWithoutReread(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Arena.cls")
	writeSemaFile(t, file, "public class Arena {\r\n  enum Café { 東京, Osaka }\r\n  public void run() {\r\n    for (Account item : [SELECT Missing__c FROM Account]) { insert item; }\r\n    System.debug(Café.東京);\r\n  }\r\n}\r\n")
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	opts := AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: &artifacts}
	want := AnalyzeWithOptions(index, opts)
	legacy := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
	if !reflect.DeepEqual(want, legacy) {
		t.Fatalf("artifact analysis differs from legacy source reads:\nartifact=%#v\nlegacy=%#v", want, legacy)
	}
	if len(want.Diagnostics) == 0 {
		t.Fatal("fixture produced no source-backed diagnostics")
	}
	if err := os.Rename(file, file+".removed"); err != nil {
		t.Fatal(err)
	}
	got := AnalyzeWithOptions(index, opts)
	again := AnalyzeWithOptions(index, opts)
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(again, want) {
		t.Fatalf("artifact analysis changed after source removal:\nwant=%#v\ngot=%#v\nagain=%#v", want, got, again)
	}
	counters := PerfCounters{Enabled: true}
	_ = AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts, PerfCounters: &counters})
	if counters.SourceArenaFallbackReads != 0 || counters.SourceArenaMisses != 0 || counters.SourceArenaHits == 0 {
		t.Fatalf("source counters = %#v", counters)
	}
	stats := artifacts.Sources.Stats()
	if counters.WorkspacePhysicalReads != stats.PhysicalReadAttempts || counters.WorkspacePhysicalSources != stats.PhysicalSources || counters.WorkspaceLogicalViews != stats.LogicalViews || counters.WorkspaceOccurrences != stats.Occurrences {
		t.Fatalf("workspace counters = %#v, stats = %#v", counters, stats)
	}
}

func TestAnalyzeWithArtifactsMissingSourceDoesNotFallBackToDisk(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "MissingArena.cls")
	writeSemaFile(t, file, "public class MissingArena { public void run() { for (Account item : [SELECT Id FROM Account]) { insert item; } } }")
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	empty := typesys.BuildArtifacts{Sources: typesys.NewWorkspaceSources()}
	counters := PerfCounters{Enabled: true}
	result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &empty, PerfCounters: &counters})
	if hasDiagnosticCode(result.Diagnostics, "GLADEPERF001") {
		t.Fatalf("missing artifact fell back to disk: %#v", result.Diagnostics)
	}
	if counters.SourceArenaFallbackReads != 0 || counters.SourceArenaMisses == 0 {
		t.Fatalf("source counters = %#v", counters)
	}
}

func TestSemaSourcesCachesFallbackMissForEntireAnalysis(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "AppearsLater.cls")
	typ := typesys.TypeSymbol{File: file}
	counters := PerfCounters{Enabled: true}
	recorder := newPerfRecorder(&counters)
	sources := newSemaSources(nil, &recorder)

	if _, ok := sources.normalizedForType(typ); ok {
		t.Fatal("missing source unexpectedly resolved")
	}
	writeSemaFile(t, file, "public class AppearsLater {}")
	if _, ok := sources.normalizedForType(typ); ok {
		t.Fatal("source appearing mid-analysis changed the resolved miss")
	}
	recorder.finish()
	if counters.SourceArenaFallbackReads != 1 || counters.SourceArenaMisses != 2 {
		t.Fatalf("source counters = %#v, want one fallback read and two stable misses", counters)
	}
}

func TestAnalyzeSharedTypeMemberModelPreservesDiagnosticOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bodyFile := filepath.Join(root, "BodyFailure.cls")
	inheritanceFile := filepath.Join(root, "InheritanceFailure.cls")
	writeSemaFile(t, bodyFile, `
public class BodyFailure {
  public void run() {
    missingHelper();
  }
}
`)
	writeSemaFile(t, inheritanceFile, `
public class InheritanceFailure {
  public override void invalidOverride() {
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{bodyFile, inheritanceFile}}, schema.Schema{})
	result := Analyze(index)
	type diagnosticIdentity struct {
		severity diagnostic.Severity
		code     string
		file     string
		range_   diagnostic.Range
		message  string
	}
	var got []diagnosticIdentity
	for _, diag := range result.Diagnostics {
		if diag.Code != "GLADESEMA008" && diag.Code != "GLADESEMA016" {
			continue
		}
		identity := diagnosticIdentity{severity: diag.Severity, code: diag.Code, file: diag.File, message: diag.Message}
		if diag.Range != nil {
			identity.range_ = *diag.Range
		}
		got = append(got, identity)
	}
	want := []diagnosticIdentity{
		{
			severity: diagnostic.Error,
			code:     "GLADESEMA008",
			file:     bodyFile,
			range_: diagnostic.Range{
				Start: diagnostic.Position{Line: 4, Column: 5, Offset: 54},
				End:   diagnostic.Position{Line: 4, Column: 18, Offset: 67},
			},
			message: "method \"run\" calls unknown method \"missingHelper\"",
		},
		{
			severity: diagnostic.Error,
			code:     "GLADESEMA016",
			file:     inheritanceFile,
			range_: diagnostic.Range{
				Start: diagnostic.Position{Line: 3, Column: 3, Offset: 37},
				End:   diagnostic.Position{Line: 4, Column: 4, Offset: 81},
			},
			message: "method \"invalidOverride\" is marked override but no inherited method has the same signature",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostic identity/order changed\nwant: %#v\n got: %#v", want, got)
	}
}

func TestAnalyzeSOQLAggregateForEachLiteralUsesAggregateResultElements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "AggregateLoop.cls"), `
public class AggregateLoop {
  public void run(Set<String> keys) {
    for (AggregateResult ar : [
      SELECT COUNT(Id) cnt, External_Key__c keyValue
      FROM Scan_Event__c
      WHERE External_Key__c IN :keys
      GROUP BY External_Key__c
      HAVING COUNT(Id) > 1
    ]) {
      String keyValue = (String) ar.get('keyValue');
      Integer count = (Integer) ar.get('cnt');
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "AggregateLoop.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Scan_Event__c", Fields: []schema.Field{{Name: "External_Key__c", Type: "String"}}}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA024" {
			t.Fatalf("aggregate SOQL foreach should infer AggregateResult elements: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeProjectClassNamedStandardObjectKeepsLocalInstances(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Order.cls"), `
public class Order {
  public static List<Order> newInstances() {
    return new List<Order>{ new Order() };
  }

  public Decimal getBalance() {
    return 0;
  }

  public void addLines(List<String> lines) {
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "AdjustmentOrder.cls"), `
public class AdjustmentOrder extends Order {
}
`)
	writeSemaFile(t, filepath.Join(root, "OrderUse.cls"), `
public class OrderUse {
  public static void run() {
    Order testOrder = new AdjustmentOrder();
    testOrder.addLines(new List<String>());
    Decimal balance = testOrder.getBalance();

    List<Order> wrappers = Order.newInstances();
    Order actualWrapper = wrappers[0];
    System.assertNotEquals(null, actualWrapper);
    Decimal wrapperBalance = actualWrapper.getBalance();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Order.cls"),
		filepath.Join(root, "AdjustmentOrder.cls"),
		filepath.Join(root, "OrderUse.cls"),
	}}, schema.Schema{Objects: []schema.Object{{Name: "Order"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		switch diag.Code {
		case "GLADESEMA013", "GLADESEMA027", "GLADESEMA008":
			t.Fatalf("project class named like standard object should keep local instance typing: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMultilineSOQLDoesNotDeclareClauseLocals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "QueryUse.cls"), `
public class QueryUse {
  public Account run(Id accountId) {
    System.assertEquals(0, [SELECT Id FROM Account].size());
    Account row = [
      SELECT Id, Name, RecordTypeId
      FROM Account
      WHERE Id = :accountId
      ORDER BY Name
    ];
    return row;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "QueryUse.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA006" || diag.Code == "GLADESEMA014" {
			t.Fatalf("SOQL clauses should not be treated as local declarations: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCallArgumentAssignmentDoesNotDeclareLocal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ArgumentAssignment.cls"), `
public class ArgumentAssignment {
  private static Map<Type, List<ArgumentAssignment>> ByType = new Map<Type, List<ArgumentAssignment>>();

  private static void put(Type domainClass, ArgumentAssignment domain) {
    List<ArgumentAssignment> domains = ByType.get(domainClass);
    if (domains == null)
      ByType.put(
        domainClass,
        domains = new List<ArgumentAssignment>()
      );
    domains.add(domain);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "ArgumentAssignment.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA014" {
			t.Fatalf("call argument assignment should not be treated as a local declaration: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeSchemaAliasInstanceReceiverAllowsInstanceMethods(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SchemaAliasReceiver.cls"), `
public class SchemaAliasReceiver {
  private static String run(SObject customMetadataRecord) {
    SObjectType recordType = customMetadataRecord.getSObjectType();
    return recordType.getDescribe().getName();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "SchemaAliasReceiver.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" {
			t.Fatalf("schema alias variable should call instance methods as an instance: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeLowercaseLoopVariableDoesNotResolveAsPlatformType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "LoopVariableReceiver.cls"), `
public class LoopVariableReceiver {
  private static void run() {
    for (
      SObjectField sObjectField : Account.SObjectType.getDescribe()
        .fields.getMap()
        .values()
    ) {
      DescribeFieldResult dsr = sObjectField.getDescribe();
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "LoopVariableReceiver.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA027" {
			t.Fatalf("lowercase loop variable should not be treated as platform type receiver: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeStaticSOQLLiteralMatchesTypedListParameter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "QueryArg.cls"), `
public class QueryArg {
  public void run(Database.BatchableContext context) {
    acceptAccounts([SELECT Id FROM Account]);
    execute(context, [SELECT Id FROM Account]);
  }

  public void acceptAccounts(List<Account> accounts) {
  }

  public void execute(Database.BatchableContext context, List<Account> accounts) {
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "QueryArg.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" {
			t.Fatalf("static SOQL literal should match typed list parameter: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeDatabaseDMLNewListLiteralReturnsResultList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DatabaseDMLInlineList.cls"), `
public class DatabaseDMLInlineList {
  public void run(Account account) {
    Database.SaveResult[] results = Database.insert(new List<Account>{ account, account.clone(false, false, false, false) }, false);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "DatabaseDMLInlineList.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("Database.insert(new List<T>{...}) should return result list: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeInheritedNestedTypeShortName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FlexiblePayment.cls"), `
public abstract class FlexiblePayment {
  public abstract FlexiblePayment construct(Id recordId);
  protected abstract Builder getBuilder(SObject record);

  public class Builder {
    public Builder(FlexiblePayment payment, Object failure) {
    }

    public FlexiblePayment build() {
      return null;
    }
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "FlexiblePaymentForError.cls"), `
public class FlexiblePaymentForError extends FlexiblePayment {
  private Object failedResult;

  public override FlexiblePayment construct(Id recordId) {
    return this.getBuilder(null).build();
  }

  protected override Builder getBuilder(SObject record) {
    return new Builder(this, this.failedResult);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "FlexiblePayment.cls"),
		filepath.Join(root, "FlexiblePaymentForError.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA011" || diag.Code == "GLADESEMA019" {
			t.Fatalf("subclass should resolve inherited nested Builder short name: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeMessagingEmailClone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EmailClone.cls"), `
public class EmailClone {
  public Messaging.SingleEmailMessage copy(Messaging.SingleEmailMessage email) {
    return email.clone();
	}
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "EmailClone.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "email.clone") {
			t.Fatalf("Messaging email clone should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeUserDefinedClassClone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Email.cls"), `
public class Email {
  public String Html;
}
`)
	writeSemaFile(t, filepath.Join(root, "EmailService.cls"), `
public class EmailService {
  private String build(Email email) {
    return email.Html;
  }

  public String send(Email email) {
    return build(email.clone());
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Email.cls"),
		filepath.Join(root, "EmailService.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA008" && strings.Contains(diag.Message, "email.clone") {
			t.Fatalf("user-defined class clone should be recognized: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeChainedCallWithArgumentsOnNextLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringSelector.cls"), `
public class StringSelector {
  public static StringSelector newInstance() {
    return new StringSelector();
  }

  public List<String> selectNames(Set<String> names) {
    return new List<String>();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChainLineBreak.cls"), `
public class ChainLineBreak {
  public void run(Set<String> names) {
    List<String> selected = StringSelector.newInstance().selectNames
      (names);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "StringSelector.cls"),
		filepath.Join(root, "ChainLineBreak.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("chained call with next-line arguments should keep method return type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFluentStaticConstructorChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "OrderTestData.cls"), `
public class OrderTestData {
  public static OrderTestData newInstance() {
    return new OrderTestData();
  }

  public OrderTestData withIdentifier(Id recordId) {
    return this;
  }

  public OrderTestData withName(String name) {
    return this;
  }

  public OrderTestData withState(String state) {
    return this;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "OrderBuilderUse.cls"), `
public class OrderBuilderUse {
  public void run(Id orderId) {
    OrderTestData data = OrderTestData.newInstance()
      .withIdentifier(orderId)
      .withName('Order 0000024')
      .withState('Processed');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "OrderTestData.cls"),
		filepath.Join(root, "OrderBuilderUse.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("fluent static constructor chain should resolve left to right: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFluentConstructorChainKeepsReceiverType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Fabricated.cls"), `
public class Fabricated {
  public Fabricated(Type recordType) {
  }

  public Fabricated set(Schema.SObjectField field, Object value) {
    return this;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "FabricatedUse.cls"), `
public class FabricatedUse {
  public void run(Id productId) {
    Fabricated product = new Fabricated(Product__c.class)
      .set(Product__c.Id, productId)
      .set(Product__c.Name, 'Test Product')
      .set(Product__c.Mandatory__c, false);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Fabricated.cls"),
		filepath.Join(root, "FabricatedUse.cls"),
	}}, schema.Schema{Objects: []schema.Object{{
		Name: "Product__c",
		Fields: []schema.Field{
			{Name: "Name", Type: "Text"},
			{Name: "Mandatory__c", Type: "Checkbox"},
		},
	}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" || diag.Code == "GLADESEMA019" {
			t.Fatalf("constructor fluent chain should keep Fabricated return type: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCommaSeparatedLocalInitializersStopAtDeclaratorComma(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CommaLocals.cls"), `
public class CommaLocals {
  public void run(Account acct1, Account acct2) {
    Account acct1changed = acct1.clone(),
      acct2changed = acct2.clone();
    acct1changed.Name = 'Changed';
    acct2changed.Name = 'Changed';
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "CommaLocals.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("first comma-separated initializer should not include the next declarator: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeCustomMetadataStandardFieldsAreStrings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesCustomMetadataStandardFields.cls"), `
public class UsesCustomMetadataStandardFields {
  public String apiName(Feature__mdt feature) {
    return feature.QualifiedApiName;
  }

  public String label(Feature__mdt feature) {
    return feature.Label;
  }

  public Boolean blankNamespace(Feature__mdt feature) {
    return String.isBlank(feature.NamespacePrefix);
  }

  public VisualEditor.DataRow row(Feature__mdt feature) {
    return new VisualEditor.DataRow(feature.Label, feature.QualifiedApiName);
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesCustomMetadataStandardFields.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Feature__mdt"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("custom metadata standard fields should be typed as strings: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectReferencedLiteralFieldTypesFlowToAssignmentsAndConstructors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FeeDto.cls"), `
public class FeeDto {
  public String paymentMethod;
  public Boolean mandatory;

  public FeeDto(String paymentMethod, Boolean mandatory) {
    this.paymentMethod = paymentMethod;
    this.mandatory = mandatory;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesInferredFeeFields.cls"), `
public class UsesInferredFeeFields {
  public void seed() {
    Fee__c fee = new Fee__c(
      PaymentMethodType__c = 'Credit Card',
      Mandatory__c = false
    );
  }

  public void run(Fee__c fee) {
    FeeDto dto = new FeeDto(fee.PaymentMethodType__c, fee.Mandatory__c);
    dto.paymentMethod = fee.PaymentMethodType__c;
    dto.mandatory = fee.Mandatory__c;
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "FeeDto.cls"),
			filepath.Join(root, "UsesInferredFeeFields.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "Fee__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("literal-backed inferred fields should carry useful types: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectReferencedDateFactoryFieldTypeFlowsToCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesInferredDateFields.cls"), `
public class UsesInferredDateFields {
  public void seed() {
    Agreement__c agreement = new Agreement__c(
      StartDate__c = Date.today(),
      EndDate__c = Date.newInstance(2026, 5, 7)
    );
  }

  public String run(Agreement__c agreement) {
    return agreement.StartDate__c.format() + agreement.EndDate__c.format();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesInferredDateFields.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Agreement__c"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("date factory inferred fields should carry Date type: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectReferencedAnyFieldDoesNotBlockStringOverload(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "StringConsumer.cls"), `
public class StringConsumer {
  public static void accept(String value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesUnknownInferredField.cls"), `
public class UsesUnknownInferredField {
  public void run(Config__mdt config) {
    StringConsumer.accept(config.FromStates__c);
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "StringConsumer.cls"),
			filepath.Join(root, "UsesUnknownInferredField.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{Name: "Config__mdt"}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unknown inferred fields should not force Object overload mismatches: %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectReferencedSchemaDoesNotInferFieldsForAuthoritativeObjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesTypoField.cls"), `
public class UsesTypoField {
  public void seed() {
    Fee__c seeded = new Fee__c(Typo__c = 'not metadata');
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesTypoField.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Fee__c",
		Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
		},
	}}})
	enriched := enrichIndexWithProjectReferencedSchemaFields(index)
	for _, object := range enriched.Objects {
		if !strings.EqualFold(object.Name, "Fee__c") {
			continue
		}
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, "Typo__c") {
				t.Fatalf("authoritative object inferred typo field: %#v", object.Fields)
			}
		}
		return
	}
	t.Fatalf("missing Fee__c object: %#v", enriched.Objects)
}

func TestAnalyzeProjectReferencedSchemaIgnoresNestedNamedArgsInSObjectLiteral(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "NestedArgs.cls"), `
public class NestedArgs {
  public Object helper(Object value) {
    return value;
  }

  public void seed() {
    Fee__c fee = new Fee__c(
      Name = 'Fee',
      Description__c = helper(new Object(Nested__c = 'not a Fee field'))
    );
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "NestedArgs.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Fee__c"}}})
	enriched := enrichIndexWithProjectReferencedSchemaFields(index)
	for _, object := range enriched.Objects {
		if !strings.EqualFold(object.Name, "Fee__c") {
			continue
		}
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, "Nested__c") {
				t.Fatalf("nested named arg inferred as Fee__c field: %#v", object.Fields)
			}
		}
		return
	}
	t.Fatalf("missing Fee__c object: %#v", enriched.Objects)
}

func TestAnalyzeProjectReferencedSchemaInfersObjectWhenSchemaEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ManageApprovalSteps.cls"), `
global with sharing class ManageApprovalSteps {
  public static List<Member> getApprovalProcessSteps(Id approvalProcessId) {
    List<ApprovalProcessStepDefinition__c> steps = [
      SELECT Id, Name
      FROM ApprovalProcessStepDefinition__c
      WHERE ApprovalProcessDefinition__c = :approvalProcessId
      ORDER BY Order__c ASC
    ];
    List<Member> members = new List<Member>();
    for (ApprovalProcessStepDefinition__c step : steps) {
      members.add(new Member(step.Name, step.Id));
    }
    return members;
  }

  public static void saveApprovalSteps(List<ApprovalProcessStepDefinition__c> appSteps) {
    update appSteps;
  }

  global class Member {
    public String label;
    public String value;
    public Member(String label, String value) {
      this.label = label;
      this.value = value;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "ManageApprovalSteps.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", "ApprovalProcessStepDefinition__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA004", "ApprovalProcessStepDefinition__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA006", "ApprovalProcessStepDefinition__c")
}

func TestAnalyzeProjectReferencedSchemaInfersEnhancedForObjectWhenSchemaPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "ManageApprovalSteps.cls"), `
global with sharing class ManageApprovalSteps {
  public static List<Member> getApprovalProcessSteps(Id approvalProcessId) {
    List<ApprovalProcessStepDefinition__c> steps = [
      SELECT Id, Name
      FROM ApprovalProcessStepDefinition__c
      WHERE ApprovalProcessDefinition__c = :approvalProcessId
      ORDER BY Order__c ASC
    ];
    List<Member> members = new List<Member>();
    for (ApprovalProcessStepDefinition__c step : steps) {
      members.add(new Member(step.Name, step.Id));
    }
    return members;
  }

  public static void saveApprovalSteps(List<ApprovalProcessStepDefinition__c> appSteps) {
    update appSteps;
  }

  global class Member {
    public String label;
    public String value;
    public Member(String label, String value) {
      this.label = label;
      this.value = value;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "ManageApprovalSteps.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Existing__c", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", "ApprovalProcessStepDefinition__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA004", "ApprovalProcessStepDefinition__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA006", "ApprovalProcessStepDefinition__c")
}

func TestAnalyzeProjectReferencedFieldsRefineExistingAnyAndObjectFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "FeeDto.cls"), `
public class FeeDto {
  public FeeDto(String paymentMethod, Boolean mandatory) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "UsesExistingLooseFields.cls"), `
public class UsesExistingLooseFields {
  public void seed() {
    Fee__c fee = new Fee__c(
      PaymentMethodType__c = 'Credit Card',
      Mandatory__c = false
    );
  }

  public void run(Fee__c fee) {
    FeeDto dto = new FeeDto(fee.PaymentMethodType__c, fee.Mandatory__c);
  }
}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "FeeDto.cls"),
			filepath.Join(root, "UsesExistingLooseFields.cls"),
		},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Fee__c",
		Fields: []schema.Field{
			{Name: "PaymentMethodType__c", Type: "Any"},
			{Name: "Mandatory__c", Type: "Object"},
		},
	}}})
	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("literal-backed loose fields should refine to useful types: %#v", result.Diagnostics)
	}
}

func TestProjectReferencedSchemaInfersStandardObjectLiteralFields(t *testing.T) {
	t.Parallel()
	ctx := newSemaProjectReferencedSchemaContext([]schema.Object{{Name: "Existing__c"}}, "")
	semaProjectReferencedSchemaFieldsFromSource(ctx, `
public class UsesStandardObject {
  public void seed() {
    new Campaign(
      Name = 'Campaign',
      ExpectedRevenue = 100, // Currency(18,0)
      ExpectedResponse = Math.mod(1,10) * 10 + Math.mod(1,100)/100 // Percent(8,2)
    );
  }
}
`)
	for _, object := range ctx.objects {
		if !strings.EqualFold(object.Name, "Campaign") {
			continue
		}
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, "ExpectedResponse") {
				return
			}
		}
		t.Fatalf("Campaign missing ExpectedResponse field: %#v", object.Fields)
	}
	t.Fatalf("missing inferred Campaign object: %#v", ctx.objects)
}

func TestProjectReferencedSchemaFieldsFromSourceKeepsMixedScanAllocationsBounded(t *testing.T) {
	var source strings.Builder
	source.WriteString("public class UsesInferredFields {\n")
	source.WriteString("  public void run() {\n")
	for i := 0; i < 40; i++ {
		n := strconv.Itoa(i)
		source.WriteString("    Entity__c entity")
		source.WriteString(n)
		source.WriteString(" = new Entity__c();\n")
		source.WriteString("    Gateway__c gateway")
		source.WriteString(n)
		source.WriteString(" = new Gateway__c(Entity__c = entity")
		source.WriteString(n)
		source.WriteString(".Id);\n")
		source.WriteString("    Fee__c fee")
		source.WriteString(n)
		source.WriteString(" = new Fee__c(PaymentMethodType__c = 'Credit Card', Mandatory__c = false, Gateway__c = gateway")
		source.WriteString(n)
		source.WriteString(".Id);\n")
		source.WriteString("    String paymentMethod")
		source.WriteString(n)
		source.WriteString(" = fee")
		source.WriteString(n)
		source.WriteString(".PaymentMethodType__c;\n")
	}
	source.WriteString("  }\n")
	source.WriteString("}\n")
	sourceText := source.String()
	objects := []schema.Object{
		{Name: "Entity__c"},
		{Name: "Gateway__c"},
		{Name: "Fee__c"},
	}

	allocs := testing.AllocsPerRun(5, func() {
		ctx := newSemaProjectReferencedSchemaContext(objects, "")
		semaProjectReferencedSchemaFieldsFromSource(ctx, sourceText)
	})
	if allocs > 1100 {
		t.Fatalf("project-referenced schema scan allocated %.0f times, want at most 1100", allocs)
	}
}

func TestSemaApexTypeForObjectSchemaFieldUsesNameFallbacks(t *testing.T) {
	tests := []struct {
		field schema.Field
		want  string
	}{
		{field: schema.Field{Name: "pkg__PaymentMethodType__c", Type: "Object"}, want: "String"},
		{field: schema.Field{Name: "pkg__Mandatory__c", Type: "Object"}, want: "Boolean"},
		{field: schema.Field{Name: "pkg__StartDate__c", Type: "Object"}, want: "Date"},
		{field: schema.Field{Name: "pkg__StartDateTime__c", Type: "Object"}, want: "Datetime"},
	}
	for _, tt := range tests {
		if got := semaApexTypeForSchemaField(tt.field); got != tt.want {
			t.Fatalf("semaApexTypeForSchemaField(%#v) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestInferSemaFluentNestedConstructorChainType(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Query.cls"), `
public class Query {
  public class Condition {
    public Condition equals(String fieldName, Object value) {
      return this;
    }
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "Query.cls")},
	}, schema.Schema{})
	model := buildSemaTypeMemberView(index)
	scope := map[string]string{semaCurrentTypeScopeKey: "Query"}

	got := inferSemaArgTypeWithModel("new Query.Condition().equals('Name', 'test').equals('Reason', 'Other')", scope, model)
	if got != "Query.Condition" {
		t.Fatalf("fluent nested constructor chain type = %q, want Query.Condition", got)
	}
	got = inferSemaArgTypeWithModel("return new Query.Condition()\n      .equals('Name', 'test')\n      .equals('Reason', 'Other')", scope, model)
	if got != "Query.Condition" {
		t.Fatalf("returned fluent nested constructor chain type = %q, want Query.Condition", got)
	}
	got = inferSemaArgTypeWithModel("return new Query.Condition(\n        Query.LogicalOperator.OR_VALUE\n      )\n      .equals('Name', 'test')\n      .equals('Reason', 'Other')", scope, model)
	if got != "Query.Condition" {
		t.Fatalf("multiline constructor fluent chain type = %q, want Query.Condition", got)
	}
}

func TestSplitSemaMethodPathIgnoresDottedGenericArgument(t *testing.T) {
	if receiver, field, ok := splitSemaMethodPath("new Map<String, Schema.SObjectField>"); ok {
		t.Fatalf("generic type argument dot should not split as method path: %q %q", receiver, field)
	}
	receiver, field, ok := splitSemaMethodPath("new Map<String, Schema.SObjectField>{}.put")
	if !ok {
		t.Fatal("expected method path after generic literal")
	}
	if receiver != "new Map<String, Schema.SObjectField>{}" || field != "put" {
		t.Fatalf("method path = %q.%q", receiver, field)
	}
}

func TestInferSemaArgTypeWithModelIgnoresStatementSpans(t *testing.T) {
	if got := inferSemaArgTypeWithModel("new Map<String, Schema.SObjectField>{}; fflib_SObjectDescribe", map[string]string{}, nil); got != "" {
		t.Fatalf("statement span type = %q, want empty", got)
	}
	if got := inferSemaArgTypeWithModel("'a;b'", map[string]string{}, nil); got != "String" {
		t.Fatalf("quoted semicolon type = %q, want String", got)
	}
}

func TestInferSemaArgTypeSObjectTypeFieldsPath(t *testing.T) {
	if got := inferSemaArgTypeWithModel("Schema.SObjectType.Account.fields.Name", map[string]string{}, nil); got != "Schema.DescribeFieldResult" {
		t.Fatalf("SObjectType fields path type = %q, want Schema.DescribeFieldResult", got)
	}
}

func TestAnalyzeSafeNavigationMethodReceiverField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CustomerOrder.cls"), `
public class CustomerOrder {
  public String Name;
}
`)
	writeSemaFile(t, filepath.Join(root, "OrderLine.cls"), `
public class OrderLine {
  public CustomerOrder getOrder() {
    return new CustomerOrder();
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "OrderLineUse.cls"), `
public class OrderLineUse {
  public String run(OrderLine line) {
    return line.getOrder()?.Name;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "CustomerOrder.cls"),
		filepath.Join(root, "OrderLine.cls"),
		filepath.Join(root, "OrderLineUse.cls"),
	}}, schema.Schema{})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA019" || diag.Code == "GLADESEMA008" {
			t.Fatalf("safe-navigation method receiver field should resolve: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeFieldVisibilityDiagnostics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Secret.cls"), `
public class Secret {
  private String code;
  protected String guarded;
  public String ownAccess() {
    return code;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "ChildSecret.cls"), `
public class ChildSecret extends Secret {
  public String run() {
    return guarded;
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "Intruder.cls"), `
public class Intruder {
  public void run() {
    Secret s = new Secret();
    String a = s.code;
    String b = s.guarded;
    s.code = 'x';
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Secret.cls"),
		filepath.Join(root, "ChildSecret.cls"),
		filepath.Join(root, "Intruder.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	count := 0
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA010" {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("GLADESEMA010 count = %d diagnostics=%#v", count, result.Diagnostics)
	}
}

func TestAnalyzeTriggerObject(t *testing.T) {
	t.Parallel()
	index := typesys.Index{
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "Missing__c", File: "Thing.trigger"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "GLADESEMA001" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func TestAnalyzeBareInstanceFieldArgumentInChainedInterfaceCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "QueryBindings.cls"), `
public class QueryBindings {}
`)
	writeSemaFile(t, filepath.Join(root, "QueryConditions.cls"), `
public class QueryConditions {
  public interface Condition {
    String toSOQL(QueryBindings bindings);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "QueryObject.cls"), `
public class QueryObject {
  protected final QueryBindings bindings = new QueryBindings();
  private List<QueryConditions.Condition> conditions = new List<QueryConditions.Condition>();
  private QueryConditions.Condition compileConditions(List<QueryConditions.Condition> source) {
    return source.get(0);
  }
  public String run() {
    return compileConditions(conditions).toSOQL(bindings);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "QueryBindings.cls"),
		filepath.Join(root, "QueryConditions.cls"),
		filepath.Join(root, "QueryObject.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" || diag.Code == "GLADESEMA008" {
			t.Fatalf("bare instance field argument should type-check in chained interface call: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeBareInstanceFieldsInImplicitChainedInterfaceCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "QueryBindings.cls"), `
public class QueryBindings {}
`)
	writeSemaFile(t, filepath.Join(root, "QueryConditions.cls"), `
public class QueryConditions {
  public interface Condition {
    String toSOQL(QueryBindings bindings);
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "QueryBuilder.cls"), `
public class QueryBuilder {
  public QueryBuilder setWhere(String whereClause) {
    return this;
  }
  public QueryBuilder setHaving(String having) {
    return this;
  }
  public QueryBuilder fromObject(String objectName) {
    return this;
  }
  public String toSOQL() {
    return '';
  }
}
`)
	queryObjectSource := `
public class QueryObject {
  protected final QueryBuilder queryBuilder = new QueryBuilder();
  protected final List<QueryConditions.Condition> havingConditions = new List<QueryConditions.Condition>();
  protected final QueryBindings bindings = new QueryBindings();
  private QueryConditions.Condition compileConditions(QueryConditions.Condition[] source) {
    return source.get(0);
  }
  public void run() {
    this.queryBuilder
      .setWhere(compileConditions(havingConditions).toSOQL(bindings))
      .setHaving(compileConditions(havingConditions).toSOQL(bindings))
      .fromObject('Account')
      .toSOQL();
  }
}
`
	writeSemaFile(t, filepath.Join(root, "QueryObject.cls"), queryObjectSource)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "QueryBindings.cls"),
		filepath.Join(root, "QueryConditions.cls"),
		filepath.Join(root, "QueryBuilder.cls"),
		filepath.Join(root, "QueryObject.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA009" || diag.Code == "GLADESEMA008" {
			t.Fatalf("bare instance fields should type-check in implicit chained interface call: %#v", result.Diagnostics)
		}
	}
}

func TestAnalyzeAbstractReceiverMethodThroughThisField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "EmailTemplate.cls"), `
public abstract class EmailTemplate {
  public abstract String getId();
  public abstract EmailTemplateApiName getApiName();
}
`)
	writeSemaFile(t, filepath.Join(root, "EmailTemplateApiName.cls"), `
public class EmailTemplateApiName {}
`)
	writeSemaFile(t, filepath.Join(root, "fflib_ApexMocks.cls"), `
public class fflib_ApexMocks {
  public Object mock(Type typ) { return null; }
  public void startStubbing() {}
  public Stubber when(Object value) { return new Stubber(); }
  public void stopStubbing() {}
  public class Stubber {
    public void thenReturn(Object value) {}
  }
}
`)
	writeSemaFile(t, filepath.Join(root, "EmailTemplateTestData.cls"), `
public class EmailTemplateTestData {
  private final fflib_ApexMocks mocks;
  private final EmailTemplate mockEmailTemplate;
  private String id;
  private EmailTemplateApiName apiName;

  public EmailTemplateTestData() {
    this.mocks = new fflib_ApexMocks();
    this.mockEmailTemplate = (EmailTemplate)this.mocks.mock(EmailTemplate.class);
  }

  public EmailTemplate build() {
    mocks.startStubbing();
    mocks.when(this.mockEmailTemplate.getId())
      .thenReturn(this.id);
    mocks.when(this.mockEmailTemplate.getApiName())
      .thenReturn(this.apiName);
    mocks.stopStubbing();
    return this.mockEmailTemplate;
  }
}
`)
	duplicateRoot := filepath.Join(root, "dependency")
	if err := os.MkdirAll(duplicateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(duplicateRoot, "EmailTemplateTestData.cls"), `
public class EmailTemplateTestData {}
`)
	index := typesys.Build(project.Project{
		Root: root,
		ApexFiles: []string{
			filepath.Join(root, "EmailTemplate.cls"),
			filepath.Join(root, "EmailTemplateApiName.cls"),
			filepath.Join(root, "fflib_ApexMocks.cls"),
			filepath.Join(root, "EmailTemplateTestData.cls"),
			filepath.Join(duplicateRoot, "EmailTemplateTestData.cls"),
		},
	}, schema.Schema{})
	for i := range index.Types {
		if index.Types[i].File == filepath.Join(duplicateRoot, "EmailTemplateTestData.cls") {
			index.Types[i].Namespace = "znu"
			index.Types[i].Dependency = true
			index.Types[i].SourceRoot = duplicateRoot
		} else {
			index.Types[i].Namespace = "acme"
		}
	}
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "this.mockEmailTemplate.getId")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "this.mockEmailTemplate.getApiName")
}

func TestDiagnoseMethodCallResolvesInferredDottedReceiverMethod(t *testing.T) {
	t.Parallel()
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass,
			Name: "EmailTemplate",
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationMethod, Name: "getId", Type: "String", Modifiers: []string{"public", "abstract"}},
			},
		},
		{
			Kind: apexast.DeclarationClass,
			Name: "EmailTemplateTestData",
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationField, Name: "mockEmailTemplate", Type: "EmailTemplate", Modifiers: []string{"private"}},
			},
		},
	}}
	model := buildSemaTypeMemberView(index)
	scope := map[string]string{semaCurrentTypeScopeKey: "EmailTemplateTestData"}
	for name, fieldType := range semaFieldScope(model, "EmailTemplateTestData", make(map[string]bool)) {
		scope[name] = fieldType
	}
	typ := index.Types[1]
	member := typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "build"}
	diags := NewAnalyzer().diagnoseMethodCall(typ, member, "this.mockEmailTemplate.getId", nil, nil, true, "instance", 0, len("this.mockEmailTemplate.getId"), "this.mockEmailTemplate.getId()", scope, model)
	for _, diag := range diags {
		if diag.Code == "GLADESEMA008" {
			t.Fatalf("inferred dotted receiver method should resolve: %#v", diags)
		}
	}

	irScope := newIRSemaScope(scope)
	irDiags := NewAnalyzer().checkIRCall(typ, member, ir.Expr{Kind: ir.ExprCall, Callee: "this.mockEmailTemplate.getId"}, irScope, 0, 0, "this.mockEmailTemplate.getId()", model, nil)
	for _, diag := range irDiags {
		if diag.Code == "GLADESEMA008" {
			t.Fatalf("IR inferred dotted receiver method should resolve: %#v", irDiags)
		}
	}
}

func TestAnalyzeKnownPlatformFieldReceiverMethodStaysPermissive(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "UsesEmailTemplate.cls"), `
public class UsesEmailTemplate {
  private EmailTemplate template;
  public void run() {
    this.template.getId();
    this.template.getApiName();
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "UsesEmailTemplate.cls")},
	}, schema.Schema{})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "this.template.getId")
	assertNoDiagnosticContaining(t, result, "GLADESEMA008", "this.template.getApiName")
}

func TestAnalyzeStandardSObjectInitializerNamedArgAfterLineComment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "SortCollectionTest.cls"), `
public class SortCollectionTest {
  static void makeData() {
    Integer i = 1;
    String name = 'Campaign';
    new Campaign(
      Name = name,
      ExpectedRevenue = i * 100, // Currency(18,0)
      ExpectedResponse = Math.mod(i,10) * 10 + Math.mod(i,100)/100 // Percent(8,2)
    );
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "SortCollectionTest.cls")},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Existing__c",
	}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA013", "ExpectedResponse")
}

func writeSemaFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTypeNames(t *testing.T) {
	got := extractTypeNames("Map<String,List<Account>>")
	want := []string{"String", "Account"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v", got)
		}
	}
}

func TestIsSemaConstructorCallAtHonorsWhitespaceBeforeNew(t *testing.T) {
	body := "return new DomainBase.Context().value;"
	start := strings.Index(body, "DomainBase.Context")
	if start < 0 {
		t.Fatal("missing constructor call")
	}
	if !isSemaConstructorCallAt(body, start) {
		t.Fatalf("constructor call after whitespace was not recognized")
	}
	notConstructor := "return renew DomainBase.Context().value;"
	start = strings.Index(notConstructor, "DomainBase.Context")
	if start < 0 {
		t.Fatal("missing non-constructor call")
	}
	if isSemaConstructorCallAt(notConstructor, start) {
		t.Fatalf("identifier ending in new was recognized as a constructor call")
	}
}

func TestSemaEnumValuesStripsComments(t *testing.T) {
	tests := []struct {
		name     string
		decl     string
		expected []string
	}{
		{
			name:     "no comments",
			decl:     `public enum E { A, B, C }`,
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "line comments",
			decl:     "public enum E {\nA, // comment\nB, // another\nC\n}",
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "block comments",
			decl:     "public enum E { /* block */ A, /* block */ B, C }",
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "mixed comments",
			decl:     "public enum E { /* block */ A, // line\nB, /* block */ C }",
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "comment before first value",
			decl:     "public enum E { /* header */ A, B }",
			expected: []string{"A", "B"},
		},
		{
			name:     "comment after last value",
			decl:     "public enum E { A, B /* trailing */ }",
			expected: []string{"A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "Test.cls")
			if err := os.WriteFile(path, []byte(tt.decl), 0644); err != nil {
				t.Fatal(err)
			}
			typ := typesys.TypeSymbol{
				Kind: "enum",
				Name: "E",
				File: path,
				Range: diagnostic.Range{
					Start: diagnostic.Position{Offset: 0},
					End:   diagnostic.Position{Offset: len(tt.decl)},
				},
			}
			got := semaEnumValues(typ)
			if !slicesEqual(got, tt.expected) {
				t.Errorf("semaEnumValues() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hasDiagnosticCode(diagnostics []diagnostic.Diagnostic, code string) bool {
	for _, diag := range diagnostics {
		if diag.Code == code {
			return true
		}
	}
	return false
}
