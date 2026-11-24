package apextest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

// errRuntimeAbandoned ends a runtime build started beside the semantic gate
// when the gate fails or reports a compile error. The build stops before page
// registration and stores nothing.
var errRuntimeAbandoned = errors.New("runtime build abandoned")

// errRuntimeBuildIncomplete reports a build goroutine that exited without a
// result or a panic (runtime.Goexit).
var errRuntimeBuildIncomplete = errors.New("runtime build beside the semantic gate exited without a result")

// testOnlyRuntimeBesideGateStartHook is a package test seam run on the build
// goroutine before the build starts. Nothing outside _test.go files sets it.
var testOnlyRuntimeBesideGateStartHook func()

// testOnlyRuntimeBesideGateStageHook is a package test seam a build beside
// the gate calls at named points of a partly completed build
// ("after_methods", "after_org", "after_publish"). Nothing outside _test.go
// files sets it.
var testOnlyRuntimeBesideGateStageHook func(stage string)

// testOnlyRuntimeBesideGateLog, when set by a package test, receives the
// build-panic log line instead of os.Stderr.
var testOnlyRuntimeBesideGateLog io.Writer

// runtimeBesideGateEnv opts in to the build beside the semantic gate. The
// build is off by default in v0.3.14: runs use the sequential runtime build
// (the v0.3.13 path) unless this is exactly "1".
const runtimeBesideGateEnv = "GLADE_EXPERIMENTAL_BESIDE_GATE"

// testOnlyRuntimeBesideGateEnabled lets package tests opt in without the
// environment. Nothing outside _test.go files sets it.
var testOnlyRuntimeBesideGateEnabled atomic.Bool

// ExperimentalRuntimeBesideGate reports the process opt-in for the runner and daemon.
func ExperimentalRuntimeBesideGate() bool { return os.Getenv(runtimeBesideGateEnv) == "1" }

func runtimeBesideGateEnabled() bool {
	return testOnlyRuntimeBesideGateEnabled.Load() || ExperimentalRuntimeBesideGate()
}

// runtimeBesideGateBuildIDs numbers builds for the panic log line.
var runtimeBesideGateBuildIDs atomic.Uint64

// runtimeBesideGate is a project runtime build started before the semantic
// gate finishes. It runs only when runtimeBesideGateEnabled. The build runs exactly as the sequential path does; only
// Visualforce page registration, the one step that reads the gate's result,
// waits for it.
//
// The build never outlives its run. abandon waits for the goroutine to exit,
// so an abandoned build cannot overlap a later run, a cache invalidation or
// a test's cleanup of process-global seams, and a panic it raises cannot be
// lost while it leaves shared state behind for the next build. A rejected
// build checks rejected between compile phases and stops at the next one.
type runtimeBesideGate struct {
	once   sync.Once
	hint   chan runtimeGateHint
	done   chan struct{}
	cancel context.CancelFunc
	result runtimeBesideGateResult // read only after done closes
}

type runtimeGateHint struct {
	compilation *visualforce.ControllerCompilation
	ok          bool
}

type runtimeBesideGateResult struct {
	key        runtimeCacheKey
	runtime    runtimeExecutionView
	err        error
	retry      bool
	discardErr error
}

// runtimeBuildPanic is a panic recovered on the build goroutine, with the
// original value and the stack captured at recover time.
type runtimeBuildPanic struct {
	build uint64
	value any
	stack []byte
}

const runtimeBuildPanicStackLimit = 8 << 10

// Preserve the familiar nil-dereference diagnostic without ever calling a
// panic value's Error or String method. Those methods may panic or print source.
func logRuntimeBuildPanic(out io.Writer, p *runtimeBuildPanic, abandoned bool) {
	state := "accepted; the run falls back to the sequential build"
	if abandoned {
		state = "abandoned"
	}
	value := "<not shown>"
	v := reflect.ValueOf(p.value)
	const nilFault = "invalid memory address or nil pointer dereference"
	if v.Kind() == reflect.String && v.Type().PkgPath() == "runtime" && v.String() == nilFault {
		value = nilFault
	}
	stack := truncateRuntimeBuildPanicText(string(p.stack), runtimeBuildPanicStackLimit)
	_, _ = fmt.Fprintf(out, "glade: runtime build %d beside the semantic gate panicked (%s): type=%T value=%q stack=%q\n", p.build, state, p.value, value, stack)
}

