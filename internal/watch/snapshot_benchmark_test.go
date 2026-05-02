package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkCaptureSnapshotAndDiff(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 200; i++ {
		writeBenchmarkWatchFile(b, filepath.Join(root, fmt.Sprintf("Class%03d.cls", i)), fmt.Sprintf("public class Class%03d {}", i))
	}
	before, err := CaptureSnapshot(root)
	if err != nil {
		b.Fatal(err)
	}
	changed := filepath.Join(root, "Class100.cls")
	writeBenchmarkWatchFile(b, changed, "public class Class100 { public void run() {} }")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		after, err := CaptureSnapshot(root)
		if err != nil {
			b.Fatal(err)
		}
		changes := DiffSnapshots(before, after)
		if len(changes) == 0 {
			b.Fatal("expected at least one change")
		}
	}
}

func writeBenchmarkWatchFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
