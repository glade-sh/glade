package testdaemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/watch"
)

// TestServerWatchLoopSkipsSaveTheRequestApplied pins one update per write. A
// request that finds its generation stale applies the save itself; the
// watcher then reports the same save. That report must not publish a second
// generation or start a second warm.
func TestServerWatchLoopSkipsSaveTheRequestApplied(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{daemon: d}
	warms := make(chan struct{}, 4)
	server.afterWatchUpdateFn = func() { warms <- struct{}{} }
	created := make(chan serverWatchCreation, 2)
	server.newBackendWatcherFn = func(_ context.Context, cfg watch.Config, initial watch.Snapshot) (watch.BackendWatcher, watch.Backend, error) {
		stub := newServerWatchStub()
		created <- serverWatchCreation{scope: cfg.Scope, initial: initial, watcher: stub}
		return stub, watch.BackendPoll, nil
	}
	loads := 0
	load := d.loadProjectFn
	d.loadProjectFn = func(root string) (project.Project, error) {
		loads++
		return load(root)
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

	path := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	change := []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
	writeFile(t, path, "\npublic class Helper {\n  public static Integer value() { return 1 + 0; }\n}\n")
	stale := d.snapshotGeneration()
	refreshed, ok := d.refreshStaleGeneration(stale)
	if !ok || refreshed.serial != stale.serial+1 {
		t.Fatalf("request refresh: ok=%v serial %d -> %d", ok, stale.serial, refreshed.serial)
	}
	loads = 0
	first.watcher.changes <- change
	// A real save afterwards proves the loop handled the repeat before it.
	writeFile(t, path, "\npublic class Helper {\n  public static Integer value() { return 1 + 1; }\n}\n")
	first.watcher.changes <- change
	select {
	case <-warms:
	case <-time.After(30 * time.Second):
		t.Fatal("the real save did not warm")
	}
	select {
	case <-warms:
		t.Fatal("the repeated save warmed a second time")
	case <-time.After(200 * time.Millisecond):
	}
	if got := d.snapshotGeneration().serial; got != refreshed.serial+1 {
		t.Fatalf("generation serial = %d, want %d: the repeated save published a generation", got, refreshed.serial+1)
	}
	if loads != 1 {
		t.Fatalf("project loaded %d times for two watcher batches, want one", loads)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
		t.Fatalf("source unreadable: %v", err)
	}
	cancel()
	select {
	case <-loopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not stop")
	}
}

// TestGenerationHoldsSources covers the repeat check itself.
func TestGenerationHoldsSources(t *testing.T) {
	root := newDaemonLifecycleProject(t)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "force-app/main/default/classes/Helper.cls")
	generation := d.snapshotGeneration()
	modified := []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}}
	if !generationHoldsSources(generation, modified) {
		t.Fatal("unchanged source not recognized")
	}
	for name, changes := range map[string][]watch.Change{
		"none":     nil,
		"deleted":  {{Path: path, Op: watch.ChangeDeleted, Kind: watch.FileKindApexClass, Name: "Helper"}},
		"metadata": {{Path: path + "-meta.xml", Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "Helper"}},
		"unknown":  {{Path: filepath.Join(root, "force-app/main/default/classes/New.cls"), Op: watch.ChangeModified, Kind: watch.FileKindApexClass, Name: "New"}},
	} {
		if generationHoldsSources(generation, changes) {
			t.Fatalf("%s: treated as a repeat", name)
		}
	}
	writeFile(t, path, "\npublic class Helper {\n  public static Integer value() { return 2; }\n}\n")
	if generationHoldsSources(generation, modified) {
		t.Fatal("changed source treated as a repeat")
	}
}
