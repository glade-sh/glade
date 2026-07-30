package typesys

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func TestRefreshBuildArtifactsReusesOnlyExactUnchangedOccurrences(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.cls")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Second {}")
	proj := project.Project{Root: root, ApexFiles: []string{first, second}}
	index, previous := BuildWithArtifacts(proj, schema.Schema{})

	refreshed, err := RefreshBuildArtifacts(index, &previous)
	if err != nil {
		t.Fatalf("RefreshBuildArtifacts() = %v", err)
	}
	if refreshed.Sources == previous.Sources {
		t.Fatal("RefreshBuildArtifacts retained the previous mutable source arena")
	}
	if got := refreshed.Sources.Stats().PhysicalReadAttempts; got != 0 {
		t.Fatalf("unchanged refresh read %d physical sources, want 0", got)
	}
	if err := ValidateBuildGeneration(index, &refreshed); err != nil {
		t.Fatalf("ValidateBuildGeneration(refreshed) = %v", err)
	}
}

func TestRefreshBuildArtifactsReadsChangedOccurrenceAndDropsDeletedOccurrence(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First.cls")
	second := filepath.Join(root, "Second.cls")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Second {}")
	previousIndex, previous := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{first, second}}, schema.Schema{})
	_ = previousIndex

	writeFile(t, first, "public class First { public void changed() {} }")
	index, _ := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{first}}, schema.Schema{})
	refreshed, err := RefreshBuildArtifacts(index, &previous)
	if err != nil {
		t.Fatalf("RefreshBuildArtifacts() = %v", err)
	}
	if got := refreshed.Sources.Stats().PhysicalReadAttempts; got != 1 {
		t.Fatalf("changed refresh physical reads = %d, want 1", got)
	}
	if got := len(refreshed.Sources.All()); got != 1 {
		t.Fatalf("refreshed occurrences = %d, want deleted occurrence removed", got)
	}
	if err := ValidateBuildGeneration(index, &refreshed); err != nil {
		t.Fatalf("ValidateBuildGeneration(refreshed) = %v", err)
	}
}

func TestRefreshBuildArtifactsPreservesAliasAndRemapOccurrences(t *testing.T) {
	root := t.TempDir()
	physical := filepath.Join(root, "Shared.cls")
	alias := filepath.Join(root, "Alias.cls")
	writeFile(t, physical, "public class Shared { String value = 'Base__Item__c'; }")
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	remaps := []namespaceremap.Rule{{From: "Base", To: "runtime"}}
	project := project.Project{Root: root, Namespace: "runtime", NamespaceRemaps: remaps, ApexFiles: []string{physical, alias}}
	index, previous := BuildWithArtifacts(project, schema.Schema{})

	refreshed, err := RefreshBuildArtifacts(index, &previous)
	if err != nil {
		t.Fatalf("RefreshBuildArtifacts() = %v", err)
	}
	if got := refreshed.Sources.Stats().PhysicalReadAttempts; got != 0 {
		t.Fatalf("alias refresh read %d physical sources, want 0", got)
	}
	if got := refreshed.Sources.Stats().PhysicalSources; got != 1 {
		t.Fatalf("alias refresh physical sources = %d, want 1", got)
	}
	if got := len(refreshed.Sources.All()); got != 2 {
		t.Fatalf("alias refresh occurrences = %d, want 2", got)
	}
	if err := ValidateBuildGeneration(index, &refreshed); err != nil {
		t.Fatalf("ValidateBuildGeneration(refreshed) = %v", err)
	}
}

