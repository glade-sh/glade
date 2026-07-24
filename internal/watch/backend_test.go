package watch

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/glade-sh/glade/internal/project"
)

func TestAuxiliaryPollIntervalBoundsFastDebounce(t *testing.T) {
	if got := auxiliaryPollInterval(10 * time.Millisecond); got != 500*time.Millisecond {
		t.Fatalf("auxiliaryPollInterval(fast) = %s, want 500ms", got)
	}
	if got := auxiliaryPollInterval(2 * time.Second); got != 2*time.Second {
		t.Fatalf("auxiliaryPollInterval(slow) = %s, want debounce", got)
	}
}

func TestPollingWatcherReportsChanges(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "InvoiceService.cls")
	writeWatchFile(t, classPath, "public class InvoiceService {}")
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher := NewPollingWatcher(ctx, Config{Root: root, Debounce: 10 * time.Millisecond}, initial)
	defer watcher.Close()

	writeWatchFile(t, classPath, "public class InvoiceService { void run() {} }")
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, classPath, ChangeModified, FileKindApexClass)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for polling change")
	}
}

func TestPollingWatcherReportsLightningWebComponentChanges(t *testing.T) {
	root := t.TempDir()
	lwcPath := filepath.Join(root, "force-app", "main", "default", "lwc", "accountWorkspace", "accountWorkspace.js")
	writeWatchFile(t, lwcPath, "export default class AccountWorkspace {}")
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher := NewPollingWatcher(ctx, Config{Root: root, Debounce: 10 * time.Millisecond}, initial)
	defer watcher.Close()

	writeWatchFile(t, lwcPath, "export default class AccountWorkspace { connectedCallback() {} }")
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, lwcPath, ChangeModified, FileKindLightningWebComponent)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for polling LWC change")
	}
}

func TestPollingWatcherReportsProjectConfigChanges(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "sfdx-project.json")
	writeWatchFile(t, configPath, `{"sourceApiVersion":"63.0","packageDirectories":[]}`)
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher := NewPollingWatcher(ctx, Config{Root: root, Debounce: 10 * time.Millisecond}, initial)
	defer watcher.Close()

	writeWatchFile(t, configPath, `{"sourceApiVersion":"64.00","packageDirectories":[]}`)
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, configPath, ChangeModified, FileKindIgnored)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for polling project config change")
	}
}

func TestBackendSelectionUsesPolling(t *testing.T) {
	root := t.TempDir()
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	watcher, backend, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendPoll}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if backend != BackendPoll {
		t.Fatalf("backend = %s", backend)
	}
}

func TestNativeWatcherReportsChanges(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "InvoiceService.cls")
	writeWatchFile(t, classPath, "public class InvoiceService {}")
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: root, Debounce: 25 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()

	writeWatchFile(t, classPath, "public class InvoiceService { void run() {} }")
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, classPath, ChangeModified, FileKindApexClass)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for native change")
	}
}

func TestNativeWatcherAddsCreatedDirectories(t *testing.T) {
	root := t.TempDir()
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: root, Debounce: 25 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()

	classPath := filepath.Join(root, "classes", "CreatedTest.cls")
	writeWatchFile(t, classPath, "@IsTest private class CreatedTest {}")
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, classPath, ChangeAdded, FileKindApexClass)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		if _, err := os.Stat(classPath); err != nil {
			t.Fatal(err)
		}
		t.Fatal("timeout waiting for native created directory change")
	}
}

func TestNativeWatcherReportsProjectConfigChanges(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "glade.yml")
	writeWatchFile(t, configPath, "project:\n  defaultNamespace: localpkg\n")
	initial, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: root, Debounce: 25 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()

	writeWatchFile(t, configPath, "project:\n  defaultNamespace: otherpackage\n")
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, configPath, ChangeModified, FileKindIgnored)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for native project config change")
	}
}

