package apextest

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestCanceledRuntimeBuildMatchesSequentialReport(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)
	t.Cleanup(InvalidateRuntimeCaches)
	index, _ := digestProject(t, 3, false)
	cases := Discover(index, Options{})
	for _, selected := range [][]TestCase{cases, {}} {
		var want string
		var wantEvents []TestProgress
		for _, setting := range []string{"0", "1"} {
			t.Setenv(runtimeBesideGateEnv, setting)
			InvalidateRuntimeCaches()
			ctx, cancel := context.WithCancel(context.Background())
			var events []TestProgress
			opts := Options{NoDiskCache: true, CodeCoverage: true, Progress: func(p TestProgress) {
				// Setup progress describes work the sequential path reached.
				// Compare the compile outcome and every selected case's outcome.
				if p.Event == "compile_start" || p.Event == "compile_done" || p.Event == "test_done" {
					p.DurationMS = 0
					events = append(events, p)
				}
			}}
			var gate semanticGateHooks
			if setting == "0" {
				gate.afterIdentity = cancel
			} else {
				opts.testBuildHooks = &runtimeBuildHooks{stage: func(stage string) {
					if stage == "after_methods" {
						cancel()
					}
				}}
			}
			run := runCasesContextWithSemanticGateHooks(ctx, index, opts, selected, gate)
			interrupted := ctx.Err() != nil
			cancel()
			if !interrupted || run.Summary().Total != len(selected) {
				t.Fatalf("setting %s: canceled=%t, summary=%+v", setting, interrupted, run.Summary())
			}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Status != testreport.StatusUnsupported || c.Reason != testreport.ReasonCancelled {
						t.Errorf("setting %s: %s.%s got %s/%s", setting, c.ClassName, c.MethodName, c.Status, c.Reason)
					}
				}
			}
			got := besideGateOutcomeDigest(t, run)
			if setting == "0" {
				want, wantEvents = got, events
			} else if got != want || !reflect.DeepEqual(events, wantEvents) {
				t.Errorf("canceled run differs: report %s, want %s; progress %+v, want %+v", got, want, events, wantEvents)
			}
		}
	}
}

func TestDirectRuntimePublicationIndependentOfQueuedWrite(t *testing.T) {
	t.Setenv(runtimeBesideGateEnv, "0")
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	index, artifacts := digestProject(t, 1, false)
	root := index.Project.Root
	queued := false
	func() {
		backgroundPublications.Lock()
		defer backgroundPublications.Unlock()
		publishInBackground(root, func() { queued = true })
		run := Run(index, Options{SourceDigests: artifacts.SourceDigests})
		if run.Summary().Passed != 1 {
			t.Errorf("default build: %+v", run.Summary())
		}
		entry, err := startupcache.Read(root, startupcache.SubdirTest)
		if err != nil || entry == nil || entry.RuntimeKey != RuntimeContentKey(index, artifacts.SourceDigests) {
			t.Errorf("synchronous build did not publish its exact key: %v", err)
		}
	}()
	WaitForRuntimeCacheWork()
	if !queued {
		t.Error("synchronous publication suppressed an independent queued write")
	}
}

func TestSequentialBuildWorkerPanicKeepsStack(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)
	defer func() {
		p, ok := recover().(*runtimeBuildPanic)
		if !ok || p.value != "worker failed" || !bytes.Contains(p.stack, []byte("panicInSequentialWorker")) {
			t.Fatal("worker panic lost its original value or stack")
		}
		if message := fmt.Sprint(p); !strings.Contains(message, "worker failed") || !strings.Contains(message, "panicInSequentialWorker") {
			t.Fatal("worker panic omits its cause or stack when printed")
		}
	}()
	runRuntimeBuildJobs(64, nil, "compile_method", panicInSequentialWorker)
}

func panicInSequentialWorker(int) { panic("worker failed") }

func TestRuntimeBuildPanicLogIncludesBoundsError(t *testing.T) {
	defer func() {
		err, ok := recover().(runtime.Error)
		if !ok {
			t.Fatal("expected a Go bounds error")
		}
		var out bytes.Buffer
		logRuntimeBuildPanic(&out, &runtimeBuildPanic{value: err}, false)
		if !strings.Contains(out.String(), "value="+strconv.Quote(err.Error())) {
			t.Fatalf("missing bounds error: %s", out.String())
		}
	}()
	var empty []int
	_ = empty[0]
}
