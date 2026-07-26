package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzeDeclarationProject(t *testing.T, files map[string]string) Result {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, len(files))
	for name, contents := range files {
		path := filepath.Join(root, name)
		writeSemaFile(t, path, contents)
		paths = append(paths, path)
	}
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: paths}, schema.Schema{}))
}

func declarationDiagnosticMatching(result Result, substring string) bool {
	for _, diag := range result.Diagnostics {
		if strings.Contains(strings.ToLower(diag.Message), strings.ToLower(substring)) {
			return true
		}
	}
	return false
}

func TestDuplicateDeclarationSameOwnerClassAndInterface(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"ProbeDuplicateTypeName.cls": `
public class ProbeDuplicateTypeName {
  class Item {}
  interface Item {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected same-owner class/interface duplicate error, got %#v", result.Diagnostics)
	}
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADETYPE001" && diag.Severity == diagnostic.Error {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADETYPE001 error, got %#v", result.Diagnostics)
	}
}

func TestDuplicateDeclarationCrossFileWorkspaceAmbiguityRemainsWarning(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"One.cls": "public class Hello {}",
		"Two.cls": "public class hello {}",
	})
	if result.HasErrors() {
		t.Fatalf("cross-file workspace ambiguity should remain warning-only: %#v", result.Diagnostics)
	}
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADETYPE001" && diag.Severity == diagnostic.Warning {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADETYPE001 warning for cross-file ambiguity, got %#v", result.Diagnostics)
	}
}

func TestInnerTypeEqualToAncestor(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Holder.cls": `
public class Holder {
  class Mid {
    class Holder {}
  }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected inner type equal to ancestor error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "ancestor") && !declarationDiagnosticMatching(result, "already in use") {
		t.Fatalf("expected ancestor/type-name diagnostic, got %#v", result.Diagnostics)
	}
}

func TestOverrideMayUsePublicVisibilityForGlobalProjectMethod(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"GlobalBase.cls": `
global virtual class GlobalBase {
  global virtual String value() { return 'base'; }
}
`,
		"PublicChild.cls": `
public class PublicChild extends GlobalBase {
  public override String value() { return 'child'; }
}
`,
	})
	if declarationDiagnosticMatching(result, "cannot reduce inherited visibility") {
		t.Fatalf("public override of global method rejected: %#v", result.Diagnostics)
	}
}

