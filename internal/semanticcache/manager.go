package semanticcache

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/glade-sh/glade/internal/sema"
)

type Source string

const (
	SourceMemory Source = "memory"
	SourceDisk   Source = "disk"
	SourceBuild  Source = "build"
)

type Limits struct {
	MaxEntries int
	MaxBytes   int64
}

type Request struct {
	Identity     Identity
	ProjectRoot  string
	RelativePath string
	NoDisk       bool
	BypassMemory bool
}

type Access struct {
	Source         Source
	Waited         bool
	DiskMissReason MissReason
	RetainedBytes  int64
	Evictions      uint64
}

type Stats struct {
	Entries       int
	RetainedBytes int64
	Evictions     uint64
}

type managerEntry struct {
	key      string
	snapshot sema.ResultSnapshot
	size     int64
	active   int
	element  *list.Element
}

type managerFlight struct {
	done   chan struct{}
	epoch  uint64
	result sema.ResultSnapshot
	access Access
	err    error
}

// Manager bounds immutable semantic results and collapses exact concurrent
// requests. Callers receive private result copies, never the cache-owned value.
type Manager struct {
	mu            sync.Mutex
	publicationMu sync.Mutex
	workMu        sync.Mutex
	workCond      *sync.Cond
	activeWork    int
	limits        Limits
	entries       map[string]*managerEntry
	lru           list.List
	flights       map[string]*managerFlight
	bytes         int64
	evictions     uint64
	epoch         uint64
}

func NewManager(limits Limits) *Manager {
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = 8
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = 512 << 20
	}
	manager := &Manager{
		limits:  limits,
		entries: make(map[string]*managerEntry),
		flights: make(map[string]*managerFlight),
	}
	manager.workCond = sync.NewCond(&manager.workMu)
	return manager
}

func (manager *Manager) GetOrCompute(ctx context.Context, request Request, compute func() (sema.Result, error)) (sema.Result, Access, error) {
	if manager == nil {
		return sema.Result{}, Access{}, fmt.Errorf("semantic cache manager is nil")
	}
	if err := request.Identity.validate(); err != nil {
		return sema.Result{}, Access{}, err
	}
	if err := ctx.Err(); err != nil {
		return sema.Result{}, Access{}, err
	}
	if request.BypassMemory {
		result, err := compute()
		if err == nil {
			err = ctx.Err()
		}
		return result, Access{Source: SourceBuild}, err
	}
	key, err := identityKey(request.Identity)
	if err != nil {
		return sema.Result{}, Access{}, err
	}
	if result, access, ok := manager.memoryResult(key); ok {
		return result, access, nil
	}

	manager.mu.Lock()
	joined := false
	if flight := manager.flights[key]; flight != nil {
		joined = true
		manager.mu.Unlock()
		return waitForFlight(ctx, flight, joined)
	}
	flight := &managerFlight{done: make(chan struct{}), epoch: manager.epoch}
	manager.flights[key] = flight
	manager.mu.Unlock()

	manager.beginWork()
	go manager.runFlight(request, key, flight, compute)
	return waitForFlight(ctx, flight, joined)
}

func waitForFlight(ctx context.Context, flight *managerFlight, waited bool) (sema.Result, Access, error) {
	select {
	case <-ctx.Done():
		return sema.Result{}, Access{Waited: waited}, ctx.Err()
	case <-flight.done:
		access := flight.access
		access.Waited = waited
		return flight.result.Result(), access, flight.err
	}
}

func (manager *Manager) runFlight(request Request, key string, flight *managerFlight, compute func() (sema.Result, error)) {
	defer manager.endWork()
	result, access, computeErr := manager.resolveLeader(request, key, flight.epoch, compute)
	flight.result = sema.SnapshotResult(result)
	flight.access = access
	flight.err = computeErr

	manager.mu.Lock()
	if manager.flights[key] == flight {
		delete(manager.flights, key)
	}
	close(flight.done)
	manager.mu.Unlock()
}

// Wait blocks until all detached shared computations have completed. Callers
// must first stop admitting new work.
func (manager *Manager) Wait() {
	if manager == nil {
		return
	}
	manager.workMu.Lock()
	for manager.activeWork != 0 {
		manager.workCond.Wait()
	}
	manager.workMu.Unlock()
}

func (manager *Manager) beginWork() {
	manager.workMu.Lock()
	manager.activeWork++
	manager.workMu.Unlock()
}

func (manager *Manager) endWork() {
	manager.workMu.Lock()
	manager.activeWork--
	if manager.activeWork == 0 {
		manager.workCond.Broadcast()
	}
	manager.workMu.Unlock()
}

