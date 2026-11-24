package apextest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

func besideGateProject(t *testing.T, body string) (typesys.Index, typesys.BuildArtifacts, []TestCase) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "BesideGateTest.cls")
	writeFile(t, path, "@IsTest\nprivate class BesideGateTest {\n  @IsTest static void runs() {\n    "+body+"\n  }\n}\n")
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
	if index.HasErrors() {
		t.Fatalf("index diagnostics = %#v", index.Diagnostics)
	}
	cases := Discover(index, Options{})
	if len(cases) != 1 {
		t.Fatalf("cases = %#v", cases)
	}
	return index, artifacts, cases
}

func assertNoBesideGateTrace(t *testing.T, key runtimeCacheKey) {
	t.Helper()
	// WaitForRuntimeCacheWork returns only once the abandoned build has.
	WaitForRuntimeCacheWork()
	assertNoCompiledGenerationCaches(t, key)
}

// TestRuntimeBesideGateLeavesNoTraceWhenGateRejects pins the build started
// beside the semantic gate: when the gate reports a compile error, the build
// stops without storing a runtime or compiled tests.
func TestRuntimeBesideGateLeavesNoTraceWhenGateRejects(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	index, artifacts, cases := besideGateProject(t, "Object value = new MissingBesideGateType();")
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true, BuildArtifacts: &artifacts}, cases)
	if summary := run.Summary(); summary.CompileErrors != 1 {
		t.Fatalf("summary = %#v, problem %q", summary, firstRunProblem(run))
	}
	assertNoBesideGateTrace(t, buildArtifactRuntimeKey(t, index, &artifacts))
}

// TestRuntimeBesideGateLeavesNoTraceWhenGatePanics covers a panic between
// starting the build and handing it the gate's result.
func TestRuntimeBesideGateLeavesNoTraceWhenGatePanics(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	index, artifacts, cases := besideGateProject(t, "System.assertEquals(1, 1);")
	func() {
		defer func() {
			if recovered := recover(); recovered == nil || !strings.Contains(strings.ToLower(toString(recovered)), "gate panic") {
				t.Fatalf("recovered %v, want the gate panic", recovered)
			}
		}()
		runCasesContextWithSemanticGateHooks(context.Background(), index, Options{NoDiskCache: true, BuildArtifacts: &artifacts}, cases, semanticGateHooks{afterIdentity: func() { panic("gate panic") }})
	}()
	assertNoBesideGateTrace(t, buildArtifactRuntimeKey(t, index, &artifacts))
	// The same index still runs once the gate behaves.
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true, BuildArtifacts: &artifacts}, cases)
	if summary := run.Summary(); summary.Passed != 1 {
		t.Fatalf("summary after the panic = %#v, problem %q", summary, firstRunProblem(run))
	}
}

