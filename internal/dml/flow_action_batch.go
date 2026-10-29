package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ApplyFlowActionBatch handles the action-first Create interviews captured by
// flow_followups AO04 and AO07-AO14. Other graphs retain ordinary automation.
// Admission is read-only: a false result has consumed no creation events and
// has invoked neither Apex nor the completion callback.
func (e *Engine) ApplyFlowActionBatch(records []storage.Record, after func(int, AutomationResult) error) (bool, error) {
	if e == nil || e.Org == nil || e.FlowActionBatchInvoker == nil || len(records) < 2 || len(records) > 200 || e.Options.AssignmentRule != nil {
		return false, nil
	}
	objectName, exists := storage.ResolveObjectName(*e.Org, records[0].Object)
	if !exists {
		return false, nil
	}
	object := e.Org.Objects[objectName]
	definition := object.Definition
	for _, workflow := range definition.WorkflowRules {
		if workflow.Active {
			return false, nil
		}
	}
	var rule storage.FlowRule
	foundRule := false
	for _, candidate := range definition.FlowRules {
		if !candidate.Active {
			continue
		}
		trigger := strings.TrimSpace(candidate.TriggerType)
		if trigger == "" {
			return false, nil
		}
		if !strings.EqualFold(trigger, "RecordAfterSave") {
			continue
		}
		if foundRule || !strings.EqualFold(strings.TrimSpace(candidate.RecordTriggerType), "Create") || len(candidate.Paths) != 0 || len(candidate.Branches) != 0 {
			return false, nil
		}
		rule, foundRule = candidate, true
	}
	if !foundRule {
		return false, nil
	}
	action, supported := flowActionBatchEntry(rule)
	if !supported {
		return false, nil
	}
	previous := make([]storage.Record, len(records))
	seen := make(map[storage.ID]bool, len(records))
	for i, input := range records {
		name, found := storage.ResolveObjectName(*e.Org, input.Object)
		if !found || name != objectName || input.ID == "" || input.AssignmentRule != nil || !e.pendingCreatedAutomation[input.ID] || seen[input.ID] {
			return false, nil
		}
		_, stored, found := storage.LookupRecordByID(object.Records, input.ID)
		if !found || stored.System.IsDeleted || stored.AssignmentRule != nil {
			return false, nil
		}
		seen[input.ID] = true
		previous[i] = stored.Clone()
		previous[i].Object = objectName
	}
	if e.flowDepth > 8 {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow field update recursion limit exceeded")
	}
	e.flowDepth++
	defer func() { e.flowDepth-- }()

	frames := make([]*flowFrame, len(records))
	working := make([]storage.Record, len(records))
	requests := make([]FlowActionRequest, 0, len(records))
	for i := range records {
		working[i] = previous[i].Clone()
		if working[i].Fields == nil {
			working[i].Fields = make(map[string]storage.Value)
		}
		if working[i].ExplicitNulls == nil {
			working[i].ExplicitNulls = make(map[string]bool)
		}
		restore := e.beginCreatedAutomation(records[i].ID)
		matches, modeled := e.evaluateFlowRule(rule, working[i], definition)
		e.traceAutomation("apex.flow.rule", map[string]any{
			"flow": rule.Name, "object": objectName, "record": string(records[i].ID),
			"matched": modeled && matches, "modeled": modeled,
		})
		if modeled && matches {
			frame := e.newFlowStepFrame(rule.Name, &working[i], definition, rule.Steps, rule.TextTemplates, rule.Variables)
			prepared := &flowPreparedAction{name: flowFrameKey(action.Name)}
			frame.preparedAction = prepared
			frames[i] = frame
			context := e.flowActionContextFromFrame(frame)
			context.Output = func(value storage.Value) {
				prepared.output, prepared.hasOutput = value.Clone(), true
			}
			requests = append(requests, FlowActionRequest{Record: working[i].Clone(), Context: context})
		}
		restore()
	}
	if len(requests) > 0 {
		err := e.FlowActionBatchInvoker(action, requests)
		for _, frame := range frames {
			if frame != nil {
				frame.preparedAction.err = err
			}
		}
	}
	for i, frame := range frames {
		outcome := AutomationResult{}
		if frame != nil {
			changed, err := func() (bool, error) {
				activeCreated := e.activeCreatedAutomationID
				e.activeCreatedAutomationID = working[i].ID
				defer func() { e.activeCreatedAutomationID = activeCreated }()
				changed, err := e.applyFlowStepsWithFrame(rule.Name, objectName, &working[i], definition, rule.Steps, frame)
				if err != nil {
					return false, flowCoreRecordTriggerError(rule, err)
				}
				if changed {
					if err := e.storeFlowFieldUpdates(objectName, previous[i], working[i]); err != nil {
						return false, err
					}
				}
				return changed, nil
			}()
			if err != nil {
				return true, err
			}
			outcome.FlowUpdated = changed
		}
		e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
		if after != nil {
			// The ordinary route finishes Flow before refiring Apex triggers.
			// Nested DML in the callback therefore starts at the caller's depth.
			e.flowDepth--
			err := after(i, outcome)
			e.flowDepth++
			if err != nil {
				return true, err
			}
		}
	}
	return true, nil
}

// Inspect both success and fault graphs. Memoized summaries count repeated
// references separately while rejecting cycles and multiple action visits.
func flowActionBatchEntry(rule storage.FlowRule) (storage.FlowAction, bool) {
	type summary struct {
		first  string
		action storage.FlowAction
		count  int
	}
	active := make(map[string]bool)
	memo := make(map[string]summary)
	visitTargets := make(map[string]string)
	var walk func([]storage.FlowStep) (summary, bool)
	walk = func(steps []storage.FlowStep) (summary, bool) {
		var result summary
		for _, step := range steps {
			part := summary{first: step.Kind}
			switch step.Kind {
			case "reference":
				key := flowFrameKey(step.Target)
				target, found := rule.StepTargets[key]
				if !found || active[key] {
					return summary{}, false
				}
				if visit := flowFrameKey(step.VisitKey); visit != "" {
					if earlier, exists := visitTargets[visit]; exists && earlier != key {
						return summary{}, false
					}
					visitTargets[visit] = key
				}
				var cached bool
				part, cached = memo[key]
				if !cached {
					active[key] = true
					var ok bool
					part, ok = walk(target)
					delete(active, key)
					if !ok {
						return summary{}, false
					}
					memo[key] = part
				}
			case "action":
				if !strings.EqualFold(strings.TrimSpace(step.Action.ActionType), "apex") || flowFrameKey(step.Action.Name) == "" {
					return summary{}, false
				}
				part.action, part.count = step.Action, 1
			case "fieldUpdate", "recordUpdate", "customError":
			default:
				return summary{}, false
			}
			if result.first == "" {
				result.first = part.first
			}
			if part.count > 0 {
				result.action = part.action
			}
			result.count += part.count
			fault, ok := walk(step.FaultBranch)
			if !ok || result.count+fault.count > 1 {
				return summary{}, false
			}
			result.count += fault.count
			if fault.count > 0 {
				result.action = fault.action
			}
		}
		return result, true
	}
	result, ok := walk(rule.Steps)
	return result.action, ok && result.first == "action" && result.count == 1
}
