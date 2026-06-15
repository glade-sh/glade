package watch

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func classSymbol(root, name string, isTest bool, super string) typesys.TypeSymbol {
	typ := typesys.TypeSymbol{
		Kind:       apexast.DeclarationClass,
		Name:       name,
		File:       filepath.Join(root, name+".cls"),
		IsTest:     isTest,
		SuperClass: super,
	}
	if isTest {
		typ.Members = []typesys.MemberSymbol{{
			Kind:   apexast.DeclarationMethod,
			Name:   "runs",
			IsTest: true,
			Range:  diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}},
		}}
	}
	return typ
}

func classChange(root, name string, op ChangeOp) Change {
	return Change{
		Path: filepath.Join(root, name+".cls"),
		Op:   op,
		Kind: FileKindApexClass,
		Name: name,
	}
}

// A change to a deep helper that no test names directly must still select the
// test that reaches it transitively, instead of falling back to the full suite.
func TestRefGraphSelectsTransitiveProductionDependency(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Helper.cls"), "public class Helper { public static void go() {} }")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service { void run() { Helper.go(); } }")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	writeWatchFile(t, filepath.Join(root, "OtherTest.cls"), "@IsTest class OtherTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Helper", false, ""),
			classSymbol(root, "Service", false, ""),
			classSymbol(root, "ServiceTest", true, ""),
			classSymbol(root, "OtherTest", true, ""),
		},
	}

	selection := SelectAffectedTests(index, []Change{classChange(root, "Helper", ChangeModified)})
	if selection.Mode != SelectionDirect {
		t.Fatalf("mode = %q, want direct (selection=%#v)", selection.Mode, selection)
	}
	if len(selection.TestClasses) != 1 || selection.TestClasses[0] != "ServiceTest" {
		t.Fatalf("test classes = %#v, want [ServiceTest]", selection.TestClasses)
	}
}

// A change to a superclass must select the tests of its subclasses.
func TestRefGraphFollowsSuperclassEdges(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Base.cls"), "public virtual class Base {}")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service extends Base {}")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	writeWatchFile(t, filepath.Join(root, "OtherTest.cls"), "@IsTest class OtherTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			classSymbol(root, "Service", false, "Base"),
			classSymbol(root, "ServiceTest", true, ""),
			classSymbol(root, "OtherTest", true, ""),
		},
	}

	selection := SelectAffectedTests(index, []Change{classChange(root, "Base", ChangeModified)})
	if selection.Mode != SelectionDirect || len(selection.TestClasses) != 1 || selection.TestClasses[0] != "ServiceTest" {
		t.Fatalf("selection = %#v, want direct [ServiceTest]", selection)
	}
}

// A changed production class that no test reaches must conservatively run the
// whole suite rather than silently skip a test that depends on it dynamically.
func TestRefGraphUnreachableProductionFallsBackToAll(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Orphan.cls"), "public class Orphan {}")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Orphan", false, ""),
			classSymbol(root, "ServiceTest", true, ""),
		},
	}

	selection := SelectAffectedTests(index, []Change{classChange(root, "Orphan", ChangeModified)})
	if selection.Mode != SelectionAll {
		t.Fatalf("selection = %#v, want all", selection)
	}
}

// Refresh must pick up a reference newly added to an existing file so a later
// change to the referenced type selects the right test.
func TestRefGraphRefreshPicksUpNewEdge(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Helper.cls"), "public class Helper { public static void go() {} }")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service { void run() {} }")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	writeWatchFile(t, filepath.Join(root, "OtherTest.cls"), "@IsTest class OtherTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Helper", false, ""),
			classSymbol(root, "Service", false, ""),
			classSymbol(root, "ServiceTest", true, ""),
			classSymbol(root, "OtherTest", true, ""),
		},
	}

	graph := BuildReferenceGraph(index)
	// Before: Service does not reference Helper, so a Helper change reaches no test.
	if sel := SelectAffectedTestsWithRefGraph(index, []Change{classChange(root, "Helper", ChangeModified)}, graph); sel.Mode != SelectionAll {
		t.Fatalf("pre-edit selection = %#v, want all", sel)
	}

	// Add the reference and refresh only the changed file.
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service { void run() { Helper.go(); } }")
	graph = graph.Refresh(index, []Change{classChange(root, "Service", ChangeModified)})

	sel := SelectAffectedTestsWithRefGraph(index, []Change{classChange(root, "Helper", ChangeModified)}, graph)
	if sel.Mode != SelectionDirect || len(sel.TestClasses) != 1 || sel.TestClasses[0] != "ServiceTest" {
		t.Fatalf("post-refresh selection = %#v, want direct [ServiceTest]", sel)
	}
}

func TestRefGraphUsesCodeintelMemberReferencesNotCommentWords(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Helper.cls"), "public class Helper { public void go() {} }")
	writeWatchFile(t, filepath.Join(root, "GhostHelper.cls"), "public class GhostHelper { public static void go() {} }")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), `
public class Service {
  void run() {
    new Helper().go();
    // GhostHelper.go();
  }
}`)
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	writeWatchFile(t, filepath.Join(root, "OtherTest.cls"), "@IsTest class OtherTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass, Name: "Helper", File: filepath.Join(root, "Helper.cls"),
				Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "go", Type: "void"}},
			},
			{
				Kind: apexast.DeclarationClass, Name: "GhostHelper", File: filepath.Join(root, "GhostHelper.cls"),
				Members: []typesys.MemberSymbol{{Kind: apexast.DeclarationMethod, Name: "go", Type: "void", Modifiers: []string{"static"}}},
			},
			classSymbol(root, "Service", false, ""),
			classSymbol(root, "ServiceTest", true, ""),
			classSymbol(root, "OtherTest", true, ""),
		},
	}

	graph := BuildReferenceGraph(index)
	sel := SelectAffectedTestsWithRefGraph(index, []Change{classChange(root, "Helper", ChangeModified)}, graph)
	if sel.Mode != SelectionDirect || len(sel.TestClasses) != 1 || sel.TestClasses[0] != "ServiceTest" {
		t.Fatalf("Helper selection = %#v, want direct [ServiceTest]", sel)
	}
	if sel := SelectAffectedTestsWithRefGraph(index, []Change{classChange(root, "GhostHelper", ChangeModified)}, graph); sel.Mode != SelectionAll {
		t.Fatalf("GhostHelper selection = %#v, want all", sel)
	}
}
