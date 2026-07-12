package typesys

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestBuildWithArtifactsMatchesBuild(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.trigger")
	writeFile(t, first, "public class First { public void run() {} }")
	writeFile(t, second, "trigger Second on Account (before insert) {}")
	proj := project.Project{Root: root, ApexFiles: []string{first, second}}
	sch := schema.Schema{Objects: []schema.Object{{Name: "Account"}}}

	want := Build(proj, sch)
	got, artifacts := BuildWithArtifacts(proj, sch)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildWithArtifacts index differs from Build:\n got: %#v\nwant: %#v", got, want)
	}
	if artifacts.Sources == nil || len(artifacts.Sources.All()) != 2 {
		t.Fatalf("sources = %#v, want two entries", artifacts.Sources)
	}
}

func TestWorkspaceSourcesPreservesRawNormalizedDigestAndMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "UsesTokens.cls")
	raw := []byte("public class UsesTokens { String value = '%%%NAMESPACE%%%Thing__c BasePkg__Item__c'; }\r\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	remaps := []namespaceremap.Rule{{From: "BasePkg", To: "runtime"}}
	depProject := project.Project{
		Root:             root,
		Namespace:        "runtime",
		SourceAPIVersion: "64.0",
		NamespaceRemaps:  remaps,
		ApexFiles:        []string{path},
	}
	proj := project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "runtime",
			SourceRoot: root,
			Version:    "1.2.3",
			Status:     "loaded",
			Project:    &depProject,
		}},
	}

	_, artifacts := BuildWithArtifacts(proj, schema.Schema{})
	sources := artifacts.Sources.All()
	if len(sources) != 1 {
		t.Fatalf("sources = %#v, want one", sources)
	}
	source := sources[0]
	if got := source.Raw(); !bytes.Equal(got, raw) {
		t.Fatalf("raw = %q, want %q", got, raw)
	}
	wantNormalized := []byte("public class UsesTokens { String value = 'runtime__Thing__c runtime__Item__c'; }\r\n")
	if got := source.Normalized(); !bytes.Equal(got, wantNormalized) {
		t.Fatalf("normalized = %q, want %q", got, wantNormalized)
	}
	if got, want := source.Digest(), sha256.Sum256(raw); got != want {
		t.Fatalf("digest = %x, want %x", got, want)
	}
	metadata := source.Metadata()
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	physical, err = filepath.Abs(physical)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.RequestedPath != path || metadata.PhysicalPath != filepath.Clean(physical) || metadata.Root != root || metadata.Namespace != "runtime" || metadata.Version != "1.2.3" || !metadata.Dependency {
		t.Fatalf("metadata = %#v", metadata)
	}
	if !reflect.DeepEqual(metadata.NamespaceRemaps, remaps) {
		t.Fatalf("remaps = %#v, want %#v", metadata.NamespaceRemaps, remaps)
	}
	metadata.NamespaceRemaps[0].To = "mutated"
	if got := source.Metadata().NamespaceRemaps[0].To; got != "runtime" {
		t.Fatalf("metadata remaps are mutable through caller: %q", got)
	}
	rawCopy := source.Raw()
	rawCopy[0] = 'X'
	if source.Raw()[0] != 'p' {
		t.Fatal("raw source is mutable through caller")
	}
}

func TestWorkspaceSourcesPreservesUTF8CRLFRangesAndPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Utf8.cls")
	raw := []byte("public class Utf8 {\r\n  String café = '東京';\r\n  public void run() {}\r\n}\r\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	proj := project.Project{Root: root, ApexFiles: []string{path}}

	idx, artifacts := BuildWithArtifacts(proj, schema.Schema{})
	direct := apexast.NewParser().ParseSource(path, string(raw))
	if len(direct.Diagnostics) != 0 || len(direct.Declarations) != 1 {
		t.Fatalf("direct parse = %#v", direct)
	}
	if len(idx.Types) != 1 || idx.Types[0].File != path || idx.Types[0].Range != direct.Declarations[0].Range {
		t.Fatalf("type = %#v, direct range = %#v", idx.Types, direct.Declarations[0].Range)
	}
	if got := artifacts.Sources.All()[0].Normalized(); !bytes.Equal(got, raw) {
		t.Fatalf("normalized source changed UTF-8 or CRLF: %q", got)
	}
}

func TestWorkspaceSourcesSharesPhysicalReadAcrossSymlinkAliases(t *testing.T) {
	root := t.TempDir()
	physical := filepath.Join(root, "Shared.cls")
	alias := filepath.Join(root, "Alias.cls")
	writeFile(t, physical, "public class Shared {}")
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	reads := 0
	sources := newWorkspaceSources(func(path string) ([]byte, error) {
		mu.Lock()
		reads++
		mu.Unlock()
		return os.ReadFile(path)
	})
	idx, artifacts := buildWithWorkspaceSources(project.Project{
		Root:      root,
		ApexFiles: []string{physical, alias},
	}, schema.Schema{}, sources)

	if reads != 1 {
		t.Fatalf("physical reads = %d, want 1", reads)
	}
	if len(idx.Types) != 2 || len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADETYPE001" {
		t.Fatalf("logical duplicate parity changed: types=%#v diagnostics=%#v", idx.Types, idx.Diagnostics)
	}
	got := artifacts.Sources.All()
	if len(got) != 2 || got[0].Metadata().RequestedPath != physical || got[1].Metadata().RequestedPath != alias {
		t.Fatalf("requested paths = %#v", got)
	}
	if got[0].Metadata().PhysicalPath != got[1].Metadata().PhysicalPath {
		t.Fatalf("physical aliases differ: %#v %#v", got[0].Metadata(), got[1].Metadata())
	}
}

