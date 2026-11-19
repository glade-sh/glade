//go:build !race

package playground

// Without the race detector the 5s public product limit is the deadline
// these tests exercise, so no run-budget override applies.
const case339RaceRunBudget = 0
