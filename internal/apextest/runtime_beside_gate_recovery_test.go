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
	"time"
	"weak"

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
	opts := Options{NoDiskCache: true, BuildArtifacts: &artifacts, testBuildHooks: new(runtimeBuildHooks)}
	opts.testBuildHooks.stage = func(stage string) {
		if stage == "after_methods" {
			runtime.Goexit()
		}
	}

	run := RunCasesContext(context.Background(), index, opts, cases)
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
	rejected := newBesideGateStressProject(t, 1)
	badIndex, badArtifacts, badCases := rejected.load(t, 1, 0, true)
	iterations := 2
	if s := os.Getenv("GLADE_BESIDE_GATE_STRESS_ITERATIONS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			t.Fatal("invalid iteration count", s)
		}
		iterations = n
	}
	log := new(besideGateLogBuffer)
	hooks := &runtimeBuildHooks{log: log}

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
	var rejectedBuilds, mutationRounds int
	for i := range iterations {
		if i%4 == 0 {
			if err := startupcache.Clear(p.root, startupcache.SubdirTest); err != nil {
				t.Fatal(err)
			}
		}
		InvalidateRuntimeCaches()
		stop := make(chan struct{})
		done := make(chan int, 1)
		if i%4 == 0 {
			rejectedBuilds++
		}
		go func() {
			if i%4 == 0 {
				report := RunCasesContext(context.Background(), badIndex, Options{BuildArtifacts: &badArtifacts, NoDiskCache: true, testBuildHooks: hooks}, badCases)
				if report.Summary().CompileErrors != 1 {
					t.Errorf("reject: %+v", report.Summary())
				}
			}
			for n := 0; ; n++ {
				undo := SetFileDigestReadHookForTesting(func(string) {})
				restoreClock := setFileDigestClockForTesting(time.Now)
				restore := setSnapshotValidationHookForTesting(func(string) {})
				InvalidateRuntimeCaches()
				// Exercise the same nil-prefix fallback as the prefix neighbor.
				standardRuntimeBase.mu.Lock()
				saved, failed := standardRuntimeBase.prefix, standardRuntimeBase.failed
				standardRuntimeBase.prefix, standardRuntimeBase.failed = weak.Pointer[standardRuntimePrefix]{}, true
				standardRuntimeBase.mu.Unlock()
				clone := prefix.machine.CloneRuntimeFrozenShared(nil)
				_ = clone.RegisterClass(vm.Class{Name: "NeighborOwned"})
				standardRuntimeBase.mu.Lock()
				standardRuntimeBase.prefix, standardRuntimeBase.failed = saved, failed
				standardRuntimeBase.mu.Unlock()
				shells[(i+n)%len(shells)].BuiltClass()
				runtime.KeepAlive(clone)
				restore()
				restoreClock()
				undo()
				select {
				case <-stop:
					done <- n + 1
					return
				default:
				}
			}
		}()
		run := Run(index, Options{SourceDigests: artifacts.SourceDigests, testBuildHooks: hooks})
		close(stop)
		rounds := <-done
		mutationRounds += rounds
		if rounds < 1 || run.Summary().Passed != 1 {
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
	if got := runtimeBesideGateBuildIDs.Load() - started; got != uint64(iterations+rejectedBuilds) {
		t.Fatalf("got %d gated builds, want %d", got, iterations+rejectedBuilds)
	}
	t.Logf("%d exact-key builds, %d rejected neighbors, %d mutation rounds, GOMAXPROCS=%d GOGC=%s", iterations, rejectedBuilds, mutationRounds, runtime.GOMAXPROCS(0), os.Getenv("GOGC"))
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
	opts := Options{BuildArtifacts: &artifacts, NoDiskCache: true, testBuildHooks: new(runtimeBuildHooks)}
	for _, stage := range []string{"compile_method", "scan_standard_fields", "scan_record_types", "worker_exit"} {
		t.Run(stage, func(t *testing.T) {
			InvalidateRuntimeCaches()
			projectReferencedStandardFieldCache.Clear()
			log := new(besideGateLogBuffer)
			opts.testBuildHooks.log = log
			var injected atomic.Bool
			opts.testBuildHooks.stage = func(s string) {
				if stage == "worker_exit" && s == "compile_method" && injected.CompareAndSwap(false, true) {
					runtime.Goexit()
				}
				if stage == s && injected.CompareAndSwap(false, true) {
					panic("worker")
				}
			}

			run := RunCasesContext(context.Background(), index, opts, cases)
			event := "panicked"
			if stage == "worker_exit" {
				event = "exited without a result"
			}
			if !injected.Load() || run.Summary().Passed != 1 || strings.Count(log.String(), event) != 1 || !strings.Contains(log.String(), "TestRuntimeBesideGateWorkerRecovery") {
				t.Fatalf("injected %v: %+v %s", injected.Load(), run.Summary(), log.String())
			}
		})
	}
	for _, at := range []string{"compile_method", "scan_standard_fields", "scan_record_types"} {
		t.Run("cancel_"+at, func(t *testing.T) {
			projectReferencedStandardFieldCache.Clear()
			InvalidateRuntimeCaches()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var started atomic.Int64
			opts.testBuildHooks.stage = func(stage string) {
				if stage == at {
					started.Add(1)
					cancel()
				}
			}

			run := RunCasesContext(ctx, index, opts, cases)
			if n := started.Load(); n < 1 || n > 2 || run.Summary().Passed != 0 {
				t.Fatalf("started %d work items, summary %+v", n, run.Summary())
			}
			assertNoCompiledGenerationCaches(t, buildArtifactRuntimeKey(t, index, &artifacts))
		})
	}
}

func TestRuntimeBesideGateRejectedPartialBuild(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	t.Cleanup(InvalidateRuntimeCaches)
	index, artifacts, cases := besideGateProject(t, "Object o = new MissingBuildType();")
	opts := Options{NoDiskCache: true, BuildArtifacts: &artifacts, testBuildHooks: new(runtimeBuildHooks)}
	old := sharedStandardRuntimePrefix()
	oldTemplates := passiveStandardRuntimeClassTemplates()
	arrived := make(chan struct{})
	opts.testBuildHooks.stage = func(stage string) {
		if stage == "after_methods" {
			close(arrived)
			panic("partial rejected build")
		}
	}
	log := new(besideGateLogBuffer)
	opts.testBuildHooks.log = log

	run := runCasesContextWithSemanticGateHooks(context.Background(), index, opts, cases, semanticGateHooks{afterIdentity: func() { <-arrived }})
	if run.Summary().CompileErrors != 1 || !strings.Contains(log.String(), "(abandoned)") {
		t.Fatalf("summary %+v, %s", run.Summary(), log.String())
	}
	fresh := passiveStandardRuntimeClassTemplates()
	if len(fresh) != len(oldTemplates) || &fresh[0] == &oldTemplates[0] {
		t.Fatal("failed build retained platform templates")
	}
	if sharedStandardRuntimePrefix() == old {
		t.Fatal("failed build retained the shared prefix")
	}
	assertNoCompiledGenerationCaches(t, buildArtifactRuntimeKey(t, index, &artifacts))
}

// Controller fallback can launch its own index and analyzer pools. A project
// with pages stays on the sequential path until those pools can join a build.
func TestRuntimeBesideGateControllerProjectUsesSequentialBuilder(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	t.Cleanup(InvalidateRuntimeCaches)
	p := newBesideGateStressProject(t, 0)
	writeFile(t, filepath.Join(p.root, "force-app", "main", "default", "pages", "Hello.page"), "<apex:page>Hello</apex:page>")
	index, artifacts, cases := p.load(t, 0, 1, false)
	start := runtimeBesideGateBuildIDs.Load()
	r := RunCasesContext(context.Background(), index, Options{BuildArtifacts: &artifacts, NoDiskCache: true}, cases)
	if runtimeBesideGateBuildIDs.Load() != start || r.Summary().Passed != 1 {
		t.Fatalf("build count changed %t: %+v", runtimeBesideGateBuildIDs.Load() != start, r.Summary())
	}
}

func TestGatedPublicationRollbackErrorReachesRun(t *testing.T) {
	enableRuntimeBesideGateForTest(t)
	prev := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(prev)
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	p := newBesideGateStressProject(t, 0)
	index, artifacts, cases := p.load(t, 0, 1, false)
	cacheDir := filepath.Join(p.root, startupcache.SubdirTest)
	var injectErr error
	hooks := runtimeBuildHooks{stage: func(stage string) {
		if stage != "after_publish" {
			return
		}
		// Keep the complete payload in this test's temporary directory. A
		// regular file in its place makes cleanup fail even for privileged users.
		if injectErr = os.Rename(cacheDir, cacheDir+"-saved"); injectErr == nil {
			injectErr = os.WriteFile(cacheDir, []byte("closed"), 0600)
		}
		panic("interrupted disk publication")
	}}
	r := RunCasesContext(context.Background(), index, Options{BuildArtifacts: &artifacts, testBuildHooks: &hooks}, cases)
	if injectErr != nil || r.Summary().CompileErrors != 1 || firstRunProblem(r) == "" {
		t.Fatalf("injection %v: %+v %s", injectErr, r.Summary(), firstRunProblem(r))
	}
}

func TestRuntimePublicationOrderingExpiresWithLastWriter(t *testing.T) {
	var writes []string
	old := orderedPublication("first", func() { writes = append(writes, "old") })
	orderedPublication("second", func() { writes = append(writes, "other") })()
	orderedPublication("first", func() { writes = append(writes, "new") })()
	old()
	if strings.Join(writes, ",") != "other,new" {
		t.Fatal(writes)
	}
	publicationRootsMu.Lock()
	defer publicationRootsMu.Unlock()
	if publicationRoots["first"] != nil || publicationRoots["second"] != nil {
		t.Fatal("completed roots retained publication state")
	}
}
