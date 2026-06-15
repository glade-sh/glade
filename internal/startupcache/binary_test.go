package startupcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestGobRoundTrip(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		BuiltAt:     "2026-06-09T00:00:00Z",
		Manifest: Manifest{
			ProjectRoot: dir,
			Files:       []File{{Path: "classes/Foo.cls", Size: 12, ModTime: 1}},
		},
		Org: storage.OrgState{
			APIVersion: "64.0",
			Objects: map[string]storage.ObjectState{
				"Account": {
					Definition: storage.ObjectDefinition{
						APIName: "Account",
						Fields: map[string]storage.Field{
							"Name": {APIName: "Name", Type: storage.FieldString},
						},
					},
				},
			},
		},
		Runtime: CompiledRuntime{
			Methods: map[string]vm.Method{
				"Foo.bar": {Name: "bar", ClassName: "Foo"},
			},
		},
	}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	gobPath := filepath.Join(dir, ".glade", "test", stateGobFile)
	if _, err := os.Stat(gobPath); err != nil {
		t.Fatalf("gob file missing: %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got == nil {
		t.Fatal("Read() = nil")
	}
	if got.Version != entry.Version {
		t.Fatalf("version = %d, want %d", got.Version, entry.Version)
	}
	if got.Runtime.Methods["Foo.bar"].Name != "bar" {
		t.Fatalf("method name = %q", got.Runtime.Methods["Foo.bar"].Name)
	}
	if got.Org.Objects["Account"].Definition.APIName != "Account" {
		t.Fatalf("org object api name = %q", got.Org.Objects["Account"].Definition.APIName)
	}
}

func TestClearRemovesGob(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{Version: Version, ProjectRoot: dir}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Clear(dir, SubdirTest); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "test", stateGobFile)); !os.IsNotExist(err) {
		t.Fatalf("gob still present after Clear(): %v", err)
	}
}

func TestFreshRejectsMissingManifestProjectFile(t *testing.T) {
	dir := t.TempDir()
	classPath := filepath.Join(dir, "classes", "Foo.cls")
	if err := os.MkdirAll(filepath.Dir(classPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(classPath, []byte("public class Foo {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	fp, ok := statFile(dir, classPath)
	if !ok {
		t.Fatal("statFile() did not fingerprint class file")
	}
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		Manifest: Manifest{
			ProjectRoot: dir,
			Files:       []File{fp},
		},
	}
	if err := os.Remove(classPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if Fresh(&entry, dir, Version) {
		t.Fatal("Fresh() = true, want false for missing manifest project file")
	}
}
