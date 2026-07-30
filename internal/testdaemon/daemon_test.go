package testdaemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonWarningBearingEditRebuildsOnceAndKeepsScope(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	otherPath := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	testPath := filepath.Join(root, "force-app/main/default/classes/ServiceTest.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Service { public static Integer value() { return 1; } }")
	writeFile(t, otherPath, "public class Other { public static Integer value() { return 1; } }")
	writeFile(t, testPath, "@IsTest private class ServiceTest { @IsTest static void testValue() { System.assertEquals(1, Service.value()); } }")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.index.Diagnostics = append(d.index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Message: "warning"})
	d.graph = watch.BuildReferenceGraph(d.index)
	beforeGraph := d.graph
	beforeGraphWant := watch.BuildReferenceGraph(d.index)
	d.mu.Unlock()
	scope := d.WatchScopeSnapshot(root)
	writeFile(t, classPath, "public class Service { public static Integer value() { return 2; } }")
	writeFile(t, otherPath, "public class Other { public static Integer value() { return 2; } }")
	changes := []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Service"}}
	loads, builds, captures, refreshes := 0, 0, 0, 0
	d.loadProjectFn = func(root string) (project.Project, error) { loads++; return loadProject(root) }
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) { builds++; return buildProjectIndex(p) }
	d.captureScopeFn = func(scope watch.Scope) (watch.Snapshot, error) { captures++; return watch.CaptureScope(scope) }
	d.refreshGraphFn = func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		if len(changes) != 2 {
			t.Errorf("authoritative graph delta = %d changes, want original plus unbatched edit", len(changes))
		}
		return graph.Refreshed(index, changes), nil
	}
	reusable, err := d.TryUpdateChanges(context.Background(), changes, scope)
	if err != nil {
		t.Fatal(err)
	}
	if !reusable || loads != 1 || builds != 1 || captures != 2 || refreshes != 1 {
		t.Fatalf("fallback = reusable:%t loads:%d builds:%d captures:%d refreshes:%d, want true/1/1/2/1", reusable, loads, builds, captures, refreshes)
	}
	_, fresh, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.IndexSnapshot(); !reflect.DeepEqual(got, fresh) {
		t.Fatal("daemon fallback differs from clean load")
	}
	d.mu.RLock()
	afterGraph := d.graph
	d.mu.RUnlock()
	if afterGraph == beforeGraph || !reflect.DeepEqual(afterGraph, watch.BuildReferenceGraph(fresh)) || !reflect.DeepEqual(beforeGraph, beforeGraphWant) {
		t.Fatal("daemon fallback graph was not an immutable refresh matching a clean build")
	}
	gotSelection := d.SelectAffected(changes)
	wantSelection := watch.SelectAffectedTestsWithRefGraph(fresh, changes, watch.BuildReferenceGraph(fresh))
	if !reflect.DeepEqual(gotSelection, wantSelection) || gotSelection.Mode != watch.SelectionDirect {
		t.Fatalf("selection = %#v, want direct %#v", gotSelection, wantSelection)
	}
}

func TestDaemonLateFallbackReloadsInsideProofWindow(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	secondPath := filepath.Join(root, "force-app/main/default/classes/Second.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Service {}")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	scope := d.WatchScopeSnapshot(root)
	loads, builds, captures, refreshes := 0, 0, 0, 0
	d.loadProjectFn = func(root string) (project.Project, error) {
		loads++
		loaded, loadErr := loadProject(root)
		if loads == 1 {
			writeFile(t, secondPath, "public class Second {}")
		}
		return loaded, loadErr
	}
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) { builds++; return buildProjectIndex(p) }
	d.captureScopeFn = func(scope watch.Scope) (watch.Snapshot, error) { captures++; return watch.CaptureScope(scope) }
	d.refreshGraphFn = func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		return graph.Refreshed(index, changes), nil
	}
	changes := []watch.Change{{Path: classPath, Op: watch.ChangeDeleted, Kind: watch.FileKindApexClass, Name: "Service"}}
	reusable, err := d.TryUpdateChanges(context.Background(), changes, scope)
	if err != nil {
		t.Fatal(err)
	}
	if !reusable || loads != 2 || builds != 1 || captures != 2 || refreshes != 0 {
		t.Fatalf("late fallback = reusable:%t loads:%d builds:%d captures:%d refreshes:%d, want true/2/1/2/0", reusable, loads, builds, captures, refreshes)
	}
	_, fresh, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.IndexSnapshot(), fresh) {
		t.Fatal("late fallback published the project loaded before its proof baseline")
	}
	d.mu.RLock()
	lateGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(lateGraph, watch.BuildReferenceGraph(fresh)) {
		t.Fatal("late fallback full graph differs from clean build")
	}
}

func TestDaemonFallbackRejectsHiddenConfigIdentityChange(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Service {}")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.index.Diagnostics = append(d.index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Message: "warning"})
	d.graph = watch.BuildReferenceGraph(d.index)
	before := d.index
	d.mu.Unlock()
	scope := d.WatchScopeSnapshot(root)
	writeFile(t, manifestPath, `{"namespace":"changed","packageDirectories":[{"path":"force-app","default":true}]}`)
	builds := 0
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) { builds++; return buildProjectIndex(p) }
	reusable, err := d.TryUpdateChanges(context.Background(), []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Service"}}, scope)
	if err != nil {
		t.Fatal(err)
	}
	if reusable || builds != 0 || !reflect.DeepEqual(d.IndexSnapshot(), before) {
		t.Fatalf("hidden config drift = reusable:%t builds:%d", reusable, builds)
	}
}

