package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestInheritanceContractsRejectInvalidInheritanceTargets(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"class implements class": {
			"Concrete.cls": `public class Concrete {}`,
			"Probe.cls":    `public class Probe implements Concrete {}`,
		},
		"class extends interface": {
			"Contract.cls": `public interface Contract {}`,
			"Probe.cls":    `public class Probe extends Contract {}`,
		},
		"interface extends class": {
			"Base.cls":  `public virtual class Base {}`,
			"Probe.cls": `public interface Probe extends Base {}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, files)
			if !result.HasErrors() {
				t.Fatalf("expected inheritance-kind diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestInheritanceContractsRejectExtendingNonVirtualSuperclass(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls":  `public class Base {}`,
		"Child.cls": `public class Child extends Base {}`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected non-virtual superclass diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRequireOverrideModifier(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  public virtual void run() {}
}
`,
		"Child.cls": `
public class Child extends Base {
  public void run() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected missing override diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsAllowStaticMethodToShareInheritedInstanceSignature(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  public virtual String describe(String value) { return value; }
}
`,
		"Child.cls": `
public class Child extends Base {
  private static String describe(String value) { return value; }
}
`,
	})
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" && strings.Contains(diag.Message, "describe") {
			t.Fatalf("static child method was treated as an inherited instance override: %#v", result.Diagnostics)
		}
	}
}

func TestInheritanceContractsRespectCrossNamespaceMethodVisibility(t *testing.T) {
	result := analyzeCrossNamespaceInheritanceFixture(t, `
public class MockProduct extends dep.ProductBase {
  public void setVariants(List<String> values) {}
  public override void setGlobalVariants(List<String> values) {}
}
`)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" && (strings.Contains(diag.Message, "setVariants") || strings.Contains(diag.Message, "setGlobalVariants")) {
			t.Fatalf("cross-namespace method visibility produced an override diagnostic: %#v", result.Diagnostics)
		}
	}

	missingOverride := analyzeCrossNamespaceInheritanceFixture(t, `
public class MockProduct extends dep.ProductBase {
  public void setGlobalVariants(List<String> values) {}
}
`)
	if !hasDiagnosticCode(missingOverride.Diagnostics, "GLADESEMA016") {
		t.Fatalf("cross-namespace global method did not require override: %#v", missingOverride.Diagnostics)
	}
}

func TestInheritanceContractsAllowTestVisiblePrivateOverrideInTestSubclass(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  @TestVisible
  private virtual void run() {}
}
`,
		"BaseTest.cls": `
@IsTest
private class BaseTest {
  private class Child extends Base {
    private override void run() {}
  }
}
`,
	})
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" && strings.Contains(diag.Message, "run") {
			t.Fatalf("@TestVisible private virtual method was not visible to a test subclass: %#v", result.Diagnostics)
		}
	}
}

func TestInheritanceContractsRejectCrossNamespaceTestVisiblePrivateOverride(t *testing.T) {
	result := analyzeCrossNamespaceInheritanceFixture(t, `
@IsTest
private class MockProduct extends dep.ProductBase {
  private override void setTestOnlyVariants(List<String> values) {}
}
`)
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA016") {
		t.Fatalf("cross-namespace @TestVisible private method was treated as inherited: %#v", result.Diagnostics)
	}
}

func analyzeCrossNamespaceInheritanceFixture(t *testing.T, childSource string) Result {
	t.Helper()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dependency")
	consumerRoot := filepath.Join(root, "consumer")
	if err := os.MkdirAll(depRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(consumerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	basePath := filepath.Join(depRoot, "ProductBase.cls")
	childPath := filepath.Join(consumerRoot, "MockProduct.cls")
	writeSemaFile(t, basePath, `
global virtual class ProductBase {
  public virtual void setVariants(List<String> values) {}
  global virtual void setGlobalVariants(List<String> values) {}
  @TestVisible private virtual void setTestOnlyVariants(List<String> values) {}
}
`)
	writeSemaFile(t, childPath, childSource)
	dependency := project.Project{
		Root:      depRoot,
		Namespace: "dep",
		ApexFiles: []string{basePath},
	}
	index := typesys.Build(project.Project{
		Root:      consumerRoot,
		Namespace: "consumer",
		ApexFiles: []string{childPath},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "dep",
			SourceRoot: depRoot,
			Project:    &dependency,
			Status:     "loaded",
		}},
	}, schema.Schema{})
	return Analyze(index)
}

func TestInheritanceContractsAllowObjectEqualityMethodsWithoutOverride(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ValueObject.cls": `
public class ValueObject {
  public Integer hashCode() { return 1; }
  public Boolean equals(Object other) { return other != null; }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("Object equality methods without override were rejected: %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRequireOverrideForNonObjectHashCodeSignature(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  public virtual String hashCode() { return 'base'; }
}
`,
		"Child.cls": `
public class Child extends Base {
  public String hashCode() { return 'child'; }
}
`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA016") {
		t.Fatalf("non-Object hashCode signature bypassed override enforcement: %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRequireOverrideForUserDeclaredObjectHashCodeSignature(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  public virtual Integer hashCode() { return 1; }
}
`,
		"Child.cls": `
public class Child extends Base {
  public Integer hashCode() { return 2; }
}
`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA016") {
		t.Fatalf("user-declared Object-shaped method bypassed override enforcement: %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsAllowAbstractImplementationWithoutOverrideAtAPIVersions64And65(t *testing.T) {
	files := map[string]string{
		"Base.cls": `
public abstract class Base {
  public abstract void run();
}
`,
		"Child.cls": `
public class Child extends Base {
  public void run() {}
}
`,
	}
	for _, apiVersion := range []string{"64.0", "65.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, files, apiVersion)
			if result.HasErrors() {
				t.Fatalf("API %s abstract implementation without override was rejected: %#v", apiVersion, result.Diagnostics)
			}
		})
	}
}