func TestWorkspaceSourcesSeparatesNamespaceAndRemapViewsWithOneRead(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Shared.cls")
	writeFile(t, path, "public class Shared { String value = '%%%NAMESPACE%%%Thing__c BasePkg__Item__c'; }")
	depProject := project.Project{
		Root:      root,
		Namespace: "dep",
		NamespaceRemaps: []namespaceremap.Rule{{
			From: "BasePkg",
			To:   "dep",
		}},
		ApexFiles: []string{path},
	}
	proj := project.Project{
		Root:      root,
		Namespace: "main",
		ApexFiles: []string{path},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "dep",
			SourceRoot: root,
			Version:    "1.2.3",
			Status:     "loaded",
			Project:    &depProject,
		}},
	}
	reads := 0
	var mu sync.Mutex
	sources := newWorkspaceSources(func(path string) ([]byte, error) {
		mu.Lock()
		reads++
		mu.Unlock()
		return os.ReadFile(path)
	})

	_, artifacts := buildWithWorkspaceSources(proj, schema.Schema{}, sources)
	if reads != 1 {
		t.Fatalf("physical reads = %d, want 1", reads)
	}
	got := artifacts.Sources.All()
	if len(got) != 2 {
		t.Fatalf("sources = %#v, want dependency and project views", got)
	}
	if got[0].Metadata().Namespace != "dep" || got[1].Metadata().Namespace != "main" {
		t.Fatalf("namespaces = %q, %q", got[0].Metadata().Namespace, got[1].Metadata().Namespace)
	}
	if bytes.Equal(got[0].Normalized(), got[1].Normalized()) {
		t.Fatalf("namespace/remap views collapsed: %q", got[0].Normalized())
	}
	if !bytes.Contains(got[0].Normalized(), []byte("dep__Thing__c dep__Item__c")) || !bytes.Contains(got[1].Normalized(), []byte("main__Thing__c BasePkg__Item__c")) {
		t.Fatalf("normalized views = %q / %q", got[0].Normalized(), got[1].Normalized())
	}
}

func TestWorkspaceSourcesLogicalKeyPreservesRemapOrder(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Ordered.cls")
	writeFile(t, path, "public class Ordered { String value = 'Base__Item__c'; }")
	sources := NewWorkspaceSources()
	firstRules := []namespaceremap.Rule{{From: "Base", To: "first"}, {From: "base", To: "second"}}
	secondRules := []namespaceremap.Rule{{From: "base", To: "second"}, {From: "Base", To: "first"}}

	first, err := sources.load(SourceMetadata{RequestedPath: path, NamespaceRemaps: firstRules})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sources.load(SourceMetadata{RequestedPath: path, NamespaceRemaps: secondRules})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.Normalized(), second.Normalized()) {
		t.Fatalf("order-sensitive remap views collapsed: %q", first.Normalized())
	}
	if !bytes.Contains(first.Normalized(), []byte("first__Item__c")) || !bytes.Contains(second.Normalized(), []byte("second__Item__c")) {
		t.Fatalf("normalized views = %q / %q", first.Normalized(), second.Normalized())
	}
}

func TestWorkspaceSourceDigestIsStableAcrossPathAliasesAndArenas(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Stable.cls")
	alias := filepath.Join(root, "Alias.cls")
	raw := "public class Stable {}\r\n"
	writeFile(t, path, raw)
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		t.Fatal(err)
	}

	digests := make([][sha256.Size]byte, 0, 3)
	for _, requested := range []string{relative, path, alias} {
		_, artifacts := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{requested}}, schema.Schema{})
		digests = append(digests, artifacts.Sources.All()[0].Digest())
	}
	if digests[0] != digests[1] || digests[1] != digests[2] {
		t.Fatalf("path aliases changed digest: %x", digests)
	}
	writeFile(t, path, "public class Stable { public void changed() {} }\r\n")
	_, changed := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if changed.Sources.All()[0].Digest() == digests[0] {
		t.Fatal("changed content retained digest in a new arena")
	}
}

func TestBuildWithArtifactsPreservesMissingReadDiagnostic(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "Missing.cls")
	proj := project.Project{Root: root, ApexFiles: []string{missing}}

	want := Build(proj, schema.Schema{})
	got, artifacts := BuildWithArtifacts(proj, schema.Schema{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing-read diagnostic differs:\n got: %#v\nwant: %#v", got.Diagnostics, want.Diagnostics)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "GLADETYPE000" || got.Diagnostics[0].File != missing {
		t.Fatalf("diagnostics = %#v", got.Diagnostics)
	}
	if len(artifacts.Sources.All()) != 0 {
		t.Fatalf("failed source retained as successful artifact: %#v", artifacts.Sources.All())
	}
}

func TestWorkspaceSourcesCachesReadErrorsWithoutDroppingDiagnostics(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "Missing.cls")
	reads := 0
	sources := newWorkspaceSources(func(path string) ([]byte, error) {
		reads++
		return os.ReadFile(path)
	})

	idx, _ := buildWithWorkspaceSources(project.Project{
		Root:      root,
		ApexFiles: []string{missing, missing},
	}, schema.Schema{}, sources)
	if reads != 1 {
		t.Fatalf("failed physical reads = %d, want 1", reads)
	}
	if len(idx.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v, want one per logical source occurrence", idx.Diagnostics)
	}
	for _, diag := range idx.Diagnostics {
		if diag.Code != "GLADETYPE000" || diag.File != missing {
			t.Fatalf("diagnostic = %#v", diag)
		}
	}
}