func TestRefreshBuildArtifactsRecanonicalizesRetargetedSameContentAlias(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first", "Shared.cls")
	second := filepath.Join(root, "second", "Shared.cls")
	alias := filepath.Join(root, "Alias.cls")
	contents := "public class Shared {}"
	writeFile(t, first, contents)
	writeFile(t, second, contents)
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	index, previous := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{alias}}, schema.Schema{})

	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshBuildArtifacts(index, &previous)
	if err != nil {
		t.Fatalf("RefreshBuildArtifacts() = %v", err)
	}
	sources := refreshed.Sources.All()
	if len(sources) != 1 {
		t.Fatalf("refreshed sources = %#v, want one", sources)
	}
	if got, want := sources[0].Metadata().PhysicalPath, canonicalPhysicalPath(second); got != want {
		t.Fatalf("refreshed physical path = %q, want retargeted %q", got, want)
	}
	if got, want := refreshed.SourceDigests.requested[alias], canonicalPhysicalPath(second); got != want {
		t.Fatalf("refreshed digest alias = %q, want retargeted %q", got, want)
	}
	if got := refreshed.Sources.Stats().PhysicalReadAttempts; got != 1 {
		t.Fatalf("retargeted alias refresh reads = %d, want 1", got)
	}
	if err := ValidateBuildGeneration(index, &refreshed); err != nil {
		t.Fatalf("ValidateBuildGeneration(refreshed) = %v", err)
	}
}

func TestRefreshBuildArtifactsFailsClosedForReadErrorAndIncompleteGeneration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Missing.cls")
	writeFile(t, path, "public class Missing {}")
	index, _ := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshBuildArtifacts(index, nil); err == nil {
		t.Fatal("RefreshBuildArtifacts accepted a missing source")
	}

	index.sourceDigests = nil
	if _, err := RefreshBuildArtifacts(index, nil); err == nil {
		t.Fatal("RefreshBuildArtifacts accepted an incomplete generation")
	}
}

