package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// API59/60 helpers called by owned named tests enqueue this single empty
// standard-event list. API61 and other overload/type/field shapes were not
// captured; they retain the existing publication path.
func (vm *VM) capturedLegacyStandardEventPublish(args []Value) bool {
	if vm == nil || vm.Org == nil || vm.testContext == nil || !vm.testContext.Started || vm.testContext.Stopped || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || vm.currentTrigger || vm.testContext.RunAsDepth != 0 || len(args) != 1 {
		return false
	}
	major, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok || major != 59 && major != 60 {
		return false
	}
	input := args[0]
	if input.Kind != ValueList || len(input.List) != 1 {
		return false
	}
	element, ok := collectionElementType(runtimeObjectType(input))
	if !ok || !strings.EqualFold(element, "SObject") {
		return false
	}
	record := input.List[0]
	if record.Kind != ValueObject || record.classInstance || record.projectClass || !strings.EqualFold(record.Type, "BatchApexErrorEvent") {
		return false
	}
	if _, published := record.Fields[sobjectPublishedEventIDField]; published {
		return false
	}
	for field := range record.Fields {
		if !isInternalSObjectField(field) && !isDefaultedSObjectField(record, field) {
			return false
		}
	}
	return true
}

// The captured SaveResult exposes an opaque acknowledgment, independently of
// the queued event's storage identity. Its org-specific digits are not an oracle.
func legacyStandardEventAcknowledgmentID(storedID storage.ID) storage.ID {
	text := string(storedID)
	if len(text) < 15 {
		return storedID
	}
	return storage.ID(apexIDTo18("1QQ" + text[3:15]))
}
