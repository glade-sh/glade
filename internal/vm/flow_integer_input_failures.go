package vm

import "github.com/glade-sh/glade/internal/dml"

// A rejected Flow action input aborts the captured save group even for partial
// Database DML. Leave pre-existing validation failures intact.
func flowActionInputDMLResults(op string, results []dml.Result, err error) ([]dml.Result, bool) {
	failure, ok := dml.FlowActionInputFailureResult(err)
	if !ok {
		return nil, false
	}
	failures := append([]dml.Result(nil), results...)
	for i, saved := range failures {
		if !saved.Success {
			continue
		}
		row := failure
		if op == "update" || op == "upsert" && !saved.Created {
			row.ID = saved.ID
		}
		failures[i] = row
	}
	return failures, true
}

// singleFlowRecordFailureResults turns a record-triggered flow's rejected save
// (a duplicate-rule rejection or an Apex action failure; native rows in
// automation_residual_paths) into the row error of the one record that the
// statement saved. Other flow failures, and multi-row saves, keep their
// earlier error path.
func singleFlowRecordFailureResults(op string, results []dml.Result, err error) []dml.Result {
	failure, ok := dml.FlowSavedRecordFailureResult(err)
	if !ok || (op != "insert" && op != "update") || len(results) != 1 || !results[0].Success {
		return nil
	}
	if op == "update" {
		failure.ID = results[0].ID
	}
	return []dml.Result{failure}
}