func TestRefreshBuildArtifactsRejectsSourceChangedAfterIndexCapture(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ChangedAfterBuild.cls")
	writeFile(t, path, "public class ChangedAfterBuild {}")
	index, _ := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	writeFile(t, path, "public class ChangedAfterBuild { public void drifted() {} }")

	if _, err := RefreshBuildArtifacts(index, nil); err == nil {
		t.Fatal("RefreshBuildArtifacts accepted a source changed after index capture")
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
	raw := []byte("public class Utf8 {\r\n  String label = '東京';\r\n  public void run() {}\r\n}\r\n")
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

func TestBuildArtifactsSourceForTypeReturnsRecordedLogicalViewWithoutReading(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "UsesTokens.cls")
	writeFile(t, path, "public class UsesTokens { String value = '%%%NAMESPACE%%%Thing__c BasePkg__Item__c'; }")
	remaps := []namespaceremap.Rule{{From: "BasePkg", To: "runtime"}}
	sources := newWorkspaceSources(func(path string) ([]byte, error) { return os.ReadFile(path) })
	idx, artifacts := buildWithWorkspaceSources(project.Project{
		Root: root, Namespace: "runtime", NamespaceRemaps: remaps, ApexFiles: []string{path},
	}, schema.Schema{}, sources)
	if len(idx.Types) != 1 {
		t.Fatalf("types = %#v", idx.Types)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	source, ok := artifacts.SourceForType(idx.Types[0])
	if !ok {
		t.Fatal("SourceForType missed recorded logical occurrence")
	}
	if got, want := source.NormalizedString(), "public class UsesTokens { String value = 'runtime__Thing__c BasePkg__Item__c'; }"; got != want {
		t.Fatalf("normalized = %q, want %q", got, want)
	}
	if got := source.RawString(); got == "" {
		t.Fatal("RawString returned empty source")
	}
}

func TestBuildArtifactsSourceForTypeIncludesOrderedRemapFingerprint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Ordered.cls")
	writeFile(t, path, "public class Ordered { String value = 'Base__Item__c'; }")
	firstRules := []namespaceremap.Rule{{From: "Base", To: "first"}, {From: "base", To: "second"}}
	secondRules := []namespaceremap.Rule{{From: "base", To: "second"}, {From: "Base", To: "first"}}
	sources := NewWorkspaceSources()
	first, err := sources.load(SourceMetadata{RequestedPath: path, Root: root, NamespaceRemaps: firstRules})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sources.load(SourceMetadata{RequestedPath: path, Root: root, NamespaceRemaps: secondRules})
	if err != nil {
		t.Fatal(err)
	}
	sources.record(first)
	sources.record(second)
	artifacts := BuildArtifacts{Sources: sources}
	for _, tc := range []struct {
		name   string
		remaps []namespaceremap.Rule
		want   string
	}{{"first", firstRules, "first__Item__c"}, {"second", secondRules, "second__Item__c"}} {
		source, ok := artifacts.SourceForType(TypeSymbol{File: path, SourceRoot: root, SourceNamespaceRemaps: tc.remaps})
		if !ok || !strings.Contains(source.NormalizedString(), tc.want) {
			t.Fatalf("%s lookup = %q, %v; want %q", tc.name, source.NormalizedString(), ok, tc.want)
		}
	}
}

func TestWorkspaceSourcesStatsCountAttemptsPhysicalLogicalAndOccurrences(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Shared.cls")
	alias := filepath.Join(root, "Alias.cls")
	writeFile(t, path, "public class Shared {}")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	sources := newWorkspaceSources(func(path string) ([]byte, error) { return os.ReadFile(path) })
	first, err := sources.load(SourceMetadata{RequestedPath: path, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := sources.load(SourceMetadata{RequestedPath: alias, Root: root, Namespace: "dep", Dependency: true})
	if err != nil {
		t.Fatal(err)
	}
	sources.record(first)
	sources.record(second)
	if got, want := sources.Stats(), (WorkspaceSourceStats{PhysicalReadAttempts: 1, PhysicalSources: 1, LogicalViews: 2, Occurrences: 2}); got != want {
		t.Fatalf("stats = %#v, want %#v", got, want)
	}
}

func TestBuildWithArtifactsReturnsCompactSourceDigests(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Present.cls")
	missing := filepath.Join(root, "Missing.cls")
	raw := []byte("public class Present {\r\n  String café = '東京 %%%NAMESPACE%%%Thing__c BasePkg__Item__c';\r\n}\r\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, artifacts := BuildWithArtifacts(project.Project{
		Root:      root,
		Namespace: "runtime",
		NamespaceRemaps: []namespaceremap.Rule{{
			From: "BasePkg",
			To:   "runtime",
		}},
		ApexFiles: []string{path, missing},
	}, schema.Schema{})

	if artifacts.SourceDigests == nil {
		t.Fatal("SourceDigests is nil")
	}
	if got, want := artifacts.SourceDigests.PhysicalCount(), 1; got != want {
		t.Fatalf("PhysicalCount = %d, want %d", got, want)
	}
	if got, ok := artifacts.SourceDigests.Digest(path); !ok || got != sha256.Sum256(raw) {
		t.Fatalf("Digest(%q) = %x, %v, want raw-byte SHA-256", path, got, ok)
	}
	if _, ok := artifacts.SourceDigests.Digest(missing); ok {
		t.Fatalf("Digest(%q) retained failed read", missing)
	}

	_, empty := BuildWithArtifacts(project.Project{Root: root}, schema.Schema{})
	if empty.SourceDigests == nil || empty.SourceDigests.PhysicalCount() != 0 {
		t.Fatalf("empty SourceDigests = %#v, want non-nil empty set", empty.SourceDigests)
	}
	var zero BuildArtifacts
	if zero.SourceDigests != nil {
		t.Fatalf("zero BuildArtifacts SourceDigests = %#v, want nil", zero.SourceDigests)
	}
	if digest, ok := (*SourceDigestSet)(nil).Digest(path); ok || digest != ([sha256.Size]byte{}) {
		t.Fatalf("nil Digest = %x, %v", digest, ok)
	}
	if got := (*SourceDigestSet)(nil).PhysicalCount(); got != 0 {
		t.Fatalf("nil PhysicalCount = %d, want 0", got)
	}
}

func TestSourceDigestSetResolvesLiteralRelativeAbsoluteAndSymlinkAliases(t *testing.T) {
	root := t.TempDir()
	physical := filepath.Join(root, "Physical.cls")
	alias := filepath.Join(root, "Alias.cls")
	raw := []byte("public class Physical {}\r\n")
	if err := os.WriteFile(physical, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, physical)
	if err != nil {
		t.Fatal(err)
	}
	dirtyRelative := filepath.Dir(relative) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(relative)

	_, artifacts := BuildWithArtifacts(project.Project{
		Root:      root,
		ApexFiles: []string{dirtyRelative, physical, alias},
	}, schema.Schema{})

	want := sha256.Sum256(raw)
	for _, requested := range []string{
		dirtyRelative,
		physical,
		alias,
		filepath.Dir(physical) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(physical),
		filepath.Dir(alias) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(alias),
	} {
		if got, ok := artifacts.SourceDigests.Digest(requested); !ok || got != want {
			t.Errorf("Digest(%q) = %x, %v, want %x, true", requested, got, ok, want)
		}
	}
	if _, ok := artifacts.SourceDigests.Digest(relative); ok {
		t.Fatalf("Digest(%q) matched an uncaptured cleaned relative spelling", relative)
	}
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalCWD); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	for _, requested := range []string{dirtyRelative, physical, alias} {
		if got, ok := artifacts.SourceDigests.Digest(requested); !ok || got != want {
			t.Errorf("Digest(%q) after chdir = %x, %v, want %x, true", requested, got, ok, want)
		}
	}
	if _, ok := artifacts.SourceDigests.Digest(filepath.Base(physical)); ok {
		t.Fatalf("Digest(%q) matched only because the new cwd resolves it to a captured file", filepath.Base(physical))
	}
	if got := artifacts.SourceDigests.PhysicalCount(); got != 1 {
		t.Fatalf("PhysicalCount = %d, want one physical source", got)
	}
}

func TestSourceDigestSetCollapsesNamespaceAndRemapViews(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Shared.cls")
	raw := []byte("public class Shared { String value = '%%%NAMESPACE%%%Thing__c BasePkg__Item__c'; }")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	depProject := project.Project{
		Root:             root,
		Namespace:        "dep",
		NamespaceRemaps:  []namespaceremap.Rule{{From: "BasePkg", To: "dep"}},
		ApexFiles:        []string{path},
		SourceAPIVersion: "64.0",
	}
	_, artifacts := BuildWithArtifacts(project.Project{
		Root:      root,
		Namespace: "main",
		ApexFiles: []string{path},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "dep",
			SourceRoot: root,
			Version:    "1.0",
			Status:     "loaded",
			Project:    &depProject,
		}},
	}, schema.Schema{})

	if got := artifacts.SourceDigests.PhysicalCount(); got != 1 {
		t.Fatalf("PhysicalCount = %d, want one source across logical views", got)
	}
	if got, ok := artifacts.SourceDigests.Digest(path); !ok || got != sha256.Sum256(raw) {
		t.Fatalf("Digest = %x, %v, want raw digest", got, ok)
	}
}

