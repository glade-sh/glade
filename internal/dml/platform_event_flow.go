package dml

import (
	"errors"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ApplyPlatformEventFlows runs subscribers on delivered event payloads, which
// are transient records rather than rows inserted into object storage.
func (e *Engine) ApplyPlatformEventFlows(records []storage.Record) error {
	if e == nil || e.Org == nil {
		return nil
	}
	// Each delivery group runs on its own engine. Keep the identities this one
	// issued, or the next group reissues them (native PE08G has two groups).
	defer func() { e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences) }()
	for _, payload := range records {
		objectName, ok := storage.ResolveObjectName(*e.Org, payload.Object)
		if !ok {
			continue
		}
		behavior := e.Org.Objects[objectName].Definition.Metadata["publishBehavior"]
		if !strings.EqualFold(behavior, "PublishAfterCommit") && !strings.EqualFold(behavior, "PublishImmediately") {
			continue
		}
		record := payload.Clone()
		record.Object = objectName
		if err := e.applyPlatformEventFlowEffects(objectName, &record); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) applyPlatformEventFlowEffects(objectName string, record *storage.Record) error {
	definition := e.Org.Objects[objectName].Definition
	for _, rule := range sortedFlowRules(definition.FlowRules) {
		if !rule.Active || !strings.EqualFold(strings.TrimSpace(rule.TriggerType), "PlatformEvent") {
			continue
		}
		matches, modeled := e.evaluateFlowRule(rule, *record, definition)
		e.traceAutomation("apex.flow.rule", map[string]any{
			"flow": rule.Name, "object": objectName, "record": string(record.ID),
			"matched": modeled && matches, "modeled": modeled,
		})
		if !modeled || !matches {
			continue
		}
		steps, updates, actions, lookups, creates := rule.Steps, rule.FieldUpdates, rule.Actions, rule.RecordLookups, rule.RecordCreates
		if len(rule.Branches) > 0 {
			branch, matched, branchErr := e.selectFlowBranch(rule, *record, definition)
			if branchErr != nil {
				return branchErr
			}
			e.traceAutomation("apex.flow.decision", map[string]any{
				"flow": rule.Name, "object": objectName, "record": string(record.ID),
				"branch": branch.Name, "default": branch.Default, "matched": matched,
			})
			if !matched {
				continue
			}
			steps, updates, actions, creates = branch.Steps, branch.FieldUpdates, branch.Actions, branch.RecordCreates
			lookups = append(append([]storage.FlowRecordLookup(nil), lookups...), branch.RecordLookups...)
		}
		if !flowTriggeredStepsSafe(rule, steps, "platform-event") {
			return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: platform-event flow %s contains unsupported side-effect steps", rule.Name)
		}
		// Event payloads are transient. Execute the ordinary side effects without
		// writing the interview's $Record back into object storage.
		if err := e.runPlatformEventInterview(rule.Name, func() error {
			_, err := e.applyFlowEffects(rule.Name, objectName, record, definition, steps, updates, actions, lookups, creates, rule.TextTemplates, rule.Variables)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// runPlatformEventInterview runs one interview as its own transaction. Native
// rows (platform_event_flow_elements/extended PE06C, PE06D, PE06F; APIs 62.0 and
// 67.0, anonymous, deliver and stopTest routes): a fault that no fault
// connector handles rolls back everything the interview wrote, and neither the
// publisher nor the delivery call sees an error. Event records the interview
// creates publish only when it finishes (PE09A to PE09D).
func (e *Engine) runPlatformEventInterview(flowName string, run func() error) error {
	previousActive, previousPending := e.platformEventInterview, e.pendingInterviewEvents
	e.platformEventInterview, e.pendingInterviewEvents = true, nil
	defer func() { e.platformEventInterview, e.pendingInterviewEvents = previousActive, previousPending }()
	var rollback func() error
	if e.BeginPlatformEventInterview != nil {
		rollback = e.BeginPlatformEventInterview()
	}
	err := run()
	if err != nil {
		var fault *flowElementFault
		if rollback == nil || !errors.As(err, &fault) {
			return err
		}
		if rbErr := rollback(); rbErr != nil {
			return rbErr
		}
		e.traceAutomation("apex.flow.unhandled_fault", map[string]any{"flow": flowName, "error": err.Error()})
		return nil
	}
	if events := e.pendingInterviewEvents; len(events) > 0 && e.PlatformEventPublisher != nil {
		e.pendingInterviewEvents = nil
		return e.PlatformEventPublisher(events)
	}
	return nil
}

// flowElementFault marks the failure of an element that a fault connector can
// catch: an Apex action that threw, or a Create Records that the save rejected.
// message is the $Flow.FaultMessage text.
type flowElementFault struct {
	err     error
	message string
}

func (f *flowElementFault) Error() string { return f.err.Error() }
func (f *flowElementFault) Unwrap() error { return f.err }

// flowFaultMessage returns the $Flow.FaultMessage text of a failed element.
func flowFaultMessage(err error) string {
	var fault *flowElementFault
	if errors.As(err, &fault) && fault.message != "" {
		return fault.message
	}
	return err.Error()
}

// publishFlowEvent holds an event record that an event-triggered interview
// created until the interview finishes. It reports false for any other flow.
func (e *Engine) publishFlowEvent(objectName string, record storage.Record) bool {
	if !e.platformEventInterview || e.PlatformEventPublisher == nil || e.Org == nil {
		return false
	}
	behavior := e.Org.Objects[objectName].Definition.Metadata["publishBehavior"]
	if !strings.EqualFold(behavior, "PublishAfterCommit") && !strings.EqualFold(behavior, "PublishImmediately") {
		return false
	}
	e.pendingInterviewEvents = append(e.pendingInterviewEvents, record.Clone())
	return true
}

func flowTriggeredStepsSafe(rule storage.FlowRule, steps []storage.FlowStep, label string) bool {
	if label != "platform-event" {
		if label == "before-save" && !flowBeforeSaveAssignedRecordLoopsSafe(rule, steps) {
			return false
		}
		return flowCoreTriggeredStepsSafe(rule, steps, label)
	}
	if len(steps) == 0 {
		return false
	}
	return flowStepsSatisfyWithLoopBodies(steps, rule.StepTargets, func(step storage.FlowStep, _ bool) bool {
		switch step.Kind {
		case "recordLookup", "assignment", "recordUpdate", "recordCreate", "recordDelete", "decision", "loop":
			return true
		case "action":
			return strings.EqualFold(step.Action.ActionType, "apex") || strings.EqualFold(step.Action.ActionType, "chatterPost")
		default:
			return false
		}
	}, true)
}

// flowActionError reports a failed Apex action. Inside an event-triggered
// interview a thrown exception is a fault the interview can handle or die on.
func (e *Engine) flowActionError(name string, err error) error {
	if !e.platformEventInterview || e.FlowFaultDetail == nil {
		return flowActionFailure(name, err)
	}
	return flowActionFailureWithDetail(name, err, e.FlowFaultDetail(err))
}

// flowCreateError reports a Create Records element that the save rejected.
func (e *Engine) flowCreateError(failure, cause error) error {
	if !e.platformEventInterview {
		return failure
	}
	var rejected dmlError
	if !errors.As(cause, &rejected) {
		return failure
	}
	// Native PE06K (anonymous, deliver and stopTest, APIs 62.0 and 67.0), where
	// the save failed with STRING_TOO_LONG. Other status codes reuse the same
	// frame with their own code and message, which no row has captured.
	message := "This error occurred when the flow tried to create records: " + rejected.code + ": " + rejected.message +
		". You can look up ExceptionCode values in the <a href='https://developer.salesforce.com/docs/atlas.en-us.api.meta/api/sforce_api_calls_concepts_core_data_objects.htm#'>SOAP API Developer Guide</a>."
	return &flowElementFault{err: failure, message: message}
}

// shareIDSequences lets an Apex action and this engine issue identities from
// one sequence while an event-triggered interview runs: the action inserts
// through its own engine (native PE06I inserts after the interview's own
// Create Records element, APIs 62.0 and 67.0).
func (e *Engine) shareIDSequences(invoke func() error) error {
	if !e.platformEventInterview || e.Org == nil {
		return invoke()
	}
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	err := invoke()
	e.IDs.Sequences = maxSequences(e.IDs.Sequences, e.Org.IDSequences)
	return err
}
