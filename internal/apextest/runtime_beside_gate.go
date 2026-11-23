package apextest

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"

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

// runtimeBesideGateStartHook is a test seam run on the build goroutine before
// the build starts.
var runtimeBesideGateStartHook func()

// runtimeBesideGate is a project runtime build started before the semantic
// gate finishes. The build runs exactly as the sequential path does; only
// Visualforce page registration, the one step that reads the gate's result,
// waits for it.
//
// The build never outlives its run. abandon waits for the goroutine to exit,
// so an abandoned build cannot overlap a later run, a cache invalidation or
// a test's cleanup of process-global seams, and a panic it raises cannot be
// lost while it leaves shared state behind for the next build. A rejected
// build checks rejected between compile phases and stops at the next one.
type runtimeBesideGate struct {
	once     sync.Once
	hint     chan runtimeGateHint
	result   chan runtimeBesideGateResult
	done     chan struct{}
	rejected atomic.Bool
}

type runtimeGateHint struct {
	compilation *visualforce.ControllerCompilation
	ok          bool
}

type runtimeBesideGateResult struct {
	key     runtimeCacheKey
	runtime runtimeExecutionView
	err     error
	// panicked holds a panic the build raised; finish raises it again on the
	// run's goroutine, where the sequential build would have raised it.
	panicked *runtimeBuildPanic
}

// runtimeBuildPanic is a panic recovered on the build goroutine. It keeps
// the original value and the stack captured at recover time, which the
// re-raise on the run's goroutine would otherwise lose.
type runtimeBuildPanic struct {
	value any
	stack []byte
}

func (p *runtimeBuildPanic) Error() string {
	return fmt.Sprintf("runtime build beside the semantic gate panicked: %v\n\noriginal stack:\n%s", p.value, p.stack)
}

// Unwrap exposes an error panic value, such as a runtime.Error, to errors.As.
func (p *runtimeBuildPanic) Unwrap() error {
	err, _ := p.value.(error)
	return err
}

// runtimeBesideGateBuilds counts builds still running beside a gate,
// abandoned ones included; WaitForRuntimeCacheWork drains them.
var runtimeBesideGateBuilds sync.WaitGroup

// startRuntimeBesideGate starts the build when another processor can take
// it. Perf-counter runs stay sequential so their phase timings keep their
// meaning.
func startRuntimeBesideGate(index typesys.Index, opts Options, sources *sourceCache, generation runtimeGeneration, runState *runExecution) *runtimeBesideGate {
	if runState.counters.enabled || runtime.GOMAXPROCS(0) < 2 {
		return nil
	}
	early := &runtimeBesideGate{hint: make(chan runtimeGateHint, 1), result: make(chan runtimeBesideGateResult, 1), done: make(chan struct{})}
	var received runtimeGateHint
	var receive sync.Once
	generation.awaitControllerCompilation = func() (*visualforce.ControllerCompilation, bool) {
		receive.Do(func() { received = <-early.hint })
		return received.compilation, received.ok
	}
	generation.besideGateRejected = early.rejected.Load
	// The seam is read here, on the run's goroutine, so a test that restores
	// it cannot race the build.
	startHook := runtimeBesideGateStartHook
	runtimeBesideGateBuilds.Add(1)
	go func() {
		defer runtimeBesideGateBuilds.Done()
		defer close(early.done)
		var result runtimeBesideGateResult
		completed := false
		defer func() {
			if recovered := recover(); recovered != nil {
				result = runtimeBesideGateResult{err: errRuntimeAbandoned, panicked: &runtimeBuildPanic{value: recovered, stack: debug.Stack()}}
			} else if !completed {
				// runtime.Goexit: the build returned no result. Never hand
				// finish a zero runtime as a success.
				result = runtimeBesideGateResult{err: errRuntimeBuildIncomplete}
			}
			early.result <- result
		}()
		if startHook != nil {
			startHook()
		}
		key, view, err := runtimeFromIndexWithPreparedGenerationProjected(index, opts.SourceDigests, sources, &generation, runtimeExecutionProjection, runState.diskCacheEnabled)
		result = runtimeBesideGateResult{key: key, runtime: view, err: err}
		completed = true
	}()
	return early
}

func (e *runtimeBesideGate) send(hint runtimeGateHint) {
	e.once.Do(func() {
		if !hint.ok {
			e.rejected.Store(true)
		}
		e.hint <- hint
	})
}

// abandon releases a build the run no longer needs and waits for it to
// exit. The build stores nothing: it stops at its next compile phase or at
// page registration. After finish, abandon returns at once. A panic of an
// abandoned build is dropped, as the sequential path never built a rejected
// index.
func (e *runtimeBesideGate) abandon() {
	if e == nil {
		return
	}
	e.send(runtimeGateHint{})
	<-e.done
}

// finish hands the build the gate's controller compilation and returns its
// result.
func (e *runtimeBesideGate) finish(compilation *visualforce.ControllerCompilation) (runtimeCacheKey, runtimeExecutionView, error) {
	e.send(runtimeGateHint{compilation: compilation, ok: true})
	result := <-e.result
	if result.panicked != nil {
		panic(result.panicked)
	}
	return result.key, result.runtime, result.err
}
