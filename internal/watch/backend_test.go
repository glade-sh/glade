package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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
