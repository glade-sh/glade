package watch

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestNormalizeScopePreservesDisjointRootsAndCollapsesNestedAliases(t *testing.T) {
	workspace := t.TempDir()
	consumer := filepath.Join(workspace, "consumer")
	dependency := filepath.Join(workspace, "dependency")
	if err := os.MkdirAll(filepath.Join(consumer, "force-app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dependency, 0o755); err != nil {
		t.Fatal(err)
	}
	dependencyAlias := filepath.Join(workspace, "dependency-alias")
	if err := os.Symlink(dependency, dependencyAlias); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(consumer, "sfdx-project.json")

	got := NormalizeScope(Scope{
		Roots: []string{
			filepath.Join(consumer, "."),
			filepath.Join(consumer, "force-app", ".."),
			dependencyAlias,
			dependency,
		},
		Files: []string{manifest, filepath.Join(consumer, ".", "sfdx-project.json")},
	})
	want := Scope{
		Roots:    []string{consumer, dependency},
		Topology: []string{dependencyAlias},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeScope() = %#v, want %#v", got, want)
	}
}

func TestPathVolumesMatchRejectsCrossVolumePathsCaseInsensitively(t *testing.T) {
	volumeName := func(path string) string {
		if len(path) >= 2 && path[1] == ':' {
			return path[:2]
		}
		return ""
	}
	if pathVolumesMatch([]string{"C:/workspace/project", "D:/dependencies/pkg"}, volumeName) {
		t.Fatal("cross-volume paths reported a common volume")
	}
	if !pathVolumesMatch([]string{"C:/workspace/project", "c:/dependencies/pkg"}, volumeName) {
		t.Fatal("same Windows volume with different case reported cross-volume")
	}
}

func TestFallbackTopologyBoundaryKeepsIntermediateWindowsAliasInScope(t *testing.T) {
	windowsDir := func(path string) string {
		path = strings.TrimRight(path, "/")
		if index := strings.LastIndex(path, "/"); index > len("D:") {
			return path[:index]
		}
		return "D:/"
	}
	symlink := func(path string) bool {
		return path == "D:/aliases/outer" || path == "D:/aliases/outer/packages/inner"
	}
	if got := fallbackTopologyBoundary("D:/aliases/outer/packages/inner/deep/pkg", windowsDir, symlink, true); got != "D:/aliases" {
		t.Fatalf("fallbackTopologyBoundary() = %q, want boundary above outermost alias", got)
	}
}

func TestCaptureScopePrefersPhysicalRootWhenSymlinkAliasSortsFirst(t *testing.T) {
	workspace := t.TempDir()
	physicalRoot := filepath.Join(workspace, "zzz-dependency")
	aliasRoot := filepath.Join(workspace, "aaa-dependency-alias")
	classPath := filepath.Join(physicalRoot, "force-app", "Dependency.cls")
	writeWatchFile(t, classPath, "public class Dependency {}")
	if err := os.Symlink(physicalRoot, aliasRoot); err != nil {
		t.Fatal(err)
	}

	scope := NormalizeScope(Scope{Roots: []string{aliasRoot, physicalRoot}})
	if !reflect.DeepEqual(scope.Roots, []string{physicalRoot}) {
		t.Fatalf("NormalizeScope() roots = %#v, want physical root %#v", scope.Roots, []string{physicalRoot})
	}
	if !reflect.DeepEqual(scope.Topology, []string{aliasRoot}) {
		t.Fatalf("NormalizeScope() topology = %#v, want lexical alias %#v", scope.Topology, []string{aliasRoot})
	}
	snapshot, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Files[absPath(t, classPath)]; !ok {
		t.Fatalf("CaptureScope() omitted dependency through physical root: %#v", snapshot.Files)
	}
}

func TestNormalizeScopeTracksIntermediateLexicalSymlinkComponents(t *testing.T) {
	workspace := t.TempDir()
	physicalRoot := filepath.Join(workspace, "targets/physical/pkg")
	if err := os.MkdirAll(physicalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(workspace, "links/current")
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(workspace, "targets/physical"), current); err != nil {
		t.Fatal(err)
	}
	lexicalRoot := filepath.Join(current, "pkg")
	scope := NormalizeScope(Scope{Roots: []string{lexicalRoot}, TopologyBase: workspace})
	if !reflect.DeepEqual(scope.Roots, []string{physicalRoot}) {
		t.Fatalf("NormalizeScope() roots = %#v, want physical component-resolved root %#v", scope.Roots, []string{physicalRoot})
	}
	if !reflect.DeepEqual(scope.Topology, []string{current}) {
		t.Fatalf("NormalizeScope() topology = %#v, want intermediate lexical endpoint %#v", scope.Topology, []string{current})
	}
}

func TestNormalizeScopeWithoutCommonBaseTracksImmediateIntermediateAlias(t *testing.T) {
	workspace := t.TempDir()
	physicalRoot := filepath.Join(workspace, "release/pkg")
	current := filepath.Join(workspace, "aliases/current")
	if err := os.MkdirAll(physicalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(physicalRoot), current); err != nil {
		t.Fatal(err)
	}
	scope := NormalizeScope(Scope{Roots: []string{filepath.Join(current, "pkg")}})
	if !reflect.DeepEqual(scope.Roots, []string{physicalRoot}) || !reflect.DeepEqual(scope.Topology, []string{current}) {
		t.Fatalf("NormalizeScope() without common base = %#v, want physical root and intermediate alias", scope)
	}
}

func TestProjectScopeIgnoresSymlinkedWorkspacePrefixButTracksSourceSuffix(t *testing.T) {
	parent := t.TempDir()
	physicalWorkspace := filepath.Join(parent, "physical-workspace")
	workspaceAlias := filepath.Join(parent, "workspace-alias")
	projectRoot := filepath.Join(workspaceAlias, "project")
	physicalDependency := filepath.Join(physicalWorkspace, "targets/dependency/pkg")
	current := filepath.Join(workspaceAlias, "links/current")
	if err := os.MkdirAll(filepath.Join(physicalWorkspace, "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(physicalDependency, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(physicalWorkspace, "links"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalWorkspace, workspaceAlias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(physicalWorkspace, "targets/dependency"), current); err != nil {
		t.Fatal(err)
	}
	lexicalDependency := filepath.Join(current, "pkg")
	scope := ProjectScope(projectRoot, project.Project{
		Root: projectRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "dependency",
			SourceRoot: lexicalDependency,
			Status:     "loaded",
			Project:    &project.Project{Root: lexicalDependency},
		}},
	})
	if !reflect.DeepEqual(scope.Topology, []string{current}) {
		t.Fatalf("ProjectScope() topology = %#v, want source suffix only %#v", scope.Topology, []string{current})
	}
	if !scopeHasWatchRoot(scope, projectRoot) {
		t.Fatalf("ProjectScope() rewrote ambient workspace prefix: %#v", scope)
	}
	if !scopeHasWatchRoot(scope, physicalDependency) {
		t.Fatalf("ProjectScope() omitted physical dependency root: %#v", scope)
	}
}