func toString(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// TestBackgroundDiskPublicationBesideConcurrentRuns runs a background
// startup-cache write while other runs read the same memory runtime; run it
// under -race. The write still lands.
func TestBackgroundDiskPublicationBesideConcurrentRuns(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	wasDisabled := disableDiskCache.Load()
	disableDiskCache.Store(false)
	t.Cleanup(func() { disableDiskCache.Store(wasDisabled) })
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	writeFile(t, filepath.Join(classes, "BackgroundWriteTest.cls"), "@IsTest\nprivate class BackgroundWriteTest {\n  @IsTest static void runs() { System.assertEquals(1, 1); }\n}\n")
	writeFile(t, filepath.Join(classes, "BackgroundWriteTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	cases := Discover(index, Options{})
	opts := Options{Parallelism: 1, BuildArtifacts: &artifacts, BackgroundDiskPublication: true}
	if summary := RunCasesContext(context.Background(), index, opts, cases).Summary(); summary.Passed != 1 {
		t.Fatalf("first run summary = %#v", summary)
	}
	done := make(chan testreport.Run, 3)
	for i := 0; i < 3; i++ {
		go func() { done <- RunCasesContext(context.Background(), index, opts, cases) }()
	}
	for i := 0; i < 3; i++ {
		if summary := (<-done).Summary(); summary.Passed != 1 {
			t.Fatalf("concurrent run summary = %#v", summary)
		}
	}
	WaitForRuntimeCacheWork()
	matches, _ := filepath.Glob(filepath.Join(index.Project.Root, ".glade", "test", "startup.payload.*"))
	if len(matches) == 0 {
		t.Fatal("the background write left no startup payload")
	}
}

// TestRuntimeBesideGateRunWaitsForAbandonedBuild pins the build's lifetime:
// a run whose gate rejects the index does not return while the build it
// started is still running. An abandoned build that outlived its run kept
// compiling the rejected index beside later runs, cache invalidations and
// test cleanups, and any panic it raised was dropped.
func TestRuntimeBesideGateRunWaitsForAbandonedBuild(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	index, artifacts, cases := besideGateProject(t, "Object value = new MissingBesideGateType();")
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	testOnlyRuntimeBesideGateStartHook = func() {
		once.Do(func() { close(started) })
		<-release
	}
	t.Cleanup(func() { testOnlyRuntimeBesideGateStartHook = nil })
	runDone := make(chan testreport.Run, 1)
	go func() {
		runDone <- RunCasesContext(context.Background(), index, Options{NoDiskCache: true, BuildArtifacts: &artifacts}, cases)
	}()
	<-started
	select {
	case <-runDone:
		close(release)
		t.Fatal("the run returned while its abandoned runtime build was still running")
	case <-time.After(time.Second):
	}
	close(release)
	run := <-runDone
	if summary := run.Summary(); summary.CompileErrors != 1 {
		t.Fatalf("summary = %#v, problem %q", summary, firstRunProblem(run))
	}
	assertNoCompiledGenerationCaches(t, buildArtifactRuntimeKey(t, index, &artifacts))
}

// besideGateOutcomeDigest hashes a run's report without its durations.
func besideGateOutcomeDigest(t *testing.T, run testreport.Run) string {
	t.Helper()
	run.DurationMS = 0
	for suiteIndex := range run.Suites {
		run.Suites[suiteIndex].DurationMS = 0
		for caseIndex := range run.Suites[suiteIndex].Cases {
			run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
		}
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type besideGateLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *besideGateLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *besideGateLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestRuntimeBesideGateBuildPanicIsFailSafe forces a nil dereference inside
// the build beside the gate, for an accepted and a rejected gate. The run
// never panics: an accepted run falls back to the sequential build and a
// rejected run reports its compile error, each with the same outcome as a
// run without the build beside the gate, and the panic is logged once with
// its value, build id and original stack.
func TestRuntimeBesideGateBuildPanicIsFailSafe(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		state string
	}{
		{name: "accepted", body: "System.assertEquals(1, 1);", state: "(accepted; the run falls back to the sequential build)"},
		{name: "rejected", body: "Object value = new MissingBesideGateType();", state: "(abandoned)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(InvalidateRuntimeCaches)
			index, artifacts, cases := besideGateProject(t, tc.body)
			opts := Options{NoDiskCache: true, BuildArtifacts: &artifacts}

			InvalidateRuntimeCaches()
			previous := runtime.GOMAXPROCS(1)
			sequential := RunCasesContext(context.Background(), index, opts, cases)
			runtime.GOMAXPROCS(4)
			defer runtime.GOMAXPROCS(previous)

			log := &besideGateLogBuffer{}
			testOnlyRuntimeBesideGateLog = log
			testOnlyRuntimeBesideGateStartHook = func() {
				var missing *struct{ value int }
				_ = missing.value
			}
			t.Cleanup(func() {
				testOnlyRuntimeBesideGateStartHook = nil
				testOnlyRuntimeBesideGateLog = nil
			})
			InvalidateRuntimeCaches()
			var besideGate testreport.Run
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("the run panicked: %v", recovered)
					}
				}()
				besideGate = RunCasesContext(context.Background(), index, opts, cases)
			}()
			if got, want := besideGateOutcomeDigest(t, besideGate), besideGateOutcomeDigest(t, sequential); got != want {
				t.Fatalf("outcome digest %s, want the sequential run's %s\n beside gate: %#v\n sequential: %#v", got, want, besideGate.Summary(), sequential.Summary())
			}
			if tc.name == "accepted" && sequential.Summary().Passed != 1 {
				t.Fatalf("sequential summary = %#v", sequential.Summary())
			}
			if tc.name == "rejected" && sequential.Summary().CompileErrors != 1 {
				t.Fatalf("sequential summary = %#v", sequential.Summary())
			}
			line := log.String()
			if strings.Count(line, "\n") != 1 || !strings.HasPrefix(line, "glade: runtime build ") || !strings.Contains(line, tc.state) ||
				!strings.Contains(line, "nil pointer dereference") || !strings.Contains(line, "TestRuntimeBesideGateBuildPanicIsFailSafe") {
				t.Fatalf("panic log = %q", line)
			}
		})
	}
}

// runtimeCacheMethodIdentities returns the identity of each compiled method
// table in the in-process runtime cache.
func runtimeCacheMethodIdentities() map[string]bool {
	runtimeCacheMu.RLock()
	defer runtimeCacheMu.RUnlock()
	out := make(map[string]bool, len(runtimeCache))
	for _, entry := range runtimeCache {
		out[fmt.Sprintf("%p", entry.Methods)] = true
	}
	return out
}

