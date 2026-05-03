package typesys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
)

func TestBuildIndex(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Hello.cls")
	triggerPath := filepath.Join(root, "HelloTrigger.trigger")
	writeFile(t, classPath, "public class Hello { public void run() {} }")
	writeFile(t, triggerPath, "trigger HelloTrigger on Account (before insert) {}")

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath, triggerPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	if len(idx.Types) != 1 || idx.Types[0].Name != "Hello" || len(idx.Types[0].Members) != 1 {
		t.Fatalf("types = %#v", idx.Types)
	}
	if len(idx.Triggers) != 1 || idx.Triggers[0].ObjectName != "Account" {
		t.Fatalf("triggers = %#v", idx.Triggers)
	}
}

func TestBuildIndexDuplicateType(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "one.cls")
	second := filepath.Join(root, "two.cls")
	writeFile(t, first, "public class Hello {}")
	writeFile(t, second, "public class hello {}")

	idx := Build(project.Project{Root: root, ApexFiles: []string{first, second}}, schema.Schema{})
	if !idx.HasErrors() {
		t.Fatalf("expected duplicate diagnostic: %#v", idx.Diagnostics)
	}
}

func TestBuildIndexDiscoversTests(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "HelloTest.cls")
	writeFile(t, classPath, "@IsTest private class HelloTest { @isTest private static void run() {} }")

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	if !idx.Types[0].IsTest || !idx.Types[0].Members[0].IsTest {
		t.Fatalf("test flags not set: %#v", idx.Types[0])
	}
}

func TestBuildIndexKeepsMethodParameters(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Hello.cls")
	writeFile(t, classPath, "public class Hello { public void run(String name, Account account) {} }")

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	params := idx.Types[0].Members[0].Parameters
	if len(params) != 2 || params[0].Name != "name" || params[1].Type != "Account" {
		t.Fatalf("parameters = %#v", params)
	}
}

func TestBuildIndexPromotesNestedTypes(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "Outer.cls")
	writeFile(t, classPath, `
public class Outer {
  public class Inner {
    public String label() { return 'inner'; }
  }
  public interface Marker {
    void mark();
  }
}
`)

	idx := Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	types := map[string]TypeSymbol{}
	for _, typ := range idx.Types {
		types[typ.Name] = typ
	}
	for _, name := range []string{"Outer", "Outer.Inner", "Outer.Marker"} {
		if _, ok := types[name]; !ok {
			t.Fatalf("missing nested type %s in %#v", name, idx.Types)
		}
	}
	if len(types["Outer.Inner"].Members) != 1 || types["Outer.Inner"].Members[0].Name != "label" {
		t.Fatalf("inner members = %#v", types["Outer.Inner"].Members)
	}
}

func TestUpdateApexFilesReplacesChangedSymbolsAndDropsDeleted(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.cls")
	trigger := filepath.Join(root, "SecondTrigger.trigger")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Second { public void oldName() {} }")
	writeFile(t, trigger, "trigger SecondTrigger on Account (before insert) {}")
	idx := Build(project.Project{Root: root, ApexFiles: []string{first, second, trigger}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}

	writeFile(t, second, "public class Second { public void newName() {} }")
	updated := UpdateApexFiles(idx, []string{second}, []string{trigger})
	if updated.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", updated.Diagnostics)
	}
	if len(updated.Triggers) != 0 {
		t.Fatalf("triggers = %#v", updated.Triggers)
	}
	types := map[string]TypeSymbol{}
	for _, typ := range updated.Types {
		types[typ.Name] = typ
	}
	if _, ok := types["First"]; !ok {
		t.Fatalf("missing retained type: %#v", updated.Types)
	}
	secondType := types["Second"]
	if len(secondType.Members) != 1 || secondType.Members[0].Name != "newName" {
		t.Fatalf("second type = %#v", secondType)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
