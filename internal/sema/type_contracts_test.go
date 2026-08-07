package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDatabaseInsertAsyncSingleRecordCallbackAccessLevelResolves(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"SaveCallback.cls": `public class SaveCallback implements DataSource.AsyncSaveCallback { public void processSave(Database.SaveResult result) {} }`,
		"Probe.cls":        `public class Probe { public void run() { Database.SaveResult result = Database.insertAsync(new Account(Name = 'Async'), new SaveCallback(), AccessLevel.USER_MODE); } }`,
	})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "GLADESEMA022" && strings.Contains(diagnostic.Message, "insertAsync") {
			t.Fatalf("single-record insertAsync callback/access-level call was ambiguous: %#v", result.Diagnostics)
		}
	}
}

func TestDataSourceAsyncSaveCallbackUsesClassInheritance(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"SaveCallback.cls": `public class SaveCallback extends DataSource.AsyncSaveCallback { public override void processSave(Database.SaveResult result) {} }`,
	})
	if result.HasErrors() {
		t.Fatalf("expected AsyncSaveCallback class inheritance to compile: %#v", result.Diagnostics)
	}
}

func TestDataSourceAsyncDeleteCallbackUsesClassInheritance(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DeleteCallback.cls": `public class DeleteCallback extends DataSource.AsyncDeleteCallback { public override void processDelete(Database.DeleteResult result) {} }`,
	})
	if result.HasErrors() {
		t.Fatalf("expected AsyncDeleteCallback class inheritance to compile: %#v", result.Diagnostics)
	}
}

func TestEventPublishCallbacksAreInterfaces(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"PublishCallback.cls": `public class PublishCallback implements eventbus.EventPublishSuccessCallback, eventbus.EventPublishFailureCallback { public void onSuccess(eventbus.SuccessResult result) {} public void onFailure(eventbus.FailureResult result) {} }`,
	})
	if result.HasErrors() {
		t.Fatalf("expected EventBus publish callbacks to be implementable interfaces: %#v", result.Diagnostics)
	}
}

func TestTypeContractRejectsInvalidSourceTypesAndLiterals(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"currency source type":      `public class Probe { public Currency value; }`,
		"raw list construction":     `public class Probe { public void run() { System.debug(new List()); } }`,
		"raw map construction":      `public class Probe { public void run() { System.debug(new Map()); } }`,
		"list generic arity":        `public class Probe { public List<String, Integer> values; }`,
		"map generic arity":         `public class Probe { public Map<String> values; }`,
		"collection depth":          `public class Probe { public List<List<List<List<List<List<List<List<List<String>>>>>>>>> values; }`,
		"integer overflow":          `public class Probe { public void run() { System.debug(2147483648); } }`,
		"minimum integer magnitude": `public class Probe { public Integer value() { return -2147483648; } }`,
		"cast minimum magnitude":    `public class Probe { public Integer value() { return (Integer) -2147483648; } }`,
		"binary integer overflow":   `public class Probe { public void run() { System.debug(0 - 2147483648); } }`,
		"postfix binary overflow":   `public class Probe { public void run() { Integer value = 0; System.debug(value++ - 2147483648); } }`,
		"scientific notation":       `public class Probe { public void run() { System.debug(1e3); } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("expected source contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractIgnoresNumericTextInStringsAndComments(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
public class Probe {
  public void run() {
    String salesforceId = '003000000000001';
    String exponent = '1e3';
    // 2147483648 and 1e3 are not source literals.
  }
}
`})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
		t.Fatalf("numeric text outside code was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsLargeDecimalLiteral(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public Decimal value() { return 2147483648.0; } }`,
	})
	if result.HasErrors() {
		t.Fatalf("large Decimal literal was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsLongSuffixLiteral(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public Long value() { return 2147483648L; } }`,
	})
	if result.HasErrors() {
		t.Fatalf("Long suffix literal was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractRejectsRawCollectionConstructorsCaseInsensitively(t *testing.T) {
	for _, constructor := range []string{"lIsT", "mAp", "SET"} {
		t.Run(constructor, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": "public class Probe { public void run() { System.debug(new " + constructor + "()); } }",
			})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("raw %s constructor was accepted: %#v", constructor, result.Diagnostics)
			}
		})
	}
}

