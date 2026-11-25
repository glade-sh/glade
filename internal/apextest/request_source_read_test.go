package apextest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/filestamp"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// requestReadFixture is a deterministic small project driven through the
// daemon-shaped request path: RunCasesContext with an attached build
// artifact, the exact options testdaemon.optionsForGeneration builds.
type requestReadFixture struct {
	root     string
	sources  []string
	sidecars []string
	contents map[string]string
	index    typesys.Index
	options  Options
	cases    []TestCase
}

func writeRequestReadFixture(t *testing.T, serviceCount int) *requestReadFixture {
	t.Helper()
	root := t.TempDir()
	contents := map[string]string{}
	write := func(rel, body string) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		contents[path] = body
		return path
	}
	write("sfdx-project.json", `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	var sources, sidecars []string
	for i := range serviceCount {
		name := fmt.Sprintf("RequestService%02d", i)
		sources = append(sources, write(filepath.Join("force-app/main/default/classes", name+".cls"),
			fmt.Sprintf("public class %s { public static Integer value() { return %d; } }", name, i)))
		sidecar := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
		sidecars = append(sidecars, write(filepath.Join("force-app/main/default/classes", name+".cls-meta.xml"), sidecar))
	}
	test := `@IsTest private class RequestReadsTest { @IsTest static void passes() { System.assertEquals(0, RequestService00.value()); } }`
	sources = append(sources, write("force-app/main/default/classes/RequestReadsTest.cls", test))
	sidecars = append(sidecars, write("force-app/main/default/classes/RequestReadsTest.cls-meta.xml",
		`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	for _, diag := range index.Diagnostics {
		if diag.Severity == diagnostic.Error {
			t.Fatalf("fixture diagnostics: %#v", index.Diagnostics)
		}
	}
	options := Options{
		NoDiskCache:     true,
		Parallelism:     1,
		BuildArtifacts:  &artifacts,
		SourceDigests:   artifacts.SourceDigests,
		ProjectSnapshot: &p,
	}
	return &requestReadFixture{
		root:     root,
		sources:  sources,
		sidecars: sidecars,
		contents: contents,
		index:    index,
		options:  options,
		cases:    Discover(index, Options{}),
	}
}

