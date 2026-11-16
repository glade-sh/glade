package vm

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// strictListAppendSource builds a list of n records in one loop, the shape
// whose strict per-statement heap check once walked the whole list on every
// add.
func strictListAppendSource(n int) string {
	return fmt.Sprintf(`List<Account> l = new List<Account>(); for (Integer i = 0; i < %d; i++) { l.add(new Account(Name='a'+i)); }`, n)
}

// strictListAppendModSource is the same build with calls in the loop: a
// Math.mod condition and a size() bound, which once forced a full heap walk.
func strictListAppendModSource(n int) string {
	return fmt.Sprintf(`List<Account> l = new List<Account>(); Integer hits = 0;
for (Integer i = 0; i < %d; i++) { l.add(new Account(Name='a'+i)); if (Math.mod(i, 97) == 0) { hits++; } }
Integer j = 0; while (j < l.size()) { j++; }`, n)
}

func runStrictListAppend(tb testing.TB, n int) time.Duration {
	tb.Helper()
	return runStrictListSource(tb, strictListAppendSource(n), n)
}

func runStrictListSource(tb testing.TB, source string, n int) time.Duration {
	tb.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		tb.Fatal(err)
	}
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	start := time.Now()
	result, err := machine.Execute(program)
	elapsed := time.Since(start)
	if err != nil {
		tb.Fatal(err)
	}
	if got := len(result.Vars["l"].List); got != n {
		tb.Fatalf("list size %d, want %d", got, n)
	}
	return elapsed
}

// Strict mode must price a list-building loop in linear time. Six times the
// rows may cost at most eighteen times as long: linear growth is about six,
// a whole-list walk per statement about thirty-six. The best of three runs
// keeps a busy host from failing it.
func TestStrictHeapListAppendScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing comparison")
	}
	previous := heapVerifyAgainstFullWalk
	heapVerifyAgainstFullWalk = false
	defer func() { heapVerifyAgainstFullWalk = previous }()
	const small, large = 1000, 6000
	best := func(n int) time.Duration {
		fastest := time.Duration(1<<63 - 1)
		for range 3 {
			fastest = min(fastest, runStrictListAppend(t, n))
		}
		return fastest
	}
	runStrictListAppend(t, small) // warm caches shared by every machine
	smallTime, largeTime := best(small), best(large)
	if ratio := float64(largeTime) / float64(smallTime); ratio > 18 {
		t.Fatalf("strict list append of %d rows took %v, %d rows %v: ratio %.1f > 18", large, largeTime, small, smallTime, ratio)
	}
}

// The same bound holds when the loop calls pure builtins (Math.mod, size()).
func TestStrictHeapPureCallLoopScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing comparison")
	}
	previous := heapVerifyAgainstFullWalk
	heapVerifyAgainstFullWalk = false
	defer func() { heapVerifyAgainstFullWalk = previous }()
	const small, large = 1000, 6000
	best := func(n int) time.Duration {
		fastest := time.Duration(1<<63 - 1)
		for range 3 {
			fastest = min(fastest, runStrictListSource(t, strictListAppendModSource(n), n))
		}
		return fastest
	}
	runStrictListSource(t, strictListAppendModSource(small), small)
	smallTime, largeTime := best(small), best(large)
	t.Logf("pure-call list build: %d rows %v, %d rows %v", small, smallTime, large, largeTime)
	if ratio := float64(largeTime) / float64(smallTime); ratio > 18 {
		t.Fatalf("pure-call loop of %d rows took %v, %d rows %v: ratio %.1f > 18", large, largeTime, small, smallTime, ratio)
	}
}

// Report timings for 1k, 4k and 8k rows: go test -run TestStrictHeapPureCallTimings -v.
func TestStrictHeapPureCallTimings(t *testing.T) {
	if testing.Short() || os.Getenv("GLADE_VM_HEAP_TIMINGS") == "" {
		t.Skip("set GLADE_VM_HEAP_TIMINGS=1 to print timings")
	}
	previous := heapVerifyAgainstFullWalk
	heapVerifyAgainstFullWalk = false
	defer func() { heapVerifyAgainstFullWalk = previous }()
	for _, n := range []int{1000, 4000, 8000} {
		t.Logf("rows=%d time=%v", n, runStrictListSource(t, strictListAppendModSource(n), n))
	}
}
