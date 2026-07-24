package apextest

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestParallelMethodDiskRuntimeGuardRemainsEnabled(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want bool
	}{
		{name: "serial-enabled", opts: Options{ParallelMethods: true, Parallelism: 1}, want: true},
		{name: "workers-2-disabled", opts: Options{ParallelMethods: true, Parallelism: 2}},
		{name: "workers-4-disabled", opts: Options{ParallelMethods: true, Parallelism: 4}},
		{name: "workers-8-disabled", opts: Options{ParallelMethods: true, Parallelism: 8}},
		{name: "workers-2-restored-opt-in", opts: Options{ParallelMethods: true, Parallelism: 2, RestoredRuntimeMultiWorker: true}, want: true},
		{name: "workers-4-restored-opt-in", opts: Options{ParallelMethods: true, Parallelism: 4, RestoredRuntimeMultiWorker: true}, want: true},
		{name: "workers-8-restored-opt-in", opts: Options{ParallelMethods: true, Parallelism: 8, RestoredRuntimeMultiWorker: true}, want: true},
		{name: "explicit-no-disk-serial", opts: Options{Parallelism: 1, NoDiskCache: true}},
		{name: "explicit-no-disk-parallel", opts: Options{ParallelMethods: true, Parallelism: 8, NoDiskCache: true}},
		{name: "explicit-no-disk-restored-opt-in", opts: Options{ParallelMethods: true, Parallelism: 8, NoDiskCache: true, RestoredRuntimeMultiWorker: true}},
	}
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled) })
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := useDiskRuntimeCache(test.opts); got != test.want {
				t.Fatalf("useDiskRuntimeCache(%#v) = %t, want %t", test.opts, got, test.want)
			}
		})
	}
	disableDiskCache.Store(true)
	if got := useDiskRuntimeCache(Options{ParallelMethods: true, Parallelism: 8, RestoredRuntimeMultiWorker: true}); got {
		t.Fatal("restored multiworker opt-in bypassed the global disk-cache disable")
	}
}

func TestDiskRuntimeIsolationDeterministicWorkers(t *testing.T) {
	fixture := buildDiskRuntimeIsolationFixture(t)
	want := runDiskRuntimeIsolationState(t, fixture, "built-no-disk", 1, fixture.cases, nil)
	for _, workers := range []int{1, 2, 4, 8} {
		for _, state := range diskRuntimeIsolationStates(workers) {
			t.Run(fmt.Sprintf("%s/workers-%d", state, workers), func(t *testing.T) {
				got := runDiskRuntimeIsolationState(t, fixture, state, workers, fixture.cases, nil)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("canonical result differs from worker-1 build oracle:\n got: %#v\nwant: %#v", got, want)
				}
			})
		}
	}
}

func TestDiskRuntimeIsolationRandomizedWorkers(t *testing.T) {
	// Randomize discovery order and scheduling priorities. VM random-state
	// isolation is covered separately with the other mutable runtime state.
	fixture := buildDiskRuntimeIsolationFixture(t)
	want := runDiskRuntimeIsolationState(t, fixture, "built-no-disk", 1, fixture.cases, nil)
	for _, seed := range []int64{11, 29, 47, 83} {
		cases := append([]TestCase(nil), fixture.cases...)
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(cases), func(i, j int) { cases[i], cases[j] = cases[j], cases[i] })
		durations := make(map[string]int64, len(cases))
		priorities := rng.Perm(len(cases))
		for i, testCase := range cases {
			durations[testCase.ClassName+"."+testCase.MethodName] = int64(priorities[i] + 1)
		}
		for _, workers := range []int{1, 2, 4, 8} {
			for _, state := range diskRuntimeIsolationStates(workers) {
				t.Run(fmt.Sprintf("seed-%d/%s/workers-%d", seed, state, workers), func(t *testing.T) {
					got := runDiskRuntimeIsolationState(t, fixture, state, workers, cases, durations)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("canonical randomized result differs from oracle:\n got: %#v\nwant: %#v", got, want)
					}
				})
			}
		}
	}
}

