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
  defaultNamespace: samplepkg
  namespaceRemaps: ["BasePkg:stagepkg"]
  managedPackageDependencies: ["pkg:../pkg:1.2", "pkg2:/tmp/pkg"]
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
	if cfg.Project.DefaultNamespace != "samplepkg" {
		t.Fatalf("default namespace = %q", cfg.Project.DefaultNamespace)
	}
	if len(cfg.Project.NamespaceRemaps) != 1 || cfg.Project.NamespaceRemaps[0].From != "BasePkg" || cfg.Project.NamespaceRemaps[0].To != "stagepkg" {
		t.Fatalf("namespace remaps = %#v", cfg.Project.NamespaceRemaps)
	}
	if got := len(cfg.Project.ManagedPackageDependencies); got != 2 {
		t.Fatalf("managed dependency count = %d", got)
	}
	if dep := cfg.Project.ManagedPackageDependencies[0]; dep.Namespace != "pkg" || dep.SourceRoot != "../pkg" || dep.Version != "1.2" {
		t.Fatalf("managed dependency[0] = %#v", dep)
	}
}

func TestParseYAMLSubsetParsesBlockManagedPackageDependencies(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  managedPackageDependencies:
    - namespace: vendorpkg
      artifactPath: .glade/deps/vendorpkg.artifact.json
      version: "1.0"
    - namespace: pkg
      sourceRoot: ../pkg
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(cfg.Project.ManagedPackageDependencies); got != 2 {
		t.Fatalf("managed dependency count = %d", got)
	}
	if dep := cfg.Project.ManagedPackageDependencies[0]; dep.Namespace != "vendorpkg" || dep.ArtifactPath != ".glade/deps/vendorpkg.artifact.json" || dep.SourceRoot != "" || dep.Version != "1.0" {
		t.Fatalf("managed dependency[0] = %#v", dep)
	}
	if dep := cfg.Project.ManagedPackageDependencies[1]; dep.Namespace != "pkg" || dep.SourceRoot != "../pkg" || dep.ArtifactPath != "" || dep.Version != "" {
		t.Fatalf("managed dependency[1] = %#v", dep)
	}
}

func TestParseYAMLSubsetRejectsMalformedNamespaceRemap(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  namespaceRemaps: ["BasePkg"]
`); err == nil {
		t.Fatal("expected malformed namespace remap error")
	}
}

func TestParseYAMLSubsetRejectsDuplicateNamespaceRemapSource(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  namespaceRemaps: ["BasePkg:stagepkg", "basepkg:other"]
`); err == nil {
		t.Fatal("expected duplicate namespace remap source error")
	}
}

func TestParseYAMLSubsetRejectsSameNamespaceRemapSourceAndTarget(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  namespaceRemaps: ["BasePkg:basepkg"]
`); err == nil {
		t.Fatal("expected same source and target namespace remap error")
	}
}

func TestParseYAMLSubsetRejectsNamespaceRemapCycle(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  namespaceRemaps: ["BasePkg:stagepkg", "stagepkg:BasePkg"]
`); err == nil {
		t.Fatal("expected namespace remap cycle error")
	}
}

func TestParseYAMLSubsetRejectsInvalidManagedPackageDependency(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  managedPackageDependencies: ["pkg"]
`); err == nil {
		t.Fatal("expected invalid managed package dependency error")
	}
}

func TestParseYAMLSubsetAllowsArtifactNamedSourceDependency(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  managedPackageDependencies: ["pkg:artifact"]
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
  managedPackageDependencies: ["pkg:artifact:1.0"]
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
  managedPackageDependencies: ["pkg:../one", "PKG:../two"]
`); err == nil {
		t.Fatal("expected duplicate namespace error")
	}
}

func TestParseYAMLSubsetPackageShimRoots(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  packageShims: ["pkg:test-support/package-shims/pkg"]
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Project.PackageShims) != 1 {
		t.Fatalf("package shims = %#v", cfg.Project.PackageShims)
	}
	shim := cfg.Project.PackageShims[0]
	if shim.Namespace != "pkg" || shim.SourceRoot != "test-support/package-shims/pkg" {
		t.Fatalf("shim = %#v", shim)
	}
}

func TestParseYAMLSubsetRejectsDuplicatePackageShimNamespace(t *testing.T) {
	if _, err := parseYAMLSubset(`
project:
  packageShims: ["pkg:one", "PKG:two"]
`); err == nil {
		t.Fatal("expected duplicate package shim namespace error")
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
  managedPackageDependencies: ["pkg:../deps/pkg"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.Join(root, "deps", "pkg"))
	if got := cfg.Project.ManagedPackageDependencies[0].SourceRoot; got != want {
		t.Fatalf("source root = %q, want %q", got, want)
	}
}

func TestLoadFileResolvesPackageShimPaths(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "nested", "glade.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
project:
  packageShims: ["pkg:../test-support/package-shims/pkg"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.Join(root, "test-support", "package-shims", "pkg"))
	if got := cfg.Project.PackageShims[0].SourceRoot; got != want {
		t.Fatalf("shim source root = %q, want %q", got, want)
	}
}

func TestLoadSchemaSnapshotBinding(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "nested", "glade.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`
project:
  schemaSnapshot: .glade/schema/org.json
  schemaSnapshotSHA256: ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, "nested", ".glade", "schema", "org.json")
	if cfg.Project.SchemaSnapshot != wantPath {
		t.Fatalf("schema snapshot = %q, want %q", cfg.Project.SchemaSnapshot, wantPath)
	}
	if got, want := cfg.Project.SchemaSnapshotSHA256, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"; got != want {
		t.Fatalf("schema snapshot SHA-256 = %q, want %q", got, want)
	}
}

func TestParseYAMLSubsetRejectsInvalidSchemaSnapshotBinding(t *testing.T) {
	for name, source := range map[string]string{
		"missing digest": `project:
  schemaSnapshot: schema.json`,
		"missing path": `project:
  schemaSnapshotSHA256: abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789`,
		"short digest": `project:
  schemaSnapshot: schema.json
  schemaSnapshotSHA256: abcdef`,
		"non-hex digest": `project:
  schemaSnapshot: schema.json
  schemaSnapshotSHA256: zzzzzz0123456789abcdef0123456789abcdef0123456789abcdef0123456789`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseYAMLSubset(source); err == nil {
				t.Fatal("expected invalid schema snapshot binding error")
			}
		})
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
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	dep := cfg.Project.ManagedPackageDependencies[0]
	want := filepath.Clean(filepath.Join(root, "packages", "pkg.glade-package.json"))
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
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:2.0"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	dep := cfg.Project.ManagedPackageDependencies[0]
	want := filepath.Clean(filepath.Join(root, "packages", "pkg.glade-package.json"))
	if dep.ArtifactPath != want || dep.Version != "2.0" {
		t.Fatalf("dependency = %#v", dep)
	}
}
