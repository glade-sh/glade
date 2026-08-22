package startupcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestValidatedInputDrivesFreshReadWithoutRebuildingManifest(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "Proof.cls")
	configPath := filepath.Join(root, "sfdx-project.json")
	writeStartupCacheTestFile(t, configPath, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	writeStartupCacheTestFile(t, classPath, "public class Proof {}\n")
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	_, artifacts := typesys.BuildWithArtifacts(loaded, schema.Schema{})
	reads := make(map[string]int)
	proof, err := validateInputWithSourceDigests(root, artifacts.SourceDigests, func(path string) ([]byte, error) {
		reads[filepath.Clean(path)]++
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatal(err)
	}
	if reads[classPath] != 0 {
		t.Fatalf("validated input Apex reads = %d, want digest snapshot reuse", reads[classPath])
	}
	if reads[configPath] != 1 {
		t.Fatalf("validated input config reads = %d, want exactly 1", reads[configPath])
	}
	entry, err := NewEntryWithValidatedInput(proof, storage.NewOrgState(), CompiledRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	entry.RuntimeABI = "runtime-v5"
	entry.RuntimeKey = "runtime-key"
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}

	manifestCopy := proof.Manifest()
	manifestCopy.ConfigFiles[0].SHA256 = "caller mutation"
	got, err := ReadFreshRuntimeWithValidatedInput(root, SubdirTest, Version, "runtime-v5", "runtime-key", proof)
	if err != nil || got == nil {
		t.Fatalf("ReadFreshRuntimeWithValidatedInput() = %#v, %v", got, err)
	}
	if got.Manifest.ConfigFiles[0].SHA256 == "caller mutation" {
		t.Fatal("validated input exposed mutable manifest storage")
	}
}

func TestValidatedInputDigestDetectsMetadataPreservingConfigMutation(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sfdx-project.json")
	writeStartupCacheTestFile(t, configPath, `{"packageDirectories":[],"sourceApiVersion":"66.0"}`)
	beforeInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeStartupCacheTestFile(t, configPath, `{"packageDirectories":[],"sourceApiVersion":"65.0"}`)
	if err := os.Chtimes(configPath, beforeInfo.ModTime(), beforeInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	second, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == second.Digest() {
		t.Fatal("metadata-preserving config mutation did not create a new validated input generation")
	}
}
