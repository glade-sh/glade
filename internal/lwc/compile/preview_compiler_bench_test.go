package compile

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func BenchmarkCompilePreview(b *testing.B) {
	if os.Getenv("GLADE_LWC_DEPS_DIR") == "" {
		b.Skip("GLADE_LWC_DEPS_DIR is required for the compiler benchmark")
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	root := b.TempDir()
	out := filepath.Join(root, "dist")
	p := project.Project{Root: root}
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("bundle%04d", i)
		dir := filepath.Join(root, "force-app", "main", "default", "lwc", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatal(err)
		}
		files := []struct{ suffix, content string }{
			{".js", "import { LightningElement } from 'lwc'; export default class Component extends LightningElement {}"},
			{".html", "<template><p>Preview</p></template>"},
			{".js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion></LightningComponentBundle>`},
		}
		for _, f := range files {
			path := filepath.Join(dir, name+f.suffix)
			if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
				b.Fatal(err)
			}
			switch f.suffix {
			case ".js":
				p.LWCFiles = append(p.LWCFiles, path)
			case ".html":
				p.LWCHTMLFiles = append(p.LWCHTMLFiles, path)
			default:
				p.LWCMetaFiles = append(p.LWCMetaFiles, path)
			}
		}
	}
	var previous [32]byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manifest, err := Compile(p, Options{OutDir: out, RepoRoot: repo, CompleteLocalModuleInventory: true})
		if err != nil {
			b.Fatal(err)
		}
		if len(manifest.Modules) != 1000 {
			b.Fatalf("compiled %d modules, want 1000", len(manifest.Modules))
		}
		b.StopTimer()
		hash := sha256.New()
		err = filepath.WalkDir(out, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(out, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			data = bytes.ReplaceAll(data, []byte(root), []byte("<root>"))
			fmt.Fprintf(hash, "%s\x00%d\x00", filepath.ToSlash(rel), len(data))
			_, err = hash.Write(data)
			return err
		})
		if err != nil {
			b.Fatal(err)
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		if i > 0 && previous != digest {
			b.Fatal("compiled artifacts changed")
		}
		previous = digest
		b.StartTimer()
	}
	b.Logf("normalized artifact sha256: %x", previous)
}
