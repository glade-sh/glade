package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

const platformEventFlowJobKind = "PlatformEventFlow"

// Anonymous publishers observe no Flow effects in their transaction. Keep the
// existing Apex subscriber route, and drain Flow interviews after commit.
func (vm *VM) deliverPublishedPlatformEventSubscribers(records []storage.Record, result *Result) error {
	if vm.testContext != nil {
		return vm.deliverPlatformEventSubscribers(records, result)
	}
	if err := vm.runPlatformEventTriggers(records, result); err != nil {
		return err
	}
	var payloads []Value
	for _, record := range records {
		if vm.Org == nil {
			continue
		}
		name, ok := storage.ResolveObjectName(*vm.Org, record.Object)
		if !ok {
			continue
		}
		for _, rule := range vm.Org.Objects[name].Definition.FlowRules {
			if rule.Active && strings.EqualFold(strings.TrimSpace(rule.TriggerType), "PlatformEvent") {
				payloads = append(payloads, vm.vmValueFromRecord(record.Clone()))
				break
			}
		}
	}
	if len(payloads) > 0 {
		vm.enqueueAsyncJob(AsyncJob{Kind: platformEventFlowJobKind, Args: payloads})
	}
	return nil
}

func (vm *VM) runPlatformEventFlowJob(payloads []Value, result *Result) error {
	records := make([]storage.Record, 0, len(payloads))
	for _, payload := range payloads {
		record, err := vm.recordFromValue(&payload)
		if err != nil {
			return err
		}
		records = append(records, record)
	}
	_, err := vm.withAsyncLimitWindow(func() (Value, error) {
		engine := vm.newDeferredAutomationDMLEngine(result)
		return Null, engine.ApplyPlatformEventFlows(records)
	})
	return err
}

func (vm *VM) deliverPlatformEventSubscribers(records []storage.Record, result *Result) error {
	if err := vm.runPlatformEventTriggers(records, result); err != nil {
		return err
	}
	engine := vm.newDeferredAutomationDMLEngine(result)
	return engine.ApplyPlatformEventFlows(records)
}

func (vm *VM) runPlatformEventTriggers(records []storage.Record, result *Result) error {
	// Native delivery uses Automated Process for Apex triggers, while Flow and
	// its invocable Apex actions retain the caller. Scope the switch to Apex.
	if user := vm.automatedProcessUser(); user.Kind != "" {
		previousExecutionUser := vm.executionUser
		vm.executionUser = user
		defer func() { vm.executionUser = previousExecutionUser }()
		if vm.testContext != nil {
			previousTestUser := vm.testContext.CurrentUser
			vm.testContext.CurrentUser = user
			defer func() { vm.testContext.CurrentUser = previousTestUser }()
		}
	}
	_, err := vm.runTriggers(triggerTimingAfter, "insert", records, nil, result)
	return err
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
