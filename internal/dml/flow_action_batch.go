package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// ApplyFlowActionBatch handles the action-first Create interviews captured by
// flow_followups AO04 and AO07-AO14. Other graphs retain ordinary automation.
// Admission is read-only: a false result has consumed no creation events and
// has invoked neither Apex nor the completion callback.
func (e *Engine) ApplyFlowActionBatch(records []storage.Record, after func(int, AutomationResult) error, afterBatch ...func([]AutomationResult, storage.FlowRule) error) (bool, error) {
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
	return true, e.runFlowActionBatch(rule, action, objectName, definition, records, previous, after, afterBatch...)
}

func (e *Engine) runFlowActionBatch(rule storage.FlowRule, action storage.FlowAction, objectName string, definition storage.ObjectDefinition, records, previous []storage.Record, after func(int, AutomationResult) error, afterBatch ...func([]AutomationResult) error) error {
	if e.flowDepth > 8 {
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow field update recursion limit exceeded")
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
			if len(action.Outputs) > 0 {
				context.OutputFields = func(fields map[string]storage.Value) { prepared.fields = fields }
			}
			requests = append(requests, FlowActionRequest{Record: working[i].Clone(), Context: context})
		}
		restore()
	}
	var batchErr error
	if len(requests) > 0 {
		batchErr = e.FlowActionBatchInvoker(action, requests)
		for _, frame := range frames {
			if frame != nil {
				frame.preparedAction.err = batchErr
			}
		}
	}
	// The native refire capture covers a successful all-admitted action whose
	// output writes both Date and Datetime on the triggering records. Delay
	// trigger refiring for that shape until all of those writes are stored.
	batchedRefire := len(afterBatch) == 1 && afterBatch[0] != nil && len(requests) == len(records) && batchErr == nil && flowActionDateRefireShape(rule, definition, e.Org.Namespace) && flowActionDateRefireOutputs(frames)
	var outcomes []AutomationResult
	if batchedRefire {
		outcomes = make([]AutomationResult, len(records))
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
				// A failed batched Apex action sends every admitted interview down
				// its fault connector. Preserve the captured all-admitted Custom
				// Error shape for the statement DML exception envelope.
				if len(requests) == len(records) && batchErr != nil && flowActionBatchLiteralFault(rule) {
					if _, ok := FlowCustomErrorFailureResult(err); ok {
						return &flowActionBatchCustomError{cause: err, count: len(records)}
					}
				}
				return err
			}
			outcome.FlowUpdated = changed
		}
		e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
		if batchedRefire {
			outcomes[i] = outcome
			continue
		}
		if after != nil {
			// The ordinary route finishes Flow before refiring Apex triggers.
			// Nested DML in the callback therefore starts at the caller's depth.
			e.flowDepth--
			err := after(i, outcome)
			e.flowDepth++
			if err != nil {
				return err
			}
		}
	}
	if batchedRefire {
		e.flowDepth--
		err := afterBatch[0](outcomes, rule)
		e.flowDepth++
		if err != nil {
			return err
		}
	}
	return nil
}

// Only the captured direct action-output Date/Datetime write shape is batched;
// other graphs retain the existing per-record completion callback.
func flowActionDateRefireShape(rule storage.FlowRule, definition storage.ObjectDefinition, namespace string) bool {
	if len(rule.Criteria) != 0 || len(rule.CriteriaGroups) != 0 || strings.TrimSpace(rule.Formula) != "" {
		return false
	}
	var steps []storage.FlowStep
	active := make(map[string]bool)
	var flatten func([]storage.FlowStep) bool
	flatten = func(input []storage.FlowStep) bool {
		for _, step := range input {
			if step.Kind != "reference" {
				steps = append(steps, step)
				continue
			}
			key := flowFrameKey(step.Target)
			target, ok := rule.StepTargets[key]
			if !ok || active[key] {
				return false
			}
			active[key] = true
			ok = flatten(target)
			delete(active, key)
			if !ok {
				return false
			}
		}
		return true
	}
	if !flatten(rule.Steps) || len(steps) < 2 || steps[0].Kind != "action" {
		return false
	}
	action := steps[0].Action.Name
	date, dateTime, count := false, false, 0
	for _, step := range steps[1:] {
		if step.Kind != "fieldUpdate" || step.FaultTarget != "" || len(step.FaultBranch) != 0 {
			return false
		}
		for _, update := range step.FieldUpdates {
			if action == "" || !strings.EqualFold(update.ActionOutput, action) {
				return false
			}
			field, ok := storage.ResolveFieldName(definition, namespace, update.Field)
			if !ok {
				return false
			}
			switch definition.Fields[field].Type {
			case storage.FieldDate:
				date = true
			case storage.FieldDateTime:
				dateTime = true
			default:
				return false
			}
			count++
		}
	}
	return count == 2 && date && dateTime
}

// The trigger-refire rows use a populated Datetime result for every action
// input; null or missing outputs retain the existing completion path.
func flowActionDateRefireOutputs(frames []*flowFrame) bool {
	for _, frame := range frames {
		if frame == nil || frame.preparedAction == nil || !frame.preparedAction.hasOutput || frame.preparedAction.output.Kind != storage.ValueDateTime {
			return false
		}
	}
	return len(frames) != 0
}

// The captured batch fault uses one direct, record-independent Custom Error.
// Dynamic messages and more complex fault graphs retain their prior envelope.
func flowActionBatchLiteralFault(rule storage.FlowRule) bool {
	if len(rule.Steps) == 0 || rule.Steps[0].Kind != "action" {
		return false
	}
	fault := rule.Steps[0].FaultBranch
	if len(fault) != 1 || fault[0].Kind != "customError" || len(fault[0].CustomError.Messages) != 1 {
		return false
	}
	message := fault[0].CustomError.Messages[0]
	return message.Field == "" && !strings.Contains(message.Message, "{!")
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
