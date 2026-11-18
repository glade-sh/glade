package gladecli

import (
	"context"
	"errors"
	"github.com/glade-sh/glade/internal/startupcache"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDAPLaunchCancelEndsWhileProjectLoadIsRunning(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	finished := make(chan struct{})
	previous := dapStartupLoader
	dapStartupLoader = func(context.Context, string) dapStartup {
		close(started)
		<-release
		close(finished)
		return dapStartup{err: errors.New("released")}
	}
	t.Cleanup(func() {
		dapStartupLoader = previous
		select {
		case <-release:
		default:
			close(release)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		handler := dap.NewHandler(dap.Snapshot{})
		result <- handleDAPLaunch(ctx, handler, dap.LaunchRequest{AnonymousBody: "System.debug(1);"}, t.TempDir(), "", false)
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("launch error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("launch stayed blocked in the project load after cancel")
	}
	select {
	case <-finished:
		t.Fatal("the load should still be running")
	default:
	}
	close(release)
	<-finished
}

func TestDAPTestLaunchBuildsTheProjectIndexOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	project := filepath.Join(root, "project")
	writeTestFile(t, filepath.Join(project, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/Adder.cls"), "public class Adder { public static Integer add(Integer a, Integer b) { return a + b; } }")
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/AdderTest.cls"), "@IsTest private class AdderTest { @IsTest static void adds() { System.assertEquals(3, Adder.add(1, 2)); } }")

	extraLoads := 0
	previous := dapTestIndexLoader
	dapTestIndexLoader = func(root string) (typesys.Index, error) {
		extraLoads++
		return previous(root)
	}
	t.Cleanup(func() { dapTestIndexLoader = previous })

	launch := func() {
		t.Helper()
		handler := dap.NewHandler(dap.Snapshot{})
		err := handleDAPLaunch(context.Background(), handler, dap.LaunchRequest{ClassName: "AdderTest", MethodName: "adds", AnonymousBody: "System.debug(1);"}, project, "", true)
		if err != nil {
			t.Fatal(err)
		}
	}
	launch() // startup cache miss: the index built for the cache is reused
	if extraLoads != 0 {
		t.Fatalf("cold test launch built the index %d extra times", extraLoads)
	}
	launch() // startup cache hit: no index in hand, so one is built
	if extraLoads != 1 {
		t.Fatalf("warm test launch built the index %d times, want 1", extraLoads)
	}
}

func TestCancelledStartupLoadDoesNotPublishCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	project := filepath.Join(root, "project")
	writeTestFile(t, filepath.Join(project, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/A.cls"), "public class A { public static Integer v() { return 1; } }")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, _, err := loadDAPStartupStateWithIndex(ctx, project); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled load error = %v", err)
	}
	cache, err := startupcache.Read(project, startupcache.SubdirDAP)
	if err == nil && cache != nil && startupcache.Fresh(cache, project, dapCacheVersion) {
		t.Fatal("a cancelled load published a fresh startup cache")
	}
}

func TestDAPLoadsForOneRootRunOneAtATime(t *testing.T) {
	var running, peak, started atomic.Int32
	release := make(chan struct{})
	previous := dapStartupLoader
	dapStartupLoader = func(context.Context, string) dapStartup {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		started.Add(1)
		<-release
		running.Add(-1)
		return dapStartup{err: errors.New("done")}
	}
	t.Cleanup(func() { dapStartupLoader = previous })
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			_, _ = awaitDAPStartup(ctx, root)
		}()
	}
	wg.Wait()
	close(release)
	if peak.Load() != 1 || started.Load() != 1 {
		t.Fatalf("concurrent loads peak=%d started=%d, want 1 and 1", peak.Load(), started.Load())
	}
}

func TestWarmTestLaunchCancelEndsDuringIndexBuild(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	previousIdx, previousStart := dapTestIndexLoader, dapStartupLoader
	dapStartupLoader = func(context.Context, string) dapStartup { return dapStartup{} } // cache hit: no index
	dapTestIndexLoader = func(string) (typesys.Index, error) {
		close(started)
		<-release
		return typesys.Index{}, nil
	}
	t.Cleanup(func() { dapTestIndexLoader, dapStartupLoader = previousIdx, previousStart; close(release) })
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	handler := dap.NewHandler(dap.Snapshot{})
	go func() {
		result <- handleDAPLaunch(ctx, handler, dap.LaunchRequest{ClassName: "T", MethodName: "m", AnonymousBody: "System.debug(1);"}, t.TempDir(), "", false)
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("warm test launch stayed blocked in the index build after cancel")
	}
}

func TestDAPLoadSlotsShareOneProjectAcrossSpellings(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	release := make(chan struct{})
	var started atomic.Int32
	previous := dapStartupLoader
	dapStartupLoader = func(context.Context, string) dapStartup {
		started.Add(1)
		<-release
		return dapStartup{err: errors.New("done")}
	}
	t.Cleanup(func() { dapStartupLoader = previous })
	spellings := []string{root, link, filepath.Join(root, ".", "x", ".."), link + "/"}
	var wg sync.WaitGroup
	for _, spelling := range spellings {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			_, _ = awaitDAPStartup(ctx, spelling)
		}()
	}
	wg.Wait()
	close(release)
	if started.Load() != 1 {
		t.Fatalf("loaders started = %d for one project, want 1", started.Load())
	}
}

// A load that is cancelled while it waits for the publish lock must not write.
func TestCancelDuringPublishLockWaitSkipsWrite(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	project := filepath.Join(root, "project")
	writeTestFile(t, filepath.Join(project, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/A.cls"), "public class A {}")
	normalized, err := normalizedDAPRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	lock := dapPublishLock(normalized)
	lock.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, _, _, err := loadDAPStartupStateWithIndex(ctx, project)
		done <- err
	}()
	time.Sleep(300 * time.Millisecond) // the load is built and waits at the lock
	cancel()
	lock.Unlock()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
	if cache, err := startupcache.Read(project, startupcache.SubdirDAP); err == nil && cache != nil {
		t.Fatal("cache was written after cancel")
	}
}
