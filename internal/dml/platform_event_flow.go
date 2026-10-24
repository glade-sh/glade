package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ApplyPlatformEventFlows runs subscribers on delivered event payloads, which
// are transient records rather than rows inserted into object storage.
func (e *Engine) ApplyPlatformEventFlows(records []storage.Record) error {
	if e == nil || e.Org == nil {
		return nil
	}
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
			branch, matched := e.selectFlowBranch(rule, *record, definition)
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
		if _, err := e.applyFlowEffects(rule.Name, objectName, record, definition, steps, updates, actions, lookups, creates, rule.TextTemplates, rule.Variables); err != nil {
			return err
		}
	}
	return nil
}

func flowTriggeredStepsSafe(rule storage.FlowRule, steps []storage.FlowStep, label string) bool {
	if label != "platform-event" {
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
