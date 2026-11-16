package vm

import (
	"fmt"
	"testing"
	"time"
)

// strictListAppendSource builds a list of n records in one loop, the shape
// whose strict per-statement heap check once walked the whole list on every
// add.
func strictListAppendSource(n int) string {
	return fmt.Sprintf(`List<Account> l = new List<Account>(); for (Integer i = 0; i < %d; i++) { l.add(new Account(Name='a'+i)); }`, n)
}

func runStrictListAppend(tb testing.TB, n int) time.Duration {
	tb.Helper()
	program, err := CompileAnonymous(strictListAppendSource(n))
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