// TestRuntimeBesideGatePartialBuildPanicDiscardsItsWrites panics inside a
// partly completed accepted build: after method compilation has filled the
// request source cache, after org construction, and after the build has
// published its runtime to the memory and startup caches. The run's outcome
// equals a run without the build beside the gate, and nothing the failed
// build published survives: the cached runtime and the startup payload are
// the sequential build's.
func TestRuntimeBesideGatePartialBuildPanicDiscardsItsWrites(t *testing.T) {
	restoreDisk := EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	writeFile(t, filepath.Join(classes, "PartialPanicHelper.cls"), "public class PartialPanicHelper { public static Integer value() { return 3; } }")
	writeFile(t, filepath.Join(classes, "PartialPanicTest.cls"), "@IsTest private class PartialPanicTest { @IsTest static void runs() { System.assertEquals(3, PartialPanicHelper.value()); } }")
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(loaded, gladeschema.Schema{})
	cases := Discover(index, Options{})
	opts := Options{BuildArtifacts: &artifacts}
	cold := func() {
		WaitForRuntimeCacheWork()
		InvalidateRuntimeCaches()
		if err := startupcache.Clear(root, startupcache.SubdirTest); err != nil {
			t.Fatal(err)
		}
	}
	payload := func() os.FileInfo {
		matches, _ := filepath.Glob(filepath.Join(root, ".glade", "test", "startup.payload.*"))
		if len(matches) != 1 {
			return nil
		}
		info, err := os.Stat(matches[0])
		if err != nil {
			return nil
		}
		return info
	}

	cold()
	previous := runtime.GOMAXPROCS(1)
	sequential := RunCasesContext(context.Background(), index, opts, cases)
	runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	if summary := sequential.Summary(); summary.Passed != 1 {
		t.Fatalf("sequential summary = %#v problem %q", summary, firstRunProblem(sequential))
	}
	want := besideGateOutcomeDigest(t, sequential)

	for _, at := range []string{"after_methods", "after_org", "after_publish"} {
		t.Run(at, func(t *testing.T) {
			cold()
			log := &besideGateLogBuffer{}
			var failedMethods map[string]bool
			var failedPayload os.FileInfo
			testOnlyRuntimeBesideGateLog = log
			testOnlyRuntimeBesideGateStageHook = func(stage string) {
				if stage != at {
					return
				}
				failedMethods = runtimeCacheMethodIdentities()
				failedPayload = payload()
				panic("partial build panic at " + stage)
			}
			t.Cleanup(func() {
				testOnlyRuntimeBesideGateStageHook = nil
				testOnlyRuntimeBesideGateLog = nil
			})
			var run testreport.Run
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("the run panicked: %v", recovered)
					}
				}()
				run = RunCasesContext(context.Background(), index, opts, cases)
			}()
			if got := besideGateOutcomeDigest(t, run); got != want {
				t.Fatalf("outcome digest %s, want the sequential run's %s: %#v problem %q", got, want, run.Summary(), firstRunProblem(run))
			}
			line := log.String()
			if strings.Count(line, "\n") != 1 || !strings.Contains(line, "(accepted;") || !strings.Contains(line, "type=string") ||
				strings.Contains(line, "partial build panic") || !strings.Contains(line, "TestRuntimeBesideGatePartialBuildPanicDiscardsItsWrites") {
				t.Fatalf("panic log = %q", line)
			}
			if at != "after_publish" {
				return
			}
			if len(failedMethods) == 0 || failedPayload == nil {
				t.Fatalf("the failed build published nothing to discard: methods=%v payload=%v", failedMethods, failedPayload)
			}
			for identity := range runtimeCacheMethodIdentities() {
				if failedMethods[identity] {
					t.Fatalf("the failed build's runtime %s survived in the memory cache", identity)
				}
			}
			WaitForRuntimeCacheWork()
			if current := payload(); current == nil || os.SameFile(current, failedPayload) {
				t.Fatalf("startup payload = %v, want the sequential build's, not the failed build's", current)
			}
		})
	}
}

// TestRuntimeBuildPanicLogIsBounded pins the log line's bounds: one line, a
// non-runtime-error value never printed, and the stack truncated.
func TestRuntimeBuildPanicLogIsBounded(t *testing.T) {
	var out bytes.Buffer
	stack := []byte(strings.Repeat("frame\n", runtimeBuildPanicStackLimit))
	logRuntimeBuildPanic(&out, &runtimeBuildPanic{build: 7, value: "class Secret {\n}", stack: stack}, true)
	line := out.String()
	if strings.Count(line, "\n") != 1 || strings.Contains(line, "Secret") || !strings.Contains(line, "(abandoned)") ||
		!strings.Contains(line, "...(truncated)") || len(line) > runtimeBuildPanicStackLimit*2+1024 {
		t.Fatalf("log line (%d bytes) = %.300q", len(line), line)
	}
}
