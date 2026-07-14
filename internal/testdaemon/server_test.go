package testdaemon

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

func TestServerWatchLoopSameScopeFallbackKeepsWatcherAndQueuesNextBatch(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalBuild := d.buildIndexFn
	buildStarted := make(chan struct{})
	releaseBuild := make(chan struct{})
	server := &Server{daemon: d}
	after := make(chan struct{}, 2)
	afterCalls := 0
	server.afterWatchUpdateFn = func() {
		afterCalls++
		after <- struct{}{}
	}
	created := make(chan serverWatchCreation, 4)
	factoryCalls := 0
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		factoryCalls++
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	first := waitForServerWatchCreation(t, created)
	d.updateMu.Lock()
	d.mu.Lock()
	d.index.Diagnostics = append(d.index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Warning, Message: "warning"})
	d.graph = watch.BuildReferenceGraph(d.index)
	d.mu.Unlock()
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) {
		close(buildStarted)
		<-releaseBuild
		d.buildIndexFn = originalBuild
		return originalBuild(p)
	}
	d.updateMu.Unlock()
	helperPath := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	writeFile(t, helperPath, "public class Helper { public static Integer value() { return 2; } }")
	first.watcher.changes <- []watch.Change{{Path: helperPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
	select {
	case <-buildStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("same-scope fallback build did not start")
	}
	queuedPath := filepath.Join(root, "force-app/main/default/classes/Queued.cls")
	writeFile(t, queuedPath, "public class Queued {}")
	first.watcher.changes <- []watch.Change{{Path: queuedPath, Op: watch.ChangeAdded, Kind: watch.FileKindApexClass, Name: "Queued"}}
	close(releaseBuild)
	for i := 0; i < 2; i++ {
		select {
		case <-after:
		case <-time.After(5 * time.Second):
			t.Fatalf("afterWatchUpdate call %d did not arrive", i+1)
		}
	}
	if afterCalls != 2 || factoryCalls != 1 {
		t.Fatalf("watch hooks = after:%d factory:%d, want 2/1", afterCalls, factoryCalls)
	}
	select {
	case creation := <-created:
		t.Fatalf("same-scope fallback created replacement watcher: %#v", creation)
	default:
	}
	select {
	case <-first.watcher.closed:
		t.Fatal("same-scope fallback closed the active watcher")
	default:
	}
	_, fresh, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.IndexSnapshot(), fresh) {
		t.Fatal("queued batch final state differs from clean load")
	}
	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, first.watcher)
}

