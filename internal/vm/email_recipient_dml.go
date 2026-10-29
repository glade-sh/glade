package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// API 67 named methods reject non-null ToIds lists before inserting
// EmailMessage; direct anonymous execution accepts them. APIs 62-66 accept
// them, and null and omitted values are accepted at both bounds.
func (vm *VM) emailRecipientInsertFailures(records []storage.Record) []dml.Result {
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 67 || (vm.currentClass == "" && vm.currentMethod.ClassName == "" && !vm.currentTrigger && vm.testContext == nil) {
		return nil
	}
	var failures []dml.Result
	for i, record := range records {
		if record.ID != "" || !strings.EqualFold(record.Object, "EmailMessage") {
			continue
		}
		recipients, present := record.GetField("ToIds")
		if !present || recipients.Kind != storage.ValueList {
			continue
		}
		if failures == nil {
			failures = make([]dml.Result, len(records))
			for j := range failures {
				failures[j].Success = true
			}
		}
		failures[i] = dmlFailure(record.ID,
			"invalid junction id-list: Must use a String[] or List<String> value to update ToIds.",
			"FIELD_INTEGRITY_EXCEPTION", []string{"ToIds"})
	}
	return failures
}
