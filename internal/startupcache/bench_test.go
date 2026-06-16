package startupcache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func benchmarkEntry(t testing.TB, root string, classCount int, objectCount int) Entry {
	t.Helper()
	files := make([]File, 0, classCount)
	classes := make([]vm.Class, 0, classCount)
	methods := make(map[string]vm.Method, classCount)
	for i := 0; i < classCount; i++ {
		name := fmt.Sprintf("BenchClass%d", i)
		path := filepath.Join(root, "force-app", "main", "default", "classes", name+".cls")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		body := []byte("public class " + name + " { public static void run() {} }\n")
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		fp, ok := statFile(root, path)
		if !ok {
			t.Fatalf("statFile(%q) failed", path)
		}
		files = append(files, fp)
		classes = append(classes, vm.Class{Name: name})
		methods[name+".run"] = vm.Method{Name: "run", ClassName: name}
	}
	objects := make(map[string]storage.ObjectState, objectCount)
	for i := 0; i < objectCount; i++ {
		name := fmt.Sprintf("BenchObject%d__c", i)
		fields := make(map[string]storage.Field, 24)
		for j := 0; j < 24; j++ {
			fieldName := fmt.Sprintf("Field%d__c", j)
			fields[fieldName] = storage.Field{APIName: fieldName, Type: storage.FieldString}
		}
		objects[name] = storage.ObjectState{
			Definition: storage.ObjectDefinition{
				APIName: name,
				Fields:  fields,
			},
		}
	}
	return Entry{
		Version:     Version,
		ProjectRoot: root,
		BuiltAt:     "2026-06-16T00:00:00Z",
		Manifest: Manifest{
			ProjectRoot: root,
			Files:       files,
		},
		Org: storage.OrgState{
			APIVersion: "64.0",
			Objects:    objects,
		},
		Runtime: CompiledRuntime{
			Methods: methods,
			Classes: classes,
		},
	}
}

func BenchmarkTestCacheWrite(b *testing.B) {
	root := b.TempDir()
	entry := benchmarkEntry(b, root, 1200, 350)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Write(&entry, SubdirTest); err != nil {
			b.Fatalf("Write() error = %v", err)
		}
	}
}

func BenchmarkTestCacheReadFresh(b *testing.B) {
	root := b.TempDir()
	entry := benchmarkEntry(b, root, 1200, 350)
	if err := Write(&entry, SubdirTest); err != nil {
		b.Fatalf("Write() error = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := Read(root, SubdirTest)
		if err != nil {
			b.Fatalf("Read() error = %v", err)
		}
		if got == nil {
			b.Fatal("Read() = nil")
		}
	}
}

func BenchmarkTestCacheReadStaleHeader(b *testing.B) {
	root := b.TempDir()
	entry := benchmarkEntry(b, root, 1200, 350)
	if err := Write(&entry, SubdirTest); err != nil {
		b.Fatalf("Write() error = %v", err)
	}
	if len(entry.Manifest.Files) == 0 {
		b.Fatal("benchmark entry has no manifest files")
	}
	stalePath := filepath.Join(root, filepath.FromSlash(entry.Manifest.Files[0].Path))
	if err := os.Remove(stalePath); err != nil {
		b.Fatalf("Remove() error = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := Read(root, SubdirTest)
		if err != nil {
			b.Fatalf("Read() error = %v", err)
		}
		if got != nil && Fresh(got, root, Version) {
			b.Fatal("Read() returned a fresh cache for a stale manifest")
		}
	}
}