func TestScopedBackendsReportExternalDependencyAndExactConfigLifecycle(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			consumer := filepath.Join(workspace, "consumer")
			dependency := filepath.Join(workspace, "dependency")
			unrelated := filepath.Join(workspace, "unrelated")
			dependencyClass := filepath.Join(dependency, "force-app", "Dependency.cls")
			renamedClass := filepath.Join(dependency, "force-app", "RenamedDependency.cls")
			unrelatedClass := filepath.Join(unrelated, "Unrelated.cls")
			externalConfig := filepath.Join(workspace, "workspace-config", "glade.yml")
			if err := os.MkdirAll(consumer, 0o755); err != nil {
				t.Fatal(err)
			}
			scope := Scope{Roots: []string{consumer, dependency}, Files: []string{externalConfig}}
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := Config{Root: consumer, Scope: scope, Backend: backend, Debounce: 15 * time.Millisecond}
			watcher, selected, err := NewBackendWatcher(ctx, cfg, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}

			writeWatchFile(t, dependencyClass, "public class Dependency {}")
			awaitScopedChange(t, watcher, dependencyClass, ChangeAdded)
			writeWatchFile(t, dependencyClass, "public class Dependency { public void run() {} }")
			awaitScopedChange(t, watcher, dependencyClass, ChangeModified)
			if err := os.Rename(dependencyClass, renamedClass); err != nil {
				t.Fatal(err)
			}
			awaitScopedChanges(t, watcher, map[string]ChangeOp{
				dependencyClass: ChangeDeleted,
				renamedClass:    ChangeAdded,
			})
			if err := os.Remove(renamedClass); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, renamedClass, ChangeDeleted)

			writeWatchFile(t, externalConfig, "project: {}\n")
			awaitScopedChange(t, watcher, externalConfig, ChangeAdded)
			writeWatchFile(t, externalConfig, "project:\n  defaultNamespace: scoped\n")
			awaitScopedChange(t, watcher, externalConfig, ChangeModified)
			if err := os.Remove(externalConfig); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, externalConfig, ChangeDeleted)

			writeWatchFile(t, unrelatedClass, "public class Unrelated {}")
			select {
			case changes := <-watcher.Changes():
				t.Fatalf("unrelated sibling emitted changes: %#v", changes)
			case err := <-watcher.Errors():
				t.Fatal(err)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestNativeWatcherReconcilesChangesBetweenBaselineAndRegistration(t *testing.T) {
	root := t.TempDir()
	scope := Scope{Roots: []string{root}}
	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	classPath := filepath.Join(root, "force-app", "Reconciled.cls")
	writeWatchFile(t, classPath, "public class Reconciled {}")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: root, Scope: scope, Backend: BackendNative, Debounce: 10 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()
	awaitScopedChange(t, watcher, classPath, ChangeAdded)
}

func TestScopedBackendsReportSymlinkTopologyDeleteAndRecreate(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			physicalA := filepath.Join(workspace, "physical-a")
			physicalB := filepath.Join(workspace, "physical-b")
			alias := filepath.Join(workspace, "dependency-x")
			if err := os.MkdirAll(physicalA, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(physicalB, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(physicalA, alias); err != nil {
				t.Fatal(err)
			}
			scope := NormalizeScope(Scope{Roots: []string{alias}})
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, selected, err := NewBackendWatcher(ctx, Config{Root: workspace, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}

			if err := os.Remove(alias); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, alias, ChangeDeleted)
			if err := os.Symlink(physicalB, alias); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, alias, ChangeAdded)
		})
	}
}

func TestScopedBackendsReportChainedSymlinkTargetRetarget(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			releaseWorkspace := t.TempDir()
			projectRoot := filepath.Join(workspace, "project")
			releaseA := filepath.Join(releaseWorkspace, "releases/release-a")
			releaseB := filepath.Join(releaseWorkspace, "releases/release-b")
			latest := filepath.Join(releaseWorkspace, "releases/latest")
			current := filepath.Join(workspace, "links/current")
			for _, root := range []string{projectRoot, filepath.Join(releaseA, "pkg"), filepath.Join(releaseB, "pkg"), filepath.Dir(current)} {
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(releaseA, latest); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(latest, "pkg"), current); err != nil {
				t.Fatal(err)
			}
			lexicalRoot := current
			p := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
				Namespace:  "dependency",
				SourceRoot: lexicalRoot,
				Status:     "loaded",
				Project:    &project.Project{Root: lexicalRoot},
			}}}
			scope := ProjectScope(projectRoot, p)
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, selected, err := NewBackendWatcher(ctx, Config{Root: projectRoot, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}

			next := latest + ".next"
			if err := os.Symlink(releaseB, next); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, latest); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, latest, ChangeModified)
			nextScope := ProjectScope(projectRoot, p)
			if !scopeHasWatchRoot(nextScope, filepath.Join(releaseB, "pkg")) {
				t.Fatalf("retargeted scope roots = %#v, want %s", nextScope.Roots, filepath.Join(releaseB, "pkg"))
			}
		})
	}
}