func TestProjectScopeTracksSymlinkedRequestedProjectRoot(t *testing.T) {
	workspace := t.TempDir()
	physicalRoot := filepath.Join(workspace, "targets/project")
	projectAlias := filepath.Join(workspace, "current-project")
	if err := os.MkdirAll(physicalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physicalRoot, projectAlias); err != nil {
		t.Fatal(err)
	}

	scope := ProjectScope(projectAlias, project.Project{Root: projectAlias})
	if !reflect.DeepEqual(scope.Roots, []string{physicalRoot}) {
		t.Fatalf("ProjectScope() roots = %#v, want physical project root %#v", scope.Roots, []string{physicalRoot})
	}
	if !reflect.DeepEqual(scope.Topology, []string{projectAlias}) {
		t.Fatalf("ProjectScope() topology = %#v, want project alias %#v", scope.Topology, []string{projectAlias})
	}
}

func TestProjectScopeResolvesAndTracksChainedSymlinkTargets(t *testing.T) {
	workspace := t.TempDir()
	releaseWorkspace := t.TempDir()
	projectRoot := filepath.Join(workspace, "project")
	physicalRoot := filepath.Join(releaseWorkspace, "releases/release-a/pkg")
	latest := filepath.Join(releaseWorkspace, "releases/latest")
	current := filepath.Join(workspace, "links/current")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(physicalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(releaseWorkspace, "releases/release-a"), latest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(latest, "pkg"), current); err != nil {
		t.Fatal(err)
	}
	lexicalRoot := current
	scope := ProjectScope(projectRoot, project.Project{
		Root: projectRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "dependency",
			SourceRoot: lexicalRoot,
			Status:     "loaded",
			Project:    &project.Project{Root: lexicalRoot},
		}},
	})
	if !scopeHasWatchRoot(scope, physicalRoot) {
		t.Fatalf("ProjectScope() roots = %#v, want fully resolved target %s", scope.Roots, physicalRoot)
	}
	if !reflect.DeepEqual(scope.Topology, []string{current, latest}) {
		t.Fatalf("ProjectScope() topology = %#v, want full chain %#v", scope.Topology, []string{current, latest})
	}
}

