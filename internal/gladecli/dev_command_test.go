package gladecli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

func TestWarningBearingLocalApexEditRebuildsOnceWithoutReplacement(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	testPath := filepath.Join(root, "force-app/main/default/classes/ServiceTest.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Service { public static Integer value() { return 1; } }")
	writeTestFile(t, testPath, "@IsTest private class ServiceTest { @IsTest static void testValue() { System.assertEquals(1, Service.value()); } }")
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Code: "TEST", Message: "warning"})
	graph := watch.BuildReferenceGraph(index)
	beforeGraph := watch.BuildReferenceGraph(index)
	scope := watch.ProjectScope(root, p)
	writeTestFile(t, classPath, "public class Service { public static Integer value() { return 2; } }")
	changes := []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Service"}}
	loads, builds, captures, refreshes := 0, 0, 0, 0
	update, err := tryUpdateWatchIndexStateWithGraphRefresh(root, scope, index, graph, changes,
		func(root string) (project.Project, error) { loads++; return project.Load(root) },
		func(p project.Project) (typesys.Index, error) { builds++; return buildProjectIndex(p), nil },
		func(scope watch.Scope) (watch.Snapshot, error) { captures++; return watch.CaptureScope(scope) },
		func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
			refreshes++
			return graph.Refreshed(index, changes), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !update.reusable || loads != 1 || builds != 1 || captures != 2 || refreshes != 1 {
		t.Fatalf("fallback = reusable:%t loads:%d builds:%d captures:%d refreshes:%d, want true/1/1/2/1", update.reusable, loads, builds, captures, refreshes)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(update.index, fresh) || !reflect.DeepEqual(update.graph, watch.BuildReferenceGraph(fresh)) {
		t.Fatal("same-scope fallback differs from a clean load")
	}
	if update.graph == graph || !reflect.DeepEqual(graph, beforeGraph) {
		t.Fatal("local fallback mutated the previously published graph")
	}
	gotSelection := watch.SelectAffectedTestsWithRefGraph(update.index, changes, update.graph)
	wantSelection := watch.SelectAffectedTestsWithRefGraph(fresh, changes, watch.BuildReferenceGraph(fresh))
	if !reflect.DeepEqual(gotSelection, wantSelection) || gotSelection.Mode != watch.SelectionDirect {
		t.Fatalf("selection = %#v, want direct %#v", gotSelection, wantSelection)
	}
}

func TestDuplicateCanonicalNameWarningForcesFullFallbackGraph(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "force-app/main/default/classes/Duplicate.cls")
	secondPath := filepath.Join(root, "other-app/main/default/classes/Duplicate.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true},{"path":"other-app"}]}`)
	writeTestFile(t, firstPath, "public class Duplicate { public static Integer value() { return 1; } }")
	writeTestFile(t, secondPath, "public class Duplicate { public static Integer value() { return 2; } }")
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	index.Diagnostics = []diagnostic.Diagnostic{{Severity: diagnostic.Warning, Message: "duplicate canonical name"}}
	graph := watch.BuildReferenceGraph(index)
	scope := watch.ProjectScope(root, p)
	writeTestFile(t, firstPath, "public class Duplicate { public static Integer value() { return 3; } }")
	changes := []watch.Change{{Path: firstPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Duplicate"}}
	refreshes := 0
	update, err := tryUpdateWatchIndexStateWithGraphRefresh(root, scope, index, graph, changes, project.Load,
		func(p project.Project) (typesys.Index, error) { return buildProjectIndex(p), nil }, watch.CaptureScope,
		func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
			refreshes++
			return graph.Refreshed(index, changes), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !update.reusable || refreshes != 0 || !reflect.DeepEqual(update.graph, watch.BuildReferenceGraph(fresh)) {
		t.Fatalf("duplicate fallback = reusable:%t refreshes:%d", update.reusable, refreshes)
	}
}

func TestLocalLateFallbackReloadsInsideProofWindow(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Service.cls")
	secondPath := filepath.Join(root, "force-app/main/default/classes/Second.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Service {}")
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := watch.BuildReferenceGraph(index)
	scope := watch.ProjectScope(root, p)
	changes := []watch.Change{{Path: classPath, Op: watch.ChangeDeleted, Kind: watch.FileKindApexClass, Name: "Service"}}
	loads, builds, captures, refreshes := 0, 0, 0, 0
	update, err := tryUpdateWatchIndexStateWithGraphRefresh(root, scope, index, graph, changes,
		func(root string) (project.Project, error) {
			loads++
			loaded, loadErr := project.Load(root)
			if loads == 1 {
				writeTestFile(t, secondPath, "public class Second {}")
			}
			return loaded, loadErr
		},
		func(p project.Project) (typesys.Index, error) { builds++; return buildProjectIndex(p), nil },
		func(scope watch.Scope) (watch.Snapshot, error) { captures++; return watch.CaptureScope(scope) },
		func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
			refreshes++
			return graph.Refreshed(index, changes), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !update.reusable || loads != 2 || builds != 1 || captures != 2 || refreshes != 0 {
		t.Fatalf("late fallback = reusable:%t loads:%d builds:%d captures:%d refreshes:%d, want true/2/1/2/0", update.reusable, loads, builds, captures, refreshes)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(update.index, fresh) || !reflect.DeepEqual(update.graph, watch.BuildReferenceGraph(fresh)) {
		t.Fatal("late fallback published the project loaded before its proof baseline")
	}
}

func TestLocalFallbackProofDriftAndBuildErrorRetainOldState(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	otherPath := filepath.Join(root, "force-app/main/default/classes/Other.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Stable {}")
	writeTestFile(t, otherPath, "public class Other {}")
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Message: "warning"})
	graph := watch.BuildReferenceGraph(index)
	scope := watch.ProjectScope(root, p)
	changes := []watch.Change{{Path: classPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Stable"}}
	writeTestFile(t, classPath, "public class Stable { public void originalChange() {} }")
	buildErr := errors.New("build failed")
	update, err := tryUpdateWatchIndexStateWithFuncs(root, scope, index, graph, changes, project.Load,
		func(project.Project) (typesys.Index, error) { return typesys.Index{}, buildErr }, watch.CaptureScope)
	if !errors.Is(err, buildErr) || !reflect.DeepEqual(update.index, index) || update.graph != graph {
		t.Fatalf("build failure published state: update=%#v err=%v", update, err)
	}
	captureErr := errors.New("capture failed")
	update, err = tryUpdateWatchIndexStateWithFuncs(root, scope, index, graph, changes, project.Load,
		func(p project.Project) (typesys.Index, error) { return buildProjectIndex(p), nil },
		func(watch.Scope) (watch.Snapshot, error) { return watch.Snapshot{}, captureErr })
	if !errors.Is(err, captureErr) || !reflect.DeepEqual(update.index, index) || update.graph != graph {
		t.Fatalf("capture failure published state: update=%#v err=%v", update, err)
	}
	refreshErr := errors.New("refresh failed")
	update, err = tryUpdateWatchIndexStateWithGraphRefresh(root, scope, index, graph, changes, project.Load,
		func(p project.Project) (typesys.Index, error) { return buildProjectIndex(p), nil }, watch.CaptureScope,
		func(*watch.RefGraph, typesys.Index, []watch.Change) (*watch.RefGraph, error) { return nil, refreshErr })
	if !errors.Is(err, refreshErr) || !reflect.DeepEqual(update.index, index) || update.graph != graph {
		t.Fatalf("refresh failure published state: update=%#v err=%v", update, err)
	}
	beforeGraph := watch.BuildReferenceGraph(index)
	refreshes := 0
	refreshGraph := func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		refreshes++
		candidate := graph.Refreshed(index, changes)
		if refreshes == 1 {
			writeTestFile(t, otherPath, "public class Other { public void changedDuringGraphRefresh() {} }")
		}
		return candidate, nil
	}
	update, err = tryUpdateWatchIndexStateWithGraphRefresh(root, scope, index, graph, changes, project.Load,
		func(p project.Project) (typesys.Index, error) { return buildProjectIndex(p), nil },
		watch.CaptureScope,
		refreshGraph)
	var drift *testdaemon.WatchStateDriftError
	if !errors.As(err, &drift) || !reflect.DeepEqual(update.index, index) || update.graph != graph || !reflect.DeepEqual(graph, beforeGraph) {
		t.Fatalf("proof drift published state: update=%#v err=%v", update, err)
	}
	retry, err := tryUpdateWatchIndexStateWithGraphRefreshAllowed(root, scope, index, graph, changes, project.Load,
		func(p project.Project) (typesys.Index, error) { return buildProjectIndex(p), nil },
		watch.CaptureScope, refreshGraph, false)
	if err != nil || !retry.reusable || refreshes != 1 {
		t.Fatalf("drift retry = reusable:%t refreshes:%d err:%v, want true/1/nil", retry.reusable, refreshes, err)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retry.graph, watch.BuildReferenceGraph(fresh)) || !reflect.DeepEqual(graph, beforeGraph) {
		t.Fatal("drift retry did not force an immutable full graph rebuild")
	}
}

func TestPrepareLocalWatchReplacementRejectsGraphBuildDrift(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Stable {}")
	p, _, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := watch.Config{Root: root, Backend: watch.BackendPoll, Debounce: 10 * time.Millisecond, Scope: watch.ProjectScope(root, p)}.Normalized()
	replacement, err := prepareLocalWatchReplacementFromProjectWithGraphBuilder(ctx, root, cfg, watch.BackendPoll, p, watch.CaptureScope, func(index typesys.Index) *watch.RefGraph {
		graph := watch.BuildReferenceGraph(index)
		writeTestFile(t, classPath, "public class Stable { public void changedDuringGraphBuild() {} }")
		return graph
	})
	if replacement.watcher != nil {
		_ = replacement.watcher.Close()
	}
	var drift *testdaemon.WatchStateDriftError
	if !errors.As(err, &drift) {
		t.Fatalf("replacement graph build drift = %v, want WatchStateDriftError", err)
	}
}

func TestRunSelectedTestsDirectMultipleClasses(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, className := range []string{"AlphaTest", "AlphaTestExtra", "BetaTest", "UnrelatedTest"} {
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes", className+".cls"), `
@IsTest
private class `+className+` {
  @IsTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	}
	index, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	run := runSelectedTests(index, apextest.Options{}, watch.TestSelection{
		Mode:        watch.SelectionDirect,
		TestClasses: []string{"betatest", "AlphaTest", "BetaTest", "  "},
	})
	var classes []string
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			classes = append(classes, testCase.ClassName)
		}
	}
	sort.Strings(classes)
	if want := []string{"AlphaTest", "BetaTest"}; !reflect.DeepEqual(classes, want) {
		t.Fatalf("executed classes = %#v, want %#v", classes, want)
	}
}

func TestPrepareDaemonWatchRunKeepsSelectionPairedWithSnapshot(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	bPath := filepath.Join(root, "force-app/main/default/classes/BTest.cls")
	writeTestFile(t, bPath, `
@IsTest private class BTest {
  @IsTest static void passes() { System.assert(true); }
}`)
	writeTestFile(t, filepath.Join(root, "c-app/main/default/classes/CTest.cls"), `
@IsTest private class CTest {
  @IsTest static void passes() { System.assert(true); }
}`)
	daemon, err := testdaemon.New(root)
	if err != nil {
		t.Fatal(err)
	}
	selection, starter := prepareDaemonWatchRun(context.Background(), daemon, apextest.Options{NoDiskCache: true}, []watch.Change{{
		Path: bPath,
		Op:   watch.ChangeModified,
		Kind: watch.FileKindApexClass,
		Name: "BTest",
	}})
	if selection.Mode != watch.SelectionDirect || !reflect.DeepEqual(selection.TestClasses, []string{"BTest"}) {
		t.Fatalf("prepared selection = %#v, want direct BTest", selection)
	}

	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"c-app","default":true}]}`)
	if err := daemon.UpdateChanges([]watch.Change{{Path: manifestPath, Op: watch.ChangeModified, Kind: watch.FileKindIgnored, Name: "sfdx-project.json"}}); err != nil {
		t.Fatal(err)
	}
	_, done := starter(7, selection)
	finished := <-done
	var classes []string
	for _, suite := range finished.Result.Suites {
		for _, testCase := range suite.Cases {
			classes = append(classes, testCase.ClassName)
		}
	}
	if !reflect.DeepEqual(classes, []string{"BTest"}) {
		t.Fatalf("prepared run used later daemon index: classes=%#v result=%#v", classes, finished.Result)
	}
}

func TestRunWatchTestsInitialRunRebuildsAfterWatcherBaseline(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/InitialTest.cls")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, `
@IsTest private class InitialTest {
  @IsTest static void stale() { System.assert(false); }
}`)
	p, staleIndex, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, classPath, `
@IsTest private class InitialTest {
  @IsTest static void fresh() { System.assert(true); }
}`)

	result, err := runWatchTests(context.Background(), root, p, staleIndex, apextest.Options{NoDiskCache: true}, watch.Config{Root: root, Backend: watch.BackendPoll}, true, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("initial local watch run used stale pre-watch index: summary=%#v result=%#v", summary, result)
	}
	if got := result.Suites[0].Cases[0].MethodName; got != "fresh" {
		t.Fatalf("initial local watch method = %q, want fresh", got)
	}
}

func TestRunWatchTestsDaemonInitialRunRebuildsAfterWatcherBaseline(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/InitialTest.cls")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, `
@IsTest private class InitialTest {
  @IsTest static void stale() { System.assert(false); }
}`)
	daemon, err := testdaemon.New(root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, classPath, `
@IsTest private class InitialTest {
  @IsTest static void fresh() { System.assert(true); }
}`)

	result, err := runWatchTestsDaemon(context.Background(), root, daemon, apextest.Options{NoDiskCache: true}, watch.Config{Root: root, Backend: watch.BackendPoll}, true, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("initial daemon watch run used stale pre-watch index: summary=%#v result=%#v", summary, result)
	}
	if got := result.Suites[0].Cases[0].MethodName; got != "fresh" {
		t.Fatalf("initial daemon watch method = %q, want fresh", got)
	}
}

func TestPrepareInitialLocalWatchRetriesOrdinaryDrift(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	captures := 0
	capture := func(scope watch.Scope) (watch.Snapshot, error) {
		snapshot, err := watch.CaptureScope(scope)
		captures++
		if captures == 1 {
			writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"other-app","default":true}]}`)
		}
		return snapshot, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	replacement, err := prepareInitialLocalWatchReplacement(ctx, root, watch.Config{Root: root, Backend: watch.BackendPoll}, watch.BackendPoll, capture)
	if err != nil {
		t.Fatalf("ordinary startup drift was not retried: %v", err)
	}
	defer replacement.watcher.Close()
	if captures != 4 {
		t.Fatalf("startup captures = %d, want failed baseline/proof plus stable baseline/proof", captures)
	}
	if got := replacement.project.PackageDirectories[0].Path; got != "other-app" {
		t.Fatalf("stable startup project path = %q, want other-app", got)
	}
}