func TestScopedBackendsObserveDeepIntermediateAliasWithAncestorScan(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			releaseA := filepath.Join(workspace, "release-a")
			releaseB := filepath.Join(workspace, "release-b")
			current := filepath.Join(workspace, "aliases/current")
			physicalA := filepath.Join(releaseA, "packages/deep/pkg")
			physicalB := filepath.Join(releaseB, "packages/deep/pkg")
			for _, root := range []string{physicalA, physicalB, filepath.Dir(current)} {
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(releaseA, current); err != nil {
				t.Fatal(err)
			}
			lexicalRoot := filepath.Join(current, "packages/deep/pkg")
			scope := NormalizeScope(Scope{Roots: []string{lexicalRoot}, ScanTopologyAncestors: true})
			if !scopeHasWatchRoot(scope, physicalA) || !reflect.DeepEqual(scope.Topology, []string{current}) {
				t.Fatalf("ancestor-scan scope = %#v, want deep physical root and outer alias", scope)
			}
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, selected, err := NewBackendWatcher(ctx, Config{Root: lexicalRoot, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}
			next := current + ".next"
			if err := os.Symlink(releaseB, next); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, current); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, current, ChangeModified)
			nextScope := NormalizeScope(Scope{Roots: []string{lexicalRoot}, ScanTopologyAncestors: true})
			if !scopeHasWatchRoot(nextScope, physicalB) {
				t.Fatalf("retargeted ancestor-scan scope = %#v, want %s", nextScope, physicalB)
			}
		})
	}
}

