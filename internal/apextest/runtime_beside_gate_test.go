package apextest

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
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
	runtimeBesideGateStartHook = func() {
		once.Do(func() { close(started) })
		<-release
	}
	t.Cleanup(func() { runtimeBesideGateStartHook = nil })
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

// TestRuntimeBesideGateFinishKeepsBuildPanicStack pins the re-raise of a
// build panic: finish raises the original value with the stack captured on
// the build goroutine, never a bare value without its origin.
func TestRuntimeBesideGateFinishKeepsBuildPanicStack(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	previous := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previous)
	index, artifacts, cases := besideGateProject(t, "System.assertEquals(1, 1);")
	runtimeBesideGateStartHook = func() {
		var missing *struct{ value int }
		_ = missing.value
	}
	t.Cleanup(func() { runtimeBesideGateStartHook = nil })
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		RunCasesContext(context.Background(), index, Options{NoDiskCache: true, BuildArtifacts: &artifacts}, cases)
	}()
	buildPanic, ok := recovered.(*runtimeBuildPanic)
	if !ok {
		t.Fatalf("recovered %T %v, want *runtimeBuildPanic", recovered, recovered)
	}
	var runtimeErr runtime.Error
	if !errors.As(buildPanic, &runtimeErr) || !strings.Contains(runtimeErr.Error(), "nil pointer dereference") {
		t.Fatalf("panic value = %v, want the original nil dereference", buildPanic.value)
	}
	if !strings.Contains(string(buildPanic.stack), "TestRuntimeBesideGateFinishKeepsBuildPanicStack") {
		t.Fatalf("stack does not show the build goroutine's origin:\n%s", buildPanic.stack)
	}
	if !strings.Contains(buildPanic.Error(), "original stack:") {
		t.Fatalf("Error() = %q", buildPanic.Error())
	}
}