func TestPrepareInitialDaemonWatchRetriesOrdinaryDrift(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	daemon, err := testdaemon.New(root)
	if err != nil {
		t.Fatal(err)
	}
	captures := 0
	capture := func(scope watch.Scope) (watch.Snapshot, error) {
		snapshot, captureErr := watch.CaptureScope(scope)
		captures++
		if captures == 1 {
			writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"other-app","default":true}]}`)
		}
		return snapshot, captureErr
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, _, err := prepareInitialDaemonWatch(ctx, daemon, watch.Config{Root: root, Backend: watch.BackendPoll}, watch.BackendPoll, capture)
	if err != nil {
		t.Fatalf("ordinary daemon startup drift was not retried: %v", err)
	}
	defer watcher.Close()
	if captures != 4 {
		t.Fatalf("daemon startup captures = %d, want failed baseline/proof plus stable baseline/proof", captures)
	}
	if got := daemon.IndexSnapshot().Project.Root; got == "" {
		t.Fatal("stable daemon startup did not publish an index")
	}
}

func TestPrepareInitialWatchRetriesRespectContextAndHardErrors(t *testing.T) {
	for _, daemonMode := range []bool{false, true} {
		name := "local"
		if daemonMode {
			name = "daemon"
		}
		t.Run(name+"/context", func(t *testing.T) {
			root := t.TempDir()
			manifestPath := filepath.Join(root, "sfdx-project.json")
			writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			ctx, cancel := context.WithCancel(context.Background())
			captures := 0
			capture := func(scope watch.Scope) (watch.Snapshot, error) {
				snapshot, err := watch.CaptureScope(scope)
				captures++
				if captures == 1 {
					writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"other-app","default":true}]}`)
				}
				if captures == 2 {
					cancel()
				}
				return snapshot, err
			}
			var err error
			if daemonMode {
				d, daemonErr := testdaemon.New(root)
				if daemonErr != nil {
					t.Fatal(daemonErr)
				}
				_, _, err = prepareInitialDaemonWatch(ctx, d, watch.Config{Root: root, Backend: watch.BackendPoll}, watch.BackendPoll, capture)
			} else {
				_, err = prepareInitialLocalWatchReplacement(ctx, root, watch.Config{Root: root, Backend: watch.BackendPoll}, watch.BackendPoll, capture)
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled startup error = %v, want context.Canceled", err)
			}
			if captures != 2 {
				t.Fatalf("canceled startup captures = %d, want one failed transaction", captures)
			}
		})

		t.Run(name+"/hard-error", func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			marker := errors.New("capture failed")
			captures := 0
			capture := func(watch.Scope) (watch.Snapshot, error) {
				captures++
				return watch.Snapshot{}, marker
			}
			var err error
			if daemonMode {
				d, daemonErr := testdaemon.New(root)
				if daemonErr != nil {
					t.Fatal(daemonErr)
				}
				_, _, err = prepareInitialDaemonWatch(context.Background(), d, watch.Config{Root: root}, watch.BackendPoll, capture)
			} else {
				_, err = prepareInitialLocalWatchReplacement(context.Background(), root, watch.Config{Root: root}, watch.BackendPoll, capture)
			}
			if !errors.Is(err, marker) || captures != 1 {
				t.Fatalf("hard startup error = %v captures=%d, want marker once", err, captures)
			}
		})
	}
}

