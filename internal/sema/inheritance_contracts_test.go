package sema

import "testing"

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
