package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func BenchmarkWatchReconcileLargeScope(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 3355; i++ {
		writeBenchmarkWatchFile(b, filepath.Join(root, fmt.Sprintf("Class%04d.cls", i)), fmt.Sprintf("public class Class%04d { /* %s */ }", i, strings.Repeat("x", 1024)))
	}
	scope := NormalizeScope(Scope{Roots: []string{root}})
	baseline, err := CaptureScope(scope)
	if err != nil {
		b.Fatal(err)
	}
	changed := filepath.Join(root, "Class1000.cls")
	writeBenchmarkWatchFile(b, changed, "public class Class1000 { void updated() {} }")
	for _, full := range []bool{false, true} {
		name := "native_touched"
		if full {
			name = "poll_full"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				changes, _, err := snapshotPendingScope(scope, baseline, []string{changed}, full)
				if err != nil || len(changes) != 1 || changes[0].Path != changed || changes[0].Op != ChangeModified {
					b.Fatalf("changes=%v err=%v", changes, err)
				}
			}
		})
	}
}

func BenchmarkWatchPollingProjectResources(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 3355; i++ {
		writeBenchmarkWatchFile(b, filepath.Join(root, "classes", fmt.Sprintf("Class%04d.cls", i)), fmt.Sprintf("public class Class%04d { /* %s */ }", i, strings.Repeat("x", 1024)))
	}
	for i := 0; i < 4; i++ {
		writeBenchmarkWatchFile(b, filepath.Join(root, "staticresources", fmt.Sprintf("asset%d.resource", i)), strings.Repeat("r", 2<<20))
	}
	changed := filepath.Join(root, "classes", "Class1000.cls")
	first := time.Unix(1600000000, 0)
	if err := os.Chtimes(changed, first, first); err != nil {
		b.Fatal(err)
	}
	scope := NormalizeScope(Scope{Roots: []string{root}})
	// Measure steady-state polling after the stamp safety window.
	time.Sleep(2100 * time.Millisecond)
	before, err := CaptureScope(scope)
	if err != nil {
		b.Fatal(err)
	}
	updated := "public class Class1000 { void updated() {} }"
	writeBenchmarkWatchFile(b, changed, updated)
	second := first.Add(time.Second)
	if err := os.Chtimes(changed, second, second); err != nil {
		b.Fatal(err)
	}
	original := fmt.Sprintf("public class Class%04d { /* %s */ }", 1000, strings.Repeat("x", 1024))
	want := []Change{{Path: changed, Op: ChangeModified, Kind: FileKindApexClass, Name: "Class1000", Before: &FileMetadata{ModTimeUnixNano: first.UnixNano(), Size: int64(len(original))}, After: &FileMetadata{ModTimeUnixNano: second.UnixNano(), Size: int64(len(updated))}}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		after, err := captureScopeWithPrevious(scope, before)
		if err != nil {
			b.Fatal(err)
		}
		changes := DiffSnapshots(before, after)
		if !reflect.DeepEqual(changes, want) {
			b.Fatalf("changes=%#v, want %#v", changes, want)
		}
	}
}
