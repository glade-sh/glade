package dml

import (
	"fmt"
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
	changed := false
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
			changed = true
		}
		for _, alert := range rule.EmailAlerts {
			if e.WorkflowEmailer == nil {
				continue
			}
			if err := e.WorkflowEmailer(alert, record); err != nil {
				return false, err
			}
		}
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
			if _, err := e.applyFlowEffects(rule.Name, objectName, record, object.Definition, branch.FieldUpdates, nil, nil, nil); err != nil {
				return err
			}
			continue
		}
		if _, err := e.applyFlowEffects(rule.Name, objectName, record, object.Definition, rule.FieldUpdates, nil, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func hasObjectAutomation(definition storage.ObjectDefinition) bool {
	return len(definition.WorkflowRules) > 0 || len(definition.FlowRules) > 0
}

func hasComplexActiveAutomation(definition storage.ObjectDefinition) bool {
	for _, rule := range definition.WorkflowRules {
		if !rule.Active {
			continue
		}
		// Workflow email alerts are external side effects and should use full rollback path.
		if len(rule.EmailAlerts) > 0 {
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
		if len(rule.FieldUpdates) > 0 || len(rule.EmailAlerts) > 0 {
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
			branchChanged, err := e.applyFlowEffects(rule.Name, objectName, &record, object.Definition, branch.FieldUpdates, branch.Actions, branchLookups, branch.RecordCreates)
			if err != nil {
				return false, err
			}
			changed = changed || branchChanged
			continue
		}
		ruleChanged, err := e.applyFlowEffects(rule.Name, objectName, &record, object.Definition, rule.FieldUpdates, rule.Actions, rule.RecordLookups, rule.RecordCreates)
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

func (e *Engine) applyFlowEffects(flowName, objectName string, record *storage.Record, definition storage.ObjectDefinition, updates []storage.WorkflowFieldUpdate, actions []storage.FlowAction, lookups []storage.FlowRecordLookup, creates []storage.FlowRecordCreate) (bool, error) {
	changed := false
	for _, update := range updates {
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
		changed = true
	}
	for _, action := range actions {
		e.traceAutomation("apex.flow.action", map[string]any{
			"flow":   flowName,
			"action": action.Name,
			"object": objectName,
			"record": string(record.ID),
		})
		if e.FlowActionInvoker == nil {
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s requires Apex action execution support", action.Name)
		}
		if err := e.FlowActionInvoker(action, record.Clone()); err != nil {
			return changed, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow action %s failed: %v", action.Name, err)
		}
	}
	for _, create := range creates {
		suppressed, err := e.flowRecordCreateSuppressedByLookup(create, lookups, *record, definition)
		if err != nil {
			return changed, err
		}
		if suppressed {
			e.traceAutomation("apex.flow.record_create_suppressed", map[string]any{
				"flow":   flowName,
				"create": create.Name,
				"object": create.ObjectName,
				"record": string(record.ID),
			})
			continue
		}
		createdID, err := e.executeFlowRecordCreate(create, *record, definition)
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

func (e *Engine) executeFlowRecordCreate(create storage.FlowRecordCreate, source storage.Record, sourceDefinition storage.ObjectDefinition) (storage.ID, error) {
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
		value, explicitNull, ok := flowRecordCreateAssignmentValue(field, source, assignment, sourceDefinition, e.Org)
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

func flowRecordCreateAssignmentValue(field storage.Field, source storage.Record, assignment storage.WorkflowFieldUpdate, sourceDefinition storage.ObjectDefinition, org *storage.OrgState) (storage.Value, bool, bool) {
	namespace := ""
	if org != nil {
		namespace = org.Namespace
	}
	if assignment.SourceField != "" {
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

func (e *Engine) flowRecordCreateSuppressedByLookup(create storage.FlowRecordCreate, lookups []storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, error) {
	createObject, ok := storage.ResolveObjectName(*e.Org, create.ObjectName)
	if !ok {
		return false, nil
	}
	for _, lookup := range lookups {
		lookupObject, ok := storage.ResolveObjectName(*e.Org, lookup.ObjectName)
		if !ok {
			return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record lookup %s targets unknown object %s", lookup.Name, lookup.ObjectName)
		}
		if lookupObject != createObject {
			continue
		}
		matches, err := e.flowRecordLookupMatches(lookup, source, sourceDefinition)
		if err != nil {
			return false, err
		}
		e.traceAutomation("apex.flow.record_lookup", map[string]any{
			"lookup":  lookup.Name,
			"object":  lookup.ObjectName,
			"source":  string(source.ID),
			"matched": matches,
		})
		if matches {
			return true, nil
		}
	}
	return false, nil
}

func (e *Engine) traceAutomation(name string, args map[string]any) {
	if e.AutomationTracer != nil {
		e.AutomationTracer(name, args)
	}
}

func (e *Engine) flowRecordLookupMatches(lookup storage.FlowRecordLookup, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, error) {
	target, targetName, err := e.object(lookup.ObjectName)
	if err != nil {
		return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", nil, "dml: flow record lookup %s targets unknown object %s", lookup.Name, lookup.ObjectName)
	}
	for _, candidate := range target.Records {
		if candidate.System.IsDeleted {
			continue
		}
		matches := true
		for _, item := range lookup.Criteria {
			match, ok := e.evaluateFlowLookupCriteria(item, candidate, target.Definition, source, sourceDefinition)
			if !ok {
				return false, dmlErrorf("CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY", []string{item.Field}, "dml: flow record lookup %s has unsupported criteria for %s.%s", lookup.Name, targetName, item.Field)
			}
			if !match {
				matches = false
				break
			}
		}
		if matches {
			return true, nil
		}
		if lookup.GetFirstRecordOnly {
			continue
		}
	}
	return false, nil
}

func (e *Engine) evaluateFlowLookupCriteria(item storage.WorkflowCriteriaItem, target storage.Record, targetDefinition storage.ObjectDefinition, source storage.Record, sourceDefinition storage.ObjectDefinition) (bool, bool) {
	if strings.TrimSpace(item.SourceField) == "" {
		return evaluateWorkflowCriteria(item, target, targetDefinition, e.Org.Namespace)
	}
	targetField, ok := storage.ResolveFieldName(targetDefinition, e.Org.Namespace, strings.TrimSpace(item.Field))
	if !ok || targetField == "" {
		return false, false
	}
	sourceField, ok := storage.ResolveFieldName(sourceDefinition, e.Org.Namespace, strings.TrimSpace(item.SourceField))
	if !ok || sourceField == "" {
		return false, false
	}
	targetValue, targetOK := target.Fields[targetField]
	sourceValue, sourceOK := sourceRecordFieldValue(source, sourceField)
	if !targetOK {
		targetValue = storage.NullValue()
	}
	if !sourceOK {
		sourceValue = storage.NullValue()
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
