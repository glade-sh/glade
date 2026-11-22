//go:build darwin && cgo

package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestRecursiveWatcherParentRecreation(t *testing.T) {
	workspace := t.TempDir()
	parent := filepath.Join(workspace, "parent")
	root := filepath.Join(parent, "project")
	class := filepath.Join(root, "Example.cls")
	writeWatchFile(t, class, "before")
	initial, err := CaptureScope(Scope{Roots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendNative, Debounce: 20 * time.Millisecond}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if _, ok := watcher.(*recursiveWatcher); !ok {
		t.Fatalf("recursive backend not selected: %T", watcher)
	}
	if err := os.Rename(parent, parent+"-old"); err != nil {
		t.Fatal(err)
	}
	awaitScopedChange(t, watcher, class, ChangeDeleted)
	writeWatchFile(t, class, "after")
	awaitScopedChange(t, watcher, class, ChangeAdded)
	replaceWatchFile(t, class, "another edit")
	awaitScopedChange(t, watcher, class, ChangeModified)
}

func TestRecursiveWatcherCallbackRecovery(t *testing.T) {
	flags := []struct {
		name string
		flag uint32
	}{
		{"must-scan", recursiveMustScan},
		{"user-dropped", recursiveUserDropped},
		{"kernel-dropped", recursiveKernelDropped},
	}
	for _, tc := range flags {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "parent", "project")
			external := filepath.Join(workspace, "outside", "glade.yml")
			writeWatchFile(t, filepath.Join(root, "Example.cls"), "class")
			writeWatchFile(t, external, "before")
			scope := Scope{Roots: []string{root}, Files: []string{external}}
			initial, err := CaptureScope(scope)
			if err != nil {
				t.Fatal(err)
			}
			watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Scope: scope, Backend: BackendNative, Debounce: 10 * time.Millisecond}, initial)
			if err != nil {
				t.Fatal(err)
			}
			defer watcher.Close()
			w, ok := watcher.(*recursiveWatcher)
			if !ok {
				t.Fatalf("recursive backend not selected: %T", watcher)
			}
			replaceWatchFile(t, external, "after")
			recursiveInjectEvent(w.stream, -1, external, tc.flag)
			select {
			case changes := <-watcher.Changes():
				assertChange(t, changes, external, ChangeModified, FileKindIgnored)
			case err := <-watcher.Errors():
				t.Fatal(err)
			case <-time.After(400 * time.Millisecond):
				t.Fatal("callback did not trigger reconciliation before auxiliary polling")
			}
		})
	}
	t.Run("unrelated-event", func(t *testing.T) {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer read.Close()
		defer write.Close()
		readFD := int(read.Fd())
		if err := syscall.SetNonblock(readFD, true); err != nil {
			t.Fatal(err)
		}
		if got := recursiveInjectEvent(nil, int(write.Fd()), "/unrelated", 0); got != 0 {
			t.Fatalf("unrelated event requested polling: %d", got)
		}
		var buf [1]byte
		if n, err := syscall.Read(readFD, buf[:]); n != -1 || err != syscall.EAGAIN {
			t.Fatalf("unrelated event produced pipe data: n=%d, err=%v", n, err)
		}
	})
	t.Run("full-pipe-root-loss", func(t *testing.T) {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer read.Close()
		defer write.Close()
		fd := int(write.Fd())
		if err := syscall.SetNonblock(fd, true); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 4096)
		for i := range buf {
			buf[i] = 'E'
		}
		for {
			_, err := syscall.Write(fd, buf)
			if err == syscall.EAGAIN {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, tc := range flags {
			if got := recursiveInjectEvent(nil, fd, "/unrelated", tc.flag); got != 0 {
				t.Fatalf("full pipe %s requested polling: %d", tc.name, got)
			}
		}
		if got := recursiveInjectEvent(nil, fd, "/root", recursiveRootChanged); got != 1 {
			t.Fatalf("full pipe root loss: first and second poll checks = %d, want 1", got)
		}
		if n, err := read.Read(buf[:1]); n != 1 || err != nil || buf[0] != 'E' {
			t.Fatalf("full pipe must retain a wakeup: n=%d, byte=%q, err=%v", n, buf[0], err)
		}
	})
}