func TestProjectScopeWithPreviousRetainsIntermediateAndChainedTopologyAfterDelete(t *testing.T) {
	workspace := t.TempDir()
	projectRoot := filepath.Join(workspace, "project")
	physicalRoot := filepath.Join(workspace, "releases/release-a/pkg")
	latest := filepath.Join(workspace, "releases/latest")
	current := filepath.Join(workspace, "aliases/current")
	for _, root := range []string{projectRoot, physicalRoot, filepath.Dir(current)} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Dir(physicalRoot), latest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(latest, current); err != nil {
		t.Fatal(err)
	}
	lexicalRoot := filepath.Join(current, "pkg")
	loaded := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
		Namespace:  "dependency",
		SourceRoot: lexicalRoot,
		Status:     "loaded",
		Project:    &project.Project{Root: lexicalRoot},
	}}}
	previous := ProjectScope(projectRoot, loaded)
	if err := os.Remove(current); err != nil {
		t.Fatal(err)
	}
	missing := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
		Namespace:  "dependency",
		SourceRoot: lexicalRoot,
		Status:     "missing",
	}}}
	retained := ProjectScopeWithPrevious(projectRoot, missing, previous)
	if !reflect.DeepEqual(retained.Topology, []string{current, latest}) {
		t.Fatalf("ProjectScopeWithPrevious() topology after delete = %#v, want retained chain %#v", retained.Topology, []string{current, latest})
	}
}

func TestProjectScopeTopologyOwnersAreStableForSharedEndpoint(t *testing.T) {
	workspace := t.TempDir()
	projectRoot := filepath.Join(workspace, "project")
	physicalRoot := filepath.Join(workspace, "release")
	current := filepath.Join(workspace, "aliases/current")
	for _, root := range []string{projectRoot, filepath.Join(physicalRoot, "pkg-a"), filepath.Join(physicalRoot, "pkg-b"), filepath.Dir(current)} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(physicalRoot, current); err != nil {
		t.Fatal(err)
	}
	ownerA := filepath.Join(current, "pkg-a")
	ownerB := filepath.Join(current, "pkg-b")
	p := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{
		{Namespace: "a", SourceRoot: ownerA, Status: "loaded", Project: &project.Project{Root: ownerA}},
		{Namespace: "b", SourceRoot: ownerB, Status: "loaded", Project: &project.Project{Root: ownerB}},
	}}
	wantOwners := map[string][]string{current: {ownerA, ownerB}}
	first := ProjectScope(projectRoot, p)
	if !reflect.DeepEqual(first.TopologyOwners, wantOwners) {
		t.Fatalf("ProjectScope() shared owners = %#v, want %#v", first.TopologyOwners, wantOwners)
	}
	for range 20 {
		if got := ProjectScope(projectRoot, p); !reflect.DeepEqual(got, first) {
			t.Fatalf("repeated shared-endpoint scope is unstable:\nfirst=%#v\ngot=%#v", first, got)
		}
	}
}

func scopeHasWatchRoot(scope Scope, root string) bool {
	want := cleanAbsPath(root)
	for _, candidate := range scope.Roots {
		if candidate == want {
			return true
		}
	}
	return false
}

