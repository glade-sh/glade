package semanticcache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

func TestManagerExactHitReturnsIsolatedCompleteResult(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	identity := testIdentity()
	want := sema.Result{
		Diagnostics: []diagnostic.Diagnostic{{Message: "original"}},
		Types:       map[string]sema.TypeReference{"Account": {Name: "Account", Kind: sema.TypeSchema}},
	}
	var builds atomic.Int64
	build := func() (sema.Result, error) {
		builds.Add(1)
		return want, nil
	}

	first, firstAccess, err := manager.GetOrCompute(context.Background(), Request{Identity: identity}, build)
	if err != nil {
		t.Fatal(err)
	}
	second, secondAccess, err := manager.GetOrCompute(context.Background(), Request{Identity: identity}, build)
	if err != nil {
		t.Fatal(err)
	}
	if firstAccess.Source != SourceBuild || secondAccess.Source != SourceMemory || builds.Load() != 1 {
		t.Fatalf("accesses = %#v, %#v; builds = %d", firstAccess, secondAccess, builds.Load())
	}
	second.Diagnostics[0].Message = "mutated"
	second.Types["Account"] = sema.TypeReference{Name: "mutated"}
	if first.Diagnostics[0].Message != "original" || first.Types["Account"].Name != "Account" {
		t.Fatalf("cache result aliases caller mutation: %#v", first)
	}
}

func TestManagerConcurrentExactRequestsSingleflight(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	identity := testIdentity()
	release := make(chan struct{})
	var builds atomic.Int64
	build := func() (sema.Result, error) {
		builds.Add(1)
		<-release
		return sema.Result{Summary: sema.Summary{Types: 1}}, nil
	}

	const callers = 8
	start := make(chan struct{})
	results := make(chan Access, callers)
	errs := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			_, access, err := manager.GetOrCompute(context.Background(), Request{Identity: identity}, build)
			results <- access
			errs <- err
		}()
	}
	ready.Wait()
	close(start)
	for builds.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(release)

	waiters := 0
	for range callers {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		if access := <-results; access.Waited {
			waiters++
		}
	}
	if builds.Load() != 1 || waiters == 0 {
		t.Fatalf("builds = %d, waiters = %d", builds.Load(), waiters)
	}
}