func truncateRuntimeBuildPanicText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "...(truncated)"
}

// runtimeBesideGateBuilds counts builds still running beside a gate,
// abandoned ones included; WaitForRuntimeCacheWork drains them.
var runtimeBesideGateBuilds sync.WaitGroup

// startRuntimeBesideGate starts the build when another processor can take
// it. Perf-counter runs stay sequential so their phase timings keep their
// meaning.
func startRuntimeBesideGate(ctx context.Context, index typesys.Index, opts Options, sources *sourceCache, generation runtimeGeneration, runState *runExecution) *runtimeBesideGate {
	if !runtimeBesideGateEnabled() || runState.counters.enabled || runtime.GOMAXPROCS(0) < 2 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	early := &runtimeBesideGate{hint: make(chan runtimeGateHint, 1), done: make(chan struct{}), cancel: cancel}
	var received runtimeGateHint
	var receive sync.Once
	generation.awaitControllerCompilation = func() (*visualforce.ControllerCompilation, bool) {
		receive.Do(func() { received = <-early.hint })
		return received.compilation, received.ok
	}
	work := &runtimeBuildWork{ctx: ctx, stage: testOnlyRuntimeBesideGateStageHook}
	// Only runtime assembly consults work. The semantic gate shares the
	// captured inputs; it owns its own cancellation and never consults work.
	sources.build = work
	startHook := testOnlyRuntimeBesideGateStartHook
	var out io.Writer = os.Stderr
	if testOnlyRuntimeBesideGateLog != nil {
		out = testOnlyRuntimeBesideGateLog
	}
	id := runtimeBesideGateBuildIDs.Add(1)
	runtimeBesideGateBuilds.Add(1)
	go func() {
		defer runtimeBesideGateBuilds.Done()
		defer close(early.done)
		defer cancel()
		defer func() { sources.build = nil }()
		completed := false
		defer func() {
			if completed {
				return
			}
			recovered := recover()
			if _, stopped := recovered.(runtimeBuildCanceled); stopped {
				early.result.err = ctx.Err()
				return
			}
			_, accepted := generation.awaitControllerCompilation()
			failure, worker := recovered.(*runtimeBuildPanic)
			if !worker {
				failure = &runtimeBuildPanic{value: recovered, stack: debug.Stack()}
			}
			if recovered == nil {
				failure.value = errRuntimeBuildIncomplete
			}
			failure.build = id
			logRuntimeBuildPanic(out, failure, !accepted)
			early.result.retry = true
			early.result.discardErr = discardFailedBesideGateBuild(index, work.diskPublication)
		}()
		if startHook != nil {
			startHook()
		}
		early.result.key, early.result.runtime, early.result.err = runtimeFromIndexWithPreparedGenerationProjected(index, opts.SourceDigests, sources, &generation, runtimeExecutionProjection, runState.diskCacheEnabled)
		completed = true
	}()
	return early
}

func (e *runtimeBesideGate) decide(hint runtimeGateHint) runtimeBesideGateResult {
	if e == nil {
		return runtimeBesideGateResult{}
	}
	e.once.Do(func() {
		if !hint.ok {
			e.cancel()
		}
		e.hint <- hint
	})
	<-e.done
	return e.result
}

func (e *runtimeBesideGate) abandon() runtimeBesideGateResult { return e.decide(runtimeGateHint{}) }

// A gated build publishes synchronously. Fence any older queued writes
// before removing a disk publication this build actually attempted.
func discardFailedBesideGateBuild(index typesys.Index, wroteDisk bool) error {
	resetFailedRuntimePlatform()
	InvalidateRuntimeCaches()
	if root := strings.TrimSpace(index.Project.Root); wroteDisk && root != "" {
		root = filepath.Clean(root)
		var err error
		orderedPublication(root, func() { err = startupcache.Clear(root, startupcache.SubdirTest) })()
		return err
	}
	return nil
}
