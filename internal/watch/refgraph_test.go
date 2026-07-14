package watch

import (
	"path/filepath"
	"reflect"
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

func TestRefGraphRefreshUpdatesKnownModifiedFileInPlace(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Base.cls"), "public virtual class Base {}")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service {}")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			classSymbol(root, "Service", false, "Base"),
			classSymbol(root, "ServiceTest", true, ""),
		},
	}
	graph := BuildReferenceGraph(index)

	refreshed := graph.Refresh(index, []Change{classChange(root, "Service", ChangeModified)})

	if refreshed != graph {
		t.Fatal("Refresh replaced graph for a known modified file; want in-place incremental update")
	}
}

func TestRefGraphRefreshedReturnsOwnedCleanEquivalentGraph(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Base.cls"), "public virtual class Base {}")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service extends Base {}")
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			classSymbol(root, "Service", false, "Base"),
		},
	}
	graph := BuildReferenceGraph(index)
	before := BuildReferenceGraph(index)
	refreshed := graph.Refreshed(index, []Change{classChange(root, "Service", ChangeModified)})
	if refreshed == graph {
		t.Fatal("Refreshed returned the published graph")
	}
	if !reflect.DeepEqual(graph, before) {
		t.Fatal("Refreshed mutated the published graph")
	}
	if clean := BuildReferenceGraph(index); !reflect.DeepEqual(refreshed, clean) {
		t.Fatalf("owned refresh differs from clean build:\nrefreshed=%#v\nclean=%#v", refreshed, clean)
	}
}

func TestRefGraphRefreshedUpdatesEveryNestedTypeInModifiedFile(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "Outer.cls")
	writeWatchFile(t, shared, "public class Outer { class Inner {} }")
	writeWatchFile(t, filepath.Join(root, "BaseA.cls"), "public class BaseA {}")
	writeWatchFile(t, filepath.Join(root, "BaseB.cls"), "public class BaseB {}")
	outer := classSymbol(root, "Outer", false, "BaseA")
	inner := classSymbol(root, "Inner", false, "BaseA")
	outer.File, inner.File = shared, shared
	initial := typesys.Index{Project: typesys.ProjectInfo{Root: root}, Types: []typesys.TypeSymbol{
		classSymbol(root, "BaseA", false, ""),
		classSymbol(root, "BaseB", false, ""),
		outer,
		inner,
	}}
	graph := BuildReferenceGraph(initial)
	before := BuildReferenceGraph(initial)
	outer.SuperClass = "BaseB"
	inner.SuperClass = "BaseB"
	updated := initial
	updated.Types = append([]typesys.TypeSymbol(nil), initial.Types...)
	updated.Types[2], updated.Types[3] = outer, inner
	refreshed := graph.Refreshed(updated, []Change{{Path: shared, Op: ChangeModified, Kind: FileKindApexClass, Name: "Outer"}})
	if !reflect.DeepEqual(refreshed, BuildReferenceGraph(updated)) {
		t.Fatal("nested structural refresh differs from clean build")
	}
	if !reflect.DeepEqual(graph, before) {
		t.Fatal("nested structural refresh mutated the published graph")
	}
}

func TestRefGraphRefreshedRebuildsWhenNestedTypeSetChanges(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "Outer.cls")
	writeWatchFile(t, shared, "public class Outer { class Inner {} }")
	outer := classSymbol(root, "Outer", false, "")
	inner := classSymbol(root, "Inner", false, "")
	outer.File, inner.File = shared, shared
	initial := typesys.Index{Project: typesys.ProjectInfo{Root: root}, Types: []typesys.TypeSymbol{outer, inner}}
	for _, tc := range []struct {
		name  string
		types []typesys.TypeSymbol
	}{
		{name: "remove nested", types: []typesys.TypeSymbol{outer}},
		{name: "add nested", types: []typesys.TypeSymbol{outer, inner, func() typesys.TypeSymbol {
			extra := classSymbol(root, "Extra", false, "")
			extra.File = shared
			return extra
		}()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := BuildReferenceGraph(initial)
			before := BuildReferenceGraph(initial)
			updated := typesys.Index{Project: initial.Project, Types: tc.types}
			refreshed := graph.Refreshed(updated, []Change{{Path: shared, Op: ChangeModified, Kind: FileKindApexClass, Name: "Outer"}})
			if !reflect.DeepEqual(refreshed, BuildReferenceGraph(updated)) {
				t.Fatal("nested type-set refresh differs from clean build")
			}
			if !reflect.DeepEqual(graph, before) {
				t.Fatal("nested type-set refresh mutated the published graph")
			}
		})
	}
}

