package apextest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

const (
	parallelRestoredRuntimeBenchmarkProjectEnv = "GLADE_APEXTEST_BENCH_PROJECT"
	parallelRestoredRuntimeBenchmarkFilterEnv  = "GLADE_APEXTEST_BENCH_FILTER"
)

func BenchmarkParallelRestoredRuntimeModes(b *testing.B) {
	root := strings.TrimSpace(os.Getenv(parallelRestoredRuntimeBenchmarkProjectEnv))
	if root == "" {
		b.Skipf("set %s to a Salesforce project root", parallelRestoredRuntimeBenchmarkProjectEnv)
	}
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	for _, diag := range index.Diagnostics {
		if diag.Severity == diagnostic.Error {
			b.Fatalf("benchmark project error diagnostics: %#v", index.Diagnostics)
		}
	}
	cases := Discover(index, Options{Filter: strings.TrimSpace(os.Getenv(parallelRestoredRuntimeBenchmarkFilterEnv))})
	methodsByClass := make(map[string]int)
	for _, testCase := range cases {
		methodsByClass[testCase.ClassName]++
	}
	hasParallelClass := false
	for _, methods := range methodsByClass {
		if methods > 1 {
			hasParallelClass = true
			break
		}
	}
	if len(cases) < 2 || !hasParallelClass {
		b.Skip("benchmark selection must include at least two methods from one Apex test class")
	}

	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	b.Cleanup(func() {
		disableDiskCache.Store(wasDisabled)
		InvalidateRuntimeCaches()
		ResetPerfCounters()
	})
	InvalidateRuntimeCaches()
	persistCounters := newRunPerfCounters(true)
	key, entry, err := runtimeFromIndexWithSourceDigestsAndPerf(index, artifacts.SourceDigests, newSourceCache(), false, persistCounters)
	if err != nil {
		b.Fatal(err)
	}
	persistDiskRuntimeWithPerf(index, artifacts.SourceDigests, key, entry.restored.CloneOrg(), entry, persistCounters)
	persistPhases := snapshotPerfCounters(persistCounters).Phases
	if persistPhases.CacheMisses != 1 || persistPhases.DiskCacheHits != 0 || persistPhases.MemoryCacheHits != 0 || persistPhases.CacheEncodeNS <= 0 {
		b.Fatalf("persist phases = %#v", persistPhases)
	}

	InvalidateRuntimeCaches()
	oracleRun := RunCasesContext(context.Background(), index, Options{
		ParallelMethods: true,
		Parallelism:     1,
		NoDiskCache:     true,
		SourceDigests:   artifacts.SourceDigests,
	}, cases)
	want := canonicalDiskRuntimeIsolationResult(oracleRun)
	InvalidateRuntimeCaches()

	type benchmarkMode struct {
		name         string
		prime        bool
		opts         Options
		wantPhases   RunnerPhasePerfCounters
		wantDiskRead bool
	}
	modes := []benchmarkMode{
		{
			name:       "built-no-disk",
			opts:       Options{ParallelMethods: true, Parallelism: 4, NoDiskCache: true, PerfCounters: true, SourceDigests: artifacts.SourceDigests},
			wantPhases: RunnerPhasePerfCounters{CacheMisses: 1},
		},
		{
			name:       "memory-warm",
			prime:      true,
			opts:       Options{ParallelMethods: true, Parallelism: 4, NoDiskCache: true, PerfCounters: true, SourceDigests: artifacts.SourceDigests},
			wantPhases: RunnerPhasePerfCounters{MemoryCacheHits: 1},
		},
		{
			name:         "restored",
			opts:         Options{ParallelMethods: true, Parallelism: 4, RestoredRuntimeMultiWorker: true, PerfCounters: true, SourceDigests: artifacts.SourceDigests},
			wantPhases:   RunnerPhasePerfCounters{DiskCacheHits: 1},
			wantDiskRead: true,
		},
	}
	for _, mode := range modes {
		mode := mode
		b.Run(mode.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				InvalidateRuntimeCaches()
				if mode.prime {
					prime := RunCasesContext(context.Background(), index, Options{
						ParallelMethods: true,
						Parallelism:     4,
						NoDiskCache:     true,
						SourceDigests:   artifacts.SourceDigests,
					}, cases)
					if got := canonicalDiskRuntimeIsolationResult(prime); !reflect.DeepEqual(got, want) {
						b.Fatalf("memory prime differs from oracle:\n got: %#v\nwant: %#v", got, want)
					}
				}
				ResetPerfCounters()
				b.StartTimer()
				run := RunCasesContext(context.Background(), index, mode.opts, cases)
				b.StopTimer()
				if got := canonicalDiskRuntimeIsolationResult(run); !reflect.DeepEqual(got, want) {
					b.Fatalf("%s result differs from oracle:\n got: %#v\nwant: %#v", mode.name, got, want)
				}
				phases := SnapshotPerfCounters().Phases
				if phases.CacheMisses != mode.wantPhases.CacheMisses ||
					phases.MemoryCacheHits != mode.wantPhases.MemoryCacheHits ||
					phases.DiskCacheHits != mode.wantPhases.DiskCacheHits {
					b.Fatalf("%s phases = %#v, want hits/misses %#v", mode.name, phases, mode.wantPhases)
				}
				if mode.wantDiskRead {
					if phases.CacheValidateNS <= 0 || phases.CacheDecodeNS <= 0 {
						b.Fatalf("%s phases = %#v, want validated and decoded disk artifact", mode.name, phases)
					}
				} else if phases.CacheValidateNS != 0 || phases.CacheDecodeNS != 0 {
					b.Fatalf("%s phases = %#v, want no disk access", mode.name, phases)
				}
			}
		})
	}
}