func TestScopedBackendsObserveNestedOuterAndInnerAliasesWithAncestorScan(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			releaseA := filepath.Join(workspace, "release-a")
			releaseB := filepath.Join(workspace, "release-b")
			componentA := filepath.Join(workspace, "components/component-a")
			componentB := filepath.Join(workspace, "components/component-b")
			componentC := filepath.Join(workspace, "components/component-c")
			outer := filepath.Join(workspace, "aliases/outer")
			inner := filepath.Join(outer, "packages/inner")
			for _, root := range []string{
				filepath.Join(releaseA, "packages"),
				filepath.Join(releaseB, "packages"),
				filepath.Join(componentA, "deep/pkg"),
				filepath.Join(componentB, "deep/pkg"),
				filepath.Join(componentC, "deep/pkg"),
				filepath.Dir(outer),
			} {
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(componentA, filepath.Join(releaseA, "packages/inner")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(componentC, filepath.Join(releaseB, "packages/inner")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(releaseA, outer); err != nil {
				t.Fatal(err)
			}
			lexicalRoot := filepath.Join(inner, "deep/pkg")
			normalize := func() Scope {
				return NormalizeScope(Scope{Roots: []string{lexicalRoot}, ScanTopologyAncestors: true})
			}
			scope := normalize()
			if !scopeHasWatchRoot(scope, filepath.Join(componentA, "deep/pkg")) || !reflect.DeepEqual(scope.Topology, []string{outer, inner}) {
				t.Fatalf("nested alias scope = %#v, want outer+inner and component A", scope)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			start := func(scope Scope) BackendWatcher {
				initial, err := CaptureScope(scope)
				if err != nil {
					t.Fatal(err)
				}
				watcher, selected, err := NewBackendWatcher(ctx, Config{Root: lexicalRoot, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
				if err != nil {
					if backend == BackendNative {
						t.Skipf("native watcher unavailable: %v", err)
					}
					t.Fatal(err)
				}
				if selected != backend {
					t.Fatalf("selected backend = %s, want %s", selected, backend)
				}
				return watcher
			}

			watcher := start(scope)
			innerPhysical := filepath.Join(releaseA, "packages/inner")
			nextInner := innerPhysical + ".next"
			if err := os.Symlink(componentB, nextInner); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(nextInner, innerPhysical); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, inner, ChangeModified)
			scope = normalize()
			if !scopeHasWatchRoot(scope, filepath.Join(componentB, "deep/pkg")) || !reflect.DeepEqual(scope.Topology, []string{outer, inner}) {
				t.Fatalf("inner-retarget scope = %#v, want component B and both aliases", scope)
			}
			_ = watcher.Close()

			watcher = start(scope)
			defer watcher.Close()
			nextOuter := outer + ".next"
			if err := os.Symlink(releaseB, nextOuter); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(nextOuter, outer); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, watcher, outer, ChangeModified)
			scope = normalize()
			if !scopeHasWatchRoot(scope, filepath.Join(componentC, "deep/pkg")) || !reflect.DeepEqual(scope.Topology, []string{outer, inner}) {
				t.Fatalf("outer-retarget scope = %#v, want component C and both aliases", scope)
			}
		})
	}
}

func TestScopedBackendsRecoverSharedEndpointWhileEitherOwnerRemains(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		for _, remaining := range []string{"a", "b"} {
			t.Run(string(backend)+"/remaining-"+remaining, func(t *testing.T) {
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
				loaded := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{
					{Namespace: "a", SourceRoot: ownerA, Status: "loaded", Project: &project.Project{Root: ownerA}},
					{Namespace: "b", SourceRoot: ownerB, Status: "loaded", Project: &project.Project{Root: ownerB}},
				}}
				previous := ProjectScope(projectRoot, loaded)
				if err := os.Remove(current); err != nil {
					t.Fatal(err)
				}
				remainingRoot := ownerA
				if remaining == "b" {
					remainingRoot = ownerB
				}
				missing := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
					Namespace:  remaining,
					SourceRoot: remainingRoot,
					Status:     "missing",
				}}}
				retained := ProjectScopeWithPrevious(projectRoot, missing, previous)
				if !slices.Contains(retained.Topology, current) {
					t.Fatalf("remaining owner %s lost shared endpoint: %#v", remaining, retained)
				}
				initial, err := CaptureScope(retained)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				watcher, selected, err := NewBackendWatcher(ctx, Config{Root: projectRoot, Scope: retained, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
				if err != nil {
					if backend == BackendNative {
						t.Skipf("native watcher unavailable: %v", err)
					}
					t.Fatal(err)
				}
				defer watcher.Close()
				if selected != backend {
					t.Fatalf("selected backend = %s, want %s", selected, backend)
				}
				if err := os.Symlink(physicalRoot, current); err != nil {
					t.Fatal(err)
				}
				awaitScopedChange(t, watcher, current, ChangeAdded)
				recoveredProject := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
					Namespace:  remaining,
					SourceRoot: remainingRoot,
					Status:     "loaded",
					Project:    &project.Project{Root: remainingRoot},
				}}}
				recovered := ProjectScopeWithPrevious(projectRoot, recoveredProject, retained)
				if !scopeHasWatchRoot(recovered, filepath.Join(physicalRoot, "pkg-"+remaining)) {
					t.Fatalf("recovered scope = %#v, want remaining physical owner", recovered)
				}
				removed := ProjectScopeWithPrevious(projectRoot, project.Project{Root: projectRoot}, recovered)
				if len(removed.Topology) != 0 {
					t.Fatalf("scope retained shared endpoint after all owners removed: %#v", removed)
				}
			})
		}
	}
}

