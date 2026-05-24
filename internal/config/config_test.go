package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAMLSubset(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  root: .
  packageDirs: ["force-app", "packages/core"]
  defaultNamespace: verifiable
  managedPackageDependencies: ["znu:../znu:1.2", "pkg:/tmp/pkg"]
`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Root != "." {
		t.Fatalf("root = %q", cfg.Project.Root)
	}
	if got := len(cfg.Project.PackageDirs); got != 2 {
		t.Fatalf("package dir count = %d", got)
	}
	if cfg.Project.DefaultNamespace != "verifiable" {
		t.Fatalf("default namespace = %q", cfg.Project.DefaultNamespace)
	}
	if got := len(cfg.Project.ManagedPackageDependencies); got != 2 {
		t.Fatalf("managed dependency count = %d", got)
	}
	if dep := cfg.Project.ManagedPackageDependencies[0]; dep.Namespace != "znu" || dep.SourceRoot != "../znu" || dep.Version != "1.2" {
		t.Fatalf("managed dependency[0] = %#v", dep)
	}
}

func TestParseYAMLSubsetRejectsInvalidManagedPackageDependency(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  managedPackageDependencies: ["znu"]
`); err == nil {
		t.Fatal("expected invalid managed package dependency error")
	}
}

func TestParseYAMLSubsetAllowsArtifactNamedSourceDependency(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  managedPackageDependencies: ["znu:artifact"]
`)
	if err != nil {
		t.Fatal(err)
	}
	dep := cfg.Project.ManagedPackageDependencies[0]
	if dep.SourceRoot != "artifact" || dep.ArtifactPath != "" {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestParseYAMLSubsetAllowsArtifactNamedSourceDependencyWithVersion(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  managedPackageDependencies: ["znu:artifact:1.0"]
`)
	if err != nil {
		t.Fatal(err)
	}
	dep := cfg.Project.ManagedPackageDependencies[0]
	if dep.SourceRoot != "artifact" || dep.ArtifactPath != "" || dep.Version != "1.0" {
		t.Fatalf("dependency = %#v", dep)
	}
}

func TestParseYAMLSubsetRejectsDuplicateManagedPackageDependencyNamespace(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  managedPackageDependencies: ["znu:../one", "ZNU:../two"]
`); err == nil {
		t.Fatal("expected duplicate namespace error")
	}
}

func TestLoadFileResolvesManagedPackageDependencyPaths(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "nested", "glade.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
project:
  managedPackageDependencies: ["znu:../deps/znu"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.Join(root, "deps", "znu"))
	if got := cfg.Project.ManagedPackageDependencies[0].SourceRoot; got != want {
		t.Fatalf("source root = %q, want %q", got, want)
	}
}

func TestLoadFileResolvesManagedPackageArtifactPaths(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "nested", "glade.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
project:
  managedPackageDependencies: ["znu:artifact:../packages/znu.glade-package.json"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	dep := cfg.Project.ManagedPackageDependencies[0]
	want := filepath.Clean(filepath.Join(root, "packages", "znu.glade-package.json"))
	if dep.ArtifactPath != want {
		t.Fatalf("artifact path = %q, want %q", dep.ArtifactPath, want)
	}
	if dep.SourceRoot != "" {
		t.Fatalf("source root = %q, want empty", dep.SourceRoot)
	}
}

func TestLoadFileParsesManagedPackageArtifactVersion(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "nested", "glade.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
project:
  managedPackageDependencies: ["znu:artifact:../packages/znu.glade-package.json:2.0"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	dep := cfg.Project.ManagedPackageDependencies[0]
	want := filepath.Clean(filepath.Join(root, "packages", "znu.glade-package.json"))
	if dep.ArtifactPath != want || dep.Version != "2.0" {
		t.Fatalf("dependency = %#v", dep)
	}
}
