package watch

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestApplyTestSelection(t *testing.T) {
	base := apextest.Options{
		Filter:              "method filter",
		SelectedClasses:     []string{" AlphaTest ", "UnrelatedTest", "betatest", "ALPHATEST"},
		SelectedMethod:      "passes",
		TraceBlocked:        true,
		TraceAll:            true,
		SlowTestThresholdMS: 17,
		TimeoutMS:           2000,
		Parallelism:         3,
		ParallelMethods:     true,
		NoDiskCache:         true,
		ClassDurationMS:     map[string]int64{"AlphaTest": 10},
		MethodDurationMS:    map[string]int64{"AlphaTest.passes": 4},
		PerfCounters:        true,
	}
	t.Run("direct stable intersection preserves other options", func(t *testing.T) {
		got, ok := ApplyTestSelection(base, TestSelection{
			Mode:        SelectionDirect,
			TestClasses: []string{"betatest", "AlphaTest", "BetaTest", " "},
		})
		if !ok {
			t.Fatal("direct intersection reported no run")
		}
		want := base
		want.SelectedClasses = []string{"AlphaTest", "betatest"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("options = %#v, want %#v", got, want)
		}
	})
	t.Run("direct stable affected classes", func(t *testing.T) {
		opts := base
		opts.SelectedClasses = nil
		got, ok := ApplyTestSelection(opts, TestSelection{
			Mode:        SelectionDirect,
			TestClasses: []string{" BetaTest ", "AlphaTest", "betatest", ""},
		})
		if !ok {
			t.Fatal("direct selection reported no run")
		}
		want := opts
		want.SelectedClasses = []string{"BetaTest", "AlphaTest"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("options = %#v, want %#v", got, want)
		}
	})
	t.Run("empty intersection does not run", func(t *testing.T) {
		if _, ok := ApplyTestSelection(base, TestSelection{Mode: SelectionDirect, TestClasses: []string{"OtherAffectedTest"}}); ok {
			t.Fatal("empty direct intersection reported a run")
		}
	})
	t.Run("none does not run", func(t *testing.T) {
		if _, ok := ApplyTestSelection(base, TestSelection{Mode: SelectionNone}); ok {
			t.Fatal("none selection reported a run")
		}
	})
	t.Run("all preserves options", func(t *testing.T) {
		got, ok := ApplyTestSelection(base, TestSelection{Mode: SelectionAll})
		if !ok || !reflect.DeepEqual(got, base) {
			t.Errorf("all options = %#v/%t, want %#v/true", got, ok, base)
		}
	})
}

func TestSelectAffectedTestsDirectTestClass(t *testing.T) {
	root := t.TempDir()
	testPath := filepath.Join(root, "InvoiceServiceTest.cls")
	index := testIndex(root)

	selection := SelectAffectedTests(index, []Change{{
		Path: testPath,
		Op:   ChangeModified,
		Kind: FileKindApexClass,
		Name: "InvoiceServiceTest",
	}})

	if selection.Mode != SelectionDirect || len(selection.TestClasses) != 1 || selection.TestClasses[0] != "InvoiceServiceTest" {
		t.Fatalf("selection = %#v", selection)
	}
}

func TestSelectAffectedTestsProductionClassFallsBackToAllTests(t *testing.T) {
	root := t.TempDir()
	index := testIndex(root)

	selection := SelectAffectedTests(index, []Change{{
		Path: filepath.Join(root, "InvoiceService.cls"),
		Op:   ChangeModified,
		Kind: FileKindApexClass,
		Name: "InvoiceService",
	}})

	if selection.Mode != SelectionAll {
		t.Fatalf("selection = %#v", selection)
	}
	if got, want := selection.TestClasses, []string{"InvoiceServiceTest", "OtherTest"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("test classes = %#v", got)
	}
}

func TestSelectAffectedTestsFallsBackWhenCodeintelCannotResolveChangedFile(t *testing.T) {
	root := t.TempDir()
	index := testIndex(root)
	graph := BuildReferenceGraph(index)
	delete(graph.deps, "InvoiceService")
	delete(graph.dependents, "InvoiceService")
	delete(graph.nameByLower, "invoiceservice")
	delete(graph.resolvedFile, cleanPath(filepath.Join(root, "InvoiceService.cls")))

	selection := SelectAffectedTestsWithRefGraph(index, []Change{{
		Path: filepath.Join(root, "InvoiceService.cls"),
		Op:   ChangeModified,
		Kind: FileKindApexClass,
		Name: "InvoiceService",
	}}, graph)

	if selection.Mode != SelectionAll {
		t.Fatalf("selection = %#v, want all", selection)
	}
}