// rewriteIdentical rewrites every fixture file with its current bytes. Every
// stamp changes, so each file is racily clean for the next request while the
// captured generation still matches the content.
func (f *requestReadFixture) rewriteIdentical(t *testing.T) {
	t.Helper()
	for path, body := range f.contents {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// countReads observes validation reads of every fixture path.
func (f *requestReadFixture) countReads(t *testing.T) (reads map[string]int, stop func()) {
	t.Helper()
	var mu sync.Mutex
	reads = make(map[string]int, len(f.sources)+len(f.sidecars))
	stop = SetFileDigestReadHookForTesting(func(path string) {
		mu.Lock()
		reads[path]++
		mu.Unlock()
	})
	return reads, stop
}

func (f *requestReadFixture) assertReadOnce(t *testing.T, reads map[string]int) {
	t.Helper()
	var unexpected []string
	for _, group := range []struct {
		kind  string
		paths []string
	}{
		{"source", f.sources},
		{"companion metadata", f.sidecars},
	} {
		for _, path := range group.paths {
			if got := reads[path]; got != 1 {
				unexpected = append(unexpected, fmt.Sprintf("%s %s: %d reads", group.kind, filepath.Base(path), got))
			}
		}
	}
	if len(unexpected) != 0 {
		t.Fatalf("want exactly one digest computation per file:\n%s", strings.Join(unexpected, "\n"))
	}
}

// TestWarmRequestReadsAndHashesEachSourceOnce proves one daemon-shaped request
// reads every captured source and companion metadata file exactly once, at
// any validation boundary or at the seal. Files are rewritten with identical
// bytes so no boundary may shortcut on a trusted old stamp.
func TestWarmRequestReadsAndHashesEachSourceOnce(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	prime := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	primeSummary := prime.Summary()
	if primeSummary.Total != 1 || primeSummary.Passed != 1 || !primeSummary.OK() {
		t.Fatalf("prime run summary = %#v; suites = %#v", primeSummary, prime.Suites)
	}
	for range 2 {
		f.rewriteIdentical(t)
		reads, stop := f.countReads(t)
		run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
		stop()
		f.assertReadOnce(t, reads)
		if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
			t.Fatalf("warm run summary = %#v, want the prime run's %#v", summary, primeSummary)
		}
	}
}

func TestWarmRequestRevalidatesRacySourcesAcrossRequests(t *testing.T) {
	skipWithoutStamps(t)
	resetFileDigestTrust()
	t.Cleanup(resetFileDigestTrust)
	t.Cleanup(InvalidateRuntimeCaches)
	f := writeRequestReadFixture(t, 4)
	target := f.sources[0]
	stamp, ok := filestamp.Stat(target)
	if !ok {
		t.Fatal("missing source stamp")
	}
	t.Cleanup(setFileDigestClockForTesting(func() time.Time {
		return time.Unix(0, max(stamp.Mtime, stamp.Ctime))
	}))
	// Model a coarse filesystem reporting the same lookup stamp on both
	// sides of the next save.
	stat := func(path string) (filestamp.Stamp, bool) {
		if path == target {
			return stamp, true
		}
		return filestamp.Stat(path)
	}
	previous := requestDigestStatHook.Swap(&stat)
	t.Cleanup(func() { requestDigestStatHook.Store(previous) })

	reads, stop := f.countReads(t)
	prime := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	stop()
	if summary := prime.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
		t.Fatalf("first request failed: %#v", summary)
	}
	f.assertReadOnce(t, reads)

	edited := strings.Replace(f.contents[target], "return 0;", "return 7;", 1)
	if err := os.WriteFile(target, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	reads, stop = f.countReads(t)
	run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	stop()
	if reads[target] != 1 || run.Summary().CompileErrors == 0 ||
		!strings.Contains(testreportRunDetails(run), "source snapshot mismatch for "+filepath.ToSlash(target)) {
		t.Fatalf("second request read the source %d times: %s", reads[target], testreportRunDetails(run))
	}
}

// TestWarmRequestStillDetectsEditBetweenValidations keeps the request's
// mismatch guarantee under within-request digest reuse: a save that lands
// between two validation boundaries must still fail the request, exactly as
// it does when every boundary re-reads the file.
func TestWarmRequestStillDetectsEditBetweenValidations(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	if prime := RunCasesContext(context.Background(), f.index, f.options, f.cases); !prime.Summary().OK() {
		t.Fatalf("prime run summary = %#v", prime.Summary())
	}
	f.rewriteIdentical(t)
	mutated := false
	target := f.sources[0]
	stop := setSnapshotValidationHookForTesting(func(stage string) {
		if stage == "runtime_after_initial_validation" && !mutated {
			mutated = true
			if err := os.WriteFile(target, []byte("public class RequestService00 { public Integer value() { return 7; } }"), 0o644); err != nil {
				t.Error(err)
			}
		}
	})
	defer stop()
	run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	stop()
	if !mutated {
		t.Fatal("request never reached runtime_after_initial_validation")
	}
	if summary := run.Summary(); summary.CompileErrors == 0 {
		t.Fatalf("run accepted a source saved between validation boundaries: %#v", summary)
	}
	found := false
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem == nil {
				continue
			}
			if strings.Contains(testCase.Problem.Message, "source snapshot mismatch for") && strings.Contains(testCase.Problem.Message, filepath.ToSlash(target)) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no case reported the mismatch for %s; run = %#v", target, testreportRunDetails(run))
	}
}

func TestWarmRequestRejectsSameStampEditBetweenValidations(t *testing.T) {
	skipWithoutStamps(t)
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	target := f.sources[0]
	stamp, ok := filestamp.Stat(target)
	if !ok {
		t.Fatal("missing source stamp")
	}
	t.Cleanup(setFileDigestClockForTesting(func() time.Time {
		return time.Unix(0, max(stamp.Mtime, stamp.Ctime))
	}))
	stat := func(path string) (filestamp.Stamp, bool) {
		if path == target {
			return stamp, true
		}
		return filestamp.Stat(path)
	}
	previous := requestDigestStatHook.Swap(&stat)
	t.Cleanup(func() { requestDigestStatHook.Store(previous) })
	if prime := RunCasesContext(context.Background(), f.index, f.options, f.cases); prime.Summary().Passed != 1 {
		t.Fatalf("prime run: %s", testreportRunDetails(prime))
	}

	mutated := false
	t.Cleanup(setSnapshotValidationHookForTesting(func(stage string) {
		if stage == "runtime_after_initial_validation" && !mutated {
			mutated = true
			edited := strings.Replace(f.contents[target], "return 0;", "return 7;", 1)
			if err := os.WriteFile(target, []byte(edited), 0o644); err != nil {
				t.Error(err)
			}
		}
	}))
	run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	if !mutated {
		t.Fatal("request never reached runtime_after_initial_validation")
	}
	if run.Summary().CompileErrors == 0 ||
		!strings.Contains(testreportRunDetails(run), "source snapshot mismatch for "+filepath.ToSlash(target)) {
		t.Fatalf("request accepted a same-stamp edit: %s", testreportRunDetails(run))
	}
}

