//go:build !race

package vm

// Normal builds enforce the nested collection refresh allocation budget.
const nestedCollectionRefreshBudgetEnforced = true
