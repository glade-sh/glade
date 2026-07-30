package typesys

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestValidateBuildGenerationRejectsChangedApexSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Generation.cls")
	if err := os.WriteFile(path, []byte("public class Generation { public static Integer value() { return 1; } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	index, artifacts := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if err := os.WriteFile(path, []byte("public class Generation { public static Integer value() { return 2; } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ValidateBuildGeneration(index, &artifacts)
	var mismatch *SourceSnapshotMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("ValidateBuildGeneration() error = %T %v, want SourceSnapshotMismatchError", err, err)
	}
	if mismatch.File != path || mismatch.ExpectedSHA256 == "" || mismatch.ActualSHA256 == "" {
		t.Fatalf("mismatch = %#v", mismatch)
	}
}

func TestValidateBuildGenerationRejectsChangedApexMetadataSidecar(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Generation.cls")
	sidecar := path + "-meta.xml"
	if err := os.WriteFile(path, []byte("public class Generation {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, []byte("<ApexClass><apiVersion>60.0</apiVersion></ApexClass>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	index, artifacts := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if err := os.WriteFile(sidecar, []byte("<ApexClass><apiVersion>61.0</apiVersion></ApexClass>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ValidateBuildGeneration(index, &artifacts)
	var mismatch *SourceSnapshotMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("ValidateBuildGeneration() error = %T %v, want SourceSnapshotMismatchError", err, err)
	}
	if mismatch.File != sidecar || mismatch.ExpectedSHA256 == "" || mismatch.ActualSHA256 == "" {
		t.Fatalf("mismatch = %#v", mismatch)
	}
}
