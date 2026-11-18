package dml

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// A Process Builder process (processType Workflow) runs as a bulk interview,
// after workflow rules and before record-triggered after-save flows. Native
// process_builder_shapes rows, API 62.0 and 67.0:
//   - interviews stop at their next record update or action; interviews stopped
//     at one element run together, in the order of the first record to reach it;
//   - each element is its own save, so a record update fires update triggers at
//     once and a later element reads the record as stored after it;
//   - a save of a record of the cohort from inside the run fires its triggers
//     only (Engine callers keep the cohort in the guard described on the VM).

// ProcessFailure is a process element's failure. Failed names the triggering
// records whose interviews reached the element, so partial save can retry the
// other rows.
type ProcessFailure struct {
	Result Result
	Failed map[storage.ID]bool
	// RowResults holds distinct errors when captured interviews fail together.
	RowResults map[storage.ID]Result
}

type capturedCaseMissingOwnerError struct{}

func (*capturedCaseMissingOwnerError) Error() string { return "Owner ID: owner cannot be blank" }

func (f *ProcessFailure) Error() string { return f.Result.Error }

// Unwrap gives callers without row results the same code and message.
func (f *ProcessFailure) Unwrap() error {
	return dmlErrorf(f.Result.StatusCode, nil, "%s", f.Result.Error)
}

// HasActiveProcess reports whether objectName has an active process.
func (e *Engine) HasActiveProcess(objectName string) bool {
	if e == nil || e.Org == nil {
		return false
	}
	canonical, ok := storage.ResolveObjectName(*e.Org, objectName)
	if !ok {
		return false
	}
	for _, rule := range e.Org.Objects[canonical].Definition.FlowRules {
		if flowRuleIsProcess(rule) {
			return true
		}
	}
	return false
}

func flowRuleIsProcess(rule storage.FlowRule) bool {
	return rule.Active && strings.EqualFold(strings.TrimSpace(rule.ProcessType), "Workflow") && strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordAfterSave")
}

type processCursor struct {
	steps    []storage.FlowStep
	position int
}

type processInterview struct {
	record           storage.Record
	capturedSnapshot storage.Record
	frame            *flowFrame
	stack            []processCursor
	pending          storage.FlowStep
	waiting          bool
}

// CreatedAutomationIDs returns the records whose pending creation event belongs
// to this cohort. RestoreCreatedAutomation hands them back to the next phase.
func (e *Engine) CreatedAutomationIDs(ids []storage.ID) map[storage.ID]bool {
	created := make(map[storage.ID]bool)
	for _, id := range ids {
		if e.pendingCreatedAutomation[id] {
			created[id] = true
		}
	}
	return created
}

func (e *Engine) RestoreCreatedAutomation(created map[storage.ID]bool) {
	for id := range created {
		if e.pendingCreatedAutomation == nil {
			e.pendingCreatedAutomation = make(map[storage.ID]bool, len(created))
		}
		e.pendingCreatedAutomation[id] = true
	}
}

// ApplyWorkflowPhase runs assignment and workflow rules, the phases native
// orders ahead of a process.
func (e *Engine) ApplyWorkflowPhase(objectName string, id storage.ID) (bool, error) {
	restore := e.beginCreatedAutomation(id)
	defer restore()
	defer e.syncOrgIDSequences()
	if err := e.ApplyAssignmentRules(storage.Record{Object: objectName, ID: id}); err != nil {
		return false, err
	}
	assignment := e.Options.AssignmentRule
	e.Options.AssignmentRule = nil
	defer func() { e.Options.AssignmentRule = assignment }()
	before := e.storedRecordFields(objectName, id)
	updated, err := e.applyWorkflowFieldUpdates(objectName, id)
	if err != nil || !updated {
		return false, err
	}
	// A field update that leaves every value as it was refires no trigger
	// (ord, seq upd rows).
	return !reflect.DeepEqual(before, e.storedRecordFields(objectName, id)), nil
}

func (e *Engine) storedRecordFields(objectName string, id storage.ID) map[string]storage.Value {
	_, stored, found := storage.LookupRecordByID(e.Org.Objects[objectName].Records, id)
	if !found {
		return nil
	}
	return stored.Clone().Fields
}

// syncOrgIDSequences keeps ids this engine issued from being issued again by the
// next DML's engine, as ApplyAutomation does.
func (e *Engine) syncOrgIDSequences() {
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
}

