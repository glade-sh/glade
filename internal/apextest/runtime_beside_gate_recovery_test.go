package apextest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRuntimeProjectionPanicReleasesLock(t *testing.T) {
	key := runtimeCacheKey(t.Name())
	for _, recheck := range []bool{false, true} {
		runtimeCacheMu.Lock()
		runtimeCache[key] = runtimeCacheEntry{restored: vm.NewRestoredRuntimeTemplate(storage.NewOrgState(), vm.New(nil))}
		runtimeCacheMu.Unlock()
		project := func(runtimeCacheEntry) (bool, bool) { panic("projection") }
		func() {
			defer func() {
				if recover() != "projection" {
					t.Fatal("projection did not run")
				}
			}()
			if recheck {
				recheckMemoryRuntimeProjectionAfterInvalidObservation(key, project)
			} else {
				validMemoryRuntimeProjection(key, project)
			}
		}()
		if !runtimeCacheMu.TryLock() {
			// Release the leaked lock so a failing test cannot hang its neighbors.
			if recheck {
				runtimeCacheMu.Unlock()
			} else {
				runtimeCacheMu.RUnlock()
			}
			t.Errorf("projection retained the lock (recheck=%t)", recheck)
			runtimeCacheMu.Lock()
		}
		delete(runtimeCache, key)
		runtimeCacheMu.Unlock()
	}
}

type privateBuildError struct{}

func (privateBuildError) RuntimeError() {}
func (privateBuildError) Error() string { panic("must not call an application error method") }

func TestRuntimeBuildPanicLogIgnoresCustomRuntimeError(t *testing.T) {
	var out bytes.Buffer
	logRuntimeBuildPanic(&out, &runtimeBuildPanic{value: privateBuildError{}, stack: []byte("original frame")}, false)
	if !strings.Contains(out.String(), "original frame") || strings.Count(out.String(), "\n") != 1 {
		t.Fatal(out.String())
	}
}

func TestRuntimeBesideGateGoexitRetries(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	t.Cleanup(InvalidateRuntimeCaches)
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	index, artifacts, cases := besideGateProject(t, "System.assert(true);")
	testOnlyRuntimeBesideGateStageHook = func(stage string) {
		if stage == "after_methods" {
			runtime.Goexit()
		}
	}
	defer func() { testOnlyRuntimeBesideGateStageHook = nil }()
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true, BuildArtifacts: &artifacts}, cases)
	if run.Summary().Passed != 1 {
		t.Fatal(firstRunProblem(run))
	}
}

// Exercise the exact-digest, disk-backed Run entry point while another owner
// invalidates caches, restores test observers, and clones the platform prefix.
// Fail on a recovered build panic too: sequential fallback can hide one.
func TestRuntimeBesideGateNeighborStress(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	if runtime.GOMAXPROCS(0) < 2 {
		prev := runtime.GOMAXPROCS(2)
		defer runtime.GOMAXPROCS(prev)
	}
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	p := newBesideGateStressProject(t, 0)
	index, artifacts, _ := p.load(t, 0, 1, false)
	want := RuntimeContentKey(index, artifacts.SourceDigests)
	iterations := 2
	if s := os.Getenv("GLADE_BESIDE_GATE_STRESS_ITERATIONS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			t.Fatal("invalid iteration count", s)
		}
		iterations = n
	}
	log := new(besideGateLogBuffer)
	testOnlyRuntimeBesideGateLog = log
	defer func() { testOnlyRuntimeBesideGateLog = nil }()
	prefix := sharedStandardRuntimePrefix()
	var shells []vm.Class
	for _, c := range prefix.classes {
		if c.IsLazy() {
			shells = append(shells, c)
		}
	}
	if len(shells) == 0 {
		t.Fatal("no platform shells")
	}
	started := runtimeBesideGateBuildIDs.Load()
	for i := range iterations {
		if i%4 == 0 {
			if err := startupcache.Clear(p.root, startupcache.SubdirTest); err != nil {
				t.Fatal(err)
			}
		}
		InvalidateRuntimeCaches()
		stop := make(chan struct{})
		done := make(chan int, 1)
		go func() {
			for n := 0; ; n++ {
				undo := SetFileDigestReadHookForTesting(func(string) {})
				restore := setSnapshotValidationHookForTesting(func(string) {})
				InvalidateRuntimeCaches()
				clone := prefix.machine.CloneRuntimeFrozenShared(nil)
				shells[(i+n)%len(shells)].BuiltClass()
				runtime.KeepAlive(clone)
				restore()
				undo()
				select {
				case <-stop:
					done <- n + 1
					return
				default:
				}
			}
		}()
		run := Run(index, Options{SourceDigests: artifacts.SourceDigests})
		close(stop)
		if <-done < 1 || run.Summary().Passed != 1 {
			t.Fatal(firstRunProblem(run))
		}
		entry, err := startupcache.Read(p.root, startupcache.SubdirTest)
		if err != nil || entry == nil || entry.RuntimeKey != want {
			t.Fatalf("exact-key read failed: %v", err)
		}
		if s := log.String(); s != "" {
			t.Fatal(s)
		}
	}
	if got := runtimeBesideGateBuildIDs.Load() - started; got != uint64(iterations) {
		t.Fatalf("got %d gated builds, want %d", got, iterations)
	}
	t.Logf("%d exact-key builds, GOMAXPROCS=%d GOGC=%s", iterations, runtime.GOMAXPROCS(0), os.Getenv("GOGC"))
}

