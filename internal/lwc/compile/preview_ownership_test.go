package compile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func previewOwnershipFixture(t testing.TB) (project.Project, string) {
	t.Helper()
	root := t.TempDir()
	p := project.Project{Root: root}
	for i := 0; i < 1000; i++ {
		dir := filepath.Join(root, "force-app", "main", "default", "lwc", fmt.Sprintf("bundle%04d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		meta := filepath.Join(dir, "bundle.js-meta.xml")
		if err := os.WriteFile(meta, []byte(`<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion></LightningComponentBundle>`), 0644); err != nil {
			t.Fatal(err)
		}
		p.LWCMetaFiles = append(p.LWCMetaFiles, meta)
		p.LWCFiles = append(p.LWCFiles, filepath.Join(dir, "bundle.js"))
		p.LWCHTMLFiles = append(p.LWCHTMLFiles, filepath.Join(dir, "bundle.html"))
	}
	return p, root
}

func TestPreviewOwnershipWideAllocationBudget(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("Windows retains the filepath.Rel fallback")
	}
	p, root := previewOwnershipFixture(t)
	var cfg compileConfig
	var err error
	allocations := testing.AllocsPerRun(1, func() { cfg, err = buildCompileConfig(p, root, filepath.Join(root, "dist"), "c") })
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.LWCAPIVersions) != 1000 || len(cfg.LWCFiles) != 1000 || len(cfg.LWCHTMLFiles) != 1000 {
		t.Fatal("preview config lost bundle sources")
	}
	if allocations > 300000 {
		t.Fatalf("wide ownership validation allocated %.0f times, budget 300000", allocations)
	}
}

func BenchmarkBuildCompileConfigPreview(b *testing.B) {
	p, root := previewOwnershipFixture(b)
	var digest [32]byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg, err := buildCompileConfig(p, root, filepath.Join(root, "dist"), "c")
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		cfg.ProjectRoot, cfg.OutDir = "<root>", "<out>"
		data, err := json.Marshal(cfg)
		if err != nil {
			b.Fatal(err)
		}
		next := sha256.Sum256(data)
		if i > 0 && digest != next {
			b.Fatal("compile configuration changed")
		}
		digest = next
		b.StartTimer()
	}
	b.Logf("config sha256: %x", digest)
}
