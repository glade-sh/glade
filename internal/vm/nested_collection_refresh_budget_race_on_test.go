//go:build race

package vm

// Race instrumentation can increase allocations; normal builds enforce the
// numeric budget while race builds still check the result and log the count.
const nestedCollectionRefreshBudgetEnforced = false