// ApplyAfterSaveFlowPhase runs the record-triggered after-save flows of a
// cohort that has finished its processes. A flow whose first effect is its only
// Apex action invokes it once for the cohort, on insert, update, upsert and
// partial save (ordbulk, ordsaves rows). after sees each record's outcome.
func (e *Engine) ApplyAfterSaveFlowPhase(objectName string, records []storage.Record, after func(int, AutomationResult) error) error {
	defer e.syncOrgIDSequences()
	object := e.Org.Objects[objectName]
	var rules []storage.FlowRule
	for _, rule := range sortedFlowRules(object.Definition.FlowRules) {
		trigger := strings.TrimSpace(rule.TriggerType)
		if rule.Active && !flowRuleIsProcess(rule) && (trigger == "" || strings.EqualFold(trigger, "RecordAfterSave")) {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil
	}
	if len(rules) == 4 && e.FlowActionBatchInvoker != nil && e.ProcessRecordUpdater != nil && e.ProcessRecordCreator != nil {
		if plan, ok := storage.CapturedBranchedAfterSavePlan(rules, object.Definition, branchedDefinitions(e.Org)); ok {
			snapshots := make([]storage.Record, len(records))
			for i, record := range records {
				if _, stored, found := storage.LookupRecordByID(object.Records, record.ID); found {
					snapshots[i] = stored.Clone()
					snapshots[i].Object = objectName
					if snapshots[i].System.OwnerID != "" {
						if snapshots[i].Fields == nil {
							snapshots[i].Fields = make(map[string]storage.Value)
						}
						snapshots[i].Fields["OwnerId"] = storage.IDValue(snapshots[i].System.OwnerID)
					}
				}
			}
			oldMixed, oldBranch, oldReplacement := e.capturedMixedAfterSave, e.capturedBranchedAfterSave, e.capturedShareReplacement
			e.capturedMixedAfterSave, e.capturedBranchedAfterSave = true, true
			e.capturedShareReplacement = len(plan[0].Steps[0].Branches[0].Steps) == 2
			defer func() {
				e.capturedMixedAfterSave, e.capturedBranchedAfterSave, e.capturedShareReplacement = oldMixed, oldBranch, oldReplacement
			}()
			created := e.CreatedAutomationIDs(recordIDs(snapshots))
			defer e.RestoreCreatedAutomation(created)
			dateFirst := len(plan[2].Branches) == 5 && len(plan[2].Branches[0].Steps) == 1 && len(plan[2].Branches[0].Steps[0].FieldUpdates) == 1 && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(plan[2].Branches[0].Steps[0].FieldUpdates[0].Formula)), "DATEVALUE(")
			var firstFailure error
			for i, rule := range plan {
				e.RestoreCreatedAutomation(created)
				if err := e.runProcess(rule, objectName, snapshots); err != nil {
					if !dateFirst || i != 0 {
						return err
					}
					if firstFailure == nil {
						firstFailure = err
					}
				}
			}
			return firstFailure
		}
	}
	if plan, ok := storage.CapturedCaseAfterSavePlan(rules, object.Definition); ok && e.FlowActionBatchInvoker != nil && e.ProcessRecordUpdater != nil {
		snapshots := make([]storage.Record, len(records))
		for i, record := range records {
			if _, stored, found := storage.LookupRecordByID(object.Records, record.ID); found {
				snapshots[i] = stored.Clone()
				snapshots[i].Object = objectName
			}
		}
		oldMixed, oldCase := e.capturedMixedAfterSave, e.capturedCaseAfterSave
		e.capturedMixedAfterSave, e.capturedCaseAfterSave = true, true
		defer func() { e.capturedMixedAfterSave, e.capturedCaseAfterSave = oldMixed, oldCase }()
		created := e.CreatedAutomationIDs(recordIDs(snapshots))
		defer e.RestoreCreatedAutomation(created)
		for _, rule := range plan {
			e.RestoreCreatedAutomation(created)
			if err := e.runProcess(rule, objectName, snapshots); err != nil {
				return err
			}
		}
		return nil
	}
	if len(rules) == 4 && e.FlowActionBatchInvoker != nil && e.ProcessRecordUpdater != nil && e.ProcessRecordCreator != nil {
		definitions := make(map[string]storage.ObjectDefinition, len(e.Org.Objects))
		for name, object := range e.Org.Objects {
			definitions[name] = object.Definition
		}
		if plan, ok := storage.CapturedMixedAfterSavePlan(rules, object.Definition, definitions); ok {
			action := plan[3].Steps[0].Branches[0].Steps[0].Action
			if len(action.Inputs) == 3 {
				snapshots := make([]storage.Record, len(records))
				for i, record := range records {
					if _, stored, found := storage.LookupRecordByID(object.Records, record.ID); found {
						snapshots[i] = stored.Clone()
						snapshots[i].Object = objectName
					}
				}
				prior := e.capturedMixedAfterSave
				e.capturedMixedAfterSave = true
				defer func() { e.capturedMixedAfterSave = prior }()
				created := e.CreatedAutomationIDs(recordIDs(snapshots))
				defer e.RestoreCreatedAutomation(created)
				for _, rule := range plan {
					e.RestoreCreatedAutomation(created)
					if err := e.runProcess(rule, objectName, snapshots); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}
	if len(rules) == 4 && e.FlowActionBatchInvoker != nil && e.ProcessRecordUpdater != nil && e.ProcessRecordCreator != nil {
		definitions := make(map[string]storage.ObjectDefinition, len(e.Org.Objects))
		for name, object := range e.Org.Objects {
			definitions[name] = object.Definition
		}
		if plan, ok := storage.CapturedFourAfterSavePlan(rules, object.Definition, definitions); ok {
			for _, rule := range plan {
				if err := e.runProcess(rule, objectName, records); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if len(rules) == 2 && len(records) > 0 && e.FlowActionBatchInvoker != nil {
		first, okFirst := rules[0].SimpleProcessAfterSaveAction()
		second, okSecond := rules[1].SimpleProcessAfterSaveAction()
		if okFirst && okSecond {
			for index, action := range []storage.FlowAction{first, second} {
				current := e.Org.Objects[objectName]
				previous := make([]storage.Record, len(records))
				for i, input := range records {
					_, stored, found := storage.LookupRecordByID(current.Records, input.ID)
					if !found {
						return nil
					}
					previous[i] = stored.Clone()
					previous[i].Object = objectName
				}
				if err := e.runFlowActionBatch(rules[index], action, objectName, current.Definition, records, previous, after); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if len(rules) == 1 && len(records) > 1 && e.FlowActionBatchInvoker != nil && len(rules[0].Paths) == 0 && len(rules[0].Branches) == 0 {
		if action, ok := flowActionBatchEntry(rules[0]); ok {
			previous := make([]storage.Record, len(records))
			for i, input := range records {
				_, stored, found := storage.LookupRecordByID(object.Records, input.ID)
				if !found {
					return nil
				}
				previous[i] = stored.Clone()
				previous[i].Object = objectName
			}
			return e.runFlowActionBatch(rules[0], action, objectName, object.Definition, records, previous, after)
		}
	}
	for i, input := range records {
		restore := e.beginCreatedAutomation(input.ID)
		updated, err := e.applyFlowFieldUpdates(objectName, input.ID)
		restore()
		if err != nil {
			return err
		}
		if after != nil {
			if err := after(i, AutomationResult{FlowUpdated: updated}); err != nil {
				return err
			}
		}
	}
	return nil
}

// runProcessesForRecord serves a caller without Apex trigger orchestration.
func (e *Engine) runProcessesForRecord(objectName string, id storage.ID) error {
	if id == e.activeCreatedAutomationID {
		e.RestoreCreatedAutomation(map[storage.ID]bool{id: true})
	}
	err := e.RunProcesses(objectName, []storage.Record{{Object: objectName, ID: id}})
	delete(e.pendingCreatedAutomation, id)
	return err
}

// RunProcesses runs each active process of objectName for records.
func (e *Engine) RunProcesses(objectName string, records []storage.Record) error {
	if e == nil || e.Org == nil || len(records) == 0 {
		return nil
	}
	object, ok := e.Org.Objects[objectName]
	if !ok {
		return nil
	}
	defer e.syncOrgIDSequences()
	created := e.CreatedAutomationIDs(recordIDs(records))
	for _, rule := range sortedFlowRules(object.Definition.FlowRules) {
		if !flowRuleIsProcess(rule) {
			continue
		}
		e.RestoreCreatedAutomation(created)
		if err := e.runProcess(rule, objectName, records); err != nil {
			return err
		}
	}
	e.RestoreCreatedAutomation(created)
	return nil
}

func recordIDs(records []storage.Record) []storage.ID {
	ids := make([]storage.ID, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func (e *Engine) runProcess(rule storage.FlowRule, objectName string, records []storage.Record) error {
	if e.flowDepth > 8 {
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow field update recursion limit exceeded")
	}
	e.flowDepth++
	defer func() { e.flowDepth-- }()
	definition := e.Org.Objects[objectName].Definition
	created := make(map[storage.ID]bool)
	interviews := make([]*processInterview, 0, len(records))
	outcomes := processRuleNames(rule)
	for _, input := range records {
		_, stored, found := storage.LookupRecordByID(e.Org.Objects[objectName].Records, input.ID)
		if !found || stored.System.IsDeleted {
			continue
		}
		record := stored.Clone()
		record.Object = objectName
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		if record.ExplicitNulls == nil {
			record.ExplicitNulls = make(map[string]bool)
		}
		restore := e.beginCreatedAutomation(input.ID)
		if e.activeCreatedAutomationID == input.ID {
			created[input.ID] = true
		}
		matchRecord := record
		if e.capturedMixedAfterSave && !flowRuleIsProcess(rule) {
			matchRecord = input.Clone()
		}
		matches, modeled := e.evaluateFlowRule(rule, matchRecord, definition)
		e.traceAutomation("apex.flow.rule", map[string]any{
			"flow": rule.Name, "object": objectName, "record": string(input.ID),
			"matched": modeled && matches, "modeled": modeled,
		})
		if modeled && matches {
			interview := &processInterview{record: record}
			if e.capturedMixedAfterSave && !flowRuleIsProcess(rule) {
				interview.capturedSnapshot = input.Clone()
			}
			interview.frame = e.newFlowStepFrame(rule.Name, &interview.record, definition, rule.Steps, rule.TextTemplates, rule.Variables)
			if e.capturedBranchedAfterSave {
				interview.frame.priorDecisions = true
				interview.frame.saveDecisions = true
			}
			if e.capturedMixedAfterSave && !flowRuleIsProcess(rule) {
				interview.frame.records[flowFrameKey("$Record")] = interview.capturedSnapshot.Clone()
			}
			interview.frame.processInterview = flowRuleIsProcess(rule)
			interview.frame.processOutcomes = outcomes.clone()
			for name := range interview.frame.processOutcomes {
				interview.frame.scalars[name] = storage.BooleanValue(false)
			}
			steps := rule.Steps
			if len(rule.Branches) > 0 {
				steps = []storage.FlowStep{{Kind: "decision", Branches: rule.Branches}}
			}
			interview.stack = []processCursor{{steps: steps}}
			interviews = append(interviews, interview)
		}
		restore()
	}
	for {
		var order []string
		groups := make(map[string][]*processInterview)
		for _, interview := range interviews {
			if !interview.waiting {
				if err := e.advanceProcessInterview(rule, objectName, definition, interview, created); err != nil {
					return processStepFailure(rule, err, interview.record.ID)
				}
			}
			if !interview.waiting {
				continue
			}
			key := interview.pending.Kind + "/" + flowStepDisplayName(interview.pending)
			if _, seen := groups[key]; !seen {
				order = append(order, key)
			}
			groups[key] = append(groups[key], interview)
		}
		if len(order) == 0 {
			return nil
		}
		live := interviews[:0]
		if e.capturedCaseAfterSave {
			var normal []string
			for _, key := range order {
				if !strings.HasPrefix(key, "customError/") {
					normal = append(normal, key)
				}
			}
			if len(normal) > 0 {
				order = normal
			} else {
				failed := make(map[storage.ID]bool)
				results := make(map[storage.ID]Result)
				var first Result
				for _, key := range order {
					for _, interview := range groups[key] {
						_, err := e.applyFlowStep(rule.Name, objectName, &interview.record, definition, interview.pending, interview.frame)
						if err == nil {
							continue
						}
						code := "FIELD_CUSTOM_VALIDATION_EXCEPTION"
						message := err.Error()
						row := Result{Success: false, Error: message, StatusCode: code, Errors: []Error{{Message: message, StatusCode: code}}}
						if len(results) == 0 {
							first = row
						}
						failed[interview.record.ID] = true
						results[interview.record.ID] = row
					}
				}
				if len(results) > 0 {
					return &ProcessFailure{Result: first, Failed: failed, RowResults: results}
				}
			}
		}
		for _, key := range order {
			if err := e.runProcessElement(rule, objectName, definition, groups[key]); err != nil {
				return err
			}
			for _, interview := range groups[key] {
				interview.waiting = false
			}
			// Later elements read the record as stored after this one.
			for _, interview := range interviews {
				e.refreshProcessInterview(objectName, interview)
			}
		}
		for _, interview := range interviews {
			if interview.frame != nil {
				live = append(live, interview)
			}
		}
		interviews = live
	}
}

// processOutcomeSet holds each rule of a process by key; a rule is true once its
// interview took that outcome. An isChanged criterion reads an earlier rule's
// outcome by name (chg rows).
type processOutcomeSet map[string]bool

func (s processOutcomeSet) clone() map[string]bool {
	out := make(map[string]bool, len(s))
	for name := range s {
		out[name] = false
	}
	return out
}

func processRuleNames(rule storage.FlowRule) processOutcomeSet {
	names := make(processOutcomeSet)
	var walk func([]storage.FlowStep)
	walk = func(steps []storage.FlowStep) {
		for _, step := range steps {
			for _, branch := range step.Branches {
				if !branch.Default {
					name, _, _ := strings.Cut(flowFrameKey(branch.Name), "#")
					names[name] = false
				}
				walk(branch.Steps)
			}
		}
	}
	walk(rule.Steps)
	for _, branch := range rule.Branches {
		if !branch.Default {
			name, _, _ := strings.Cut(flowFrameKey(branch.Name), "#")
			names[name] = false
		}
		walk(branch.Steps)
	}
	for _, steps := range rule.StepTargets {
		walk(steps)
	}
	return names
}

// processCriterion evaluates the operands only a process's generated decisions
// use: an earlier rule's outcome, the prior record itself and a prior field,
// whose right side may read the current record (chg rows, API 62.0 and 67.0).
func (e *Engine) processCriterion(item storage.WorkflowCriteriaItem, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (matches, supported, handled bool) {
	if frame == nil || !frame.processInterview {
		return false, false, false
	}
	field := strings.TrimSpace(item.Field)
	key := flowFrameKey(field)
	if outcome, known := frame.processOutcomes[key]; known {
		return evaluateProcessBoolean(outcome, item)
	}
	prior := frame.records[flowFrameKey("$Record__Prior")]
	if key == flowFrameKey("$Record__Prior") {
		if !strings.EqualFold(strings.TrimSpace(item.Operation), "isnull") {
			return false, false, true
		}
		return evaluateProcessBoolean(prior.ID == "", storage.WorkflowCriteriaItem{Operation: "equals", Value: item.Value})
	}
	name, isPrior := strings.CutPrefix(key, flowFrameKey("$Record__Prior")+".")
	right := strings.TrimSpace(item.Value)
	current, comparesCurrent := strings.CutPrefix(right, "{!$Record.")
	comparesCurrent = comparesCurrent && strings.HasSuffix(current, "}")
	if !isPrior && !comparesCurrent {
		return false, false, false
	}
	read := func(record storage.Record, field string) storage.Value {
		if value, ok := sourceRecordResolvedFieldValue(record, definition, e.Org.Namespace, field); ok {
			return value
		}
		return storage.NullValue()
	}
	left := read(source, field)
	if isPrior {
		left = read(prior, name)
	}
	if comparesCurrent {
		value := read(source, strings.TrimSuffix(current, "}"))
		// Two null operands are equal (cgl nullfield row).
		equal := workflowValueString(left) == workflowValueString(value) && (left.Kind == storage.ValueNull) == (value.Kind == storage.ValueNull)
		switch strings.ToLower(strings.TrimSpace(item.Operation)) {
		case "equals":
			return equal, true, true
		case "notequal", "notequals":
			return !equal, true, true
		}
		return false, false, true
	}
	matched, ok := evaluateFlowValueCriteria(left, item)
	return matched, ok, true
}

func evaluateProcessBoolean(value bool, item storage.WorkflowCriteriaItem) (bool, bool, bool) {
	want := !strings.EqualFold(strings.TrimSpace(item.Value), "false")
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "equals":
		return value == want, true, true
	case "notequal", "notequals":
		return value != want, true, true
	}
	return false, false, true
}

func (e *Engine) refreshProcessInterview(objectName string, interview *processInterview) {
	if interview.frame == nil {
		return
	}
	_, stored, found := storage.LookupRecordByID(e.Org.Objects[objectName].Records, interview.record.ID)
	if !found || stored.System.IsDeleted {
		// An action deleted the record: the interview has nothing left to read.
		interview.frame, interview.stack, interview.waiting = nil, nil, false
		return
	}
	record := stored.Clone()
	record.Object = objectName
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	interview.record = record
	if !e.capturedMixedAfterSave {
		interview.frame.records[flowFrameKey("$Record")] = record.Clone()
	}
}

// advanceProcessInterview runs decisions and other in-memory steps until the
// interview reaches an element with an effect or finishes.
func (e *Engine) advanceProcessInterview(rule storage.FlowRule, objectName string, definition storage.ObjectDefinition, interview *processInterview, created map[storage.ID]bool) error {
	if interview.frame == nil {
		return nil
	}
	active := e.activeCreatedAutomationID
	if created[interview.record.ID] {
		e.activeCreatedAutomationID = interview.record.ID
	} else {
		e.activeCreatedAutomationID = ""
	}
	defer func() { e.activeCreatedAutomationID = active }()
	for len(interview.stack) > 0 {
		top := &interview.stack[len(interview.stack)-1]
		if top.position >= len(top.steps) {
			interview.stack = interview.stack[:len(interview.stack)-1]
			continue
		}
		step := top.steps[top.position]
		top.position++
		switch step.Kind {
		case "reference":
			target, found := interview.frame.stepTargets[flowFrameKey(step.Target)]
			if !found {
				return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow %s references unknown step target %q", rule.Name, step.Target)
			}
			interview.stack = append(interview.stack, processCursor{steps: target})
		case "decision":
			decisionRecord := interview.record
			if e.capturedMixedAfterSave && !flowRuleIsProcess(rule) {
				decisionRecord = interview.capturedSnapshot
			}
			branch, matched, err := e.selectFlowStepBranch(step.Branches, decisionRecord, definition, interview.frame)
			if err != nil {
				return err
			}
			if interview.frame.processUnmodeled {
				e.traceAutomation("apex.flow.rule", map[string]any{
					"flow": rule.Name, "object": objectName, "record": string(interview.record.ID),
					"matched": false, "modeled": false,
				})
				interview.frame, interview.stack = nil, nil
				return nil
			}
			e.traceAutomation("apex.flow.decision", map[string]any{
				"flow": rule.Name, "object": objectName, "record": string(interview.record.ID),
				"branch": branch.Name, "default": branch.Default, "matched": matched,
			})
			if matched {
				if !branch.Default {
					// Custom logic compiles to alternatives named rule#n.
					name, _, _ := strings.Cut(flowFrameKey(branch.Name), "#")
					interview.frame.processOutcomes[name] = true
					interview.frame.scalars[name] = storage.BooleanValue(true)
				}
				interview.stack = append(interview.stack, processCursor{steps: branch.Steps})
			}
		case "assignment":
			if _, err := e.applyFlowStep(rule.Name, objectName, &interview.record, definition, step, interview.frame); err != nil {
				return err
			}
		default:
			interview.pending, interview.waiting = step, true
			return nil
		}
	}
	interview.frame = nil
	return nil
}

func (e *Engine) runProcessElement(rule storage.FlowRule, objectName string, definition storage.ObjectDefinition, cohort []*processInterview) error {
	step := cohort[0].pending
	failedIDs := func() []storage.ID {
		ids := make([]storage.ID, 0, len(cohort))
		for _, interview := range cohort {
			ids = append(ids, interview.record.ID)
		}
		return ids
	}
	switch {
	case e.capturedBranchedAfterSave && step.Kind == "recordDelete":
		for _, interview := range cohort {
			count, err := e.executeFlowRecordDelete(step.RecordDelete, interview.capturedSnapshot, definition, interview.frame)
			if err != nil || count == 0 {
				if err == nil {
					err = fmt.Errorf("No records in Salesforce match your delete criteria.")
				}
				return newProcessFailure(processFailureMessage(rule, err.Error()), []storage.ID{interview.record.ID})
			}
		}
		return nil
	case step.Kind == "action" && strings.EqualFold(strings.TrimSpace(step.Action.ActionType), "flow"):
		// A flow action's interviews are bulk too: each record update of the flow
		// saves the cohort at once (flowact, flow2 rows). The loader admits only a
		// flow made of record updates.
		subflow := storage.FlowSubflow{Name: step.Action.Name, FlowName: step.Action.ActionName, InputAssignments: step.Action.Inputs}
		target, updates, linear := e.subflowRecordUpdates(subflow.FlowName)
		if !linear || e.ProcessRecordUpdater == nil {
			for _, interview := range cohort {
				if err := e.executeFlowSubflow(subflow, interview.record, definition, interview.frame); err != nil {
					return processStepFailure(rule, err, interview.record.ID)
				}
			}
			return nil
		}
		frames := make(map[*processInterview]*flowFrame, len(cohort))
		for _, interview := range cohort {
			frames[interview] = e.subflowInputFrame(target, subflow, interview.record, definition, interview.frame)
		}
		recordSteps, recordShape := target.CapturedProcessRecordActionSteps()
		invokeRecordAction := func(action storage.FlowAction) error {
			if e.FlowActionBatchInvoker == nil {
				return processStepFailure(rule, fmt.Errorf("flow action batching unavailable"), failedIDs()...)
			}
			requests := make([]FlowActionRequest, 0, len(cohort))
			for _, interview := range cohort {
				requests = append(requests, FlowActionRequest{Record: interview.record.Clone(), Context: e.flowActionContextFromFrame(frames[interview])})
			}
			if err := e.FlowActionBatchInvoker(action, requests); err != nil {
				return processStepFailure(rule, flowActionFailure(action.Name, err), failedIDs()...)
			}
			return nil
		}
		if recordShape {
			if err := invokeRecordAction(recordSteps[0].Action); err != nil {
				return err
			}
			for _, interview := range cohort {
				if _, err := e.applyFlowStep(target.Name, objectName, &interview.record, definition, recordSteps[1], frames[interview]); err != nil {
					return processStepFailure(rule, err, interview.record.ID)
				}
			}
		}
		if steps, ok := target.CapturedProcessDependentLookupSteps(); ok {
			for _, prefix := range steps[:3] {
				for _, interview := range cohort {
					if _, err := e.applyFlowStep(target.Name, objectName, &interview.record, definition, prefix, frames[interview]); err != nil {
						return processStepFailure(rule, err, interview.record.ID)
					}
				}
			}
		}
		if steps, ok := target.CapturedProcessLookupSteps(); ok {
			for _, prefix := range steps[:3] {
				for _, interview := range cohort {
					if _, err := e.applyFlowStep(target.Name, objectName, &interview.record, definition, prefix, frames[interview]); err != nil {
						return processStepFailure(rule, err, interview.record.ID)
					}
				}
			}
		}
		if steps, ok := target.CapturedProcessActionSteps(); ok {
			for _, interview := range cohort {
				if _, err := e.applyFlowStep(target.Name, objectName, &interview.record, definition, steps[0], frames[interview]); err != nil {
					return processStepFailure(rule, err, interview.record.ID)
				}
			}
			for _, actionStep := range steps[1:3] {
				if e.FlowActionBatchInvoker == nil {
					return processStepFailure(rule, fmt.Errorf("flow action batching unavailable"), failedIDs()...)
				}
				requests := make([]FlowActionRequest, 0, len(cohort))
				for _, interview := range cohort {
					requests = append(requests, FlowActionRequest{Record: interview.record.Clone(), Context: e.flowActionContextFromFrame(frames[interview])})
				}
				if err := e.FlowActionBatchInvoker(actionStep.Action, requests); err != nil {
					return processStepFailure(rule, flowActionFailure(actionStep.Action.Name, err), failedIDs()...)
				}
			}
		}
		for _, update := range updates {
			if err := e.runProcessCollectedUpdate(rule, cohort, func(interview *processInterview) error {
				_, err := e.executeFlowRecordUpdate(update, interview.record, definition, frames[interview])
				return err
			}); err != nil {
				// One failed record fails every interview of the flow action, and
				// its text gains a trailing space (flow2fail rows).
				var failure *ProcessFailure
				if errors.As(err, &failure) {
					return newProcessFailure(failure.Result.Error+" ", failedIDs())
				}
				return err
			}
			for _, interview := range cohort {
				e.refreshProcessInterview(objectName, interview)
				if interview.frame == nil {
					return nil
				}
			}
		}
		if recordShape {
			if err := invokeRecordAction(recordSteps[3].Action); err != nil {
				return err
			}
		}
		return nil
	case step.Kind == "recordCreate" && !flowRuleIsProcess(rule) && e.ProcessRecordCreator != nil:
		var creates []storage.Record
		previous := e.InvocableFlowRecordCreator
		defer func() { e.InvocableFlowRecordCreator = previous }()
		e.InvocableFlowRecordCreator = func(record storage.Record) (Result, error) {
			creates = append(creates, record)
			return Result{Success: true}, nil
		}
		for _, interview := range cohort {
			if _, err := e.applyFlowStep(rule.Name, objectName, &interview.record, definition, step, interview.frame); err != nil {
				return processStepFailure(rule, err, interview.record.ID)
			}
		}
		e.InvocableFlowRecordCreator = previous
		results, err := e.ProcessRecordCreator(creates)
		if err != nil {
			return processStepFailure(rule, err, failedIDs()...)
		}
		if e.capturedShareReplacement {
			failed := map[storage.ID]bool{}
			rows := map[storage.ID]Result{}
			var first Result
			for i, result := range results {
				if result.Success || i >= len(cohort) {
					continue
				}
				id := cohort[i].record.ID
				if len(failed) == 0 {
					first = result
				}
				failed[id] = true
				rows[id] = result
			}
			if len(failed) > 0 {
				return &ProcessFailure{Result: first, Failed: failed, RowResults: rows}
			}
		}
		for _, result := range results {
			if !result.Success {
				return newProcessFailure(processFailureMessage(rule, result.Error), failedIDs())
			}
		}
		return nil
	case step.Kind == "fieldUpdate" && !flowRuleIsProcess(rule) && e.ProcessRecordUpdater != nil:
		return e.runProcessCollectedUpdate(rule, cohort, func(interview *processInterview) error {
			_, err := e.applyFlowStep(rule.Name, objectName, &interview.record, definition, step, interview.frame)
			if err != nil {
				return err
			}
			if e.capturedCaseAfterSave && interview.record.HasExplicitNull("OwnerId") {
				return &capturedCaseMissingOwnerError{}
			}
			_, err = e.InvocableFlowRecordUpdater(interview.record.Clone())
			return err
		})
	case step.Kind == "recordUpdate" && e.ProcessRecordUpdater != nil:
		return e.runProcessCollectedUpdate(rule, cohort, func(interview *processInterview) error {
			_, err := e.executeFlowRecordUpdate(step.RecordUpdate, interview.record, definition, interview.frame)
			return err
		})
	case e.capturedMixedAfterSave && step.Kind == "action" && strings.EqualFold(strings.TrimSpace(step.Action.ActionType), "emailAlert") && len(step.Action.Inputs) == 3 && step.Action.Inputs[1].Name == storage.CapturedMixedRecipientInput && step.Action.Inputs[2].Name == storage.CapturedMixedTemplateInput:
		binding := step.Action.Inputs[1]
		for _, interview := range cohort {
			record := interview.capturedSnapshot
			value, present := record.Fields[binding.SourceField]
			if !present || value.Kind == storage.ValueNull || strings.TrimSpace(workflowValueString(value)) == "" {
				message := processFailureMessage(rule, "CANNOT_EXECUTE_FLOW_TRIGGER: Probably Limit Exceeded or 0 recipients")
				if e.CapturedMixedAfterSaveErrorSuffix != nil {
					message += e.CapturedMixedAfterSaveErrorSuffix()
				}
				code := "EMAIL_NOT_PROCESSED_DUE_TO_PRIOR_ERROR"
				return &ProcessFailure{Result: Result{Success: false, Error: message, StatusCode: code, Errors: []Error{{Message: message, StatusCode: code}}}, Failed: map[storage.ID]bool{record.ID: true}}
			}
		}
		return nil
	case step.Kind == "action" && strings.EqualFold(strings.TrimSpace(step.Action.ActionType), "apex") && e.FlowActionBatchInvoker != nil:
		requests := make([]FlowActionRequest, 0, len(cohort))
		for _, interview := range cohort {
			e.traceAutomation("apex.flow.action", map[string]any{
				"flow": rule.Name, "action": step.Action.Name, "object": objectName, "record": string(interview.record.ID),
			})
			actionRecord := interview.record
			if e.capturedMixedAfterSave && !flowRuleIsProcess(rule) {
				actionRecord = interview.capturedSnapshot
			}
			requests = append(requests, FlowActionRequest{Record: actionRecord.Clone(), Context: e.flowActionContextFromFrame(interview.frame)})
		}
		if err := e.FlowActionBatchInvoker(step.Action, requests); err != nil {
			if e.ProcessActionFailure != nil {
				if detail := e.ProcessActionFailure(err); detail != "" {
					return newProcessFailure(processFailureMessage(rule, "An Apex error occurred: "+detail+" "), failedIDs())
				}
			}
			return processStepFailure(rule, flowActionFailure(step.Action.Name, err), failedIDs()...)
		}
		return nil
	}
	for _, interview := range cohort {
		if _, err := e.applyFlowStep(rule.Name, objectName, &interview.record, definition, interview.pending, interview.frame); err != nil {
			return processStepFailure(rule, err, interview.record.ID)
		}
	}
	return nil
}

// runProcessCollectedUpdate plans each interview's update, then saves the
// cohort's target records in one DML. A target reached twice is saved once.
func (e *Engine) runProcessCollectedUpdate(rule storage.FlowRule, cohort []*processInterview, plan func(*processInterview) error) error {
	var batches [][]processTarget
	updater := e.InvocableFlowRecordUpdater
	defer func() { e.InvocableFlowRecordUpdater = updater }()
	latest := make(map[storage.ID]int)
	for _, interview := range cohort {
		source := interview.record.ID
		e.InvocableFlowRecordUpdater = func(record storage.Record) (Result, error) {
			batches = placeProcessTarget(batches, latest, record, source)
			return Result{ID: record.ID, Success: true}, nil
		}
		if err := plan(interview); err != nil {
			if e.capturedCaseAfterSave {
				var missing *capturedCaseMissingOwnerError
				if errors.As(err, &missing) {
					label := strings.TrimSpace(rule.Label)
					if label == "" {
						label = rule.Name
					}
					message := fmt.Sprintf("We can't save this record because the “%s” process failed. Give your Salesforce admin these details. CANNOT_EXECUTE_FLOW_TRIGGER: This error occurred when the flow tried to update records: INVALID_CROSS_REFERENCE_KEY: %s. You can look up ExceptionCode values in the SOAP API Developer Guide.", label, missing.Error())
					if e.CapturedMixedAfterSaveErrorSuffix != nil {
						if suffix := strings.TrimSpace(e.CapturedMixedAfterSaveErrorSuffix()); suffix != "" {
							message += " " + suffix
						}
					}
					row := failedResult(source, message, "INVALID_CROSS_REFERENCE_KEY", []string{"OwnerId"})
					return &ProcessFailure{Result: row, Failed: map[storage.ID]bool{source: true}}
				}
			}
			return processStepFailure(rule, err, source)
		}
	}
	e.InvocableFlowRecordUpdater = updater
	var details []string
	var failed []storage.ID
	for _, batch := range batches {
		records := make([]storage.Record, 0, len(batch))
		for _, target := range batch {
			e.spellProcessNumbers(&target.record)
			records = append(records, target.record)
		}
		results, err := e.ProcessRecordUpdater(records)
		if err != nil {
			ids := make([]storage.ID, 0, len(cohort))
			for _, interview := range cohort {
				ids = append(ids, interview.record.ID)
			}
			return processStepFailure(rule, err, ids...)
		}
		for i, result := range results {
			if result.Success || i >= len(batch) {
				continue
			}
			failed = append(failed, batch[i].sources...)
			details = append(details, processUpdateDetail(result))
		}
	}
	if len(details) > 0 {
		return newProcessFailure(processFailureMessage(rule, strings.Join(details, "---")), failed)
	}
	return nil
}

// processTarget is one record an element saves, and the interviews that reached it.
type processTarget struct {
	record  storage.Record
	sources []storage.ID
}

// placeProcessTarget batches an element's targets (mergep and sibmerge rows). A
// target with the same values as any earlier entry saves once. A new value
// for a target saves in a later DML, after its earlier values. A new target
// joins the first DML.
func placeProcessTarget(batches [][]processTarget, latest map[storage.ID]int, record storage.Record, source storage.ID) [][]processTarget {
	at := 0
	if last, seen := latest[record.ID]; seen {
		for batch := 0; batch <= last; batch++ {
			for i := range batches[batch] {
				entry := &batches[batch][i]
				if entry.record.ID != record.ID {
					continue
				}
				if reflect.DeepEqual(entry.record.Fields, record.Fields) && reflect.DeepEqual(entry.record.ExplicitNulls, record.ExplicitNulls) {
					entry.sources = append(entry.sources, source)
					return batches
				}
			}
		}
		at = last + 1
	}
	for len(batches) <= at {
		batches = append(batches, nil)
	}
	batches[at] = append(batches[at], processTarget{record: record, sources: []storage.ID{source}})
	latest[record.ID] = at
	return batches
}

// spellProcessNumbers keeps a whole value a process writes to a Number field of
// scale 0 in the spelling native triggers read back as 5.0 (upd, evalnext rows).
// SOQL still reads it at the field's scale. Only that field shape was captured.
func (e *Engine) spellProcessNumbers(record *storage.Record) {
	object, ok := e.Org.Objects[record.Object]
	if !ok {
		return
	}
	for name, value := range record.Fields {
		field, declared := object.Definition.Fields[name]
		if !declared || field.Type != storage.FieldDecimal || field.Scale != 0 {
			continue
		}
		if display := strings.ToUpper(strings.TrimSpace(field.DisplayType)); display != "" && display != "DOUBLE" {
			continue
		}
		var text string
		switch value.Kind {
		case storage.ValueDecimal:
			text = value.Decimal
		case storage.ValueInteger:
			text = strconv.FormatInt(value.Integer, 10)
		default:
			continue
		}
		if whole, err := strconv.ParseInt(text, 10, 64); err == nil {
			record.Fields[name] = storage.DecimalValue(strconv.FormatInt(whole, 10) + ".0")
		}
	}
}

// subflowRecordUpdates returns an autolaunched flow's record updates when it has
// no other step.
func (e *Engine) subflowRecordUpdates(flowName string) (storage.FlowRule, []storage.FlowRecordUpdate, bool) {
	for _, rule := range e.Org.Metadata.Flows {
		if !rule.Active || !strings.EqualFold(rule.Name, flowName) {
			continue
		}
		updates, ok := rule.ProcessRecordUpdates()
		return rule, updates, ok
	}
	return storage.FlowRule{}, nil, false
}

func processFlowLabel(rule storage.FlowRule) string {
	if label := strings.TrimSpace(rule.Label); label != "" {
		return label
	}
	return rule.Name
}

func processFailureMessage(rule storage.FlowRule, detail string) string {
	return "We can't save this record because the “" + processFlowLabel(rule) + "” process failed. Give your Salesforce admin these details. " + detail
}

func processUpdateDetail(result Result) string {
	return "This error occurred when the flow tried to update records: " + result.StatusCode + ": " + result.Error + ". You can look up ExceptionCode values in the SOAP API Developer Guide."
}

func newProcessFailure(message string, failed []storage.ID) error {
	set := make(map[storage.ID]bool, len(failed))
	for _, id := range failed {
		set[id] = true
	}
	return &ProcessFailure{
		Result: Result{Success: false, Error: message, StatusCode: "CANNOT_EXECUTE_FLOW_TRIGGER", Errors: []Error{{Message: message, StatusCode: "CANNOT_EXECUTE_FLOW_TRIGGER"}}},
		Failed: set,
	}
}

// processStepFailure keeps other element failures in their existing form; the
// rows pin only the record-update message.
func processStepFailure(rule storage.FlowRule, err error, _ ...storage.ID) error {
	return flowCoreRecordTriggerError(rule, err)
}

func branchedDefinitions(org *storage.OrgState) map[string]storage.ObjectDefinition {
	defs := make(map[string]storage.ObjectDefinition, len(org.Objects))
	for name, object := range org.Objects {
		defs[name] = object.Definition
	}
	return defs
}