func TestCanRefreshAuthoritativeFallbackGraphIsWarningModifiedClassOnly(t *testing.T) {
	root := t.TempDir()
	service := filepath.Join(root, "Service.cls")
	modified := []Change{{Path: service, Op: ChangeModified, Kind: FileKindApexClass, Name: "Service"}}
	warning := typesys.Index{
		Diagnostics: []diagnostic.Diagnostic{{Severity: diagnostic.Warning, Message: "warning"}},
		Types:       []typesys.TypeSymbol{{Name: "Service", File: service}},
	}
	if !CanRefreshAuthoritativeFallbackGraph(warning, warning, modified) {
		t.Fatal("warning-bearing known modified class did not qualify for refresh")
	}
	errorIndex := warning
	errorIndex.Diagnostics = []diagnostic.Diagnostic{{Severity: diagnostic.Error, Message: "error"}}
	if CanRefreshAuthoritativeFallbackGraph(errorIndex, errorIndex, modified) {
		t.Fatal("error recovery qualified for refresh")
	}
	if CanRefreshAuthoritativeFallbackGraph(warning, warning, []Change{{Path: service, Op: ChangeDeleted, Kind: FileKindApexClass, Name: "Service"}}) {
		t.Fatal("delete fallback qualified for refresh")
	}
	duplicate := warning
	duplicate.Types = append(append([]typesys.TypeSymbol(nil), warning.Types...), typesys.TypeSymbol{
		Name: "Service",
		File: filepath.Join(root, "other", "..", "Duplicate.cls"),
	})
	if CanRefreshAuthoritativeFallbackGraph(duplicate, duplicate, modified) {
		t.Fatal("duplicate canonical name in another file qualified for refresh")
	}
	sameFileAlias := warning
	sameFileAlias.Types = append(append([]typesys.TypeSymbol(nil), warning.Types...), typesys.TypeSymbol{
		Name: "NestedService",
		File: filepath.Join(root, ".", "Service.cls"),
	})
	if !CanRefreshAuthoritativeFallbackGraph(sameFileAlias, sameFileAlias, modified) {
		t.Fatal("equivalent path for the edited file was treated as a duplicate file")
	}
	renamed := warning
	renamed.Types = []typesys.TypeSymbol{{Name: "RenamedService", File: service}}
	if CanRefreshAuthoritativeFallbackGraph(warning, renamed, modified) {
		t.Fatal("changed declaration-name multiplicity qualified for refresh")
	}
	updatedWithoutDigests := warning
	if delta, covered := AuthoritativeApexGraphChanges(warning, updatedWithoutDigests); covered || delta != nil {
		t.Fatalf("missing source digests = %#v, %t, want nil/false", delta, covered)
	}
}

func TestRefGraphRefreshRebuildsForNonApexConfigurationChange(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { System.assert(true); } }")
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types:   []typesys.TypeSymbol{classSymbol(root, "ServiceTest", true, "")},
	}
	graph := BuildReferenceGraph(index)
	refreshed := graph.Refresh(index, []Change{{
		Path: filepath.Join(root, "glade.yml"),
		Op:   ChangeModified,
		Kind: FileKindIgnored,
		Name: "glade.yml",
	}})
	if refreshed == graph {
		t.Fatal("configuration change reused graph; want authoritative rebuild")
	}
	selection := SelectAffectedTestsWithRefGraph(index, []Change{{Kind: FileKindIgnored, Name: "glade.yml"}}, refreshed)
	if selection.Mode != SelectionAll || len(selection.TestClasses) != 1 || selection.TestClasses[0] != "ServiceTest" {
		t.Fatalf("configuration selection = %#v, want all", selection)
	}
}

func TestRefGraphRefreshRemovesModifiedFileMissingFromIndex(t *testing.T) {
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "Base.cls"), "public virtual class Base {}")
	writeWatchFile(t, filepath.Join(root, "Service.cls"), "public class Service extends Base {}")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			classSymbol(root, "Service", false, "Base"),
			classSymbol(root, "ServiceTest", true, ""),
		},
	}
	graph := BuildReferenceGraph(index)

	refreshedIndex := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			classSymbol(root, "ServiceTest", true, ""),
		},
	}
	refreshed := graph.Refresh(refreshedIndex, []Change{classChange(root, "Service", ChangeModified)})

	if _, ok := refreshed.deps["Service"]; ok {
		t.Fatalf("deps still contain Service: %#v", refreshed.deps["Service"])
	}
	if _, ok := refreshed.dependents["Base"]["Service"]; ok {
		t.Fatalf("Base dependents still contain Service: %#v", refreshed.dependents["Base"])
	}
}

func TestRefGraphRefreshRebuildsWhenModifiedClassChangesCanonicalName(t *testing.T) {
	root := t.TempDir()
	serviceFile := filepath.Join(root, "Service.cls")
	writeWatchFile(t, filepath.Join(root, "Base.cls"), "public virtual class Base {}")
	writeWatchFile(t, serviceFile, "public class Service extends Base {}")
	writeWatchFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			classSymbol(root, "Service", false, "Base"),
			classSymbol(root, "ServiceTest", true, ""),
		},
	}
	graph := BuildReferenceGraph(index)

	writeWatchFile(t, serviceFile, "public class RenamedService extends Base {}")
	renamed := classSymbol(root, "RenamedService", false, "Base")
	renamed.File = serviceFile
	refreshedIndex := typesys.Index{
		Project: index.Project,
		Types: []typesys.TypeSymbol{
			classSymbol(root, "Base", false, ""),
			renamed,
			classSymbol(root, "ServiceTest", true, ""),
		},
	}

	refreshed := graph.Refresh(refreshedIndex, []Change{classChange(root, "Service", ChangeModified)})
	fresh := BuildReferenceGraph(refreshedIndex)
	if !reflect.DeepEqual(refreshed, fresh) {
		t.Fatalf("renamed-class graph differs from fresh build:\nrefreshed = %#v\nfresh = %#v", refreshed, fresh)
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

func TestAuthoritativeApexFilesIgnoreGeneratedNonApexTypeFiles(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Service.cls")
	flowPath := filepath.Join(root, "Generated.flow-meta.xml")
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "Service", File: classPath},
		{Kind: apexast.DeclarationClass, Name: "Flow.Interview.Generated", File: flowPath},
	}}

	files, ok := authoritativeApexFiles(index)
	if !ok {
		t.Fatal("authoritative Apex file inventory failed")
	}
	if len(files) != 1 || files[cleanPath(classPath)] != FileKindApexClass {
		t.Fatalf("authoritative Apex files = %#v, want only %s", files, classPath)
	}
	if !isApexSourceFile(classPath, ".cls") || isApexSourceFile(flowPath, ".cls") {
		t.Fatal("Apex source extension classification accepted a generated flow file")
	}
}