func TestInheritanceContractsGateAbstractAndOverrideAccessModifiersAtAPIVersion65(t *testing.T) {
	for name, source := range map[string]string{
		"abstract": `public abstract class Probe { abstract void run(); }`,
		"override": `
public virtual class Base { public virtual void run() {} }
public class Probe extends Base { override void run() {} }
`,
	} {
		for _, test := range []struct {
			apiVersion string
			wantReject bool
		}{
			{apiVersion: "64.0", wantReject: false},
			{apiVersion: "65.0", wantReject: true},
		} {
			t.Run(name+"_"+test.apiVersion, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
				gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032")
				if gotReject != test.wantReject {
					t.Fatalf("API %s %s access rejection = %v, want %v: %#v", test.apiVersion, name, gotReject, test.wantReject, result.Diagnostics)
				}
			})
		}
	}

	for name, source := range map[string]string{
		"abstract": `public abstract class Probe { public abstract void run(); }`,
		"override": `
public virtual class Base { public virtual void run() {} }
public class Probe extends Base { public override void run() {} }
`,
	} {
		t.Run(name+"_positive_access", func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "65.0")
			if result.HasErrors() {
				t.Fatalf("API 65 %s declaration with explicit access was rejected: %#v", name, result.Diagnostics)
			}
		})
	}
}

func TestInheritanceContractsRejectNonVirtualMethodOverride(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  public void run() {}
}
`,
		"Child.cls": `
public class Child extends Base {
  public override void run() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected virtuality diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRejectVisibilityNarrowing(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `
public virtual class Base {
  public virtual void run() {}
}
`,
		"Child.cls": `
public class Child extends Base {
  private override void run() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected visibility-narrowing diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRequireOverrideForAbstractImplementation(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"Base.cls": `
public abstract class Base {
  public abstract String value();
}
`,
		"Child.cls": `
public class Child extends Base {
  public String value() { return 'x'; }
}
`,
	}
	for _, apiVersion := range []string{"66.0", "67.0"} {
		t.Run("reject_"+apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, files, apiVersion)
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA016") {
				t.Fatalf("API %s abstract implementation without override was NOT rejected: %#v", apiVersion, result.Diagnostics)
			}
		})
	}
	t.Run("accept with override", func(t *testing.T) {
		filesWithOverride := map[string]string{
			"Base.cls": `
public abstract class Base {
  public abstract String value();
}
`,
			"Child.cls": `
public class Child extends Base {
  public override String value() { return 'x'; }
}
`,
		}
		result := analyzeDeclarationProjectWithAPIVersion(t, filesWithOverride, "66.0")
		if result.HasErrors() {
			t.Fatalf("abstract implementation with override was rejected: %#v", result.Diagnostics)
		}
	})
}

func TestInheritanceContractsRequireVisibleInterfaceImplementation(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe implements Schedulable {
  void execute(SchedulableContext context) {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected interface implementation visibility diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsSubstituteIteratorTypeArguments(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe implements Iterator<String> {
  public Boolean hasNext() { return false; }
  public Integer next() { return 1; }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected Iterator<String> return-type diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsSubstituteIterableTypeArguments(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe implements Iterable<String> {
  public Iterator<Integer> iterator() { return null; }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected Iterable<String> return-type diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsAllowStaticPublicInterfaceImplementation(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe implements Schedulable {
  public static void execute(SchedulableContext context) {}
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("unexpected static-interface diagnostic: %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRejectRawDatabaseBatchable(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe implements Database.Batchable {
  public Database.QueryLocator start(Database.BatchableContext context) { return null; }
  public void execute(Database.BatchableContext context, List<SObject> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected raw Database.Batchable diagnostic, got %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsRequireVisibleBatchableMethods(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe implements Database.Batchable<SObject> { Database.QueryLocator start(Database.BatchableContext context) { return null; } void execute(Database.BatchableContext context, List<SObject> scope) {} void finish(Database.BatchableContext context) {} }`,
	})
	if !result.HasErrors() {
		t.Fatalf("package-interface methods without public/global access were accepted: %#v", result.Diagnostics)
	}
}

func TestInheritanceContractsAllowBatchableScopeVariance(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"sobject scope": `
public class Probe implements Database.Batchable<Account> {
  public Database.QueryLocator start(Database.BatchableContext context) { return null; }
  public void execute(Database.BatchableContext context, List<SObject> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`,
		"concrete scope": `
public class Probe implements Database.Batchable<Account> {
  public Database.QueryLocator start(Database.BatchableContext context) { return null; }
  public void execute(Database.BatchableContext context, List<Account> scope) {}
  public void finish(Database.BatchableContext context) {}
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if result.HasErrors() {
				t.Fatalf("unexpected Batchable scope diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

func TestInheritanceContractsRequireTransitiveInterfaceMethods(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Parent.cls": `public interface Parent { void parentMethod(); }`,
		"Child.cls":  `public interface Child extends Parent { void childMethod(); }`,
		"Probe.cls": `
public class Probe implements Child {
  public void childMethod() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected transitive interface diagnostic, got %#v", result.Diagnostics)
	}
}