func TestScopedBackendsRetainOuterEndpointAfterInnerTargetDelete(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			projectRoot := filepath.Join(workspace, "project")
			releaseA := filepath.Join(workspace, "releases/release-a")
			releaseB := filepath.Join(workspace, "releases/release-b")
			latest := filepath.Join(workspace, "releases/latest")
			current := filepath.Join(workspace, "aliases/current")
			for _, root := range []string{projectRoot, filepath.Join(releaseA, "pkg"), filepath.Join(releaseB, "pkg"), filepath.Dir(current)} {
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(releaseA, latest); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(latest, current); err != nil {
				t.Fatal(err)
			}
			owner := filepath.Join(current, "pkg")
			loaded := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
				Namespace: "dependency", SourceRoot: owner, Status: "loaded", Project: &project.Project{Root: owner},
			}}}
			initialScope := ProjectScope(projectRoot, loaded)
			initial, err := CaptureScope(initialScope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initialWatcher, selected, err := NewBackendWatcher(ctx, Config{Root: projectRoot, Scope: initialScope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}
			if err := os.Remove(latest); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, initialWatcher, latest, ChangeDeleted)
			_ = initialWatcher.Close()

			missing := project.Project{Root: projectRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{{
				Namespace: "dependency", SourceRoot: owner, Status: "missing",
			}}}
			retained := ProjectScopeWithPrevious(projectRoot, missing, initialScope)
			if !reflect.DeepEqual(retained.Topology, []string{current, latest}) {
				t.Fatalf("inner-delete retained topology = %#v, want outer and inner endpoints", retained.Topology)
			}
			baseline, err := CaptureScope(retained)
			if err != nil {
				t.Fatal(err)
			}
			retainedWatcher, _, err := NewBackendWatcher(ctx, Config{Root: projectRoot, Scope: retained, Backend: backend, Debounce: 10 * time.Millisecond}, baseline)
			if err != nil {
				t.Fatal(err)
			}
			defer retainedWatcher.Close()
			next := current + ".next"
			if err := os.Symlink(releaseB, next); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(next, current); err != nil {
				t.Fatal(err)
			}
			awaitScopedChange(t, retainedWatcher, current, ChangeModified)
			recovered := ProjectScopeWithPrevious(projectRoot, loaded, retained)
			if !scopeHasWatchRoot(recovered, filepath.Join(releaseB, "pkg")) || !reflect.DeepEqual(recovered.Topology, []string{current}) {
				t.Fatalf("outer-retarget recovered scope = %#v, want release B and current only", recovered)
			}
		})
	}
}

func TestScopedBackendsReportAncestorConfigCandidateCreation(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "packages/app/project")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			farther := filepath.Join(workspace, "glade.yml")
			closer := filepath.Join(workspace, "packages/app", "glade.yml")
			scope := ProjectScope(root, project.Project{Root: root})
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, selected, err := NewBackendWatcher(ctx, Config{Root: root, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}
			writeWatchFile(t, farther, "project: {}\n")
			awaitScopedChange(t, watcher, farther, ChangeAdded)
			writeWatchFile(t, closer, "project:\n  defaultNamespace: closer\n")
			awaitScopedChange(t, watcher, closer, ChangeAdded)
		})
	}
}

func TestNativeWatcherDoesNotRegisterExcludedSubtrees(t *testing.T) {
	root := t.TempDir()
	excluded := filepath.Join(root, "vendor/transitive")
	writeWatchFile(t, filepath.Join(root, "force-app/Included.cls"), "public class Included {}")
	writeWatchFile(t, filepath.Join(excluded, "nested/Excluded.cls"), "public class Excluded {}")
	scope := NormalizeScope(Scope{Roots: []string{root}, ExcludedRoots: []string{excluded}})
	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: root, Scope: scope, Debounce: 10 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()
	for _, watched := range watcher.inner.WatchList() {
		if pathWithin(watched, excluded) {
			t.Fatalf("native watcher registered excluded subtree %s: %#v", excluded, watcher.inner.WatchList())
		}
	}
}

func TestNativeWatcherRegistersExplicitRootThatOverlapsExclusion(t *testing.T) {
	root := t.TempDir()
	directRoot := filepath.Join(root, "deps/parent/deps/shared")
	writeWatchFile(t, filepath.Join(directRoot, "force-app/Shared.cls"), "global class Shared {}")
	scope := NormalizeScope(Scope{
		Roots:         []string{root, directRoot},
		ExcludedRoots: []string{directRoot},
	})
	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: root, Scope: scope, Debounce: 10 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()
	for _, watched := range watcher.inner.WatchList() {
		if watched == directRoot {
			return
		}
	}
	t.Fatalf("native watcher omitted explicit overlapping root %s: %#v", directRoot, watcher.inner.WatchList())
}

