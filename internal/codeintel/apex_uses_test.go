package codeintel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestApexUsesResolveConstructorAndInstanceMethodCall(t *testing.T) {
	root := t.TempDir()
	consumer := writeApexUseFile(t, root, "Consumer.cls", `
public class Consumer {
  public void run(Account a) {
    new InvoiceService().total(a);
  }
}`)
	index := apexUseIndex(root, consumer, []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass, Name: "InvoiceService", File: filepath.Join(root, "InvoiceService.cls"),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationConstructor, Name: "InvoiceService", Range: testRange(1, 1, 1, 24)},
				{Kind: apexast.DeclarationMethod, Name: "total", Type: "Decimal", Parameters: []apexast.Parameter{{Name: "account", Type: "Account"}}, Range: testRange(2, 3, 2, 34)},
			},
		},
		{
			Kind: apexast.DeclarationClass, Name: "Consumer", File: consumer,
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationMethod, Name: "run", Type: "void", Parameters: []apexast.Parameter{{Name: "a", Type: "Account"}}, Range: testRange(3, 3, 5, 4)},
			},
		},
	})

	graph := codeintel.Build(index)

	assertResolvedUse(t, graph, codeintel.ApexMemberID("", "InvoiceService", "constructor", "InvoiceService", "()"), codeintel.UseConstruct, "InvoiceService")
	assertResolvedUse(t, graph, codeintel.ApexMemberID("", "InvoiceService", "method", "total", "Decimal(Account)"), codeintel.UseCall, "total")
}

func TestApexUsesResolveStaticMethodCall(t *testing.T) {
	root := t.TempDir()
	consumer := writeApexUseFile(t, root, "Consumer.cls", `
public class Consumer {
  public void run(Account a) {
    InvoiceService.staticTotal(a);
  }
}`)
	index := apexUseIndex(root, consumer, []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass, Name: "InvoiceService", File: filepath.Join(root, "InvoiceService.cls"),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationMethod, Name: "staticTotal", Type: "Decimal", Modifiers: []string{"static"}, Parameters: []apexast.Parameter{{Name: "account", Type: "Account"}}, Range: testRange(2, 3, 2, 41)},
			},
		},
		{Kind: apexast.DeclarationClass, Name: "Consumer", File: consumer},
	})

	graph := codeintel.Build(index)

	assertResolvedUse(t, graph, codeintel.ApexMemberID("", "InvoiceService", "method", "staticTotal", "Decimal(Account)"), codeintel.UseCall, "staticTotal")
}

func TestApexUsesResolveClassReferenceInLocalType(t *testing.T) {
	root := t.TempDir()
	consumer := writeApexUseFile(t, root, "Consumer.cls", `
public class Consumer {
  public void run() {
    InvoiceService svc = new InvoiceService();
  }
}`)
	index := apexUseIndex(root, consumer, []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "InvoiceService", File: filepath.Join(root, "InvoiceService.cls")},
		{Kind: apexast.DeclarationClass, Name: "Consumer", File: consumer},
	})

	graph := codeintel.Build(index)

	assertResolvedUse(t, graph, codeintel.ApexTypeID("", "InvoiceService"), codeintel.UseRead, "InvoiceService")
}

func TestApexUsesResolveExtendsAndImplements(t *testing.T) {
	root := t.TempDir()
	child := writeApexUseFile(t, root, "Child.cls", `public class Child extends Base implements Worker {}`)
	index := apexUseIndex(root, child, []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "Base", File: filepath.Join(root, "Base.cls")},
		{Kind: apexast.DeclarationInterface, Name: "Worker", File: filepath.Join(root, "Worker.cls")},
		{Kind: apexast.DeclarationClass, Name: "Child", File: child, SuperClass: "Base", Interfaces: []string{"Worker"}},
	})

	graph := codeintel.Build(index)

	assertResolvedUse(t, graph, codeintel.ApexTypeID("", "Base"), codeintel.UseExtends, "Base")
	assertResolvedUse(t, graph, codeintel.ApexTypeID("", "Worker"), codeintel.UseImplements, "Worker")
}

func TestApexUsesIgnoreCommentsAndStringLiterals(t *testing.T) {
	root := t.TempDir()
	consumer := writeApexUseFile(t, root, "Consumer.cls", `
public class Consumer {
  public void run() {
    // new InvoiceService().total();
    String s = 'InvoiceService.staticTotal()';
    String x = "InvoiceService";
  }
}`)
	index := apexUseIndex(root, consumer, []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass, Name: "InvoiceService", File: filepath.Join(root, "InvoiceService.cls"),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationConstructor, Name: "InvoiceService"},
				{Kind: apexast.DeclarationMethod, Name: "total", Type: "Decimal"},
				{Kind: apexast.DeclarationMethod, Name: "staticTotal", Type: "Decimal", Modifiers: []string{"static"}},
			},
		},
		{Kind: apexast.DeclarationClass, Name: "Consumer", File: consumer},
	})

	graph := codeintel.Build(index)

	for _, use := range graph.References(codeintel.ApexTypeID("", "InvoiceService"), false) {
		t.Fatalf("comment or string produced type use: %#v", use)
	}
	for _, id := range []codeintel.SymbolID{
		codeintel.ApexMemberID("", "InvoiceService", "constructor", "InvoiceService", "()"),
		codeintel.ApexMemberID("", "InvoiceService", "method", "total", "Decimal()"),
		codeintel.ApexMemberID("", "InvoiceService", "method", "staticTotal", "Decimal()"),
	} {
		if refs := graph.References(id, false); len(refs) != 0 {
			t.Fatalf("comment or string produced member use for %s: %#v", id, refs)
		}
	}
}