func TestDaemonFallbackProofDriftRetriesWithoutIntermediatePublish(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	otherPath := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Service {}")
	writeFile(t, otherPath, "public class Other {}")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.index.Diagnostics = append(d.index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Message: "warning"})
	d.graph = watch.BuildReferenceGraph(d.index)
	before := d.index
	beforeGraph := d.graph
	beforeGraphWant := watch.BuildReferenceGraph(d.index)
	d.mu.Unlock()
	scope := d.WatchScopeSnapshot(root)
	writeFile(t, classPath, "public class Service { public static Integer originalChange() { return 1; } }")
	loads, builds, captures, refreshes := 0, 0, 0, 0
	d.loadProjectFn = func(root string) (project.Project, error) { loads++; return loadProject(root) }
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) {
		builds++
		if !reflect.DeepEqual(d.IndexSnapshot(), before) {
			t.Error("fallback published before proof")
		}
		return buildProjectIndex(p)
	}
	d.captureScopeFn = func(scope watch.Scope) (watch.Snapshot, error) {
		captures++
		return watch.CaptureScope(scope)
	}
	d.refreshGraphFn = func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		candidate := graph.Refreshed(index, changes)
		if refreshes == 1 {
			writeFile(t, otherPath, "public class Other { public static Integer value() { return 2; } }")
		}
		return candidate, nil
	}
	reusable, err := d.TryUpdateChanges(context.Background(), []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Service"}}, scope)
	if err != nil || !reusable {
		t.Fatalf("retry = reusable:%t err:%v", reusable, err)
	}
	if loads != 2 || builds != 2 || captures != 4 || refreshes != 1 {
		t.Fatalf("retry counts = loads:%d builds:%d captures:%d refreshes:%d, want 2/2/4/1", loads, builds, captures, refreshes)
	}
	_, fresh, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.IndexSnapshot(), fresh) {
		t.Fatal("retry did not publish the queued second edit")
	}
	d.mu.RLock()
	finalGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(finalGraph, watch.BuildReferenceGraph(fresh)) {
		t.Fatal("retry refreshed only the original change instead of rebuilding the full graph")
	}
	if !reflect.DeepEqual(beforeGraph, beforeGraphWant) {
		t.Fatal("failed graph transaction mutated the previously published graph")
	}
}

func TestDaemonFallbackBuildAndCaptureErrorsRetainOldState(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Service {}")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.index.Diagnostics = append(d.index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Message: "warning"})
	d.graph = watch.BuildReferenceGraph(d.index)
	beforeProject, beforeIndex, beforeGraph := d.project, d.index, d.graph
	d.mu.Unlock()
	scope := d.WatchScopeSnapshot(root)
	changes := []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Service"}}
	buildErr := errors.New("build failed")
	d.buildIndexFn = func(project.Project) (typesys.Index, error) { return typesys.Index{}, buildErr }
	if reusable, gotErr := d.TryUpdateChanges(context.Background(), changes, scope); reusable || !errors.Is(gotErr, buildErr) {
		t.Fatalf("build failure = reusable:%t err:%v", reusable, gotErr)
	}
	assertState := func(stage string) {
		d.mu.RLock()
		defer d.mu.RUnlock()
		if !reflect.DeepEqual(d.project, beforeProject) || !reflect.DeepEqual(d.index, beforeIndex) || d.graph != beforeGraph {
			t.Fatalf("%s published daemon state", stage)
		}
	}
	assertState("build failure")
	captureErr := errors.New("capture failed")
	d.captureScopeFn = func(watch.Scope) (watch.Snapshot, error) { return watch.Snapshot{}, captureErr }
	if reusable, gotErr := d.TryUpdateChanges(context.Background(), changes, scope); reusable || !errors.Is(gotErr, captureErr) {
		t.Fatalf("capture failure = reusable:%t err:%v", reusable, gotErr)
	}
	assertState("capture failure")
	d.buildIndexFn = buildProjectIndex
	d.captureScopeFn = watch.CaptureScope
	writeFile(t, classPath, "public class Service { public void changed() {} }")
	refreshErr := errors.New("refresh failed")
	d.refreshGraphFn = func(*watch.RefGraph, typesys.Index, []watch.Change) (*watch.RefGraph, error) { return nil, refreshErr }
	if reusable, gotErr := d.TryUpdateChanges(context.Background(), changes, scope); reusable || !errors.Is(gotErr, refreshErr) {
		t.Fatalf("refresh failure = reusable:%t err:%v", reusable, gotErr)
	}
	assertState("refresh failure")
}

