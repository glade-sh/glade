package apextest

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
