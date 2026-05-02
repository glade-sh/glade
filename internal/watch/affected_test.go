package watch

import (
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/typesys"
)

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