func TestScopedWatchBackendReplacementSwitchesExternalDependencyRoots(t *testing.T) {
	root := t.TempDir()
	dependencyA := filepath.Join(t.TempDir(), "dependency-a")
	dependencyB := filepath.Join(t.TempDir(), "dependency-b")
	aPath := filepath.Join(dependencyA, "A.cls")
	bPath := filepath.Join(dependencyB, "B.cls")
	writeTestFile(t, aPath, "public class A {}")
	writeTestFile(t, bPath, "public class B {}")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := watch.Config{Root: root, Backend: watch.BackendPoll, Debounce: 10 * time.Millisecond}
	watcherA, effectiveCfg, err := startScopedWatchBackend(ctx, cfg, watch.BackendPoll, watch.Scope{Roots: []string{root, dependencyA}})
	if err != nil {
		t.Fatal(err)
	}
	defer watcherA.Close()
	writeTestFile(t, aPath, "public class A { public void changed() {} }")
	waitForWatchChange(t, watcherA, aPath, watch.ChangeModified)

	watcherB, _, err := startScopedWatchBackend(ctx, effectiveCfg, watch.BackendPoll, watch.Scope{Roots: []string{root, dependencyB}})
	if err != nil {
		t.Fatal(err)
	}
	defer watcherB.Close()
	if err := watcherA.Close(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, bPath, "public class B { public void changed() {} }")
	waitForWatchChange(t, watcherB, bPath, watch.ChangeModified)

	writeTestFile(t, aPath, "public class A { public void changedAgain() {} }")
	select {
	case changes := <-watcherB.Changes():
		t.Fatalf("replacement watcher reported removed dependency root: %#v", changes)
	case err := <-watcherB.Errors():
		t.Fatal(err)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSymlinkedDirectDependencyWatchMatchesFreshLoad(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "project")
	physicalDependency := filepath.Join(workspace, "physical-dependency")
	dependencyAlias := filepath.Join(workspace, "dependency-alias")
	dependencyClass := filepath.Join(physicalDependency, "force-app/main/default/classes/StageService.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"stagepkg:../dependency-alias:1.0\"]\n")
	writeTestFile(t, filepath.Join(physicalDependency, "sfdx-project.json"), `{"namespace":"stagepkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, dependencyClass, "global class StageService { global static String value() { return 'initial'; } }")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/StageServiceTest.cls"), `
@IsTest private class StageServiceTest {
  @IsTest static void passes() { System.assertEquals('modified', stagepkg.StageService.value()); }
}`)
	if err := os.Symlink(physicalDependency, dependencyAlias); err != nil {
		t.Fatal(err)
	}
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ManagedPackageDependencies) != 1 || p.ManagedPackageDependencies[0].SourceRoot != dependencyAlias {
		t.Fatalf("dependency source root = %#v, want lexical alias %s", p.ManagedPackageDependencies, dependencyAlias)
	}

	for _, backend := range []watch.Backend{watch.BackendPoll, watch.BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := watch.Config{Root: root, Backend: backend, Debounce: 10 * time.Millisecond}
			watcher, _, err := startScopedWatchBackend(ctx, cfg, backend, watch.ProjectScope(root, p))
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.Close()

			writeTestFile(t, dependencyClass, "global class StageService { global static String value() { return 'modified'; } }")
			changes := waitForWatchChanges(t, watcher, dependencyClass, watch.ChangeModified)
			updated, graph, err := updateWatchIndexState(root, index, watch.BuildReferenceGraph(index), changes)
			if err != nil {
				t.Fatal(err)
			}
			fresh, err := loadIndex(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(updated, fresh) {
				t.Errorf("symlink dependency incremental index differs from forced clean:\nincremental: %#v\nfresh: %#v", updated, fresh)
			}
			freshGraph := watch.BuildReferenceGraph(fresh)
			if !reflect.DeepEqual(graph, freshGraph) {
				t.Errorf("symlink dependency incremental graph differs from forced clean:\nincremental: %#v\nfresh: %#v", graph, freshGraph)
			}
			selection := watch.SelectAffectedTestsWithRefGraph(updated, changes, graph)
			freshSelection := watch.SelectAffectedTestsWithRefGraph(fresh, changes, freshGraph)
			if !reflect.DeepEqual(selection, freshSelection) || selection.Mode != watch.SelectionAll {
				t.Errorf("symlink dependency selection = %#v, fresh = %#v, want all", selection, freshSelection)
			}
			if got, want := canonicalWatchRun(runSelectedTests(updated, apextest.Options{}, selection)), canonicalWatchRun(runSelectedTests(fresh, apextest.Options{}, freshSelection)); !reflect.DeepEqual(got, want) {
				t.Errorf("symlink dependency incremental run = %#v, fresh run = %#v", got, want)
			}
		})
	}
}

func TestNativeWatchRegistrationGapReconcilesToFreshLoad(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	scope := watch.ProjectScope(root, p)
	initial, err := watch.CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	classPath := filepath.Join(root, "force-app/main/default/classes/GapTest.cls")
	writeTestFile(t, classPath, `
@IsTest private class GapTest {
  @IsTest static void passes() { System.assertEquals(2, 1 + 1); }
}`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := watch.NewNativeWatcher(ctx, watch.Config{Root: root, Scope: scope, Backend: watch.BackendNative, Debounce: 10 * time.Millisecond}, initial)
	if err != nil {
		t.Skipf("native watcher unavailable: %v", err)
	}
	defer watcher.Close()
	changes := waitForWatchChanges(t, watcher, classPath, watch.ChangeAdded)
	updated, graph, err := updateWatchIndexState(root, index, watch.BuildReferenceGraph(index), changes)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated, fresh) {
		t.Errorf("registration-gap index differs from forced clean:\nincremental: %#v\nfresh: %#v", updated, fresh)
	}
	freshGraph := watch.BuildReferenceGraph(fresh)
	if !reflect.DeepEqual(graph, freshGraph) {
		t.Errorf("registration-gap graph differs from forced clean:\nincremental: %#v\nfresh: %#v", graph, freshGraph)
	}
	selection := watch.SelectAffectedTestsWithRefGraph(updated, changes, graph)
	freshSelection := watch.SelectAffectedTestsWithRefGraph(fresh, changes, freshGraph)
	if !reflect.DeepEqual(selection, freshSelection) || selection.Mode == watch.SelectionNone {
		t.Errorf("registration-gap selection = %#v, fresh = %#v, want runnable selection", selection, freshSelection)
	}
	if got, want := canonicalWatchRun(runSelectedTests(updated, apextest.Options{}, selection)), canonicalWatchRun(runSelectedTests(fresh, apextest.Options{}, freshSelection)); !reflect.DeepEqual(got, want) {
		t.Errorf("registration-gap incremental run = %#v, fresh run = %#v", got, want)
	}
}

func TestDirectPackageShimWatchMatchesFreshLoad(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "project")
	shimRoot := filepath.Join(workspace, "shim")
	shimClass := filepath.Join(shimRoot, "force-app/main/default/classes/ShimService.cls")
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  packageShims: [\"shimpkg:../shim\"]\n")
	writeTestFile(t, filepath.Join(shimRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, shimClass, "global class ShimService { global static String value() { return 'initial'; } }")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/ShimServiceTest.cls"), `
@IsTest private class ShimServiceTest {
  @IsTest static void passes() { System.assertEquals('modified', shimpkg.ShimService.value()); }
}`)
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PackageShims) != 1 || p.PackageShims[0].Status != "loaded" || !scopeHasRoot(watch.ProjectScope(root, p), shimRoot) {
		t.Fatalf("loaded direct package shim missing from project scope: project=%#v scope=%#v", p.PackageShims, watch.ProjectScope(root, p))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, _, err := startScopedWatchBackend(ctx, watch.Config{Root: root, Backend: watch.BackendPoll, Debounce: 10 * time.Millisecond}, watch.BackendPoll, watch.ProjectScope(root, p))
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	writeTestFile(t, shimClass, "global class ShimService { global static String value() { return 'modified'; } }")
	changes := waitForWatchChanges(t, watcher, shimClass, watch.ChangeModified)
	updated, graph, err := updateWatchIndexState(root, index, watch.BuildReferenceGraph(index), changes)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated, fresh) || !reflect.DeepEqual(graph, watch.BuildReferenceGraph(fresh)) {
		t.Errorf("package-shim incremental state differs from forced clean")
	}
	selection := watch.SelectAffectedTestsWithRefGraph(updated, changes, graph)
	freshSelection := watch.SelectAffectedTestsWithRefGraph(fresh, changes, watch.BuildReferenceGraph(fresh))
	if !reflect.DeepEqual(selection, freshSelection) || selection.Mode == watch.SelectionNone {
		t.Errorf("package-shim selection = %#v, fresh = %#v, want runnable selection", selection, freshSelection)
	}
	if got, want := canonicalWatchRun(runSelectedTests(updated, apextest.Options{}, selection)), canonicalWatchRun(runSelectedTests(fresh, apextest.Options{}, freshSelection)); !reflect.DeepEqual(got, want) {
		t.Errorf("package-shim incremental run = %#v, fresh run = %#v", got, want)
	}
}

func TestSymlinkedDependencyRetargetReloadsScopeAndStopsOldTarget(t *testing.T) {
	for _, backend := range []watch.Backend{watch.BackendPoll, watch.BackendNative} {
		t.Run(string(backend), func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "project")
			dependencyA := filepath.Join(workspace, "targets/dependency-a/pkg")
			dependencyB := filepath.Join(workspace, "targets/dependency-b/pkg")
			dependencyAlias := filepath.Join(workspace, "links/current")
			latestAlias := filepath.Join(workspace, "releases/latest")
			lexicalDependencyRoot := filepath.Join(dependencyAlias, "pkg")
			classA := filepath.Join(dependencyA, "force-app/main/default/classes/StageService.cls")
			classB := filepath.Join(dependencyB, "force-app/main/default/classes/StageService.cls")
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"stagepkg:../links/current/pkg:1.0\"]\n")
			for _, dependencyRoot := range []string{dependencyA, dependencyB} {
				writeTestFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{"namespace":"stagepkg","packageDirectories":[{"path":"force-app","default":true}]}`)
			}
			writeTestFile(t, classA, "global class StageService { global static String value() { return 'aaaa'; } }")
			writeTestFile(t, classB, "global class StageService { global static String value() { return 'bbbb'; } }")
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/LocalTest.cls"), "@IsTest private class LocalTest { @IsTest static void passes() { System.assert(true); } }")
			for _, dir := range []string{filepath.Dir(dependencyAlias), filepath.Dir(latestAlias)} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(filepath.Dir(dependencyA), latestAlias); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(latestAlias, dependencyAlias); err != nil {
				t.Fatal(err)
			}

			p, _, err := loadProjectIndex(root)
			if err != nil {
				t.Fatal(err)
			}
			scope := watch.ProjectScope(root, p)
			if !scopeHasRoot(scope, dependencyA) || !reflect.DeepEqual(scope.Topology, []string{dependencyAlias, latestAlias}) {
				t.Fatalf("initial alias scope = %#v, want physical A plus lexical topology endpoint", scope)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := watch.Config{Root: root, Backend: backend, Debounce: 10 * time.Millisecond}
			oldWatcher, effectiveCfg, err := startScopedWatchBackend(ctx, cfg, backend, scope)
			if err != nil {
				if backend == watch.BackendNative {
					t.Skipf("native watcher unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer oldWatcher.Close()
			activeWatcher := oldWatcher
			activeCfg := effectiveCfg

			if err := os.Remove(dependencyAlias); err != nil {
				t.Fatal(err)
			}
			waitForWatchChanges(t, activeWatcher, dependencyAlias, watch.ChangeDeleted)
			missingReplacement, err := prepareLocalWatchReplacement(ctx, root, activeCfg, backend)
			if err != nil {
				t.Fatal(err)
			}
			defer missingReplacement.watcher.Close()
			if scopeHasRoot(missingReplacement.cfg.Scope, dependencyA) || !reflect.DeepEqual(missingReplacement.cfg.Scope.Topology, []string{dependencyAlias, latestAlias}) {
				t.Fatalf("missing alias scope = %#v, want retained intermediate and chained topology", missingReplacement.cfg.Scope)
			}
			if err := activeWatcher.Close(); err != nil {
				t.Fatal(err)
			}
			activeWatcher = missingReplacement.watcher
			activeCfg = missingReplacement.cfg

			if err := os.Symlink(latestAlias, dependencyAlias); err != nil {
				t.Fatal(err)
			}
			waitForWatchChanges(t, activeWatcher, dependencyAlias, watch.ChangeAdded)
			recoveredReplacement, err := prepareLocalWatchReplacement(ctx, root, activeCfg, backend)
			if err != nil {
				t.Fatal(err)
			}
			defer recoveredReplacement.watcher.Close()
			if !scopeHasRoot(recoveredReplacement.cfg.Scope, dependencyA) || !reflect.DeepEqual(recoveredReplacement.cfg.Scope.Topology, []string{dependencyAlias, latestAlias}) {
				t.Fatalf("recreated alias scope = %#v, want physical A and retained chain", recoveredReplacement.cfg.Scope)
			}
			if err := activeWatcher.Close(); err != nil {
				t.Fatal(err)
			}
			activeWatcher = recoveredReplacement.watcher
			activeCfg = recoveredReplacement.cfg

			nextAlias := latestAlias + ".next"
			if err := os.Symlink(filepath.Dir(dependencyB), nextAlias); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(nextAlias, latestAlias); err != nil {
				t.Fatal(err)
			}
			topologyChanges := waitForWatchChanges(t, activeWatcher, latestAlias, watch.ChangeModified)
			topologyChangeFound := false
			for _, change := range topologyChanges {
				topologyChangeFound = topologyChangeFound || change.Path == latestAlias && change.Kind == watch.FileKindTopology
			}
			if !topologyChangeFound {
				t.Fatalf("alias retarget changes = %#v, want explicit topology change", topologyChanges)
			}
			if got := p.ManagedPackageDependencies[0].SourceRoot; got != lexicalDependencyRoot {
				t.Fatalf("loaded dependency source root = %s, want lexical intermediate path %s", got, lexicalDependencyRoot)
			}

			replacement, err := prepareLocalWatchReplacement(ctx, root, activeCfg, backend)
			if err != nil {
				t.Fatal(err)
			}
			defer replacement.watcher.Close()
			if !scopeHasRoot(replacement.cfg.Scope, dependencyB) || scopeHasRoot(replacement.cfg.Scope, dependencyA) || !reflect.DeepEqual(replacement.cfg.Scope.Topology, []string{dependencyAlias, latestAlias}) {
				t.Fatalf("retargeted scope = %#v, want physical B only plus lexical alias", replacement.cfg.Scope)
			}
			fresh, err := loadIndex(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(replacement.index, fresh) || !reflect.DeepEqual(replacement.graph, watch.BuildReferenceGraph(fresh)) {
				t.Fatal("retargeted prepared replacement differs from forced clean state")
			}
			selection := watch.SelectAffectedTestsWithRefGraph(replacement.index, topologyChanges, replacement.graph)
			if selection.Mode != watch.SelectionAll {
				t.Fatalf("topology selection = %#v, want all", selection)
			}

			if err := activeWatcher.Close(); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, classB, "global class StageService { global static String value() { return 'cccc'; } }")
			waitForWatchChange(t, replacement.watcher, classB, watch.ChangeModified)
			writeTestFile(t, classA, "global class StageService { global static String value() { return 'dddd'; } }")
			select {
			case changes := <-replacement.watcher.Changes():
				t.Fatalf("retargeted watcher observed old target A: %#v", changes)
			case err := <-replacement.watcher.Errors():
				t.Fatal(err)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestLocalClassUpdateWithConfigDriftRetainsOldStateAndWatcherUntilPrepared(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	dependencyA := filepath.Join(parent, "dependency-a")
	dependencyB := filepath.Join(parent, "dependency-b")
	manifestPath := filepath.Join(root, "sfdx-project.json")
	configPath := filepath.Join(root, "glade.yml")
	helperPath := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(dependencyA, "sfdx-project.json"), `{"namespace":"PkgA","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(dependencyB, "sfdx-project.json"), `{"namespace":"PkgB","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, configPath, "project:\n  managedPackageDependencies: [\"PkgA:../dependency-a:1.0\"]\n")
	writeTestFile(t, helperPath, "public class Helper { public static Integer value() { return 1; } }")
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := watch.BuildReferenceGraph(index)
	beforeIndex := index
	beforeGraph := graph

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := watch.Config{Root: root, Backend: watch.BackendPoll, Debounce: 10 * time.Millisecond}
	oldWatcher, effectiveCfg, err := startScopedWatchBackend(ctx, cfg, watch.BackendPoll, watch.ProjectScope(root, p))
	if err != nil {
		t.Fatal(err)
	}
	defer oldWatcher.Close()
	writeTestFile(t, helperPath, "public class Helper { public static Integer value() { return 2; } }")
	writeTestFile(t, configPath, "project:\n  managedPackageDependencies: [\"PkgB:../dependency-b:2.0\"]\n")
	classChanges := []watch.Change{{Path: helperPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
	update, err := tryUpdateWatchIndexState(root, effectiveCfg.Scope, index, graph, classChanges)
	if err != nil {
		t.Fatal(err)
	}
	if update.reusable {
		t.Fatal("class-only transaction published through project identity drift")
	}
	if !reflect.DeepEqual(update.index, beforeIndex) || update.graph != beforeGraph {
		t.Fatal("non-exact class update changed local published state")
	}
	waitForWatchChange(t, oldWatcher, helperPath, watch.ChangeModified)

	replacement, err := prepareLocalWatchReplacementFromProjectWithCapture(ctx, root, effectiveCfg, watch.BackendPoll, update.project, watch.CaptureScope)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.watcher.Close()
	if len(replacement.project.ManagedPackageDependencies) != 1 || replacement.project.ManagedPackageDependencies[0].Namespace != "PkgB" {
		t.Fatalf("prepared replacement project = %#v, want PkgB", replacement.project.ManagedPackageDependencies)
	}
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replacement.index, fresh) || !reflect.DeepEqual(replacement.graph, watch.BuildReferenceGraph(fresh)) {
		t.Fatal("prepared replacement does not match forced clean state")
	}
	writeTestFile(t, helperPath, "public class Helper { public static Integer value() { return 3; } }")
	waitForWatchChange(t, oldWatcher, helperPath, watch.ChangeModified)
}

func TestPrepareLocalWatchReplacementFailureLeavesOldWatcherLive(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Stable {}")
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	beforeIndex := index
	beforeGraph := watch.BuildReferenceGraph(index)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := watch.Config{Root: root, Backend: watch.BackendPoll, Debounce: 10 * time.Millisecond}
	oldWatcher, effectiveCfg, err := startScopedWatchBackend(ctx, cfg, watch.BackendPoll, watch.ProjectScope(root, p))
	if err != nil {
		t.Fatal(err)
	}
	defer oldWatcher.Close()
	writeTestFile(t, manifestPath, "{")
	if replacement, err := prepareLocalWatchReplacement(ctx, root, effectiveCfg, watch.BackendPoll); err == nil {
		if replacement.watcher != nil {
			_ = replacement.watcher.Close()
		}
		t.Fatal("replacement succeeded for invalid project configuration")
	}
	if !reflect.DeepEqual(index, beforeIndex) || !reflect.DeepEqual(watch.BuildReferenceGraph(index), beforeGraph) {
		t.Fatal("failed replacement changed the caller's published state")
	}

	writeTestFile(t, classPath, "public class Stable { public void changed() {} }")
	waitForWatchChange(t, oldWatcher, classPath, watch.ChangeModified)
}

func TestPrepareLocalWatchReplacementRejectsDriftAfterCandidateBaseline(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, classPath, "public class Stable {}")
	p, _, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := watch.Config{Root: root, Backend: watch.BackendPoll, Debounce: 10 * time.Millisecond}
	oldWatcher, effectiveCfg, err := startScopedWatchBackend(ctx, cfg, watch.BackendPoll, watch.ProjectScope(root, p))
	if err != nil {
		t.Fatal(err)
	}
	defer oldWatcher.Close()

	captures := 0
	capture := func(scope watch.Scope) (watch.Snapshot, error) {
		snapshot, captureErr := watch.CaptureScope(scope)
		captures++
		if captures == 1 {
			writeTestFile(t, manifestPath, `{"packageDirectories":[{"path":"other-app","default":true}]}`)
		}
		return snapshot, captureErr
	}
	if replacement, err := prepareLocalWatchReplacementWithCapture(ctx, root, effectiveCfg, watch.BackendPoll, capture); err == nil {
		if replacement.watcher != nil {
			_ = replacement.watcher.Close()
		}
		t.Fatal("replacement published after project inputs changed behind its candidate baseline")
	}
	writeTestFile(t, classPath, "public class Stable { public void oldWatcherStillLive() {} }")
	waitForWatchChange(t, oldWatcher, classPath, watch.ChangeModified)
}

func scopeHasRoot(scope watch.Scope, root string) bool {
	root, _ = filepath.Abs(root)
	for _, candidate := range scope.Roots {
		if filepath.Clean(candidate) == filepath.Clean(root) {
			return true
		}
	}
	return false
}

func waitForWatchChange(t *testing.T, watcher watch.BackendWatcher, path string, op watch.ChangeOp) {
	t.Helper()
	_ = waitForWatchChanges(t, watcher, path, op)
}

func waitForWatchChanges(t *testing.T, watcher watch.BackendWatcher, path string, op watch.ChangeOp) []watch.Change {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	var observed []watch.Change
	for {
		select {
		case changes, ok := <-watcher.Changes():
			if !ok {
				t.Fatalf("watcher closed waiting for %s %s; observed %#v", op, path, observed)
			}
			observed = append(observed, changes...)
			for _, change := range changes {
				if filepath.Clean(change.Path) == filepath.Clean(path) && change.Op == op {
					return changes
				}
			}
		case err, ok := <-watcher.Errors():
			if !ok {
				t.Fatalf("watcher errors closed waiting for %s %s; observed %#v", op, path, observed)
			}
			t.Fatal(err)
		case <-deadline.C:
			t.Fatalf("timeout waiting for %s %s; observed %#v", op, path, observed)
		}
	}
}

func TestWatchIndexStateLifecycleMatchesFreshLoad(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	configPath := filepath.Join(root, "glade.yml")
	dependencyRoot := filepath.Join(root, "deps", "stage")
	writeTestFile(t, manifestPath, `{"namespace":"localpkg","sourceApiVersion":"63.0","packageDirectories":[{"path":"force-app","default":true},{"path":"packages/extra"}]}`)
	writeTestFile(t, configPath, "project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n  managedPackageDependencies: [\"stagepkg:deps/stage:1.0\"]\n")
	writeTestFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{"namespace":"BasePkg","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	helperPath := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	servicePath := filepath.Join(root, "packages/extra/classes/Service.cls")
	lifecycleClassPath := filepath.Join(root, "force-app/main/default/classes/LifecycleClass.cls")
	renamedClassPath := filepath.Join(root, "force-app/main/default/classes/RenamedLifecycle.cls")
	triggerPath := filepath.Join(root, "force-app/main/default/triggers/LifecycleTrigger.trigger")
	addedTriggerPath := filepath.Join(root, "force-app/main/default/triggers/AddedTrigger.trigger")
	renamedTriggerPath := filepath.Join(root, "force-app/main/default/triggers/RenamedTrigger.trigger")
	dependencyClassPath := filepath.Join(dependencyRoot, "force-app/main/default/classes/StageService.cls")
	dependencyAddedClassPath := filepath.Join(dependencyRoot, "force-app/main/default/classes/StageAdded.cls")
	dependencyRenamedClassPath := filepath.Join(dependencyRoot, "force-app/main/default/classes/StageRenamed.cls")
	dependencyTriggerPath := filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageTrigger.trigger")
	dependencyAddedTriggerPath := filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageAddedTrigger.trigger")
	dependencyRenamedTriggerPath := filepath.Join(dependencyRoot, "force-app/main/default/triggers/StageRenamedTrigger.trigger")
	objectPath := filepath.Join(root, "force-app/main/default/objects/Lifecycle__c/Lifecycle__c.object-meta.xml")
	fieldPath := filepath.Join(root, "force-app/main/default/objects/Lifecycle__c/fields/Status__c.field-meta.xml")
	writeTestFile(t, helperPath, "public class Helper { public static void go() {} }")
	writeTestFile(t, servicePath, "public class Service { public static void run() {} }")
	writeTestFile(t, triggerPath, "trigger LifecycleTrigger on Account (before insert) {}")
	writeTestFile(t, dependencyClassPath, "global class StageService { global static String value() { return 'initial'; } }")
	writeTestFile(t, dependencyTriggerPath, "trigger StageTrigger on Account (before insert) {}")
	for _, className := range []string{"AlphaTest", "BetaTest"} {
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes", className+".cls"), `
@IsTest private class `+className+` {
  @IsTest static void passes() { Service.run(); System.assert(true); }
}`)
	}
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/UnrelatedTest.cls"), `
@IsTest private class UnrelatedTest {
  @IsTest static void passes() { System.assert(true); }
}`)

	index, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := watch.BuildReferenceGraph(index)
	change := func(path string, op watch.ChangeOp) watch.Change {
		classification := watch.ClassifyPath(path)
		return watch.Change{
			Path:       path,
			Op:         op,
			Kind:       classification.Kind,
			Name:       classification.Name,
			ObjectName: classification.ObjectName,
		}
	}
	remove := func(t *testing.T, path string) {
		t.Helper()
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	rename := func(t *testing.T, oldPath, newPath, source string) {
		t.Helper()
		if err := os.Rename(oldPath, newPath); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, newPath, source)
	}
	allTests := []string{"AlphaTest", "BetaTest", "UnrelatedTest"}
	helperChange := []watch.Change{change(helperPath, watch.ChangeModified)}
	steps := []struct {
		name            string
		mutate          func(*testing.T) []watch.Change
		selection       func([]watch.Change) []watch.Change
		assertIndex     func(*testing.T, typesys.Index)
		wantMode        watch.SelectionMode
		wantTestClasses []string
	}{
		{
			name: "local class dependency edge added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, servicePath, "public class Service { public static void run() { Helper.go(); } }")
				return []watch.Change{change(servicePath, watch.ChangeModified)}
			},
			selection:       func([]watch.Change) []watch.Change { return helperChange },
			wantMode:        watch.SelectionDirect,
			wantTestClasses: []string{"AlphaTest", "BetaTest"},
		},
		{
			name: "local class dependency edge removed",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, servicePath, "public class Service { public static void run() {} }")
				return []watch.Change{change(servicePath, watch.ChangeModified)}
			},
			selection:       func([]watch.Change) []watch.Change { return helperChange },
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local Apex parse error matches clean diagnostics",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, servicePath, "public class Service {")
				return []watch.Change{change(servicePath, watch.ChangeModified)}
			},
			assertIndex: func(t *testing.T, index typesys.Index) {
				if len(index.Diagnostics) == 0 {
					t.Fatal("parse-error update did not publish clean-build diagnostics")
				}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local Apex parse error repaired",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, servicePath, "public class Service { public static void run() {} }")
				return []watch.Change{change(servicePath, watch.ChangeModified)}
			},
			assertIndex: func(t *testing.T, index typesys.Index) {
				if len(index.Diagnostics) != 0 {
					t.Fatalf("repaired update retained diagnostics: %#v", index.Diagnostics)
				}
			},
			wantMode:        watch.SelectionDirect,
			wantTestClasses: []string{"AlphaTest", "BetaTest"},
		},
		{
			name: "local class added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, lifecycleClassPath, "public class LifecycleClass { public static String value() { return 'added'; } }")
				return []watch.Change{change(lifecycleClassPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local class modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, lifecycleClassPath, "public class LifecycleClass { public static String value() { return 'modified'; } }")
				return []watch.Change{change(lifecycleClassPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local class renamed",
			mutate: func(t *testing.T) []watch.Change {
				rename(t, lifecycleClassPath, renamedClassPath, "public class RenamedLifecycle { public static String value() { return 'renamed'; } }")
				return []watch.Change{change(lifecycleClassPath, watch.ChangeDeleted), change(renamedClassPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local class deleted",
			mutate: func(t *testing.T) []watch.Change {
				remove(t, renamedClassPath)
				return []watch.Change{change(renamedClassPath, watch.ChangeDeleted)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local trigger modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, triggerPath, "trigger LifecycleTrigger on Account (before insert, after update) {}")
				return []watch.Change{change(triggerPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local trigger added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, addedTriggerPath, "trigger AddedTrigger on Contact (after insert) {}")
				return []watch.Change{change(addedTriggerPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local trigger renamed",
			mutate: func(t *testing.T) []watch.Change {
				rename(t, addedTriggerPath, renamedTriggerPath, "trigger RenamedTrigger on Contact (after insert) {}")
				return []watch.Change{change(addedTriggerPath, watch.ChangeDeleted), change(renamedTriggerPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "local trigger deleted",
			mutate: func(t *testing.T) []watch.Change {
				remove(t, renamedTriggerPath)
				return []watch.Change{change(renamedTriggerPath, watch.ChangeDeleted)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency class modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, dependencyClassPath, "global class StageService { global static String value() { return 'modified'; } }")
				return []watch.Change{change(dependencyClassPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency class added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, dependencyAddedClassPath, "global class StageAdded { global static String value() { return 'added'; } }")
				return []watch.Change{change(dependencyAddedClassPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency class renamed",
			mutate: func(t *testing.T) []watch.Change {
				rename(t, dependencyAddedClassPath, dependencyRenamedClassPath, "global class StageRenamed { global static String value() { return 'renamed'; } }")
				return []watch.Change{change(dependencyAddedClassPath, watch.ChangeDeleted), change(dependencyRenamedClassPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency class deleted",
			mutate: func(t *testing.T) []watch.Change {
				remove(t, dependencyRenamedClassPath)
				return []watch.Change{change(dependencyRenamedClassPath, watch.ChangeDeleted)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency trigger modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, dependencyTriggerPath, "trigger StageTrigger on Account (before insert, after update) {}")
				return []watch.Change{change(dependencyTriggerPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency trigger added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, dependencyAddedTriggerPath, "trigger StageAddedTrigger on Contact (after insert) {}")
				return []watch.Change{change(dependencyAddedTriggerPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency trigger renamed",
			mutate: func(t *testing.T) []watch.Change {
				rename(t, dependencyAddedTriggerPath, dependencyRenamedTriggerPath, "trigger StageRenamedTrigger on Contact (after insert) {}")
				return []watch.Change{change(dependencyAddedTriggerPath, watch.ChangeDeleted), change(dependencyRenamedTriggerPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency trigger deleted",
			mutate: func(t *testing.T) []watch.Change {
				remove(t, dependencyRenamedTriggerPath)
				return []watch.Change{change(dependencyRenamedTriggerPath, watch.ChangeDeleted)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "schema object added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Lifecycle</label><pluralLabel>Lifecycles</pluralLabel><nameField><label>Lifecycle Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
				return []watch.Change{change(objectPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "schema object modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Lifecycle Record</label><pluralLabel>Lifecycle Records</pluralLabel><nameField><label>Lifecycle Record Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
				return []watch.Change{change(objectPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "schema field added",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Status__c</fullName><label>Status</label><type>Text</type><length>40</length></CustomField>`)
				return []watch.Change{change(fieldPath, watch.ChangeAdded)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "schema field modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Status__c</fullName><label>Lifecycle Status</label><type>Text</type><length>80</length></CustomField>`)
				return []watch.Change{change(fieldPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "schema field deleted",
			mutate: func(t *testing.T) []watch.Change {
				remove(t, fieldPath)
				return []watch.Change{change(fieldPath, watch.ChangeDeleted)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "schema object deleted",
			mutate: func(t *testing.T) []watch.Change {
				remove(t, objectPath)
				return []watch.Change{change(objectPath, watch.ChangeDeleted)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "dependency version and namespace remap modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, configPath, "project:\n  namespaceRemaps: [\"BasePkg:otherstage\"]\n  managedPackageDependencies: [\"otherstage:deps/stage:2.0\"]\n")
				return []watch.Change{change(configPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
		{
			name: "project configuration modified",
			mutate: func(t *testing.T) []watch.Change {
				writeTestFile(t, manifestPath, `{"namespace":"localpkg","sourceApiVersion":"64.0","packageDirectories":[{"path":"force-app","default":true},{"path":"packages/extra"}]}`)
				return []watch.Change{change(manifestPath, watch.ChangeModified)}
			},
			wantMode:        watch.SelectionAll,
			wantTestClasses: allTests,
		},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			changes := step.mutate(t)
			index, graph, err = updateWatchIndexState(root, index, graph, changes)
			if err != nil {
				t.Fatal(err)
			}
			fresh, err := loadIndex(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(index, fresh) {
				t.Errorf("incremental index differs from forced clean:\nincremental: %#v\nfresh: %#v", index, fresh)
			}
			if step.assertIndex != nil {
				step.assertIndex(t, index)
			}
			freshGraph := watch.BuildReferenceGraph(fresh)
			if !reflect.DeepEqual(graph, freshGraph) {
				t.Errorf("incremental graph differs from forced clean:\nincremental: %#v\nfresh: %#v", graph, freshGraph)
			}
			selectionChanges := changes
			if step.selection != nil {
				selectionChanges = step.selection(changes)
			}
			selection := watch.SelectAffectedTestsWithRefGraph(index, selectionChanges, graph)
			freshSelection := watch.SelectAffectedTestsWithRefGraph(fresh, selectionChanges, freshGraph)
			if !reflect.DeepEqual(selection, freshSelection) {
				t.Errorf("incremental selection = %#v, fresh = %#v", selection, freshSelection)
			}
			if selection.Mode != step.wantMode || !reflect.DeepEqual(selection.TestClasses, step.wantTestClasses) {
				t.Errorf("selection = %#v, want mode=%s classes=%#v", selection, step.wantMode, step.wantTestClasses)
			}
			if got, want := canonicalWatchRun(runSelectedTests(index, apextest.Options{}, selection)), canonicalWatchRun(runSelectedTests(fresh, apextest.Options{}, freshSelection)); !reflect.DeepEqual(got, want) {
				t.Errorf("incremental run = %#v, fresh run = %#v", got, want)
			}
		})
	}
}

func canonicalWatchRun(run testreport.Run) testreport.Run {
	run.DurationMS = 0
	run.Suites = append([]testreport.Suite(nil), run.Suites...)
	for suiteIndex := range run.Suites {
		run.Suites[suiteIndex].DurationMS = 0
		run.Suites[suiteIndex].Cases = append([]testreport.Case(nil), run.Suites[suiteIndex].Cases...)
		for caseIndex := range run.Suites[suiteIndex].Cases {
			run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
		}
	}
	return run
}

func TestWatchRunCoordinatorWaitsForCanceledRunBeforeStartingPending(t *testing.T) {
	var starts []watch.TestSelection
	cancels := make([]context.CancelFunc, 0, 2)
	starter := func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		starts = append(starts, selection)
		done := make(chan watchRunResult, 1)
		cancelled := false
		cancel := func() {
			cancelled = true
		}
		cancels = append(cancels, func() {
			if !cancelled {
				t.Fatalf("run %d was not canceled", runID)
			}
		})
		return cancel, done
	}

	coordinator := newWatchRunCoordinator(1)
	first := coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll, Reason: "initial"}, starter)
	if !first.Started || first.RunID != 1 || len(starts) != 1 {
		t.Fatalf("initial start = %#v, starts=%#v", first, starts)
	}

	pending := coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"InvoiceTest"}}, starter)
	if pending.Started {
		t.Fatalf("pending run started before active run drained: %#v", pending)
	}
	if len(starts) != 1 {
		t.Fatalf("starts = %d, want only the active run", len(starts))
	}
	cancels[0]()

	emit, next := coordinator.Complete(watchRunResult{RunID: 1, Result: testreport.Run{Name: "old"}})
	if emit {
		t.Fatalf("canceled run should be drained without emitting a finish event")
	}
	if !next.Started || next.RunID != 2 {
		t.Fatalf("next start = %#v, want run 2", next)
	}
	if len(starts) != 2 || starts[1].TestClasses[0] != "InvoiceTest" {
		t.Fatalf("starts = %#v", starts)
	}
}

func TestWatchRunCoordinatorCoalescesPendingDirectSelections(t *testing.T) {
	var starts []watch.TestSelection
	starter := func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		starts = append(starts, selection)
		return func() {}, make(chan watchRunResult, 1)
	}

	coordinator := newWatchRunCoordinator(1)
	coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll, Reason: "initial"}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"BillingTest"}}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"AccountTest"}}, starter)

	_, next := coordinator.Complete(watchRunResult{RunID: 1, Result: testreport.Run{Name: "old"}})
	if !next.Started {
		t.Fatalf("coalesced run did not start")
	}
	if len(starts) != 2 {
		t.Fatalf("starts = %d, want active plus coalesced pending", len(starts))
	}
	if got, want := starts[1].Mode, watch.SelectionDirect; got != want {
		t.Fatalf("mode = %s, want %s", got, want)
	}
	if got, want := starts[1].TestClasses, []string{"AccountTest", "BillingTest"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test classes = %#v, want %#v", got, want)
	}
}

func TestCoalesceWatchSelectionsNormalizesClassesStably(t *testing.T) {
	a := watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{" billingtest ", "AccountTest", "BILLINGTEST"}}
	b := watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"BillingTest", " accounttest ", "  "}}
	for _, test := range []struct {
		name string
		a    watch.TestSelection
		b    watch.TestSelection
		want []string
	}{
		{name: "forward", a: a, b: b, want: []string{"AccountTest", "billingtest"}},
		{name: "reverse", a: b, b: a, want: []string{"accounttest", "BillingTest"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := coalesceWatchSelections(test.a, test.b)
			if got.Mode != watch.SelectionDirect || !reflect.DeepEqual(got.TestClasses, test.want) {
				t.Fatalf("coalesced selection = %#v, want direct %#v", got, test.want)
			}
		})
	}
}