func TestSelectAffectedTestsUsesDependencyGraph(t *testing.T) {
	root := t.TempDir()
	index := testIndex(root)
	writeWatchFile(t, filepath.Join(root, "InvoiceServiceTest.cls"), `
@IsTest private class InvoiceServiceTest {
  @isTest static void coversService() {
    InvoiceService svc = new InvoiceService();
  }
}
`)
	writeWatchFile(t, filepath.Join(root, "OtherTest.cls"), `
@IsTest private class OtherTest {
  @isTest static void unrelated() {
    System.assert(true);
  }
}
`)

	selection := SelectAffectedTests(index, []Change{{
		Path: filepath.Join(root, "InvoiceService.cls"),
		Op:   ChangeModified,
		Kind: FileKindApexClass,
		Name: "InvoiceService",
	}})

	if selection.Mode != SelectionDirect || len(selection.TestClasses) != 1 || selection.TestClasses[0] != "InvoiceServiceTest" {
		t.Fatalf("selection = %#v", selection)
	}
}

func TestSelectAffectedTestsTriggerAndSchemaFallBackToAllTests(t *testing.T) {
	root := t.TempDir()
	index := testIndex(root)
	changes := []Change{
		{
			Path: filepath.Join(root, "InvoiceTrigger.trigger"),
			Op:   ChangeModified,
			Kind: FileKindApexTrigger,
			Name: "InvoiceTrigger",
		},
		{
			Path:       filepath.Join(root, "objects", "Invoice__c", "Invoice__c.object-meta.xml"),
			Op:         ChangeModified,
			Kind:       FileKindObjectMeta,
			Name:       "Invoice__c",
			ObjectName: "Invoice__c",
		},
		{
			Path:       filepath.Join(root, "objects", "Invoice__c", "fields", "Amount__c.field-meta.xml"),
			Op:         ChangeModified,
			Kind:       FileKindFieldMeta,
			Name:       "Amount__c",
			ObjectName: "Invoice__c",
		},
	}

	for _, change := range changes {
		selection := SelectAffectedTests(index, []Change{change})
		if selection.Mode != SelectionAll || len(selection.TestClasses) != 2 {
			t.Fatalf("selection for %#v = %#v", change, selection)
		}
	}
}

func TestSelectAffectedTestsUnknownChangeFallsBackToAllTests(t *testing.T) {
	root := t.TempDir()
	index := testIndex(root)
	selection := SelectAffectedTests(index, []Change{{
		Path: filepath.Join(root, "glade.yml"),
		Op:   ChangeModified,
		Kind: FileKindIgnored,
		Name: "glade.yml",
	}})
	if selection.Mode != SelectionAll || !reflect.DeepEqual(selection.TestClasses, []string{"InvoiceServiceTest", "OtherTest"}) {
		t.Fatalf("unknown/config selection = %#v, want all tests", selection)
	}
}

func TestSelectAffectedTestsNoChangesOrTests(t *testing.T) {
	if selection := SelectAffectedTests(typesys.Index{}, nil); selection.Mode != SelectionNone {
		t.Fatalf("empty selection = %#v", selection)
	}
	selection := SelectAffectedTests(typesys.Index{}, []Change{{Kind: FileKindApexClass, Name: "Missing"}})
	if selection.Mode != SelectionNone {
		t.Fatalf("selection with no tests = %#v", selection)
	}
}

func testIndex(root string) typesys.Index {
	return typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "InvoiceService",
				File: filepath.Join(root, "InvoiceService.cls"),
			},
			{
				Kind:   apexast.DeclarationClass,
				Name:   "InvoiceServiceTest",
				File:   filepath.Join(root, "InvoiceServiceTest.cls"),
				IsTest: true,
				Members: []typesys.MemberSymbol{{
					Kind:   apexast.DeclarationMethod,
					Name:   "runs",
					IsTest: true,
					Range:  diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}},
				}},
			},
			{
				Kind: apexast.DeclarationClass,
				Name: "OtherTest",
				File: filepath.Join(root, "OtherTest.cls"),
				Members: []typesys.MemberSymbol{{
					Kind:   apexast.DeclarationMethod,
					Name:   "runs",
					IsTest: true,
					Range:  diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}},
				}},
			},
		},
	}
}
