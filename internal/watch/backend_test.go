package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

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
