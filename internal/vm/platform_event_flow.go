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
	convertRecord := vm.recordConverterForOrg(vm.Org)
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
				payloads = append(payloads, convertRecord(record.Clone()))
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
	// Event subscribers run in their own transaction. Setup DML earlier in the
	// publisher's test must not trip mixed-DML checks on subscriber DML (native
	// event_created_by rows, EvtRunAs.deliverOutsideRunAs, APIs 62.0 and 67.0).
	restoreMixedDML := vm.beginMixedDMLTransaction()
	defer restoreMixedDML()
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
	// event_trigger_exceptions_c4 (APIs 62.0 and 67.0): the subscriber starts
	// with its own governor limits (Limits.getQueries() and getDmlStatements()
	// are 0 after the publisher's DML), and its usage does not reach the
	// publisher.
	parentLimits := vm.limits
	vm.limits = Limits{}
	defer func() { vm.limits = parentLimits }()
	vm.eventSubscriberTriggerDepth++
	_, err := vm.runTriggers(triggerTimingAfter, "insert", records, nil, result)
	vm.eventSubscriberTriggerDepth--
	if err == nil && vm.testContext != nil {
		storage.AdvanceProjectEventSubscriberTestPositions(vm.Org, records)
	}
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

// inEventSubscriberTrigger reports Apex running inside an event trigger frame
// (runPlatformEventTriggers) in a test. Native automated_process_user_mode rows
// (62.0 to 68.0): async work enqueued there, queueable or future, does not run
// during the test, including at Test.stopTest(), when the event is delivered at
// stopTest. An explicit Test.getEventBus().deliver() does run them at stopTest
// (automated_process_deliver rows), also when the deliver() call itself runs
// inside a batch execute drained by stopTest (batch_event_async
// b_execute_*_queue_dlv, API 62/67). Publish success callbacks and runAs of the
// Automated Process user are other contexts and are not covered.
func (vm *VM) inEventSubscriberTrigger() bool {
	return vm.testContext != nil && vm.eventSubscriberTriggerDepth > 0 && vm.testContext.StopTimeEventDelivery
}

// eventSubscriberRunAsUser is the Automated Process user while an event trigger
// frame runs, so jobs it enqueues keep that user when they run later.
func (vm *VM) eventSubscriberRunAsUser() Value {
	if vm.testContext == nil || vm.eventSubscriberTriggerDepth == 0 {
		return Value{}
	}
	return vm.testContext.CurrentUser
}