// holdRequestStamp pins the file digest clock to path's stamp, so the file
// stays recent, and makes request validation see that one stamp on both sides
// of any later write: a same-size write inside one coarse timestamp tick.
func holdRequestStamp(t *testing.T, path string) {
	t.Helper()
	stamp, ok := filestamp.Stat(path)
	if !ok {
		t.Fatal("missing source stamp")
	}
	t.Cleanup(setFileDigestClockForTesting(func() time.Time {
		return time.Unix(0, max(stamp.Mtime, stamp.Ctime))
	}))
	stat := func(p string) (filestamp.Stamp, bool) {
		if p == path {
			return stamp, true
		}
		return filestamp.Stat(p)
	}
	previous := requestDigestStatHook.Swap(&stat)
	t.Cleanup(func() { requestDigestStatHook.Store(previous) })
}

// validationStages runs once and returns the distinct validation stages the
// run announced, in order.
func validationStages(t *testing.T, run func()) []string {
	t.Helper()
	var mu sync.Mutex
	var stages []string
	seen := map[string]bool{}
	stop := setSnapshotValidationHookForTesting(func(stage string) {
		mu.Lock()
		if !seen[stage] {
			seen[stage] = true
			stages = append(stages, stage)
		}
		mu.Unlock()
	})
	run()
	stop()
	return stages
}

// editAtStage rewrites path with content the first time a run announces
// stage, and reports whether it did.
func editAtStage(t *testing.T, stage, path, content string) (edited func() bool) {
	t.Helper()
	var once sync.Once
	var done atomic.Bool
	t.Cleanup(setSnapshotValidationHookForTesting(func(current string) {
		if current != stage {
			return
		}
		once.Do(func() {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Error(err)
			}
			done.Store(true)
		})
	}))
	return done.Load
}

// TestRequestRejectsSameStampEditAtEveryValidationStage saves a same-size,
// same-stamp edit at each validation stage a request announces, in turn. The
// request must fail on every one, and must still read the file only once.
func TestRequestRejectsSameStampEditAtEveryValidationStage(t *testing.T) {
	skipWithoutStamps(t)
	for _, disk := range []bool{false, true} {
		name := "memory"
		if disk {
			name = "disk"
		}
		t.Run(name, func(t *testing.T) {
			if disk {
				t.Cleanup(EnableDiskCacheForTesting())
			}
			t.Cleanup(InvalidateRuntimeCaches)
			t.Cleanup(resetFileDigestTrust)
			resetFileDigestTrust()
			f := writeRequestReadFixture(t, 4)
			f.options.NoDiskCache = !disk
			target := f.sources[0]
			original := f.contents[target]
			edited := strings.Replace(original, "return 0;", "return 7;", 1)
			holdRequestStamp(t, target)
			request := func() testreport.Run {
				return RunCasesContext(context.Background(), f.index, f.options, f.cases)
			}
			// Cold, warm in memory, and (with the disk cache) warm on disk only.
			shapes := []func(){func() {
				InvalidateRuntimeCaches()
				if disk {
					if err := startupcache.Clear(f.root, startupcache.SubdirTest); err != nil {
						t.Fatal(err)
					}
				}
			}, func() {}}
			if disk {
				shapes = append(shapes, func() {
					if prime := request(); prime.Summary().Passed != 1 {
						t.Fatalf("disk prime: %s", testreportRunDetails(prime))
					}
					InvalidateRuntimeCaches()
				})
			}
			probed := map[string]bool{}
			for _, prepare := range shapes {
				prepare()
				var stages []string
				stages = validationStages(t, func() {
					if prime := request(); prime.Summary().Passed != 1 {
						t.Fatalf("prime run: %s", testreportRunDetails(prime))
					}
				})
				for _, stage := range stages {
					prepare()
					didEdit := editAtStage(t, stage, target, edited)
					reads, stop := f.countReads(t)
					run := request()
					stop()
					if !didEdit() {
						// The warm request before it took another path.
						if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
							t.Fatal(err)
						}
						continue
					}
					if run.Summary().CompileErrors == 0 ||
						!strings.Contains(testreportRunDetails(run), "source snapshot mismatch for "+filepath.ToSlash(target)) {
						t.Fatalf("request accepted a same-stamp edit at %s: %s", stage, testreportRunDetails(run))
					}
					if reads[target] != 1 {
						t.Fatalf("request read the edited source %d times at %s, want 1", reads[target], stage)
					}
					probed[stage] = true
					if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
						t.Fatal(err)
					}
					if restored := request(); restored.Summary().Passed != 1 {
						t.Fatalf("restored source after %s: %s", stage, testreportRunDetails(restored))
					}
				}
			}
			want := []string{"semantic_after_initial_validation", "semantic_before_cache_return", "runtime_after_initial_validation",
				"runtime_before_memory_cache_publication", "runtime_before_memory_cache_return",
				"setup_before_cache_publication", "setup_before_cache_return", "test_before_cache_publication", "test_before_cache_return"}
			if disk {
				want = append(want, "runtime_before_disk_cache_publication", "runtime_before_disk_cache_return")
			}
			for _, stage := range want {
				if !probed[stage] {
					t.Errorf("no request was edited and rejected at validation stage %s", stage)
				}
			}
		})
	}
}

