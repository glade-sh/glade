package scripts

import "testing"

func TestCIReceiptBudgetExclusionRequiresAllJobs(t *testing.T) {
	jobs := map[string][]string{
		"build":        nil,
		"test":         {"build"},
		"required-ci":  {"test"},
		"tree-receipt": {"required-ci"},
	}
	if !ciReceiptWaitsForJobs(jobs, "tree-receipt") {
		t.Fatal("receipt should follow all other jobs")
	}
	jobs["parallel"] = nil
	if ciReceiptWaitsForJobs(jobs, "tree-receipt") {
		t.Fatal("receipt may overlap a job outside the dependency chain")
	}
}
