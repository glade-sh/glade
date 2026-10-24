package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestRunTestMemoryLimitsWarmDiskCacheRoute(t *testing.T) {
	for _, policy := range testMemoryLimitPolicies {
		t.Run(policy.name, func(t *testing.T) {
			for _, api := range []string{"62.0", "67.0"} {
				t.Run(api+"/local_warm_disk_cache", func(t *testing.T) {
					prepareMemoryLimitRoute(t, policy)
					t.Cleanup(apextest.EnableDiskCacheForTesting())
					root := writeMemoryLimitParallelProject(t, api)
					perfPath := filepath.Join(t.TempDir(), "perf.json")
					args := []string{"--project", root, "--no-serve", "--no-progress", "--parallelism", "1", "--no-parallel-methods", "--perf-json", perfPath}
					cold := runMemoryLimitProjectRoute(t, args, perfPath, 4)
					if phases := cold.ApexPerf.Phases; phases.CacheMisses != 1 || phases.DiskCacheHits != 0 || phases.MemoryCacheHits != 0 || phases.CacheEncodeNS <= 0 {
						t.Fatalf("cold runtime phases = %#v, want one persisted miss", phases)
					}
					// Discard only in-process caches so the next invocation must restore disk state.
					apextest.InvalidateRuntimeCaches()
					apextest.ResetPerfCounters()
					debug.SetGCPercent(73)
					warm := runMemoryLimitProjectRoute(t, append(args, "--gc-aggressive"), perfPath, 4)
					if warm.Execution.DiskCachePolicy != apextest.DiskRuntimeCacheEnabled {
						t.Fatalf("disk cache policy = %q, want enabled", warm.Execution.DiskCachePolicy)
					}
					if phases := warm.ApexPerf.Phases; phases.DiskCacheHits != 1 || phases.MemoryCacheHits != 0 || phases.CacheMisses != 0 {
						t.Fatalf("warm runtime phases = %#v, want one disk hit", phases)
					}
					t.Log("verified warm invocation: disk hits=1, memory hits=0, misses=0")
					assertMemoryLimitRoutePolicy(t, policy)
				})
			}
		})
	}
}

func TestRunTestMemoryLimitsParallelMethodAndClassRoutes(t *testing.T) {
	for _, policy := range testMemoryLimitPolicies {
		t.Run(policy.name, func(t *testing.T) {
			for _, api := range []string{"62.0", "67.0"} {
				for _, route := range []struct {
					name    string
					args    []string
					methods bool
					total   int
				}{
					{"parallel_methods", []string{"--parallel-methods", "--class", "AllocationFirstTest"}, true, 2},
					{"parallel_classes", []string{"--no-parallel-methods"}, false, 4},
				} {
					t.Run(api+"/"+route.name, func(t *testing.T) {
						prepareMemoryLimitRoute(t, policy)
						root := writeMemoryLimitParallelProject(t, api)
						perfPath := filepath.Join(t.TempDir(), "perf.json")
						args := []string{"--project", root, "--no-serve", "--no-cache", "--gc-aggressive", "--parallelism", "2", "--progress-json", "--perf-json", perfPath}
						perf := runMemoryLimitProjectRoute(t, append(args, route.args...), perfPath, route.total)
						if perf.Execution.EffectiveParallelism != 2 || perf.Execution.ParallelMethods != route.methods {
							t.Fatalf("parallel execution = %#v", perf.Execution)
						}
						assertMemoryLimitRoutePolicy(t, policy)
					})
				}
			}
		})
	}
}

func TestRunTestMemoryLimitFlagHelpRoutes(t *testing.T) {
	for _, args := range [][]string{
		{"help", "test"},
		{"test", "--help"},
		{"test", "clear-cache", "--help"},
		{"test", "daemon", "--help"},
		{"test", "changed", "--help"},
		{"test", "failed", "--help"},
		{"test", "serve", "--help"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
				t.Fatalf("help exit = %d; stderr=%s", code, &stderr)
			}
			for _, want := range []string{"--gc-aggressive", "soft memory limit instead of heap growth", "falls back to GOGC=50"} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("help missing %q", want)
				}
			}
		})
	}
}

func prepareMemoryLimitRoute(t *testing.T, policy testMemoryLimitPolicy) {
	t.Helper()
	t.Setenv("GOMEMLIMIT", policy.env)
	oldLimit := debug.SetMemoryLimit(policy.limit)
	oldGC := debug.SetGCPercent(73)
	t.Cleanup(func() {
		debug.SetGCPercent(oldGC)
		debug.SetMemoryLimit(oldLimit)
		apextest.InvalidateRuntimeCaches()
	})
	apextest.InvalidateRuntimeCaches()
}

func assertMemoryLimitRoutePolicy(t *testing.T, policy testMemoryLimitPolicy) {
	t.Helper()
	if got := debug.SetGCPercent(policy.gc); got != policy.gc {
		t.Errorf("project route GC percent = %d, want %d", got, policy.gc)
	}
	if got := debug.SetMemoryLimit(-1); got != policy.limit {
		t.Errorf("project route memory limit = %d, want %d", got, policy.limit)
	}
}

func writeMemoryLimitParallelProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	for _, class := range []string{"AllocationFirstTest", "AllocationSecondTest"} {
		writeTestFile(t, filepath.Join(classes, class+".cls"), fmt.Sprintf(`@IsTest private class %s {
	@IsTest static void allocatesFirst() { allocate(); }
	@IsTest static void allocatesSecond() { allocate(); }
	private static void allocate() {
		List<String> values = new List<String>();
		for (Integer i = 0; i < 4000; i++) { values.add('entry-' + i); }
		System.assertEquals(4000, values.size());
	}
}`, class))
		writeTestFile(t, filepath.Join(classes, class+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	return root
}

func runMemoryLimitProjectRoute(t *testing.T, args []string, perfPath string, total int) runPerfSummary {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	run, err := runTest(ctx, args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v; stdout=%s stderr=%s", err, &stdout, &stderr)
	}
	if got := run.Summary(); got.Total != total || got.Passed != total {
		t.Fatalf("project summary = %#v, want %d passing tests; stdout=%s stderr=%s", got, total, &stdout, &stderr)
	}
	for _, arg := range args {
		if arg == "--progress-json" {
			// The CLI increments this count on test_start and decrements on test_done.
			if !strings.Contains(stderr.String(), "2 running") {
				t.Fatalf("parallel route never had two tests in flight: %s", &stderr)
			}
			t.Logf("verified parallel invocation: %d passing tests, two tests in flight", total)
		}
	}
	raw, err := os.ReadFile(perfPath)
	if err != nil {
		t.Fatal(err)
	}
	var perf runPerfSummary
	if err := json.Unmarshal(raw, &perf); err != nil {
		t.Fatal(err)
	}
	return perf
}