func TestTypeContractRejectsInvalidOperators(t *testing.T) {
	t.Parallel()
	for name, expression := range map[string]string{
		"negates integer":          "!1",
		"orders booleans":          "true < false",
		"multiplies string":        "'value' * 2",
		"casts incompatible types": "(String) 1",
		"impossible instanceof":    "1 instanceof String",
		"incompatible coalesce":    "'value' ?? 1",
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": "public class Probe { public void run() { System.debug(" + expression + "); } }",
			})
			if !result.HasErrors() {
				t.Fatalf("expected expression contract diagnostic for %s, got %#v", expression, result.Diagnostics)
			}
		})
	}
}

func TestTypeContractsAllowTimeOrdering(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"TimeOrdering.cls": `
public class TimeOrdering {
  public Integer compare(Time left, Time right) {
    return left < right ? -1 : 1;
  }
}
`,
	})
	assertNoDiagnosticContaining(t, result, "GLADESEMA019", "ordering operator")
}

func TestTypeContractsAllowDateAndDatetimeOrdering(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"DateOrdering.cls": `
public class DateOrdering {
  public Boolean compare(Date day, Datetime instant) {
    return day <= instant && instant > day;
  }
}
`,
	})
	assertNoDiagnosticContaining(t, result, "GLADESEMA019", "ordering operator")
}

