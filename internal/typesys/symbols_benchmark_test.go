package typesys

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func BenchmarkBuildIndex(b *testing.B) {
	root := b.TempDir()
	files := make([]string, 0, 80)
	for i := 0; i < 80; i++ {
		path := filepath.Join(root, fmt.Sprintf("Class%03d.cls", i))
		writeBenchmarkFile(b, path, fmt.Sprintf("public class Class%03d { public void run() {} }", i))
		files = append(files, path)
	}
	for i := 0; i < 20; i++ {
		path := filepath.Join(root, fmt.Sprintf("Trigger%03d.trigger", i))
		writeBenchmarkFile(b, path, fmt.Sprintf("trigger Trigger%03d on Account (before insert) {}", i))
		files = append(files, path)
	}
	proj := project.Project{Root: root, ApexFiles: files}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := Build(proj, schema.Schema{})
		if idx.HasErrors() {
			b.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
		}
	}
}

func BenchmarkStandardPlatformSymbols(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		symbols := StandardPlatformSymbols()
		if len(symbols) == 0 {
			b.Fatal("missing standard symbols")
		}
	}
}

func BenchmarkStandardPlatformSymbolView(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		symbols := StandardPlatformSymbolView()
		if len(symbols) == 0 {
			b.Fatal("missing standard symbols")
		}
	}
}

func BenchmarkBuildStandardPlatformSymbolsUncached(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		symbols := buildStandardPlatformSymbols()
		if len(symbols) == 0 {
			b.Fatal("missing standard symbols")
		}
	}
}

func writeBenchmarkFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
