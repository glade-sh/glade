package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) deliverPlatformEventSubscribers(records []storage.Record, result *Result) error {
	if _, err := vm.runTriggers(triggerTimingAfter, "insert", records, nil, result); err != nil {
		return err
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	return engine.ApplyPlatformEventFlows(records)
}

// Captured event Flow inputs bind $Record directly. Source-class wrappers and
// other input shapes retain the shared named-field binding path.
func (vm *VM) platformEventFlowActionInput(action storage.FlowAction, record storage.Record, method Method, elementType string) (Value, bool) {
	if !vm.hasPlatformEventMetadata(record.Object) || len(method.Params) != 1 || len(action.Inputs) != 1 {
		return Value{}, false
	}
	if _, sourceClass := vm.lookupClass(elementType); sourceClass {
		return Value{}, false
	}
	objectName, ok := vm.resolveObjectName(elementType)
	if !ok || !strings.EqualFold(objectName, record.Object) {
		return Value{}, false
	}
	input := action.Inputs[0]
	if !strings.EqualFold(strings.TrimSpace(input.Name), method.Params[0].Name) || !strings.EqualFold(strings.TrimSpace(input.SourceField), "$Record") {
		return Value{}, false
	}
	return vm.vmValueFromRecord(record), true
}