func TestScopedBackendsCarveDirectRootFromBroaderExclusion(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			root := t.TempDir()
			broad := filepath.Join(root, "vendor/broad")
			direct := filepath.Join(broad, "packages/direct")
			sibling := filepath.Join(broad, "packages/excluded-sibling")
			directClass := filepath.Join(direct, "force-app/Direct.cls")
			siblingClass := filepath.Join(sibling, "force-app/Sibling.cls")
			writeWatchFile(t, directClass, "global class Direct {}")
			writeWatchFile(t, siblingClass, "global class Sibling {}")
			p := project.Project{Root: root, ManagedPackageDependencies: []project.ManagedPackageDependency{
				{Namespace: "broad", SourceRoot: broad, Status: "missing"},
				{Namespace: "direct", SourceRoot: direct, Status: "loaded", Project: &project.Project{Root: direct}},
			}}
			scope := ProjectScope(root, p)
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := initial.Files[absPath(t, directClass)]; !ok {
				t.Fatalf("CaptureScope() omitted direct carve-out %s: %#v", directClass, scope)
			}
			if _, ok := initial.Files[absPath(t, siblingClass)]; ok {
				t.Fatalf("CaptureScope() included excluded sibling %s: %#v", siblingClass, scope)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, selected, err := NewBackendWatcher(ctx, Config{Root: root, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}
			writeWatchFile(t, directClass, "global class Direct { global void changed() {} }")
			awaitScopedChange(t, watcher, directClass, ChangeModified)
			writeWatchFile(t, siblingClass, "global class Sibling { global void ignored() {} }")
			select {
			case changes := <-watcher.Changes():
				t.Fatalf("excluded sibling emitted changes: %#v", changes)
			case err := <-watcher.Errors():
				t.Fatal(err)
			case <-time.After(100 * time.Millisecond):
			}
			if native, ok := watcher.(*NativeWatcher); ok {
				watchedDirect := false
				for _, watched := range native.inner.WatchList() {
					watchedDirect = watchedDirect || watched == direct
					if pathWithin(watched, sibling) {
						t.Fatalf("native watcher registered excluded sibling %s: %#v", sibling, native.inner.WatchList())
					}
				}
				if !watchedDirect {
					t.Fatalf("native watcher omitted direct carve-out %s: %#v", direct, native.inner.WatchList())
				}
			}
		})
	}
}

