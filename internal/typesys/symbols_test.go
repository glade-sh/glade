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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
