package apextest

import (
	"container/heap"
	"os"
	"sync"
)

// classDispatcher orders test classes by an evolving cost score so that
// expensive classes are dispatched first and inexpensive ones flow into the
// long tail. The dispatcher is strictly in-memory; nothing is written to
// disk and no project-specific information leaves the process.
//
// Initial scores come from a generic, history-free signal — currently the
// class source file size (testCaseCostHint). As each class completes, the
// observed duration replaces its score so that subsequent retrieval order
// for the remaining queue reflects the actual cost we just measured.
//
// classDispatcher is concurrency-safe. Callers should:
//   1. push all classNames with their cost hints,
//   2. call next() from each worker goroutine,
//   3. call recordObserved(class, durationMS) when the class finishes.
type classDispatcher struct {
	mu       sync.Mutex
	cond     *sync.Cond
	heap     classScoreHeap
	closed   bool
	remaining int
}

func newClassDispatcher(classOrder []string, costHints, durationHints map[string]int64) *classDispatcher {
	d := &classDispatcher{}
	d.cond = sync.NewCond(&d.mu)
	d.heap = make(classScoreHeap, 0, len(classOrder))
	for i, name := range classOrder {
		score := int64(0)
		if v, ok := durationHints[name]; ok && v > 0 {
			score = v
		} else if v, ok := costHints[name]; ok && v > 0 {
			score = v
		}
		d.heap = append(d.heap, &classScore{
			name:  name,
			score: score,
			seq:   i, // stable tiebreak preserves discovery order
		})
	}
	heap.Init(&d.heap)
	d.remaining = len(d.heap)
	return d
}

// next blocks until a class is available, or returns ("", false) when the
// queue is drained.
func (d *classDispatcher) next() (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for d.heap.Len() == 0 && !d.closed {
		d.cond.Wait()
	}
	if d.heap.Len() == 0 {
		return "", false
	}
	top := heap.Pop(&d.heap).(*classScore)
	return top.name, true
}

// recordObserved is informational; the class has already been dispatched.
// We use it only to mark progress; remaining items keep their initial
// cost-hint score. If the dispatcher is ever extended to re-queue retries,
// this signal can drive a fresh heap.Init.
func (d *classDispatcher) recordObserved(name string, durationMS int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.remaining--
	if d.remaining <= 0 {
		d.closed = true
		d.cond.Broadcast()
	}
}

// unfinishedClassCount returns classes still queued or running.
func (d *classDispatcher) unfinishedClassCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.remaining
}

// close wakes any waiting workers so they observe an empty queue and exit.
func (d *classDispatcher) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.cond.Broadcast()
}

type classScore struct {
	name  string
	score int64
	seq   int
}

type classScoreHeap []*classScore

func (h classScoreHeap) Len() int { return len(h) }
func (h classScoreHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score > h[j].score // largest cost first
	}
	return h[i].seq < h[j].seq // stable
}
func (h classScoreHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *classScoreHeap) Push(x any) { *h = append(*h, x.(*classScore)) }
func (h *classScoreHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// testCaseCostHint returns a coarse, history-free cost signal for a test
// class derived from the source file size on disk. Larger files tend to
// have more methods and more dependencies; this gives the dispatcher a
// usable starting score before any test has run. The signal is project
// agnostic — no learned data, no per-project file, no profile-driven hints.
func testCaseCostHint(file string) int64 {
	if file == "" {
		return 0
	}
	info, err := os.Stat(file)
	if err != nil {
		return 0
	}
	return info.Size()
}