// TestWarmRuntimeRejectsSameStampEditBetweenValidations is the warm-up form:
// the warm-up must not publish a generation whose source changed under an
// unchanged stamp after its first validation.
func TestWarmRuntimeRejectsSameStampEditBetweenValidations(t *testing.T) {
	skipWithoutStamps(t)
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	target := f.sources[0]
	holdRequestStamp(t, target)
	didEdit := editAtStage(t, "runtime_after_initial_validation", target, strings.Replace(f.contents[target], "return 0;", "return 7;", 1))
	reads, stop := f.countReads(t)
	err := WarmRuntimeWithBuildArtifacts(context.Background(), f.index, f.options.BuildArtifacts)
	stop()
	if !didEdit() {
		t.Fatal("warm-up never reached runtime_after_initial_validation")
	}
	var mismatch *SourceSnapshotMismatchError
	if !errors.As(err, &mismatch) || mismatch.File != target {
		t.Fatalf("warm-up accepted a same-stamp edit: %v", err)
	}
	if reads[target] != 1 {
		t.Fatalf("warm-up read the edited source %d times, want 1", reads[target])
	}
	if stats := semanticResults.Stats(); stats.Entries != 0 {
		t.Fatalf("rejected warm-up kept its semantic result: %#v", stats)
	}
}

func TestWarmRuntimeWithBuildArtifactsHashesEachRecentInputOnce(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	f.rewriteIdentical(t)
	reads, stop := f.countReads(t)
	err := WarmRuntimeWithBuildArtifacts(context.Background(), f.index, f.options.BuildArtifacts)
	stop()
	if err != nil {
		t.Fatal(err)
	}
	f.assertReadOnce(t, reads)
}

func TestDiskRuntimeHitHashesEachRecentInputOnce(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(resetFileDigestTrust)
	t.Cleanup(ResetPerfCounters)
	resetFileDigestTrust()
	f := writeRequestReadFixture(t, 4)
	f.options.NoDiskCache = false
	f.options.PerfCounters = true
	if prime := RunCasesContext(context.Background(), f.index, f.options, f.cases); prime.Summary().Passed != 1 {
		t.Fatalf("prime run: %s", testreportRunDetails(prime))
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	f.rewriteIdentical(t)
	reads, stop := f.countReads(t)
	run := RunCasesContext(context.Background(), f.index, f.options, f.cases)
	stop()
	if run.Summary().Passed != 1 {
		t.Fatalf("disk-backed run: %s", testreportRunDetails(run))
	}
	if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 1 || phases.CacheMisses != 0 {
		t.Fatalf("expected a disk hit, got %#v", phases)
	}
	f.assertReadOnce(t, reads)
}

func testreportRunDetails(run testreport.Run) string {
	var out strings.Builder
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem == nil {
				fmt.Fprintf(&out, "%s.%s %s\n", suite.Name, testCase.MethodName, testCase.Status)
				continue
			}
			fmt.Fprintf(&out, "%s.%s %s: %s\n", suite.Name, testCase.MethodName, testCase.Status, testCase.Problem.Message)
		}
	}
	return out.String()
}