func TestProjectScopeIncludesOnlyDirectSourceDependenciesAndAuthoritativeConfigs(t *testing.T) {
	workspace := t.TempDir()
	requestedRoot := filepath.Join(workspace, "workspace", "consumer-alias")
	projectRoot := filepath.Join(workspace, "workspace", "consumer")
	directSourceRoot := filepath.Join(workspace, "dependencies", "direct-alias")
	directProjectRoot := filepath.Join(workspace, "dependencies", "direct")
	directShimRoot := filepath.Join(workspace, "shims", "direct-alias")
	directShimProjectRoot := filepath.Join(workspace, "shims", "direct")
	artifactRoot := filepath.Join(workspace, "dependencies", "artifact")
	transitiveRoot := filepath.Join(workspace, "dependencies", "transitive")
	transitiveShimRoot := filepath.Join(workspace, "shims", "transitive")
	missingShimRoot := filepath.Join(workspace, "shims", "missing")
	for _, root := range []string{requestedRoot, projectRoot, directSourceRoot, directProjectRoot, directShimRoot, directShimProjectRoot, artifactRoot, transitiveRoot, transitiveShimRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	workspaceConfig := filepath.Join(workspace, "glade.yml")
	writeWatchFile(t, workspaceConfig, "project: {}\n")

	p := project.Project{
		Root: projectRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{
			{
				Namespace:  "direct",
				SourceRoot: directSourceRoot,
				Status:     "loaded",
				Project: &project.Project{
					Root: directProjectRoot,
					ManagedPackageDependencies: []project.ManagedPackageDependency{{
						Namespace:  "transitive",
						SourceRoot: transitiveRoot,
						Project:    &project.Project{Root: transitiveRoot},
					}},
				},
			},
			{
				Namespace:    "artifact",
				SourceRoot:   artifactRoot,
				ArtifactPath: filepath.Join(workspace, "packages", "artifact.glade-package.json"),
				Status:       "loaded",
				Project:      &project.Project{Root: artifactRoot},
			},
			{Namespace: "missing", SourceRoot: filepath.Join(workspace, "dependencies", "missing"), Status: "missing"},
			{Namespace: "invalid", SourceRoot: filepath.Join(workspace, "dependencies", "invalid"), Status: "invalid", Project: &project.Project{Root: filepath.Join(workspace, "dependencies", "invalid")}},
			{Namespace: "load-error", SourceRoot: filepath.Join(workspace, "dependencies", "load-error"), Status: "load_error", Project: &project.Project{Root: filepath.Join(workspace, "dependencies", "load-error")}},
			{Namespace: "mismatch", SourceRoot: filepath.Join(workspace, "dependencies", "mismatch"), Status: "namespace_mismatch", Project: &project.Project{Root: filepath.Join(workspace, "dependencies", "mismatch")}},
		},
		PackageShims: []project.PackageShim{
			{
				Namespace:  "shim",
				SourceRoot: directShimRoot,
				Status:     "loaded",
				Project: &project.Project{
					Root: directShimProjectRoot,
					PackageShims: []project.PackageShim{{
						Namespace:  "transitive-shim",
						SourceRoot: transitiveShimRoot,
						Status:     "loaded",
						Project:    &project.Project{Root: transitiveShimRoot},
					}},
				},
			},
			{Namespace: "missing-shim", SourceRoot: missingShimRoot, Status: "missing"},
		},
	}

	got := ProjectScope(requestedRoot, p)
	excludedRoots := []string{
		artifactRoot,
		transitiveRoot,
		transitiveShimRoot,
		missingShimRoot,
		filepath.Join(workspace, "dependencies", "missing"),
		filepath.Join(workspace, "dependencies", "invalid"),
		filepath.Join(workspace, "dependencies", "load-error"),
		filepath.Join(workspace, "dependencies", "mismatch"),
	}
	want := NormalizeScope(Scope{
		Roots: []string{requestedRoot, projectRoot, directSourceRoot, directProjectRoot, directShimRoot, directShimProjectRoot},
		TopologyBase: commonPathAncestor([]string{
			requestedRoot, projectRoot, directSourceRoot, directProjectRoot, directShimRoot, directShimProjectRoot,
		}),
		Files: append(ancestorGladeConfigCandidates(requestedRoot),
			filepath.Join(requestedRoot, "sfdx-project.json"),
			filepath.Join(requestedRoot, "glade.yml"),
			filepath.Join(projectRoot, "sfdx-project.json"),
			filepath.Join(projectRoot, "glade.yml"),
			filepath.Join(directSourceRoot, "sfdx-project.json"),
			filepath.Join(directSourceRoot, "glade.yml"),
			filepath.Join(directProjectRoot, "sfdx-project.json"),
			filepath.Join(directProjectRoot, "glade.yml"),
			filepath.Join(directShimRoot, "sfdx-project.json"),
			filepath.Join(directShimRoot, "glade.yml"),
			filepath.Join(directShimProjectRoot, "sfdx-project.json"),
			filepath.Join(directShimProjectRoot, "glade.yml"),
		),
		ExcludedRoots: excludedRoots,
	})
	if !slices.Contains(want.Files, workspaceConfig) {
		t.Fatalf("test fixture normalized authoritative files = %#v, want workspace config candidate %s", want.Files, workspaceConfig)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProjectScope() = %#v, want %#v", got, want)
	}
	for _, excluded := range excludedRoots {
		for _, root := range got.Roots {
			if root == excluded {
				t.Fatalf("ProjectScope() included non-direct source root %s: %#v", excluded, got)
			}
		}
	}
}

func TestProjectScopeExclusionTraversalStopsAtProjectCycles(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "consumer")
	dependencyRoot := filepath.Join(root, "vendor/dependency")
	transitiveRoot := filepath.Join(root, "vendor/transitive")
	cyclic := &project.Project{Root: dependencyRoot}
	cyclic.ManagedPackageDependencies = []project.ManagedPackageDependency{{
		Namespace:  "transitive",
		SourceRoot: transitiveRoot,
		Status:     "loaded",
		Project:    cyclic,
	}}

	scope := ProjectScope(root, project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "dependency",
			SourceRoot: dependencyRoot,
			Status:     "loaded",
			Project:    cyclic,
		}},
	})
	if !scopeExcludesPath(scope, transitiveRoot) {
		t.Fatalf("ProjectScope() exclusions = %#v, want cyclic transitive root %s", scope.ExcludedRoots, transitiveRoot)
	}
}

