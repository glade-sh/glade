package dml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type AutomationResult struct {
	WorkflowUpdated bool
	FlowUpdated     bool
}

func (e *Engine) applyWorkflowFieldUpdates(objectName string, id storage.ID) (bool, error) {
	if e.workflowDepth > 8 {
		return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: workflow field update recursion limit exceeded")
	}
	e.workflowDepth++
	defer func() {
		e.workflowDepth--
	}()

	object := e.Org.Objects[objectName]
	if len(object.Definition.WorkflowRules) == 0 {
		return false, nil
	}
	record, ok := object.Records[id]
	if !ok || record.System.IsDeleted {
		return false, nil
	}
	recordChanged := false
	sideEffect := false
	previous := record
	record = record.Clone()
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	for _, rule := range object.Definition.WorkflowRules {
		if !rule.Active {
			continue
		}
		matches, ok := evaluateWorkflowRule(rule, record, object.Definition, e.Org)
		if !ok || !matches {
			continue
		}
		for _, update := range rule.FieldUpdates {
			fieldName, ok := storage.ResolveFieldName(object.Definition, e.Org.Namespace, update.Field)
			if !ok || fieldName == "Id" {
				return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{update.Field}, "dml: workflow field update %s targets unknown or read-only field %s.%s", update.Name, objectName, update.Field)
			}
			if isCalculatedOrSummaryField(object.Definition.Fields[fieldName]) {
				return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: workflow field update %s targets calculated field %s.%s", update.Name, objectName, fieldName)
			}
			value, explicitNull, ok := workflowUpdateValue(object.Definition.Fields[fieldName], record, update, object.Definition, e.Org)
			if !ok {
				return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: workflow field update %s has unsupported value expression", update.Name)
			}
			if explicitNull {
				delete(record.Fields, fieldName)
				record.ExplicitNulls[fieldName] = true
			} else {
				record.Fields[fieldName] = value
				delete(record.ExplicitNulls, fieldName)
			}
			recordChanged = true
		}
		for _, alert := range rule.EmailAlerts {
			if e.WorkflowEmailer == nil {
				continue
			}
			if err := e.WorkflowEmailer(alert, record); err != nil {
				return false, err
			}
			sideEffect = true
		}
		for _, task := range rule.Tasks {
			if err := e.createWorkflowTask(objectName, record, task); err != nil {
				return false, err
			}
			sideEffect = true
		}
	}
	if !recordChanged {
		return sideEffect, nil
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return false, err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return false, err
	}
	priorRecord := e.priorRecordForValidation(id, previous)
	if err := e.validateValidationRules(objectName, object.Definition, record, &priorRecord, false); err != nil {
		return false, err
	}
	if err := e.validateUnique(objectName, object.Definition, record, record.ID); err != nil {
		return false, err
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
	}
	stamp := e.systemTimestamp()
	record.System.LastModifiedDate = stamp
	record.System.SystemModstamp = stamp
	record.System.LastModifiedByID = e.systemUserID()
	object.Records[id] = record
	e.Org.Objects[objectName] = object
	e.removeUniqueIndexRecord(objectName, object.Definition, previous)
	e.addUniqueIndexRecord(objectName, object.Definition, record)
	return true, nil
}

func (e *Engine) createWorkflowTask(objectName string, record storage.Record, task storage.WorkflowTask) error {
	storage.EnsureStandardObject(e.Org, "Task")
	if e.IDs.Prefixes["Task"] == "" {
		e.IDs.Prefixes["Task"] = e.Org.Objects["Task"].Definition.KeyPrefix
	}
	fields := map[string]storage.Value{
		"Subject":     storage.StringValue(task.Subject),
		"Status":      storage.StringValue(task.Status),
		"Priority":    storage.StringValue(task.Priority),
		"Description": storage.StringValue(task.Description),
	}
	if task.HasDueDateOffset {
		dueDate := e.Now().AddDate(0, 0, task.DueDateOffset).Format("2006-01-02")
		fields["ActivityDate"] = storage.DateValue(dueDate)
	}
	if task.AssignedToType != "" {
		switch {
		case strings.EqualFold(task.AssignedToType, "owner"):
			ownerID := record.System.OwnerID
			if ownerID == "" {
				ownerID = e.systemUserID()
			}
			fields["OwnerId"] = storage.IDValue(ownerID)
		case strings.EqualFold(task.AssignedToType, "creator"):
			fields["OwnerId"] = storage.IDValue(e.systemUserID())
		}
	}
	if strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead") {
		fields["WhoId"] = storage.IDValue(record.ID)
	} else {
		fields["WhatId"] = storage.IDValue(record.ID)
	}
	taskRecord := storage.Record{Object: "Task", Fields: fields}
	if _, err := e.insertOne(taskRecord, nil); err != nil {
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: workflow task %s creation failed: %v", task.Name, err)
	}
	e.traceAutomation("apex.workflow.task", map[string]any{
		"task":    task.Name,
		"object":  objectName,
		"record":  string(record.ID),
		"subject": task.Subject,
	})
	return nil
}

func (e *Engine) ApplyAutomation(objectName string, id storage.ID) (AutomationResult, error) {
	var result AutomationResult
	workflowUpdated, err := e.applyWorkflowFieldUpdates(objectName, id)
	if err != nil {
		return result, err
	}
	result.WorkflowUpdated = workflowUpdated
	flowUpdated, err := e.applyFlowFieldUpdates(objectName, id)
	if err != nil {
		return result, err
	}
	result.FlowUpdated = flowUpdated
	e.Org.IDSequences = maxSequences(e.Org.IDSequences, e.IDs.Sequences)
	return result, nil
}