func TestSourceDigestSetSnapshotIsImmutableAcrossSameSizeEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Mutable.cls")
	firstRaw := []byte("public class Mutable { Integer value = 1; }")
	secondRaw := []byte("public class Mutable { Integer value = 2; }")
	if len(firstRaw) != len(secondRaw) {
		t.Fatal("test fixture edit must preserve size")
	}
	if err := os.WriteFile(path, firstRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, first := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	if err := os.WriteFile(path, secondRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, second := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})

	firstDigest, firstOK := first.SourceDigests.Digest(path)
	secondDigest, secondOK := second.SourceDigests.Digest(path)
	if !firstOK || !secondOK || firstDigest != sha256.Sum256(firstRaw) || secondDigest != sha256.Sum256(secondRaw) {
		t.Fatalf("digests = %x/%v %x/%v", firstDigest, firstOK, secondDigest, secondOK)
	}
	if firstDigest == secondDigest {
		t.Fatal("same-size edit did not change a new snapshot")
	}
}

func TestSourceDigestSetSnapshotMapsDoNotAliasWorkspaceSources(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "First.cls")
	secondPath := filepath.Join(root, "Second.cls")
	writeFile(t, firstPath, "public class First {}")
	writeFile(t, secondPath, "public class Second {}")
	sources := NewWorkspaceSources()
	first, err := sources.load(SourceMetadata{RequestedPath: firstPath, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	sources.record(first)
	snapshot := sources.sourceDigestSet()

	second, err := sources.load(SourceMetadata{RequestedPath: secondPath, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	sources.record(second)
	if got := snapshot.PhysicalCount(); got != 1 {
		t.Fatalf("snapshot PhysicalCount after arena mutation = %d, want 1", got)
	}
	if _, ok := snapshot.Digest(secondPath); ok {
		t.Fatal("snapshot retained later arena mutation")
	}
	sources.mu.Lock()
	delete(sources.physical, canonicalPhysicalPath(firstPath))
	delete(sources.occurrence, sourceOccurrenceKeyForMetadata(first.metadata))
	sources.mu.Unlock()
	if got, ok := snapshot.Digest(firstPath); !ok || got != first.Digest() {
		t.Fatalf("snapshot changed after arena map deletion: %x, %v", got, ok)
	}
}

func TestSourceDigestSetSupportsConcurrentLookup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Concurrent.cls")
	raw := []byte("public class Concurrent {}")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, artifacts := BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{})
	want := sha256.Sum256(raw)

	workers := runtime.GOMAXPROCS(0) * 2
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if got, ok := artifacts.SourceDigests.Digest(path); !ok || got != want {
					t.Errorf("Digest = %x, %v, want %x, true", got, ok, want)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestSourceDigestSetWithoutSourceRetainsPhysicalLookupForAnotherAlias(t *testing.T) {
	root := t.TempDir()
	physical := filepath.Join(root, "Physical.cls")
	alias := filepath.Join(root, "Alias.cls")
	want := sha256.Sum256([]byte("public class Physical {}"))
	digests := &SourceDigestSet{
		physical:  map[string][sha256.Size]byte{physical: want},
		requested: map[string]string{physical: physical, alias: physical},
		absolute:  map[string]string{physical: physical, alias: physical},
	}

	updated := digests.withoutSource(physical)
	if got, ok := updated.Digest(physical); !ok || got != want {
		t.Fatalf("retained physical digest = %x, %t, want %x, true", got, ok, want)
	}
	if got, ok := updated.Digest(alias); !ok || got != want {
		t.Fatalf("remaining alias digest = %x, %t, want %x, true", got, ok, want)
	}
	if _, exists := updated.requested[physical]; exists {
		t.Fatal("deleted requested alias remained in digest snapshot")
	}
}

func TestBuildArtifactsSourceForTriggerReturnsRecordedLogicalViewWithoutReading(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ThingTrigger.trigger")
	writeFile(t, path, "trigger ThingTrigger on %%%NAMESPACE%%%Thing__c (before insert) {}")
	sources := newWorkspaceSources(func(path string) ([]byte, error) { return os.ReadFile(path) })
	idx, artifacts := buildWithWorkspaceSources(project.Project{
		Root: root, Namespace: "runtime", ApexFiles: []string{path},
	}, schema.Schema{}, sources)
	if len(idx.Triggers) != 1 {
		t.Fatalf("triggers = %#v", idx.Triggers)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	source, ok := artifacts.SourceForTrigger(idx.Triggers[0])
	if !ok {
		t.Fatal("SourceForTrigger missed recorded logical occurrence")
	}
	if got, want := source.NormalizedString(), "trigger ThingTrigger on runtime__Thing__c (before insert) {}"; got != want {
		t.Fatalf("normalized = %q, want %q", got, want)
	}
}