func TestScopedBackendsCarvePrimaryProjectFromBroaderExclusion(t *testing.T) {
	for _, backend := range []Backend{BackendPoll, BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			broad := filepath.Join(workspace, "vendor/broad")
			primary := filepath.Join(broad, "projects/primary")
			sibling := filepath.Join(broad, "projects/unrelated")
			primaryClass := filepath.Join(primary, "force-app/Primary.cls")
			siblingClass := filepath.Join(sibling, "force-app/Sibling.cls")
			writeWatchFile(t, primaryClass, "public class Primary {}")
			writeWatchFile(t, siblingClass, "public class Sibling {}")
			p := project.Project{Root: primary, ManagedPackageDependencies: []project.ManagedPackageDependency{{
				Namespace: "broad", SourceRoot: broad, Status: "missing",
			}}}
			scope := ProjectScope(primary, p)
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := initial.Files[absPath(t, primaryClass)]; !ok {
				t.Fatalf("CaptureScope() omitted primary carve-out %s: %#v", primaryClass, scope)
			}
			if _, ok := initial.Files[absPath(t, siblingClass)]; ok {
				t.Fatalf("CaptureScope() included unrelated sibling %s", siblingClass)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			watcher, selected, err := NewBackendWatcher(ctx, Config{Root: primary, Scope: scope, Backend: backend, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				if backend == BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer watcher.Close()
			if selected != backend {
				t.Fatalf("selected backend = %s, want %s", selected, backend)
			}
			writeWatchFile(t, primaryClass, "public class Primary { public void changed() {} }")
			awaitScopedChange(t, watcher, primaryClass, ChangeModified)
			writeWatchFile(t, siblingClass, "public class Sibling { public void ignored() {} }")
			select {
			case changes := <-watcher.Changes():
				t.Fatalf("unrelated sibling emitted changes: %#v", changes)
			case err := <-watcher.Errors():
				t.Fatal(err)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestNativeWatcherReportsScopedRootRemovalAndRecreation(t *testing.T) {
	workspace := t.TempDir()
	consumer := filepath.Join(workspace, "consumer")
	dependency := filepath.Join(workspace, "dependency")
	classPath := filepath.Join(dependency, "force-app", "Dependency.cls")
	writeWatchFile(t, filepath.Join(consumer, "Consumer.cls"), "public class Consumer {}")
	writeWatchFile(t, classPath, "public class Dependency {}")
	scope := Scope{Roots: []string{consumer, dependency}}
	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewNativeWatcher(ctx, Config{Root: consumer, Scope: scope, Debounce: 15 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()

	if err := os.RemoveAll(dependency); err != nil {
		t.Fatal(err)
	}
	awaitScopedChange(t, watcher, classPath, ChangeDeleted)
	writeWatchFile(t, classPath, "public class Dependency { public void recreated() {} }")
	awaitScopedChange(t, watcher, classPath, ChangeAdded)
}

func awaitScopedChange(t *testing.T, watcher BackendWatcher, path string, op ChangeOp) {
	t.Helper()
	awaitScopedChanges(t, watcher, map[string]ChangeOp{path: op})
}

func awaitScopedChanges(t *testing.T, watcher BackendWatcher, want map[string]ChangeOp) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	remaining := make(map[string]ChangeOp, len(want))
	for path, op := range want {
		remaining[absPath(t, path)] = op
	}
	for len(remaining) > 0 {
		select {
		case changes := <-watcher.Changes():
			for _, change := range changes {
				if op, ok := remaining[change.Path]; ok && change.Op == op {
					delete(remaining, change.Path)
				}
			}
		case err := <-watcher.Errors():
			t.Fatal(err)
		case <-deadline.C:
			t.Fatalf("timeout waiting for scoped changes: %#v", remaining)
		}
	}
}

func TestSnapshotPendingPathsUpdatesOnlyChangedWatchableFiles(t *testing.T) {
	root := t.TempDir()
	changedPath := filepath.Join(root, "Changed.cls")
	unchangedPath := filepath.Join(root, "Unchanged.cls")
	writeWatchFile(t, changedPath, "public class Changed {}")
	writeWatchFile(t, unchangedPath, "public class Unchanged {}")
	previous, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}

	writeWatchFile(t, changedPath, "public class Changed { void run() {} }")
	changes, current, err := snapshotPendingPaths(root, previous, []string{changedPath}, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(changes) != 1 {
		t.Fatalf("changes = %#v, want one modified file", changes)
	}
	assertChange(t, changes, changedPath, ChangeModified, FileKindApexClass)
	if _, ok := current.Files[unchangedPath]; !ok {
		t.Fatalf("unchanged file missing after targeted snapshot: %#v", current.Files)
	}
}

func TestSnapshotPendingPathsDeletesOnlyChangedWatchableFile(t *testing.T) {
	root := t.TempDir()
	deletedPath := filepath.Join(root, "Deleted.cls")
	unchangedPath := filepath.Join(root, "Unchanged.cls")
	writeWatchFile(t, deletedPath, "public class Deleted {}")
	writeWatchFile(t, unchangedPath, "public class Unchanged {}")
	previous, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(deletedPath); err != nil {
		t.Fatal(err)
	}
	changes, current, err := snapshotPendingPaths(root, previous, []string{deletedPath}, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(changes) != 1 {
		t.Fatalf("changes = %#v, want one deleted file", changes)
	}
	assertChange(t, changes, deletedPath, ChangeDeleted, FileKindApexClass)
	if _, ok := current.Files[deletedPath]; ok {
		t.Fatalf("deleted file still present after targeted snapshot: %#v", current.Files)
	}
	if _, ok := current.Files[unchangedPath]; !ok {
		t.Fatalf("unchanged file missing after targeted delete: %#v", current.Files)
	}
}

func TestNativeEventNeedsFullSnapshotForWatchableBundleDirectoryRemove(t *testing.T) {
	event := fsnotify.Event{
		Name: filepath.Join("force-app", "main", "default", "lwc", "accountWorkspace"),
		Op:   fsnotify.Remove,
	}

	if !nativeEventNeedsFullSnapshot(event, ClassifyPath(event.Name)) {
		t.Fatal("watchable bundle directory removal should force a full snapshot")
	}
}
