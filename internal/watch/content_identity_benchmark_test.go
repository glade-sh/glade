package watch

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
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
