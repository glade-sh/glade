package apextest

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/startupcache"
)

// Only a build owned by the semantic gate has a work scope. The private
// sentinel unwinds partially built values through compiler and scanner APIs
// that also serve callers without a cancellation or error return.
type runtimeBuildWork struct {
	ctx        context.Context
	stage      func(string)
	discardErr error
}

type runtimeBuildCanceled struct{}

func (w *runtimeBuildWork) check() {
	if w != nil && w.ctx.Err() != nil {
		panic(runtimeBuildCanceled{})
	}
}

func (w *runtimeBuildWork) at(name string) {
	w.check()
	if w != nil && w.stage != nil {
		w.stage(name)
	}
	w.check()
}

// Keep rollback inside the root's publication lock. A later run can never
// have its completed write removed by cleanup of this attempt.
func (w *runtimeBuildWork) publish(root string, write func()) {
	completed := false
	defer func() {
		if !completed {
			w.discardErr = startupcache.Clear(root, startupcache.SubdirTest)
		}
	}()
	write()
	if w.stage != nil {
		w.stage("after_publish")
	}
	completed = true
}

// Each job owns an index. No worker blocks on a producer after a sibling
// exits. Capture the original worker stack, join all workers, and unwind on
// the owning goroutine, whose result is delivered to the run.
func runRuntimeBuildJobs(count int, work *runtimeBuildWork, stage string, job func(int)) {
	workers := compileWorkers(count)
	if workers <= 1 {
		for i := range count {
			work.at(stage)
			job(i)
		}
		return
	}
	var next atomic.Int64
	var failed atomic.Pointer[runtimeBuildPanic]
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			completed := false
			defer func() {
				if !completed {
					value := recover()
					if _, canceled := value.(runtimeBuildCanceled); canceled {
						return
					}
					if value == nil {
						value = errRuntimeBuildIncomplete
					}
					failed.CompareAndSwap(nil, &runtimeBuildPanic{value: value, stack: debug.Stack()})
				}
			}()
			for failed.Load() == nil {
				i := int(next.Add(1) - 1)
				if i >= count {
					break
				}
				work.at(stage)
				job(i)
			}
			completed = true
		}()
	}
	group.Wait()
	if failure := failed.Load(); failure != nil {
		panic(failure)
	}
	work.check()
}