func TestOverrideRejectsProtectedVisibilityForGlobalProjectMethod(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"GlobalBase.cls": `
global virtual class GlobalBase {
  global virtual String value() { return 'base'; }
}
`,
		"ProtectedChild.cls": `
public class ProtectedChild extends GlobalBase {
  protected override String value() { return 'child'; }
}
`,
	})
	if !declarationDiagnosticMatching(result, "cannot reduce inherited visibility") {
		t.Fatalf("protected override of global method accepted: %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberCaseInsensitiveFields(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupFields.cls": `
public class DupFields {
  public Integer value;
  public String Value;
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate field error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "value") {
		t.Fatalf("expected diagnostic naming the field, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberRemainsErrorWithSourceBackedDependency(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	for _, dir := range []string{depRoot, consumerRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	depFile := filepath.Join(depRoot, "DepHelper.cls")
	consumerFile := filepath.Join(consumerRoot, "DupFields.cls")
	writeSemaFile(t, depFile, `
global class DepHelper {
  global static String ok() { return 'ok'; }
}
`)
	writeSemaFile(t, consumerFile, `
public class DupFields {
  public Integer value;
  public String Value;
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "deppkg",
		ApexFiles: []string{depFile},
	}
	result := Analyze(typesys.Build(project.Project{
		Root:      consumerRoot,
		ApexFiles: []string{consumerFile},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "deppkg",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
	}, schema.Schema{}))
	if !result.HasErrors() {
		t.Fatalf("same-owner duplicate member must remain an error with source-backed deps: %#v", result.Diagnostics)
	}
	found := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA031" && diag.Severity == diagnostic.Error {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GLADESEMA031 error, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberCaseInsensitiveProperties(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupProps.cls": `
public class DupProps {
  public Integer value { get; set; }
  public String Value { get; set; }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate property error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "value") {
		t.Fatalf("expected diagnostic naming the property, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberConstructors(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupCtor.cls": `
public class DupCtor {
  public DupCtor(Integer value) {}
  public DupCtor(Integer other) {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate constructor error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "constructor") {
		t.Fatalf("expected constructor diagnostic, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberMethods(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupMethods.cls": `
public class DupMethods {
  public void run(Integer value) {}
  public void run(Integer other) {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected duplicate method error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "run") {
		t.Fatalf("expected diagnostic naming the method, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberMethodsDifferOnlyByReturnType(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupReturn.cls": `
public class DupReturn {
  public Integer run(Integer value) { return value; }
  public String run(Integer value) { return String.valueOf(value); }
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected return-type-only overload to be an error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "run") {
		t.Fatalf("expected diagnostic naming the method, got %#v", result.Diagnostics)
	}
}

func TestDuplicateMemberCaseOnlyMethodNames(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"DupCase.cls": `
public class DupCase {
  public void run() {}
  public void Run() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected case-only method name collision error, got %#v", result.Diagnostics)
	}
	if !declarationDiagnosticMatching(result, "run") {
		t.Fatalf("expected diagnostic naming the method, got %#v", result.Diagnostics)
	}
}

func TestDeclarationContractTopLevelVisibility(t *testing.T) {
	t.Parallel()
	privateResult := analyzeDeclarationProject(t, map[string]string{
		"Hidden.cls": `private class Hidden {}`,
	})
	if !privateResult.HasErrors() || !declarationDiagnosticMatching(privateResult, "public or global") {
		t.Fatalf("expected top-level visibility error, got %#v", privateResult.Diagnostics)
	}

	isTestResult := analyzeDeclarationProject(t, map[string]string{
		"HiddenTest.cls": `
@IsTest
private class HiddenTest {
  @IsTest static Integer run() { return 1; }
}

	`,
	})
	if declarationDiagnosticMatching(isTestResult, "public or global") {
		t.Fatalf("@IsTest private top-level must remain allowed: %#v", isTestResult.Diagnostics)
	}

	publicResult := analyzeDeclarationProject(t, map[string]string{
		"Visible.cls": `public class Visible {}`,
	})
	if publicResult.HasErrors() {
		t.Fatalf("public top-level should pass: %#v", publicResult.Diagnostics)
	}
}

func TestDeclarationContractRejectsOmittedTopLevelVisibility(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"class":     `class HiddenClass {}`,
		"interface": `interface HiddenInterface { void run(); }`,
	} {
		result := analyzeDeclarationProject(t, map[string]string{"Hidden.cls": source})
		if !result.HasErrors() || !declarationDiagnosticMatching(result, "public or global") {
			t.Fatalf("%s: expected top-level visibility error, got %#v", name, result.Diagnostics)
		}
	}
}

func TestDeclarationContractNestingDepth(t *testing.T) {
	t.Parallel()
	ok := analyzeDeclarationProject(t, map[string]string{
		"Outer.cls": `
public class Outer {
  class Inner {}
}
`,
	})
	if declarationDiagnosticMatching(ok, "nests deeper") {
		t.Fatalf("one inner level should be allowed: %#v", ok.Diagnostics)
	}

	deep := analyzeDeclarationProject(t, map[string]string{
		"Outer.cls": `
public class Outer {
  class Mid {
    class Deep {}
  }
}
`,
	})
	if !deep.HasErrors() || !declarationDiagnosticMatching(deep, "nests deeper") {
		t.Fatalf("expected deeper nesting error, got %#v", deep.Diagnostics)
	}
}

func TestDeclarationContractIllegalClassModifiers(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"StaticClass.cls":     `public static class StaticClass {}`,
		"FinalClass.cls":      `public final class FinalClass {}`,
		"AbstractVirtual.cls": `public abstract virtual class AbstractVirtual {}`,
	} {
		result := analyzeDeclarationProject(t, map[string]string{name: source})
		if !result.HasErrors() {
			t.Fatalf("%s: expected modifier error, got %#v", name, result.Diagnostics)
		}
	}
}

func TestDeclarationContractIllegalMethodModifierCombinations(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"BadMethods.cls": `
public abstract class BadMethods {
  public abstract virtual void both() {}
  public abstract override void absOverride() {}
  public abstract static void absStatic() {}
  public virtual static void virtStatic() {}
  public override static void overStatic() {}
}
`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected method modifier errors, got %#v", result.Diagnostics)
	}
	for _, needle := range []string{"abstract and virtual", "abstract and override", "abstract and static", "virtual and static", "override and static"} {
		if !declarationDiagnosticMatching(result, needle) {
			t.Fatalf("missing %q diagnostic in %#v", needle, result.Diagnostics)
		}
	}
}

func TestDeclarationContractRejectsAdditionalStaticAndAccessContracts(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"inner static method": {
			"Outer.cls": `public class Outer { public class Worker { public static void run() {} } }`,
		},
		"protected static method": {
			"Probe.cls": `public class Probe { protected static void run() {} }`,
		},
		"global member in public owner": {
			"Probe.cls": `public class Probe { global void run() {} }`,
		},
		"explicit final method": {
			"Probe.cls": `public class Probe { public final void run() {} }`,
		},
	} {
		result := analyzeDeclarationProject(t, files)
		if !result.HasErrors() {
			t.Fatalf("%s: expected declaration diagnostic, got %#v", name, result.Diagnostics)
		}
	}
}

func TestDeclarationContractRejectsStaticAndFinalFieldMisuse(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"final static reassignment":         `public class Probe { public static final Integer Value = 1; public static void run() { Value = 2; } }`,
		"static field through instance":     `public class Probe { public static Integer Value; public void run() { Probe item = new Probe(); item.Value = 1; } }`,
		"instance field from static method": `public class Probe { public Integer Value; public static Integer run() { return Value; } }`,
		"this from static method":           `public class Probe { public static Probe run() { return this; } }`,
	} {
		result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
		if !result.HasErrors() {
			t.Fatalf("%s: expected field access diagnostic, got %#v", name, result.Diagnostics)
		}
	}
}

func TestDeclarationContractInnerStaticInitializerAndSharing(t *testing.T) {
	t.Parallel()
	innerInit := analyzeDeclarationProject(t, map[string]string{
		"Outer.cls": `
public class Outer {
  class Inner {
    static { Integer x = 1; }
  }
}
`,
	})
	if !innerInit.HasErrors() || !declarationDiagnosticMatching(innerInit, "static initializer") {
		t.Fatalf("expected inner static initializer error, got %#v", innerInit.Diagnostics)
	}

	sharing := analyzeDeclarationProject(t, map[string]string{
		"Share.cls": `public with sharing without sharing class Share {}`,
	})
	if !sharing.HasErrors() || !declarationDiagnosticMatching(sharing, "sharing") {
		t.Fatalf("expected mutually exclusive sharing error, got %#v", sharing.Diagnostics)
	}
}

func TestDeclarationContractParameterLimit(t *testing.T) {
	t.Parallel()
	okParams := make([]string, 32)
	overParams := make([]string, 33)
	for i := range okParams {
		okParams[i] = fmt.Sprintf("Integer p%d", i)
	}
	for i := range overParams {
		overParams[i] = fmt.Sprintf("Integer p%d", i)
	}
	ok := analyzeDeclarationProject(t, map[string]string{
		"OkParams.cls": fmt.Sprintf(`
public class OkParams {
  public void run(%s) {}
  public OkParams(%s) {}
}
`, strings.Join(okParams, ", "), strings.Join(okParams, ", ")),
	})
	if declarationDiagnosticMatching(ok, "parameter limit") {
		t.Fatalf("32 parameters should be allowed: %#v", ok.Diagnostics)
	}

	over := analyzeDeclarationProject(t, map[string]string{
		"OverParams.cls": fmt.Sprintf(`
public class OverParams {
  public void run(%s) {}
  public OverParams(%s) {}
}
`, strings.Join(overParams, ", "), strings.Join(overParams, ", ")),
	})
	if !over.HasErrors() || !declarationDiagnosticMatching(over, "parameter limit") {
		t.Fatalf("expected parameter limit error, got %#v", over.Diagnostics)
	}
}

func TestDeclarationContractMethodBodyConsistency(t *testing.T) {
	t.Parallel()
	iface := analyzeDeclarationProject(t, map[string]string{
		"IFace.cls": `public interface IFace { void run() { System.debug('x'); } }`,
	})
	if !iface.HasErrors() || !declarationDiagnosticMatching(iface, "cannot have a body") {
		t.Fatalf("expected interface body error, got %#v", iface.Diagnostics)
	}

	absBody := analyzeDeclarationProject(t, map[string]string{
		"Abs.cls": `
public abstract class Abs {
  public abstract void run() { System.debug('x'); }
}
`,
	})
	if !absBody.HasErrors() || !declarationDiagnosticMatching(absBody, "abstract method") {
		t.Fatalf("expected abstract-with-body error, got %#v", absBody.Diagnostics)
	}

	missing := analyzeDeclarationProject(t, map[string]string{
		"Missing.cls": `
public class Missing {
  public void run();
}
`,
	})
	if !missing.HasErrors() || !declarationDiagnosticMatching(missing, "must have a body") {
		t.Fatalf("expected missing body error, got %#v", missing.Diagnostics)
	}
}

func TestDeclarationContractAccessorVisibilityAndDuplicates(t *testing.T) {
	t.Parallel()
	vis := analyzeDeclarationProject(t, map[string]string{
		"Wide.cls": `
public class Wide {
  private String Name { get; public set; }
}
`,
	})
	if !vis.HasErrors() || !declarationDiagnosticMatching(vis, "accessor visibility") {
		t.Fatalf("expected accessor visibility error, got %#v", vis.Diagnostics)
	}

	dup := analyzeDeclarationProject(t, map[string]string{
		"DupGet.cls": `
public class DupGet {
  public String Name { get; get; set; }
}
`,
	})
	if !dup.HasErrors() || !declarationDiagnosticMatching(dup, "duplicate getter") {
		t.Fatalf("expected duplicate getter error, got %#v", dup.Diagnostics)
	}
}

func TestDeclarationContractUserGenericClass(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Box.cls": `public class Box<T> { public T value; }`,
	})
	if !result.HasErrors() || !declarationDiagnosticMatching(result, "type parameters") {
		t.Fatalf("expected user generic error, got %#v", result.Diagnostics)
	}
}

func TestDeclarationContractConstructorChaining(t *testing.T) {
	t.Parallel()
	notFirst := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `public class Base { public Base(Integer value) {} }`,
		"Child.cls": `
public class Child extends Base {
  public Child() {
    Integer x = 1;
    super(x);
  }
}
`,
	})
	if !notFirst.HasErrors() || !declarationDiagnosticMatching(notFirst, "first statement") {
		t.Fatalf("expected chain-must-be-first error, got %#v", notFirst.Diagnostics)
	}

	twoChains := analyzeDeclarationProject(t, map[string]string{
		"Plain.cls": `
public class Plain {
  public Plain() { this(1); }
  public Plain(Integer value) {
    this();
    super();
  }
}
`,
	})
	if !twoChains.HasErrors() || !declarationDiagnosticMatching(twoChains, "at most one") {
		t.Fatalf("expected at-most-one chain error, got %#v", twoChains.Diagnostics)
	}

	implicit := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `public class Base { public Base(Integer value) {} }`,
		"Child.cls": `
public class Child extends Base {
  public Child() {}
}
`,
	})
	if !implicit.HasErrors() || !declarationDiagnosticMatching(implicit, "implicit super()") {
		t.Fatalf("expected implicit super() error, got %#v", implicit.Diagnostics)
	}

	ok := analyzeDeclarationProject(t, map[string]string{
		"Base.cls": `public class Base { public Base() {} public Base(Integer value) {} }`,
		"Child.cls": `
public class Child extends Base {
  public Child() { super(1); }
  public Child(String value) { this(); }
}
`,
	})
	if declarationDiagnosticMatching(ok, "first statement") || declarationDiagnosticMatching(ok, "implicit super()") {
		t.Fatalf("valid chaining rejected: %#v", ok.Diagnostics)
	}
}

func TestDeclarationContractAmbiguousNullOverload(t *testing.T) {
	t.Parallel()
	ambiguous := analyzeDeclarationProject(t, map[string]string{
		"Over.cls": `
public class Over {
  public void run(String value) {}
  public void run(Integer value) {}
  public void call() { run(null); }
}
`,
	})
	if !ambiguous.HasErrors() || !declarationDiagnosticMatching(ambiguous, "ambiguous") {
		t.Fatalf("expected ambiguous null overload error, got %#v", ambiguous.Diagnostics)
	}

	specific := analyzeDeclarationProject(t, map[string]string{
		"Over.cls": `
public class Over {
  public void run(Object value) {}
  public void run(String value) {}
  public void call() { run(null); }
}
`,
	})
	if declarationDiagnosticMatching(specific, "ambiguous") {
		t.Fatalf("most-specific overload should win for null: %#v", specific.Diagnostics)
	}
}