func TestCaptureProjectScopeExcludesNestedNonSourceDependencies(t *testing.T) {
	root := t.TempDir()
	localClass := filepath.Join(root, "force-app/main/default/classes/Local.cls")
	directRoot := filepath.Join(root, "deps/direct")
	directClass := filepath.Join(directRoot, "force-app/main/default/classes/Direct.cls")
	transitiveRoot := filepath.Join(directRoot, "deps/transitive")
	missingRoot := filepath.Join(root, "deps/missing")
	artifactRoot := filepath.Join(root, "deps/artifact")
	invalidRoot := filepath.Join(root, "deps/invalid")
	for path, source := range map[string]string{
		localClass:  "public class Local {}",
		directClass: "global class Direct {}",
		filepath.Join(transitiveRoot, "Transitive.cls"): "global class Transitive {}",
		filepath.Join(missingRoot, "Missing.cls"):       "global class Missing {}",
		filepath.Join(artifactRoot, "Artifact.cls"):     "global class Artifact {}",
		filepath.Join(invalidRoot, "Invalid.cls"):       "global class Invalid {}",
	} {
		writeWatchFile(t, path, source)
	}
	p := project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{
			{
				Namespace:  "direct",
				SourceRoot: directRoot,
				Status:     "loaded",
				Project: &project.Project{
					Root: directRoot,
					ManagedPackageDependencies: []project.ManagedPackageDependency{{
						Namespace:  "transitive",
						SourceRoot: transitiveRoot,
						Status:     "loaded",
						Project:    &project.Project{Root: transitiveRoot},
					}},
				},
			},
			{Namespace: "missing", SourceRoot: missingRoot, Status: "missing"},
			{Namespace: "artifact", SourceRoot: artifactRoot, ArtifactPath: filepath.Join(root, "packages/artifact.json"), Status: "loaded", Project: &project.Project{Root: artifactRoot}},
			{Namespace: "invalid", SourceRoot: invalidRoot, Status: "invalid", Project: &project.Project{Root: invalidRoot}},
		},
	}
	scope := ProjectScope(root, p)
	snapshot, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, included := range []string{localClass, directClass} {
		if _, ok := snapshot.Files[absPath(t, included)]; !ok {
			t.Errorf("CaptureScope omitted legitimate source %s", included)
		}
	}
	for _, excluded := range []string{
		filepath.Join(transitiveRoot, "Transitive.cls"),
		filepath.Join(missingRoot, "Missing.cls"),
		filepath.Join(artifactRoot, "Artifact.cls"),
		filepath.Join(invalidRoot, "Invalid.cls"),
	} {
		if _, ok := snapshot.Files[absPath(t, excluded)]; ok {
			t.Errorf("CaptureScope included nested non-source dependency %s", excluded)
		}
	}
}