func TestDaemonUpdateChangesDoesNotPublishFailedFallback(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Stable { public void beforeEdit() {} }")
	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	daemon.mu.RLock()
	beforeProject := daemon.project
	beforeIndex := daemon.index
	beforeGraph := daemon.graph
	daemon.mu.RUnlock()

	writeFile(t, manifestPath, "{")
	writeFile(t, classPath, "public class Stable {")
	err = daemon.UpdateChanges([]watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Stable"}})
	if err == nil {
		t.Fatal("UpdateChanges succeeded after authoritative fallback failed")
	}

	daemon.mu.RLock()
	afterProject := daemon.project
	afterIndex := daemon.index
	afterGraph := daemon.graph
	daemon.mu.RUnlock()
	if !reflect.DeepEqual(afterProject, beforeProject) {
		t.Errorf("daemon published project after failed fallback:\nafter: %#v\nbefore: %#v", afterProject, beforeProject)
	}
	if !reflect.DeepEqual(afterIndex, beforeIndex) {
		t.Errorf("daemon published index after failed fallback:\nafter: %#v\nbefore: %#v", afterIndex, beforeIndex)
	}
	if !reflect.DeepEqual(afterGraph, beforeGraph) {
		t.Errorf("daemon refreshed graph after failed fallback:\nafter: %#v\nbefore: %#v", afterGraph, beforeGraph)
	}
	if afterGraph != beforeGraph {
		t.Error("daemon replaced the graph after failed fallback")
	}

	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Stable { public void afterEdit() {} }")
	if err := daemon.UpdateChanges([]watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Stable"}}); err != nil {
		t.Fatalf("UpdateChanges did not recover after repaired fallback: %v", err)
	}
	wantIndex, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	daemon.mu.RLock()
	recoveredIndex := daemon.index
	recoveredGraph := daemon.graph
	daemon.mu.RUnlock()
	if !reflect.DeepEqual(recoveredIndex, wantIndex) {
		t.Errorf("recovered index differs from clean load:\nrecovered: %#v\nclean: %#v", recoveredIndex, wantIndex)
	}
	if recoveredGraph == beforeGraph {
		t.Error("successful recovery retained the pre-failure graph")
	}
	if wantGraph := watch.BuildReferenceGraph(wantIndex); !reflect.DeepEqual(recoveredGraph, wantGraph) {
		t.Errorf("recovered graph differs from clean build:\nrecovered: %#v\nclean: %#v", recoveredGraph, wantGraph)
	}
}

func TestDaemonPublishesConfigProjectIndexAndGraphAtomically(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	initial := d.project
	d.mu.RUnlock()
	if len(initial.ManagedPackageDependencies) != 1 || initial.ManagedPackageDependencies[0].Namespace != "stagepkg" {
		t.Fatalf("initial project dependency = %#v, want stagepkg", initial.ManagedPackageDependencies)
	}

	parent := filepath.Dir(root)
	dependencyRoot := filepath.Join(parent, "dependency-b")
	writeFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{"namespace":"OtherPkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(dependencyRoot, "force-app/main/default/classes/DependencyB.cls"), "public class DependencyB {}")
	configPath := filepath.Join(root, "glade.yml")
	writeFile(t, configPath, "project:\n  namespaceRemaps: [\"OtherPkg:nextpkg\"]\n  managedPackageDependencies: [\"nextpkg:../dependency-b:2.0\"]\n")
	if err := d.UpdateChanges([]watch.Change{{Path: configPath, Op: watch.ChangeModified, Kind: watch.FileKindIgnored, Name: "glade.yml"}}); err != nil {
		t.Fatalf("config reload: %v", err)
	}

	wantProject, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	wantIndex, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	wantGraph := watch.BuildReferenceGraph(wantIndex)
	d.mu.RLock()
	gotProject := d.project
	gotIndex := d.index
	gotGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(gotProject, wantProject) {
		t.Errorf("published project differs from authoritative load:\ngot:  %#v\nwant: %#v", gotProject, wantProject)
	}
	if !reflect.DeepEqual(gotIndex, wantIndex) {
		t.Errorf("published index differs from authoritative load:\ngot:  %#v\nwant: %#v", gotIndex, wantIndex)
	}
	if !reflect.DeepEqual(gotGraph, wantGraph) {
		t.Errorf("published graph differs from authoritative build:\ngot:  %#v\nwant: %#v", gotGraph, wantGraph)
	}
	if len(gotProject.ManagedPackageDependencies) != 1 || gotProject.ManagedPackageDependencies[0].Namespace != "nextpkg" || gotProject.ManagedPackageDependencies[0].SourceRoot != dependencyRoot {
		t.Errorf("published dependency metadata = %#v, want nextpkg at %s", gotProject.ManagedPackageDependencies, dependencyRoot)
	}
	if len(gotIndex.Dependencies) != 1 || gotIndex.Dependencies[0].Namespace != "nextpkg" || gotIndex.Dependencies[0].SourceRoot != dependencyRoot {
		t.Errorf("published index dependency metadata = %#v, want nextpkg at %s", gotIndex.Dependencies, dependencyRoot)
	}
}

func TestDaemonWatchScopeSnapshotReturnsOwnedProjectScope(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	want := watch.ProjectScope(root, d.project)
	first := d.WatchScopeSnapshot(root)
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("watch scope = %#v, want %#v", first, want)
	}
	if len(first.Roots) == 0 {
		t.Fatalf("watch scope is incomplete: %#v", first)
	}
	first.Roots[0] = "poison-root"
	if len(first.Files) > 0 {
		first.Files[0] = "poison-file"
	}
	if second := d.WatchScopeSnapshot(root); !reflect.DeepEqual(second, want) {
		t.Fatalf("caller mutation changed daemon watch scope:\ngot:  %#v\nwant: %#v", second, want)
	}
}

func TestDaemonReadsRemainAvailableWhileUpdateBuildIsBlocked(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	testPath := filepath.Join(root, "force-app/main/default/classes/BlockedTest.cls")
	writeFile(t, testPath, `
@IsTest
private class BlockedTest {
  @IsTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the concurrency assertion about daemon state locks, not duplicate
	// cold runtime compilation under the race detector.
	d.Warm()

	updateStarted := make(chan struct{})
	releaseUpdate := make(chan struct{})
	d.tryUpdateIndexFn = func(previous typesys.Index, _, _ []string, _ project.Project) (typesys.Index, bool, error) {
		close(updateStarted)
		<-releaseUpdate
		candidate := previous
		candidate.Project.Namespace = "candidate"
		return candidate, true, nil
	}
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- d.UpdateChanges([]watch.Change{{
			Path: filepath.Join(root, "force-app/main/default/classes/Blocked.cls"),
			Op:   watch.ChangeModified,
			Kind: watch.FileKindApexClass,
			Name: "Blocked",
		}})
	}()
	<-updateStarted

	runDone := make(chan testreport.Run, 1)
	go func() {
		runDone <- d.RunSelectionContext(context.Background(), apextest.Options{NoDiskCache: true}, watch.TestSelection{
			Mode:        watch.SelectionDirect,
			TestClasses: []string{"BlockedTest"},
		})
	}()
	selectDone := make(chan watch.TestSelection, 1)
	go func() {
		selectDone <- d.SelectAffected([]watch.Change{{
			Path: testPath,
			Op:   watch.ChangeModified,
			Kind: watch.FileKindApexClass,
			Name: "BlockedTest",
		}})
	}()
	warmDone := make(chan struct{})
	go func() {
		d.Warm()
		close(warmDone)
	}()

	completed := 0
	deadline := time.After(10 * time.Second)
	for completed < 3 {
		select {
		case run := <-runDone:
			runDone = nil
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				close(releaseUpdate)
				<-updateDone
				t.Fatalf("run against published snapshot = %#v, want one pass", got)
			}
			completed++
		case selection := <-selectDone:
			selectDone = nil
			if selection.Mode != watch.SelectionDirect || !reflect.DeepEqual(selection.TestClasses, []string{"BlockedTest"}) {
				close(releaseUpdate)
				<-updateDone
				t.Fatalf("selection against published snapshot = %#v, want direct BlockedTest", selection)
			}
			completed++
		case <-warmDone:
			warmDone = nil
			completed++
		case <-deadline:
			close(releaseUpdate)
			<-updateDone
			t.Fatalf("daemon reads blocked behind candidate build: completed=%d/3", completed)
		}
	}
	d.mu.RLock()
	publishedBeforeRelease := d.index.Project.Namespace
	d.mu.RUnlock()
	if publishedBeforeRelease == "candidate" {
		close(releaseUpdate)
		<-updateDone
		t.Fatal("candidate index published before the blocked update was released")
	}
	close(releaseUpdate)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	publishedAfterRelease := d.index.Project.Namespace
	d.mu.RUnlock()
	if publishedAfterRelease != "candidate" {
		t.Fatalf("published namespace = %q, want candidate after update completion", publishedAfterRelease)
	}
}

func TestDaemonUpdateChangesMatchesCleanLoadAcrossLifecycle(t *testing.T) {
	type lifecycleCase struct {
		name   string
		mutate func(t *testing.T, root string) []watch.Change
	}
	cases := []lifecycleCase{
		{
			name: "class modify",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
				writeFile(t, path, "public class Helper { public static Integer value() { Integer unchanged = 1; return unchanged; } }")
				return []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
			},
		},
		{
			name: "class add",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/classes/Added.cls")
				writeFile(t, path, "public class Added { public static void run() {} }")
				return []watch.Change{{Path: path, Op: watch.ChangeAdded, Kind: watch.FileKindApexClass, Name: "Added"}}
			},
		},
		{
			name: "class delete",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/classes/Unused.cls")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				return []watch.Change{{Path: path, Op: watch.ChangeDeleted, Kind: watch.FileKindApexClass, Name: "Unused"}}
			},
		},
		{
			name: "class rename",
			mutate: func(t *testing.T, root string) []watch.Change {
				oldPath := filepath.Join(root, "force-app/main/default/classes/Unused.cls")
				newPath := filepath.Join(root, "force-app/main/default/classes/Renamed.cls")
				if err := os.Remove(oldPath); err != nil {
					t.Fatal(err)
				}
				writeFile(t, newPath, "public class Renamed { public static void run() {} }")
				return []watch.Change{
					{Path: oldPath, Op: watch.ChangeDeleted, Kind: watch.FileKindApexClass, Name: "Unused"},
					{Path: newPath, Op: watch.ChangeAdded, Kind: watch.FileKindApexClass, Name: "Renamed"},
				}
			},
		},
		{
			name: "trigger modify",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/triggers/SampleTrigger.trigger")
				writeFile(t, path, "trigger SampleTrigger on Account (before insert, before update) {}")
				return []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexTrigger, Name: "SampleTrigger"}}
			},
		},
		{
			name: "trigger add",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/triggers/AddedTrigger.trigger")
				writeFile(t, path, "trigger AddedTrigger on Contact (before insert) {}")
				return []watch.Change{{Path: path, Op: watch.ChangeAdded, Kind: watch.FileKindApexTrigger, Name: "AddedTrigger"}}
			},
		},
		{
			name: "trigger delete",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/triggers/SampleTrigger.trigger")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				return []watch.Change{{Path: path, Op: watch.ChangeDeleted, Kind: watch.FileKindApexTrigger, Name: "SampleTrigger"}}
			},
		},
		{
			name: "trigger rename",
			mutate: func(t *testing.T, root string) []watch.Change {
				oldPath := filepath.Join(root, "force-app/main/default/triggers/SampleTrigger.trigger")
				newPath := filepath.Join(root, "force-app/main/default/triggers/RenamedTrigger.trigger")
				if err := os.Remove(oldPath); err != nil {
					t.Fatal(err)
				}
				writeFile(t, newPath, "trigger RenamedTrigger on Account (before insert) {}")
				return []watch.Change{
					{Path: oldPath, Op: watch.ChangeDeleted, Kind: watch.FileKindApexTrigger, Name: "SampleTrigger"},
					{Path: newPath, Op: watch.ChangeAdded, Kind: watch.FileKindApexTrigger, Name: "RenamedTrigger"},
				}
			},
		},
		{
			name: "dependency trigger modify",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(filepath.Dir(root), "dependency/force-app/main/default/triggers/DependencyTrigger.trigger")
				writeFile(t, path, "trigger DependencyTrigger on Account (before insert, after update) {}")
				return []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexTrigger, Name: "DependencyTrigger"}}
			},
		},
		{
			name: "dependency trigger add",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(filepath.Dir(root), "dependency/force-app/main/default/triggers/AddedDependencyTrigger.trigger")
				writeFile(t, path, "trigger AddedDependencyTrigger on Contact (before insert) {}")
				return []watch.Change{{Path: path, Op: watch.ChangeAdded, Kind: watch.FileKindApexTrigger, Name: "AddedDependencyTrigger"}}
			},
		},
		{
			name: "dependency trigger delete",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(filepath.Dir(root), "dependency/force-app/main/default/triggers/DependencyTrigger.trigger")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				return []watch.Change{{Path: path, Op: watch.ChangeDeleted, Kind: watch.FileKindApexTrigger, Name: "DependencyTrigger"}}
			},
		},
		{
			name: "dependency trigger rename",
			mutate: func(t *testing.T, root string) []watch.Change {
				oldPath := filepath.Join(filepath.Dir(root), "dependency/force-app/main/default/triggers/DependencyTrigger.trigger")
				newPath := filepath.Join(filepath.Dir(root), "dependency/force-app/main/default/triggers/RenamedDependencyTrigger.trigger")
				if err := os.Remove(oldPath); err != nil {
					t.Fatal(err)
				}
				writeFile(t, newPath, "trigger RenamedDependencyTrigger on Account (before insert) {}")
				return []watch.Change{
					{Path: oldPath, Op: watch.ChangeDeleted, Kind: watch.FileKindApexTrigger, Name: "DependencyTrigger"},
					{Path: newPath, Op: watch.ChangeAdded, Kind: watch.FileKindApexTrigger, Name: "RenamedDependencyTrigger"},
				}
			},
		},
		{
			name: "project config fallback",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "sfdx-project.json")
				writeFile(t, path, `{"namespace":"lifecycle","packageDirectories":[{"path":"force-app","default":true}]}`)
				return []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindIgnored, Name: "sfdx-project.json"}}
			},
		},
		{
			name: "schema fallback",
			mutate: func(t *testing.T, root string) []watch.Change {
				path := filepath.Join(root, "force-app/main/default/objects/Lifecycle__c/Lifecycle__c.object-meta.xml")
				writeFile(t, path, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Lifecycle</label><pluralLabel>Lifecycles</pluralLabel><nameField><label>Lifecycle Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
				return []watch.Change{{Path: path, Op: watch.ChangeAdded, Kind: watch.FileKindObjectMeta, Name: "Lifecycle__c", ObjectName: "Lifecycle__c"}}
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := newDaemonLifecycleProject(t)
			d, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			changes := test.mutate(t, root)
			if err := d.UpdateChanges(changes); err != nil {
				t.Fatalf("UpdateChanges: %v", err)
			}

			wantIndex, err := loadIndex(root)
			if err != nil {
				t.Fatalf("clean load: %v", err)
			}
			wantGraph := watch.BuildReferenceGraph(wantIndex)
			d.mu.RLock()
			gotIndex := d.index
			gotGraph := d.graph
			d.mu.RUnlock()
			if !reflect.DeepEqual(gotIndex, wantIndex) {
				t.Errorf("daemon index differs from clean load:\ngot:  %#v\nwant: %#v", gotIndex, wantIndex)
			}
			if !reflect.DeepEqual(gotGraph, wantGraph) {
				t.Errorf("daemon graph differs from clean build:\ngot:  %#v\nwant: %#v", gotGraph, wantGraph)
			}

			gotSelection := d.SelectAffected(changes)
			wantSelection := watch.SelectAffectedTestsWithRefGraph(wantIndex, changes, wantGraph)
			if !reflect.DeepEqual(gotSelection, wantSelection) {
				t.Errorf("daemon selection differs from clean selection:\ngot:  %#v\nwant: %#v", gotSelection, wantSelection)
			}

			opts := apextest.Options{NoDiskCache: true}
			gotRun := canonicalDaemonRun(d.RunSelectionContext(context.Background(), opts, gotSelection))
			wantOpts, runnable := watch.ApplyTestSelection(opts, wantSelection)
			wantRun := testreport.Run{Name: "glade test"}
			if runnable {
				wantRun = apextest.Run(wantIndex, wantOpts)
			}
			wantRun = canonicalDaemonRun(wantRun)
			if !reflect.DeepEqual(gotRun, wantRun) {
				t.Errorf("daemon run differs from clean run:\ngot:  %#v\nwant: %#v", gotRun, wantRun)
			}
		})
	}
}

func TestDaemonParseDiagnosticAndRepairMatchesCleanLoad(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	change := []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
	steps := []struct {
		name       string
		source     string
		wantErrors bool
	}{
		{name: "parse diagnostic", source: "public class Helper {", wantErrors: true},
		{name: "repair", source: "public class Helper { public static Integer value() { return 1; } }"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			writeFile(t, path, step.source)
			if err := d.UpdateChanges(change); err != nil {
				t.Fatalf("UpdateChanges: %v", err)
			}
			wantIndex, err := loadIndex(root)
			if err != nil {
				t.Fatalf("clean load: %v", err)
			}
			gotIndex, gotSelection := d.SnapshotSelection(change)
			if !reflect.DeepEqual(gotIndex, wantIndex) {
				t.Errorf("daemon index differs from clean load:\ngot:  %#v\nwant: %#v", gotIndex, wantIndex)
			}
			if !reflect.DeepEqual(gotIndex.Diagnostics, wantIndex.Diagnostics) {
				t.Errorf("diagnostic order differs:\ngot:  %#v\nwant: %#v", gotIndex.Diagnostics, wantIndex.Diagnostics)
			}
			if gotErrors := len(gotIndex.Diagnostics) > 0; gotErrors != step.wantErrors {
				t.Errorf("diagnostics = %#v, want errors=%t", gotIndex.Diagnostics, step.wantErrors)
			}
			wantGraph := watch.BuildReferenceGraph(wantIndex)
			d.mu.RLock()
			gotGraph := d.graph
			d.mu.RUnlock()
			if !reflect.DeepEqual(gotGraph, wantGraph) {
				t.Errorf("daemon graph differs from clean build:\ngot:  %#v\nwant: %#v", gotGraph, wantGraph)
			}
			wantSelection := watch.SelectAffectedTestsWithRefGraph(wantIndex, change, wantGraph)
			if !reflect.DeepEqual(gotSelection, wantSelection) {
				t.Errorf("selection differs:\ngot:  %#v\nwant: %#v", gotSelection, wantSelection)
			}
			opts := apextest.Options{NoDiskCache: true}
			gotRun := canonicalDaemonRun(d.RunSelectionContext(context.Background(), opts, gotSelection))
			wantOpts, runnable := watch.ApplyTestSelection(opts, wantSelection)
			wantRun := testreport.Run{Name: "glade test"}
			if runnable {
				wantRun = apextest.Run(wantIndex, wantOpts)
			}
			if wantRun = canonicalDaemonRun(wantRun); !reflect.DeepEqual(gotRun, wantRun) {
				t.Errorf("run differs:\ngot:  %#v\nwant: %#v", gotRun, wantRun)
			}
		})
	}
}

func TestDaemonSnapshotSelectionKeepsSelectionPairedWithPublishedIndex(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	helperChange := []watch.Change{{Path: helperPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
	snapshot, selection := d.SnapshotSelection(helperChange)
	if selection.Mode != watch.SelectionDirect || !reflect.DeepEqual(selection.TestClasses, []string{"ServiceTest"}) {
		t.Fatalf("snapshot selection = %#v, want direct ServiceTest", selection)
	}

	oldTestPath := filepath.Join(root, "force-app/main/default/classes/ServiceTest.cls")
	newTestPath := filepath.Join(root, "force-app/main/default/classes/NewServiceTest.cls")
	if err := os.Remove(oldTestPath); err != nil {
		t.Fatal(err)
	}
	writeFile(t, newTestPath, `
@IsTest
private class NewServiceTest {
  @IsTest static void passes() { System.assertEquals(1, Helper.value()); }
}
`)
	if err := d.UpdateChanges([]watch.Change{
		{Path: oldTestPath, Op: watch.ChangeDeleted, Kind: watch.FileKindApexClass, Name: "ServiceTest"},
		{Path: newTestPath, Op: watch.ChangeAdded, Kind: watch.FileKindApexClass, Name: "NewServiceTest"},
	}); err != nil {
		t.Fatal(err)
	}
	_, currentSelection := d.SnapshotSelection(helperChange)
	if currentSelection.Mode != watch.SelectionDirect || !reflect.DeepEqual(currentSelection.TestClasses, []string{"NewServiceTest"}) {
		t.Fatalf("current selection = %#v, want direct NewServiceTest", currentSelection)
	}

	opts, ok := watch.ApplyTestSelection(apextest.Options{NoDiskCache: true}, selection)
	if !ok {
		t.Fatal("captured selection unexpectedly became unrunnable")
	}
	run := apextest.Run(snapshot, opts)
	if !runHasClass(run, "ServiceTest") || runHasClass(run, "NewServiceTest") {
		t.Fatalf("captured run used a later index: %#v", run)
	}
}

func TestDaemonReloadAndUpdateChangesSerializeWriters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	d.mu.RLock()
	updated := d.index
	reloaded := d.index
	reloadedProject := d.project
	updateProject := d.project
	d.mu.RUnlock()
	updated.Project.Namespace = "updated"
	reloaded.Project.Namespace = "reloaded"
	reloadedProject.Namespace = "reloaded"

	updateStarted := make(chan struct{})
	releaseUpdate := make(chan struct{})
	d.tryUpdateIndexFn = func(typesys.Index, []string, []string, project.Project) (typesys.Index, bool, error) {
		close(updateStarted)
		<-releaseUpdate
		return updated, true, nil
	}
	reloadStarted := make(chan struct{})
	loads := 0
	d.loadProjectFn = func(string) (project.Project, error) {
		loads++
		if loads == 1 {
			return updateProject, nil
		}
		close(reloadStarted)
		return reloadedProject, nil
	}
	d.buildIndexFn = func(got project.Project) (typesys.Index, error) {
		if !reflect.DeepEqual(got, reloadedProject) {
			t.Errorf("index builder project = %#v, want %#v", got, reloadedProject)
		}
		return reloaded, nil
	}

	updateDone := make(chan error, 1)
	go func() {
		updateDone <- d.UpdateChanges([]watch.Change{{
			Path: filepath.Join(root, "force-app/main/default/classes/Changed.cls"),
			Op:   watch.ChangeModified,
			Kind: watch.FileKindApexClass,
			Name: "Changed",
		}})
	}()
	<-updateStarted
	reloadDone := make(chan error, 1)
	go func() { reloadDone <- d.Reload() }()

	reloadStartedEarly := false
	select {
	case <-reloadStarted:
		reloadStartedEarly = true
	case <-time.After(200 * time.Millisecond):
	}
	close(releaseUpdate)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if !reloadStartedEarly {
		select {
		case <-reloadStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("serialized reload did not start after update completed")
		}
	}
	if err := <-reloadDone; err != nil {
		t.Fatal(err)
	}
	if reloadStartedEarly {
		t.Error("Reload entered its load operation while UpdateChanges was still active")
	}

	d.mu.RLock()
	finalProject := d.project
	finalIndex := d.index
	finalGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(finalProject, reloadedProject) {
		t.Errorf("last serialized writer did not publish paired project:\nfinal: %#v\nwant reload: %#v", finalProject, reloadedProject)
	}
	if !reflect.DeepEqual(finalIndex, reloaded) {
		t.Errorf("last serialized writer did not win:\nfinal: %#v\nwant reload: %#v", finalIndex, reloaded)
	}
	if wantGraph := watch.BuildReferenceGraph(reloaded); !reflect.DeepEqual(finalGraph, wantGraph) {
		t.Errorf("final graph does not match last serialized index:\nfinal: %#v\nwant: %#v", finalGraph, wantGraph)
	}
}

func TestDaemonReloadPreparedStableUsesOneProjectForScopeBuildAndPublication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	d.mu.RLock()
	beforeProject := d.project
	beforeIndex := d.index
	beforeGraph := d.graph
	candidate := d.project
	d.mu.RUnlock()
	candidate.Namespace = "prepared"

	loadCalls := 0
	d.loadProjectFn = func(string) (project.Project, error) {
		loadCalls++
		return candidate, nil
	}
	buildStarted := make(chan project.Project, 1)
	releaseBuild := make(chan struct{})
	d.buildIndexFn = func(got project.Project) (typesys.Index, error) {
		buildStarted <- got
		<-releaseBuild
		return buildProjectIndex(got)
	}
	prepared := make(chan project.Project, 1)
	reloadDone := make(chan error, 1)
	go func() {
		reloadDone <- d.ReloadPreparedStable(watch.Scope{}, watch.CaptureScope, func(got project.Project, _ watch.Scope, _ watch.Snapshot) error {
			prepared <- got
			return nil
		})
	}()

	preparedProject := <-prepared
	builtProject := <-buildStarted
	if loadCalls != 2 {
		t.Fatalf("project load calls = %d, want 2 matching scope and authoritative loads", loadCalls)
	}
	if !reflect.DeepEqual(preparedProject, candidate) {
		t.Fatalf("prepared project = %#v, want %#v", preparedProject, candidate)
	}
	if !reflect.DeepEqual(builtProject, preparedProject) {
		t.Fatalf("built project differs from prepared project:\nbuilt: %#v\nprepared: %#v", builtProject, preparedProject)
	}

	d.mu.RLock()
	duringProject := d.project
	duringIndex := d.index
	duringGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(duringProject, beforeProject) || !reflect.DeepEqual(duringIndex, beforeIndex) || duringGraph != beforeGraph {
		t.Fatalf("reload published state before index build completed:\nproject: %#v\nindex: %#v\ngraph: %p", duringProject, duringIndex, duringGraph)
	}

	close(releaseBuild)
	if err := <-reloadDone; err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	afterProject := d.project
	afterIndex := d.index
	afterGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(afterProject, candidate) {
		t.Fatalf("published project = %#v, want %#v", afterProject, candidate)
	}
	if afterIndex.Project.Namespace != candidate.Namespace {
		t.Fatalf("published index namespace = %q, want %q", afterIndex.Project.Namespace, candidate.Namespace)
	}
	if afterGraph == beforeGraph || !reflect.DeepEqual(afterGraph, watch.BuildReferenceGraph(afterIndex)) {
		t.Fatalf("published graph is not freshly paired with index: %#v", afterGraph)
	}
}

func TestDaemonReloadPreparedStableRejectsInputDriftBeforePublication(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Stable { public void beforeEdit() {} }")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	beforeProject := d.project
	beforeIndex := d.index
	beforeGraph := d.graph
	d.mu.RUnlock()

	err = d.ReloadPreparedStable(watch.Scope{}, watch.CaptureScope, func(_ project.Project, _ watch.Scope, _ watch.Snapshot) error {
		writeFile(t, classPath, "public class Stable { public void afterEdit() {} }")
		return nil
	})
	if err == nil {
		t.Fatal("stable prepared reload published after its watcher baseline drifted")
	}
	var drift *WatchStateDriftError
	if !errors.As(err, &drift) {
		t.Fatalf("stable prepared reload error = %T %v, want WatchStateDriftError", err, err)
	}
	d.mu.RLock()
	afterProject := d.project
	afterIndex := d.index
	afterGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(afterProject, beforeProject) || !reflect.DeepEqual(afterIndex, beforeIndex) || afterGraph != beforeGraph {
		t.Fatalf("drifted stable reload published state:\nproject=%#v\nindex=%#v\ngraph=%p", afterProject, afterIndex, afterGraph)
	}
}

func TestDaemonRunsFilterAgainstWarmProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)

	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	first := daemon.RunFilter("WarmOneTest")
	if got := first.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("first summary = %#v", got)
	}
	second := daemon.RunFilter("WarmTwoTest")
	if got := second.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("second summary = %#v", got)
	}
}

func TestDaemonConcurrentRunsKeepOneGenerationPairedInternally(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	helperPath := filepath.Join(root, "force-app/main/default/classes/GenerationHelper.cls")
	testPath := filepath.Join(root, "force-app/main/default/classes/GenerationTest.cls")
	writeFile(t, helperPath, `public class GenerationHelper { public static Integer value() { return 1; } }`)
	writeFile(t, testPath, `@IsTest private class GenerationTest { @IsTest static void passes() { System.assertEquals(1, GenerationHelper.value()); } }`)

	projectA := project.Project{Root: root, Namespace: "generation_a", ApexFiles: []string{helperPath, testPath}}
	indexA, artifactsA := typesys.BuildWithArtifacts(projectA, schema.Schema{})
	writeFile(t, helperPath, `public class GenerationHelper { public static Integer value() { return 2; } }`)
	projectB := project.Project{Root: root, Namespace: "generation_b", ApexFiles: []string{helperPath, testPath}}
	indexB, artifactsB := typesys.BuildWithArtifacts(projectB, schema.Schema{})

	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	change := []watch.Change{{Path: helperPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "GenerationHelper"}}
	newGeneration := func(serial uint64, p project.Project, index typesys.Index, artifacts typesys.BuildArtifacts) daemonGeneration {
		return daemonGeneration{serial: serial, project: p, index: index, artifacts: artifacts, graph: watch.BuildReferenceGraph(index)}
	}
	generationA := newGeneration(1, projectA, indexA, artifactsA)
	generationB := newGeneration(2, projectB, indexB, artifactsB)
	d.publishGeneration(generationA)

	var assertionErr error
	var assertionMu sync.Mutex
	assertGeneration := func(generation daemonGeneration, opts apextest.Options) testreport.Run {
		assertionMu.Lock()
		defer assertionMu.Unlock()
		if assertionErr != nil {
			return testreport.Run{}
		}
		if generation.serial == 0 || generation.project.Namespace != generation.index.Project.Namespace {
			assertionErr = fmt.Errorf("serial/project/index mismatch: serial=%d project=%q index=%q", generation.serial, generation.project.Namespace, generation.index.Project.Namespace)
			return testreport.Run{}
		}
		if opts.BuildArtifacts == nil || opts.BuildArtifacts.SourceDigests != generation.artifacts.SourceDigests {
			assertionErr = fmt.Errorf("serial %d received artifacts from another generation", generation.serial)
			return testreport.Run{}
		}
		var helper typesys.TypeSymbol
		for _, typ := range generation.index.Types {
			if typ.Name == "GenerationHelper" {
				helper = typ
				break
			}
		}
		expectedDigest, hasDigest := generation.artifacts.SourceDigests.Digest(helperPath)
		if source, ok := generation.artifacts.SourceForType(helper); !ok || !hasDigest || source.Digest() != expectedDigest {
			assertionErr = fmt.Errorf("serial %d index/artifacts mismatch", generation.serial)
			return testreport.Run{}
		}
		selection := watch.SelectAffectedTestsWithRefGraph(generation.index, change, generation.graph)
		if selection.Mode != watch.SelectionDirect || !reflect.DeepEqual(selection.TestClasses, []string{"GenerationTest"}) {
			assertionErr = fmt.Errorf("serial %d index/graph selection = %#v", generation.serial, selection)
			return testreport.Run{}
		}
		if !reflect.DeepEqual(opts.SelectedClasses, []string{"GenerationTest"}) {
			assertionErr = fmt.Errorf("serial %d result selection = %#v", generation.serial, opts.SelectedClasses)
			return testreport.Run{}
		}
		return testreport.Run{Name: fmt.Sprintf("generation-%d-%s", generation.serial, generation.project.Namespace)}
	}
	d.runGenerationFn = func(_ context.Context, generation daemonGeneration, opts apextest.Options) testreport.Run {
		return assertGeneration(generation, opts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for iteration := 0; iteration < 100; iteration++ {
				run := d.RunSelectionContext(ctx, apextest.Options{NoDiskCache: true}, watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"GenerationTest"}})
				if !strings.HasPrefix(run.Name, "generation-") {
					assertionMu.Lock()
					if assertionErr == nil {
						assertionErr = fmt.Errorf("run did not retain its captured generation: %#v", run)
					}
					assertionMu.Unlock()
					return
				}
			}
		}()
	}
	for serial := uint64(3); serial < 303; serial++ {
		if serial%2 == 0 {
			generation := generationA
			generation.serial = serial
			d.publishGeneration(generation)
		} else {
			generation := generationB
			generation.serial = serial
			d.publishGeneration(generation)
		}
	}
	workers.Wait()
	assertionMu.Lock()
	err = assertionErr
	assertionMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDaemonWarmBlocksGenerationPublicationUntilCapturedGenerationCompletes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/classes/WarmTest.cls")
	writeFile(t, path, "@IsTest private class WarmTest { @IsTest static void passes() {} }")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	d.warmRuntimeFn = func(context.Context, typesys.Index, *typesys.BuildArtifacts) error {
		close(started)
		<-release
		return nil
	}
	warmDone := make(chan struct{})
	go func() {
		d.Warm()
		close(warmDone)
	}()
	<-started

	writeFile(t, path, "@IsTest private class WarmTest { @IsTest static void changed() {} }")
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- d.UpdateChanges([]watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "WarmTest"}})
	}()
	select {
	case err := <-updateDone:
		t.Fatalf("UpdateChanges published while the captured warm generation was active: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	<-warmDone
	if err := <-updateDone; err != nil {
		t.Fatalf("UpdateChanges() = %v", err)
	}
}

func TestDaemonRunSelectionNarrowsMultipleDirectClasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmThreeTest.cls"), `
@isTest
private class WarmThreeTest {
  @isTest static void passes() { System.assertEquals(4, 2 + 2); }
}
`)

	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	run := daemon.RunSelectionContext(context.Background(), apextest.Options{}, watch.TestSelection{
		Mode:        watch.SelectionDirect,
		TestClasses: []string{"WarmOneTest", "WarmTwoTest"},
	})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	if runHasClass(run, "WarmThreeTest") {
		t.Fatalf("run included unselected class WarmThreeTest: %#v", run)
	}
}

func newDaemonLifecycleProject(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n  managedPackageDependencies: [\"stagepkg:../dependency:1.0\"]\n")
	writeFile(t, filepath.Join(parent, "dependency/sfdx-project.json"), `{"namespace":"BasePkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(parent, "dependency/force-app/main/default/triggers/DependencyTrigger.trigger"), "trigger DependencyTrigger on Account (before insert) {}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Helper.cls"), `
public class Helper {
  public static Integer value() { return 1; }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Unused.cls"), `
public class Unused {
  public static void run() {}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ServiceTest.cls"), `
@IsTest
private class ServiceTest {
  @IsTest static void passes() { System.assertEquals(1, Helper.value()); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OtherTest.cls"), `
@IsTest
private class OtherTest {
  @IsTest static void passes() { System.assert(true); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/SampleTrigger.trigger"), "trigger SampleTrigger on Account (before insert) {}")
	return root
}

func canonicalDaemonRun(run testreport.Run) testreport.Run {
	run.DurationMS = 0
	for suiteIndex := range run.Suites {
		run.Suites[suiteIndex].DurationMS = 0
		for caseIndex := range run.Suites[suiteIndex].Cases {
			run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
		}
	}
	return run
}

func runHasClass(run testreport.Run, className string) bool {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.ClassName == className {
				return true
			}
		}
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
