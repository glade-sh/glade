package semanticcache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestIdentityForBuildChangesWithEverySemanticInputClass(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Identity.cls")
	if err := os.WriteFile(path, []byte(`public class Identity { public static Integer value() { return 1; } }`), 0o600); err != nil {
		t.Fatal(err)
	}
	build := func(objects []schema.Object) (typesys.Index, typesys.BuildArtifacts) {
		return typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{Objects: objects})
	}
	index, artifacts := build(nil)
	options := sema.AnalyzeOptions{Diagnostics: true, SuppressPerformanceDiagnostics: true}
	base, err := IdentityForBuild(index, &artifacts, options)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(`public class Identity { public static Integer value() { return 2; } }`), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceIndex, sourceArtifacts := build(nil)
	sourceChanged, err := IdentityForBuild(sourceIndex, &sourceArtifacts, options)
	if err != nil {
		t.Fatal(err)
	}
	if sourceChanged.ProjectContentSHA256 == base.ProjectContentSHA256 {
		t.Fatal("source change retained project content identity")
	}

	schemaIndex, schemaArtifacts := build([]schema.Object{{Name: "Invoice__c"}})
	schemaChanged, err := IdentityForBuild(schemaIndex, &schemaArtifacts, options)
	if err != nil {
		t.Fatal(err)
	}
	if schemaChanged.SchemaContentSHA256 == sourceChanged.SchemaContentSHA256 {
		t.Fatal("schema change retained schema content identity")
	}

	optionChanged, err := IdentityForBuild(sourceIndex, &sourceArtifacts, sema.AnalyzeOptions{
		Diagnostics:                    true,
		ExportTypes:                    true,
		SuppressPerformanceDiagnostics: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if optionChanged.OptionsFingerprint == sourceChanged.OptionsFingerprint {
		t.Fatal("option change retained option fingerprint")
	}
}

func TestIdentityForBuildRejectsIncompleteGeneration(t *testing.T) {
	_, err := IdentityForBuild(typesys.Index{}, &typesys.BuildArtifacts{}, sema.AnalyzeOptions{})
	if err == nil {
		t.Fatal("incomplete build generation produced a trusted identity")
	}
}

func TestSemanticPlatformABIIsVersioned(t *testing.T) {
	if sema.PlatformABI == "" {
		t.Fatal("semantic platform ABI must not be empty")
	}
}

func TestIdentityForBuildDistinguishesPartialIndex(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "IdentityPartial.cls")
	if err := os.WriteFile(path, []byte(`public class IdentityPartial {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	complete, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	index.Subset = true
	partial, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if complete.ProjectContentSHA256 == partial.ProjectContentSHA256 {
		t.Fatal("partial index retained complete index identity")
	}
}

func TestIdentityForBuildRejectsVolatileGeneration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Draft.cls")
	if err := os.WriteFile(path, []byte("public class Draft {}"), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ApexFiles: []string{path}}
	overlay := typesys.NewSourceOverlay(map[string]string{path: "public class Draft { void added() {} }"})
	index, artifacts := typesys.BuildWithArtifactsOverlay(p, schema.Schema{}, overlay)
	if _, err := IdentityForBuild(index, &artifacts, sema.AnalyzeOptions{Diagnostics: true}); !errors.Is(err, typesys.ErrVolatileGeneration) {
		t.Fatalf("volatile generation identity error = %v", err)
	}
	disk, diskArtifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
	if _, err := IdentityForBuild(disk, &diskArtifacts, sema.AnalyzeOptions{Diagnostics: true}); err != nil {
		t.Fatalf("disk generation identity error = %v", err)
	}
}