func BenchmarkRestoredRuntimeStructuralValidation(b *testing.B) {
	for _, owners := range []int{1, 4096} {
		entry := runtimeStructuralAllocationFixture(owners, true)
		b.Run(fmt.Sprintf("walk/owners-%d", owners), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if !validateRuntimeCacheEntryStructure(entry) {
					b.Fatal("valid restored runtime rejected")
				}
			}
		})
		b.Run(fmt.Sprintf("clone-oracle/owners-%d", owners), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, ok := cloneRuntimeCacheEntryChecked(entry); !ok {
					b.Fatal("valid restored runtime rejected")
				}
			}
		})
	}
}

func BenchmarkRunTestSuite(b *testing.B) {
	for _, tests := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("methods=%d/setup=false", tests), func(b *testing.B) {
			benchmarkRunTestSuite(b, tests, false)
		})
		b.Run(fmt.Sprintf("methods=%d/setup=true", tests), func(b *testing.B) {
			benchmarkRunTestSuite(b, tests, true)
		})
	}
}

func BenchmarkRunTestSuiteWithClassSetup(b *testing.B) {
	benchmarkRunTestSuite(b, 100, true)
}

func benchmarkRunTestSuite(b *testing.B, tests int, withSetup bool) {
	root := b.TempDir()
	writeBenchmarkApexTestProject(b, root, tests, withSetup)
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := Run(index, Options{})
		summary := run.Summary()
		if summary.Total != tests || summary.Passed != tests {
			b.Fatalf("summary = %#v", summary)
		}
	}
}

func writeBenchmarkApexTestProject(b *testing.B, root string, tests int, withSetup bool) {
	b.Helper()
	writeBenchmarkTestFile(b, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeBenchmarkTestFile(b, filepath.Join(root, "force-app/main/classes/MathUtil.cls"), `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
`)
	for i := 0; i < tests; i++ {
		setupBlock := ""
		if withSetup {
			setupBlock = `
  @TestSetup
  static void buildFixture() {
    insert new Account(Name = 'Fixture');
  }
`
		}
		writeBenchmarkTestFile(b, filepath.Join(root, fmt.Sprintf("force-app/main/classes/MathUtil%03dTest.cls", i)), fmt.Sprintf(`
@isTest
private class MathUtil%03dTest {
%s
  @isTest static void adds() {
    System.assertEquals(%d, MathUtil.add(%d, %d));
  }
}
`, i, setupBlock, i+i+1, i, i+1))
	}
}

func benchmarkLoadTestIndex(b *testing.B, root string) typesys.Index {
	b.Helper()
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	return typesys.Build(p, s)
}

func writeBenchmarkTestFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}
