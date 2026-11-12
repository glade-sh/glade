package gladecli

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func buildLSPTestIndex(t *testing.T, root string) (typesys.Index, *typesys.BuildArtifacts) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	return index, &artifacts
}

func TestLSPAnalysisReadsDiskSemanticCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	broken := filepath.Join(project, "force-app/main/classes/Broken.cls")
	writeTestFile(t, broken, "public class Broken { public void run() { Integer x = missing; } }")

	index, artifacts := buildLSPTestIndex(t, project)
	built, source := cachedLSPAnalysis(index, artifacts, true, semanticcache.NewManager(semanticcache.Limits{}))
	if source != semanticcache.SourceBuild {
		t.Fatalf("first source = %q, want build", source)
	}
	if len(built.Diagnostics) == 0 {
		t.Fatal("fixture produced no diagnostics")
	}

	index, artifacts = buildLSPTestIndex(t, project)
	cached, source := cachedLSPAnalysis(index, artifacts, true, semanticcache.NewManager(semanticcache.Limits{}))
	if source != semanticcache.SourceDisk {
		t.Fatalf("second source = %q, want disk", source)
	}
	if len(cached.Diagnostics) != len(built.Diagnostics) {
		t.Fatalf("cached diagnostics = %d, built = %d", len(cached.Diagnostics), len(built.Diagnostics))
	}

	writeTestFile(t, broken, "public class Broken { public void run() {} }")
	index, artifacts = buildLSPTestIndex(t, project)
	edited, source := cachedLSPAnalysis(index, artifacts, true, semanticcache.NewManager(semanticcache.Limits{}))
	if source != semanticcache.SourceBuild {
		t.Fatalf("edited source = %q, want build", source)
	}
	if len(edited.Diagnostics) >= len(built.Diagnostics) {
		t.Fatalf("edit kept stale diagnostics: %d >= %d", len(edited.Diagnostics), len(built.Diagnostics))
	}
}