func TestRestoredRuntimeMultiWorkerCorruptDiskFallsBack(t *testing.T) {
	fixture := buildDiskRuntimeIsolationFixture(t)
	want := runDiskRuntimeIsolationState(t, fixture, "built-no-disk", 1, fixture.cases, nil)
	InvalidateRuntimeCaches()
	headerPath := filepath.Join(fixture.index.Project.Root, ".glade", "test", "startup.meta.json")
	if err := os.WriteFile(headerPath, []byte("{malformed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ResetPerfCounters()
	run := RunCasesContext(context.Background(), fixture.index, Options{
		ParallelMethods:            true,
		Parallelism:                4,
		RestoredRuntimeMultiWorker: true,
		PerfCounters:               true,
		SourceDigests:              fixture.digests,
	}, fixture.cases)
	assertDiskRuntimeIsolationPass(t, run)
	if got := canonicalDiskRuntimeIsolationResult(run); !reflect.DeepEqual(got, want) {
		t.Fatalf("corrupt-disk fallback differs from oracle:\n got: %#v\nwant: %#v", got, want)
	}
	phases := SnapshotPerfCounters().Phases
	if phases.DiskCacheHits != 0 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 1 ||
		phases.CacheValidateNS <= 0 || phases.CacheDecodeNS != 0 || phases.CacheEncodeNS <= 0 {
		t.Fatalf("corrupt-disk fallback phases = %#v, want validation, one repaired miss, and no cache hit", phases)
	}
}

func TestRestoredRuntimeMultiWorkerDefaultRemainsOff(t *testing.T) {
	fixture := buildDiskRuntimeIsolationFixture(t)
	want := runDiskRuntimeIsolationState(t, fixture, "built-no-disk", 1, fixture.cases, nil)
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	run := RunCasesContext(context.Background(), fixture.index, Options{
		ParallelMethods: true,
		Parallelism:     4,
		PerfCounters:    true,
		SourceDigests:   fixture.digests,
	}, fixture.cases)
	assertDiskRuntimeIsolationPass(t, run)
	if got := canonicalDiskRuntimeIsolationResult(run); !reflect.DeepEqual(got, want) {
		t.Fatalf("default-off result differs from oracle:\n got: %#v\nwant: %#v", got, want)
	}
	phases := SnapshotPerfCounters().Phases
	if phases.DiskCacheHits != 0 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 1 ||
		phases.CacheValidateNS != 0 || phases.CacheDecodeNS != 0 {
		t.Fatalf("default-off phases = %#v, want one build miss without disk access", phases)
	}
}

func diskRuntimeIsolationStates(workers int) []string {
	states := []string{"built-no-disk", "memory-warm", "restored-preloaded"}
	if workers > 1 {
		states = append(states, "restored-opt-in")
	}
	return states
}

type diskRuntimeIsolationFixture struct {
	index   typesys.Index
	digests *typesys.SourceDigestSet
	cases   []TestCase
}

func buildDiskRuntimeIsolationFixture(t *testing.T) diskRuntimeIsolationFixture {
	t.Helper()
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
		ResetPerfCounters()
	})
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/IsolationProbe.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DiskIsolationState.cls"), `
public class DiskIsolationState {
  public static Integer counter = 0;
  public static List<String> markers = new List<String>();
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DiskIsolationJob.cls"), `
public class DiskIsolationJob implements Queueable {
  private String marker;
  public DiskIsolationJob(String marker) { this.marker = marker; }
  public void execute(QueueableContext context) {
    insert new Account(Name = 'async-' + marker);
  }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/DiskRuntimeIsolationTest.cls"), `
@isTest
private class DiskRuntimeIsolationTest {
  private static void exercise(String marker) {
    System.assertEquals(0, [SELECT COUNT() FROM Account]);
    System.assertEquals(0, DiskIsolationState.counter);
    System.assertEquals(0, DiskIsolationState.markers.size());
    System.assert(!ApexPages.hasMessages());
    System.assertEquals(null, ApexPages.currentPage().getParameters().get('marker'));

    DiskIsolationState.counter = DiskIsolationState.counter + 1;
    DiskIsolationState.markers.add(marker);
    insert new Account(Name = marker);
    System.assertEquals(1, Limits.getDmlStatements());
    System.assertEquals(1, [SELECT COUNT() FROM Account]);

    Test.setCurrentPage(Page.IsolationProbe);
    ApexPages.currentPage().getParameters().put('marker', marker);
    ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.ERROR, marker));
    System.assert(ApexPages.hasMessages());
    System.assertEquals(marker, ApexPages.currentPage().getParameters().get('marker'));

    String firstRandom = UUID.randomUUID().toString();
    String secondRandom = UUID.randomUUID().toString();
    System.assertNotEquals(firstRandom, secondRandom);

    Test.startTest();
    System.enqueueJob(new DiskIsolationJob(marker));
    Test.stopTest();
    System.assertEquals(2, [SELECT COUNT() FROM Account]);
    System.assertEquals(1, DiskIsolationState.counter);
    System.assertEquals(marker, DiskIsolationState.markers.get(0));
  }

  @isTest static void methodA() { exercise('A'); }
  @isTest static void methodB() { exercise('B'); }
  @isTest static void methodC() { exercise('C'); }
  @isTest static void methodD() { exercise('D'); }
  @isTest static void methodE() { exercise('E'); }
  @isTest static void methodF() { exercise('F'); }
  @isTest static void methodG() { exercise('G'); }
  @isTest static void methodH() { exercise('H'); }
}
`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	if len(index.Diagnostics) != 0 {
		t.Fatalf("fixture diagnostics: %#v", index.Diagnostics)
	}
	fixture := diskRuntimeIsolationFixture{
		index:   index,
		digests: artifacts.SourceDigests,
		cases:   Discover(index, Options{SelectedClasses: []string{"DiskRuntimeIsolationTest"}}),
	}
	if len(fixture.cases) != 8 {
		t.Fatalf("fixture cases = %d, want 8", len(fixture.cases))
	}
	persistDiskRuntimeIsolation(t, fixture)
	return fixture
}

func persistDiskRuntimeIsolation(t *testing.T, fixture diskRuntimeIsolationFixture) {
	t.Helper()
	InvalidateRuntimeCaches()
	counters := newRunPerfCounters(true)
	_, _, err := runtimeFromIndexWithSourceDigestsAndPerf(fixture.index, fixture.digests, newSourceCache(), true, counters)
	if err != nil {
		t.Fatal(err)
	}
	phases := snapshotPerfCounters(counters).Phases
	if phases.CacheMisses != 1 || phases.DiskCacheHits != 0 || phases.MemoryCacheHits != 0 || phases.CacheEncodeNS <= 0 {
		t.Fatalf("persist phases = %#v", phases)
	}
	InvalidateRuntimeCaches()
}

func runDiskRuntimeIsolationState(t *testing.T, fixture diskRuntimeIsolationFixture, state string, workers int, cases []TestCase, durations map[string]int64) []diskRuntimeIsolationResult {
	t.Helper()
	InvalidateRuntimeCaches()
	switch state {
	case "built-no-disk":
	case "memory-warm":
		prime := RunCasesContext(context.Background(), fixture.index, Options{
			ParallelMethods: true, Parallelism: workers, NoDiskCache: true, SourceDigests: fixture.digests,
		}, cases)
		assertDiskRuntimeIsolationPass(t, prime)
	case "restored-preloaded":
		counters := newRunPerfCounters(true)
		_, _, err := runtimeFromIndexWithSourceDigestsAndPerf(fixture.index, fixture.digests, newSourceCache(), true, counters)
		if err != nil {
			t.Fatal(err)
		}
		phases := snapshotPerfCounters(counters).Phases
		if phases.DiskCacheHits != 1 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 0 {
			t.Fatalf("restore phases = %#v", phases)
		}
	case "restored-opt-in":
	default:
		t.Fatalf("unknown runtime state %q", state)
	}

	ResetPerfCounters()
	opts := Options{
		ParallelMethods:            true,
		Parallelism:                workers,
		NoDiskCache:                state == "built-no-disk",
		RestoredRuntimeMultiWorker: state == "restored-opt-in",
		MethodDurationMS:           durations,
		PerfCounters:               true,
		SourceDigests:              fixture.digests,
	}
	run := RunCasesContext(context.Background(), fixture.index, opts, cases)
	assertDiskRuntimeIsolationPass(t, run)
	phases := SnapshotPerfCounters().Phases
	switch state {
	case "built-no-disk":
		if phases.CacheMisses != 1 || phases.MemoryCacheHits != 0 || phases.DiskCacheHits != 0 {
			t.Fatalf("built phases = %#v", phases)
		}
	case "memory-warm", "restored-preloaded":
		if phases.MemoryCacheHits != 1 || phases.DiskCacheHits != 0 || phases.CacheMisses != 0 {
			t.Fatalf("preloaded phases = %#v", phases)
		}
	case "restored-opt-in":
		if phases.DiskCacheHits != 1 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 0 {
			t.Fatalf("restored opt-in phases = %#v", phases)
		}
	}
	return canonicalDiskRuntimeIsolationResult(run)
}

func assertDiskRuntimeIsolationPass(t *testing.T, run testreport.Run) {
	t.Helper()
	if summary := run.Summary(); summary.Total != 8 || summary.Passed != 8 {
		t.Fatalf("isolation summary = %#v problem=%q", summary, firstRunProblem(run))
	}
}

type diskRuntimeIsolationResult struct {
	Class   string
	Method  string
	Status  testreport.Status
	Problem string
}

func canonicalDiskRuntimeIsolationResult(run testreport.Run) []diskRuntimeIsolationResult {
	results := make([]diskRuntimeIsolationResult, 0, run.Summary().Total)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			problem := ""
			if testCase.Problem != nil {
				problem = testCase.Problem.Type + ":" + testCase.Problem.Message
			}
			results = append(results, diskRuntimeIsolationResult{
				Class: testCase.ClassName, Method: testCase.MethodName, Status: testCase.Status, Problem: problem,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Class == results[j].Class {
			return results[i].Method < results[j].Method
		}
		return results[i].Class < results[j].Class
	})
	return results
}