// A large project puts all three scans above the real worker-pool threshold.
func TestRuntimeBesideGateWorkerRecovery(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	t.Cleanup(InvalidateRuntimeCaches)
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	p := newBesideGateStressProject(t, 0)
	for i := range 64 {
		name := fmt.Sprintf("CompileHelper%d", i)
		writeFile(t, filepath.Join(filepath.Dir(p.helper), name+".cls"), "public class "+name+" { public static Integer value() { return 1; } }")
	}
	index, artifacts, cases := p.load(t, 0, 1, false)
	opts := Options{BuildArtifacts: &artifacts, NoDiskCache: true}
	for _, stage := range []string{"compile_method", "scan_standard_fields", "scan_record_types"} {
		t.Run(stage, func(t *testing.T) {
			InvalidateRuntimeCaches()
			projectReferencedStandardFieldCache.Clear()
			log := new(besideGateLogBuffer)
			testOnlyRuntimeBesideGateLog = log
			var injected atomic.Bool
			testOnlyRuntimeBesideGateStageHook = func(s string) {
				if stage == s && injected.CompareAndSwap(false, true) {
					panic("worker")
				}
			}
			defer func() { testOnlyRuntimeBesideGateLog = nil; testOnlyRuntimeBesideGateStageHook = nil }()
			run := RunCasesContext(context.Background(), index, opts, cases)
			if !injected.Load() || run.Summary().Passed != 1 || strings.Count(log.String(), "panicked") != 1 || !strings.Contains(log.String(), "TestRuntimeBesideGateWorkerRecovery") {
				t.Fatalf("injected %v: %+v %s", injected.Load(), run.Summary(), log.String())
			}
		})
	}
	t.Run("cancel", func(t *testing.T) {
		InvalidateRuntimeCaches()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var started atomic.Int64
		testOnlyRuntimeBesideGateStageHook = func(stage string) {
			if stage == "compile_method" {
				started.Add(1)
				cancel()
			}
		}
		defer func() { testOnlyRuntimeBesideGateStageHook = nil }()
		run := RunCasesContext(ctx, index, opts, cases)
		if n := started.Load(); n < 1 || n > 2 || run.Summary().Passed != 0 {
			t.Fatalf("started %d methods, summary %+v", n, run.Summary())
		}
		assertNoCompiledGenerationCaches(t, buildArtifactRuntimeKey(t, index, &artifacts))
	})
}

func TestRuntimeBesideGateRejectedPartialBuild(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	t.Cleanup(InvalidateRuntimeCaches)
	index, artifacts, cases := besideGateProject(t, "Object o = new MissingBuildType();")
	opts := Options{NoDiskCache: true, BuildArtifacts: &artifacts}
	old := sharedStandardRuntimePrefix()
	arrived := make(chan struct{})
	testOnlyRuntimeBesideGateStageHook = func(stage string) {
		if stage == "after_methods" {
			close(arrived)
			panic("partial rejected build")
		}
	}
	log := new(besideGateLogBuffer)
	testOnlyRuntimeBesideGateLog = log
	defer func() { testOnlyRuntimeBesideGateStageHook = nil; testOnlyRuntimeBesideGateLog = nil }()
	run := runCasesContextWithSemanticGateHooks(context.Background(), index, opts, cases, semanticGateHooks{afterIdentity: func() { <-arrived }})
	if run.Summary().CompileErrors != 1 || !strings.Contains(log.String(), "(abandoned)") {
		t.Fatalf("summary %+v, %s", run.Summary(), log.String())
	}
	if sharedStandardRuntimePrefix() == old {
		t.Fatal("failed build retained the shared prefix")
	}
	assertNoCompiledGenerationCaches(t, buildArtifactRuntimeKey(t, index, &artifacts))
}
