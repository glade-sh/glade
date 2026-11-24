package startupcache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BenchmarkFreshManifestWithSourceDigests(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for i := 0; i < 64; i++ {
		name := fmt.Sprintf("Class%d", i)
		writeBenchmarkFile(b, filepath.Join(root, "force-app", "main", "default", "classes", name+".cls"), "public class "+name+" {}\n")
	}
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
	// Let file stamps age before the manifest generation, as on a warm project.
	time.Sleep(3 * time.Second)
	manifest := BuildManifestWithSourceDigests(root, p, index, artifacts.SourceDigests)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !freshManifestWithSourceDigests(Version, root, currentPlatformABI(), manifest, root, Version, artifacts.SourceDigests, nil) {
			b.Fatal("manifest is stale")
		}
	}
}

func writeBenchmarkFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatal(err)
	}
}
