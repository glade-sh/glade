package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFixedIndexPublishesOnlyCurrentParseDiagnostics(t *testing.T) {
	original := "public class First { void run() { Integer x = 'wrong'; } }"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"67.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "classes", "First.cls")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	h := NewPendingHandler()
	h.SetBase(Base{Index: typesys.Build(p, schema.Schema{}), Project: project.Project{Root: root}})
	uri := uriFromPath(path)
	if got := diagnosticsCount(t, overlayOpen(h, uri, original)); got == 0 {
		t.Fatal("missing reflected semantic diagnostic")
	}
	if got := overlayChange(t, h, uri, "public class First { void run() { Integer x = 1; } }"); got != nil {
		t.Fatalf("published from an unreflected generation: %#v", got)
	}
	if got := diagnosticsCount(t, overlayChange(t, h, uri, "public class First {")); got == 0 {
		t.Fatal("missing parse diagnostic")
	}
	if got := diagnosticsCount(t, overlayChange(t, h, uri, original)); got == 0 {
		t.Fatal("missing restored semantic diagnostic")
	}
}