func TestManagerLeaderCancellationDoesNotCancelSharedWork(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, _, err := manager.GetOrCompute(ctx, Request{Identity: testIdentity()}, func() (sema.Result, error) {
			close(started)
			<-release
			close(completed)
			return sema.Result{Summary: sema.Summary{Types: 1}}, nil
		})
		resultCh <- err
	}()
	<-started
	cancel()
	if err := <-resultCh; err != context.Canceled {
		t.Fatalf("leader error = %v, want context canceled", err)
	}

	follower := make(chan error, 1)
	go func() {
		result, access, err := manager.GetOrCompute(context.Background(), Request{Identity: testIdentity()}, func() (sema.Result, error) {
			t.Error("follower started a second build")
			return sema.Result{}, nil
		})
		if err == nil && (result.Summary.Types != 1 || !access.Waited) {
			err = fmt.Errorf("follower result/access = %#v %#v", result, access)
		}
		follower <- err
	}()
	select {
	case err := <-follower:
		t.Fatalf("follower returned before shared build completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-completed
	if err := <-follower; err != nil {
		t.Fatal(err)
	}
}

func TestManagerResetFencesActiveFlightPublication(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	root := t.TempDir()
	const relativePath = ".glade/semantic/reset.json"
	request := Request{Identity: testIdentity(), ProjectRoot: root, RelativePath: relativePath}
	started := make(chan struct{})
	release := make(chan struct{})
	oldDone := make(chan error, 1)
	go func() {
		_, _, err := manager.GetOrCompute(context.Background(), request, func() (sema.Result, error) {
			close(started)
			<-release
			return sema.Result{Summary: sema.Summary{Types: 1}}, nil
		})
		oldDone <- err
	}()
	<-started
	manager.Reset()

	current, _, err := manager.GetOrCompute(context.Background(), request, func() (sema.Result, error) {
		return sema.Result{Summary: sema.Summary{Types: 2}}, nil
	})
	if err != nil || current.Summary.Types != 2 {
		t.Fatalf("current result = %#v, %v", current, err)
	}
	close(release)
	if err := <-oldDone; err != nil {
		t.Fatal(err)
	}
	disk, err := Load(root, relativePath, request.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if disk.Summary.Types != 2 {
		t.Fatalf("stale pre-reset flight overwrote disk result: %#v", disk)
	}
	hit, access, err := manager.GetOrCompute(context.Background(), request, func() (sema.Result, error) {
		t.Fatal("memory result was not retained")
		return sema.Result{}, nil
	})
	if err != nil || hit.Summary.Types != 2 || access.Source != SourceMemory {
		t.Fatalf("post-reset memory result = %#v %#v %v", hit, access, err)
	}
}

func TestManagerNoDiskSkipsReadAndWrite(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	root := t.TempDir()
	const relativePath = ".glade/semantic/result.json"
	_, access, err := manager.GetOrCompute(context.Background(), Request{
		Identity:     testIdentity(),
		ProjectRoot:  root,
		RelativePath: relativePath,
		NoDisk:       true,
		BypassMemory: true,
	}, func() (sema.Result, error) {
		return sema.Result{Summary: sema.Summary{Types: 1}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if access.Source != SourceBuild || access.DiskMissReason != "" {
		t.Fatalf("access = %#v", access)
	}
	if _, err := Load(root, relativePath, testIdentity()); err == nil {
		t.Fatal("no-disk request wrote a semantic cache file")
	}
	_, _, err = manager.GetOrCompute(context.Background(), Request{
		Identity:     testIdentity(),
		ProjectRoot:  root,
		RelativePath: relativePath,
		NoDisk:       true,
		BypassMemory: true,
	}, func() (sema.Result, error) {
		return sema.Result{Summary: sema.Summary{Types: 2}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.Stats().Entries != 0 {
		t.Fatal("no-cache request retained an in-memory result")
	}
}

func TestManagerBypassMemoryHonorsCancellationWithoutPublishing(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := manager.GetOrCompute(ctx, Request{
		Identity:     testIdentity(),
		BypassMemory: true,
	}, func() (sema.Result, error) {
		t.Fatal("cancelled bypass request computed")
		return sema.Result{}, nil
	})
	if err != context.Canceled {
		t.Fatalf("bypass error = %v, want context canceled", err)
	}
	if stats := manager.Stats(); stats.Entries != 0 || stats.RetainedBytes != 0 {
		t.Fatalf("cancelled bypass retained state: %#v", stats)
	}
}

func TestManagerBypassCancellationDoesNotDetachCompute(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	started := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := manager.GetOrCompute(ctx, Request{
			Identity:     testIdentity(),
			BypassMemory: true,
		}, func() (sema.Result, error) {
			close(started)
			<-release
			return sema.Result{}, nil
		})
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		t.Fatalf("bypass request detached its compute: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != context.Canceled {
		t.Fatalf("bypass error = %v, want context canceled", err)
	}
}

func TestManagerWaitDrainsCancelledLeaderWork(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: 1 << 20})
	started := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	requestDone := make(chan error, 1)
	go func() {
		_, _, err := manager.GetOrCompute(ctx, Request{Identity: testIdentity()}, func() (sema.Result, error) {
			close(started)
			<-release
			return sema.Result{}, nil
		})
		requestDone <- err
	}()
	<-started
	cancel()
	if err := <-requestDone; err != context.Canceled {
		t.Fatalf("leader error = %v, want context canceled", err)
	}
	drained := make(chan struct{})
	go func() {
		manager.Wait()
		close(drained)
	}()
	select {
	case <-drained:
		t.Fatal("Wait returned before detached work completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after detached work completed")
	}
}

func TestManagerBoundedLRUEviction(t *testing.T) {
	manager := NewManager(Limits{MaxEntries: 2, MaxBytes: 1 << 20})
	for i := 0; i < 3; i++ {
		identity := testIdentity()
		identity.ProjectContentSHA256 = string(rune('a'+i)) + identity.ProjectContentSHA256[1:]
		_, _, err := manager.GetOrCompute(context.Background(), Request{Identity: identity}, func() (sema.Result, error) {
			return sema.Result{Summary: sema.Summary{Types: i}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	stats := manager.Stats()
	if stats.Entries != 2 || stats.Evictions != 1 || stats.RetainedBytes <= 0 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestManagerByteLimitIncludesRetainedContainers(t *testing.T) {
	result := sema.Result{
		Diagnostics: []diagnostic.Diagnostic{{Range: &diagnostic.Range{}, Message: "x"}},
		Types:       map[string]sema.TypeReference{"Account": {Name: "Account", Kind: sema.TypeSchema}},
	}
	encoded, err := json.Marshal(sema.SnapshotResult(result))
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Limits{MaxEntries: 8, MaxBytes: int64(len(encoded))})
	_, _, err = manager.GetOrCompute(context.Background(), Request{Identity: testIdentity()}, func() (sema.Result, error) {
		return result, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	stats := manager.Stats()
	if stats.Entries != 0 || stats.Evictions != 1 {
		t.Fatalf("container overhead did not participate in byte bound: %#v", stats)
	}
}
