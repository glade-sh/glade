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
