package apextest

import (
	"reflect"
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

// A run may mix classes with and without @TestSetup. No-setup classes all
// start at the same seed; their journal workers can be reused across classes
// after rollback. Setup orgs and source-unit metadata projections never enter
// this pool. A worker whose schema changed is discarded.
type testWorkerOrgPool struct {
	mu       sync.Mutex
	seed     storage.OrgState
	capacity int
	idle     []*testWorkerOrg
}

type testWorkerOrg struct {
	org               storage.OrgState
	journal           *storage.IsolationJournal
	generation        uint64
	runtimeGeneration uint64
}

func (p *testWorkerOrgPool) accepts(org storage.OrgState, shared bool) bool {
	return p != nil && shared && org.Objects != nil &&
		reflect.ValueOf(org.Objects).Pointer() == reflect.ValueOf(p.seed.Objects).Pointer()
}

func (p *testWorkerOrgPool) take(className string, counters *runPerfCounters) *testWorkerOrg {
	p.mu.Lock()
	if n := len(p.idle); n > 0 {
		worker := p.idle[n-1]
		p.idle[n-1] = nil
		p.idle = p.idle[:n-1]
		p.mu.Unlock()
		return worker
	}
	p.mu.Unlock()
	worker := &testWorkerOrg{org: cloneRuntimeOrgForClass(p.seed, className, "test-worker", counters)}
	worker.generation = worker.org.SchemaGeneration()
	worker.runtimeGeneration = worker.org.RuntimeGenerationID()
	worker.journal = storage.NewIsolationJournal(&worker.org)
	return worker
}

func (p *testWorkerOrgPool) put(worker *testWorkerOrg) {
	if worker.org.SchemaGeneration() != worker.generation || worker.org.RuntimeGenerationID() != worker.runtimeGeneration || worker.journal.Rollback(storage.IsolationMark{}) != nil {
		return
	}
	// Runtime clocks capture the last VM. Do not retain that VM while idle.
	worker.org.Now = nil
	worker.org.Today = nil
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle) < p.capacity {
		p.idle = append(p.idle, worker)
	}
}