func TestServerPingAndRun(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	socket := filepath.Join(root, "serve.sock")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := NewServer(ServerConfig{
		Root:   root,
		Socket: socket,
		Warm:   true,
		Watch:  false,
	})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe(ctx, io.Discard)
	}()

	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := Ping(ctx, socket)
		if err == nil && resp.Ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %v %#v", err, resp)
		}
		time.Sleep(20 * time.Millisecond)
	}

	first, err := Run(ctx, socket, Request{Filter: "WarmOneTest"})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if got := first.Run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("first summary = %#v", got)
	}

	second, err := Run(ctx, socket, Request{Filter: "WarmTwoTest"})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := second.Run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("second summary = %#v", got)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestServerWatchLoopScopesSiblingDependencyAndSwapsConfigScope(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	initialProject := d.project
	d.mu.RUnlock()
	originalBuildIndex := d.buildIndexFn
	buildStarted := make(chan project.Project, 1)
	releaseBuild := make(chan struct{})
	buildCalls := 0
	var releaseBuildOnce sync.Once
	release := func() { releaseBuildOnce.Do(func() { close(releaseBuild) }) }
	defer release()
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) {
		buildCalls++
		if buildCalls == 1 {
			return originalBuildIndex(p)
		}
		buildStarted <- p
		<-releaseBuild
		return originalBuildIndex(p)
	}
	server := &Server{daemon: d}
	server.afterWatchUpdateFn = func() {}
	created := make(chan serverWatchCreation, 4)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()

	first := waitForServerWatchCreation(t, created)
	d.updateMu.Lock()
	d.updateMu.Unlock()
	wantFirst := watch.ProjectScope(root, d.project)
	if !reflect.DeepEqual(first.scope, wantFirst) {
		t.Fatalf("initial watcher scope = %#v, want %#v", first.scope, wantFirst)
	}
	dependencyA := filepath.Join(filepath.Dir(root), "dependency")
	if !scopeContainsRoot(first.scope, dependencyA) {
		t.Fatalf("initial watcher scope omits sibling dependency %s: %#v", dependencyA, first.scope)
	}

	dependencyB := filepath.Join(filepath.Dir(root), "dependency-b")
	writeFile(t, filepath.Join(dependencyB, "sfdx-project.json"), `{"namespace":"OtherPkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(dependencyB, "force-app/main/default/classes/DependencyB.cls"), "public class DependencyB {}")
	configPath := filepath.Join(root, "glade.yml")
	writeFile(t, configPath, "project:\n  namespaceRemaps: [\"OtherPkg:nextpkg\"]\n  managedPackageDependencies: [\"nextpkg:../dependency-b:2.0\"]\n")
	// Only the class event is delivered. The checked incremental update must
	// notice the concurrent project identity drift and require a prepared reload.
	helperPath := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	writeFile(t, helperPath, "public class Helper { public static Integer value() { return 2; } }")
	first.watcher.changes <- []watch.Change{{Path: helperPath, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}

	second := waitForServerWatchCreation(t, created)
	preparedProject := <-buildStarted
	wantProject, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preparedProject, wantProject) {
		t.Fatalf("index build project = %#v, want prepared project %#v", preparedProject, wantProject)
	}
	wantSecond := watch.ProjectScope(root, wantProject)
	if !reflect.DeepEqual(second.scope, wantSecond) {
		t.Fatalf("replacement watcher scope = %#v, want %#v", second.scope, wantSecond)
	}
	if !scopeContainsRoot(second.scope, dependencyB) || scopeContainsRoot(second.scope, dependencyA) {
		t.Fatalf("replacement watcher did not swap A->B: %#v", second.scope)
	}
	d.mu.RLock()
	publishedWhileBuilding := d.project
	d.mu.RUnlock()
	if !reflect.DeepEqual(publishedWhileBuilding, initialProject) {
		t.Fatalf("daemon published project before replacement build completed: %#v", publishedWhileBuilding)
	}
	select {
	case <-first.watcher.closed:
		t.Fatal("daemon closed old watcher before prepared replacement published")
	default:
	}

	release()
	waitForServerWatchClose(t, first.watcher)
	waitForServerCondition(t, func() bool {
		d.mu.RLock()
		defer d.mu.RUnlock()
		return reflect.DeepEqual(d.project, wantProject)
	}, "daemon did not publish project B")

	latePath := filepath.Join(dependencyB, "force-app/main/default/classes/LateDependency.cls")
	writeFile(t, latePath, "public class LateDependency {}")
	second.watcher.changes <- []watch.Change{{Path: latePath, Op: watch.ChangeAdded, Kind: watch.FileKindApexClass, Name: "LateDependency"}}
	waitForServerCondition(t, func() bool {
		for _, typ := range d.IndexSnapshot().Types {
			if typ.Name == "LateDependency" && typ.Namespace == "nextpkg" {
				return true
			}
		}
		return false
	}, "replacement watcher did not deliver dependency B change")

	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, second.watcher)
}

func TestServerWatchLoopInitialStateMatchesWatcherBaseline(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	classPath := filepath.Join(root, "force-app/main/default/classes/Stable.cls")
	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, classPath, "public class Stable { public void stale() {} }")
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, classPath, "public class Stable { public void fresh() {} }")

	server := &Server{daemon: d}
	server.afterWatchUpdateFn = func() {}
	created := make(chan serverWatchCreation, 1)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	first := waitForServerWatchCreation(t, created)
	fresh, err := loadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	waitForServerCondition(t, func() bool {
		return reflect.DeepEqual(d.IndexSnapshot(), fresh)
	}, "initial server state differs from watcher baseline")
	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, first.watcher)
}

func TestServerWatchLoopRetriesOrdinaryInitialDrift(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	writeFile(t, manifestPath, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{daemon: d}
	server.afterWatchUpdateFn = func() {}
	captures := 0
	server.captureScopeFn = func(scope watch.Scope) (watch.Snapshot, error) {
		snapshot, captureErr := watch.CaptureScope(scope)
		captures++
		if captures == 1 {
			if err := os.WriteFile(manifestPath, []byte(`{"packageDirectories":[{"path":"other-app","default":true}]}`), 0o644); err != nil {
				return watch.Snapshot{}, err
			}
		}
		return snapshot, captureErr
	}
	created := make(chan serverWatchCreation, 2)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	failed := waitForServerWatchCreation(t, created)
	waitForServerWatchClose(t, failed.watcher)
	stable := waitForServerWatchCreation(t, created)
	d.updateMu.Lock()
	d.updateMu.Unlock()
	if captures != 4 {
		t.Fatalf("initial server captures = %d, want failed baseline/proof plus stable baseline/proof", captures)
	}
	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, stable.watcher)
}

func TestServerWatchLoopInitialRegistrationDoesNotRunChangeWarmHook(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	server, err := NewServer(ServerConfig{
		Root:   root,
		Socket: filepath.Join(root, "serve.sock"),
		Watch:  true,
		Warm:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if ready, warming := server.status(); !server.watchOn || !ready || warming {
		t.Fatalf("Watch=true Warm=false state: watch=%t ready=%t warming=%t", server.watchOn, ready, warming)
	}
	afterCalls := 0
	server.afterWatchUpdateFn = func() { afterCalls++ }
	created := make(chan serverWatchCreation, 1)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	initial := waitForServerWatchCreation(t, created)
	server.daemon.updateMu.Lock()
	server.daemon.updateMu.Unlock()
	if afterCalls != 0 {
		t.Fatalf("initial watch registration invoked change-only warm hook %d times", afterCalls)
	}
	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, initial.watcher)
}

func TestServerWatchLoopRejectsReplacementDriftAndKeepsOldWatcher(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{daemon: d}
	server.afterWatchUpdateFn = func() {}
	classPath := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	captures := 0
	server.captureScopeFn = func(scope watch.Scope) (watch.Snapshot, error) {
		captures++
		if captures == 4 {
			if err := os.WriteFile(classPath, []byte("public class Helper { public static Integer drifted() { return 3; } }"), 0o644); err != nil {
				return watch.Snapshot{}, err
			}
		}
		return watch.CaptureScope(scope)
	}
	created := make(chan serverWatchCreation, 2)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	first := waitForServerWatchCreation(t, created)
	d.updateMu.Lock()
	d.updateMu.Unlock()
	d.mu.RLock()
	beforeProject := d.project
	beforeIndex := d.index
	beforeGraph := d.graph
	d.mu.RUnlock()

	configPath := filepath.Join(root, "glade.yml")
	writeFile(t, configPath, "project:\n  managedPackageDependencies: []\n")
	first.watcher.changes <- []watch.Change{{Path: configPath, Op: watch.ChangeModified, Kind: watch.FileKindIgnored, Name: "glade.yml"}}
	failedCandidate := waitForServerWatchCreation(t, created)
	waitForServerWatchClose(t, failedCandidate.watcher)
	select {
	case <-first.watcher.closed:
		t.Fatal("replacement drift closed the old watcher")
	default:
	}
	d.mu.RLock()
	afterProject := d.project
	afterIndex := d.index
	afterGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(afterProject, beforeProject) || !reflect.DeepEqual(afterIndex, beforeIndex) || afterGraph != beforeGraph {
		t.Fatal("replacement drift published daemon state")
	}

	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, first.watcher)
}

func TestServerWatchLoopRetainsWatcherAndStateWhenReplacementBuildFails(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	beforeProject := d.project
	beforeIndex := d.index
	beforeGraph := d.graph
	d.mu.RUnlock()
	originalBuildIndex := d.buildIndexFn
	buildAttempts := 0
	d.buildIndexFn = func(p project.Project) (typesys.Index, error) {
		buildAttempts++
		if buildAttempts == 2 {
			return typesys.Index{}, errors.New("replacement build failed")
		}
		return originalBuildIndex(p)
	}

	server := &Server{daemon: d}
	server.afterWatchUpdateFn = func() {}
	created := make(chan serverWatchCreation, 4)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	first := waitForServerWatchCreation(t, created)
	d.updateMu.Lock()
	d.updateMu.Unlock()
	d.mu.RLock()
	beforeProject = d.project
	beforeIndex = d.index
	beforeGraph = d.graph
	d.mu.RUnlock()

	dependencyB := filepath.Join(filepath.Dir(root), "dependency-b")
	writeFile(t, filepath.Join(dependencyB, "sfdx-project.json"), `{"namespace":"OtherPkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(dependencyB, "force-app/main/default/classes/DependencyB.cls"), "public class DependencyB {}")
	configPath := filepath.Join(root, "glade.yml")
	writeFile(t, configPath, "project:\n  managedPackageDependencies: [\"nextpkg:../dependency-b:2.0\"]\n")
	change := []watch.Change{{Path: configPath, Op: watch.ChangeModified, Kind: watch.FileKindIgnored, Name: "glade.yml"}}
	first.watcher.changes <- change
	failedCandidate := waitForServerWatchCreation(t, created)
	waitForServerWatchClose(t, failedCandidate.watcher)
	select {
	case <-first.watcher.closed:
		t.Fatal("old watcher closed after replacement build failure")
	case <-time.After(100 * time.Millisecond):
	}
	d.mu.RLock()
	afterProject := d.project
	afterIndex := d.index
	afterGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(afterProject, beforeProject) || !reflect.DeepEqual(afterIndex, beforeIndex) || afterGraph != beforeGraph {
		t.Fatalf("replacement build failure published daemon state:\nproject=%#v\nindex=%#v\ngraph=%p", afterProject, afterIndex, afterGraph)
	}

	first.watcher.changes <- change
	second := waitForServerWatchCreation(t, created)
	waitForServerWatchClose(t, first.watcher)
	wantProject, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	waitForServerCondition(t, func() bool {
		d.mu.RLock()
		defer d.mu.RUnlock()
		return reflect.DeepEqual(d.project, wantProject)
	}, "daemon did not publish after replacement build recovered")

	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
	waitForServerWatchClose(t, second.watcher)
}

func TestServerWatchLoopRetainsWatcherAndStateWhenReplacementStartFails(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	d.mu.RLock()
	beforeProject := d.project
	beforeIndex := d.index
	beforeGraph := d.graph
	d.mu.RUnlock()

	server := &Server{daemon: d}
	server.afterWatchUpdateFn = func() {}
	created := make(chan serverWatchCreation, 4)
	attempts := 0
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		attempts++
		if attempts == 2 {
			d.mu.RLock()
			publishedDuringStart := d.project
			d.mu.RUnlock()
			if !reflect.DeepEqual(publishedDuringStart, beforeProject) {
				t.Errorf("daemon published candidate project before replacement watcher start: %#v", publishedDuringStart)
			}
			created <- serverWatchCreation{scope: cfg.Scope, initial: initial, err: errors.New("replacement start failed")}
			return nil, "", errors.New("replacement start failed")
		}
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loopDone := make(chan struct{})
	go func() {
		server.watchLoop(ctx, root)
		close(loopDone)
	}()
	first := waitForServerWatchCreation(t, created)
	d.updateMu.Lock()
	d.updateMu.Unlock()
	d.mu.RLock()
	beforeProject = d.project
	beforeIndex = d.index
	beforeGraph = d.graph
	d.mu.RUnlock()

	dependencyB := filepath.Join(filepath.Dir(root), "dependency-b")
	writeFile(t, filepath.Join(dependencyB, "sfdx-project.json"), `{"namespace":"OtherPkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(dependencyB, "force-app/main/default/classes/DependencyB.cls"), "public class DependencyB {}")
	configPath := filepath.Join(root, "glade.yml")
	writeFile(t, configPath, "project:\n  managedPackageDependencies: [\"nextpkg:../dependency-b:2.0\"]\n")
	change := []watch.Change{{Path: configPath, Op: watch.ChangeModified, Kind: watch.FileKindIgnored, Name: "glade.yml"}}
	first.watcher.changes <- change
	failed := waitForServerWatchCreation(t, created)
	if failed.err == nil {
		t.Fatal("replacement watcher unexpectedly started")
	}
	select {
	case <-first.watcher.closed:
		t.Fatal("old watcher closed after replacement failure")
	case <-time.After(100 * time.Millisecond):
	}
	d.mu.RLock()
	afterProject := d.project
	afterIndex := d.index
	afterGraph := d.graph
	d.mu.RUnlock()
	if !reflect.DeepEqual(afterProject, beforeProject) || !reflect.DeepEqual(afterIndex, beforeIndex) || afterGraph != beforeGraph {
		t.Fatalf("replacement failure published daemon state:\nproject=%#v\nindex=%#v\ngraph=%p", afterProject, afterIndex, afterGraph)
	}

	first.watcher.changes <- change
	second := waitForServerWatchCreation(t, created)
	if second.watcher == nil {
		t.Fatal("watch loop did not recover after replacement failure")
	}
	waitForServerWatchClose(t, first.watcher)
	wantProject, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	waitForServerCondition(t, func() bool {
		d.mu.RLock()
		defer d.mu.RUnlock()
		return reflect.DeepEqual(d.project, wantProject)
	}, "daemon did not publish recovered replacement project")

	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
}

type serverWatchCreation struct {
	scope   watch.Scope
	initial watch.Snapshot
	watcher *serverWatchStub
	err     error
}

type serverWatchStub struct {
	changes chan []watch.Change
	errors  chan error
	closed  chan struct{}
	once    sync.Once
}

func newServerWatchStub() *serverWatchStub {
	return &serverWatchStub{
		changes: make(chan []watch.Change, 4),
		errors:  make(chan error, 1),
		closed:  make(chan struct{}),
	}
}

func (w *serverWatchStub) Changes() <-chan []watch.Change { return w.changes }
func (w *serverWatchStub) Errors() <-chan error           { return w.errors }
func (w *serverWatchStub) Close() error {
	w.once.Do(func() { close(w.closed) })
	return nil
}

func waitForServerWatchCreation(t *testing.T, created <-chan serverWatchCreation) serverWatchCreation {
	t.Helper()
	select {
	case creation := <-created:
		return creation
	case <-time.After(5 * time.Second):
		t.Fatal("watcher was not created")
		return serverWatchCreation{}
	}
}

func waitForServerWatchClose(t *testing.T, watcher *serverWatchStub) {
	t.Helper()
	select {
	case <-watcher.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher was not closed")
	}
}

func waitForServerCondition(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func scopeContainsRoot(scope watch.Scope, root string) bool {
	want, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	want = filepath.Clean(want)
	for _, candidate := range scope.Roots {
		if candidate == want {
			return true
		}
	}
	return false
}

func writeTestProject(t *testing.T, root string) {
	t.Helper()
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
}