func TestRecursiveWatcherRootRecreation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	class := filepath.Join(root, "Example.cls")
	writeWatchFile(t, class, "before")
	initial, err := CaptureScope(Scope{Roots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendNative, Debounce: 20 * time.Millisecond}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if _, ok := watcher.(*recursiveWatcher); !ok {
		t.Fatalf("recursive backend not selected: %T", watcher)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	awaitScopedChange(t, watcher, class, ChangeDeleted)
	writeWatchFile(t, class, "after")
	awaitScopedChange(t, watcher, class, ChangeAdded)
	replaceWatchFile(t, class, "another edit")
	awaitScopedChange(t, watcher, class, ChangeModified)
}

func TestRecursiveWatcherReconcilesInitialSnapshot(t *testing.T) {
	root := t.TempDir()
	initial, err := CaptureScope(Scope{Roots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	class := filepath.Join(root, "Example.cls")
	writeWatchFile(t, class, "created after baseline")
	watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Backend: BackendNative, Debounce: 20 * time.Millisecond}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if _, ok := watcher.(*recursiveWatcher); !ok {
		t.Fatalf("recursive backend not selected: %T", watcher)
	}
	awaitScopedChange(t, watcher, class, ChangeAdded)
}

func TestRecursiveWatcherConcurrentClose(t *testing.T) {
	root := t.TempDir()
	initial, err := CaptureScope(Scope{Roots: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	watcher, _, err := NewBackendWatcher(ctx, Config{Root: root, Backend: BackendNative}, initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := watcher.(*recursiveWatcher); !ok {
		t.Fatalf("recursive backend not selected: %T", watcher)
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() { defer group.Done(); _ = watcher.Close() }()
	}
	cancel()
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent close blocked")
	}
}

func TestRecursiveWatcherReconcilesPathlessSignal(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "parent", "project")
	external := filepath.Join(workspace, "outside", "glade.yml")
	writeWatchFile(t, filepath.Join(root, "Example.cls"), "class")
	writeWatchFile(t, external, "before")
	scope := Scope{Roots: []string{root}, Files: []string{external}}
	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	watcher, _, err := NewBackendWatcher(context.Background(), Config{Root: root, Scope: scope, Backend: BackendNative, Debounce: 10 * time.Millisecond}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	w, ok := watcher.(*recursiveWatcher)
	if !ok {
		t.Fatalf("recursive backend not selected: %T", watcher)
	}
	// This file lies outside the event stream. A pathless notification must
	// reconcile the whole scope, as it does for a coalesced or dropped event.
	replaceWatchFile(t, external, "after")
	if _, err := w.write.Write([]byte{'E'}); err != nil {
		t.Fatal(err)
	}
	select {
	case changes := <-watcher.Changes():
		assertChange(t, changes, external, ChangeModified, FileKindIgnored)
	case err := <-watcher.Errors():
		t.Fatal(err)
	case <-time.After(400 * time.Millisecond):
		t.Fatal("pathless signal did not reconcile before auxiliary polling")
	}
}

func TestRecursiveWatcherFallsBackForMultipleRoots(t *testing.T) {
	workspace := t.TempDir()
	first := filepath.Join(workspace, "first")
	second := filepath.Join(workspace, "second")
	writeWatchFile(t, filepath.Join(first, "First.cls"), "first")
	writeWatchFile(t, filepath.Join(second, "Second.cls"), "second")
	scope := Scope{Roots: []string{first, second}}
	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	watcher, selected, err := NewBackendWatcher(context.Background(), Config{Root: first, Scope: scope, Backend: BackendNative}, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if _, ok := watcher.(*NativeWatcher); !ok || selected != BackendNative {
		t.Fatalf("multiple roots used %T (%s), want existing native watcher", watcher, selected)
	}
}

func TestRecursiveWatcherLinkedFileScopeGuard(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	writeWatchFile(t, target, "content")
	if err := os.Symlink(target, filepath.Join(root, "unwatched.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, "unwatched-hard.txt")); err != nil {
		t.Fatal(err)
	}
	excluded := filepath.Join(root, "excluded")
	if err := os.Mkdir(excluded, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(excluded, "Excluded.cls")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(excluded, "ExcludedHard.cls")); err != nil {
		t.Fatal(err)
	}
	scope := NormalizeScope(Scope{Roots: []string{root}, ExcludedRoots: []string{excluded}})
	if linked, err := recursiveScopeHasLinkedFile(scope); err != nil || linked {
		t.Fatalf("unwatched and excluded links: linked=%v, err=%v", linked, err)
	}
	if err := os.Symlink(target, filepath.Join(root, "Included.cls")); err != nil {
		t.Fatal(err)
	}
	if linked, err := recursiveScopeHasLinkedFile(scope); err != nil || !linked {
		t.Fatalf("watchable link: linked=%v, err=%v", linked, err)
	}
	if err := os.Remove(filepath.Join(root, "Included.cls")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, filepath.Join(root, "IncludedHard.cls")); err != nil {
		t.Fatal(err)
	}
	if linked, err := recursiveScopeHasLinkedFile(scope); err != nil || !linked {
		t.Fatalf("watchable hard link: linked=%v, err=%v", linked, err)
	}
}

func TestRecursiveWatcherMountGuard(t *testing.T) {
	if nested, err := recursiveScopeHasNestedMount(t.TempDir()); err != nil || nested {
		t.Fatalf("ordinary directory: nested=%v, err=%v", nested, err)
	}
	if nested, err := recursiveScopeHasNestedMount(string(filepath.Separator)); err != nil || !nested {
		t.Fatalf("filesystem root: nested=%v, err=%v", nested, err)
	}
}

func TestRecursiveWatcherEventPathBoundaries(t *testing.T) {
	for _, tc := range []struct {
		parent, child string
		want          bool
	}{
		{"/Project", "/project/classes/Example.cls", true},
		{"/Café", "/Cafe\u0301/Example.cls", true},
		{"/Project", "/Project", true},
		{"/Project", "/ProjectExtra/Example.cls", false},
		{"/Project", "/Other/Example.cls", false},
	} {
		if got := recursivePathContains(tc.parent, tc.child); got != tc.want {
			t.Errorf("recursivePathContains(%q, %q) = %v, want %v", tc.parent, tc.child, got, tc.want)
		}
	}
}