func (e *Engine) ApplyBeforeSaveFlows(records []storage.Record) error {
	if e == nil || e.Org == nil {
		return nil
	}
	for i := range records {
		if err := e.applyBeforeSaveFlowFieldUpdates(&records[i]); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) applyBeforeSaveFlowFieldUpdates(record *storage.Record) error {
	if record == nil || record.Object == "" {
		return nil
	}
	objectName, ok := storage.ResolveObjectName(*e.Org, record.Object)
	if !ok {
		return nil
	}
	object := e.Org.Objects[objectName]
	if len(object.Definition.FlowRules) == 0 {
		return nil
	}
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	record.Object = objectName
	for _, rule := range object.Definition.FlowRules {
		if !rule.Active || !flowRuleRunsBeforeSave(rule) {
			continue
		}
		matches, ok := evaluateFlowRule(rule, *record, object.Definition, e.Org)
		e.traceAutomation("apex.flow.rule", map[string]any{
			"flow":    rule.Name,
			"object":  objectName,
			"record":  string(record.ID),
			"matched": ok && matches,
			"modeled": ok,
		})
		if !ok || !matches {
			continue
		}
		if len(rule.Branches) > 0 {
			branch, matched := e.selectFlowBranch(rule, *record, object.Definition)
			e.traceAutomation("apex.flow.decision", map[string]any{
				"flow":    rule.Name,
				"object":  objectName,
				"record":  string(record.ID),
				"branch":  branch.Name,
				"default": branch.Default,
				"matched": matched,
			})
			if !matched {
				continue
			}
			if !flowStepsBeforeSaveSafe(branch.Steps) {
				return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: before-save flow %s contains unsupported side-effect steps", rule.Name)
			}
			if _, err := e.applyFlowEffects(rule.Name, objectName, record, object.Definition, branch.Steps, branch.FieldUpdates, nil, nil, nil); err != nil {
				return err
			}
			continue
		}
		if !flowStepsBeforeSaveSafe(rule.Steps) {
			return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: before-save flow %s contains unsupported side-effect steps", rule.Name)
		}
		if _, err := e.applyFlowEffects(rule.Name, objectName, record, object.Definition, rule.Steps, rule.FieldUpdates, nil, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func flowStepsBeforeSaveSafe(steps []storage.FlowStep) bool {
	for _, step := range steps {
		switch step.Kind {
		case "fieldUpdate":
		case "decision":
			for _, branch := range step.Branches {
				if !flowStepsBeforeSaveSafe(branch.Steps) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func hasObjectAutomation(definition storage.ObjectDefinition) bool {
	return len(definition.WorkflowRules) > 0 || len(definition.FlowRules) > 0
}

func hasComplexActiveAutomation(definition storage.ObjectDefinition) bool {
	for _, rule := range definition.WorkflowRules {
		if !rule.Active {
			continue
		}
		// Workflow email alerts and tasks are external side effects and should use full rollback path.
		if len(rule.EmailAlerts) > 0 || len(rule.Tasks) > 0 {
			return true
		}
	}
	for _, rule := range definition.FlowRules {
		if !rule.Active {
			continue
		}
		if len(rule.Actions) > 0 || len(rule.RecordCreates) > 0 {
			return true
		}
		for _, branch := range rule.Branches {
			if len(branch.Actions) > 0 || len(branch.RecordCreates) > 0 {
				return true
			}
		}
	}
	return false
}

func objectCacheKey(objectName string) string {
	key := strings.ToLower(strings.TrimSpace(objectName))
	if key == "" {
		key = "__anon__"
	}
	return key
}

func (e *Engine) hasUniqueFields(definition storage.ObjectDefinition) bool {
	return len(e.uniqueFieldNames(definition.APIName, definition)) > 0
}

func (e *Engine) hasActiveValidationRules(definition storage.ObjectDefinition) bool {
	return len(e.activeValidationRules(definition.APIName, definition)) > 0
}

func (e *Engine) uniqueFieldNames(objectName string, definition storage.ObjectDefinition) []string {
	if e != nil {
		key := objectCacheKey(objectName)
		if cached, ok := e.uniqueFields[key]; ok {
			return cached
		}
	}
	out := make([]string, 0)
	for fieldName, field := range definition.Fields {
		if field.Unique {
			out = append(out, fieldName)
		}
	}
	if e != nil {
		if e.uniqueFieldMap == nil {
			e.uniqueFieldMap = make(map[string]bool)
		}
		key := objectCacheKey(objectName)
		e.uniqueFieldMap[key] = len(out) > 0
		if e.uniqueFields == nil {
			e.uniqueFields = make(map[string][]string)
		}
		e.uniqueFields[key] = out
	}
	return out
}

func (e *Engine) activeValidationRules(objectName string, definition storage.ObjectDefinition) []storage.ValidationRule {
	out := make([]storage.ValidationRule, 0)
	for _, rule := range definition.ValidationRules {
		if rule.Active {
			out = append(out, rule)
		}
	}
	if e != nil {
		if e.activeValRules == nil {
			e.activeValRules = make(map[string]bool)
		}
		e.activeValRules[objectCacheKey(objectName)] = len(out) > 0
	}
	return out
}

func (e *Engine) hasActiveObjectAutomationFor(objectName string, definition storage.ObjectDefinition) bool {
	if e == nil {
		return hasObjectAutomation(definition)
	}
	key := objectCacheKey(objectName)
	if e.automationRoll != nil {
		if cached, ok := e.automationRoll[key]; ok {
			return cached
		}
	} else {
		e.automationRoll = make(map[string]bool)
	}
	active := false
	for _, rule := range definition.WorkflowRules {
		if !rule.Active {
			continue
		}
		if len(rule.FieldUpdates) > 0 || len(rule.EmailAlerts) > 0 || len(rule.Tasks) > 0 {
			active = true
			break
		}
	}
	if !active {
		for _, rule := range definition.FlowRules {
			if !rule.Active {
				continue
			}
			active = true
			break
		}
	}
	e.automationRoll[key] = active
	return active
}

func (e *Engine) applyFlowFieldUpdates(objectName string, id storage.ID) (bool, error) {
	if e.flowDepth > 8 {
		return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow field update recursion limit exceeded")
	}
	e.flowDepth++
	defer func() {
		e.flowDepth--
	}()

	object := e.Org.Objects[objectName]
	if len(object.Definition.FlowRules) == 0 {
		return false, nil
	}
	record, ok := object.Records[id]
	if !ok || record.System.IsDeleted {
		return false, nil
	}
	changed := false
	previous := record
	record = record.Clone()
	if record.Fields == nil {
		record.Fields = make(map[string]storage.Value)
	}
	if record.ExplicitNulls == nil {
		record.ExplicitNulls = make(map[string]bool)
	}
	for _, rule := range object.Definition.FlowRules {
		if !rule.Active || flowRuleRunsBeforeSave(rule) {
			continue
		}
		matches, ok := evaluateFlowRule(rule, record, object.Definition, e.Org)
		e.traceAutomation("apex.flow.rule", map[string]any{
			"flow":    rule.Name,
			"object":  objectName,
			"record":  string(record.ID),
			"matched": ok && matches,
			"modeled": ok,
		})
		if !ok || !matches {
			continue
		}
		if len(rule.Branches) > 0 {
			branch, matched := e.selectFlowBranch(rule, record, object.Definition)
			e.traceAutomation("apex.flow.decision", map[string]any{
				"flow":    rule.Name,
				"object":  objectName,
				"record":  string(record.ID),
				"branch":  branch.Name,
				"default": branch.Default,
				"matched": matched,
			})
			if !matched {
				continue
			}
			branchLookups := append([]storage.FlowRecordLookup(nil), rule.RecordLookups...)
			branchLookups = append(branchLookups, branch.RecordLookups...)
			branchChanged, err := e.applyFlowEffects(rule.Name, objectName, &record, object.Definition, branch.Steps, branch.FieldUpdates, branch.Actions, branchLookups, branch.RecordCreates)
			if err != nil {
				return false, err
			}
			changed = changed || branchChanged
			continue
		}
		ruleChanged, err := e.applyFlowEffects(rule.Name, objectName, &record, object.Definition, rule.Steps, rule.FieldUpdates, rule.Actions, rule.RecordLookups, rule.RecordCreates)
		if err != nil {
			return false, err
		}
		changed = changed || ruleChanged
	}
	if !changed {
		return false, nil
	}
	if err := validateRequired(object.Definition, record); err != nil {
		return false, err
	}
	if err := e.validateReferences(object.Definition, record); err != nil {
		return false, err
	}
	priorRecord := e.priorRecordForValidation(id, previous)
	if err := e.validateValidationRules(objectName, object.Definition, record, &priorRecord, false); err != nil {
		return false, err
	}
	if err := e.validateUnique(objectName, object.Definition, record, record.ID); err != nil {
		return false, err
	}
	if _, cloned := storage.EnsureMutableObjectRecords(e.Org, objectName); cloned {
		object = e.Org.Objects[objectName]
	}
	stamp := e.systemTimestamp()
	record.System.LastModifiedDate = stamp
	record.System.SystemModstamp = stamp
	record.System.LastModifiedByID = e.systemUserID()
	object.Records[id] = record
	e.Org.Objects[objectName] = object
	e.removeUniqueIndexRecord(objectName, object.Definition, previous)
	e.addUniqueIndexRecord(objectName, object.Definition, record)
	return true, nil
}

func flowRuleRunsBeforeSave(rule storage.FlowRule) bool {
	return strings.EqualFold(strings.TrimSpace(rule.TriggerType), "RecordBeforeSave")
}

func (e *Engine) selectFlowBranch(rule storage.FlowRule, record storage.Record, definition storage.ObjectDefinition) (storage.FlowBranch, bool) {
	var defaultBranch storage.FlowBranch
	hasDefault := false
	for _, branch := range rule.Branches {
		if branch.Default {
			if !hasDefault {
				defaultBranch = branch
				hasDefault = true
			}
			continue
		}
		matches, ok := evaluateFlowBranch(branch, record, definition, e.Org.Namespace)
		if ok && matches {
			return branch, true
		}
	}
	if hasDefault {
		return defaultBranch, true
	}
	return storage.FlowBranch{}, false
}

func evaluateFlowBranch(branch storage.FlowBranch, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	if len(branch.Criteria) == 0 {
		return true, true
	}
	for _, item := range branch.Criteria {
		matches, ok := evaluateWorkflowCriteria(item, record, definition, namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func (e *Engine) applyFlowEffects(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, steps []storage.FlowStep, updates []storage.WorkflowFieldUpdate, actions []storage.FlowAction, lookups []storage.FlowRecordLookup, creates []storage.FlowRecordCreate) (bool, error) {
	changed := false
	if len(steps) > 0 {
		return e.applyFlowSteps(flowName, objectName, record, definition, steps)
	}
	for _, update := range updates {
		stepChanged, err := e.applyFlowFieldUpdate(flowName, objectName, record, definition, update)
		if err != nil {
			return changed, err
		}
		changed = changed || stepChanged
	}
	var lookupOutputs map[string]flowLookupOutput
	if len(creates) > 0 || flowActionsNeedLookupOutputs(actions) {
		var err error
		lookupOutputs, _, err = e.flowRecordLookupOutputs(lookups, *record, definition)
		if err != nil {
			return changed, err
		}
	}
	for _, action := range actions {
		e.traceAutomation("apex.flow.action", map[string]any{
			"flow":   flowName,
			"action": action.Name,
			"object": objectName,
			"record": string(record.ID),
		})
		if handled, err := e.applyBuiltinFlowAction(flowName, action, *record, definition, lookupOutputs); handled {
			if err != nil {
				return changed, err
			}
			continue
		}
		if e.FlowActionInvoker == nil {
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s requires Apex action execution support", action.Name)
		}
		if err := e.FlowActionInvoker(action, record.Clone()); err != nil {
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s failed: %v", action.Name, err)
		}
	}
	for _, create := range creates {
		createdID, err := e.executeFlowRecordCreate(create, *record, definition, lookupOutputs)
		if err != nil {
			return changed, err
		}
		e.traceAutomation("apex.flow.record_create", map[string]any{
			"flow":      flowName,
			"create":    create.Name,
			"object":    create.ObjectName,
			"sourceId":  string(record.ID),
			"createdId": string(createdID),
		})
	}
	return changed, nil
}

func (e *Engine) applyFlowSteps(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, steps []storage.FlowStep) (bool, error) {
	frame := newFlowFrame()
	return e.applyFlowStepsWithFrame(flowName, objectName, record, definition, steps, frame)
}

type flowRecordCollection struct {
	records    []storage.Record
	definition storage.ObjectDefinition
}

type flowFrame struct {
	lookupOutputs     map[string]flowLookupOutput
	lookupCollections map[string]flowRecordCollection
	matchedObjects    map[string]bool
	scalars           map[string]storage.Value
	records           map[string]storage.Record
	collections       map[string][]storage.Record
}

func newFlowFrame() *flowFrame {
	return &flowFrame{
		lookupOutputs:     make(map[string]flowLookupOutput),
		lookupCollections: make(map[string]flowRecordCollection),
		matchedObjects:    make(map[string]bool),
		scalars:           make(map[string]storage.Value),
		records:           make(map[string]storage.Record),
		collections:       make(map[string][]storage.Record),
	}
}

func (e *Engine) applyFlowStepsWithFrame(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, steps []storage.FlowStep, frame *flowFrame) (bool, error) {
	changed := false
	for _, step := range steps {
		switch step.Kind {
		case "fieldUpdate":
			for _, update := range step.FieldUpdates {
				stepChanged, err := e.applyFlowFieldUpdate(flowName, objectName, record, definition, update)
				if err != nil {
					return changed, err
				}
				changed = changed || stepChanged
			}
		case "recordLookup":
			if err := e.applyFlowRecordLookupStep(step.RecordLookup, *record, definition, frame); err != nil {
				return changed, err
			}
		case "recordCreate":
			if strings.TrimSpace(step.RecordCreate.InputReference) != "" {
				created, err := e.executeFlowRecordCreateCollection(step.RecordCreate, frame)
				if err != nil {
					return changed, err
				}
				e.traceAutomation("apex.flow.record_create", map[string]any{
					"flow":      flowName,
					"create":    step.RecordCreate.Name,
					"object":    step.RecordCreate.ObjectName,
					"sourceId":  string(record.ID),
					"createdId": "",
					"count":     created,
				})
				continue
			}
			createdID, err := e.executeFlowRecordCreate(step.RecordCreate, *record, definition, frame.lookupOutputs)
			if err != nil {
				return changed, err
			}
			frame.scalars[flowFrameKey(step.RecordCreate.Name)] = storage.IDValue(createdID)
			e.traceAutomation("apex.flow.record_create", map[string]any{
				"flow":      flowName,
				"create":    step.RecordCreate.Name,
				"object":    step.RecordCreate.ObjectName,
				"sourceId":  string(record.ID),
				"createdId": string(createdID),
			})
		case "recordUpdate":
			updated, err := e.executeFlowRecordUpdate(step.RecordUpdate, *record, definition, frame)
			if err != nil {
				return changed, err
			}
			e.traceAutomation("apex.flow.record_update", map[string]any{
				"flow":   flowName,
				"update": step.RecordUpdate.Name,
				"object": step.RecordUpdate.ObjectName,
				"record": string(record.ID),
				"count":  updated,
			})
		case "action":
			e.traceAutomation("apex.flow.action", map[string]any{
				"flow":   flowName,
				"action": step.Action.Name,
				"object": objectName,
				"record": string(record.ID),
			})
			if handled, err := e.applyBuiltinFlowAction(flowName, step.Action, *record, definition, frame.lookupOutputs); handled {
				if err != nil {
					return changed, err
				}
				continue
			}
			if e.FlowActionInvoker == nil {
				return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s requires Apex action execution support", step.Action.Name)
			}
			if err := e.FlowActionInvoker(step.Action, record.Clone()); err != nil {
				return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s failed: %v", step.Action.Name, err)
			}
		case "assignment":
			if err := e.applyFlowAssignmentStep(step.Assignment, frame); err != nil {
				return changed, err
			}
		case "loop":
			collection, ok := frame.flowCollection(step.Loop.CollectionReference)
			if !ok {
				return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow loop %s references unknown collection %s", step.Loop.Name, step.Loop.CollectionReference)
			}
			currentItem := strings.TrimSpace(step.Loop.CurrentItemReference)
			if currentItem == "" {
				currentItem = step.Loop.Name
			}
			key := flowFrameKey(currentItem)
			previous, hadPrevious := frame.records[key]
			for _, item := range collection {
				frame.records[key] = item.Clone()
				stepChanged, err := e.applyFlowStepsWithFrame(flowName, objectName, record, definition, step.Loop.Steps, frame)
				if err != nil {
					return changed, err
				}
				changed = changed || stepChanged
			}
			if hadPrevious {
				frame.records[key] = previous
			} else {
				delete(frame.records, key)
			}
		case "decision":
			branch, ok, err := e.selectFlowStepBranch(step.Branches, *record, definition, frame)
			if err != nil {
				return changed, err
			}
			if !ok {
				continue
			}
			stepChanged, err := e.applyFlowStepsWithFrame(flowName, objectName, record, definition, branch.Steps, frame)
			if err != nil {
				return changed, err
			}
			changed = changed || stepChanged
		default:
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow step kind %q is not supported", step.Kind)
		}
	}
	return changed, nil
}

func flowFrameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (f *flowFrame) flowCollection(name string) ([]storage.Record, bool) {
	key := flowFrameKey(name)
	if records, ok := f.collections[key]; ok {
		return records, true
	}
	if collection, ok := f.lookupCollections[key]; ok {
		return collection.records, true
	}
	return nil, false
}

func (e *Engine) applyFlowAssignmentStep(assignment storage.FlowAssignment, frame *flowFrame) error {
	operator := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(assignment.Operator), " ", ""))
	if operator == "" {
		operator = "assign"
	}
	target := strings.TrimSpace(assignment.Target)
	if target == "" {
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has empty target", assignment.Name)
	}
	switch operator {
	case "assigncount":
		records, ok := frame.flowCollection(assignment.SourceField)
		if !ok {
			return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s references unknown collection %s", assignment.Name, assignment.SourceField)
		}
		frame.scalars[flowFrameKey(target)] = storage.IntegerValue(int64(len(records)))
		return nil
	case "add":
		source, ok := frame.records[flowFrameKey(assignment.SourceField)]
		if !ok {
			return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s references unknown record %s", assignment.Name, assignment.SourceField)
		}
		key := flowFrameKey(target)
		frame.collections[key] = append(frame.collections[key], source.Clone())
		return nil
	case "assign", "equalto":
		dot := strings.LastIndex(target, ".")
		if dot <= 0 || dot == len(target)-1 {
			value, ok := e.flowFrameValue(assignment, frame)
			if !ok {
				return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported value", assignment.Name)
			}
			frame.scalars[flowFrameKey(target)] = value
			return nil
		}
		recordName := flowFrameKey(target[:dot])
		fieldName := strings.TrimSpace(target[dot+1:])
		value, ok := e.flowFrameValue(assignment, frame)
		if !ok {
			return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s has unsupported value", assignment.Name)
		}
		record := frame.records[recordName]
		if record.Fields == nil {
			record.Fields = make(map[string]storage.Value)
		}
		record.Fields[fieldName] = value
		frame.records[recordName] = record
		return nil
	default:
		return dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow assignment %s operator %q is not supported", assignment.Name, assignment.Operator)
	}
}

func (e *Engine) flowFrameValue(assignment storage.FlowAssignment, frame *flowFrame) (storage.Value, bool) {
	if strings.TrimSpace(assignment.SourceField) != "" {
		return e.flowFrameReferenceValue(assignment.SourceField, frame)
	}
	if strings.TrimSpace(assignment.LiteralValue) != "" {
		return storage.StringValue(assignment.LiteralValue), true
	}
	return storage.NullValue(), true
}

func (e *Engine) flowFrameReferenceValue(reference string, frame *flowFrame) (storage.Value, bool) {
	if frame == nil {
		return storage.Value{}, false
	}
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return storage.Value{}, false
	}
	if value, ok := frame.scalars[flowFrameKey(reference)]; ok {
		return value.Clone(), true
	}
	if value, explicitNull, ok := flowLookupSourceValue(reference, frame.lookupOutputs, e.Org.Namespace); ok {
		if explicitNull {
			return storage.NullValue(), true
		}
		return value, true
	}
	dot := strings.LastIndex(reference, ".")
	if dot <= 0 || dot == len(reference)-1 {
		return storage.Value{}, false
	}
	record, ok := frame.records[flowFrameKey(reference[:dot])]
	if !ok {
		return storage.Value{}, false
	}
	field := strings.TrimSpace(reference[dot+1:])
	value, ok := sourceRecordFieldValue(record, field)
	if !ok {
		return storage.NullValue(), true
	}
	return value.Clone(), true
}

func (e *Engine) executeFlowRecordCreateCollection(create storage.FlowRecordCreate, frame *flowFrame) (int, error) {
	records, ok := frame.flowCollection(create.InputReference)
	if !ok {
		return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s references unknown collection %s", create.Name, create.InputReference)
	}
	targetName, ok := storage.ResolveObjectName(*e.Org, create.ObjectName)
	if !ok {
		return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s targets unknown object %s", create.Name, create.ObjectName)
	}
	for _, source := range records {
		record := source.Clone()
		record.Object = targetName
		record.ID = ""
		if record.ExplicitNulls == nil {
			record.ExplicitNulls = make(map[string]bool)
		}
		if _, err := e.insertOne(record, nil); err != nil {
			return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s failed: %v", create.Name, err)
		}
	}
	return len(records), nil
}

func (e *Engine) executeFlowRecordUpdate(update storage.FlowRecordUpdate, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (int, error) {
	targetName, ok := storage.ResolveObjectName(*e.Org, update.ObjectName)
	if !ok {
		return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s targets unknown object %s", update.Name, update.ObjectName)
	}
	if strings.TrimSpace(update.InputReference) != "" {
		records, ok := frame.flowCollection(update.InputReference)
		if !ok {
			return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s references unknown collection %s", update.Name, update.InputReference)
		}
		count := 0
		for _, record := range records {
			out := record.Clone()
			out.Object = targetName
			if out.ID == "" {
				if id, ok := flowRecordID(out); ok {
					out.ID = id
				}
			}
			delete(out.Fields, "Id")
			if err := e.updateOne(out); err != nil {
				return count, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s failed: %v", update.Name, err)
			}
			count++
		}
		return count, nil
	}
	target, _, err := e.object(targetName)
	if err != nil {
		return 0, err
	}
	if ids, ok, err := e.flowRecordUpdateTargetIDs(update, source, sourceDefinition, frame); err != nil {
		return 0, err
	} else if ok {
		var updates []storage.Record
		for _, id := range ids {
			_, candidate, found := storage.LookupRecordByID(target.Records, id)
			if !found || candidate.System.IsDeleted {
				continue
			}
			matches, ok := e.evaluateFlowRecordUpdateCriteria(update, candidate, target.Definition, source, sourceDefinition, frame)
			if !ok {
				return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s has unsupported criteria", update.Name)
			}
			if !matches {
				continue
			}
			record, ok := e.flowRecordUpdateRecord(update, candidate.ID, targetName, target.Definition, source, sourceDefinition, frame)
			if !ok {
				return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s has unsupported assignment", update.Name)
			}
			updates = append(updates, record)
		}
		for i, record := range updates {
			if err := e.updateOne(record); err != nil {
				return i, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s failed: %v", update.Name, err)
			}
		}
		return len(updates), nil
	}
	var updates []storage.Record
	for _, candidate := range target.Records {
		if candidate.System.IsDeleted {
			continue
		}
		matches, ok := e.evaluateFlowRecordUpdateCriteria(update, candidate, target.Definition, source, sourceDefinition, frame)
		if !ok {
			return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s has unsupported criteria", update.Name)
		}
		if !matches {
			continue
		}
		record, ok := e.flowRecordUpdateRecord(update, candidate.ID, targetName, target.Definition, source, sourceDefinition, frame)
		if !ok {
			return 0, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s has unsupported assignment", update.Name)
		}
		updates = append(updates, record)
	}
	for i, record := range updates {
		if err := e.updateOne(record); err != nil {
			return i, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s failed: %v", update.Name, err)
		}
	}
	return len(updates), nil
}

func (e *Engine) flowRecordUpdateTargetIDs(update storage.FlowRecordUpdate, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) ([]storage.ID, bool, error) {
	for _, item := range update.Criteria {
		if !strings.EqualFold(strings.TrimSpace(item.Field), "Id") {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.Operation)) {
		case "", "equals", "equal", "eq":
		default:
			continue
		}
		var value storage.Value
		var ok bool
		if strings.TrimSpace(item.SourceField) != "" {
			value, ok = e.flowRecordUpdateSourceValue(item.SourceField, source, sourceDefinition, frame)
		} else {
			value = storage.IDValue(storage.ID(trimFormulaLiteral(item.Value)))
			ok = strings.TrimSpace(item.Value) != ""
		}
		if !ok {
			return nil, false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s has unsupported id criteria", update.Name)
		}
		id := storage.ID("")
		switch value.Kind {
		case storage.ValueID:
			id = value.ID
		case storage.ValueString:
			id = storage.ID(strings.TrimSpace(value.String))
		default:
			return nil, false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record update %s id criteria is not an id value", update.Name)
		}
		if id == "" {
			return nil, true, nil
		}
		return []storage.ID{id}, true, nil
	}
	return nil, false, nil
}

func flowRecordID(record storage.Record) (storage.ID, bool) {
	if record.ID != "" {
		return record.ID, true
	}
	value, ok := record.Fields["Id"]
	if !ok {
		return "", false
	}
	switch value.Kind {
	case storage.ValueID:
		return value.ID, value.ID != ""
	case storage.ValueString:
		return storage.ID(strings.TrimSpace(value.String)), strings.TrimSpace(value.String) != ""
	default:
		return "", false
	}
}

func (e *Engine) evaluateFlowRecordUpdateCriteria(update storage.FlowRecordUpdate, candidate storage.Record, targetDefinition storage.ObjectDefinition, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	for _, item := range update.Criteria {
		matches, ok := e.evaluateFlowRecordUpdateCriterion(item, candidate, targetDefinition, source, sourceDefinition, frame)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func (e *Engine) evaluateFlowRecordUpdateCriterion(item storage.WorkflowCriteriaItem, candidate storage.Record, targetDefinition storage.ObjectDefinition, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	if strings.TrimSpace(item.SourceField) == "" {
		return evaluateWorkflowCriteria(item, candidate, targetDefinition, e.Org.Namespace)
	}
	fieldName, ok := storage.ResolveFieldName(targetDefinition, e.Org.Namespace, strings.TrimSpace(item.Field))
	if !ok || fieldName == "" {
		return false, false
	}
	targetValue, ok := sourceRecordFieldValue(candidate, fieldName)
	if !ok {
		targetValue = storage.NullValue()
	}
	sourceValue, ok := e.flowRecordUpdateSourceValue(item.SourceField, source, sourceDefinition, frame)
	if !ok {
		return false, false
	}
	field := targetDefinition.Fields[fieldName]
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "", "equals", "equal", "eq":
		return storageValuesEqual(field, targetValue, sourceValue), true
	case "notequal", "not equal", "notequals", "not equals", "ne":
		return !storageValuesEqual(field, targetValue, sourceValue), true
	case "contains":
		return strings.Contains(workflowValueString(targetValue), workflowValueString(sourceValue)), true
	case "notcontain", "doesnotcontain":
		return !strings.Contains(workflowValueString(targetValue), workflowValueString(sourceValue)), true
	case "isnull", "isblank":
		return targetValue.Kind == storage.ValueNull, true
	default:
		return false, false
	}
}

func (e *Engine) flowRecordUpdateSourceValue(sourceField string, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool) {
	if value, ok := e.flowFrameReferenceValue(sourceField, frame); ok {
		return value, true
	}
	value, ok := sourceRecordResolvedFieldValue(source, sourceDefinition, e.Org.Namespace, sourceField)
	return value, ok
}

func (e *Engine) flowRecordUpdateRecord(update storage.FlowRecordUpdate, id storage.ID, objectName string, targetDefinition storage.ObjectDefinition, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (storage.Record, bool) {
	out := storage.Record{
		Object:        objectName,
		ID:            id,
		Fields:        make(map[string]storage.Value, len(update.InputAssignments)),
		ExplicitNulls: make(map[string]bool),
	}
	for _, assignment := range update.InputAssignments {
		fieldName, ok := storage.ResolveFieldName(targetDefinition, e.Org.Namespace, assignment.Field)
		if !ok || fieldName == "Id" {
			return storage.Record{}, false
		}
		field := targetDefinition.Fields[fieldName]
		if isCalculatedOrSummaryField(field) {
			return storage.Record{}, false
		}
		value, explicitNull, ok := e.flowRecordUpdateAssignmentValue(field, source, assignment, sourceDefinition, frame)
		if !ok {
			return storage.Record{}, false
		}
		if explicitNull {
			out.ExplicitNulls[fieldName] = true
			continue
		}
		out.Fields[fieldName] = value
	}
	return out, true
}

func (e *Engine) flowRecordUpdateAssignmentValue(field storage.Field, source storage.Record, assignment storage.WorkflowFieldUpdate, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (storage.Value, bool, bool) {
	if strings.TrimSpace(assignment.SourceField) != "" {
		if value, ok := e.flowRecordUpdateSourceValue(assignment.SourceField, source, sourceDefinition, frame); ok {
			if value.Kind == storage.ValueNull {
				return value, true, true
			}
			return value, false, true
		}
		return storage.Value{}, false, false
	}
	return workflowUpdateValue(field, source, assignment, sourceDefinition, e.Org)
}

func (e *Engine) selectFlowStepBranch(branches []storage.FlowBranch, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (storage.FlowBranch, bool, error) {
	var defaultBranch storage.FlowBranch
	hasDefault := false
	for _, branch := range branches {
		if branch.Default {
			defaultBranch = branch
			hasDefault = true
			continue
		}
		matches, ok := e.evaluateFlowFrameBranch(branch, source, definition, frame)
		if !ok {
			return storage.FlowBranch{}, false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow decision branch %s has unsupported criteria", branch.Name)
		}
		if matches {
			return branch, true, nil
		}
	}
	if hasDefault {
		return defaultBranch, true, nil
	}
	return storage.FlowBranch{}, false, nil
}

func (e *Engine) evaluateFlowFrameBranch(branch storage.FlowBranch, source storage.Record, definition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	if len(branch.Criteria) == 0 {
		return true, true
	}
	for _, item := range branch.Criteria {
		if value, ok := frame.scalars[flowFrameKey(item.Field)]; ok {
			matches, supported := evaluateFlowValueCriteria(value, item)
			if !supported || !matches {
				return matches, supported
			}
			continue
		}
		matches, ok := evaluateWorkflowCriteria(item, source, definition, e.Org.Namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func evaluateFlowValueCriteria(value storage.Value, item storage.WorkflowCriteriaItem) (bool, bool) {
	want := trimFormulaLiteral(item.Value)
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "", "equals", "equal", "eq":
		return workflowValueString(value) == want, true
	case "notequal", "not equal", "notequals", "not equals", "ne":
		return workflowValueString(value) != want, true
	case "greaterthan", "greater than", "gt":
		return compareFormulaValues(flowFormulaValue(value), formulaValue{kind: formulaString, text: want}, ">"), true
	case "greaterthanorequalto", "greater than or equal", "greater than or equal to", "gte", "ge":
		return compareFormulaValues(flowFormulaValue(value), formulaValue{kind: formulaString, text: want}, ">="), true
	case "lessthan", "less than", "lt":
		return compareFormulaValues(flowFormulaValue(value), formulaValue{kind: formulaString, text: want}, "<"), true
	case "lessthanorequalto", "less than or equal", "less than or equal to", "lte", "le":
		return compareFormulaValues(flowFormulaValue(value), formulaValue{kind: formulaString, text: want}, "<="), true
	case "isnull", "isblank":
		return value.Kind == storage.ValueNull || workflowValueString(value) == "", true
	default:
		return false, false
	}
}

func flowFormulaValue(value storage.Value) formulaValue {
	switch value.Kind {
	case storage.ValueInteger:
		return formulaValue{kind: formulaNumber, number: float64(value.Integer)}
	case storage.ValueDecimal:
		number, _ := strconv.ParseFloat(value.Decimal, 64)
		return formulaValue{kind: formulaNumber, number: number}
	case storage.ValueBoolean:
		if value.Boolean {
			return formulaValue{kind: formulaBool, bool: true, text: "true"}
		}
		return formulaValue{kind: formulaBool, text: "false"}
	default:
		return formulaValue{kind: formulaString, text: workflowValueString(value)}
	}
}

func (e *Engine) applyFlowFieldUpdate(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, update storage.WorkflowFieldUpdate) (bool, error) {
	fieldName, ok := storage.ResolveFieldName(definition, e.Org.Namespace, update.Field)
	if !ok || fieldName == "Id" {
		return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{update.Field}, "dml: flow field update %s targets unknown or read-only field %s.%s", update.Name, objectName, update.Field)
	}
	if isCalculatedOrSummaryField(definition.Fields[fieldName]) {
		return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow field update %s targets calculated field %s.%s", update.Name, objectName, fieldName)
	}
	value, explicitNull, ok := workflowUpdateValue(definition.Fields[fieldName], *record, update, definition, e.Org)
	if !ok {
		return false, dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow field update %s has unsupported value expression", update.Name)
	}
	if explicitNull {
		delete(record.Fields, fieldName)
		record.ExplicitNulls[fieldName] = true
	} else {
		record.Fields[fieldName] = value
		delete(record.ExplicitNulls, fieldName)
	}
	e.traceAutomation("apex.flow.field_update", map[string]any{
		"flow":         flowName,
		"update":       update.Name,
		"object":       objectName,
		"record":       string(record.ID),
		"field":        fieldName,
		"value":        workflowValueString(value),
		"explicitNull": explicitNull,
	})
	return true, nil
}

func flowActionsNeedLookupOutputs(actions []storage.FlowAction) bool {
	for _, action := range actions {
		for _, input := range action.Inputs {
			if strings.Contains(input.SourceField, ".") || strings.Contains(input.LiteralValue, "{!") {
				return true
			}
		}
	}
	return false
}

func (e *Engine) applyBuiltinFlowAction(flowName string, action storage.FlowAction, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput) (bool, error) {
	if !strings.EqualFold(action.ActionType, "chatterPost") && !strings.EqualFold(action.ActionName, "chatterPost") {
		return false, nil
	}
	inputs, ok := e.flowActionInputValues(action, source, sourceDefinition, lookupOutputs)
	if !ok {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s has unsupported chatterPost input", action.Name)
	}
	parent := inputs["subjectnameorid"]
	body := inputs["text"]
	if parent.Kind == storage.ValueNull || strings.TrimSpace(workflowValueString(parent)) == "" {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s chatterPost requires subjectNameOrId", action.Name)
	}
	if body.Kind == storage.ValueNull || strings.TrimSpace(workflowValueString(body)) == "" {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s chatterPost requires text", action.Name)
	}
	storage.EnsureStandardObject(e.Org, "FeedItem")
	if e.IDs.Prefixes["FeedItem"] == "" {
		e.IDs.Prefixes["FeedItem"] = e.Org.Objects["FeedItem"].Definition.KeyPrefix
	}
	fields := map[string]storage.Value{
		"ParentId": parent,
		"Body":     storage.StringValue(workflowValueString(body)),
		"Type":     storage.StringValue("TextPost"),
	}
	if visibility, ok := inputs["visibility"]; ok && visibility.Kind != storage.ValueNull {
		fields["Visibility"] = storage.StringValue(normalizeChatterVisibility(workflowValueString(visibility)))
	}
	feedID, err := e.insertOne(storage.Record{Object: "FeedItem", Fields: fields}, nil)
	if err != nil {
		return true, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s chatterPost failed: %v", action.Name, err)
	}
	e.traceAutomation("apex.flow.chatter_post", map[string]any{
		"flow":     flowName,
		"action":   action.Name,
		"record":   string(source.ID),
		"feedItem": string(feedID),
		"parent":   workflowValueString(parent),
	})
	return true, nil
}

func (e *Engine) flowActionInputValues(action storage.FlowAction, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput) (map[string]storage.Value, bool) {
	out := make(map[string]storage.Value, len(action.Inputs))
	for _, input := range action.Inputs {
		name := strings.ToLower(strings.TrimSpace(input.Name))
		if name == "" {
			return nil, false
		}
		value, ok := e.flowActionInputValue(input, source, sourceDefinition, lookupOutputs)
		if !ok {
			return nil, false
		}
		out[name] = value
	}
	return out, true
}

func (e *Engine) flowActionInputValue(input storage.WorkflowFieldUpdate, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput) (storage.Value, bool) {
	namespace := ""
	if e.Org != nil {
		namespace = e.Org.Namespace
	}
	if strings.TrimSpace(input.SourceField) != "" {
		if value, explicitNull, ok := flowLookupSourceValue(input.SourceField, lookupOutputs, namespace); ok {
			if explicitNull {
				return storage.NullValue(), true
			}
			return value, true
		}
		value, ok := sourceRecordResolvedFieldValue(source, sourceDefinition, namespace, input.SourceField)
		if !ok {
			return storage.Value{}, false
		}
		return value, true
	}
	if strings.TrimSpace(input.LiteralValue) != "" {
		return storage.StringValue(e.interpolateFlowString(input.LiteralValue, source, sourceDefinition, lookupOutputs)), true
	}
	return storage.NullValue(), true
}

func (e *Engine) interpolateFlowString(template string, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput) string {
	var b strings.Builder
	for len(template) > 0 {
		start := strings.Index(template, "{!")
		if start < 0 {
			b.WriteString(template)
			break
		}
		b.WriteString(template[:start])
		rest := template[start+2:]
		end := strings.Index(rest, "}")
		if end < 0 {
			b.WriteString(template[start:])
			break
		}
		ref := rest[:end]
		if value, ok := e.resolveFlowReferenceValue(ref, source, sourceDefinition, lookupOutputs); ok {
			b.WriteString(workflowValueString(value))
		} else {
			b.WriteString("{!")
			b.WriteString(ref)
			b.WriteString("}")
		}
		template = rest[end+1:]
	}
	return b.String()
}

func (e *Engine) resolveFlowReferenceValue(reference string, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput) (storage.Value, bool) {
	namespace := ""
	if e.Org != nil {
		namespace = e.Org.Namespace
	}
	if value, explicitNull, ok := flowLookupSourceValue(reference, lookupOutputs, namespace); ok {
		if explicitNull {
			return storage.NullValue(), true
		}
		return value, true
	}
	return sourceRecordResolvedFieldValue(source, sourceDefinition, namespace, reference)
}

func sourceRecordResolvedFieldValue(source storage.Record, definition storage.ObjectDefinition, namespace, sourceField string) (storage.Value, bool) {
	sourceField = strings.TrimSpace(sourceField)
	if field, ok := storage.ResolveFieldName(definition, namespace, sourceField); ok {
		value, ok := sourceRecordFieldValue(source, field)
		if !ok {
			return storage.NullValue(), true
		}
		return value.Clone(), true
	}
	if value, ok := sourceRecordRelationshipValue(source, definition, namespace, sourceField); ok {
		return value, true
	}
	return storage.Value{}, false
}

func sourceRecordRelationshipValue(source storage.Record, definition storage.ObjectDefinition, namespace, sourceField string) (storage.Value, bool) {
	dot := strings.LastIndex(sourceField, ".")
	if dot <= 0 || dot == len(sourceField)-1 {
		return storage.Value{}, false
	}
	relationship := strings.TrimSpace(sourceField[:dot])
	field := strings.TrimSpace(sourceField[dot+1:])
	if strings.EqualFold(field, "Id") {
		for apiName, fieldDef := range definition.Fields {
			if fieldDef.Type != storage.FieldReference {
				continue
			}
			if !dmlRelationshipNameMatches(namespace, storage.ParentRelationshipName(fieldDef), relationship) {
				continue
			}
			value, ok := sourceRecordFieldValue(source, apiName)
			if !ok {
				return storage.NullValue(), true
			}
			return value.Clone(), true
		}
	}
	return storage.Value{}, false
}

func normalizeChatterVisibility(visibility string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(visibility), "_", "")) {
	case "allusers":
		return "AllUsers"
	case "internalusers":
		return "InternalUsers"
	default:
		return strings.TrimSpace(visibility)
	}
}

type flowLookupOutput struct {
	record     storage.Record
	definition storage.ObjectDefinition
}

func (e *Engine) flowRecordLookupOutputs(lookups []storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (map[string]flowLookupOutput, map[string]bool, error) {
	if len(lookups) == 0 {
		return nil, nil, nil
	}
	out := make(map[string]flowLookupOutput, len(lookups))
	matchedObjects := make(map[string]bool, len(lookups))
	for _, lookup := range lookups {
		record, definition, matched, err := e.flowRecordLookupMatch(lookup, source, sourceDefinition)
		if err != nil {
			return nil, nil, err
		}
		e.traceAutomation("apex.flow.record_lookup", map[string]any{
			"lookup":  lookup.Name,
			"object":  lookup.ObjectName,
			"source":  string(source.ID),
			"matched": matched,
		})
		if matched {
			if objectName, ok := storage.ResolveObjectName(*e.Org, lookup.ObjectName); ok {
				matchedObjects[strings.ToLower(objectName)] = true
			}
		}
		if matched && lookup.StoreOutputAutomatically && lookup.GetFirstRecordOnly {
			out[strings.ToLower(strings.TrimSpace(lookup.Name))] = flowLookupOutput{record: record, definition: definition}
		}
	}
	return out, matchedObjects, nil
}

func (e *Engine) applyFlowRecordLookupStep(lookup storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) error {
	records, definition, err := e.flowRecordLookupRecords(lookup, source, sourceDefinition, frame)
	if err != nil {
		return err
	}
	matched := len(records) > 0
	e.traceAutomation("apex.flow.record_lookup", map[string]any{
		"lookup":  lookup.Name,
		"object":  lookup.ObjectName,
		"source":  string(source.ID),
		"matched": matched,
	})
	if matched && lookup.StoreOutputAutomatically && lookup.GetFirstRecordOnly {
		frame.lookupOutputs[flowFrameKey(lookup.Name)] = flowLookupOutput{record: records[0], definition: definition}
	}
	if matched && lookup.StoreOutputAutomatically && !lookup.GetFirstRecordOnly {
		frame.lookupCollections[flowFrameKey(lookup.Name)] = flowRecordCollection{records: records, definition: definition}
	}
	if matched {
		if objectName, ok := storage.ResolveObjectName(*e.Org, lookup.ObjectName); ok {
			frame.matchedObjects[strings.ToLower(objectName)] = true
		}
	}
	return nil
}

func (e *Engine) executeFlowRecordCreate(create storage.FlowRecordCreate, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput) (storage.ID, error) {
	targetName, ok := storage.ResolveObjectName(*e.Org, create.ObjectName)
	if !ok {
		return "", dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s targets unknown object %s", create.Name, create.ObjectName)
	}
	target := e.Org.Objects[targetName]
	if target.Records == nil {
		target.Records = make(map[storage.ID]storage.Record)
	}
	record := storage.Record{
		Object:        targetName,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for _, assignment := range create.InputAssignments {
		fieldName, ok := storage.ResolveFieldName(target.Definition, e.Org.Namespace, assignment.Field)
		if !ok || fieldName == "Id" {
			return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{assignment.Field}, "dml: flow record create %s targets unknown or read-only field %s.%s", create.Name, targetName, assignment.Field)
		}
		field := target.Definition.Fields[fieldName]
		if isCalculatedOrSummaryField(field) {
			return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow record create %s targets calculated field %s.%s", create.Name, targetName, fieldName)
		}
		value, explicitNull, ok := flowRecordCreateAssignmentValue(field, source, assignment, sourceDefinition, e.Org, lookupOutputs)
		if !ok {
			return "", dmlErrorf("INVALID_FIELD_FOR_INSERT_UPDATE", []string{fieldName}, "dml: flow record create %s has unsupported value expression for %s.%s", create.Name, targetName, fieldName)
		}
		if explicitNull {
			record.ExplicitNulls[fieldName] = true
			delete(record.Fields, fieldName)
		} else {
			record.Fields[fieldName] = value
			delete(record.ExplicitNulls, fieldName)
		}
	}
	createdID, err := e.insertOne(record, nil)
	if err != nil {
		return "", dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record create %s failed: %v", create.Name, err)
	}
	return createdID, nil
}

func flowRecordCreateAssignmentValue(field storage.Field, source storage.Record, assignment storage.WorkflowFieldUpdate, sourceDefinition storage.ObjectDefinition, org *storage.OrgState, lookupOutputs map[string]flowLookupOutput) (storage.Value, bool, bool) {
	namespace := ""
	if org != nil {
		namespace = org.Namespace
	}
	if assignment.SourceField != "" {
		if value, explicitNull, ok := flowLookupSourceValue(assignment.SourceField, lookupOutputs, namespace); ok {
			return value, explicitNull, true
		}
		sourceField, ok := storage.ResolveFieldName(sourceDefinition, namespace, assignment.SourceField)
		if !ok {
			return storage.Value{}, false, false
		}
		value, ok := sourceRecordFieldValue(source, sourceField)
		if !ok {
			return storage.NullValue(), true, true
		}
		return value.Clone(), false, true
	}
	return workflowUpdateValue(field, source, assignment, sourceDefinition, org)
}

func flowLookupSourceValue(sourceField string, lookupOutputs map[string]flowLookupOutput, namespace string) (storage.Value, bool, bool) {
	if len(lookupOutputs) == 0 {
		return storage.Value{}, false, false
	}
	sourceField = strings.TrimSpace(sourceField)
	dot := strings.LastIndex(sourceField, ".")
	if dot <= 0 || dot == len(sourceField)-1 {
		return storage.Value{}, false, false
	}
	output, ok := lookupOutputs[strings.ToLower(strings.TrimSpace(sourceField[:dot]))]
	if !ok {
		return storage.Value{}, false, false
	}
	fieldName, ok := storage.ResolveFieldName(output.definition, namespace, strings.TrimSpace(sourceField[dot+1:]))
	if !ok || fieldName == "" {
		return storage.Value{}, false, false
	}
	value, ok := sourceRecordFieldValue(output.record, fieldName)
	if !ok {
		return storage.NullValue(), true, true
	}
	return value.Clone(), false, true
}

func (e *Engine) flowRecordCreateSuppressedByLookup(create storage.FlowRecordCreate, matchedObjects map[string]bool) (bool, error) {
	createObject, ok := storage.ResolveObjectName(*e.Org, create.ObjectName)
	if !ok {
		return false, nil
	}
	if matchedObjects[strings.ToLower(createObject)] {
		return true, nil
	}
	return false, nil
}

func (e *Engine) traceAutomation(name string, args map[string]any) {
	if e.AutomationTracer != nil {
		e.AutomationTracer(name, args)
	}
}

func (e *Engine) flowRecordLookupMatches(lookup storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, error) {
	_, _, matches, err := e.flowRecordLookupMatch(lookup, source, sourceDefinition)
	return matches, err
}

func (e *Engine) flowRecordLookupRecords(lookup storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) ([]storage.Record, storage.ObjectDefinition, error) {
	target, targetName, err := e.object(lookup.ObjectName)
	if err != nil {
		return nil, storage.ObjectDefinition{}, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record lookup %s targets unknown object %s", lookup.Name, lookup.ObjectName)
	}
	var records []storage.Record
	for _, candidate := range target.Records {
		if candidate.System.IsDeleted {
			continue
		}
		matches := true
		for _, item := range lookup.Criteria {
			match, ok := e.evaluateFlowLookupCriteria(item, candidate, target.Definition, source, sourceDefinition, frame)
			if !ok {
				return nil, storage.ObjectDefinition{}, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", []string{item.Field}, "dml: flow record lookup %s has unsupported criteria for %s.%s", lookup.Name, targetName, item.Field)
			}
			if !match {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		records = append(records, candidate.Clone())
		if lookup.GetFirstRecordOnly {
			break
		}
	}
	return records, target.Definition, nil
}

func (e *Engine) flowRecordLookupMatch(lookup storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (storage.Record, storage.ObjectDefinition, bool, error) {
	records, definition, err := e.flowRecordLookupRecords(lookup, source, sourceDefinition, nil)
	if err != nil {
		return storage.Record{}, storage.ObjectDefinition{}, false, err
	}
	if len(records) == 0 {
		return storage.Record{}, definition, false, nil
	}
	return records[0], definition, true, nil
}

func (e *Engine) evaluateFlowLookupCriteria(item storage.WorkflowCriteriaItem, target storage.Record, targetDefinition storage.ObjectDefinition, source storage.Record, sourceDefinition storage.ObjectDefinition, frame *flowFrame) (bool, bool) {
	if strings.TrimSpace(item.SourceField) == "" {
		return evaluateWorkflowCriteria(item, target, targetDefinition, e.Org.Namespace)
	}
	targetField, ok := storage.ResolveFieldName(targetDefinition, e.Org.Namespace, strings.TrimSpace(item.Field))
	if !ok || targetField == "" {
		return false, false
	}
	sourceValue, ok := e.flowRecordUpdateSourceValue(item.SourceField, source, sourceDefinition, frame)
	if !ok {
		return false, false
	}
	targetValue, targetOK := target.Fields[targetField]
	if !targetOK {
		targetValue = storage.NullValue()
	}
	field := targetDefinition.Fields[targetField]
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "", "equals", "equal", "eq":
		return storageValuesEqual(field, targetValue, sourceValue), true
	case "notequal", "not equal", "notequals", "not equals", "ne":
		return !storageValuesEqual(field, targetValue, sourceValue), true
	case "contains":
		return strings.Contains(workflowValueString(targetValue), workflowValueString(sourceValue)), true
	case "notcontain", "doesnotcontain":
		return !strings.Contains(workflowValueString(targetValue), workflowValueString(sourceValue)), true
	case "isnull", "isblank":
		return targetValue.Kind == storage.ValueNull, true
	default:
		return false, false
	}
}

func sourceRecordFieldValue(record storage.Record, field string) (storage.Value, bool) {
	if strings.EqualFold(field, "Id") {
		if record.ID == "" {
			return storage.Value{}, false
		}
		return storage.IDValue(record.ID), true
	}
	value, ok := record.Fields[field]
	return value, ok
}

func evaluateWorkflowRule(rule storage.WorkflowRule, record storage.Record, definition storage.ObjectDefinition, org *storage.OrgState) (bool, bool) {
	namespace := ""
	if org != nil {
		namespace = org.Namespace
	}
	if strings.TrimSpace(rule.Formula) != "" {
		if org != nil {
			return evaluateValidationFormulaInOrg(rule.Formula, org, definition, record, nil, false)
		}
		return evaluateValidationFormula(rule.Formula, record)
	}
	if len(rule.Criteria) == 0 {
		return true, true
	}
	for _, item := range rule.Criteria {
		matches, ok := evaluateWorkflowCriteria(item, record, definition, namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func evaluateFlowRule(rule storage.FlowRule, record storage.Record, definition storage.ObjectDefinition, org *storage.OrgState) (bool, bool) {
	namespace := ""
	if org != nil {
		namespace = org.Namespace
	}
	if strings.TrimSpace(rule.Formula) != "" {
		if org != nil {
			return evaluateValidationFormulaInOrg(rule.Formula, org, definition, record, nil, false)
		}
		return evaluateValidationFormula(rule.Formula, record)
	}
	if len(rule.Criteria) == 0 {
		return true, true
	}
	for _, item := range rule.Criteria {
		matches, ok := evaluateWorkflowCriteria(item, record, definition, namespace)
		if !ok || !matches {
			return matches, ok
		}
	}
	return true, true
}

func evaluateWorkflowCriteria(item storage.WorkflowCriteriaItem, record storage.Record, definition storage.ObjectDefinition, namespace string) (bool, bool) {
	field, ok := storage.ResolveFieldName(definition, namespace, strings.TrimSpace(item.Field))
	if !ok || field == "" {
		return false, false
	}
	want := trimFormulaLiteral(item.Value)
	switch strings.ToLower(strings.TrimSpace(item.Operation)) {
	case "", "equals", "equal", "eq":
		return validationFieldEquals(record, field, want), true
	case "notequal", "not equal", "notequals", "not equals", "ne":
		if want == "" && validationFieldBlank(record, field) {
			return false, true
		}
		return !validationFieldEquals(record, field, want), true
	case "greaterthan", "greater than", "gt":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, ">"), true
	case "greaterthanorequalto", "greater than or equal", "greater than or equal to", "gte", "ge":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, ">="), true
	case "lessthan", "less than", "lt":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, "<"), true
	case "lessthanorequalto", "less than or equal", "less than or equal to", "lte", "le":
		return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, "<="), true
	case "contains":
		value, ok := record.Fields[field]
		return ok && strings.Contains(workflowValueString(value), want), true
	case "notcontain", "doesnotcontain":
		value, ok := record.Fields[field]
		return !ok || !strings.Contains(workflowValueString(value), want), true
	case "isnull", "isblank":
		return validationFieldBlank(record, field), true
	default:
		return false, false
	}
}

func workflowUpdateValue(field storage.Field, record storage.Record, update storage.WorkflowFieldUpdate, definition storage.ObjectDefinition, org *storage.OrgState) (storage.Value, bool, bool) {
	namespace := ""
	if org != nil {
		namespace = org.Namespace
	}
	switch {
	case update.SourceField != "":
		sourceField, ok := storage.ResolveFieldName(definition, namespace, update.SourceField)
		if !ok {
			return storage.Value{}, false, false
		}
		value, ok := record.Fields[sourceField]
		if !ok {
			return storage.NullValue(), true, true
		}
		return value.Clone(), false, true
	case update.Formula != "":
		return workflowExpressionValue(field, record, update.Formula, definition, org)
	default:
		return workflowLiteralValue(field, update.LiteralValue)
	}
}

func workflowExpressionValue(field storage.Field, record storage.Record, expression string, definition storage.ObjectDefinition, org *storage.OrgState) (storage.Value, bool, bool) {
	expression = strings.TrimSpace(expression)
	if expression == "" || strings.EqualFold(expression, "NULL") {
		return storage.NullValue(), true, true
	}
	namespace := ""
	if org != nil {
		namespace = org.Namespace
	}
	if fieldName, ok := storage.ResolveFieldName(definition, namespace, expression); ok {
		if value, ok := record.Fields[fieldName]; ok {
			return value.Clone(), false, true
		}
		return storage.NullValue(), true, true
	}
	if org != nil {
		if value, explicitNull, ok := EvaluateRecordFormulaValueInOrg(expression, field, org, definition, record); ok {
			return value, explicitNull, true
		}
	}
	if value, explicitNull, ok := evaluateRecordFormulaValue(expression, field, record); ok {
		return value, explicitNull, true
	}
	return workflowLiteralValue(field, trimFormulaLiteral(expression))
}

func workflowLiteralValue(field storage.Field, literal string) (storage.Value, bool, bool) {
	literal = strings.TrimSpace(literal)
	if strings.EqualFold(literal, "NULL") {
		return storage.NullValue(), true, true
	}
	switch field.Type {
	case storage.FieldCalculated:
		switch strings.ToUpper(field.DisplayType) {
		case "INTEGER":
			var value int64
			if _, err := fmt.Sscanf(literal, "%d", &value); err != nil {
				return storage.Value{}, false, false
			}
			return storage.IntegerValue(value), false, true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return storage.DecimalValue(literal), false, true
		case "BOOLEAN":
			if strings.EqualFold(literal, "true") {
				return storage.BooleanValue(true), false, true
			}
			if strings.EqualFold(literal, "false") {
				return storage.BooleanValue(false), false, true
			}
			return storage.Value{}, false, false
		case "DATE":
			return storage.DateValue(literal), false, true
		case "DATETIME":
			return storage.DateTimeValue(literal), false, true
		case "ID":
			return storage.IDValue(storage.ID(literal)), false, true
		default:
			return storage.StringValue(literal), false, true
		}
	case storage.FieldBoolean:
		if strings.EqualFold(literal, "true") {
			return storage.BooleanValue(true), false, true
		}
		if strings.EqualFold(literal, "false") {
			return storage.BooleanValue(false), false, true
		}
		if literal == "1" {
			return storage.BooleanValue(true), false, true
		}
		if literal == "0" {
			return storage.BooleanValue(false), false, true
		}
		return storage.Value{}, false, false
	case storage.FieldInteger:
		var value int64
		if _, err := fmt.Sscanf(literal, "%d", &value); err != nil {
			return storage.Value{}, false, false
		}
		return storage.IntegerValue(value), false, true
	case storage.FieldDecimal:
		return storage.DecimalValue(literal), false, true
	case storage.FieldID, storage.FieldReference:
		return storage.IDValue(storage.ID(literal)), false, true
	case storage.FieldDate:
		return storage.DateValue(literal), false, true
	case storage.FieldDateTime:
		return storage.DateTimeValue(literal), false, true
	default:
		return storage.StringValue(literal), false, true
	}
}

func workflowValueString(value storage.Value) string {
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueDecimal:
		return value.String
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return fmt.Sprintf("%d", value.Integer)
	case storage.ValueBoolean:
		return fmt.Sprintf("%t", value.Boolean)
	default:
		return ""
	}
}
