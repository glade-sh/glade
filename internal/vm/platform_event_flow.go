package vm

import (
	"errors"
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
	var payloads, afterCommitPayloads []Value
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
				// Native PE08B, PE08E and PE08G (anonymous, APIs 62.0 and 67.0): a
				// rollback drops a publish-after-commit event's Flow interview
				// and keeps a publish-immediately one.
				if strings.EqualFold(vm.Org.Objects[name].Definition.Metadata["publishBehavior"], "PublishAfterCommit") {
					afterCommitPayloads = append(afterCommitPayloads, convertRecord(record.Clone()))
				} else {
					payloads = append(payloads, convertRecord(record.Clone()))
				}
				break
			}
		}
	}
	if len(payloads) > 0 {
		vm.enqueueAsyncJob(AsyncJob{Kind: platformEventFlowJobKind, Args: payloads})
	}
	if len(afterCommitPayloads) > 0 {
		vm.flowAsyncSeq++
		vm.enqueueAsyncJob(AsyncJob{Kind: platformEventFlowJobKind, Args: afterCommitPayloads, EventSeq: vm.flowAsyncSeq})
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
	defer vm.withInterviewJournal()()
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
	defer vm.withInterviewJournal()()
	engine := vm.newDeferredAutomationDMLEngine(result)
	return engine.ApplyPlatformEventFlows(records)
}

// withInterviewJournal makes sure DML is journaled while event-triggered
// interviews run, so a faulted interview can roll back its own writes. It
// returns the function that restores the previous journal.
func (vm *VM) withInterviewJournal() func() {
	if vm == nil || vm.Org == nil || (vm.isolationJournal != nil && vm.isolationJournal.Org() == vm.Org) {
		return func() {}
	}
	previous := vm.isolationJournal
	vm.isolationJournal = storage.NewIsolationJournal(vm.Org)
	return func() { vm.isolationJournal = previous }
}

// beginPlatformEventInterview starts the transaction of one event-triggered
// interview. The returned function rolls back its writes and side effects.
func (vm *VM) beginPlatformEventInterview() func() error {
	mark, ok := vm.beginBatchChunkTransaction()
	if !ok {
		return nil
	}
	effects := vm.snapshotSideEffects()
	return func() error {
		if err := vm.rollbackBatchChunkTransaction(mark); err != nil {
			return err
		}
		vm.restoreSideEffects(effects)
		return nil
	}
}

// flowApexFaultDetail is the $Flow.FaultMessage text of an Apex action that
// threw (native PE06B and PE06E rows, APIs 62.0 and 67.0, every route).
func (vm *VM) flowApexFaultDetail(err error) string {
	var failure *RuntimeError
	if !errors.As(err, &failure) {
		return ""
	}
	return "An Apex error occurred: " + apexFailureText(failure)
}

// publishFlowEvents publishes the events an event-triggered interview created.
func (vm *VM) publishFlowEvents(records []storage.Record, result *Result) error {
	if vm.testContext != nil {
		vm.testContext.FlowPublishedEvents = append(vm.testContext.FlowPublishedEvents, records...)
		return nil
	}
	return vm.deliverPublishedPlatformEventSubscribers(records, result)
}

// publishFlowCreatedEvents publishes the events that a record-triggered or
// autolaunched Flow created, the way Apex EventBus.publish does: a test queues
// them for its next delivery, any other run delivers them after the publisher.
func (vm *VM) publishFlowCreatedEvents(records []storage.Record, result *Result) error {
	if vm.testContext != nil && (!vm.testContext.Stopped || vm.testContext.StopDraining) {
		vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, records...)
		return nil
	}
	return vm.deliverPublishedPlatformEventSubscribers(records, result)
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
