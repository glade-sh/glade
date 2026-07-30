package semanticcache

import (
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
