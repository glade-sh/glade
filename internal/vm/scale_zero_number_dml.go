package vm

import (
	"fmt"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// String-put Long inputs are rejected before insert triggers.
// Typed/constructor inputs have no dynamic
// provenance and retain their captured Decimal read and insertion paths.
func (vm *VM) scaleZeroNumberInsertFailures(records []storage.Record, targets []*Value, failures []dml.Result) []dml.Result {
	for i, record := range records {
		if record.ID != "" || i >= len(targets) || targets[i] == nil ||
			(failures != nil && failures[i].Error != "") {
			continue
		}
		target := *targets[i]
		for _, field := range explicitSObjectFieldNames(target) {
			entry, exists := scaleZeroNumberDynamicInput(target, field)
			input := entry.Fields["value"]
			if !exists || !entry.Fields["stringPut"].Bool || input.Kind != ValueInt || !isLongIntValue(input) {
				continue
			}
			definition, column, known := vm.sObjectFieldDefinition(target.Type, field)
			if !known || !scaleZeroNumberCapturedField(definition, column) {
				continue
			}
			if failures == nil {
				failures = make([]dml.Result, len(records))
				for j := range failures {
					failures[j].Success = true
				}
			}
			message := fmt.Sprintf("%s: value not of required type: %s", column.Label, input.String())
			failures[i] = dmlFailure(record.ID, message, "INVALID_TYPE_ON_FIELD_IN_RECORD", []string{column.APIName})
			break
		}
	}
	return failures
}