func (manager *Manager) resolveLeader(request Request, key string, epoch uint64, compute func() (sema.Result, error)) (sema.Result, Access, error) {
	if result, access, ok := manager.memoryResult(key); ok {
		return result, access, nil
	}
	access := Access{}
	if !request.NoDisk && request.ProjectRoot != "" && request.RelativePath != "" {
		result, err := Load(request.ProjectRoot, request.RelativePath, request.Identity)
		if err == nil {
			manager.publish(epoch, key, result, nil)
			access = manager.access(SourceDisk)
			return result, access, nil
		}
		var miss *MissError
		if !errors.As(err, &miss) {
			return sema.Result{}, access, err
		}
		access.DiskMissReason = miss.Reason
	}
	result, err := compute()
	if err != nil {
		return sema.Result{}, access, err
	}
	missReason := access.DiskMissReason
	var persist func()
	if !request.NoDisk && request.ProjectRoot != "" && request.RelativePath != "" {
		persist = func() {
			// Persistence is an optimization. A valid semantic result remains
			// authoritative if the private cache cannot be written.
			_ = Store(request.ProjectRoot, request.RelativePath, request.Identity, result)
		}
	}
	manager.publish(epoch, key, result, persist)
	access = manager.access(SourceBuild)
	access.DiskMissReason = missReason
	return result, access, nil
}

func (manager *Manager) publish(epoch uint64, key string, result sema.Result, persist func()) {
	manager.publicationMu.Lock()
	defer manager.publicationMu.Unlock()
	manager.mu.Lock()
	current := manager.epoch == epoch
	manager.mu.Unlock()
	if !current {
		return
	}
	if persist != nil {
		persist()
	}
	manager.insert(key, result)
}

func (manager *Manager) memoryResult(key string) (sema.Result, Access, bool) {
	manager.mu.Lock()
	entry := manager.entries[key]
	if entry == nil {
		manager.mu.Unlock()
		return sema.Result{}, Access{}, false
	}
	entry.active++
	manager.lru.MoveToFront(entry.element)
	snapshot := entry.snapshot
	manager.mu.Unlock()

	result := snapshot.Result()

	manager.mu.Lock()
	entry.active--
	manager.evictLocked()
	access := manager.accessLocked(SourceMemory)
	manager.mu.Unlock()
	return result, access, true
}

func (manager *Manager) insert(key string, result sema.Result) {
	snapshot := sema.SnapshotResult(result)
	encoded, _ := json.Marshal(snapshot)
	size := int64(len(encoded)) + sema.EstimateResultRetainedBytes(result)

	manager.mu.Lock()
	if existing := manager.entries[key]; existing != nil {
		manager.bytes -= existing.size
		existing.snapshot = snapshot
		existing.size = size
		manager.bytes += size
		manager.lru.MoveToFront(existing.element)
	} else {
		entry := &managerEntry{key: key, snapshot: snapshot, size: size}
		entry.element = manager.lru.PushFront(entry)
		manager.entries[key] = entry
		manager.bytes += size
	}
	manager.evictLocked()
	manager.mu.Unlock()
}

func (manager *Manager) evictLocked() {
	for len(manager.entries) > manager.limits.MaxEntries || manager.bytes > manager.limits.MaxBytes {
		var victim *managerEntry
		for element := manager.lru.Back(); element != nil; element = element.Prev() {
			candidate := element.Value.(*managerEntry)
			if candidate.active == 0 {
				victim = candidate
				break
			}
		}
		if victim == nil {
			return
		}
		delete(manager.entries, victim.key)
		manager.lru.Remove(victim.element)
		manager.bytes -= victim.size
		manager.evictions++
	}
}

func (manager *Manager) access(source Source) Access {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.accessLocked(source)
}

func (manager *Manager) accessLocked(source Source) Access {
	return Access{Source: source, RetainedBytes: manager.bytes, Evictions: manager.evictions}
}

func (manager *Manager) Stats() Stats {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return Stats{Entries: len(manager.entries), RetainedBytes: manager.bytes, Evictions: manager.evictions}
}

func (manager *Manager) Reset() {
	manager.publicationMu.Lock()
	defer manager.publicationMu.Unlock()
	manager.mu.Lock()
	manager.epoch++
	manager.entries = make(map[string]*managerEntry)
	manager.flights = make(map[string]*managerFlight)
	manager.lru.Init()
	manager.bytes = 0
	manager.evictions = 0
	manager.mu.Unlock()
}

func (manager *Manager) Evict(identity Identity) {
	key, err := identityKey(identity)
	if err != nil {
		return
	}
	manager.mu.Lock()
	if entry := manager.entries[key]; entry != nil && entry.active == 0 {
		delete(manager.entries, key)
		manager.lru.Remove(entry.element)
		manager.bytes -= entry.size
		manager.evictions++
	}
	manager.mu.Unlock()
}

func identityKey(identity Identity) (string, error) {
	data, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}