func TestDirectSourceRootWinsOverTransitiveExclusionOverlap(t *testing.T) {
	root := t.TempDir()
	parentRoot := filepath.Join(root, "deps/parent")
	directNestedRoot := filepath.Join(parentRoot, "deps/shared")
	directClass := filepath.Join(directNestedRoot, "force-app/main/default/classes/Shared.cls")
	writeWatchFile(t, directClass, "global class Shared {}")
	directNestedProject := &project.Project{Root: directNestedRoot}
	p := project.Project{
		Root: root,
		ManagedPackageDependencies: []project.ManagedPackageDependency{
			{
				Namespace:  "parent",
				SourceRoot: parentRoot,
				Status:     "loaded",
				Project: &project.Project{
					Root: parentRoot,
					ManagedPackageDependencies: []project.ManagedPackageDependency{{
						Namespace:  "shared-transitive",
						SourceRoot: directNestedRoot,
						Status:     "loaded",
						Project:    directNestedProject,
					}},
				},
			},
			{
				Namespace:  "shared-direct",
				SourceRoot: directNestedRoot,
				Status:     "loaded",
				Project:    directNestedProject,
			},
		},
	}

	scope := ProjectScope(root, p)
	if scopeExcludesPath(scope, directNestedRoot) {
		t.Fatalf("ProjectScope() exclusions %#v hide loaded direct root %s", scope.ExcludedRoots, directNestedRoot)
	}
	snapshot, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Files[absPath(t, directClass)]; !ok {
		t.Fatalf("CaptureScope() omitted loaded direct overlap %s: %#v", directClass, snapshot.Files)
	}
}

func TestProjectScopeRetainsNestedNearestConfigInsideBroaderRoot(t *testing.T) {
	workspace := t.TempDir()
	requestedRoot := filepath.Join(workspace, "packages", "consumer")
	nearestConfig := filepath.Join(workspace, "packages", "glade.yml")
	unrelatedConfig := filepath.Join(workspace, "unrelated", "glade.yml")
	writeWatchFile(t, nearestConfig, "project: {}\n")
	writeWatchFile(t, unrelatedConfig, "project: {}\n")
	if err := os.MkdirAll(requestedRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	scope := ProjectScope(requestedRoot, project.Project{Root: workspace})
	if !reflect.DeepEqual(scope.Roots, []string{workspace}) {
		t.Fatalf("ProjectScope() roots = %#v, want broader root %s", scope.Roots, workspace)
	}
	foundNearest := false
	for _, path := range scope.Files {
		if path == nearestConfig {
			foundNearest = true
		}
	}
	if !foundNearest {
		t.Fatalf("ProjectScope() omitted nested nearest config %s: %#v", nearestConfig, scope)
	}

	snapshot, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Files[nearestConfig]; !ok {
		t.Fatalf("CaptureScope() omitted exact nested config: %#v", snapshot.Files)
	}
	if _, ok := snapshot.Files[unrelatedConfig]; ok {
		t.Fatalf("CaptureScope() included unrelated nested config: %#v", snapshot.Files)
	}
}