func TestApexUsesDuplicateMethodNamesDoNotCrossResolve(t *testing.T) {
	root := t.TempDir()
	consumer := writeApexUseFile(t, root, "Consumer.cls", `
public class Consumer {
  public void run() {
    BillingService billing = new BillingService();
    billing.total();
    unknown.total();
  }
}`)
	index := apexUseIndex(root, consumer, []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass, Name: "InvoiceService", File: filepath.Join(root, "InvoiceService.cls"),
			Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "total", Type: "Decimal", Range: testRange(2, 3, 2, 19)}},
		},
		{
			Kind: apexast.DeclarationClass, Name: "BillingService", File: filepath.Join(root, "BillingService.cls"),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationConstructor, Name: "BillingService"},
				{Kind: apexast.DeclarationMethod, Name: "total", Type: "Decimal", Range: testRange(2, 3, 2, 19)},
			},
		},
		{Kind: apexast.DeclarationClass, Name: "Consumer", File: consumer},
	})

	graph := codeintel.Build(index)

	assertResolvedUse(t, graph, codeintel.ApexMemberID("", "BillingService", "method", "total", "Decimal()"), codeintel.UseCall, "total")
	if refs := graph.References(codeintel.ApexMemberID("", "InvoiceService", "method", "total", "Decimal()"), false); len(refs) != 0 {
		t.Fatalf("InvoiceService.total was cross-resolved: %#v", refs)
	}
	assertUnresolvedUse(t, graph, codeintel.UseCall, "total")
}

func TestApexUsesDoNotResolveLocalsDeclaredInAnotherMethod(t *testing.T) {
	root := t.TempDir()
	consumer := writeApexUseFile(t, root, "Consumer.cls", `
public class Consumer {
  public void first() {
    InvoiceService svc = new InvoiceService();
  }
  public void second() {
    svc.total();
  }
}`)
	index := apexUseIndex(root, consumer, []typesys.TypeSymbol{
		{
			Kind: apexast.DeclarationClass, Name: "InvoiceService", File: filepath.Join(root, "InvoiceService.cls"),
			Members: []typesys.MemberSymbol{
				{Kind: apexast.DeclarationConstructor, Name: "InvoiceService"},
				{Kind: apexast.DeclarationMethod, Name: "total", Type: "Decimal", Range: testRange(2, 3, 2, 19)},
			},
		},
		{Kind: apexast.DeclarationClass, Name: "Consumer", File: consumer},
	})

	graph := codeintel.Build(index)

	if refs := graph.References(codeintel.ApexMemberID("", "InvoiceService", "method", "total", "Decimal()"), false); len(refs) != 0 {
		t.Fatalf("method-local receiver leaked across methods: %#v", refs)
	}
	assertUnresolvedUse(t, graph, codeintel.UseCall, "total")
}

func TestApexUsesResolveTriggerObjectReference(t *testing.T) {
	root := t.TempDir()
	trigger := writeApexUseFile(t, root, "InvoiceTrigger.trigger", `trigger InvoiceTrigger on Invoice__c (before insert) {}`)
	index := apexUseIndex(root, trigger, nil)
	index.Triggers = []typesys.TriggerSymbol{{Name: "InvoiceTrigger", ObjectName: "Invoice__c", File: trigger, Range: testRange(1, 1, 1, 54)}}

	graph := codeintel.Build(index)

	assertResolvedUse(t, graph, codeintel.SObjectID("Invoice__c"), codeintel.UseRead, "Invoice__c")
}

func apexUseIndex(root, sourceFile string, types []typesys.TypeSymbol) typesys.Index {
	return typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types:   types,
		Objects: nil,
	}
}

func writeApexUseFile(t *testing.T, root, name, source string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func assertResolvedUse(t *testing.T, graph codeintel.Graph, id codeintel.SymbolID, kind codeintel.UseKind, name string) {
	t.Helper()
	for _, use := range graph.References(id, false) {
		if use.Kind == kind && use.Name == name && use.Resolved {
			return
		}
	}
	t.Fatalf("missing resolved %s use %q for %s; uses: %#v", kind, name, id, graph.Uses)
}

func assertUnresolvedUse(t *testing.T, graph codeintel.Graph, kind codeintel.UseKind, name string) {
	t.Helper()
	for _, use := range graph.Uses {
		if use.Kind == kind && use.Name == name && !use.Resolved && use.SymbolID == "" {
			return
		}
	}
	t.Fatalf("missing unresolved %s use %q; uses: %#v", kind, name, graph.Uses)
}
