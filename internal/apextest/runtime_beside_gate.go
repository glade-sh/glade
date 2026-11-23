package apextest

import (
	"errors"
	"runtime"
	"sync"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

// errRuntimeAbandoned ends a runtime build started beside the semantic gate
// when the gate fails or reports a compile error. The build stops before page
// registration and stores nothing.
var errRuntimeAbandoned = errors.New("runtime build abandoned")

// runtimeBesideGate is a project runtime build started before the semantic
// gate finishes. The build runs exactly as the sequential path does; only
// Visualforce page registration, the one step that reads the gate's result,
// waits for it.
type runtimeBesideGate struct {
	once   sync.Once
	hint   chan runtimeGateHint
	result chan runtimeBesideGateResult
}

type runtimeGateHint struct {
	compilation *visualforce.ControllerCompilation
	ok          bool
}

type runtimeBesideGateResult struct {
	key     runtimeCacheKey
	runtime runtimeExecutionView
	err     error
}

// startRuntimeBesideGate starts the build when another processor can take
// it. Perf-counter runs stay sequential so their phase timings keep their
// meaning.
func startRuntimeBesideGate(index typesys.Index, opts Options, sources *sourceCache, generation runtimeGeneration, runState *runExecution) *runtimeBesideGate {
	if runState.counters.enabled || runtime.GOMAXPROCS(0) < 2 {
		return nil
	}
	early := &runtimeBesideGate{hint: make(chan runtimeGateHint, 1), result: make(chan runtimeBesideGateResult, 1)}
	var received runtimeGateHint
	var receive sync.Once
	generation.awaitControllerCompilation = func() (*visualforce.ControllerCompilation, bool) {
		receive.Do(func() { received = <-early.hint })
		return received.compilation, received.ok
	}
	go func() {
		key, view, err := runtimeFromIndexWithPreparedGenerationProjected(index, opts.SourceDigests, sources, &generation, runtimeExecutionProjection, runState.diskCacheEnabled)
		early.result <- runtimeBesideGateResult{key: key, runtime: view, err: err}
	}()
	return early
}

func (e *runtimeBesideGate) send(hint runtimeGateHint) {
	e.once.Do(func() { e.hint <- hint })
}

// abandon releases a build the run no longer needs. It stops at page
// registration without storing anything; the run does not wait for it.
func (e *runtimeBesideGate) abandon() {
	if e == nil {
		return
	}
	e.send(runtimeGateHint{})
}

// finish hands the build the gate's controller compilation and returns its
// result.
func (e *runtimeBesideGate) finish(compilation *visualforce.ControllerCompilation) (runtimeCacheKey, runtimeExecutionView, error) {
	e.send(runtimeGateHint{compilation: compilation, ok: true})
	result := <-e.result
	return result.key, result.runtime, result.err
}