func TestWatchRunCoordinatorIgnoresStaleCompletion(t *testing.T) {
	starts := 0
	starter := func(int, watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		starts++
		return func() {}, make(chan watchRunResult, 1)
	}
	coordinator := newWatchRunCoordinator(10)
	coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"PendingTest"}}, starter)
	activeDone := coordinator.Done()

	emit, next := coordinator.Complete(watchRunResult{RunID: 9})
	if emit || next.Started || starts != 1 || coordinator.Done() != activeDone || coordinator.activeRunID != 10 || coordinator.pending == nil {
		t.Fatalf("stale completion changed coordinator: emit=%t next=%#v starts=%d active=%d pending=%#v", emit, next, starts, coordinator.activeRunID, coordinator.pending)
	}

	emit, next = coordinator.Complete(watchRunResult{RunID: 10})
	if emit || !next.Started || next.RunID != 11 || starts != 2 {
		t.Fatalf("active completion after stale = emit=%t next=%#v starts=%d", emit, next, starts)
	}
}

func TestWatchRunCoordinatorCoalescedRunUsesLatestStarter(t *testing.T) {
	coordinator := newWatchRunCoordinator(1)
	activeDone := make(chan watchRunResult, 1)
	coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll}, func(int, watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return func() {}, activeDone
	})
	var startedBy []string
	starter := func(label string) watchRunStarter {
		return func(_ int, _ watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
			startedBy = append(startedBy, label)
			return func() {}, make(chan watchRunResult, 1)
		}
	}
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"AlphaTest"}}, starter("older"))
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"BetaTest"}}, starter("latest"))
	_, next := coordinator.Complete(watchRunResult{RunID: 1})
	if !next.Started || !reflect.DeepEqual(startedBy, []string{"latest"}) {
		t.Fatalf("next = %#v starters = %#v, want latest starter", next, startedBy)
	}
}