func TestTypeContractAllowsIterableInstanceofAtAllTestedAPIVersions(t *testing.T) {
	for _, apiVersion := range []string{"59.0", "60.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe { public Boolean run(List<Account> values) { return values instanceof Iterable<SObject>; } }`,
			}, apiVersion)
			if result.HasErrors() {
				t.Fatalf("Iterable instanceof control was rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractRejectsAlwaysTrueNestedIterableInstanceofAtAPIVersion60(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe { public virtual class Base {} public class Sub extends Base {} public Boolean run() { List<Sub> values = new List<Sub>(); return values instanceof Iterable<Base>; } }`,
	}, "60.0")
	if !result.HasErrors() || !declarationDiagnosticMatching(result, "always true") {
		t.Fatalf("always-true nested Iterable instanceof was accepted: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsNumericWideningAndCompatibleCasts(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run() {
    Decimal widened = 1;
    System.debug((Object) 'value');
    System.debug(1.0 + 2);
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("unexpected compatible expression diagnostic: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsDateAndDatetimeDayArithmetic(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run() {
    Date yesterday = Date.today() - 1;
    Date nextYear = System.today() + 365;
    Datetime earlier = System.now() - 1;
    Datetime later = Datetime.now() + 1;
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("Date and Datetime day arithmetic was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsRuntimeCastsBetweenInterfacesAndClasses(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public interface Selector {}
  public abstract class BaseSelector {}
  public void run(Selector selector) {
    if (selector instanceof BaseSelector) {
      BaseSelector base = (BaseSelector) selector;
    }
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("runtime interface/class cast was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsInterfaceValueInstanceofNestedImplementer(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Calculator.cls": `public interface Calculator {}`,
		"Probe.cls": `
public class Probe {
  private class LocalCalculator implements Calculator {}
  public void assertLocalCalculator(Calculator calculator) {
    System.assert(calculator instanceof LocalCalculator);
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("interface value instanceof its nested implementation was rejected: %#v", result.Diagnostics)
	}
}

func TestRuntimeTypeTestCompatibilityAllowsNestedConcreteInterfaceImplementer(t *testing.T) {
	model := newSemaTypeMemberState(buildTypeMembers(typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationInterface, Name: "Calculator"},
		{Kind: apexast.DeclarationClass, Name: "Probe.LocalCalculator", NestingDepth: 1, Interfaces: []string{"Calculator"}},
	}})).view()
	if !semaRuntimeTypeTestCompatible("Probe", "Calculator", "LocalCalculator", model) {
		t.Fatal("interface value instanceof nested concrete implementer was treated as impossible")
	}
}

func TestRuntimeTypeTestCompatibilityRejectsUnrelatedConcreteClass(t *testing.T) {
	model := newSemaTypeMemberState(buildTypeMembers(typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationInterface, Name: "Calculator"},
		{Kind: apexast.DeclarationClass, Name: "Probe.Unrelated", NestingDepth: 1},
	}})).view()
	if semaRuntimeTypeTestCompatible("Probe", "Calculator", "Unrelated", model) {
		t.Fatal("interface value instanceof unrelated concrete class was accepted")
	}
}

func TestRuntimeTypeTestCompatibilityIsConservativeForIncompleteExternalTypes(t *testing.T) {
	model := newSemaTypeMemberState(buildTypeMembers(typesys.Index{})).view()
	if !semaRuntimeTypeTestCompatible("", "external.Selector", "external.BaseSelector", model) {
		t.Fatal("incomplete external runtime types were treated as definitely incompatible")
	}
}

func TestTypeContractAllowsExplicitSObjectListCasts(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(List<SObject> records) {
    List<Account> accounts = (List<Account>) records;
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("explicit SObject list cast was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsQueryLocatorRuntimeCasts(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(Iterable<Object> values) {
    Database.QueryLocator locator = (Database.QueryLocator) values;
    Iterator<Object> iterator = locator.iterator();
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("QueryLocator runtime cast was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsQueryLocatorRuntimeTypeTests(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(Database.QueryLocator locator, Iterable<Object> values) {
    Iterable<Object> iterable = (Iterable<Object>) locator;
    if (values instanceof Database.QueryLocator) {}
    if (locator instanceof Iterable<Object>) {}
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("QueryLocator runtime type test was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsQueryLocatorRuntimeTypesForAnyIterableElement(t *testing.T) {
	for name, iterable := range map[string]string{
		"Object":                 "Iterable<Object>",
		"SObject":                "Iterable<SObject>",
		"Account":                "Iterable<Account>",
		"System Iterable Object": "System.Iterable<Object>",
		"String":                 "Iterable<String>",
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": `
public class Probe {
  public void run(Database.QueryLocator locator, ` + iterable + ` values) {
    Database.QueryLocator fromIterable = (Database.QueryLocator) values;
    ` + iterable + ` fromLocator = (` + iterable + `) locator;
    if (values instanceof Database.QueryLocator) {}
    if (locator instanceof ` + iterable + `) {}
  }
}
`,
			})
			if result.HasErrors() {
				t.Fatalf("QueryLocator runtime relation rejected %s: %#v", iterable, result.Diagnostics)
			}
		})
	}
}

func TestTypeContractRejectsIncompatibleParameterizedCollectionCasts(t *testing.T) {
	for name, source := range map[string]string{
		"different collection bases": `
public class Probe {
  public void run() {
    Set<Integer> values = (Set<Integer>) new List<String>();
  }
}
`,
		"incompatible list elements": `
public class Probe {
  public void run() {
    List<Integer> values = (List<Integer>) new List<String>();
  }
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("incompatible parameterized cast was accepted: %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractRejectsPropertyCapabilityAndSafeNavigationMisuse(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"writes get-only property":          `public class Probe { public String ReadOnly { get; } public void run() { ReadOnly = 'value'; } }`,
		"reads set-only property":           `public class Probe { public String WriteOnly { set; } public void run() { System.debug(WriteOnly); } }`,
		"safe navigation static receiver":   `public class Probe { public static String label; public void run() { System.debug(Probe?.label); } }`,
		"safe navigation assignment target": `public class Probe { public String label; public void run(Probe value) { value?.label = 'value'; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("expected property/safe-navigation contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractAllowsLocalToShadowGetOnlyPropertyOnAssignment(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public String VALUE { get; }
  public void run() {
    String value;
    VaLuE = 'local';
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("local assignment was resolved as a get-only property: %#v", result.Diagnostics)
	}
}

func TestTypeContractAcceptsSafeNavigationEqualityRead(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
public class Probe {
  public Boolean label;
  public void run(Probe value) {
    if (value?.label == true) {}
  }
}`})
	if result.HasErrors() {
		t.Fatalf("safe-navigation equality read was rejected: %#v", result.Diagnostics)
	}
}
