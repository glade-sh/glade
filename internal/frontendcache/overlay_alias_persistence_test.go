package frontendcache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAliasedOverlayCannotEnterPersistentCaches(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "First.cls")
	text := "public class First {}"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "Alias.cls")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}}
	disk, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
	index, ok := typesys.TryUpdateApexFileWithOverlay(disk, path, p, typesys.NewSourceOverlay(map[string]string{alias: text}))
	if !ok {
		t.Fatal("update declined")
	}
	if _, err := semanticcache.IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{}); !errors.Is(err, typesys.ErrVolatileGeneration) {
		t.Errorf("semantic identity: %v", err)
	}
	LoadOrBuild(p, schema.Schema{}, func() (typesys.Index, typesys.BuildArtifacts) { return index, artifacts })
	if _, err := os.Stat(filepath.Join(root, ".glade", "semantic", name)); !os.IsNotExist(err) {
		t.Errorf("frontend cache published: %v", err)
	}
}