func TestWatchRunCoordinatorAllDominatesAndCancelsOnce(t *testing.T) {
	cancelCount := 0
	coordinator := newWatchRunCoordinator(1)
	coordinator.Start(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"InitialTest"}}, func(int, watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return func() { cancelCount++ }, make(chan watchRunResult, 1)
	})
	starter := func(int, watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return func() {}, make(chan watchRunResult, 1)
	}
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"AlphaTest"}}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionAll, TestClasses: []string{"BetaTest"}}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"GammaTest"}}, starter)
	if cancelCount != 1 {
		t.Fatalf("cancel count = %d, want 1", cancelCount)
	}
	_, next := coordinator.Complete(watchRunResult{RunID: 1})
	if !next.Started || next.Selection.Mode != watch.SelectionAll {
		t.Fatalf("next = %#v, want started all selection", next)
	}
	if want := []string{"AlphaTest", "BetaTest", "GammaTest"}; !reflect.DeepEqual(next.Selection.TestClasses, want) {
		t.Fatalf("coalesced classes = %#v, want %#v", next.Selection.TestClasses, want)
	}
}

func TestWatchRunCoordinatorStopCancelsActiveRunOnce(t *testing.T) {
	cancelCount := 0
	coordinator := newWatchRunCoordinator(1)
	coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll}, func(int, watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return func() { cancelCount++ }, make(chan watchRunResult, 1)
	})
	coordinator.Stop()
	coordinator.Stop()
	if cancelCount != 1 {
		t.Fatalf("stop cancel count = %d, want 1", cancelCount)
	}
}
