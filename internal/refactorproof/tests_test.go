package refactorproof

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSelectAffectedTestsUsesWatchRefGraph(t *testing.T) {
	root := t.TempDir()
	writeProofFile(t, filepath.Join(root, "Helper.cls"), "public class Helper { public static void go() {} }")
	writeProofFile(t, filepath.Join(root, "Service.cls"), "public class Service { void run() { Helper.go(); } }")
	writeProofFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service s = new Service(); } }")
	writeProofFile(t, filepath.Join(root, "OtherTest.cls"), "@IsTest class OtherTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			proofClassSymbol(root, "Helper", false),
			proofClassSymbol(root, "Service", false),
			proofClassSymbol(root, "ServiceTest", true),
			proofClassSymbol(root, "OtherTest", true),
		},
	}

	selection := SelectAffectedTests(index, []ChangedFile{{
		Path:      filepath.Join(root, "Helper.cls"),
		Kind:      "apex_class",
		Operation: "modified",
		Symbol:    "Helper",
	}})
	if selection.Mode != "direct" {
		t.Fatalf("mode = %q, want direct: %#v", selection.Mode, selection)
	}
	if len(selection.TestClasses) != 1 || selection.TestClasses[0] != "ServiceTest" {
		t.Fatalf("test classes = %#v, want [ServiceTest]", selection.TestClasses)
	}
}

func proofClassSymbol(root, name string, isTest bool) typesys.TypeSymbol {
	typ := typesys.TypeSymbol{
		Kind:   apexast.DeclarationClass,
		Name:   name,
		File:   filepath.Join(root, name+".cls"),
		IsTest: isTest,
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

func writeProofFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
