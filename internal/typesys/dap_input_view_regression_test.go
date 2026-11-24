package typesys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestFrontendInputCaptureObservesArtifactSwap(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "dependency.json")
	original := []byte("<Flow><label>original</label></Flow>")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	view := inputview.New()
	defer view.Close()
	p := (project.Project{Root: root, ManagedPackageDependencies: []project.ManagedPackageDependency{{ArtifactPath: path}}}).WithInputs(view)
	if _, err := p.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<Flow><label>changed</label></Flow>"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _ = CaptureFrontendInputs(p, schema.Schema{})
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if view.Err() == nil {
		t.Fatal("frontend capture bypassed the input view")
	}
}
